import { KeyManager, UnlockThrottledError } from "./keyManager";
import { equalsBytes } from "./utils";

export async function runKeyManagerHarness(): Promise<void> {
  const manager = new KeyManager("harness-user", { inactivityMs: 2000 });

  await manager.setupPin("1234");
  if (!(await manager.hasWrappedKey())) {
    throw new Error("expected wrapped key record");
  }
  if (!manager.isUnlocked()) {
    throw new Error("expected manager to be unlocked after setup");
  }

  const initialDek = manager.withDEK((dek) => dek);
  manager.lock();
  if (manager.isUnlocked()) {
    throw new Error("expected manager to be locked");
  }

  let throttled = false;
  try {
    await manager.unlock("0000");
  } catch (err) {
    throttled = err instanceof UnlockThrottledError;
  }

  await manager.unlock("1234");
  const unlockedDek = manager.withDEK((dek) => dek);
  if (!equalsBytes(initialDek, unlockedDek)) {
    throw new Error("expected DEK to round trip through wrap/unwrap");
  }

  if (throttled) {
    console.log("Unlock throttling engaged as expected.");
  }
  console.log("KeyManager harness completed successfully.");
}
