import React, { useEffect, useState } from "react";
import { LoginForm } from "./LoginForm";
import { RegisterForm } from "./RegisterForm";
import { Register } from "../services/register";
import { Login } from "../services/login";
import { useNavigate } from "react-router-dom";
import { RegisterResponse } from "../types/types";
import { setItem, wipeDatabaseIfExists } from "../lib/storage";
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
  const navigate = useNavigate();

  useEffect(() => {
    setServiceHost(serviceHost);
  }, [serviceHost]);

  const handleLogin = async (values: { email: string; password: string }) => {
    setIsLoading(true);
    setError(null);
    try {
      const tokenResponse = await Login(values.email, values.password);
      await setItem("accessToken", tokenResponse.accessToken);
      await setItem("refreshToken", tokenResponse.refreshToken);
      console.log("Received tokens:", tokenResponse);
    } catch (err) {
      setError("Failed to sign in. Please try again.");
    } finally {
      setIsLoading(false);
    }
  };
  const handleRegister = async (values: {
    name: string;
    email: string;
    password: string;
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
        await setItem("userId", resp.userId);
        navigate("/dRegister");
        console.log("Registration successful");
      }
      console.log("register submit", values);
    } catch (err) {
      setError("Failed to create account. Please try again.");
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
    </div>
  );
};
