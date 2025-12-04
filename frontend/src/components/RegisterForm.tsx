import React, { useState } from "react";

export type RegisterFormValues = {
  name: string;
  email: string;
  password: string;
};

type RegisterFormProps = {
  onSubmit: (values: RegisterFormValues) => Promise<void> | void;
  isLoading?: boolean;
};

export const RegisterForm: React.FC<RegisterFormProps> = ({
  onSubmit,
  isLoading = false,
}) => {
  const [values, setValues] = useState<RegisterFormValues>({
    name: "",
    email: "",
    password: "",
  });
  const [confirmPassword, setConfirmPassword] = useState("");
  const [localError, setLocalError] = useState<string | null>(null);

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>): void => {
    const { name, value } = e.target;
    if (name === "confirmPassword") {
      setConfirmPassword(value);
    } else {
      setValues((prev) => ({ ...prev, [name]: value }));
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (values.password !== confirmPassword) {
      setLocalError("Passwords do not match.");
      return;
    }
    setLocalError(null);
    await onSubmit(values);
  };

  return (
    <form className="space-y-4" onSubmit={handleSubmit}>
      {localError && (
        <div className="text-xs text-red-400 bg-red-950/40 border border-red-800/60 rounded-md px-3 py-2">
          {localError}
        </div>
      )}

      <div className="space-y-1">
        <label
          htmlFor="name"
          className="block text-sm font-medium text-slate-200"
        >
          Username
        </label>
        <input
          id="name"
          name="name"
          type="text"
          required
          value={values.name}
          onChange={handleChange}
          className="w-full rounded-lg bg-slate-900 border border-slate-700 px-3 py-2 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-sky-500 focus:border-sky-500"
          placeholder="Username"
        />
      </div>

      <div className="space-y-1">
        <label
          htmlFor="email"
          className="block text-sm font-medium text-slate-200"
        >
          Email
        </label>
        <input
          id="email"
          name="email"
          type="email"
          required
          autoComplete="email"
          value={values.email}
          onChange={handleChange}
          className="w-full rounded-lg bg-slate-900 border border-slate-700 px-3 py-2 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-sky-500 focus:border-sky-500"
          placeholder="you@example.com"
        />
      </div>

      <div className="space-y-1">
        <label
          htmlFor="password"
          className="block text-sm font-medium text-slate-200"
        >
          Password
        </label>
        <input
          id="password"
          name="password"
          type="password"
          required
          autoComplete="new-password"
          value={values.password}
          onChange={handleChange}
          className="w-full rounded-lg bg-slate-900 border border-slate-700 px-3 py-2 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-sky-500 focus:border-sky-500"
          placeholder="••••••••"
        />
      </div>

      <div className="space-y-1">
        <label
          htmlFor="confirmPassword"
          className="block text-sm font-medium text-slate-200"
        >
          Confirm password
        </label>
        <input
          id="confirmPassword"
          name="confirmPassword"
          type="password"
          required
          autoComplete="new-password"
          value={confirmPassword}
          onChange={handleChange}
          className="w-full rounded-lg bg-slate-900 border border-slate-700 px-3 py-2 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-sky-500 focus:border-sky-500"
          placeholder="••••••••"
        />
      </div>

      <button
        type="submit"
        disabled={isLoading}
        className="w-full rounded-lg bg-sky-500 hover:bg-sky-400 disabled:opacity-60 disabled:cursor-not-allowed transition px-3 py-2 text-sm font-medium text-slate-900"
      >
        {isLoading ? "Creating account..." : "Create account"}
      </button>
    </form>
  );
};
