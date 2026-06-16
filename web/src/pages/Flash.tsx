import { useEffect, useState } from "react";
import { backupFlashNow, listFlashSnapshots, restoreFlash } from "../lib/api";
import type { Snapshot } from "../lib/api";
import { useT } from "../lib/i18n";

type T = ReturnType<typeof useT>["t"];

// ---------------------------------------------------------------------------
// Backup button
// ---------------------------------------------------------------------------

function FlashBackupButton({ t, onBackedUp }: { t: T; onBackedUp: () => void }) {
  type State =
    | { phase: "idle" }
    | { phase: "pending" }
    | { phase: "success"; snapshotId?: string }
    | { phase: "error"; message: string };
  const [state, setState] = useState<State>({ phase: "idle" });

  async function handleBackup() {
    setState({ phase: "pending" });
    try {
      const res = await backupFlashNow();
      if (res.ok) {
        setState({ phase: "success", snapshotId: res.snapshotId });
        onBackedUp();
        setTimeout(() => setState({ phase: "idle" }), 4000);
      } else {
        setState({ phase: "error", message: res.error ?? "Backup failed" });
      }
    } catch (err) {
      setState({ phase: "error", message: err instanceof Error ? err.message : "Network error" });
    }
  }

  const isPending = state.phase === "pending";
  return (
    <div className="flex flex-col gap-1 items-start">
      <button
        onClick={() => void handleBackup()}
        disabled={isPending}
        className="inline-flex items-center gap-1.5 rounded-lg bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
      >
        {isPending ? (
          <>
            <span
              className="h-3.5 w-3.5 rounded-full border-2 border-t-transparent animate-spin inline-block"
              style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
            />
            {t("flash.backingUp")}
          </>
        ) : (
          t("flash.backupNow")
        )}
      </button>
      {state.phase === "success" && (
        <span className="text-xs text-[#6fdc8c]">
          ✓ {t("settings.saved")}
          {state.snapshotId && (
            <span className="font-mono ml-1 text-carbon-textMuted">{state.snapshotId.slice(0, 8)}</span>
          )}
        </span>
      )}
      {state.phase === "error" && (
        <span className="text-xs text-[#ff8389] max-w-[28rem] break-words">{state.message}</span>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Snapshot row (safe extract restore)
// ---------------------------------------------------------------------------

function FlashSnapshotRow({ snap, t }: { snap: Snapshot; t: T }) {
  const [confirmed, setConfirmed] = useState(false);
  type State =
    | { phase: "idle" }
    | { phase: "pending" }
    | { phase: "success"; target?: string }
    | { phase: "error"; message: string };
  const [state, setState] = useState<State>({ phase: "idle" });

  async function handleRestore() {
    if (!confirmed) return;
    setState({ phase: "pending" });
    try {
      const res = await restoreFlash(snap.id, true);
      if (res.ok) {
        setState({ phase: "success", target: res.target });
      } else {
        setState({ phase: "error", message: res.error ?? "Restore failed" });
      }
    } catch (err) {
      setState({ phase: "error", message: err instanceof Error ? err.message : "Network error" });
    }
  }

  const isPending = state.phase === "pending";
  const done = state.phase === "success";
  return (
    <div className="flex flex-col gap-1 py-2.5 border-b border-carbon-border last:border-0">
      <div className="flex items-center gap-3 text-sm">
        <span className="font-mono text-carbon-text text-xs w-20 shrink-0">{snap.id.slice(0, 8)}</span>
        <span className="text-carbon-textMuted text-xs flex-1">
          {new Date(snap.time).toLocaleString()}
        </span>
        <label className="flex items-center gap-1.5 text-xs text-carbon-textSub cursor-pointer shrink-0">
          <input
            type="checkbox"
            checked={confirmed}
            onChange={(e) => setConfirmed(e.target.checked)}
            disabled={isPending || done}
            className="rounded border-carbon-border bg-carbon-surface2 focus:ring-offset-0"
            style={{ accentColor: "var(--accent)" }}
          />
          {t("restore.confirm")}
        </label>
        <button
          onClick={() => void handleRestore()}
          disabled={!confirmed || isPending || done}
          className="inline-flex items-center gap-1.5 rounded-lg bg-accent px-2.5 py-1 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-40 shrink-0"
        >
          {isPending ? (
            <>
              <span
                className="h-2.5 w-2.5 rounded-full border-2 border-t-transparent animate-spin inline-block"
                style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
              />
              {t("flash.restoring")}
            </>
          ) : (
            t("snapshots.restore")
          )}
        </button>
      </div>
      {state.phase === "success" && (
        <p className="text-xs text-[#6fdc8c] pl-24 break-words">
          {t("flash.restoredTo")} <span className="font-mono">{state.target}</span>
        </p>
      )}
      {state.phase === "error" && (
        <p className="text-xs text-[#ff8389] pl-24 break-words">{state.message}</p>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Flash page
// ---------------------------------------------------------------------------

export function Flash() {
  const { t } = useT();
  const [snapshots, setSnapshots] = useState<Snapshot[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  function load() {
    return listFlashSnapshots()
      .then((res) => {
        if (res.ok) setSnapshots(res.snapshots ?? []);
        else setError("Failed to load flash backups");
      })
      .catch((err: unknown) =>
        setError(err instanceof Error ? err.message : "Failed to load flash backups")
      );
  }

  useEffect(() => {
    void load().finally(() => setLoading(false));
  }, []);

  return (
    <div className="flex flex-col gap-6 max-w-3xl">
      {/* Page heading */}
      <div>
        <h1 className="text-2xl font-semibold text-carbon-text">{t("flash.title")}</h1>
        <p className="mt-1 text-sm text-carbon-textSub">{t("flash.subtitle")}</p>
      </div>

      {/* Backup card */}
      <div className="bg-carbon-surface rounded-card border border-carbon-border p-5 flex flex-col gap-4">
        <h2 className="text-sm font-semibold text-carbon-textSub uppercase tracking-widest">
          {t("flash.backupTitle")}
        </h2>
        <p className="text-xs text-carbon-textMuted -mt-1">{t("flash.backupHint")}</p>
        <FlashBackupButton t={t} onBackedUp={() => void load()} />
      </div>

      {/* Restore card */}
      <div className="bg-carbon-surface rounded-card border border-carbon-border p-5 flex flex-col gap-4">
        <h2 className="text-sm font-semibold text-carbon-textSub uppercase tracking-widest">
          {t("snapshots.title")}
        </h2>
        {/* Safe-restore explainer */}
        <div className="rounded-lg bg-[#1c2a3a] border border-[#2a4055] px-3 py-2.5 text-xs text-[#78a9ff] leading-relaxed">
          {t("flash.restoreNote")}
        </div>

        {loading && <p className="text-xs text-carbon-textMuted">{t("dashboard.checking")}</p>}
        {error && <p className="text-xs text-[#ff8389]">{error}</p>}
        {!loading && !error && snapshots.length === 0 && (
          <p className="text-xs text-carbon-textMuted">{t("flash.none")}</p>
        )}
        {!loading && snapshots.length > 0 && (
          <div className="rounded-lg border border-carbon-border bg-carbon-background px-3 py-1">
            {snapshots.map((snap) => (
              <FlashSnapshotRow key={snap.id} snap={snap} t={t} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
