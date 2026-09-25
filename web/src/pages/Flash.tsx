import { useEffect, useRef, useState, type CSSProperties } from "react";
import { hueVars } from "../lib/appearance";
import { backupFlashNow, listFlashSnapshots, flashDownloadURL, deleteSnapshot } from "../lib/api";
import type { Snapshot } from "../lib/api";
import { useT } from "../lib/i18n";
import { PAGE_SHELL } from "../lib/pageShell";
import { BackupCancelButton } from "../components/BackupCancelButton";
import { ProgressBar } from "../components/ProgressBar";
import { useProgress, anyActive, busyPhraseKey } from "../lib/progress";
import { useBackupWatch } from "../lib/backupWatch";
import { SourceToggle, type RepoSource } from "../components/SourceToggle";
import { OffsiteIndicator } from "../components/OffsiteIndicator";
import { useConfirm } from "../lib/useConfirm";
import { Badge } from "../components/Badge";
import { Button } from "../components/Button";
import { useToast } from "../lib/toast";
import { FlashZipExportCard } from "./settings/FlashZipExportCard";
import { InfoBubble } from "../components/InfoBubble";
import { IconBackupNow, IconDownload, IconTrash } from "../components/Sidebar";
import { tLtr } from "../lib/ltrFragments";
import { ItemAnomalyBadge } from "../components/ItemAnomalyBadge";
import { ItemAnomalySettings } from "../components/ItemAnomalySettings";
import { MissingRestorePoint, restorePointOf } from "../components/restore/MissingRestorePoint";
import { findingSnapshotId } from "../lib/anomalies";
import { useAnomalyItems, useAnomalySummary, useOpenAnomalies } from "../lib/useAnomalies";
import { useRestoreRequest } from "../lib/restoreRequest";

type T = ReturnType<typeof useT>["t"];

// FlashBackupButton is a square icon badge, like Containers.tsx's
// BackupButton. A glyph leaves no room for inline results, so success and
// failure arrive as toasts and a failure also shakes the button.
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
  // The backup runs detached on the server, so the outcome comes from the
  // "flash" progress and the recorded run rather than from the POST.
  const { state, fire, isPending } = useBackupWatch({
    progressKey: "flash",
    start: () => backupFlashNow(),
    matchRun: (r) => r.domain === "flash",
    onDone: onBackedUp,
  });
  // A backup/restore/replication elsewhere blocks a new flash backup.
  const blockedByOther = externallyBusy && !isPending;
  const { push } = useToast();
  const [shake, setShake] = useState(0);
  // The last phase already reported, so each transition toasts once. The
  // phase starts at "idle", so nothing fires on mount.
  const seenPhase = useRef(state.phase);

  useEffect(() => {
    if (state.phase === seenPhase.current) return;
    seenPhase.current = state.phase;
    if (state.phase === "success") {
      push(
        state.snapshotId ? `${t("settings.saved")} · ${state.snapshotId.slice(0, 8)}` : t("settings.saved"),
        "success"
      );
    } else if (state.phase === "error") {
      push(state.message, "fail");
      setShake((n) => n + 1);
    }
  }, [state, push, t]);

  // The label stays fixed; busy states only show in the tooltip.
  const stateTip = isPending
    ? t("flash.backingUp")
    : blockedByOther
      ? t(busyPhraseKey(busyPhase))
      : undefined;

  return (
    <Button
      key={shake}
      label={t("flash.backupNow")}
      labelKey="flash.backupNow"
      glyph={<IconBackupNow />}
      tone="accent"
      onClick={() => void fire()}
      disabled={isPending || blockedByOther}
      busy={isPending}
      title={stateTip}
      className={shake ? "glim-shake" : ""}
    />
  );
}

// How long the download button spins after a click. The server sends nothing
// until it has dumped and recompressed the whole snapshot (dumpFlashZipCompat),
// and the browser has no event to wait for, so this is a fixed guess.
const DOWNLOAD_PREPARING_MS = 20_000;

