import { useState } from "react";
import { backupNow } from "../lib/api";
import type { useT } from "../lib/i18n";

type T = ReturnType<typeof useT>["t"];

interface BackupButtonProps {
  name: string;
  t: T;
}

type BackupState =
  | { phase: "idle" }
  | { phase: "pending" }
  | { phase: "success"; snapshotId?: string }
  | { phase: "error"; message: string };

export function BackupButton({ name, t }: BackupButtonProps) {
  const [state, setState] = useState<BackupState>({ phase: "idle" });

  async function handleBackup() {
    setState({ phase: "pending" });
    try {
      const res = await backupNow(name);
      if (res.ok) {
        setState({ phase: "success", snapshotId: res.snapshotId });
        // Auto-clear success after 4 s
        setTimeout(() => setState({ phase: "idle" }), 4000);
      } else {
        setState({ phase: "error", message: res.error ?? "Backup failed" });
      }
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Network error";
      setState({ phase: "error", message: msg });
    }
  }

  const isPending = state.phase === "pending";

  return (
    <div className="flex flex-col gap-1 items-start">
      <button
        onClick={() => void handleBackup()}
        disabled={isPending}
        className="inline-flex items-center gap-1.5 rounded-lg bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50 disabled:cursor-not-allowed"
      >
        {isPending ? (
          <>
            <span
              className="h-3 w-3 rounded-full border-2 border-t-transparent animate-spin inline-block"
              style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
            />
            Backing up…
          </>
        ) : (
          t("containers.backupNow")
        )}
      </button>

      {state.phase === "success" && (
        <span className="text-xs text-[#6fdc8c]">
          ✓ Done
          {state.snapshotId && (
            <span className="font-mono ml-1 text-carbon-textMuted">
              {state.snapshotId.slice(0, 8)}
            </span>
          )}
        </span>
      )}

      {state.phase === "error" && (
        <span className="text-xs text-[#ff8389] max-w-[18rem] break-words">
          {state.message}
        </span>
      )}
    </div>
  );
}
