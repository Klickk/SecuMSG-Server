import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { KeyManager, UnlockThrottledError } from "../crypto-core/keyManager";
import {
  InboundMessage,
  MessagingClient,
  OutboundMessage,
} from "../lib/messagingClient";
import { getItem, setItem } from "../lib/storage";
import { SecureStore } from "../lib/secureStore";
import { migrateLegacyMessages } from "../lib/messageStorage";
import { PinModal } from "./PinModal";
import { getKeyManager } from "../lib/keyManagerInstance";
import { ActivityTracker } from "../lib/activityTracker";
import { resolveContact } from "../services/resolveContact";
import { resolveContactByDevice } from "../services/resolveDevice";

type Contact = {
  deviceId: string;
  label: string;
  username?: string;
  convId: string;
  lastActivity?: string;
};

const CONTACTS_KEY = "secumsg-contacts";

const defaultConvId = () => crypto.randomUUID();

export const MessagingPage: React.FC = () => {
  const [client, setClient] = useState<MessagingClient | null>(null);
  const [lockState, setLockState] = useState<"checking" | "locked" | "unlocked">(
    "checking"
  );
  const [unlockError, setUnlockError] = useState<string | null>(null);
  const [unlockWaitSeconds, setUnlockWaitSeconds] = useState<number | null>(null);
  const keyManagerRef = useRef<ReturnType<typeof getKeyManager> | null>(null);
  const activityRef = useRef<ActivityTracker | null>(null);
  const [contacts, setContacts] = useState<Contact[]>([]);
  const [contactsInitialized, setContactsInitialized] = useState(false);
  const [selectedConvId, setSelectedConvId] = useState<string | null>(null);
  const [text, setText] = useState<string>("");
  const [messages, setMessages] = useState<
    (InboundMessage | OutboundMessage)[]
  >([]);
  const [username, setUsername] = useState<string | null>(null);
  const [wsStatus, setWsStatus] = useState<string>(
    "Connecting to message stream..."
  );
  const [showAddContact, setShowAddContact] = useState(false);
  const [newContact, setNewContact] = useState({
    username: "",
    label: "",
  });
  const [resolvingContact, setResolvingContact] = useState(false);
  const [resolveError, setResolveError] = useState<string | null>(null);
  const [historyFetched, setHistoryFetched] = useState<Record<string, boolean>>(
    {}
  );
  const [localHistoryLoaded, setLocalHistoryLoaded] = useState(false);
  const [serverConversationIds, setServerConversationIds] = useState<string[]>(
    []
  );
  const navigate = useNavigate();

  const handleLock = useCallback(() => {
    setLockState("locked");
    setClient(null);
    setMessages([]);
    setContacts([]);
    setContactsInitialized(false);
    setHistoryFetched({});
    setLocalHistoryLoaded(false);
    activityRef.current?.stop();
  }, []);

  const resolveUsernameForDevice = useCallback(
    async (deviceId: string, convId?: string) => {
      try {
        const res = await resolveContactByDevice(deviceId);
        setContacts((prev) =>
          prev.map((c) => {
            const match =
              c.deviceId === deviceId || (convId && c.convId === convId);
            if (!match) return c;
            const nextLabel =
              c.label === "New contact" || c.label.trim() === ""
                ? res.username
                : c.label;
            return {
              ...c,
              deviceId: res.deviceId,
              username: res.username,
              label: nextLabel,
            };
          })
        );
      } catch (err) {
        console.warn("Could not resolve username for device", err);
      }
    },
    []
  );

  useEffect(() => {
    (async () => {
      const userId = await getItem("userId");
      if (!userId) {
        navigate("/");
        return;
      }
      const manager = getKeyManager(userId, {
        onLock: handleLock,
        onNeedsUnlock: () => setLockState("locked"),
        inactivityMs: 24 * 60 * 60 * 1000,
      });
      keyManagerRef.current = manager;
      const hasWrapped = await manager.hasWrappedKey();
      if (!hasWrapped) {
        setUnlockError("Set up a PIN to protect your device.");
        setLockState("locked");
        return;
      }
      if (manager.isUnlocked()) {
        setLockState("unlocked");
      } else {
        setLockState("locked");
      }
    })();
    return () => {
      activityRef.current?.stop();
    };
  }, [handleLock, navigate]);

  useEffect(() => {
    if (lockState !== "unlocked") {
      activityRef.current?.stop();
      return;
    }
    const manager = keyManagerRef.current;
    if (!manager) {
      return;
    }
    const tracker = new ActivityTracker({
      idleMs: 5 * 60 * 1000,
      onIdle: () => manager.lock("inactivity"),
    });
    activityRef.current = tracker;
    tracker.start();
    return () => {
      tracker.stop();
    };
  }, [lockState]);

  useEffect(() => {
    if (lockState !== "unlocked") {
      return;
    }
    let cancelled = false;
    let socket: WebSocket | null = null;
    (async () => {
      const userId = await getItem("userId");
      if (!userId) {
        navigate("/");
        return;
      }
      const manager = keyManagerRef.current;
      if (!manager) {
        return;
      }
      const store = new SecureStore(manager, userId);
      await migrateLegacyMessages(store);
      const loaded = await MessagingClient.load(store);
      if (cancelled) return;
      if (!loaded) {
        navigate("/dRegister");
        return;
      }
      setClient(loaded);

      try {
        socket = await loaded.connectWebSocket(
          (msg) => {
            setMessages((prev) => [...prev, msg]);
            setContacts((prev) => {
              const existingByConv = prev.find((c) => c.convId === msg.convId);
              if (existingByConv) {
                return prev.map((c) =>
                  c.convId === msg.convId
                    ? { ...c, lastActivity: msg.sentAt.toISOString() }
                    : c
                );
              }

              const byDeviceIdx = prev.findIndex(
                (c) => c.deviceId === msg.peerDeviceId
              );
              if (byDeviceIdx >= 0) {
                const clone = [...prev];
                clone[byDeviceIdx] = {
                  ...clone[byDeviceIdx],
                  convId: msg.convId,
                  lastActivity: msg.sentAt.toISOString(),
                };
                return clone;
              }

              return [
                ...prev,
                {
                  deviceId: msg.peerDeviceId,
                  label: "New contact",
                  convId: msg.convId,
                  lastActivity: msg.sentAt.toISOString(),
                },
              ];
            });
            resolveUsernameForDevice(msg.peerDeviceId, msg.convId);
            setSelectedConvId((prev) => prev ?? msg.convId);
          },
          (state) => {
            if (state === "open") setWsStatus("Listening for incoming messages");
            if (state === "closed") setWsStatus("Connection closed");
            if (state === "error")
              setWsStatus("Connection error – retry or refresh");
          }
        );
      } catch (err) {
        console.error("Failed to open websocket", err);
        setWsStatus("Authentication required to connect.");
      }
    })();

    return () => {
      cancelled = true;
      socket?.close();
    };
  }, [lockState, navigate, resolveUsernameForDevice]);

  useEffect(() => {
    if (!unlockWaitSeconds || unlockWaitSeconds <= 0) {
      return;
    }
    const timer = window.setTimeout(() => {
      setUnlockWaitSeconds(null);
    }, unlockWaitSeconds * 1000);
    return () => {
      window.clearTimeout(timer);
    };
  }, [unlockWaitSeconds]);

  useEffect(() => {
    if (!client) return;
    if (contacts.length === 0) {
      setLocalHistoryLoaded(true);
      return;
    }
    let cancelled = false;
    setLocalHistoryLoaded(false);
    (async () => {
      const all: (InboundMessage | OutboundMessage)[] = [];
      for (const contact of contacts) {
        const convMessages = await client.loadLocalHistory(contact.convId);
        if (cancelled) return;
        all.push(...convMessages);
      }
      all.sort((a, b) => a.sentAt.getTime() - b.sentAt.getTime());
      setMessages(all);
      if (!cancelled) {
        setLocalHistoryLoaded(true);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [client, contacts]);

  useEffect(() => {
    if (!contactsInitialized || lockState !== "unlocked") return;
    (async () => {
      await setItem(CONTACTS_KEY, JSON.stringify(contacts));
    })();
  }, [contacts, contactsInitialized, lockState]);

  useEffect(() => {
    let cancelled = false;
    const loadContacts = async () => {
      try {
        const stored = await getItem(CONTACTS_KEY);
        if (!cancelled && stored) {
          const parsed = JSON.parse(stored) as Contact[];
          setContacts(parsed);
          setSelectedConvId((prev) => prev ?? parsed[0]?.convId ?? null);
        }
      } catch (err) {
        console.error("Failed to load contacts", err);
      } finally {
        if (!cancelled) {
          setContactsInitialized(true);
        }
      }
    };
    if (lockState === "unlocked") {
      loadContacts();
    }
    return () => {
      cancelled = true;
    };
  }, [lockState]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const storedUsername = await getItem("username");
      if (!cancelled) {
        setUsername(storedUsername);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    // Backfill usernames for any contacts missing them
    (async () => {
      for (const contact of contacts) {
        if (!contact.username) {
          await resolveUsernameForDevice(contact.deviceId, contact.convId);
        }
      }
    })();
  }, [contacts, resolveUsernameForDevice]);

  useEffect(() => {
    if (!selectedConvId && contacts.length > 0) {
      setSelectedConvId(contacts[0].convId);
    }
  }, [contacts, selectedConvId]);

  const header = useMemo(() => {
    if (!client) return "";
    return username ? `Signed in as ${username}` : "Secure messaging";
  }, [client, username]);

  const activeContact = useMemo(
    () => contacts.find((c) => c.convId === selectedConvId) ?? null,
    [contacts, selectedConvId]
  );

  const activeMessages = useMemo(() => {
    if (!activeContact) return [];
    return messages
      .filter((m) => m.convId === activeContact.convId)
      .slice()
      .sort((a, b) => a.sentAt.getTime() - b.sentAt.getTime());
  }, [activeContact, messages]);

  const syncHistoryForConversation = useCallback(
    async (convId: string) => {
      if (!client) return [] as (InboundMessage | OutboundMessage)[];
      const last = await client.latestLocalMessage(convId);
      const since = last ? new Date(last.getTime() + 1000) : undefined;
      const fetched = await client.fetchHistorySince(convId, since);
      if (fetched.length === 0) return fetched;

      const mostRecent = fetched[fetched.length - 1];

      setContacts((prev) => {
        const existing = prev.find((c) => c.convId === convId);
        if (existing) {
          return prev.map((c) =>
            c.convId === convId
              ? {
                  ...c,
                  deviceId: existing.deviceId || mostRecent.peerDeviceId,
                  lastActivity: mostRecent.sentAt.toISOString(),
                }
              : c
          );
        }

        return [
          ...prev,
          {
            deviceId: mostRecent.peerDeviceId,
            label: "New contact",
            convId,
            lastActivity: mostRecent.sentAt.toISOString(),
          },
        ];
      });

      setMessages((prev) => [...prev, ...fetched]);
      return fetched;
    },
    [client]
  );

  useEffect(() => {
    if (!client || !activeContact) return;
    if (!localHistoryLoaded) return; // wait until local history is available to compute "last"
    if (historyFetched[activeContact.convId]) return;
    let cancelled = false;
    (async () => {
      try {
        await syncHistoryForConversation(activeContact.convId);
      } catch (err) {
        console.error("Failed to sync message history", err);
      } finally {
        if (!cancelled) {
          setHistoryFetched((prev) => ({
            ...prev,
            [activeContact.convId]: true,
          }));
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [
    activeContact?.convId,
    client,
    historyFetched,
    localHistoryLoaded,
    syncHistoryForConversation,
  ]);

  useEffect(() => {
    console.log("Fetching conversation IDs from server");
    if (!client) return;
    let cancelled = false;

    (async () => {
      try {
        const ids = await client.fetchConversationIds();
        if (!cancelled) {
          setServerConversationIds(ids);
        }
      } catch (err) {
        console.error("Failed to fetch conversations", err);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [client]);

  useEffect(() => {
    if (!client || !localHistoryLoaded) return;
    if (serverConversationIds.length === 0) return;

    let cancelled = false;

    (async () => {
      for (const convId of serverConversationIds) {
        if (cancelled) return;
        if (historyFetched[convId]) continue;
        try {
          await syncHistoryForConversation(convId);
        } catch (err) {
          console.error("Failed to sync message history", err);
        }
        if (!cancelled) {
          setHistoryFetched((prev) => ({ ...prev, [convId]: true }));
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [
    client,
    historyFetched,
    localHistoryLoaded,
    serverConversationIds,
    syncHistoryForConversation,
  ]);

  const sendMessage = async () => {
    if (!client || !activeContact || !text.trim()) return;
    try {
      const outbound = await client.sendMessage(
        activeContact.convId,
        activeContact.deviceId,
        text.trim()
      );
      setMessages((prev) => [...prev, outbound]);
      setContacts((prev) =>
        prev.map((c) =>
          c.convId === activeContact.convId
            ? { ...c, lastActivity: new Date().toISOString() }
            : c
        )
      );
      setText("");
    } catch (err) {
      console.error("Failed to send message", err);
      setWsStatus("Send failed – please try again.");
    }
  };

  const handleRenameContact = () => {
    if (!activeContact) return;
    const next = window.prompt(
      "Set a label for this contact",
      activeContact.label || activeContact.username || ""
    );
    if (next === null) return;
    const trimmed = next.trim();
    if (!trimmed) return;
    setContacts((prev) =>
      prev.map((c) =>
        c.convId === activeContact.convId
          ? { ...c, label: trimmed }
          : c
      )
    );
  };

  const handleSend = async (e: React.FormEvent) => {
    e.preventDefault();
    await sendMessage();
  };

  const handleAddContact = async (e: React.FormEvent) => {
    e.preventDefault();
    const username = newContact.username.trim();
    if (!username) return;
    setResolveError(null);
    setResolvingContact(true);
    try {
      const resolved = await resolveContact(username);
      const convId = defaultConvId();
      const label = newContact.label.trim() || resolved.username;
      const next: Contact = {
        deviceId: resolved.deviceId,
        username: resolved.username,
        label,
        convId,
        lastActivity: new Date().toISOString(),
      };
      setContacts((prev) => [...prev, next]);
      setSelectedConvId(convId);
      setShowAddContact(false);
      setNewContact({ username: "", label: "" });
    } catch (err) {
      console.error("Failed to resolve contact", err);
      setResolveError("Could not find that user. Please check the username.");
    } finally {
      setResolvingContact(false);
    }
  };

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 px-4 py-10">
      <PinModal
        isOpen={lockState === "locked"}
        mode="unlock"
        title="Unlock your vault"
        helper="Enter your 4-digit PIN to continue."
        error={unlockError}
        waitSeconds={unlockWaitSeconds ?? undefined}
        onSubmit={async (pin) => {
          const manager = keyManagerRef.current;
          if (!manager) {
            setUnlockError("Key manager unavailable. Refresh and try again.");
            return;
          }
          setUnlockError(null);
          setUnlockWaitSeconds(null);
          try {
            await manager.unlock(pin);
            setLockState("unlocked");
          } catch (err) {
            if (err instanceof UnlockThrottledError) {
              const wait = Math.ceil(err.waitMs / 1000);
              setUnlockWaitSeconds(wait);
              return;
            }
            setUnlockError("Invalid PIN. Please try again.");
          }
        }}
      />
      <div className="max-w-6xl mx-auto space-y-6">
        <div className="flex flex-col gap-2">
          <h1 className="text-3xl font-semibold">Messages</h1>
          <p className="text-slate-400 text-sm">
            {header || "Preparing device..."}
          </p>
          <p className="text-xs text-slate-500">{wsStatus}</p>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-4 gap-4">
          <aside className="lg:col-span-1 bg-slate-900/70 border border-slate-800 rounded-2xl p-4 flex flex-col gap-3">
            <div className="flex items-center justify-between gap-2">
              <div>
                <p className="text-sm text-slate-300">Contacts</p>
                <p className="text-xs text-slate-500">Start a private chat</p>
              </div>
              <button
                type="button"
                onClick={() => setShowAddContact(true)}
                className="rounded-full bg-sky-500 text-slate-900 text-sm px-3 py-1 font-medium hover:bg-sky-400"
              >
                Add
              </button>
            </div>

            <div className="flex-1 space-y-2 overflow-y-auto pr-1 custom-scroll">
              {contacts.length === 0 && (
                <div className="text-sm text-slate-500 border border-dashed border-slate-800 rounded-lg p-3">
                  No contacts yet. Add a recipient to begin.
                </div>
              )}
              {contacts.map((contact) => {
                const lastMsg = messages
                  .filter((m) => m.convId === contact.convId)
                  .sort((a, b) => b.sentAt.getTime() - a.sentAt.getTime())[0];

                return (
                  <button
                    key={contact.convId}
                    onClick={() => setSelectedConvId(contact.convId)}
                    className={`w-full text-left rounded-xl border px-3 py-2 transition ${
                      contact.convId === selectedConvId
                        ? "border-sky-600 bg-sky-900/30"
                        : "border-slate-800 bg-slate-900/60 hover:border-slate-700"
                    }`}
                  >
                    <p className="font-medium text-slate-100">
                      {contact.username || contact.label}
                    </p>
                    {contact.lastActivity && (
                      <p className="text-[11px] text-slate-500">
                        Active {new Date(contact.lastActivity).toLocaleString()}
                      </p>
                    )}
                    {lastMsg && (
                      <p className="text-xs text-slate-500 mt-1 truncate">
                        {lastMsg.direction === "inbound" ? "Them: " : "You: "}
                        {lastMsg.plaintext}
                      </p>
                    )}
                  </button>
                );
              })}
            </div>
          </aside>

          <section className="lg:col-span-3 bg-slate-900/70 border border-slate-800 rounded-2xl min-h-[520px] flex flex-col">
            {!activeContact ? (
              <div className="flex-1 flex items-center justify-center text-slate-500 text-sm">
                Choose a contact to start chatting.
              </div>
            ) : (
              <>
                <div className="border-b border-slate-800 px-6 py-4 flex items-center justify-between">
                  <div>
                    <p className="text-lg font-semibold text-slate-100">
                      {activeContact.username || activeContact.label}
                    </p>
                    {activeContact.lastActivity && (
                      <p className="text-xs text-slate-400">
                        Active {new Date(activeContact.lastActivity).toLocaleString()}
                      </p>
                    )}
                  </div>
                  <button
                    type="button"
                    onClick={handleRenameContact}
                    className="text-xs px-3 py-1 rounded-full border border-slate-700 text-slate-200 hover:border-sky-500 hover:text-sky-300"
                  >
                    Rename
                  </button>
                </div>

                <div className="flex-1 overflow-y-auto px-6 py-4 space-y-3 custom-scroll">
                  {activeMessages.length === 0 && (
                    <div className="text-sm text-slate-500 text-center mt-6">
                      No messages yet. Your history will appear here.
                    </div>
                  )}
                  {activeMessages.map((msg, idx) => (
                    <div
                      key={`${msg.convId}-${idx}`}
                      className={`max-w-xl rounded-2xl px-4 py-3 text-sm shadow ${
                        msg.direction === "inbound"
                          ? "bg-slate-800 border border-slate-700/70"
                          : "bg-sky-600/90 text-slate-950 ml-auto"
                      }`}
                    >
                      <div className="text-[11px] text-slate-300/80 mb-1 flex items-center justify-between gap-4">
                        <span>
                          {msg.direction === "inbound" ? "Incoming" : "You"}
                        </span>
                        <span>{msg.sentAt.toLocaleTimeString()}</span>
                      </div>
                      <p className="whitespace-pre-wrap break-words text-slate-100">
                        {msg.plaintext}
                      </p>
                    </div>
                  ))}
                </div>

                <form
                  onSubmit={handleSend}
                  className="border-t border-slate-800 p-4 space-y-3"
                >
                  <label className="text-sm text-slate-300">Message</label>
                  <textarea
                    className="w-full rounded-lg bg-slate-900 border border-slate-700 px-3 py-2 text-sm text-slate-100 focus:outline-none focus:ring-2 focus:ring-sky-500 min-h-[96px]"
                    value={text}
                    onChange={(e) => setText(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" && !e.shiftKey) {
                        e.preventDefault();
                        sendMessage();
                      }
                    }}
                    placeholder="Type a secure message"
                  />
                  <div className="flex items-center justify-between">
                    <p className="text-xs text-slate-500">
                      Messages are end-to-end encrypted.
                    </p>
                    <button
                      type="submit"
                      className="px-4 py-2 rounded-lg bg-sky-500 text-slate-900 font-medium hover:bg-sky-400 disabled:opacity-60 disabled:cursor-not-allowed"
                      disabled={!text.trim()}
                    >
                      Send
                    </button>
                  </div>
                </form>
              </>
            )}
          </section>
        </div>

        {showAddContact && (
          <div className="fixed inset-0 bg-slate-950/70 backdrop-blur-sm flex items-center justify-center px-4">
            <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 w-full max-w-md shadow-xl space-y-4">
              <div className="flex items-start justify-between">
                <div>
                  <h2 className="text-xl font-semibold">Add recipient</h2>
                  <p className="text-sm text-slate-400 mt-1">
                    Enter their username to start chatting.
                  </p>
                </div>
                <button
                  className="text-slate-400 hover:text-slate-100"
                  onClick={() => setShowAddContact(false)}
                >
                  ✕
                </button>
              </div>

              <form className="space-y-3" onSubmit={handleAddContact}>
                <div className="space-y-1">
                  <label className="text-sm text-slate-300">
                    Recipient username
                  </label>
                  <input
                    className="w-full rounded-lg bg-slate-950 border border-slate-800 px-3 py-2 text-sm text-slate-100 focus:outline-none focus:ring-2 focus:ring-sky-500"
                    value={newContact.username}
                    onChange={(e) =>
                      setNewContact((prev) => ({
                        ...prev,
                        username: e.target.value,
                      }))
                    }
                    placeholder="username"
                    required
                  />
                </div>
                <div className="space-y-1">
                  <label className="text-sm text-slate-300">
                    Label (optional)
                  </label>
                  <input
                    className="w-full rounded-lg bg-slate-950 border border-slate-800 px-3 py-2 text-sm text-slate-100 focus:outline-none focus:ring-2 focus:ring-sky-500"
                    value={newContact.label}
                    onChange={(e) =>
                      setNewContact((prev) => ({
                        ...prev,
                        label: e.target.value,
                      }))
                    }
                    placeholder="Friendly name"
                  />
                </div>
                {resolveError && (
                  <div className="text-xs text-red-400 bg-red-950/40 border border-red-800/60 rounded-md px-3 py-2">
                    {resolveError}
                  </div>
                )}

                <div className="flex items-center justify-end gap-2 pt-2">
                  <button
                    type="button"
                    className="px-3 py-2 text-sm text-slate-400 hover:text-slate-100"
                    onClick={() => setShowAddContact(false)}
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    className="px-4 py-2 rounded-lg bg-sky-500 text-slate-900 font-medium hover:bg-sky-400 disabled:opacity-60 disabled:cursor-not-allowed"
                    disabled={!newContact.username.trim() || resolvingContact}
                  >
                    {resolvingContact ? "Resolving..." : "Save & open chat"}
                  </button>
                </div>
              </form>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

async function ensureUnlocked(manager: KeyManager): Promise<void> {
  if (!(await manager.hasWrappedKey())) {
    const setupPin = window.prompt("Set a 4-digit PIN to secure this device.");
    if (!setupPin) {
      throw new Error("PIN setup cancelled");
    }
    await manager.setupPin(setupPin);
    return;
  }

  while (true) {
    const pin = window.prompt("Enter your 4-digit PIN to unlock.");
    if (!pin) {
      throw new Error("PIN entry cancelled");
    }
    try {
      await manager.unlock(pin);
      return;
    } catch (err) {
      if (err instanceof UnlockThrottledError) {
        const waitSeconds = Math.ceil(err.waitMs / 1000);
        window.alert(`Too many attempts. Try again in ${waitSeconds}s.`);
        await sleep(err.waitMs);
        continue;
      }
      window.alert("Invalid PIN. Please try again.");
    }
  }
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}
