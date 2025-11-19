import { ChaCha20Poly1305 } from "@stablelib/chacha20poly1305";
import { curve25519 } from "@noble/curves/curve25519";
import { hkdf } from "@noble/hashes/hkdf";
import { sha256 } from "@noble/hashes/sha256";
import { hmac } from "@noble/hashes/hmac";
import { generateX25519KeyPair } from "./core";
import {
  ErrDecryptionFailed,
  ErrDuplicateMessage,
  ErrInvalidRemoteKey,
} from "./errors";
import type { MessageHeader, SessionState } from "./types";
import { copyBytes, equalsBytes, zeroBytes, utf8 } from "./utils";

const hkdfInfoRatchet = "SecuMSG-DR";
const hkdfInfoAEAD = "SecuMSG-AEAD";
const maxSkippedMessageKeys = 64;

export function Encrypt(
  session: SessionState,
  plaintext: Uint8Array,
): { ciphertext: Uint8Array; header: MessageHeader } {
  if (!session) {
    throw new Error("cryptocore: nil session");
  }
  if (isZeroKey(session.SendChain.Key)) {
    RotateRatchetOnSend(session, undefined);
  }
  const { chainKey: newCK, messageKey: mk } = kdfChain(session.SendChain.Key);
  const n = session.SendChain.Index;
  session.SendChain.Key = newCK;
  session.SendChain.Index += 1;

  const { key, nonce } = deriveCipherParams(mk);
  const aead = new ChaCha20Poly1305(key);
  const header = new MessageHeader(copyBytes(session.RatchetPublic), session.PN, n, nonce);
  const ad = header.associatedData();
  const ciphertext = aead.seal(nonce, plaintext, ad);
  return { ciphertext, header };
}

export function Decrypt(session: SessionState, ciphertext: Uint8Array, header: MessageHeader): Uint8Array {
  if (!session) {
    throw new Error("cryptocore: nil session");
  }
  if (!header) {
    throw new Error("cryptocore: nil header");
  }
  const skipped = consumeSkipped(session, header);
  if (skipped) {
    const { key, nonce } = deriveCipherParams(skipped);
    const aead = new ChaCha20Poly1305(key);
    try {
      return aead.open(nonce, ciphertext, header.associatedData());
    } catch {
      throw ErrDecryptionFailed;
    }
  }
  RotateRatchetOnRecv(session, header);
  if (header.N < session.RecvChain.Index) {
    throw ErrDuplicateMessage;
  }
  while (session.RecvChain.Index < header.N) {
    const { chainKey: newCK, messageKey } = kdfChain(session.RecvChain.Key);
    storeSkippedKey(session, session.RemoteRatchet, session.RecvChain.Index, messageKey);
    session.RecvChain.Key = newCK;
    session.RecvChain.Index += 1;
  }
  const { chainKey: newCK, messageKey: mk } = kdfChain(session.RecvChain.Key);
  session.RecvChain.Key = newCK;
  session.RecvChain.Index += 1;
  const { key, nonce } = deriveCipherParams(mk);
  const aead = new ChaCha20Poly1305(key);
  try {
    return aead.open(nonce, ciphertext, header.associatedData());
  } catch {
    throw ErrDecryptionFailed;
  }
}

export function RotateRatchetOnSend(session: SessionState, _header?: MessageHeader): void {
  if (!session) {
    throw new Error("cryptocore: nil session");
  }
  if (isZeroKey(session.RemoteRatchet)) {
    throw ErrInvalidRemoteKey;
  }
  const kp = generateX25519KeyPair();
  const dh = curve25519.scalarMult(kp.Private, session.RemoteRatchet);
  const { root, chain } = kdfRoot(session.RootKey, dh);
  session.RootKey = root;
  session.PN = session.SendChain.Index;
  session.SendChain = { Key: chain, Index: 0 };
  session.RatchetPrivate = copyBytes(kp.Private);
  session.RatchetPublic = copyBytes(kp.Public);
}

export function RotateRatchetOnRecv(session: SessionState, header: MessageHeader): void {
  if (!session) {
    throw new Error("cryptocore: nil session");
  }
  if (!header) {
    throw new Error("cryptocore: nil header");
  }
  if (equalsBytes(header.DHPublic, session.RemoteRatchet)) {
    return;
  }
  const dh = curve25519.scalarMult(session.RatchetPrivate, header.DHPublic);
  const { root, chain } = kdfRoot(session.RootKey, dh);
  session.RootKey = root;
  session.RemoteRatchet = copyBytes(header.DHPublic);
  session.RecvChain = { Key: chain, Index: 0 };
  session.SendChain = { Key: zeroBytes(32), Index: 0 };
  session.PN = header.PN;
}

function kdfRoot(root: Uint8Array, dh: Uint8Array): { root: Uint8Array; chain: Uint8Array } {
  const okm = hkdf(sha256, dh, root, utf8(hkdfInfoRatchet), 64);
  return {
    root: okm.slice(0, 32),
    chain: okm.slice(32, 64),
  };
}

function kdfChain(chain: Uint8Array): { chainKey: Uint8Array; messageKey: Uint8Array } {
  const mk = hmac(sha256, chain, new Uint8Array([0x01]));
  const out = hmac(sha256, chain, new Uint8Array([0x02]));
  return {
    chainKey: mk.slice(0, 32),
    messageKey: out.slice(0, 32),
  };
}

function deriveCipherParams(messageKey: Uint8Array): { key: Uint8Array; nonce: Uint8Array } {
  const okm = hkdf(sha256, messageKey, new Uint8Array(), utf8(hkdfInfoAEAD), 44);
  return {
    key: okm.slice(0, 32),
    nonce: okm.slice(32, 44),
  };
}

function isZeroKey(key: Uint8Array): boolean {
  return key.every((b) => b === 0);
}

function storeSkippedKey(
  session: SessionState,
  pub: Uint8Array,
  index: number,
  key: Uint8Array,
): void {
  if (!session.skipped) {
    session.skipped = new Map();
  }
  if (session.skipped.size >= maxSkippedMessageKeys) {
    const firstKey = session.skipped.keys().next().value;
    if (firstKey) {
      session.skipped.delete(firstKey);
    }
  }
  session.skipped.set(skippedKey(pub, index), copyBytes(key));
}

function consumeSkipped(session: SessionState, header: MessageHeader): Uint8Array | undefined {
  if (!session.skipped || session.skipped.size === 0) {
    return undefined;
  }
  const name = skippedKey(header.DHPublic, header.N);
  const val = session.skipped.get(name);
  if (val) {
    session.skipped.delete(name);
    return copyBytes(val);
  }
  return undefined;
}

function skippedKey(pub: Uint8Array, index: number): string {
  const buf = new Uint8Array(36);
  buf.set(pub, 0);
  const view = new DataView(buf.buffer, buf.byteOffset + 32, 4);
  view.setUint32(0, index, false);
  let out = "";
  for (let i = 0; i < buf.length; i += 1) {
    out += String.fromCharCode(buf[i]);
  }
  return out;
}
