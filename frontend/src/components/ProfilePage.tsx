import React, { useCallback, useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { UnlockThrottledError } from "../crypto-core/keyManager";
import { getKeyManager, resetKeyManager } from "../lib/keyManagerInstance";
import { getItem, removeItem, wipeDatabaseIfExists } from "../lib/storage";
import { ProfileDataOwnership } from "../lib/profileDataOwnership";
import { loadProfileSections } from "../services/profileApi";
import {
  deleteAuthData,
  deleteKeysData,
  deleteMessagesData,
} from "../services/profileDeleteApi";
import type {
  AuthProfile,
  MessagesProfile,
  ProfileFieldKey,
  ProfileSectionState,
  ProfileSections,
} from "../types/profile";
import { PinModal } from "./PinModal";

const serviceLabels: Record<"auth" | "messages" | "keys", string> = {
  auth: "Auth service",
  keys: "Keys service",
  messages: "Messages service",
};

const loadingSections: ProfileSections = {
  auth: { status: "loading" },
  messages: { status: "loading" },
};

type DeleteService = "auth" | "keys" | "messages";
type DeleteTarget = DeleteService | "all";

type DeleteResultState = {
  status: "pending" | "success" | "error" | "skipped";
  error?: string;
};

type DeleteResults = Record<DeleteService, DeleteResultState>;

const renderFieldValue = (value: string | number | boolean | null) => {
  if (value === null || value === "") return "—";
  if (typeof value === "boolean") return value ? "Yes" : "No";
  return String(value);
};

const FieldCard: React.FC<{
  label: string;
  value: string | number | boolean | null;
  fieldKey: ProfileFieldKey;
}> = ({ label, value, fieldKey }) => {
  const owner = ProfileDataOwnership[fieldKey];
  return (
    <div className="rounded-xl border border-slate-800 bg-slate-950/60 px-4 py-3">
      <p className="text-xs uppercase tracking-wide text-slate-400">{label}</p>
      <p className="mt-1 text-sm text-slate-100">
        {renderFieldValue(value)}
      </p>
      <p className="mt-2 text-[11px] text-slate-500">
        Source service: {serviceLabels[owner.service]}
      </p>
    </div>
  );
};

const SectionCard: React.FC<{
  title: string;
  service: "auth" | "messages";
  state: ProfileSectionState<AuthProfile> | ProfileSectionState<MessagesProfile>;
  children: React.ReactNode;
}> = ({ title, service, state, children }) => {
  return (
    <section className="rounded-2xl border border-slate-800 bg-slate-900/70 p-6 shadow-xl">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-lg font-semibold text-slate-100">{title}</h2>
          <p className="text-xs text-slate-400">
            Source service: {serviceLabels[service]}
          </p>
        </div>
        {state.status === "loading" && (
          <span className="rounded-full border border-slate-700 px-3 py-1 text-xs text-slate-400">
            Loading
          </span>
        )}
        {state.status === "error" && (
          <span className="rounded-full border border-red-700/60 bg-red-950/40 px-3 py-1 text-xs text-red-300">
            Error
          </span>
        )}
      </div>
      <div className="mt-5">
        {state.status === "loading" && (
          <p className="text-sm text-slate-400">Fetching data...</p>
        )}
        {state.status === "error" && (
          <p className="text-sm text-red-300">{state.error}</p>
        )}
        {state.status === "ready" && children}
      </div>
    </section>
  );
};

export const ProfilePage: React.FC = () => {
  const [lockState, setLockState] = useState<"checking" | "locked" | "unlocked">(
    "checking"
  );
  const [unlockError, setUnlockError] = useState<string | null>(null);
  const [unlockWaitSeconds, setUnlockWaitSeconds] = useState<number | null>(null);
  const keyManagerRef = useRef<ReturnType<typeof getKeyManager> | null>(null);
  const [sections, setSections] = useState<ProfileSections>(loadingSections);
  const [confirmTarget, setConfirmTarget] = useState<DeleteTarget | null>(null);
  const [confirmText, setConfirmText] = useState("");
  const [confirmError, setConfirmError] = useState<string | null>(null);
  const [deleteStatus, setDeleteStatus] = useState<
    "idle" | "running" | "done"
  >("idle");
  const [deleteResults, setDeleteResults] = useState<DeleteResults | null>(null);
  const navigate = useNavigate();

  const handleLock = useCallback(() => {
    setLockState("locked");
    setSections(loadingSections);
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
      if (!cancelled) {
        setLockState(manager.isUnlocked() ? "unlocked" : "locked");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [handleLock, navigate]);

  useEffect(() => {
    if (lockState !== "unlocked") {
      return;
    }
    let cancelled = false;
    setSections(loadingSections);
    (async () => {
      const snapshot = await loadProfileSections();
      if (!cancelled) {
        setSections(snapshot);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [lockState]);

  const ownershipEntries = Object.entries(ProfileDataOwnership).map(
    ([field, info]) => ({
      field,
      service: info.service,
      description: info.description,
    })
  );

  const groupedOwnership = ownershipEntries.reduce<
    Record<"auth" | "messages", typeof ownershipEntries>
  >(
    (acc, entry) => {
      acc[entry.service].push(entry);
      return acc;
    },
    { auth: [], messages: [] }
  );

  const buildDeleteResults = (services: DeleteService[]): DeleteResults => ({
    auth: services.includes("auth") ? { status: "pending" } : { status: "skipped" },
    keys: services.includes("keys") ? { status: "pending" } : { status: "skipped" },
    messages: services.includes("messages")
      ? { status: "pending" }
      : { status: "skipped" },
  });

  const updateDeleteResult = (
    service: DeleteService,
    next: DeleteResultState
  ) => {
    setDeleteResults((prev) => {
      const current = prev ?? buildDeleteResults(["auth", "keys", "messages"]);
      return { ...current, [service]: next };
    });
  };

  const cleanupLocalState = async () => {
    await wipeDatabaseIfExists();
    await Promise.all([
      removeItem("accessToken"),
      removeItem("refreshToken"),
      removeItem("userId"),
      removeItem("username"),
      removeItem("deviceId"),
      removeItem("deviceName"),
      removeItem("devicePlatform"),
    ]);
    resetKeyManager();
    setLockState("locked");
  };

  const handleDelete = async (target: DeleteTarget) => {
    setDeleteStatus("running");
    setConfirmError(null);
    const deviceId = await getItem("deviceId");

    if (target === "all") {
      setDeleteResults(buildDeleteResults(["auth", "keys", "messages"]));
      const [messagesResult, keysResult] = await Promise.allSettled([
        deleteMessagesData(deviceId ?? undefined),
        deleteKeysData(),
      ]);

      if (messagesResult.status === "fulfilled") {
        updateDeleteResult("messages", { status: "success" });
      } else {
        updateDeleteResult("messages", {
          status: "error",
          error: messagesResult.reason instanceof Error
            ? messagesResult.reason.message
            : "Deletion failed.",
        });
      }

      if (keysResult.status === "fulfilled") {
        updateDeleteResult("keys", { status: "success" });
      } else {
        updateDeleteResult("keys", {
          status: "error",
          error: keysResult.reason instanceof Error
            ? keysResult.reason.message
            : "Deletion failed.",
        });
      }

      if (messagesResult.status === "fulfilled" && keysResult.status === "fulfilled") {
        const authResult = await Promise.allSettled([deleteAuthData()]);
        if (authResult[0].status === "fulfilled") {
          updateDeleteResult("auth", { status: "success" });
          await cleanupLocalState();
          navigate("/");
        } else {
          updateDeleteResult("auth", {
            status: "error",
            error: authResult[0].reason instanceof Error
              ? authResult[0].reason.message
              : "Deletion failed.",
          });
        }
      } else {
        updateDeleteResult("auth", {
          status: "error",
          error: "Skipped because another service failed.",
        });
      }
      setDeleteStatus("done");
      return;
    }

    const servicesToDelete: DeleteService[] = [target];
    setDeleteResults(buildDeleteResults(servicesToDelete));
    const requests: Array<Promise<unknown>> = [];
    if (target === "messages") {
      requests.push(deleteMessagesData(deviceId ?? undefined));
    } else if (target === "keys") {
      requests.push(deleteKeysData());
    } else {
      requests.push(deleteAuthData());
    }
    const settled = await Promise.allSettled(requests);
    const result = settled[0];
    if (result.status === "fulfilled") {
      updateDeleteResult(target, { status: "success" });
      if (target === "auth") {
        await cleanupLocalState();
        navigate("/");
      }
    } else {
      updateDeleteResult(target, {
        status: "error",
        error: result.reason instanceof Error
          ? result.reason.message
          : "Deletion failed.",
      });
    }
    setDeleteStatus("done");
  };

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 px-4 py-10">
      <PinModal
        isOpen={lockState === "locked"}
        mode="unlock"
        title="Unlock your vault"
        helper="Enter your 4-digit PIN to view your profile."
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

      {confirmTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/80 px-4">
          <div className="w-full max-w-md rounded-2xl border border-slate-800 bg-slate-900/90 p-6 shadow-2xl">
            <h3 className="text-lg font-semibold text-slate-100">
              Confirm deletion
            </h3>
            <p className="mt-2 text-sm text-slate-400">
              This will permanently delete your data from the selected service.
              Type <span className="text-slate-100">DELETE</span> to confirm.
            </p>
            <input
              className="mt-4 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 focus:outline-none focus:ring-2 focus:ring-red-500"
              placeholder="Type DELETE to confirm"
              value={confirmText}
              onChange={(e) => setConfirmText(e.target.value.toUpperCase())}
            />
            {confirmError && (
              <p className="mt-2 text-xs text-red-300">{confirmError}</p>
            )}
            <div className="mt-5 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => {
                  setConfirmTarget(null);
                  setConfirmText("");
                  setConfirmError(null);
                }}
                className="rounded-lg border border-slate-700 px-4 py-2 text-sm text-slate-200 hover:border-slate-500"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={async () => {
                  if (confirmText !== "DELETE") {
                    setConfirmError("Please type DELETE to confirm.");
                    return;
                  }
                  const target = confirmTarget;
                  setConfirmTarget(null);
                  setConfirmText("");
                  await handleDelete(target);
                }}
                className="rounded-lg bg-red-500 px-4 py-2 text-sm font-semibold text-slate-950 hover:bg-red-400"
              >
                Delete data
              </button>
            </div>
          </div>
        </div>
      )}

      <div className="mx-auto flex w-full max-w-4xl flex-col gap-8">
        <header className="rounded-2xl border border-slate-800 bg-slate-900/70 p-6 shadow-xl">
          <h1 className="text-2xl font-semibold">Profile overview</h1>
          <p className="mt-2 text-sm text-slate-400">
            Data is sourced from multiple backend services. Each field shows
            which service owns it.
          </p>
        </header>

        <SectionCard title="Account details" service="auth" state={sections.auth}>
          {sections.auth.status === "ready" && (
            <div className="grid gap-4 sm:grid-cols-2">
              <FieldCard
                label="User ID"
                value={sections.auth.data.userId}
                fieldKey="auth.userId"
              />
              <FieldCard
                label="Session ID"
                value={sections.auth.data.sessionId ?? null}
                fieldKey="auth.sessionId"
              />
              <FieldCard
                label="Token device ID"
                value={sections.auth.data.tokenDeviceId ?? null}
                fieldKey="auth.tokenDeviceId"
              />
              <FieldCard
                label="Device authorized"
                value={sections.auth.data.deviceAuthorized}
                fieldKey="auth.deviceAuthorized"
              />
            </div>
          )}
        </SectionCard>

        <SectionCard
          title="Messaging footprint"
          service="messages"
          state={sections.messages}
        >
          {sections.messages.status === "ready" && (
            <div className="grid gap-4 sm:grid-cols-2">
              <FieldCard
                label="Conversation count"
                value={sections.messages.data.conversationCount}
                fieldKey="messages.conversationCount"
              />
            </div>
          )}
        </SectionCard>

        <section className="rounded-2xl border border-slate-800 bg-slate-900/70 p-6 shadow-xl">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <h2 className="text-lg font-semibold text-slate-100">
                Delete my data
              </h2>
              <p className="text-xs text-slate-400">
                Trigger GDPR-style deletions per service or remove everything at
                once.
              </p>
            </div>
            {deleteStatus === "running" && (
              <span className="rounded-full border border-slate-700 px-3 py-1 text-xs text-slate-400">
                Deleting...
              </span>
            )}
          </div>

          <div className="mt-5 grid gap-3 sm:grid-cols-2">
            <button
              type="button"
              disabled={deleteStatus === "running"}
              onClick={() => {
                setConfirmTarget("messages");
                setConfirmText("");
                setConfirmError(null);
              }}
              className="rounded-lg border border-slate-700 bg-slate-950/60 px-4 py-2 text-sm text-slate-200 hover:border-slate-500 disabled:opacity-60"
            >
              Delete from Messages
            </button>
            <button
              type="button"
              disabled={deleteStatus === "running"}
              onClick={() => {
                setConfirmTarget("keys");
                setConfirmText("");
                setConfirmError(null);
              }}
              className="rounded-lg border border-slate-700 bg-slate-950/60 px-4 py-2 text-sm text-slate-200 hover:border-slate-500 disabled:opacity-60"
            >
              Delete from Keys
            </button>
            <button
              type="button"
              disabled={deleteStatus === "running"}
              onClick={() => {
                setConfirmTarget("auth");
                setConfirmText("");
                setConfirmError(null);
              }}
              className="rounded-lg border border-red-700/60 bg-red-950/40 px-4 py-2 text-sm text-red-200 hover:border-red-500 disabled:opacity-60"
            >
              Delete from Auth (logs you out)
            </button>
            <button
              type="button"
              disabled={deleteStatus === "running"}
              onClick={() => {
                setConfirmTarget("all");
                setConfirmText("");
                setConfirmError(null);
              }}
              className="rounded-lg bg-red-500 px-4 py-2 text-sm font-semibold text-slate-950 hover:bg-red-400 disabled:opacity-60"
            >
              Delete everything
            </button>
          </div>

          {deleteResults && (
            <div className="mt-6 space-y-2 text-sm">
              {(["messages", "keys", "auth"] as DeleteService[]).map((service) => {
                const result = deleteResults[service];
                if (!result) return null;
                const statusText =
                  result.status === "pending"
                    ? "Pending"
                    : result.status === "success"
                    ? "Deleted"
                    : result.status === "skipped"
                    ? "Not requested"
                    : "Failed";
                const statusClass =
                  result.status === "success"
                    ? "text-emerald-300"
                    : result.status === "skipped"
                    ? "text-slate-500"
                    : result.status === "error"
                    ? "text-red-300"
                    : "text-slate-400";
                return (
                  <div
                    key={service}
                    className="flex items-center justify-between rounded-lg border border-slate-800 bg-slate-950/50 px-3 py-2"
                  >
                    <span className="text-slate-200">
                      {serviceLabels[service]}
                    </span>
                    <span className={statusClass}>
                      {statusText}
                      {result.error ? ` — ${result.error}` : ""}
                    </span>
                  </div>
                );
              })}
            </div>
          )}
        </section>

        <section className="rounded-2xl border border-slate-800 bg-slate-900/70 p-6 shadow-xl">
          <h2 className="text-lg font-semibold text-slate-100">
            Data transparency
          </h2>
          <p className="mt-1 text-sm text-slate-400">
            What we store where, based on the services that own each field.
          </p>
          <div className="mt-4 grid gap-4 md:grid-cols-2">
            {(Object.keys(groupedOwnership) as Array<
              keyof typeof groupedOwnership
            >).map((service) => (
              <div
                key={service}
                className="rounded-xl border border-slate-800 bg-slate-950/60 p-4"
              >
                <p className="text-xs uppercase tracking-wide text-slate-400">
                  Source service: {serviceLabels[service]}
                </p>
                <ul className="mt-3 space-y-2 text-sm text-slate-200">
                  {groupedOwnership[service].map((entry) => (
                    <li
                      key={entry.field}
                      className="rounded-lg border border-slate-800/70 bg-slate-950/70 px-3 py-2"
                    >
                      <p className="text-slate-100">{entry.field}</p>
                      <p className="text-xs text-slate-400">
                        {entry.description}
                      </p>
                    </li>
                  ))}
                </ul>
              </div>
            ))}
          </div>
        </section>
      </div>
    </div>
  );
};
