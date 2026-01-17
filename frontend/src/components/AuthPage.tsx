import React, { useEffect, useRef, useState } from "react";
import { LoginForm } from "./LoginForm";
import { RegisterForm } from "./RegisterForm";
import { Register } from "../services/register";
import { Login } from "../services/login";
import { useNavigate } from "react-router-dom";
import { RegisterResponse } from "../types/types";
import { setItem, wipeDatabaseIfExists } from "../lib/storage";
import { verifyAccessToken } from "../services/verify";
import { MessagingClient, SECURE_STORE, STORAGE_KEY } from "../lib/messagingClient";
import { hasSecureRecord } from "../lib/secureStore";
import { getKeyManager } from "../lib/keyManagerInstance";
import { PinModal } from "./PinModal";
import {
  getApiBaseUrl,
  getServiceHost,
  setServiceHost,
} from "../config/config";

type AuthMode = "login" | "register";

export const AuthPage: React.FC = () => {
  const [mode, setMode] = useState<AuthMode>("login");
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [serviceHost, setServiceHostInput] = useState(getServiceHost());
  const [pinPromptOpen, setPinPromptOpen] = useState(false);
  const [pinPromptError, setPinPromptError] = useState<string | null>(null);
  const pinPromptRef = useRef<{
    resolve: (pin: string) => void;
    reject: (err: Error) => void;
  } | null>(null);
  const navigate = useNavigate();

  useEffect(() => {
    setServiceHost(serviceHost);
  }, [serviceHost]);

  const loadValidMessagingState = async (
    expectedUserId: string
  ): Promise<MessagingClient | null> => {
    try {
      const state = await MessagingClient.load();
      if (!state) return null;
      if (state.userId() !== expectedUserId) {
        return null;
      }
      return state;
    } catch (err) {
      console.warn("Failed to load messaging state", err);
      return null;
    }
  };

  const requestPinPrompt = (): Promise<string> => {
    return new Promise((resolve, reject) => {
      pinPromptRef.current = { resolve, reject };
      setPinPromptError(null);
      setPinPromptOpen(true);
    });
  };

  const handleLogin = async (values: { email: string; password: string }) => {
    setIsLoading(true);
    setError(null);
    try {
      // Drop any stale local state before setting fresh credentials.
      await wipeDatabaseIfExists();
      const tokenResponse = await Login(values.email, values.password);
      await setItem("accessToken", tokenResponse.accessToken);
      await setItem("refreshToken", tokenResponse.refreshToken);
      const verification = await verifyAccessToken();
      if (!verification.valid || !verification.userId) {
        throw new Error("Token verification failed");
      }
      await setItem("userId", verification.userId);
      await setItem("username", values.email);
      await ensurePinSetup(verification.userId, requestPinPrompt);
      const state = await loadValidMessagingState(verification.userId);
      if (state) {
        await setItem("deviceId", state.deviceId());
        navigate("/messages");
      } else if (await hasSecureRecord(SECURE_STORE, STORAGE_KEY)) {
        navigate("/messages");
      } else {
        // No valid device state found → send user to device registration flow.
        navigate("/dRegister");
      }
      console.log("Received tokens and resolved device state");
    } catch (err) {
      if (err instanceof Error && err.message.includes("PIN")) {
        setError(err.message);
      } else {
        setError("Failed to sign in. Please try again.");
      }
    } finally {
      setIsLoading(false);
    }
  };
  const handleRegister = async (values: {
    name: string;
    email: string;
    password: string;
    pin: string;
  }) => {
    setIsLoading(true);
    setError(null);
    try {
      const success = await Register(
        values.name,
        values.email,
        values.password
      );
      if (!success) {
        setError("Registration failed. Please try again.");
      } else {
        const resp: RegisterResponse = success as RegisterResponse;
        await wipeDatabaseIfExists();
        await setItem("username", values.name);
        await setItem("accessToken", resp.accessToken);
        await setItem("refreshToken", resp.refreshToken);
        await setItem("userId", resp.userId);
        const manager = getKeyManager(resp.userId);
        await withTimeout(
          manager.setupPin(values.pin),
          8000,
          "PIN setup timed out. Please try again."
        );
        // After account creation, always go to device registration to provision the device.
        navigate("/dRegister");
        console.log("Registration successful");
      }
      console.log("register submit", values);
    } catch (err) {
      if (err instanceof Error && err.message.includes("PIN")) {
        setError(err.message);
      } else {
        setError("Failed to create account. Please try again.");
      }
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex items-center justify-center px-4">
      <div className="w-full max-w-md">
        <div className="mb-4 bg-slate-900/70 border border-slate-800 rounded-2xl p-4 backdrop-blur">
          <div className="flex items-center justify-between gap-3">
            <div>
              <p className="text-sm text-slate-300">Service address</p>
              <p className="text-xs text-slate-500">
                All services run on this host (ports unchanged).
              </p>
            </div>
            <div className="flex flex-col items-end gap-2 w-40">
              <input
                className="w-full rounded-lg bg-slate-950 border border-slate-800 px-3 py-2 text-sm text-slate-100 focus:outline-none focus:ring-2 focus:ring-sky-500"
                value={serviceHost}
                onChange={(e) => setServiceHostInput(e.target.value)}
                placeholder="e.g. 192.168.1.50"
              />
              <span className="text-[11px] text-slate-500">
                API: {getApiBaseUrl()}
              </span>
            </div>
          </div>
        </div>

        <div className="bg-slate-900/70 border border-slate-800 rounded-2xl shadow-xl p-8 backdrop-blur">
          <div className="flex justify-between items-center mb-6">
            <div>
              <h1 className="text-2xl font-semibold">
                {mode === "login" ? "Welcome back" : "Create your account"}
              </h1>
              <p className="text-sm text-slate-400 mt-1">
                {mode === "login"
                  ? "Sign in to access your encrypted messages."
                  : "Register to start using the secure messaging platform."}
              </p>
            </div>

            <div className="flex gap-1 bg-slate-800/80 rounded-full p-1">
              <button
                type="button"
                onClick={() => setMode("login")}
                className={`px-3 py-1 text-xs rounded-full transition ${
                  mode === "login"
                    ? "bg-slate-100 text-slate-900"
                    : "text-slate-400 hover:text-slate-100"
                }`}
              >
                Login
              </button>
              <button
                type="button"
                onClick={() => setMode("register")}
                className={`px-3 py-1 text-xs rounded-full transition ${
                  mode === "register"
                    ? "bg-slate-100 text-slate-900"
                    : "text-slate-400 hover:text-slate-100"
                }`}
              >
                Register
              </button>
            </div>
          </div>

          {error && (
            <div className="mb-4 text-sm text-red-400 bg-red-950/40 border border-red-800/60 rounded-md px-3 py-2">
              {error}
            </div>
          )}

          {mode === "login" ? (
            <LoginForm onSubmit={handleLogin} isLoading={isLoading} />
          ) : (
            <RegisterForm onSubmit={handleRegister} isLoading={isLoading} />
          )}
        </div>

        <p className="mt-4 text-center text-xs text-slate-500">
          End-to-end encryption enabled by design. 💬🔐
        </p>
      </div>

      <PinModal
        isOpen={pinPromptOpen}
        mode="setup"
        title="Set your 4-digit PIN"
        helper="This PIN protects your device encryption key."
        error={pinPromptError}
        onCancel={() => {
          pinPromptRef.current?.reject(new Error("PIN setup is required to continue."));
          pinPromptRef.current = null;
          setPinPromptOpen(false);
        }}
        onSubmit={async (pin) => {
          pinPromptRef.current?.resolve(pin);
          pinPromptRef.current = null;
          setPinPromptOpen(false);
        }}
      />
    </div>
  );
};

async function ensurePinSetup(
  userId: string,
  requestPinPrompt: () => Promise<string>
): Promise<void> {
  const manager = getKeyManager(userId);
  if (await manager.hasWrappedKey()) {
    return;
  }
  const pin = await requestPinPrompt();
  await withTimeout(manager.setupPin(pin), 8000, "PIN setup timed out. Please try again.");
}

async function withTimeout<T>(
  task: Promise<T>,
  ms: number,
  message: string
): Promise<T> {
  let timer: number | undefined;
  const timeout = new Promise<T>((_, reject) => {
    timer = window.setTimeout(() => {
      reject(new Error(message));
    }, ms);
  });
  try {
    return await Promise.race([task, timeout]);
  } finally {
    if (typeof timer === "number") {
      window.clearTimeout(timer);
    }
  }
}
