import { KeyLockedError, KeyManager } from "../crypto-core/keyManager";
import { equalsBytes, fromBase64, toBase64, utf8 } from "../crypto-core/utils";
import {
  STORE_NAMES,
  getItemFromStore,
  listKeysInStore,
  removeItemFromStore,
  setItemInStore,
} from "./storage";

const SECURE_SCHEMA_VERSION = 1;
const SECURE_PREFIX = "secure:";
const IV_BYTES = 12;

export class SecureStoreLockedError extends Error {
  code = "ERR_LOCKED";
  constructor() {
    super("securestore: locked");
  }
}

export type SecureRecord = {
  ciphertext: string;
  iv: string;
  aad: string;
  schemaVersion: number;
  encAlg: "AES-GCM-256";
};

export class SecureStore {
  constructor(private keyManager: KeyManager, private userId: string) {}

  async securePut(storeName: string, key: string, value: unknown): Promise<void> {
    const payload = JSON.stringify(value);
    const iv = randomBytes(IV_BYTES);
    const aad = buildAad(storeName, key, this.userId, SECURE_SCHEMA_VERSION);

    const ciphertext = await this.withDekCrypto(async (cryptoKey) => {
      const encoded = utf8(payload);
      const encrypted = await crypto.subtle.encrypt(
        {
          name: "AES-GCM",
          iv: toBufferSource(iv),
          additionalData: toBufferSource(aad),
        },
        cryptoKey,
        toBufferSource(encoded)
      );
      return new Uint8Array(encrypted);
    });

    const record: SecureRecord = {
      ciphertext: toBase64(ciphertext),
      iv: toBase64(iv),
      aad: toBase64(aad),
      schemaVersion: SECURE_SCHEMA_VERSION,
      encAlg: "AES-GCM-256",
    };

    await setItemInStore(
      STORE_NAMES.secure,
      SecureStore.makeKey(storeName, key),
      JSON.stringify(record)
    );
  }

  async secureGet<T>(storeName: string, key: string): Promise<T | null> {
    const raw = await getItemFromStore(
      STORE_NAMES.secure,
      SecureStore.makeKey(storeName, key)
    );
    if (!raw) {
      return null;
    }
    const record = JSON.parse(raw) as SecureRecord;
    if (record.schemaVersion !== SECURE_SCHEMA_VERSION) {
      throw new Error("securestore: unsupported schema version");
    }
    const aad = buildAad(storeName, key, this.userId, record.schemaVersion);
    if (!equalsBytes(aad, fromBase64(record.aad))) {
      throw new Error("securestore: AAD mismatch");
    }
    const plaintext = await this.withDekCrypto(async (cryptoKey) => {
      const iv = fromBase64(record.iv);
      const ciphertext = fromBase64(record.ciphertext);
      const decrypted = await crypto.subtle.decrypt(
        {
          name: "AES-GCM",
          iv: toBufferSource(iv),
          additionalData: toBufferSource(aad),
        },
        cryptoKey,
        toBufferSource(ciphertext)
      );
      return new Uint8Array(decrypted);
    });
    const decoded = new TextDecoder().decode(plaintext);
    return JSON.parse(decoded) as T;
  }

  async secureDelete(storeName: string, key: string): Promise<void> {
    await removeItemFromStore(
      STORE_NAMES.secure,
      SecureStore.makeKey(storeName, key)
    );
  }

  async secureList(storeName: string): Promise<string[]> {
    const prefix = SecureStore.prefixFor(storeName);
    const keys = await listKeysInStore(STORE_NAMES.secure, prefix);
    return keys.map((full) => full.slice(prefix.length));
  }

  async hasRecord(storeName: string, key: string): Promise<boolean> {
    const raw = await getItemFromStore(
      STORE_NAMES.secure,
      SecureStore.makeKey(storeName, key)
    );
    return !!raw;
  }

  static makeKey(storeName: string, key: string): string {
    return `${SECURE_PREFIX}${storeName}:${key}`;
  }

  static prefixFor(storeName: string): string {
    return `${SECURE_PREFIX}${storeName}:`;
  }

  private async withDekCrypto<T>(fn: (cryptoKey: CryptoKey) => Promise<T>): Promise<T> {
    try {
      return await this.keyManager.withDEK(async (dek) => {
        try {
          if (typeof crypto === "undefined" || !crypto.subtle) {
            throw new Error("securestore: WebCrypto not available");
          }
          const cryptoKey = await crypto.subtle.importKey(
            "raw",
            toBufferSource(dek),
            "AES-GCM",
            false,
            ["encrypt", "decrypt"]
          );
          return fn(cryptoKey);
        } finally {
          dek.fill(0);
        }
      });
    } catch (err) {
      if (err instanceof KeyLockedError) {
        throw new SecureStoreLockedError();
      }
      throw err;
    }
  }
}

function randomBytes(length: number): Uint8Array {
  if (typeof crypto === "undefined" || !crypto.getRandomValues) {
    throw new Error("securestore: WebCrypto not available");
  }
  const out = new Uint8Array(length);
  crypto.getRandomValues(out);
  return out;
}

function buildAad(
  storeName: string,
  key: string,
  userId: string,
  version: number
): Uint8Array {
  const origin = typeof window !== "undefined" ? window.location.origin : "unknown";
  return utf8(JSON.stringify({ storeName, key, userId, version, origin }));
}

export const SECURE_STORE_VERSION = SECURE_SCHEMA_VERSION;

function toBufferSource(data: Uint8Array): ArrayBuffer {
  const copy = new Uint8Array(data.byteLength);
  copy.set(data);
  return copy.buffer;
}

export async function hasSecureRecord(storeName: string, key: string): Promise<boolean> {
  const raw = await getItemFromStore(
    STORE_NAMES.secure,
    SecureStore.makeKey(storeName, key)
  );
  return !!raw;
}
