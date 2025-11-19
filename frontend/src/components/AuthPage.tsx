import React, { useState } from "react";
import { LoginForm } from "./LoginForm";
import { RegisterForm } from "./RegisterForm";
import { Register } from "../services/register";
import { Login } from "../services/login";
import { useNavigate } from "react-router-dom";
import { RegisterResponse } from "../types/types";

type AuthMode = "login" | "register";

export const AuthPage: React.FC = () => {
  const [mode, setMode] = useState<AuthMode>("login");
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const navigate = useNavigate();

  const handleLogin = async (values: { email: string; password: string }) => {
    setIsLoading(true);
    setError(null);
    try {
      Login(values.email, values.password).then((tokenResponse) => {
        localStorage.setItem("accessToken", tokenResponse.accessToken);
        localStorage.setItem("refreshToken", tokenResponse.refreshToken);
        console.log("Received tokens:", tokenResponse);
      });
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
      Register(values.name, values.email, values.password).then((success) => {
        if (!success) {
          setError("Registration failed. Please try again.");
        } else {
          const resp: RegisterResponse = success as RegisterResponse;
          localStorage.setItem("userId", resp.userId);
          navigate("/dRegister");
          console.log("Registration successful");
        }
      });
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
