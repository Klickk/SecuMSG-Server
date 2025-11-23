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
  MessageHeader,
  type PrekeyBundle,
  type SessionState,
} from "../crypto-core";
import { fromBase64, toBase64, utf8 } from "../crypto-core/utils";
import {
  appendMessages,
  deserializeMessages,
  latestTimestamp,
  loadMessages,
  type PersistedMessage,
} from "./messageStorage";
import { getItem, setItem } from "./storage";
import { getApiBaseUrl } from "../config/config";

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
    oneTimePrekeyId?: string;
  };
  ratchet: {
    dhPublic: string;
    pn: number;
    n: number;
    nonce: string;
  };
};

type ByteLike = string | number[] | ArrayBuffer | Uint8Array | ArrayBufferView;

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
    sessions: Map<string, SessionState> = new Map()
  ) {
    this.device = device;
    this.sessions = sessions;
  }

  static async registerDevice(
    userId: string,
    deviceName: string,
    baseUrl: string
  ): Promise<{
    client: MessagingClient;
    device: {
      deviceId: string;
      userId: string;
      name: string;
      platform: string;
    };
  }> {
    const device = GenerateIdentityKeypair();
    const bundle = device.PublishPrekeyBundle(5);

    const authPayload = {
      UserID: userId,
      Name: deviceName,
      Platform: navigator.userAgent,
    };

    const authResp = await axios.post(
      `${baseUrl}/auth/devices/register`,
      authPayload
    );
    const deviceInfo = authResp.data as {
      deviceId: string;
      userId: string;
      name: string;
      platform: string;
    };

    const keysPayload = {
      UserID: userId,
      DeviceID: deviceInfo.deviceId,
      IdentityKey: toBase64(bundle.IdentityKey),
      IdentitySignatureKey: toBase64(bundle.IdentitySignatureKey),
      SignedPreKey: {
        PublicKey: toBase64(bundle.SignedPrekey),
        Signature: toBase64(bundle.SignedPrekeySig),
        CreatedAt: new Date().toISOString(),
      },
      OneTimePreKeys: bundle.OneTimePrekeys.map((x) => ({
        ID: x.ID.toString(),
        PublicKey: toBase64(x.Public),
      })),
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
      new Map()
    );

    await client.save();

    return { client, device: deviceInfo };
  }

  static async load(): Promise<MessagingClient | null> {
    const stored = await getItem(STORAGE_KEY);
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
      const baseUrl = getApiBaseUrl();
      return new MessagingClient(
        {
          userId: parsed.userId,
          deviceId: parsed.deviceId,
          keysBaseUrl: baseUrl,
          messagesBaseUrl: baseUrl,
        },
        device,
        sessions
      );
    } catch (err) {
      console.error("Failed to load messaging state", err);
      return null;
    }
  }

  async save(): Promise<void> {
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

    await setItem(STORAGE_KEY, JSON.stringify(snapshot));
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
    plaintext: string
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

    await this.save();

    const outbound: OutboundMessage = {
      direction: "outbound",
      convId,
      peerDeviceId: toDeviceId,
      plaintext,
      sentAt: new Date(),
    };

    await appendMessages(convId, [serialize(outbound)]);

    return outbound;
  }

  async handleEnvelope(env: InboundEnvelope): Promise<InboundMessage> {
    const ciphertext = toBytes(env.ciphertext);
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
    const clear = new TextDecoder().decode(toBytes(plaintextBytes));

    await this.save();

    const inbound: InboundMessage = {
      direction: "inbound",
      convId: env.conv_id,
      peerDeviceId: env.from_device_id,
      plaintext: clear,
      sentAt: new Date(env.sent_at),
    };

    await appendMessages(env.conv_id, [serialize(inbound)]);

    return inbound;
  }

  connectWebSocket(
    onMessage: (msg: InboundMessage) => void,
    onStatus?: (state: "open" | "closed" | "error") => void
  ): WebSocket {
    const url = buildWebSocketURL(
      this.state.messagesBaseUrl,
      this.state.deviceId
    );
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
    toDeviceId: string
  ): Promise<{ session: SessionState; handshake?: HandshakeMessage }> {
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
    const endpoint = `${
      this.state.keysBaseUrl
    }/keys/bundle?device_id=${encodeURIComponent(deviceId)}`;
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
      bundle.OneTimePrekeys.push({
        ID: data.oneTimePreKey.id,
        Public: toUint32Array(data.oneTimePreKey.publicKey),
      });
    }
    return bundle;
  }

  async fetchConversationIds(): Promise<string[]> {
    const response = await axios.get(
      `${this.state.messagesBaseUrl}/messages/conversations`,
      {
        params: { device_id: this.state.deviceId },
      }
    );

    const payload = response.data as { conversations?: string[] };
    return payload.conversations ?? [];
  }

  async fetchHistorySince(
    convId?: string,
    since?: Date,
    limit = 100
  ): Promise<InboundMessage[]> {
    const params: Record<string, string> = {
      device_id: this.state.deviceId,
      limit: limit.toString(),
    };
    if (convId) {
      params.conv_id = convId;
    }
    if (since) {
      params.since = since.toISOString();
    }

    const response = await axios.get(
      `${this.state.messagesBaseUrl}/messages/history`,
      {
        params,
      }
    );

    const payload = response.data as { messages: InboundEnvelope[] };
    const sorted = (payload.messages ?? []).sort(
      (a, b) => new Date(a.sent_at).getTime() - new Date(b.sent_at).getTime()
    );

    const results: InboundMessage[] = [];
    for (const env of sorted) {
      const msg = await this.handleEnvelope(env);
      results.push(msg);
    }

    await this.save();
    return results;
  }

  async loadLocalHistory(
    convId: string
  ): Promise<(InboundMessage | OutboundMessage)[]> {
    const stored = await loadMessages(convId);
    return deserializeMessages(stored);
  }

  async latestLocalMessage(convId: string): Promise<Date | null> {
    return latestTimestamp(convId);
  }
}

function toUint32Array(b64: ByteLike): Uint8Array {
  const buf = toBytes(b64);
  if (buf.length !== 32) {
    throw new Error(`expected 32 bytes got ${buf.length}`);
  }
  return buf;
}

function buildHeaderPayload(
  header: MessageHeader,
  handshake?: HandshakeMessage
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

function serialize(msg: InboundMessage | OutboundMessage): PersistedMessage {
  return {
    direction: msg.direction,
    convId: msg.convId,
    peerDeviceId: msg.peerDeviceId,
    plaintext: msg.plaintext,
    sentAt: msg.sentAt.toISOString(),
  };
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
      const nonce = toBytes(p.nonce);
      if (nonce.length !== 12) {
        throw new Error(`invalid nonce length ${nonce.length}`);
      }
      return nonce;
    })()
  );
}

function toBytes(input: ByteLike): Uint8Array {
  if (typeof input === "string") {
    return fromBase64(input);
  }
  if (input instanceof Uint8Array) {
    return input;
  }
  if (ArrayBuffer.isView(input)) {
    return new Uint8Array(input.buffer, input.byteOffset, input.byteLength);
  }
  if (input instanceof ArrayBuffer) {
    return new Uint8Array(input);
  }
  if (Array.isArray(input)) {
    return new Uint8Array(input);
  }
  throw new Error("unsupported byte input");
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
