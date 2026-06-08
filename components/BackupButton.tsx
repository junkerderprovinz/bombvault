"use client";

import { useState } from "react";
import { backupNowAction, type BackupActionState } from "@/app/containers/actions";

// One-click backup button. Calls the backup server action and shows the result
// (error or success) inline next to the button — the page never crashes via the
// error boundary. A manual `pending` flag (not useTransition) reliably tracks the
// async server-action call on React 18 as well as 19.
export function BackupButton({
  containerName,
  label,
}: {
  containerName: string;
  label: string;
}) {
  const [state, setState] = useState<BackupActionState | null>(null);
  const [pending, setPending] = useState(false);

  async function onClick() {
    setPending(true);
    try {
      const formData = new FormData();
      formData.set("container", containerName);
      setState(await backupNowAction({ ok: true }, formData));
    } catch {
      // The action itself returns a graceful state and does not throw; this guard
      // only covers a transport-level failure (e.g. the request was aborted).
      setState({ ok: false, error: "request failed" });
    } finally {
      setPending(false);
    }
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
      <button
        type="button"
        onClick={onClick}
        disabled={pending}
        style={{
          background: "var(--accent)",
          color: "#fff",
          border: "none",
          borderRadius: 2,
          padding: "0.35rem 0.9rem",
          fontSize: "0.875rem",
          cursor: pending ? "progress" : "pointer",
          opacity: pending ? 0.6 : 1,
          fontFamily: "inherit",
        }}
      >
        {pending ? "…" : label}
      </button>

      {state && !state.ok && state.error ? (
        <span style={{ color: "var(--error)", fontSize: "0.75rem", maxWidth: 220, wordBreak: "break-word" }}>
          {state.error}
        </span>
      ) : state && state.ok && state.message ? (
        <span style={{ color: "var(--success)", fontSize: "0.75rem" }}>{state.message}</span>
      ) : null}
    </div>
  );
}
