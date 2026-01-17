import { KeyManager, type KeyManagerOptions } from "../crypto-core/keyManager";

let currentManager: KeyManager | null = null;
let currentUserId: string | null = null;

export function getKeyManager(
  userId: string,
  options: Omit<KeyManagerOptions, "userId"> = {}
): KeyManager {
  if (!currentManager || currentUserId !== userId) {
    currentManager?.lock("manual");
    currentManager = new KeyManager(userId, options);
    currentUserId = userId;
    return currentManager;
  }
  currentManager.setCallbacks({
    onLock: options.onLock,
    onNeedsUnlock: options.onNeedsUnlock,
  });
  currentManager.setTiming({
    inactivityMs: options.inactivityMs,
    visibilityDelayMs: options.visibilityDelayMs,
  });
  return currentManager;
}

export function resetKeyManager(): void {
  currentManager?.lock("manual");
  currentManager = null;
  currentUserId = null;
}
