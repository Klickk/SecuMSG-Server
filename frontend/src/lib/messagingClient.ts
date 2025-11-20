import axios from "axios";
import {
  AcceptSession,
  Decrypt,
  Encrypt,
  ExportDevice,
  ExportSession,
  GenerateIdentityKeypair,
  ImportDevice,
  ImportSession,
  InitSession,
  type Device,
  type HandshakeMessage,
  type MessageHeader,
  type PrekeyBundle,
  type SessionState,
} from "../crypto-core";
import { fromBase64, toBase64, utf8 } from "../crypto-core/utils";

export type InboundEnvelope = {
  id: string;
  conv_id: string;
  from_device_id: string;
  to_device_id: string;
  ciphertext: string;
  header: HeaderPayload;
  sent_at: string;
};

export type HeaderPayload = {
  handshake?: {
    identityKey: string;
    identitySignatureKey: string;
    ephemeralKey: string;
    oneTimePrekeyId?: number;
  };
  ratchet: {
    dhPublic: string;
    pn: number;
    n: number;
    nonce: string;
  };
};

export type StoredMessagingState = {
  userId: string;
  deviceId: string;
  keysBaseUrl: string;
  messagesBaseUrl: string;
  device: ReturnType<typeof ExportDevice>;
  sessions?: Record<string, ReturnType<typeof ExportSession>>;
};

export type OutboundMessage = {
  direction: "outbound";
  convId: string;
  peerDeviceId: string;
  plaintext: string;
  sentAt: Date;
};

export type InboundMessage = {
  direction: "inbound";
  convId: string;
  peerDeviceId: string;
  plaintext: string;
  sentAt: Date;
};

const STORAGE_KEY = "secumsg-state";

export class MessagingClient {
  private device: Device;
  private sessions: Map<string, SessionState>;

  constructor(
    private state: {
      userId: string;
      deviceId: string;
      keysBaseUrl: string;
      messagesBaseUrl: string;
    },
    device: Device,
    sessions: Map<string, SessionState> = new Map(),
  ) {
    this.device = device;
    this.sessions = sessions;
  }

  static async registerDevice(
    userId: string,
    deviceName: string,
    baseUrl: string,
  ): Promise<{
    client: MessagingClient;
    device: { deviceId: string; userId: string; name: string; platform: string };
  }>
  {
    const device = GenerateIdentityKeypair();
    const bundle = device.PublishPrekeyBundle(0);

    const authPayload = {
      UserID: userId,
      Name: deviceName,
      Platform: navigator.userAgent,
      KeyBundle: {
        IdentityKeyPub: toBase64(bundle.IdentityKey),
        SignedPrekeyPub: toBase64(bundle.SignedPrekey),
        SignedPrekeySig: toBase64(bundle.SignedPrekeySig),
        OneTimePrekeys: [],
      },
    };

    const authResp = await axios.post(`${baseUrl}/auth/devices/register`, authPayload);
    const deviceInfo = authResp.data as {
      deviceId: string;
      userId: string;
      name: string;
      platform: string;
    };

    const keysPayload = {
      userId,
      deviceId: deviceInfo.deviceId,
      identityKey: toBase64(bundle.IdentityKey),
      identitySignatureKey: toBase64(bundle.IdentitySignatureKey),
      signedPreKey: {
        publicKey: toBase64(bundle.SignedPrekey),
        signature: toBase64(bundle.SignedPrekeySig),
        createdAt: new Date().toISOString(),
      },
      oneTimePreKeys: [] as Array<{ id?: string; publicKey: string }>,
    };

    await axios.post(`${baseUrl}/keys/device/register`, keysPayload);

    const client = new MessagingClient(
      {
        userId,
        deviceId: deviceInfo.deviceId,
        keysBaseUrl: baseUrl,
        messagesBaseUrl: baseUrl,
      },
      device,
      new Map(),
    );

    client.save();

    return { client, device: deviceInfo };
  }

