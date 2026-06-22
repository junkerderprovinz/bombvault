import { useEffect, useState } from "react";
import { backupFlashNow, listFlashSnapshots, flashDownloadURL, deleteSnapshot } from "../lib/api";
import type { Snapshot } from "../lib/api";
import { useT } from "../lib/i18n";
import { ProgressBar } from "../components/ProgressBar";
import { useProgress } from "../lib/progress";

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
// Snapshot row (zip download restore)
// ---------------------------------------------------------------------------

function FlashSnapshotRow({ snap, onDeleted, t }: { snap: Snapshot; onDeleted: () => void; t: T }) {
  const [deleting, setDeleting] = useState(false);
  const [deleteErr, setDeleteErr] = useState<string | null>(null);

  async function handleDelete() {
    if (!window.confirm(t("snapshots.deleteConfirm"))) return;
    setDeleting(true);
    setDeleteErr(null);
    try {
      const res = await deleteSnapshot("flash", snap.id);
      if (res.ok) onDeleted();
      else setDeleteErr(res.error ?? "Delete failed");
    } catch (err) {
      setDeleteErr(err instanceof Error ? err.message : "Delete failed");
    } finally {
      setDeleting(false);
    }
  }

  return (
    <div className="flex flex-col gap-1 py-2.5 border-b border-carbon-border last:border-0">
      <div className="flex items-center gap-3 text-sm">
        <span className="font-mono text-carbon-text text-xs w-20 shrink-0">{snap.id.slice(0, 8)}</span>
        <span className="text-carbon-textMuted text-xs flex-1">
          {new Date(snap.time).toLocaleString()}
        </span>
        {/* Non-destructive zip download: a GET link carries the session cookie,
            so the browser downloads flash-<id>.zip straight from restic dump. */}
        <a
          href={flashDownloadURL(snap.id)}
          download
          className="inline-flex items-center gap-1.5 rounded-lg bg-accent px-2.5 py-1 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity shrink-0"
        >
          {t("flash.download")}
        </a>
        <button
          onClick={() => void handleDelete()}
          disabled={deleting}
          title={t("snapshots.delete")}
          className="shrink-0 rounded-lg border border-carbon-border px-2 py-1 text-xs text-carbon-textSub hover:bg-[#3a1c1c] hover:text-[#ff8389] transition-colors disabled:opacity-50"
        >
          {deleting ? "…" : t("snapshots.delete")}
        </button>
      </div>
      {deleteErr && <p className="text-xs text-[#ff8389] pl-24 break-words">{deleteErr}</p>}
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
  const progress = useProgress()["flash"];

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
      <div className="relative overflow-hidden bg-carbon-surface rounded-card border border-carbon-border p-5 flex flex-col gap-4">
        <h2 className="text-sm font-semibold text-carbon-textSub uppercase tracking-widest">
          {t("flash.backupTitle")}
        </h2>
        <p className="text-xs text-carbon-textMuted -mt-1">{t("flash.backupHint")}</p>
        <FlashBackupButton t={t} onBackedUp={() => void load()} />

        {/* Live backup/restore progress, pinned to the card's bottom edge */}
        {progress && (
          <ProgressBar percent={progress.percent} active={progress.active} />
        )}
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
              <FlashSnapshotRow key={snap.id} snap={snap} onDeleted={() => void load()} t={t} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
