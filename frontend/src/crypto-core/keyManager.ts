import { getItem, setItem } from "../lib/storage";
import { copyBytes, equalsBytes, fromBase64, toBase64, utf8 } from "./utils";

const WRAPPED_DEK_KEY = "keymanager:wrapped-dek";
const ATTEMPTS_KEY = "keymanager:unlock-attempts";
const RECORD_VERSION = 1;
const DEK_BYTES = 32;
const SALT_BYTES = 16;
const WRAP_IV_BYTES = 12;

const PBKDF2_PARAMS = {
  algo: "PBKDF2",
  hash: "SHA-256",
  iterations: 250_000,
  keyLength: 32,
} as const;

export type LockReason = "manual" | "inactivity" | "visibility" | "pagehide";

export class KeyLockedError extends Error {
  constructor() {
    super("keymanager: locked");
  }
}

export class UnlockThrottledError extends Error {
  constructor(public waitMs: number) {
    super(`keymanager: unlock throttled for ${waitMs}ms`);
  }
}

export interface KeyManagerOptions {
  userId: string;
  inactivityMs?: number;
  visibilityDelayMs?: number;
  onLock?: (reason: LockReason) => void;
  onNeedsUnlock?: () => void;
}

interface WrappedDekRecord {
  wrappedDEK: string;
  wrapIv: string;
  wrapAad: string;
  kdfParams: typeof PBKDF2_PARAMS;
  salt: string;
  version: number;
}

interface UnlockAttempts {
  failCount: number;
  nextAllowedAt: number;
}

export class KeyManager {
  private dek: Uint8Array | null = null;
  private inactivityMs: number;
  private visibilityDelayMs: number;
  private onLock?: (reason: LockReason) => void;
  private onNeedsUnlock?: () => void;
  private inactivityTimer: number | null = null;
  private visibilityTimer: number | null = null;
  private attempts: UnlockAttempts | null = null;

  constructor(private userId: string, options: Omit<KeyManagerOptions, "userId"> = {}) {
    this.inactivityMs = options.inactivityMs ?? 5 * 60 * 1000;
    this.visibilityDelayMs = options.visibilityDelayMs ?? 0;
    this.onLock = options.onLock;
    this.onNeedsUnlock = options.onNeedsUnlock;

    if (typeof window !== "undefined") {
      window.addEventListener("visibilitychange", this.handleVisibility);
      window.addEventListener("pagehide", this.handlePageHide);
      window.addEventListener("beforeunload", this.handlePageHide);
      this.installActivityListeners();
    }
  }

  setCallbacks(callbacks: Pick<KeyManagerOptions, "onLock" | "onNeedsUnlock">): void {
    if (callbacks.onLock) {
      this.onLock = callbacks.onLock;
    }
    if (callbacks.onNeedsUnlock) {
      this.onNeedsUnlock = callbacks.onNeedsUnlock;
    }
  }

  setTiming(options: Pick<KeyManagerOptions, "inactivityMs" | "visibilityDelayMs">): void {
    if (typeof options.inactivityMs === "number") {
      this.inactivityMs = options.inactivityMs;
    }
    if (typeof options.visibilityDelayMs === "number") {
      this.visibilityDelayMs = options.visibilityDelayMs;
    }
  }

  async hasWrappedKey(): Promise<boolean> {
    const record = await loadWrappedRecord();
    return !!record;
  }

  isUnlocked(): boolean {
    return !!this.dek;
  }

  lock(reason: LockReason = "manual"): void {
    if (this.dek) {
      this.dek.fill(0);
      this.dek = null;
    }
    if (typeof window !== "undefined") {
      if (this.inactivityTimer) {
        window.clearTimeout(this.inactivityTimer);
        this.inactivityTimer = null;
      }
      if (this.visibilityTimer) {
        window.clearTimeout(this.visibilityTimer);
        this.visibilityTimer = null;
      }
    }
    this.onLock?.(reason);
  }

  withDEK<T>(fn: (dek: Uint8Array) => T): T {
    if (!this.dek) {
      this.onNeedsUnlock?.();
      throw new KeyLockedError();
    }
    return fn(copyBytes(this.dek));
  }

  async setupPin(pin: string): Promise<void> {
    ensurePin(pin);
    const dek = randomBytes(DEK_BYTES);
    const salt = randomBytes(SALT_BYTES);
    const wrapIv = randomBytes(WRAP_IV_BYTES);
    const aad = buildAad(this.userId, RECORD_VERSION);
    const kek = await deriveKek(pin, salt, PBKDF2_PARAMS);
    const wrappedDEK = await aesGcmEncrypt(kek, wrapIv, aad, dek);

    const record: WrappedDekRecord = {
      wrappedDEK: toBase64(wrappedDEK),
      wrapIv: toBase64(wrapIv),
      wrapAad: toBase64(aad),
      kdfParams: PBKDF2_PARAMS,
      salt: toBase64(salt),
      version: RECORD_VERSION,
    };
    await setItem(WRAPPED_DEK_KEY, JSON.stringify(record));
    await this.resetAttempts();
    this.dek = dek;
    this.resetInactivityTimer();
  }

  async unlock(pin: string): Promise<void> {
    ensurePin(pin);
    const record = await loadWrappedRecord();
    if (!record) {
      throw new Error("keymanager: no wrapped key record");
    }

    const attempts = await this.loadAttempts();
    const now = Date.now();
    if (attempts && now < attempts.nextAllowedAt) {
      throw new UnlockThrottledError(attempts.nextAllowedAt - now);
    }

    const aad = buildAad(this.userId, record.version);
    if (!equalsBytes(aad, fromBase64(record.wrapAad))) {
      throw new Error("keymanager: AAD mismatch");
    }

    try {
      const salt = fromBase64(record.salt);
      const wrapIv = fromBase64(record.wrapIv);
      const kek = await deriveKek(pin, salt, record.kdfParams);
      const dek = await aesGcmDecrypt(kek, wrapIv, aad, fromBase64(record.wrappedDEK));
      this.dek = dek;
      await this.resetAttempts();
      this.resetInactivityTimer();
    } catch (err) {
      await this.recordFailedAttempt();
      throw err;
    }
  }