  static load(): MessagingClient | null {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (!stored) return null;
    try {
      const parsed: StoredMessagingState = JSON.parse(stored);
      const device = ImportDevice(parsed.device);
      const sessions = new Map<string, SessionState>();
      if (parsed.sessions) {
        for (const [id, snap] of Object.entries(parsed.sessions)) {
          sessions.set(id, ImportSession(snap));
        }
      }
      return new MessagingClient(
        {
          userId: parsed.userId,
          deviceId: parsed.deviceId,
          keysBaseUrl: parsed.keysBaseUrl,
          messagesBaseUrl: parsed.messagesBaseUrl,
        },
        device,
        sessions,
      );
    } catch (err) {
      console.error("Failed to load messaging state", err);
      return null;
    }
  }

  save(): void {
    const snapshot: StoredMessagingState = {
      userId: this.state.userId,
      deviceId: this.state.deviceId,
      keysBaseUrl: this.state.keysBaseUrl,
      messagesBaseUrl: this.state.messagesBaseUrl,
      device: ExportDevice(this.device),
    };

    if (this.sessions.size > 0) {
      snapshot.sessions = {};
      for (const [id, sess] of this.sessions.entries()) {
        snapshot.sessions[id] = ExportSession(sess);
      }
    }

    localStorage.setItem(STORAGE_KEY, JSON.stringify(snapshot));
  }

  deviceId(): string {
    return this.state.deviceId;
  }

  userId(): string {
    return this.state.userId;
  }

  async sendMessage(
    convId: string,
    toDeviceId: string,
    plaintext: string,
  ): Promise<OutboundMessage> {
    const { session, handshake } = await this.ensureSession(convId, toDeviceId);

    const { ciphertext, header } = Encrypt(session, utf8(plaintext));
    const headerPayload = buildHeaderPayload(header, handshake);

    await axios.post(`${this.state.messagesBaseUrl}/messages/send`, {
      conv_id: convId,
      from_device_id: this.state.deviceId,
      to_device_id: toDeviceId,
      ciphertext: toBase64(ciphertext),
      header: headerPayload,
    });

    this.save();

    return {
      direction: "outbound",
      convId,
      peerDeviceId: toDeviceId,
      plaintext,
      sentAt: new Date(),
    };
  }

  async handleEnvelope(env: InboundEnvelope): Promise<InboundMessage> {
    const ciphertext = fromBase64(env.ciphertext);
    const header = payloadToMessageHeader(env.header.ratchet);

    let session = this.sessions.get(env.conv_id);
    if (!session) {
      if (!env.header.handshake) {
        throw new Error("missing handshake for new session");
      }
      const hs = payloadToHandshake(env.header.handshake);
      session = AcceptSession(this.device, hs);
      this.sessions.set(env.conv_id, session);
    }

    const plaintextBytes = Decrypt(session, ciphertext, header);
    const clear = new TextDecoder().decode(plaintextBytes);

    this.save();

    return {
      direction: "inbound",
      convId: env.conv_id,
      peerDeviceId: env.from_device_id,
      plaintext: clear,
      sentAt: new Date(env.sent_at),
    };
  }

  connectWebSocket(
    onMessage: (msg: InboundMessage) => void,
    onStatus?: (state: "open" | "closed" | "error") => void,
  ): WebSocket {
    const url = buildWebSocketURL(this.state.messagesBaseUrl, this.state.deviceId);
    const ws = new WebSocket(url);

    ws.onopen = () => {
      onStatus?.("open");
    };

    ws.onclose = () => {
      onStatus?.("closed");
    };

    ws.onerror = () => {
      onStatus?.("error");
    };

    ws.onmessage = async (event) => {
      try {
        const env = JSON.parse(event.data) as InboundEnvelope;
        const msg = await this.handleEnvelope(env);
        onMessage(msg);
      } catch (err) {
        console.error("Failed to process inbound message", err);
      }
    };

    return ws;
  }

