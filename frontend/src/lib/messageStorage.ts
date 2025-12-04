import { getItem, setItem } from "./storage";
import type { InboundMessage, OutboundMessage } from "./messagingClient";

export type PersistedMessage = {
  direction: "inbound" | "outbound";
  convId: string;
  peerDeviceId: string;
  plaintext: string;
  sentAt: string;
};

const keyForConv = (convId: string) => `conv:${convId}:messages`;

export async function appendMessages(
  convId: string,
  messages: PersistedMessage[]
): Promise<void> {
  if (messages.length === 0) return;

  const existing = await loadMessages(convId);
  const merged = [...existing, ...messages];
  merged.sort(
    (a, b) => new Date(a.sentAt).getTime() - new Date(b.sentAt).getTime()
  );
  await setItem(keyForConv(convId), JSON.stringify(merged));
}

export async function loadMessages(convId: string): Promise<PersistedMessage[]> {
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

export async function latestTimestamp(convId: string): Promise<Date | null> {
  const messages = await loadMessages(convId);
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
