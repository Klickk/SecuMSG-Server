import React, { useState } from "react";

export type LoginFormValues = {
  email: string;
  password: string;
};

type LoginFormProps = {
  onSubmit: (values: LoginFormValues) => Promise<void> | void;
  isLoading?: boolean;
};

export const LoginForm: React.FC<LoginFormProps> = ({
  onSubmit,
  isLoading = false,
}) => {
  const [values, setValues] = useState<LoginFormValues>({
    email: "",
    password: "",
  });

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>): void => {
    const { name, value } = e.target;
    setValues((prev) => ({ ...prev, [name]: value }));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    await onSubmit(values);
  };

  return (
    <form className="space-y-4" onSubmit={handleSubmit}>
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
          autoComplete="current-password"
          value={values.password}
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
        {isLoading ? "Signing in..." : "Sign in"}
      </button>
    </form>
  );
};
