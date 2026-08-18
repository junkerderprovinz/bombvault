import { useEffect, useState } from "react";
import { backupFlashNow, listFlashSnapshots, flashDownloadURL, deleteSnapshot } from "../lib/api";
import type { Snapshot } from "../lib/api";
import { useT } from "../lib/i18n";
import { ProgressBar } from "../components/ProgressBar";
import { useProgress, anyActive, busyPhraseKey } from "../lib/progress";
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
        className="inline-flex items-center gap-1.5 rounded-control bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
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
      {/* A backup/restore/replication elsewhere blocks a new flash backup. */}
      {externallyBusy && !isPending && (
        <span className="text-xs text-carbon-textMuted">
          {t(busyPhraseKey(busyPhase))}
        </span>
      )}
      {state.phase === "success" && (
        <span className="text-xs text-statusOk">
          ✓ {t("settings.saved")}
          {state.snapshotId && (
            <span className="font-mono ml-1 text-carbon-textMuted">{state.snapshotId.slice(0, 8)}</span>
          )}
        </span>
      )}
      {state.phase === "error" && (
        <span className="text-xs text-statusFail max-w-md wrap-break-word">{state.message}</span>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Snapshot row (zip download restore)
// ---------------------------------------------------------------------------

// How long the download button shows a "preparing" spinner after a click,
// bridging the gap before the browser's own download indicator takes over
// (see handleDownload). Not a real progress signal, just a best-effort
// window: the server can't send a single byte until it has finished
// dumping + recompressing the whole snapshot server-side (see
// dumpFlashZipCompat in the Go backend), which for a multi-GB flash backup
// routinely takes longer than a moment. A generous fixed timeout beats no
// feedback at all; there is no browser event to key off instead.
const DOWNLOAD_PREPARING_MS = 20_000;

function FlashSnapshotRow({ snap, source, onDeleted, t }: { snap: Snapshot; source: RepoSource; onDeleted: () => void; t: T }) {
  const [deleting, setDeleting] = useState(false);
  const [deleteErr, setDeleteErr] = useState<string | null>(null);
  const [preparing, setPreparing] = useState(false);

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

  // Native <a download>, not fetch()+Blob: the browser's own download
  // manager then owns progress/completion, so it survives this row
  // unmounting on tab switch (React was silently discarding the old fetch
  // loop's progress state on remount, not the download itself). Trades away
  // the pre-stream JSON-error surfacing that fetch() gives downloadRecoveryKit
  // in lib/api.ts — acceptable here since a flash zip is far larger and
  // failures are rare.
  function handleDownload() {
    setPreparing(true);
    setTimeout(() => setPreparing(false), DOWNLOAD_PREPARING_MS);
    const a = document.createElement("a");
    a.href = flashDownloadURL(snap.id, source);
    a.download = `flash-${snap.id.slice(0, 8)}.zip`;
    document.body.appendChild(a);
    a.click();
    a.remove();
  }

  return (
    <div className="flex flex-col gap-1 py-2.5 border-b border-carbon-border last:border-0">
      <div className="flex items-center gap-3 text-sm">
        <span className="font-mono text-carbon-text text-xs w-20 shrink-0">{snap.id.slice(0, 8)}</span>
        <span className="text-carbon-textMuted text-xs flex-1">
          {new Date(snap.time).toLocaleString()}
        </span>
        <button
          onClick={handleDownload}
          disabled={preparing}
          className="inline-flex items-center gap-1.5 rounded-control bg-accent px-2.5 py-1 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-60 shrink-0"
        >
          {preparing && (
            <span
              className="h-2.5 w-2.5 rounded-full border-2 border-t-transparent animate-spin inline-block"
              style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
            />
          )}
          {t("flash.download")}
        </button>
        <button
          onClick={() => void handleDelete()}
          disabled={deleting || preparing}
          title={t("snapshots.delete")}
          className="shrink-0 rounded-control px-2 py-1 text-xs text-carbon-textSub hover:bg-statusFailBg hover:text-statusFail transition-colors disabled:opacity-50"
        >
          {deleting ? "…" : t("snapshots.delete")}
        </button>
      </div>
      {deleteErr && <p className="text-xs text-statusFail pl-24 wrap-break-word">{deleteErr}</p>}
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
      <div className="relative overflow-hidden bg-carbon-surface rounded-card p-5 flex flex-col gap-4">
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
      <div className="bg-carbon-surface rounded-card p-5 flex flex-col gap-4">
        <h2 className="text-sm font-semibold text-carbon-textSub uppercase tracking-widest">
          {t("snapshots.title")}
        </h2>
        {/* Safe-restore explainer */}
        <div className="rounded-card bg-statusInfoBg px-3 py-2.5 text-xs text-statusInfo leading-relaxed">
          {t("flash.restoreNote")}
        </div>

        <div className="flex flex-col gap-1">
          <div className="flex items-center gap-2">
            <span className="text-xs text-carbon-textMuted">{t("source.label")}</span>
            <SourceToggle source={source} onChange={setSource} disabled={loading} domain="flash" />
          </div>
          <p className="text-[11px] text-carbon-textMuted">{t("source.hint")}</p>
        </div>

        {loading && <p className="text-xs text-carbon-textMuted">{t("dashboard.checking")}</p>}
        {error && <p className="text-xs text-statusFail">{error}</p>}
        {!loading && !error && snapshots.length === 0 && (
          <p className="text-xs text-carbon-textMuted">{t("flash.none")}</p>
        )}
        {!loading && snapshots.length > 0 && (
          <div className="rounded-card bg-carbon-background px-3 py-1">
            {snapshots.map((snap) => (
              <FlashSnapshotRow key={snap.id} snap={snap} source={source} onDeleted={() => void load()} t={t} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