  private resetInactivityTimer(): void {
    if (!this.dek) {
      return;
    }
    if (typeof window === "undefined") {
      return;
    }
    if (this.inactivityTimer) {
      window.clearTimeout(this.inactivityTimer);
    }
    this.inactivityTimer = window.setTimeout(() => {
      this.lock("inactivity");
    }, this.inactivityMs);
  }

  private installActivityListeners(): void {
    const events = ["mousemove", "keydown", "click", "touchstart", "scroll"];
    events.forEach((event) => {
      window.addEventListener(event, this.handleActivity, { passive: true });
    });
  }

  private handleActivity = (): void => {
    if (this.dek) {
      this.resetInactivityTimer();
    }
  };

  private handleVisibility = (): void => {
    if (document.hidden) {
      if (this.visibilityDelayMs > 0) {
        if (this.visibilityTimer) {
          window.clearTimeout(this.visibilityTimer);
        }
        this.visibilityTimer = window.setTimeout(() => {
          this.lock("visibility");
        }, this.visibilityDelayMs);
      } else {
        this.lock("visibility");
      }
    } else if (this.visibilityTimer) {
      window.clearTimeout(this.visibilityTimer);
      this.visibilityTimer = null;
    }
  };

  private handlePageHide = (): void => {
    this.lock("pagehide");
  };

  private async loadAttempts(): Promise<UnlockAttempts | null> {
    if (this.attempts) {
      return this.attempts;
    }
    const raw = await getItem(ATTEMPTS_KEY);
    if (!raw) {
      return null;
    }
    const parsed = JSON.parse(raw) as UnlockAttempts;
    this.attempts = parsed;
    return parsed;
  }

  private async recordFailedAttempt(): Promise<void> {
    const attempts = (await this.loadAttempts()) ?? { failCount: 0, nextAllowedAt: 0 };
    const failCount = attempts.failCount + 1;
    const backoffMs = Math.min(60_000, 1000 * 2 ** (failCount - 1));
    const nextAllowedAt = Date.now() + backoffMs;
    const updated = { failCount, nextAllowedAt };
    this.attempts = updated;
    await setItem(ATTEMPTS_KEY, JSON.stringify(updated));
  }

  private async resetAttempts(): Promise<void> {
    const reset = { failCount: 0, nextAllowedAt: 0 };
    this.attempts = reset;
    await setItem(ATTEMPTS_KEY, JSON.stringify(reset));
  }
}

function ensurePin(pin: string): void {
  if (!/^\d{4}$/.test(pin)) {
    throw new Error("keymanager: PIN must be 4 digits");
  }
}

function buildAad(userId: string, version: number): Uint8Array {
  const origin = typeof window !== "undefined" ? window.location.origin : "unknown";
  const aadJson = JSON.stringify({ userId, origin, version });
  return utf8(aadJson);
}

function randomBytes(length: number): Uint8Array {
  if (typeof crypto === "undefined" || !crypto.getRandomValues) {
    throw new Error("keymanager: WebCrypto not available");
  }
  const out = new Uint8Array(length);
  crypto.getRandomValues(out);
  return out;
}

async function deriveKek(
  pin: string,
  salt: Uint8Array,
  params: typeof PBKDF2_PARAMS
): Promise<CryptoKey> {
  if (typeof crypto === "undefined" || !crypto.subtle) {
    throw new Error("keymanager: WebCrypto not available");
  }
  const baseKey = await crypto.subtle.importKey(
    "raw",
    toBufferSource(utf8(pin)),
    "PBKDF2",
    false,
    ["deriveKey"]
  );
  return crypto.subtle.deriveKey(
    {
      name: "PBKDF2",
      salt: toBufferSource(salt),
      iterations: params.iterations,
      hash: params.hash,
    },
    baseKey,
    { name: "AES-GCM", length: params.keyLength * 8 },
    false,
    ["encrypt", "decrypt"]
  );
}

async function aesGcmEncrypt(
  key: CryptoKey,
  iv: Uint8Array,
  aad: Uint8Array,
  plaintext: Uint8Array
): Promise<Uint8Array> {
  const ciphertext = await crypto.subtle.encrypt(
    {
      name: "AES-GCM",
      iv: toBufferSource(iv),
      additionalData: toBufferSource(aad),
    },
    key,
    toBufferSource(plaintext)
  );
  return new Uint8Array(ciphertext);
}

async function aesGcmDecrypt(
  key: CryptoKey,
  iv: Uint8Array,
  aad: Uint8Array,
  ciphertext: Uint8Array
): Promise<Uint8Array> {
  const plaintext = await crypto.subtle.decrypt(
    {
      name: "AES-GCM",
      iv: toBufferSource(iv),
      additionalData: toBufferSource(aad),
    },
    key,
    toBufferSource(ciphertext)
  );
  return new Uint8Array(plaintext);
}

function toBufferSource(data: Uint8Array): ArrayBuffer {
  const copy = new Uint8Array(data.byteLength);
  copy.set(data);
  return copy.buffer;
}

async function loadWrappedRecord(): Promise<WrappedDekRecord | null> {
  const raw = await getItem(WRAPPED_DEK_KEY);
  if (!raw) {
    return null;
  }
  return JSON.parse(raw) as WrappedDekRecord;
}
