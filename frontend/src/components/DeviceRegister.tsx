import React, { useCallback, useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import registerDevice, {
  DeviceRegistrationResult,
} from "../services/registerDevice";
import { InboundMessage } from "../lib/messagingClient";
import { setItem } from "../lib/storage";
import { getItem } from "../lib/storage";
import { getKeyManager } from "../lib/keyManagerInstance";
import { UnlockThrottledError } from "../crypto-core/keyManager";
import { PinModal } from "./PinModal";

export type DeviceRegisterFormValues = {
  name: string;
};

type DeviceRegisterFormProps = {
  onSubmit: (values: DeviceRegisterFormValues) => Promise<void> | void;
  isLoading?: boolean;
};

export const DeviceRegisterForm: React.FC<DeviceRegisterFormProps> = ({
  onSubmit,
  isLoading = false,
}) => {
  const [values, setValues] = useState<DeviceRegisterFormValues>({
    name: "",
  });
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [status, setStatus] = useState<string | null>(null);
  const [messages, setMessages] = useState<InboundMessage[]>([]);
  const [lockState, setLockState] = useState<"checking" | "locked" | "unlocked">(
    "checking"
  );
  const [unlockError, setUnlockError] = useState<string | null>(null);
  const [unlockWaitSeconds, setUnlockWaitSeconds] = useState<number | null>(null);
  const keyManagerRef = useRef<ReturnType<typeof getKeyManager> | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const navigate = useNavigate();

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>): void => {
    const { value } = e.target;
    setValues({ name: value });
  };

  const handleLock = useCallback(() => {
    setLockState("locked");
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!values.name.trim()) return;
    if (lockState !== "unlocked") {
      setUnlockError("Unlock required to register this device.");
      setLockState("locked");
      return;
    }
    setIsSubmitting(true);
    setStatus(null);
    try {
      const { device, client }: DeviceRegistrationResult = await registerDevice(
        values.name,
      );

      await setItem("deviceId", device.deviceId);
      await setItem("deviceName", device.name);
      await setItem("devicePlatform", device.platform);

      const ws = await client.connectWebSocket((msg) => {
        setMessages((prev) => [...prev, msg]);
      }, (state) => {
        if (state === "open") {
          setStatus("WebSocket connected. Listening for messages...");
        } else if (state === "error") {
          setStatus("WebSocket error. Retrying might be necessary.");
        }
      });
      wsRef.current = ws;

      setStatus("Device registered. Syncing messages...");
      setTimeout(() => navigate("/messages"), 350);
    } catch (err) {
      console.error("Error registering device:", err);
      setStatus("Failed to register device. Please try again.");
    } finally {
      setIsSubmitting(false);
    }
  };

  useEffect(() => {
    return () => {
      wsRef.current?.close();
    };
  }, []);

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
    let cancelled = false;
    (async () => {
      const userId = await getItem("userId");
      if (!userId) {
        navigate("/");
        return;
      }
      const manager = getKeyManager(userId, { onLock: handleLock });
      keyManagerRef.current = manager;
      const hasWrapped = await manager.hasWrappedKey();
      if (!hasWrapped) {
        setUnlockError("Set up a PIN to protect your device.");
        setLockState("locked");
        return;
      }
      if (!cancelled) {
        setLockState(manager.isUnlocked() ? "unlocked" : "locked");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [handleLock, navigate]);

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex items-center justify-center px-4">
      <PinModal
        isOpen={lockState === "locked"}
        mode="unlock"
        title="Unlock to continue"
        helper="Enter your 4-digit PIN to register this device."
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
      <div className="w-full max-w-md">
        <div className="bg-slate-900/70 border border-slate-800 rounded-2xl shadow-xl p-8 backdrop-blur">
          <div className="mb-6">
            <h1 className="text-2xl font-semibold">Register a new device</h1>
            <p className="text-sm text-slate-400 mt-1">
              Give this device a name so you can recognize it in your account
              and manage its keys.
            </p>
          </div>

          <form className="space-y-4" onSubmit={handleSubmit}>
            <div className="space-y-1">
              <label
                htmlFor="deviceName"
                className="block text-sm font-medium text-slate-200"
              >
                Device name
              </label>
              <input
                id="deviceName"
                name="deviceName"
                type="text"
                required
                value={values.name}
                onChange={handleChange}
                className="w-full rounded-lg bg-slate-900 border border-slate-700 px-3 py-2 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-sky-500 focus:border-sky-500"
                placeholder="Ivan’s laptop, Pixel phone, etc."
              />
            </div>

            <button
              type="submit"
              disabled={isLoading || isSubmitting || !values.name.trim()}
              className="w-full rounded-lg bg-sky-500 hover:bg-sky-400 disabled:opacity-60 disabled:cursor-not-allowed transition px-3 py-2 text-sm font-medium text-slate-900"
            >
              {isSubmitting || isLoading
                ? "Registering device..."
                : "Register device"}
            </button>
            {status && (
              <p className="text-xs text-slate-400">{status}</p>
            )}
          </form>
        </div>

        <div className="mt-4 text-center text-xs text-slate-500 space-y-2">
          <p>A unique keypair will be generated locally for this device. 🔐</p>
          {messages.length > 0 && (
            <div className="rounded-lg bg-slate-900/70 border border-slate-800 px-3 py-2 text-left">
              <p className="text-slate-300 font-semibold mb-1">Recent messages</p>
              <ul className="space-y-1 text-left">
                {messages.slice(-3).map((m, idx) => (
                  <li key={`${m.convId}-${idx}`} className="text-xs text-slate-400">
                    <span className="text-slate-200">{m.peerDeviceId}</span>: {m.plaintext}
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
