import { Device } from "./core";
import type {
  ChainState,
  ChainStateSnapshot,
  DeviceState,
  SessionState,
  SessionStateSnapshot,
  SessionRole,
} from "./types";
import { copyBytes, ensureLength, fromBase64, toBase64 } from "./utils";

export function ExportDevice(device: Device): DeviceState {
  if (!device) {
    throw new Error("cryptocore: nil device");
  }
  const state: DeviceState = {
    SigningPrivate: toBase64(device.identity.signingPrivate),
    SigningPublic: toBase64(device.identity.signingPublic),
    DHPrivate: toBase64(device.identity.dhPrivate),
    DHPublic: toBase64(device.identity.dhPublic),
    SignedPrekey: {
      Private: toBase64(device.signedPrekey.Private),
      Public: toBase64(device.signedPrekey.Public),
    },
    SignedPrekeySig: toBase64(device.signedSig),
    OneTime: {},
  };
  for (const [id, entry] of device.oneTime.entries()) {
    state.OneTime![id] = {
      Private: toBase64(entry.key.Private),
      Public: toBase64(entry.key.Public),
    };
  }
  if (state.OneTime && Object.keys(state.OneTime).length === 0) {
    state.OneTime = undefined;
  }
  return state;
}

export function ImportDevice(state: DeviceState): Device {
  if (!state) {
    throw new Error("cryptocore: nil device state");
  }
  const signingPriv = fromBase64(state.SigningPrivate);
  const signingPub = fromBase64(state.SigningPublic);
  const dhPriv = decodeFixed(state.DHPrivate, 32, "dh private");
  const dhPub = decodeFixed(state.DHPublic, 32, "dh public");
  const signedPriv = decodeFixed(state.SignedPrekey.Private, 32, "signed prekey private");
  const signedPub = decodeFixed(state.SignedPrekey.Public, 32, "signed prekey public");
  const sig = fromBase64(state.SignedPrekeySig);
  const identity = {
    signingPublic: signingPub,
    signingPrivate: signingPriv,
    dhPrivate: dhPriv,
    dhPublic: dhPub,
  };
  const device = new Device(identity);
  device.signedPrekey = { Private: signedPriv, Public: signedPub };
  device.signedSig = copyBytes(sig);
  device.oneTime = new Map();
  if (state.OneTime) {
    for (const [key, value] of Object.entries(state.OneTime)) {
      const priv = decodeFixed(value.Private, 32, "one-time private");
      const pub = decodeFixed(value.Public, 32, "one-time public");
      device.oneTime.set(key, { key: { Private: priv, Public: pub } });
    }
  }
  return device;
}

export function ExportSession(state: SessionState): SessionStateSnapshot {
  if (!state) {
    throw new Error("cryptocore: nil session");
  }
  const snapshot: SessionStateSnapshot = {
    RootKey: toBase64(state.RootKey),
    SendChain: exportChain(state.SendChain),
    RecvChain: exportChain(state.RecvChain),
    RatchetPrivate: toBase64(state.RatchetPrivate),
    RatchetPublic: toBase64(state.RatchetPublic),
    RemoteRatchet: toBase64(state.RemoteRatchet),
    RemoteIdentity: toBase64(state.RemoteIdentity),
    RemoteSignature: toBase64(state.RemoteSignature),
    PN: state.PN,
    Role: state.Role as SessionRole,
    PendingPrekey: state.PendingPrekey,
    Skipped: {},
  };
  for (const [k, v] of state.skipped.entries()) {
    snapshot.Skipped![k] = toBase64(v);
  }
  if (snapshot.Skipped && Object.keys(snapshot.Skipped).length === 0) {
    snapshot.Skipped = undefined;
  }
  return snapshot;
}

export function ImportSession(snapshot: SessionStateSnapshot): SessionState {
  if (!snapshot) {
    throw new Error("cryptocore: nil session snapshot");
  }
  const root = decodeFixed(snapshot.RootKey, 32, "root key");
  const send = importChain(snapshot.SendChain);
  const recv = importChain(snapshot.RecvChain);
  const ratchetPriv = decodeFixed(snapshot.RatchetPrivate, 32, "ratchet private");
  const ratchetPub = decodeFixed(snapshot.RatchetPublic, 32, "ratchet public");
  const remoteRatchet = decodeFixed(snapshot.RemoteRatchet, 32, "remote ratchet");
  const remoteIdentity = decodeFixed(snapshot.RemoteIdentity, 32, "remote identity");
  const remoteSig = fromBase64(snapshot.RemoteSignature);
  const session: SessionState = {
    RootKey: root,
    SendChain: send,
    RecvChain: recv,
    RatchetPrivate: ratchetPriv,
    RatchetPublic: ratchetPub,
    RemoteRatchet: remoteRatchet,
    RemoteIdentity: remoteIdentity,
    RemoteSignature: remoteSig,
    PN: snapshot.PN,
    Role: snapshot.Role,
    PendingPrekey: snapshot.PendingPrekey,
    skipped: new Map(),
  };
  if (snapshot.Skipped) {
    for (const [k, v] of Object.entries(snapshot.Skipped)) {
      const keyBytes = decodeFixed(v, 32, "skipped key");
      session.skipped.set(k, keyBytes);
    }
  }
  return session;
}

function exportChain(cs: ChainState): ChainStateSnapshot {
  return {
    Key: toBase64(cs.Key),
    Index: cs.Index,
  };
}

function importChain(cs: ChainStateSnapshot): ChainState {
  return {
    Key: decodeFixed(cs.Key, 32, "chain key"),
    Index: cs.Index,
  };
}

function decodeFixed(input: string, size: number, name: string): Uint8Array {
  const data = fromBase64(input);
  if (data.length !== size) {
    throw new Error(`cryptocore: unexpected length for ${name}: ${data.length}, want ${size}`);
  }
  return ensureLength(data, size, name);
}
