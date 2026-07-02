import { useEffect, useState } from "react";
import { backupFlashNow, listFlashSnapshots, flashDownloadURL, deleteSnapshot } from "../lib/api";
import type { Snapshot } from "../lib/api";
import { useT } from "../lib/i18n";
import { ProgressBar } from "../components/ProgressBar";
import { useProgress, anyActive } from "../lib/progress";
import { useBackupWatch } from "../lib/backupWatch";
import { SourceToggle, type RepoSource } from "../components/SourceToggle";
import { OffsiteIndicator } from "../components/OffsiteIndicator";

type T = ReturnType<typeof useT>["t"];

// ---------------------------------------------------------------------------
// Backup button
// ---------------------------------------------------------------------------

function FlashBackupButton({
  t,
  onBackedUp,
  externallyBusy = false,
  busyPhase,
}: {
  t: T;
  onBackedUp: () => void;
  /** True when a backup/restore is running elsewhere (any domain). */
  externallyBusy?: boolean;
  busyPhase?: string;
}) {
  // Fire-and-watch (see useBackupWatch): the flash backup runs detached on the
  // server and the POST returns immediately, so we watch the "flash" progress +
  // recorded run for the outcome instead of awaiting the whole backup.
  const { state, fire, isPending } = useBackupWatch({
    progressKey: "flash",
    start: () => backupFlashNow(),
    matchRun: (r) => r.domain === "flash",
    onDone: onBackedUp,
  });

  return (
    <div className="flex flex-col gap-1 items-start">
      <button
        onClick={() => void fire()}
        disabled={isPending || externallyBusy}
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
      {/* A backup/restore elsewhere blocks a new flash backup — explain why. */}
      {externallyBusy && !isPending && (
        <span className="text-xs text-carbon-textMuted">
          {busyPhase === "restore" ? t("common.restoreRunning") : t("common.backupRunning")}
        </span>
      )}
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

function FlashSnapshotRow({ snap, source, onDeleted, t }: { snap: Snapshot; source: RepoSource; onDeleted: () => void; t: T }) {
  const [deleting, setDeleting] = useState(false);
  const [deleteErr, setDeleteErr] = useState<string | null>(null);
  type DL =
    | { phase: "idle" }
    | { phase: "downloading"; bytes: number }
    | { phase: "done" }
    | { phase: "error"; message: string };
  const [dl, setDl] = useState<DL>({ phase: "idle" });

  async function handleDelete() {
    if (!window.confirm(t("snapshots.deleteConfirm"))) return;
    setDeleting(true);
    setDeleteErr(null);
    try {
      const res = await deleteSnapshot("flash", snap.id, source);
      if (res.ok) onDeleted();
      else setDeleteErr(res.error ?? "Delete failed");
    } catch (err) {
      setDeleteErr(err instanceof Error ? err.message : "Delete failed");
    } finally {
      setDeleting(false);
    }
  }

  // Stream the zip via fetch so the button shows live progress (restic dump has
  // no known total size, so we report bytes received, not a percentage) and
  // can't be triggered twice. A pre-stream failure comes back as a JSON error.
  async function handleDownload() {
    setDl({ phase: "downloading", bytes: 0 });
    try {
      const res = await fetch(flashDownloadURL(snap.id, source));
      const ct = res.headers.get("Content-Type") ?? "";
      if (ct.includes("application/json") || !res.body) {
        let msg = "Download failed";
        try {
          const j = (await res.json()) as { error?: string };
          if (j.error) msg = j.error;
        } catch {
          /* keep default */
        }
        setDl({ phase: "error", message: msg });
        return;
      }
      const reader = res.body.getReader();
      const chunks: Uint8Array[] = [];
      let received = 0;
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        if (value) {
          chunks.push(value);
          received += value.length;
          setDl({ phase: "downloading", bytes: received });
        }
      }
      const blob = new Blob(chunks as BlobPart[], { type: "application/zip" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `flash-${snap.id.slice(0, 8)}.zip`;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
      setDl({ phase: "done" });
      setTimeout(() => setDl({ phase: "idle" }), 4000);
    } catch (err) {
      setDl({ phase: "error", message: err instanceof Error ? err.message : "Network error" });
    }
  }

  const downloading = dl.phase === "downloading";

  return (
    <div className="flex flex-col gap-1 py-2.5 border-b border-carbon-border last:border-0">
      <div className="flex items-center gap-3 text-sm">
        <span className="font-mono text-carbon-text text-xs w-20 shrink-0">{snap.id.slice(0, 8)}</span>
        <span className="text-carbon-textMuted text-xs flex-1">
          {new Date(snap.time).toLocaleString()}
        </span>
        <button
          onClick={() => void handleDownload()}
          disabled={downloading}
          className="inline-flex items-center gap-1.5 rounded-lg bg-accent px-2.5 py-1 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-60 shrink-0"
        >
          {downloading ? (
            <>
              <span
                className="h-2.5 w-2.5 rounded-full border-2 border-t-transparent animate-spin inline-block"
                style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
              />
              {t("flash.downloading")} {(dl.bytes / 1048576).toFixed(0)} MB
            </>
          ) : (
            t("flash.download")
          )}
        </button>
        <button
          onClick={() => void handleDelete()}
          disabled={deleting || downloading}
          title={t("snapshots.delete")}
          className="shrink-0 rounded-lg border border-carbon-border px-2 py-1 text-xs text-carbon-textSub hover:bg-[#3a1c1c] hover:text-[#ff8389] transition-colors disabled:opacity-50"
        >
          {deleting ? "…" : t("snapshots.delete")}
        </button>
      </div>
      {dl.phase === "error" && <p className="text-xs text-[#ff8389] pl-24 break-words">{dl.message}</p>}
      {deleteErr && <p className="text-xs text-[#ff8389] pl-24 break-words">{deleteErr}</p>}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Flash page
// ---------------------------------------------------------------------------

export function Flash() {
  const { t } = useT();
  const [source, setSource] = useState<RepoSource>("local");
  const [snapshots, setSnapshots] = useState<Snapshot[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const progressMap = useProgress();
  const progress = progressMap["flash"];
  // Any backup/restore/replication in flight (any domain) disables the flash
  // backup button + shows a hint, instead of relying on the 409 round-trip.
  const running = anyActive(progressMap);

  function load() {
    setError(null);
    return listFlashSnapshots(source)
      .then((res) => {
        if (res.ok) setSnapshots(res.snapshots ?? []);
        else setError(res.error ?? "Failed to load flash backups");
      })
      .catch((err: unknown) =>
        setError(err instanceof Error ? err.message : "Failed to load flash backups")
      );
  }

  useEffect(() => {
    setLoading(true);
    void load().finally(() => setLoading(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [source]);

  return (
    <div className="flex flex-col gap-6 max-w-3xl">
      {/* Page heading */}
      <div>
        <h1 className="text-2xl font-semibold text-carbon-text">{t("flash.title")}</h1>
        <p className="mt-1 text-sm text-carbon-textSub">{t("flash.subtitle")}</p>
        <div className="mt-2"><OffsiteIndicator domain="flash" /></div>
      </div>

      {/* Backup card */}
      <div className="relative overflow-hidden bg-carbon-surface rounded-card border border-carbon-border p-5 flex flex-col gap-4">
        <h2 className="text-sm font-semibold text-carbon-textSub uppercase tracking-widest">
          {t("flash.backupTitle")}
        </h2>
        <p className="text-xs text-carbon-textMuted -mt-1">{t("flash.backupHint")}</p>
        <FlashBackupButton
          t={t}
          onBackedUp={() => void load()}
          externallyBusy={running.active}
          busyPhase={running.phase}
        />

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

        <div className="flex flex-col gap-1">
          <div className="flex items-center gap-2">
            <span className="text-xs text-carbon-textMuted">{t("source.label")}</span>
            <SourceToggle source={source} onChange={setSource} disabled={loading} />
          </div>
          <p className="text-[11px] text-carbon-textMuted">{t("source.hint")}</p>
        </div>

        {loading && <p className="text-xs text-carbon-textMuted">{t("dashboard.checking")}</p>}
        {error && <p className="text-xs text-[#ff8389]">{error}</p>}
        {!loading && !error && snapshots.length === 0 && (
          <p className="text-xs text-carbon-textMuted">{t("flash.none")}</p>
        )}
        {!loading && snapshots.length > 0 && (
          <div className="rounded-lg border border-carbon-border bg-carbon-background px-3 py-1">
            {snapshots.map((snap) => (
              <FlashSnapshotRow key={snap.id} snap={snap} source={source} onDeleted={() => void load()} t={t} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
