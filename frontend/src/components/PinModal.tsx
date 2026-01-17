import React, { useMemo, useState } from "react";

type PinMode = "setup" | "unlock";

type PinModalProps = {
  isOpen: boolean;
  mode: PinMode;
  title?: string;
  helper?: string;
  error?: string | null;
  waitSeconds?: number | null;
  onSubmit: (pin: string) => Promise<void> | void;
  onCancel?: () => void;
};

export const PinModal: React.FC<PinModalProps> = ({
  isOpen,
  mode,
  title,
  helper,
  error,
  waitSeconds,
  onSubmit,
  onCancel,
}) => {
  const [pin, setPin] = useState("");
  const [confirm, setConfirm] = useState("");
  const [localError, setLocalError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const isSetup = mode === "setup";
  const label = title ?? (isSetup ? "Create your PIN" : "Unlock with PIN");
  const hint =
    helper ??
    (isSetup
      ? "Choose a 4-digit PIN to protect your device keys."
      : "Enter your 4-digit PIN to access encrypted data.");
  const canSubmit = useMemo(() => {
    if (submitting) return false;
    if (waitSeconds && waitSeconds > 0) return false;
    if (!/^\d{4}$/.test(pin)) return false;
    if (isSetup && pin !== confirm) return false;
    return true;
  }, [confirm, isSetup, pin, submitting, waitSeconds]);

  if (!isOpen) {
    return null;
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!/^\d{4}$/.test(pin)) {
      setLocalError("PIN must be exactly 4 digits.");
      return;
    }
    if (isSetup && pin !== confirm) {
      setLocalError("PINs do not match.");
      return;
    }
    setLocalError(null);
    try {
      setSubmitting(true);
      await onSubmit(pin);
      setPin("");
      setConfirm("");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/80 px-4">
      <div className="w-full max-w-sm rounded-2xl border border-slate-800 bg-slate-900/90 p-6 shadow-2xl backdrop-blur">
        <div className="space-y-2">
          <h2 className="text-xl font-semibold text-slate-100">{label}</h2>
          <p className="text-sm text-slate-400">{hint}</p>
        </div>

        {(error || localError) && (
          <div className="mt-4 text-xs text-red-300 bg-red-950/40 border border-red-800/60 rounded-md px-3 py-2">
            {localError || error}
          </div>
        )}

        {waitSeconds && waitSeconds > 0 && (
          <div className="mt-4 text-xs text-amber-200 bg-amber-900/30 border border-amber-700/60 rounded-md px-3 py-2">
            Too many attempts. Try again in {waitSeconds}s.
          </div>
        )}

        <form className="mt-4 space-y-3" onSubmit={handleSubmit}>
          <div className="space-y-1">
            <label
              htmlFor="pin"
              className="block text-sm font-medium text-slate-200"
            >
              4-digit PIN
            </label>
            <input
              id="pin"
              name="pin"
              type="password"
              inputMode="numeric"
              pattern="[0-9]{4}"
              maxLength={4}
              autoComplete="off"
              value={pin}
              onChange={(e) => setPin(e.target.value)}
              className="w-full rounded-lg bg-slate-950 border border-slate-700 px-3 py-2 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-sky-500 focus:border-sky-500"
              placeholder="••••"
            />
          </div>

          {isSetup && (
            <div className="space-y-1">
              <label
                htmlFor="confirmPin"
                className="block text-sm font-medium text-slate-200"
              >
                Confirm PIN
              </label>
              <input
                id="confirmPin"
                name="confirmPin"
                type="password"
                inputMode="numeric"
                pattern="[0-9]{4}"
                maxLength={4}
                autoComplete="off"
                value={confirm}
                onChange={(e) => setConfirm(e.target.value)}
                className="w-full rounded-lg bg-slate-950 border border-slate-700 px-3 py-2 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-sky-500 focus:border-sky-500"
                placeholder="••••"
              />
            </div>
          )}

          <div className="flex items-center justify-end gap-2 pt-2">
            {onCancel && (
              <button
                type="button"
                onClick={onCancel}
                className="rounded-lg border border-slate-700 px-3 py-2 text-xs text-slate-300 hover:text-slate-100"
              >
                Cancel
              </button>
            )}
            <button
              type="submit"
              disabled={!canSubmit}
              className="rounded-lg bg-sky-500 px-4 py-2 text-xs font-semibold text-slate-900 disabled:opacity-60"
            >
              {submitting ? "Saving..." : isSetup ? "Save PIN" : "Unlock"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
