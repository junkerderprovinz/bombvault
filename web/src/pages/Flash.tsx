import { useEffect, useState } from "react";
import { backupFlashNow, listFlashSnapshots, flashDownloadURL, deleteSnapshot } from "../lib/api";
import type { Snapshot } from "../lib/api";
import { useT } from "../lib/i18n";
import { ProgressBar } from "../components/ProgressBar";
import { useProgress, anyActive, busyPhraseKey } from "../lib/progress";
import { useBackupWatch } from "../lib/backupWatch";
import { SourceToggle, type RepoSource } from "../components/SourceToggle";
import { OffsiteIndicator } from "../components/OffsiteIndicator";
import { useConfirm } from "../lib/useConfirm";
import { Badge } from "../components/Badge";
import { useToast } from "../lib/toast";

type T = ReturnType<typeof useT>["t"];

// ---------------------------------------------------------------------------
// Backup button
// ---------------------------------------------------------------------------

// GlimStone follow-up pass (v8.0.0) audit note: the state.phase "success"/
// "error" result below is deliberately NOT migrated to a toast. Same shared-
// hook reasoning as Containers.tsx's BackupButton / VMs.tsx's VMBackupButton
// / Config.tsx's ConfigBackupButton: it's driven by lib/backupWatch.ts's
// useBackupWatch hook (kind defaults to "backup", which already self-clears
// after 4s — SUCCESS_CLEAR_MS, effectively already toast-like), but the
// identical state shape also backs RESTORE outcomes elsewhere, which are
// explicitly STICKY BY DESIGN. Splitting that shared, cross-file state
// machine's rendering by kind is a hook-level architecture change, not the
// local flash-swap this pass does everywhere else — left as its own
// deliberate follow-up.
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
            <span dir="ltr" className="font-mono ms-1 text-start text-carbon-textMuted">{state.snapshotId.slice(0, 8)}</span>
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
  const [preparing, setPreparing] = useState(false);
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();

  async function handleDelete() {
    if (!(await confirm(t("snapshots.deleteConfirm")))) return;
    setDeleting(true);
    try {
      const res = await deleteSnapshot("flash", snap.id, source);
      if (res.ok) onDeleted();
      else push(res.error ?? "Delete failed", "fail");
    } catch (err) {
      push(err instanceof Error ? err.message : "Delete failed", "fail");
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
        <span dir="ltr" className="font-mono text-start text-carbon-text text-xs w-20 shrink-0">{snap.id.slice(0, 8)}</span>
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
      {confirmDialog}
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

      {/* Backup card. GlimStone follow-up pass ("half-overlap card notch"):
          split into an outer structural `relative` div (hosting the heading
          Badge, now `position: absolute`) + this same inner
          `relative overflow-hidden` div (unchanged, still the box
          ProgressBar.tsx documents clipping itself to) — see Config.tsx's
          identical backup-card split and Badge.tsx's badgeClassName
          comment. */}
      {/* `glim-notch-card` on this OUTER div, not the inner overflow-hidden
          box: the badge itself lives here (see the split's own comment
          above), so this is the element that has to be the hover/focus zone
          for index.css's card-wide reactive-hover rule — see Settings.tsx's
          Card() for the full reasoning. */}
      <div className="relative glim-notch-card">
        {/* Task 5 (rule 11): heading is now a filled Badge, not bare eyebrow text. */}
        <h2 className="flex items-center">
          <Badge tone="heading" size="heading" wrap hueIndex={0}>{t("flash.backupTitle")}</Badge>
        </h2>
        <div className="relative overflow-hidden bg-carbon-surface rounded-card p-5 flex flex-col gap-4">
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
      </div>

      {/* Restore card. `glim-notch-card`: see Settings.tsx's Card() for the
          reasoning. */}
      <div className="relative glim-notch-card bg-carbon-surface rounded-card p-5 flex flex-col gap-4">
        <h2 className="flex items-center">
          <Badge tone="heading" size="heading" wrap hueIndex={1}>{t("snapshots.title")}</Badge>
        </h2>
        {/* Safe-restore explainer */}
        {/* Task 7: was bg-statusInfoBg/text-statusInfo (the old fifth hue) —
            same reasoning as Config.tsx's snapshotsHint banner: pure
            informational prose, folds into --status-neutral-*. */}
        <div className="rounded-card bg-statusNeutralBg px-3 py-2.5 text-xs text-statusNeutral leading-relaxed">
          {t("flash.restoreNote")}
        </div>

        <div className="flex flex-col gap-1">
          <div className="flex items-center gap-2">
            <span className="text-xs text-carbon-textMuted">{t("source.label")}</span>
            <SourceToggle source={source} onChange={setSource} disabled={loading} domain="flash" />
          </div>
          <p className="text-caption text-carbon-textMuted">{t("source.hint")}</p>
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
