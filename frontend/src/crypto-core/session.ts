import { ed25519 } from "@noble/curves/ed25519";
import { curve25519 } from "@noble/curves/curve25519";
import { hkdf } from "@noble/hashes/hkdf";
import { sha256 } from "@noble/hashes/sha256";
import { Device, generateX25519KeyPair } from "./core";
import { ErrInvalidPrekeySignature, ErrMissingOneTimeKey } from "./errors";
import type {
  ChainState,
  HandshakeMessage,
  OneTimePrekey,
  PrekeyBundle,
  SessionRole,
  SessionState,
} from "./types";
import { copyBytes, concatBytes, utf8, zeroBytes } from "./utils";

const hkdfInfoX3DH = "SecuMSG-X3DH";

export function InitSession(
  d: Device,
  bundle: PrekeyBundle,
): { session: SessionState; message: HandshakeMessage } {
  if (!d) {
    throw new Error("cryptocore: nil device");
  }
  if (!bundle) {
    throw new Error("cryptocore: nil bundle");
  }
  verifyPrekeyBundle(bundle);

  const ephemeral = generateX25519KeyPair();
  const otk = bundle.OneTimePrekeys.length > 0 ? bundle.OneTimePrekeys[0] : undefined;
  const secret = deriveSharedSecretInitiator(d, bundle, ephemeral, otk);
  const { root, chain } = deriveInitialKeys(secret);

  const pending = otk ? otk.ID : undefined;

  const session: SessionState = {
    RootKey: root,
    SendChain: { Key: chain, Index: 0 },
    RecvChain: { Key: zeroBytes(32), Index: 0 },
    RatchetPrivate: copyBytes(ephemeral.Private),
    RatchetPublic: copyBytes(ephemeral.Public),
    RemoteRatchet: copyBytes(bundle.SignedPrekey),
    RemoteIdentity: copyBytes(bundle.IdentityKey),
    RemoteSignature: copyBytes(bundle.IdentitySignatureKey),
    PN: 0,
    Role: SessionRole.RoleInitiator,
    PendingPrekey: pending,
    skipped: new Map(),
  };

  const msg: HandshakeMessage = {
    IdentityKey: copyBytes(d.identity.dhPublic),
    IdentitySignatureKey: copyBytes(d.identity.signingPublic),
    EphemeralKey: copyBytes(ephemeral.Public),
    OneTimePrekeyID: pending,
  };

  return { session, message: msg };
}

export function AcceptSession(d: Device, msg: HandshakeMessage): SessionState {
  if (!d) {
    throw new Error("cryptocore: nil device");
  }
  if (!msg) {
    throw new Error("cryptocore: nil handshake message");
  }
  let otk: { Private: Uint8Array; Public: Uint8Array } | undefined;
  if (typeof msg.OneTimePrekeyID === "number") {
    const entry = d.oneTime.get(msg.OneTimePrekeyID);
    if (!entry) {
      throw ErrMissingOneTimeKey;
    }
    otk = entry.key;
    d.oneTime.delete(msg.OneTimePrekeyID);
  }
  const secret = deriveSharedSecretResponder(d, msg, otk);
  const { root, chain } = deriveInitialKeys(secret);

  const session: SessionState = {
    RootKey: root,
    SendChain: { Key: zeroBytes(32), Index: 0 },
    RecvChain: { Key: chain, Index: 0 },
    RatchetPrivate: copyBytes(d.signedPrekey.Private),
    RatchetPublic: copyBytes(d.signedPrekey.Public),
    RemoteRatchet: copyBytes(msg.EphemeralKey),
    RemoteIdentity: copyBytes(msg.IdentityKey),
    RemoteSignature: copyBytes(msg.IdentitySignatureKey),
    PN: 0,
    Role: SessionRole.RoleResponder,
    PendingPrekey: msg.OneTimePrekeyID,
    skipped: new Map(),
  };
  return session;
}

function verifyPrekeyBundle(bundle: PrekeyBundle): void {
  if (bundle.IdentitySignatureKey.length !== 32) {
    throw ErrInvalidPrekeySignature;
  }
  const valid = ed25519.verify(
    bundle.SignedPrekeySig,
    bundle.SignedPrekey,
    bundle.IdentitySignatureKey,
  );
  if (!valid) {
    throw ErrInvalidPrekeySignature;
  }
}

function deriveSharedSecretInitiator(
  d: Device,
  bundle: PrekeyBundle,
  eph: ReturnType<typeof generateX25519KeyPair>,
  otk?: OneTimePrekey,
): Uint8Array {
  const dh1 = curve25519.scalarMult(d.identity.dhPrivate, bundle.SignedPrekey);
  const dh2 = curve25519.scalarMult(eph.Private, bundle.IdentityKey);
  const dh3 = curve25519.scalarMult(eph.Private, bundle.SignedPrekey);
  let secret = concatBytes(dh1, dh2, dh3);
  if (otk) {
    const dh4 = curve25519.scalarMult(eph.Private, otk.Public);
    secret = concatBytes(secret, dh4);
  }
  return secret;
}

function deriveSharedSecretResponder(
  d: Device,
  msg: HandshakeMessage,
  otk?: { Private: Uint8Array; Public: Uint8Array },
): Uint8Array {
  const dh1 = curve25519.scalarMult(d.signedPrekey.Private, msg.IdentityKey);
  const dh2 = curve25519.scalarMult(d.identity.dhPrivate, msg.EphemeralKey);
  const dh3 = curve25519.scalarMult(d.signedPrekey.Private, msg.EphemeralKey);
  let secret = concatBytes(dh1, dh2, dh3);
  if (otk) {
    const dh4 = curve25519.scalarMult(otk.Private, msg.EphemeralKey);
    secret = concatBytes(secret, dh4);
  }
  return secret;
}

function deriveInitialKeys(secret: Uint8Array): { root: ChainState["Key"]; chain: ChainState["Key"]; } {
  const okm = hkdf(sha256, secret, new Uint8Array(), utf8(hkdfInfoX3DH), 64);
  return {
    root: okm.slice(0, 32),
    chain: okm.slice(32, 64),
  };
}
