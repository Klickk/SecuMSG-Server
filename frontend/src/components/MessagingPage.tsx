import React, { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  InboundMessage,
  MessagingClient,
  OutboundMessage,
} from "../lib/messagingClient";

type Contact = {
  deviceId: string;
  label: string;
  convId: string;
  lastActivity?: string;
};

const CONTACTS_KEY = "secumsg-contacts";

const defaultConvId = () => crypto.randomUUID();

const loadContacts = (): Contact[] => {
  try {
    const stored = localStorage.getItem(CONTACTS_KEY);
    if (!stored) return [];
    return JSON.parse(stored) as Contact[];
  } catch (err) {
    console.error("Failed to load contacts", err);
    return [];
  }
};

const saveContacts = (contacts: Contact[]) => {
  localStorage.setItem(CONTACTS_KEY, JSON.stringify(contacts));
};

export const MessagingPage: React.FC = () => {
  const [client, setClient] = useState<MessagingClient | null>(null);
  const [contacts, setContacts] = useState<Contact[]>(loadContacts);
  const [selectedConvId, setSelectedConvId] = useState<string | null>(
    () => loadContacts()[0]?.convId ?? null
  );
  const [text, setText] = useState<string>("");
  const [messages, setMessages] = useState<(InboundMessage | OutboundMessage)[]>([]);
  const [wsStatus, setWsStatus] = useState<string>("Connecting to message stream...");
  const [showAddContact, setShowAddContact] = useState(false);
  const [newContact, setNewContact] = useState({ deviceId: "", label: "" });
  const navigate = useNavigate();

  useEffect(() => {
    const loaded = MessagingClient.load();
    if (!loaded) {
      navigate("/dRegister");
      return;
    }
    setClient(loaded);

    const socket = loaded.connectWebSocket((msg) => {
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

        const byDeviceIdx = prev.findIndex((c) => c.deviceId === msg.peerDeviceId);
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
            label: `Device ${msg.peerDeviceId.slice(0, 6)}`,
            convId: msg.convId,
            lastActivity: msg.sentAt.toISOString(),
          },
        ];
      });
      setSelectedConvId((prev) => prev ?? msg.convId);
    }, (state) => {
      if (state === "open") setWsStatus("Listening for incoming messages");
      if (state === "closed") setWsStatus("Connection closed");
      if (state === "error") setWsStatus("Connection error – retry or refresh");
    });
    return () => {
      socket.close();
    };
  }, [navigate]);

  useEffect(() => {
    saveContacts(contacts);
  }, [contacts]);

  const header = useMemo(() => {
    if (!client) return "";
    return `Device ${client.deviceId()} · User ${client.userId()}`;
  }, [client]);

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

  const handleSend = async (e: React.FormEvent) => {
    e.preventDefault();
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
      setWsStatus("Send failed – check device IDs and try again.");
    }
  };

  const handleAddContact = (e: React.FormEvent) => {
    e.preventDefault();
    if (!newContact.deviceId.trim()) return;
    const convId = defaultConvId();
    const label = newContact.label.trim() || `Device ${newContact.deviceId.slice(0, 6)}`;
    const next: Contact = {
      deviceId: newContact.deviceId.trim(),
      label,
      convId,
      lastActivity: new Date().toISOString(),
    };
    setContacts((prev) => [...prev, next]);
    setSelectedConvId(convId);
    setShowAddContact(false);
    setNewContact({ deviceId: "", label: "" });
  };

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 px-4 py-10">
      <div className="max-w-6xl mx-auto space-y-6">
        <div className="flex flex-col gap-2">
          <h1 className="text-3xl font-semibold">Messages</h1>
          <p className="text-slate-400 text-sm">{header || "Preparing device..."}</p>
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
                    <p className="font-medium text-slate-100">{contact.label}</p>
                    <p className="text-xs text-slate-400">{contact.deviceId}</p>
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
                    <p className="text-lg font-semibold text-slate-100">{activeContact.label}</p>
                    <p className="text-xs text-slate-400">Device {activeContact.deviceId}</p>
                  </div>
                  <div className="text-xs text-slate-500">
                    Conversation ID
                    <span className="ml-2 text-slate-300">{activeContact.convId}</span>
                  </div>
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
                        <span>{msg.direction === "inbound" ? "Incoming" : "You"}</span>
                        <span>{msg.sentAt.toLocaleTimeString()}</span>
                      </div>
                      <p className="whitespace-pre-wrap break-words text-slate-100">{msg.plaintext}</p>
                    </div>
                  ))}
                </div>

                <form onSubmit={handleSend} className="border-t border-slate-800 p-4 space-y-3">
                  <label className="text-sm text-slate-300">Message</label>
                  <textarea
                    className="w-full rounded-lg bg-slate-900 border border-slate-700 px-3 py-2 text-sm text-slate-100 focus:outline-none focus:ring-2 focus:ring-sky-500 min-h-[96px]"
                    value={text}
                    onChange={(e) => setText(e.target.value)}
                    placeholder="Type a secure message"
                  />
                  <div className="flex items-center justify-between">
                    <p className="text-xs text-slate-500">Messages are end-to-end encrypted.</p>
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
                  <p className="text-sm text-slate-400 mt-1">Enter their device ID to start chatting.</p>
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
                  <label className="text-sm text-slate-300">Recipient device ID</label>
                  <input
                    className="w-full rounded-lg bg-slate-950 border border-slate-800 px-3 py-2 text-sm text-slate-100 focus:outline-none focus:ring-2 focus:ring-sky-500"
                    value={newContact.deviceId}
                    onChange={(e) => setNewContact((prev) => ({ ...prev, deviceId: e.target.value }))}
                    placeholder="Paste the device ID"
                    required
                  />
                </div>
                <div className="space-y-1">
                  <label className="text-sm text-slate-300">Label (optional)</label>
                  <input
                    className="w-full rounded-lg bg-slate-950 border border-slate-800 px-3 py-2 text-sm text-slate-100 focus:outline-none focus:ring-2 focus:ring-sky-500"
                    value={newContact.label}
                    onChange={(e) => setNewContact((prev) => ({ ...prev, label: e.target.value }))}
                    placeholder="Alice, Work laptop, ..."
                  />
                </div>

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
                    disabled={!newContact.deviceId.trim()}
                  >
                    Save & open chat
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