function FlashSnapshotRow({
  snap,
  source,
  flagged,
  preselected,
  onDeleted,
  t,
}: {
  snap: Snapshot;
  source: RepoSource;
  /** An open data-loss finding was raised on this snapshot. */
  flagged: boolean;
  /** A finding's restore link asked for this snapshot, so the row stands out
   *  from its neighbours. */
  preselected: boolean;
  onDeleted: () => void;
  t: T;
}) {
  const [deleting, setDeleting] = useState(false);
  const [preparing, setPreparing] = useState(false);
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [shake, setShake] = useState(0);

  async function handleDelete() {
    if (!(await confirm(t("snapshots.deleteConfirm"), { confirmKey: "snapshots.delete" }))) return;
    setDeleting(true);
    try {
      const res = await deleteSnapshot("flash", snap.id, source);
      if (res.ok) onDeleted();
      else {
        push(res.error ?? t("common.deleteFailed"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.deleteFailed"), "fail");
      setShake((n) => n + 1);
    } finally {
      setDeleting(false);
    }
  }

  // A native <a download> leaves progress to the browser's download manager,
  // which survives this row unmounting on a tab switch. It gives up the JSON
  // error fetch() could show before the stream starts; a flash zip is large
  // and rarely fails.
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
    // py-1.5 keeps the row at 44px around the 32px icon badges, as in
    // RestorePanel's SnapshotRow.
    <div
      className={`flex flex-col gap-1 py-1.5 border-b border-carbon-border last:border-0${
        preselected ? " bg-carbon-surface2 px-2 rounded-control" : ""
      }`}
    >
      <div className="flex items-center gap-3 text-sm">
        <span dir="ltr" className="font-mono text-start text-carbon-text text-xs w-20 shrink-0">{snap.id.slice(0, 8)}</span>
        <span className="text-carbon-textMuted text-xs flex-1">
          {new Date(snap.time).toLocaleString()}
        </span>
        {flagged && (
          <Badge tone="fail" size="small">
            {t("anomaly.snapshotFlagged")}
          </Badge>
        )}
        {/* No hueIndex: both badges take the Restore card's hue through the
            cascade. Delete gets no colour of its own, since a neutral badge
            would sit flat grey beside a hued one; the glyph, the tip and the
            confirm dialog carry its meaning. */}
        <Button
          label={t("flash.download")}
          labelKey="flash.download"
          glyph={<IconDownload />}
          tone="accent"
          onClick={handleDownload}
          disabled={preparing}
          busy={preparing}
          className={"shrink-0"}
        />
        <Button
          key={shake}
          label={t("snapshots.delete")}
          labelKey="snapshots.delete"
          glyph={<IconTrash />}
          tone="accent"
          onClick={() => void handleDelete()}
          disabled={deleting || preparing}
          className={`shrink-0${shake ? " glim-shake" : ""}`}
        />
      </div>
      {confirmDialog}
    </div>
  );
}

export function Flash() {
  const { t } = useT();
  const anomaly = useAnomalyItems().find("flash", "flash");
  const anomalyEnabled = useAnomalySummary().summary?.enabled ?? false;
  const { flagged } = useOpenAnomalies();
  const restoreRequest = useRestoreRequest();
  const [source, setSource] = useState<RepoSource>("local");
  const [snapshots, setSnapshots] = useState<Snapshot[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const progressMap = useProgress();
  const progress = progressMap["flash"];
  // Any backup, restore or replication in flight disables the backup button
  // up front instead of waiting for the server's 409.
  const running = anyActive(progressMap);
  const [reloadTick, setReloadTick] = useState(0);
  const reload = () => setReloadTick((n) => n + 1);

  // A switch of source hides the list until the new one is in (the toggle sets
  // `loading`); a reload after a backup or a delete swaps it in place. An
  // answer that arrives after the next switch is dropped, and a failure
  // empties the list, whose rows belong to the source just left. t() only
  // builds the failure message, so a language switch fetches nothing.
  useEffect(() => {
    let current = true;
    const fail = (message: string) => {
      if (!current) return;
      setSnapshots([]);
      setError(message);
    };
    setError(null);
    listFlashSnapshots(source)
      .then((res) => {
        if (!res.ok) return fail(res.error ?? t("flash.loadBackupsFailed"));
        if (current) setSnapshots(res.snapshots ?? []);
      })
      .catch((err: unknown) => fail(err instanceof Error ? err.message : t("flash.loadBackupsFailed")))
      .finally(() => {
        if (current) setLoading(false);
      });
    return () => {
      current = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [source, reloadTick]);

  return (
    // The OffsiteIndicator sits inside the heading div, so the shell gap alone
    // spaces the heading and the cards.
    <div className={PAGE_SHELL}>
      <div>
        <div className="flex flex-wrap items-center gap-2">
          <h1 className="text-2xl font-semibold text-carbon-text">{t("flash.title")}</h1>
          <ItemAnomalyBadge item={anomaly} enabled={anomalyEnabled} t={t} />
        </div>
        <p className="mt-1 text-sm text-carbon-textSub">{tLtr(t, "flash.subtitle")}</p>
        <div className="mt-2"><OffsiteIndicator domain="flash" /></div>
      </div>

      {/* The outer div holds the heading badge, so it carries the notch hover
          zone and the hue, and stays unpadded so its top edge matches the
          inner box for the badge's top-0. insetStart={5} lines the badge up
          with the inner p-5. The inner box is what ProgressBar clips to, as
          in Config.tsx's backup card. */}
      <div className="relative glim-notch-card glim-hue" style={hueVars(0) as CSSProperties}>
        <h2 className="flex items-center">
          <Badge tone="heading" size="heading" wrap hueIndex={0} insetStart={5}>
            {t("flash.backupTitle")}
            <InfoBubble tip={tLtr(t, "flash.backupHint")} onAccent />
          </Badge>
        </h2>
        <div className="relative overflow-hidden bg-carbon-surface rounded-card p-5 flex flex-col gap-4">
          <div className="flex justify-end">
            <FlashBackupButton
              t={t}
              onBackedUp={reload}
              externallyBusy={running.active}
              busyPhase={running.phase}
            />
          </div>
          <ItemAnomalySettings item={anomaly} enabled={anomalyEnabled} t={t} />

          {/* As on the Folders page: a restore has its own control with its
              own warning. */}
          {progress && progress.active && progress.phase !== "restore" && (
            <div className="flex justify-end">
              <BackupCancelButton cancelKey={"flash"} name={t("nav.flash")} t={t} />
            </div>
          )}

          {/* Pinned to the card's bottom edge. */}
          {progress && (
            <ProgressBar percent={progress.percent} active={progress.active} />
          )}
        </div>
      </div>

      {/* The snapshot rows' badges take this card's hue through the cascade. */}
      <div
        className="relative glim-notch-card glim-hue bg-carbon-surface rounded-card p-5 flex flex-col gap-4"
        style={hueVars(1) as CSSProperties}
      >
        <h2 className="flex items-center">
          <Badge tone="heading" size="heading" wrap hueIndex={1}>
            {t("snapshots.title")}
            <InfoBubble tip={tLtr(t, "flash.restoreNote")} onAccent />
          </Badge>
        </h2>

        <div className="flex items-center gap-2">
          <span className="flex items-center gap-1 text-xs text-carbon-textMuted">
            {t("source.label")}
            <InfoBubble tip={t("source.hint")} />
          </span>
          <SourceToggle
            source={source}
            onChange={(next) => {
              // The selector reports a click on the active source too, and
              // no load would follow to clear `loading`.
              if (next === source) return;
              setLoading(true);
              setSource(next);
            }}
            disabled={loading}
            domain="flash"
          />
        </div>

        {loading && <p className="text-xs text-carbon-textMuted">{t("dashboard.checking")}</p>}
        {error && <p className="text-xs text-statusFail">{error}</p>}
        {!loading && !error && (
          <MissingRestorePoint
            requested={restoreRequest.snapshot}
            requestedAt={restoreRequest.at}
            points={snapshots.map(restorePointOf)}
            t={t}
          />
        )}
        {!loading && !error && snapshots.length === 0 && (
          <p className="text-xs text-carbon-textMuted">{t("flash.none")}</p>
        )}
        {!loading && snapshots.length > 0 && (
          <div className="rounded-card bg-carbon-background px-3 py-1">
            {snapshots.map((snap) => (
              <FlashSnapshotRow
                key={snap.id}
                snap={snap}
                source={source}
                flagged={flagged.has(findingSnapshotId(snap))}
                preselected={findingSnapshotId(snap) === restoreRequest.snapshot}
                onDeleted={reload}
                t={t}
              />
            ))}
          </div>
        )}
      </div>

      {/* This page numbers its notches by hand: backup 0, restore 1, this 2. */}
      <FlashZipExportCard t={t} hueIndex={2} />
    </div>
  );
}
