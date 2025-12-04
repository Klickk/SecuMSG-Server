import { ed25519, x25519 } from "@noble/curves/ed25519";
import { sha512 } from "@noble/hashes/sha512";
import type {
  IdentityKeyPair,
  KeyPair,
  OneTimeEntry,
  OneTimePrekey,
  PrekeyBundle,
} from "./types";
import { copyBytes } from "./utils";

export type RandomSource = (buffer: Uint8Array) => void;

const defaultRandomSource: RandomSource = (buffer: Uint8Array) => {
  const cryptoObj =
    typeof globalThis !== "undefined" ? globalThis.crypto : undefined;
  if (!cryptoObj || typeof cryptoObj.getRandomValues !== "function") {
    throw new Error("cryptocore: secure randomness is unavailable");
  }
  cryptoObj.getRandomValues(buffer);
};

let randomnessSrc: RandomSource = defaultRandomSource;

export function UseDeterministicRandom(source: RandomSource): () => void {
  const prev = randomnessSrc;
  randomnessSrc = source;
  return () => {
    randomnessSrc = prev;
  };
}

function readRandom(length: number): Uint8Array {
  const buf = new Uint8Array(length);
  randomnessSrc(buf);
  return buf;
}

export class Device {
  identity: IdentityKeyPair;
  signedPrekey: KeyPair;
  signedSig: Uint8Array;
  oneTime: Map<string, OneTimeEntry>;

  constructor(identity: IdentityKeyPair) {
    this.identity = identity;
    this.signedPrekey = {
      Private: new Uint8Array(32),
      Public: new Uint8Array(32),
    };
    this.signedSig = new Uint8Array();
    this.oneTime = new Map();
  }

  rotateSignedPrekey(): void {
    const kp = generateX25519KeyPair();
    const sig = ed25519.sign(kp.Public, this.identity.signingPrivate);
    this.signedPrekey = kp;
    this.signedSig = copyBytes(sig);
  }

  PublishPrekeyBundle(oneTimeCount: number): PrekeyBundle {
    if (oneTimeCount < 0) {
      oneTimeCount = 0;
    }
    if (
      this.signedPrekey.Public.length === 0 ||
      isZeroKey(this.signedPrekey.Public)
    ) {
      this.rotateSignedPrekey();
    }
    const bundle: PrekeyBundle = {
      IdentityKey: copyBytes(this.identity.dhPublic),
      IdentitySignatureKey: copyBytes(this.identity.signingPublic),
      SignedPrekey: copyBytes(this.signedPrekey.Public),
      SignedPrekeySig: copyBytes(this.signedSig),
      OneTimePrekeys: [],
    };
    for (let i = 0; i < oneTimeCount; i += 1) {
      const kp = generateX25519KeyPair();
      const id = generateUUID();
      this.oneTime.set(id, { key: kp });
      bundle.OneTimePrekeys.push({ ID: id, Public: copyBytes(kp.Public) });
    }
    return bundle;
  }

  IdentityPublic(): { dh: Uint8Array; signing: Uint8Array } {
    return {
      dh: copyBytes(this.identity.dhPublic),
      signing: copyBytes(this.identity.signingPublic),
    };
  }
}

export function GenerateIdentityKeypair(): Device {
  const seed = readRandom(32);
  const signingPublic = ed25519.getPublicKey(seed);
  const dhPrivate = ed25519PrivToCurve25519(seed);
  const dhPublic = x25519.scalarMultBase(dhPrivate);
  const identity: IdentityKeyPair = {
    signingPublic: copyBytes(signingPublic),
    signingPrivate: copyBytes(seed),
    dhPrivate: copyBytes(dhPrivate),
    dhPublic: copyBytes(dhPublic),
  };
  const device = new Device(identity);
  device.rotateSignedPrekey();
  return device;
}

export function ed25519PrivToCurve25519(seed: Uint8Array): Uint8Array {
  const h = sha512(seed);
  h[0] &= 248;
  h[31] &= 127;
  h[31] |= 64;
  return h.slice(0, 32);
}

export function generateX25519KeyPair(): KeyPair {
  const priv = readRandom(32);
  priv[0] &= 248;
  priv[31] &= 127;
  priv[31] |= 64;
  const pub = x25519.scalarMultBase(priv);
  return {
    Private: copyBytes(priv),
    Public: copyBytes(pub),
  };
}

function isZeroKey(key: Uint8Array): boolean {
  return key.every((b) => b === 0);
}

const generateUUID = (): string => {
  const bytes = readRandom(16);
  // Per RFC 4122 section 4.4
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;

  const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, "0"));
  return [
    hex.slice(0, 4).join(""),
    hex.slice(4, 6).join(""),
    hex.slice(6, 8).join(""),
    hex.slice(8, 10).join(""),
    hex.slice(10, 16).join(""),
  ].join("-");
};
