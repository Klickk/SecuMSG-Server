import { getItem, setItem, removeItem, listKeysInStore, STORE_NAMES } from "./storage";
import { SecureStore } from "./secureStore";
import type { InboundMessage, OutboundMessage } from "./messagingClient";

export type PersistedMessage = {
  direction: "inbound" | "outbound";
  convId: string;
  peerDeviceId: string;
  plaintext: string;
  sentAt: string;
};

const keyForConv = (convId: string) => `conv:${convId}:messages`;
const SECURE_STORE = "messages";

export async function appendMessages(
  convId: string,
  messages: PersistedMessage[],
  secureStore?: SecureStore
): Promise<void> {
  if (messages.length === 0) return;

  const existing = await loadMessages(convId, secureStore);
  const merged = [...existing, ...messages];
  merged.sort(
    (a, b) => new Date(a.sentAt).getTime() - new Date(b.sentAt).getTime()
  );
  if (secureStore) {
    await secureStore.securePut(SECURE_STORE, convId, merged);
    return;
  }
  await setItem(keyForConv(convId), JSON.stringify(merged));
}

export async function loadMessages(
  convId: string,
  secureStore?: SecureStore
): Promise<PersistedMessage[]> {
  if (secureStore) {
    const decrypted = await secureStore.secureGet<PersistedMessage[]>(
      SECURE_STORE,
      convId
    );
    return decrypted ?? [];
  }
  const raw = await getItem(keyForConv(convId));
  if (!raw) return [];
  try {
    const parsed = JSON.parse(raw) as PersistedMessage[];
    return parsed.map((m) => ({ ...m }));
  } catch (err) {
    console.error("Failed to parse stored messages", err);
    return [];
  }
}

export async function latestTimestamp(
  convId: string,
  secureStore?: SecureStore
): Promise<Date | null> {
  const messages = await loadMessages(convId, secureStore);
  if (messages.length === 0) return null;
  const last = messages[messages.length - 1];
  return new Date(last.sentAt);
}

export function deserializeMessages(
  entries: PersistedMessage[]
): (InboundMessage | OutboundMessage)[] {
  return entries.map((msg) => ({
    direction: msg.direction,
    convId: msg.convId,
    peerDeviceId: msg.peerDeviceId,
    plaintext: msg.plaintext,
    sentAt: new Date(msg.sentAt),
  }));
}

export function serializeMessages(
  entries: (InboundMessage | OutboundMessage)[]
): PersistedMessage[] {
  return entries.map((msg) => ({
    direction: msg.direction,
    convId: msg.convId,
    peerDeviceId: msg.peerDeviceId,
    plaintext: msg.plaintext,
    sentAt: msg.sentAt.toISOString(),
  }));
}

export async function migrateLegacyMessages(secureStore: SecureStore): Promise<void> {
  const keys = await listKeysInStore(STORE_NAMES.plaintext, "conv:");
  for (const key of keys) {
    const match = /^conv:(.+):messages$/.exec(key);
    if (!match) {
      continue;
    }
    const convId = match[1];
    if (await secureStore.hasRecord(SECURE_STORE, convId)) {
      await removeItem(key);
      continue;
    }
    const raw = await getItem(key);
    if (!raw) {
      continue;
    }
    try {
      const parsed = JSON.parse(raw) as PersistedMessage[];
      await secureStore.securePut(SECURE_STORE, convId, parsed);
      await removeItem(key);
    } catch (err) {
      console.error("Failed to migrate legacy messages", err);
    }
  }
}