  private async ensureSession(
    convId: string,
    toDeviceId: string,
  ): Promise<{ session: SessionState; handshake?: HandshakeMessage }>
  {
    const existing = this.sessions.get(convId);
    if (existing) {
      return { session: existing };
    }

    const bundle = await this.fetchBundle(toDeviceId);
    const { session, message } = InitSession(this.device, bundle);
    this.sessions.set(convId, session);
    return { session, handshake: message };
  }

  private async fetchBundle(deviceId: string): Promise<PrekeyBundle> {
    const endpoint = `${this.state.keysBaseUrl}/keys/bundle?device_id=${encodeURIComponent(deviceId)}`;
    const response = await axios.get(endpoint);
    const data = response.data as {
      deviceId: string;
      identityKey: string;
      identitySignatureKey: string;
      signedPreKey: { publicKey: string; signature: string };
      oneTimePreKey?: { id: string; publicKey: string };
    };

    const bundle: PrekeyBundle = {
      IdentityKey: toUint32Array(data.identityKey),
      IdentitySignatureKey: fromBase64(data.identitySignatureKey),
      SignedPrekey: toUint32Array(data.signedPreKey.publicKey),
      SignedPrekeySig: fromBase64(data.signedPreKey.signature),
      OneTimePrekeys: [],
    };

    if (data.oneTimePreKey) {
      const id = Number.parseInt(data.oneTimePreKey.id, 10);
      if (!Number.isNaN(id)) {
        bundle.OneTimePrekeys.push({
          ID: id,
          Public: toUint32Array(data.oneTimePreKey.publicKey),
        });
      }
    }

    return bundle;
  }
}

function toUint32Array(b64: string): Uint8Array {
  const buf = fromBase64(b64);
  if (buf.length !== 32) {
    throw new Error(`expected 32 bytes got ${buf.length}`);
  }
  return buf;
}

function buildHeaderPayload(
  header: MessageHeader,
  handshake?: HandshakeMessage,
): HeaderPayload {
  const payload: HeaderPayload = {
    ratchet: {
      dhPublic: toBase64(header.DHPublic),
      pn: header.PN,
      n: header.N,
      nonce: toBase64(header.Nonce),
    },
  };

  if (handshake) {
    payload.handshake = {
      identityKey: toBase64(handshake.IdentityKey),
      identitySignatureKey: toBase64(handshake.IdentitySignatureKey),
      ephemeralKey: toBase64(handshake.EphemeralKey),
      oneTimePrekeyId: handshake.OneTimePrekeyID,
    };
  }

  return payload;
}

function payloadToHandshake(p: HeaderPayload["handshake"]): HandshakeMessage {
  if (!p) {
    throw new Error("nil handshake payload");
  }
  return {
    IdentityKey: toUint32Array(p.identityKey),
    IdentitySignatureKey: fromBase64(p.identitySignatureKey),
    EphemeralKey: toUint32Array(p.ephemeralKey),
    OneTimePrekeyID: p.oneTimePrekeyId,
  };
}

function payloadToMessageHeader(p: HeaderPayload["ratchet"]): MessageHeader {
  if (!p) {
    throw new Error("nil ratchet payload");
  }
  return new MessageHeader(
    toUint32Array(p.dhPublic),
    p.pn,
    p.n,
    (() => {
      const nonce = fromBase64(p.nonce);
      if (nonce.length !== 12) {
        throw new Error(`invalid nonce length ${nonce.length}`);
      }
      return nonce;
    })(),
  );
}

function buildWebSocketURL(base: string, deviceId: string): string {
  const trimmed = base.replace(/\/+$/, "");
  const withPath = `${trimmed}/ws?device_id=${encodeURIComponent(deviceId)}`;
  if (withPath.startsWith("http://")) {
    return "ws://" + withPath.slice("http://".length);
  }
  if (withPath.startsWith("https://")) {
    return "wss://" + withPath.slice("https://".length);
  }
  return withPath;
}
