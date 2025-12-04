import { FormEvent, useState } from "react";
import {
  useMessagingClient,
  RegistrationForm,
  SendForm,
  MessageRecord,
} from "../hooks/useMessagingClient";

function App() {
  const {
    ready,
    state,
    info,
    engineError,
    messages,
    listener,
    register,
    reset,
    sendMessage,
    connect,
    disconnect,
  } = useMessagingClient();
  const [registerForm, setRegisterForm] = useState<RegistrationForm>({
    keysUrl: "http://localhost:8080",
    messagesUrl: "http://localhost:8080",
  });
  const [sendForm, setSendForm] = useState<SendForm>({
    convId: "",
    toDeviceId: "",
    message: "",
  });
  const [feedback, setFeedback] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [sending, setSending] = useState(false);
  const [registering, setRegistering] = useState(false);

  const handleRegister = async (evt: FormEvent) => {
    evt.preventDefault();
    setError(null);
    setFeedback(null);
    try {
      setRegistering(true);
      const result = await register(registerForm);
      setFeedback(
        `Device registered. User ${result.userId}, device ${result.deviceId}.`
        //Remaining one-time pre-keys: ${result.oneTimePrekeys}.
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setRegistering(false);
    }
  };

  const handleSend = async (evt: FormEvent) => {
    evt.preventDefault();
    setError(null);
    setFeedback(null);
    try {
      setSending(true);
      await sendMessage(sendForm);
      setFeedback("Message queued successfully.");
      setSendForm((prev) => ({ ...prev, message: "" }));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSending(false);
    }
  };

  const handleConnectToggle = () => {
    if (listener.status === "connected" || listener.status === "connecting") {
      disconnect();
    } else {
      try {
        connect();
      } catch (err) {
        setError(err instanceof Error ? err.message : String(err));
      }
    }
  };

  const canSend = Boolean(state && ready && info);

  return (
    <div className="app">
      <header className="header">
        <h1>SecuMSG Messenger</h1>
      </header>

      <main className="content">
        {!ready && !engineError && (
          <p className="muted">Loading encryption engine…</p>
        )}
        {engineError && (
          <p className="error">
            Failed to load encryption engine: {engineError}
          </p>
        )}
        <section className="card">
          <h2>Device Registration</h2>
          <form onSubmit={handleRegister} className="form">
            <label>
              Keys base URL
              <input
                type="url"
                value={registerForm.keysUrl}
                onChange={(evt) =>
                  setRegisterForm((prev) => ({
                    ...prev,
                    keysUrl: evt.target.value,
                  }))
                }
                required
              />
            </label>
            <label>
              Messages base URL
              <input
                type="url"
                value={registerForm.messagesUrl}
                onChange={(evt) =>
                  setRegisterForm((prev) => ({
                    ...prev,
                    messagesUrl: evt.target.value,
                  }))
                }
                required
              />
            </label>
            <div className="form-row">
              <label>
                User ID (optional)
                <input
                  value={registerForm.userId ?? ""}
                  onChange={(evt) =>
                    setRegisterForm((prev) => ({
                      ...prev,
                      userId: evt.target.value,
                    }))
                  }
                />
              </label>
              <label>
                Device ID (optional)
                <input
                  value={registerForm.deviceId ?? ""}
                  onChange={(evt) =>
                    setRegisterForm((prev) => ({
                      ...prev,
                      deviceId: evt.target.value,
                    }))
                  }
                />
              </label>
            </div>
            <div className="actions">
              <button type="submit" disabled={!ready || registering}>
                {registering ? "Registering…" : "Register device"}
              </button>
              <button type="button" onClick={reset} disabled={!state}>
                Reset state
              </button>
            </div>
          </form>
          {info && (
            <div className="state-info">
              <h3>Current device</h3>
              <dl>
                <div>
                  <dt>User ID</dt>
                  <dd>{info.userId}</dd>
                </div>
                <div>
                  <dt>Device ID</dt>
                  <dd>{info.deviceId}</dd>
                </div>
                <div>
                  <dt>Keys URL</dt>
                  <dd>{info.keysUrl}</dd>
                </div>
                <div>
                  <dt>Messages URL</dt>
                  <dd>{info.messagesUrl}</dd>
                </div>
              </dl>
            </div>
          )}
        </section>

        <section className="card">
          <h2>Messaging</h2>
          <div className="actions">
            <button
              type="button"
              onClick={handleConnectToggle}
              disabled={!state || !ready}
            >
              {listener.status === "connected"
                ? "Disconnect"
                : listener.status === "connecting"
                ? "Connecting…"
                : "Connect & listen"}
            </button>
            {listener.error && <span className="error">{listener.error}</span>}
          </div>
          <form onSubmit={handleSend} className="form">
            <label>
              Conversation ID
              <input
                value={sendForm.convId}
                onChange={(evt) =>
                  setSendForm((prev) => ({ ...prev, convId: evt.target.value }))
                }
                placeholder="Conversation UUID"
                required
                disabled={!canSend}
              />
            </label>
            <label>
              Recipient device ID
              <input
                value={sendForm.toDeviceId}
                onChange={(evt) =>
                  setSendForm((prev) => ({
                    ...prev,
                    toDeviceId: evt.target.value,
                  }))
                }
                placeholder="Device UUID"
                required
                disabled={!canSend}
              />
            </label>
            <label>
              Message
              <textarea
                value={sendForm.message}
                onChange={(evt) =>
                  setSendForm((prev) => ({
                    ...prev,
                    message: evt.target.value,
                  }))
                }
                rows={4}
                required
                disabled={!canSend}
              />
            </label>
            <div className="actions">
              <button type="submit" disabled={!canSend || sending}>
                {sending ? "Sending…" : "Send message"}
              </button>
            </div>
          </form>
        </section>

        <section className="card">
          <h2>Inbox</h2>
          {messages.length === 0 ? (
            <p className="empty">No messages yet.</p>
          ) : (
            <table className="message-table">
              <thead>
                <tr>
                  <th>Sent at</th>
                  <th>Conversation</th>
                  <th>From</th>
                  <th>Message</th>
                </tr>
              </thead>
              <tbody>
                {messages.map((msg) => (
                  <MessageRow key={msg.id} record={msg} />
                ))}
              </tbody>
            </table>
          )}
        </section>
      </main>

      <footer className="footer">
        {feedback && <p className="feedback">{feedback}</p>}
        {error && <p className="error">{error}</p>}
      </footer>
    </div>
  );
}

function MessageRow({ record }: { record: MessageRecord }) {
  return (
    <tr>
      <td>{new Date(record.sentAt).toLocaleString()}</td>
      <td>{record.convId}</td>
      <td>{record.fromDeviceId}</td>
      <td className="message-text">{record.plaintext}</td>
    </tr>
  );
}

export default App;
