// Files backs up arbitrary folders as file sets. It follows VMs.tsx: one card
// per set with an include-in-schedule switch, a backup button that watches the
// progress key "files:<name>", and a Backups panel that restores either in
// place (after a confirm) or into a folder. Sets are added and edited in a
// dialog with a folder picker and one exclude pattern per line.

import { useEffect, useRef, useState, type CSSProperties, type ReactNode } from "react";
import { checkRestoreOnce, restoreBlockReason, useRestoreCheck } from "../lib/useRestoreCheck";
import { RestoreCheckPanel } from "../components/restore/RestoreCheckPanel";
import { listFileSets, patchFileSet, deleteFileSet, deleteFileSetBackups, backupFileSet, backupFilesAll, restoreFileSet, listSnapshotFilesFileSet, restoreFileSetFiles, discoverFiles, getSettings, getFileSetPreset } from "../lib/api";
import type { AnomalyItem, ItemChecks, FileSetView, PlacementView, FileEntry, FileSetPresetResponse } from "../lib/api";
import { PageTitle } from "../components/PageTitle";
import { PlacementRow } from "../components/placement/PlacementRow";
import { subscribePlacement } from "../lib/placementEvents";
import { subscribeRepos } from "../lib/useNamedRepos";
import type { RepoSource } from "../components/SourceToggle";
import { PAGE_SHELL_RESPONSIVE } from "../lib/pageShell";
import { OffsiteIndicator } from "../components/OffsiteIndicator";
import { EffectiveScheduleLine } from "../components/EffectiveScheduleLine";
import { FolderBrowser } from "../components/FolderBrowser";
import { DEFAULT_RESTORE_FOLDER } from "../components/RestorePanel";
import { SnapshotFileTree } from "../components/SnapshotFileTree";
import { BackupCancelButton } from "../components/BackupCancelButton";
import { ProgressBar } from "../components/ProgressBar";
import { RecentRunsList } from "../components/RecentRunsList";
import { SizeBreakdown } from "../components/SizeBreakdown";
import { RestoreProgress } from "../components/restore/RestoreProgress";
import { EmptyStateIcon } from "../components/EmptyStateIcon";
import { IconBackupNow, IconFiles, IconPencil, IconTrash } from "../components/Sidebar";
import { BULK_HUE } from "../lib/bulkHue";
import { useT } from "../lib/i18n";
import { useProgress, anyActive, busyPhraseKey } from "../lib/progress";
import { useBackupWatch } from "../lib/backupWatch";
import { loadErrorMessage } from "../lib/errors";
import { useConfirm } from "../lib/useConfirm";
import { hueVars } from "../lib/appearance";
import { Selector, type SelectorItem } from "../components/Selector";
import { Badge } from "../components/Badge";
import { Button } from "../components/Button";
import { InfoBubble } from "../components/InfoBubble";
import { ToggleRow } from "./settings/shared";
import { CheckDraw } from "../components/CheckDraw";
import { useToast } from "../lib/toast";
import { IconRestore } from "../components/Sidebar";
import { IconDisclosure } from "../components/IconDisclosure";
import { SNAPSHOT_MISSING } from "../lib/timeline";
import { Timeline } from "../components/timeline/Timeline";
import { ItemAnomalyBadge } from "../components/ItemAnomalyBadge";
import { PauseButton, PausedBadge } from "../components/SchedulePause";
import { ItemChecksLine } from "../components/ItemChecksLine";
import { useItemChecks } from "../lib/useItemChecks";
import { useAnomalyItems, useAnomalySummary, useOpenAnomalies } from "../lib/useAnomalies";
import { useRestoreRequest, type RestoreRequest } from "../lib/restoreRequest";
import { FileSetDialog } from "../components/files/FileSetDialog";
import { FileSetFoldersEditor, fileSetEditorKey } from "../components/files/FileSetFoldersEditor";

type T = ReturnType<typeof useT>["t"];

function formatTs(unix: number | null | undefined): string {
  if (!unix) return "—";
  return new Date(unix * 1000).toLocaleString();
}

// FileSetEnabledToggle is the file-set copy of components/IncludeToggle.tsx.
function FileSetEnabledToggle({
  id,
  initial,
  onSaved,
}: {
  id: string;
  initial: boolean;
  onSaved?: (enabled: boolean) => void;
}) {
  const { t } = useT();
  const { push } = useToast();
  const [enabled, setEnabled] = useState(initial);
  const [busy, setBusy] = useState(false);
  const [shake, setShake] = useState(0);

  // Re-seed when the parent passes a fresh value (rows are keyed by id and do
  // not remount, so a list reload must reach the toggle).
  useEffect(() => setEnabled(initial), [initial]);

  async function handleChange(next: boolean) {
    setBusy(true);
    try {
      const res = await patchFileSet(id, { enabled: next });
      if (res.ok) {
        setEnabled(next);
        onSaved?.(next);
      } else {
        push(res.error ?? t("schedule.updateFailed"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("schedule.updateFailed"), "fail");
      setShake((n) => n + 1);
    } finally {
      setBusy(false);
    }
  }

  return (
    <ToggleRow
      label={t("files.enabled")}
      checked={enabled}
      onChange={(next) => void handleChange(next)}
      disabled={busy}
      shakeNonce={shake}
    />
  );
}

// FileSetBackupButton is a square icon badge like components/BackupButton.tsx.
// That one sits in a card corner and reports through toasts; this one has a
// column of its own, so the result stays below the trigger, where a backup
// result clears itself after a few seconds.
function FileSetBackupButton({
  set,
  t,
  onBackedUp,
  running,
}: {
  set: FileSetView;
  t: T;
  onBackedUp?: () => void;
  /** Whether another operation runs (anyActive). It blocks this backup, but
   *  not while this set's own backup is the one running. */
  running?: { active: boolean; phase?: string };
}) {
  const { state, fire, isPending } = useBackupWatch({
    progressKey: `files:${set.name}`,
    start: () => backupFileSet(set.id),
    matchRun: (r) => r.domain === "files" && r.target === set.name,
    onDone: onBackedUp,
  });
  const blockedByOther = !!running?.active && !isPending;
  // A discovered set without a path has nothing to back up until a folder is
  // set; restoring into a folder still works.
  const noPath = set.path === "";

  // The label stays fixed. Refusals and busy states go in the tooltip, in the
  // order BackupButton.tsx uses, plus the missing folder.
  const stateTip = noPath
    ? t("files.noPathHint")
    : isPending
      ? t("common.backingUp")
      : blockedByOther
        ? t(busyPhraseKey(running?.phase))
        : undefined;

  return (
    <div className="flex flex-col gap-1 items-end">
      <Button
        label={t("containers.backupNow")}
        labelKey="containers.backupNow"
        glyph={<IconBackupNow />}
        tone="accent"
        onClick={() => void fire()}
        disabled={isPending || blockedByOther || noPath}
        busy={isPending}
        title={stateTip}
      />
      {/* The "something else is running" note sits in FileSetRow, above the
          last-backup line it qualifies. */}
      {state.phase === "success" && (
        <span className="inline-flex items-center gap-1 text-xs text-statusOk">
          <CheckDraw />
          {t("common.done")}
          {state.snapshotId && (
            <span dir="ltr" className="font-mono ms-1 text-start text-carbon-textMuted">
              {state.snapshotId.slice(0, 8)}
            </span>
          )}
        </span>
      )}
      {state.phase === "error" && (
        <span className="text-xs text-statusFail max-w-[18rem] wrap-break-word text-end">
          {state.message}
        </span>
      )}
    </div>
  );
}

// FileSetFileBrowser restores ticked files and folders from a snapshot into a
// folder, like the container SnapshotFileBrowser. It never writes in place, so
// it needs no confirm.
function FileSetFileBrowser({
  set,
  snapshotId,
  source,
  hostMountRoot,
  restoreFolder,
  otherActive,
  onMissing,
  t,
}: {
  set: FileSetView;
  snapshotId: string;
  source: RepoSource;
  hostMountRoot: string;
  restoreFolder: string;
  otherActive: { active: boolean; phase?: string };
  onMissing: () => void;
  t: T;
}) {
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState("");
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [folder, setFolder] = useState(restoreFolder);
  const [restoredTarget, setRestoredTarget] = useState("");

  const progressKey = `files:${set.name}`;
  // One ref for useBackupWatch and, through RestoreProgress, the cancel button.
  const cancelledRef = useRef(false);
  const { state, fire, reset, isPending } = useBackupWatch({
    progressKey,
    kind: "restore",
    matchRun: (r) => r.domain === "files" && r.target === set.name,
    cancelledRef,
    start: async () => {
      const res = await restoreFileSetFiles(set.id, snapshotId, [...selected], folder.trim(), true, source);
      if (res.code === SNAPSHOT_MISSING) onMissing();
      if (res.ok) setRestoredTarget(res.target ?? "");
      return res;
    },
  });
  const prog = useProgress()[progressKey];
  const blockedByOther = otherActive.active && !isPending;
  const check = useRestoreCheck(
    selected.size > 0 && folder.trim()
      ? { kind: "fileSetFiles", name: set.id, snapshotId, source, paths: [...selected], targetPath: folder.trim() }
      : null
  );
  const idle = isPending || state.phase === "success";

  useEffect(() => {
    setLoading(true);
    listSnapshotFilesFileSet(set.id, snapshotId, source)
      .then((res) => {
        // The server's own reason, such as a stale repo lock, beats the
        // generic message.
        if (res.ok) setFiles(res.files ?? []);
        else setError(loadErrorMessage(res, t("files.loadFailed")));
      })
      .catch(() => setError(t("files.loadFailed")))
      .finally(() => setLoading(false));
  }, [set.id, snapshotId, source, t]);

  // A new selection or target clears the previous result.
  function toggle(p: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(p)) next.delete(p);
      else next.add(p);
      return next;
    });
    reset();
  }
  function pickFolder(v: string) {
    setFolder(v);
    reset();
  }

  function handleRestoreSelected() {
    if (selected.size === 0 || !folder.trim()) return;
    void fire();
  }

  const count = selected.size;

  return (
    <div className="mt-1 rounded-card bg-carbon-background p-2 flex flex-col gap-2">
      <p className="text-caption text-carbon-textMuted">{t("files.selectHint")}</p>
      <SnapshotFileTree
        files={files}
        loading={loading}
        error={error}
        filter={filter}
        onFilterChange={setFilter}
        selected={selected}
        onToggle={toggle}
        t={t}
      />

      {/* Target folder and restore action, once something is ticked. */}
      {count > 0 && (
        <div className="border-t border-carbon-border pt-2 flex flex-col gap-2">
          <FolderBrowser
            label={t("restore.targetPath")}
            value={folder}
            hostMountRoot={hostMountRoot}
            onChange={pickFolder}
          />
          {!idle && <RestoreCheckPanel check={check} t={t} />}
          <div className="flex items-center gap-2">
            <Button
              label={t("files.restoreSelected").replace("{n}", String(count))}
              labelKey="files.restoreSelected"
              glyph={<IconRestore />}
              tone="accent"
              onClick={handleRestoreSelected}
              disabled={isPending || blockedByOther || !folder.trim() || !check.ready}
              busy={isPending}
              title={isPending ? t("common.restoring") : undefined}
              hint={idle ? undefined : restoreBlockReason(check, t)}
              className="shrink-0"
            />
            {blockedByOther && (
              <span className="text-caption text-carbon-textMuted">{t(busyPhraseKey(otherActive.phase))}</span>
            )}
          </div>
          <RestoreProgress
            state={state}
            isPending={isPending}
            prog={prog}
            cancelKey={progressKey}
            inPlace={false}
            name={set.name}
            cancelledRef={cancelledRef}
            successMessage={
              restoredTarget
                ? t("restore.restoredTo").replace("{path}", restoredTarget)
                : t("files.restoreComplete")
            }
            t={t}
          />
        </div>
      )}
    </div>
  );
}

type RestoreDest = "original" | "folder" | "select";

function FileSetRestoreControl({
  set,
  snapshotId,
  source,
  hostMountRoot,
  restoreFolder,
  otherActive,
  onMissing,
  lead,
  t,
}: {
  set: FileSetView;
  snapshotId: string;
  source: RepoSource;
  hostMountRoot: string;
  restoreFolder: string;
  otherActive: { active: boolean; phase?: string };
  onMissing: () => void;
  /** The view's one accent restore goes to the timeline's lead row. */
  lead: boolean;
  t: T;
}) {
  // Without a path the server cannot restore in place, so only a folder works.
  const noPath = set.path === "";
  const [dest, setDest] = useState<RestoreDest>(noPath ? "folder" : "original");
  // Seeded from the global default restore folder, as in the container panel.
  const [targetPath, setTargetPath] = useState(restoreFolder);

  const progressKey = `files:${set.name}`;
  // One ref for useBackupWatch and, through RestoreProgress, the cancel button.
  const cancelledRef = useRef(false);
  const { state, fire, reset, isPending } = useBackupWatch({
    progressKey,
    kind: "restore",
    matchRun: (r) => r.domain === "files" && r.target === set.name,
    cancelledRef,
    start: async () => {
      const res = await restoreFileSet(set.id, snapshotId, true, dest === "folder" ? targetPath : "", source);
      if (res.code === SNAPSHOT_MISSING) onMissing();
      return res;
    },
  });
  const prog = useProgress()[progressKey];
  const blockedByOther = otherActive.active && !isPending;
  const { confirm, confirmDialog } = useConfirm();
  const [checking, setChecking] = useState(false);

  // An old result would describe another destination, so a new choice clears
  // it (a no-op while a restore runs).
  useEffect(() => reset(), [dest, targetPath, reset]);

  // Every snapshot row carries this control, so the pre-flight check runs on
  // the click rather than for each row. In place it always asks; into a folder
  // it only speaks up when the restore cannot work.
  async function handleRestore() {
    if (dest === "folder" && targetPath.trim() === "") return;
    setChecking(true);
    const check = await checkRestoreOnce({
      kind: "fileSet",
      name: set.id,
      snapshotId,
      source,
      targetPath: dest === "folder" ? targetPath.trim() : "",
    });
    setChecking(false);
    const refusal = restoreBlockReason(check, t);
    const question = dest === "original" ? t("files.restoreOriginalConfirm") : refusal;
    if (question !== undefined) {
      const extra = <RestoreCheckPanel check={check} t={t} />;
      if (!(await confirm(question, { extra, confirmBlocked: refusal }))) return;
    }
    void fire();
  }

  const destItems: SelectorItem[] = [
    { id: "original", label: t("files.restoreOriginal"), disabled: noPath, title: noPath ? t("files.noPathHint") : undefined },
    { id: "folder", label: t("files.restoreToFolder") },
    { id: "select", label: t("files.restoreSelectFiles") },
  ];

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-2 flex-wrap">
        {/* The group reuses the "Restore" label, since the items name each
            choice. buttonHeight gives the segments the height of the Restore
            button beside them. */}
        <Selector
          items={destItems}
          label={t("snapshots.restore")}
          select="one"
          active={dest}
          buttonHeight
          inline
          onChange={(id) => setDest(id as RestoreDest)}
          disabled={isPending}
        />
        {/* Selecting files brings its own controls in FileSetFileBrowser. */}
        {dest !== "select" && (
          <Button
            label={t("snapshots.restore")}
            labelKey="snapshots.restore"
            tone={lead ? "accent" : "neutral"}
            onClick={() => void handleRestore()}
            disabled={isPending || checking || blockedByOther || (dest === "folder" && targetPath.trim() === "")}
            busy={isPending || checking}
            title={isPending ? t("common.restoring") : undefined}
            className="shrink-0"
          />
        )}
        {blockedByOther && dest !== "select" && (
          <span className="text-caption text-carbon-textMuted shrink-0">
            {t(busyPhraseKey(otherActive.phase))}
          </span>
        )}
      </div>
      {/* Target folder picker for the non-destructive whole-set extract */}
      {dest === "folder" && (
        <FolderBrowser
          label={t("restore.targetPath")}
          value={targetPath}
          hostMountRoot={hostMountRoot}
          onChange={setTargetPath}
        />
      )}
      {dest !== "select" && (
        <RestoreProgress
          state={state}
          isPending={isPending}
          prog={prog}
          cancelKey={progressKey}
          inPlace={dest === "original"}
          name={set.name}
          cancelledRef={cancelledRef}
          successMessage={t("files.restoreComplete")}
          t={t}
        />
      )}
      {/* Selective restore: tick files/folders and restore just those to a folder. */}
      {dest === "select" && (
        <FileSetFileBrowser
          set={set}
          snapshotId={snapshotId}
          source={source}
          hostMountRoot={hostMountRoot}
          restoreFolder={restoreFolder}
          otherActive={otherActive}
          onMissing={onMissing}
          t={t}
        />
      )}
      {confirmDialog}
    </div>
  );
}

function FileSetRestorePanel({
  set,
  hostMountRoot,
  restoreFolder,
  t,
  onSetsChanged,
  trailing,
  preselect = "",
  preselectAt = 0,
}: {
  set: FileSetView;
  hostMountRoot: string;
  restoreFolder: string;
  t: T;
  /** Delete-all forgets the whole set, so the parent must reload the list. */
  onSetsChanged: () => void;
  /** The snapshot a finding's restore link asked for; the list starts open. */
  preselect?: string;
  /** When that snapshot was taken, in Unix seconds. */
  preselectAt?: number;
  /** A summary at the far end of the disclosure's row, always visible, as the
   *  container card shows its last backup there. */
  trailing?: ReactNode;
}) {
  const [open, setOpen] = useState(preselect !== "");
  const { flagged } = useOpenAnomalies();
  const [reloadTick, setReloadTick] = useState(0);
  const [deletingAll, setDeletingAll] = useState(false);
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [shakeDeleteAll, setShakeDeleteAll] = useState(0);
  const running = anyActive(useProgress());

  // "Delete all" empties the local place and forgets the set, which is what
  // the question it asks says; a copy at a target stays behind and no set
  // knows it any more. It fails as a toast, not inline. Bumping reloadTick
  // remounts the timeline below under a fresh key, so it reads the place
  // again instead of keeping the rows the delete just emptied.
  async function handleDeleteAll() {
    // TODO: name the stake in the confirm ("N snapshots, X GB"); this one
    // deletes every backup the set has. OrphanRemoveButton.tsx and VMs.tsx
    // need the same.
    if (!(await confirm(t("files.deleteBackupsConfirm"), { confirmKey: "snapshots.deleteAll" }))) return;
    setDeletingAll(true);
    deleteFileSetBackups(set.id)
      .then((res) => {
        if (!res.ok) {
          push(res.error ?? t("common.deleteBackupsFailed"), "fail");
          setShakeDeleteAll((n) => n + 1);
          setReloadTick((n) => n + 1);
          return;
        }
        // The set is forgotten with its snapshots, so the card has to go.
        onSetsChanged();
      })
      .catch(() => {
        push(t("common.deleteBackupsFailed"), "fail");
        setShakeDeleteAll((n) => n + 1);
        setReloadTick((n) => n + 1);
      })
      .finally(() => setDeletingAll(false));
  }

  return (
    <div className="mt-1">
      {/* Trigger left, summary right, class for class as in the container
          card's disclosure row. */}
      <div className="flex items-center gap-2 flex-wrap">
        <Button
          label={t("snapshots.title")}
          labelKey="snapshots.title"
          tone="neutral"
          onClick={() => setOpen((prev) => !prev)}
          glyph={<IconDisclosure open={open} />}
        />
        {trailing}
      </div>

      {open && (
        <div className="mt-2 rounded-card bg-carbon-background px-3 py-1">
          <RecentRunsList name={set.name} domain="files" t={t} />
          <SizeBreakdown domain="files" item={set.id} t={t} />
          <Timeline
            key={reloadTick}
            domain="files"
            itemKey={set.id}
            itemName={set.name}
            open={open}
            header={(rows) =>
              rows.some((r) => r.places.some((m) => m.place === "local")) && (
                <Button
                  key={shakeDeleteAll}
                  label={t("snapshots.deleteAll")}
                  labelKey="snapshots.deleteAll"
                  tone="neutral"
                  onClick={() => void handleDeleteAll()}
                  disabled={deletingAll}
                  busy={deletingAll}
                  title={deletingAll ? t("snapshots.deletingAll") : undefined}
                  className={`self-end my-1${shakeDeleteAll ? " glim-shake" : ""}`}
                />
              )
            }
            flagged={flagged}
            request={preselect ? { snapshot: preselect, at: preselectAt } : undefined}
            renderActions={(pick) => (
              <div className="basis-full">
                <FileSetRestoreControl
                  set={set}
                  snapshotId={pick.snapshotId}
                  source={pick.source}
                  hostMountRoot={hostMountRoot}
                  restoreFolder={restoreFolder}
                  otherActive={running}
                  onMissing={pick.onMissing}
                  lead={pick.lead}
                  t={t}
                />
              </div>
            )}
          />
        </div>
      )}
      {confirmDialog}
    </div>
  );
}

export function FileSetRow({
  set,
  hostMountRoot,
  restoreFolder,
  t,
  onRefresh,
  onEdit,
  onPlacement,
  index,
  anomaly,
  anomalyEnabled = false,
  restoreRequest,
  checks,
  onChecksChanged,
  onEnabledSaved,
}: {
  set: FileSetView;
  hostMountRoot: string;
  restoreFolder: string;
  t: T;
  onRefresh: () => void;
  onEdit: () => void;
  /** Takes the card view a placement change answered with. */
  onPlacement: (next: PlacementView) => void;
  /** Rainbow position by list index, not a hash of the id or name. */
  index: number;
  anomaly?: AnomalyItem;
  anomalyEnabled?: boolean;
  /** A finding's restore link for this set: the card opens its backups and
   *  comes into view. */
  restoreRequest?: RestoreRequest;
  checks?: ItemChecks;
  onChecksChanged?: () => void;
  /** Takes the include-in-schedule value the switch or the Pause button stored. */
  onEnabledSaved?: (enabled: boolean) => void;
}) {
  const progressMap = useProgress();
  const progress = progressMap[`files:${set.name}`];
  const running = anyActive(progressMap);
  const [removing, setRemoving] = useState(false);
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [shake, setShake] = useState(0);

  const noPath = set.path === "";
  const pathMissing = !noPath && !set.pathExists;
  const cardRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    // jsdom has no scrollIntoView.
    if (restoreRequest) cardRef.current?.scrollIntoView?.({ block: "start" });
  }, [restoreRequest]);

  async function handleRemove() {
    if (!(await confirm(t("files.deleteSetConfirm"), { confirmKey: "common.delete" }))) return;
    setRemoving(true);
    try {
      const res = await deleteFileSet(set.id);
      if (res.ok) onRefresh();
      else {
        push(res.error ?? t("common.removeFailed"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.removeFailed"), "fail");
      setShake((n) => n + 1);
    } finally {
      setRemoving(false);
    }
  }

  return (
    <div
      ref={cardRef}
      style={{ ...hueVars(index), "--row-i": String(index) } as CSSProperties}
      // glim-active while this set's own backup or restore runs, as in
      // ContainerRow and VMRow.
      className={`relative overflow-hidden bg-carbon-surface rounded-card p-4 flex flex-col gap-3 glim-hue glim-stagger-row ${
        progress?.active ? "glim-active" : ""
      }`}
    >
      {/* Top row: name, chips and path, with the action badges. The name keeps
          12rem before the badges go under it. */}
      <div className="flex items-start gap-3 flex-wrap">
        <div className="flex-1 basis-48 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="font-semibold text-carbon-text text-sm truncate">
              {set.name}
            </span>
            <ItemAnomalyBadge item={anomaly} enabled={anomalyEnabled} t={t} />
            {!set.enabled && <PausedBadge />}
            {set.excludes.length > 0 && (
              <Badge tone="neutral" wrap>
                {t("files.excludesCount").replace("{n}", String(set.excludes.length))}
              </Badge>
            )}
            {/* Source-folder problems, loudest first: no folder at all (discovered
                set), then folder configured but missing on disk. */}
            {noPath && (
              <Badge tone="warn" wrap title={t("files.noPathHint")}>
                {t("files.noPath")}
              </Badge>
            )}
            {pathMissing && (
              <Badge tone="fail" wrap>
                {t("files.pathMissing")}
              </Badge>
            )}
          </div>
          {!noPath && (
            <p dir="ltr" className="mt-1 text-xs font-mono text-carbon-textMuted truncate text-start">
              {hostMountRoot}/{set.path}
            </p>
          )}
          {noPath && (
            <p className="mt-1 text-xs text-carbon-textMuted">{t("files.noPathHint")}</p>
          )}
        </div>

        {/* Action badges in the top-right corner, as on the container card;
            the last backup sits beside the Backups trigger instead. Backup
            leads Edit and Delete, since it is what the card is for. Pause
            sits before it, because the forward action goes last. */}
        <div className="ms-auto flex min-w-0 items-start justify-end gap-1.5 flex-wrap max-md:w-full">
          <PauseButton
            paused={!set.enabled}
            save={(include) => patchFileSet(set.id, { enabled: include })}
            onSaved={onEnabledSaved}
          />
          <FileSetBackupButton set={set} t={t} onBackedUp={onRefresh} running={running} />
          {/* Just the verb: the card already names the set. files.editSet
              stays the dialog heading, where the set has to be named. */}
          <Button
            label={t("common.edit")}
            labelKey="common.edit"
            glyph={<IconPencil />}
            tone="accent"
            title={t("files.editSet")}
            onClick={onEdit}
          />
          <Button
            key={shake}
            label={t("common.delete")}
            labelKey="common.delete"
            glyph={<IconTrash />}
            tone="accent"
            title={t("files.deleteSet")}
            onClick={() => void handleRemove()}
            disabled={removing}
            className={shake ? "glim-shake" : ""}
          />
        </div>
      </div>

      {/* The schedule toggle, flush right below the badges, as on the
          container card. */}
      <div className="flex items-start">
        <div className="ms-auto flex flex-col items-end gap-2">
          <FileSetEnabledToggle id={set.id} initial={set.enabled} onSaved={onEnabledSaved} />
        </div>
      </div>

      {/* What the toggle above means for this set, in the same
          server-computed sentence as the Schedules card. */}
      <EffectiveScheduleLine effective={set.effectiveSchedule} />

      <PlacementRow
        item={{ domain: "files", key: set.id }}
        name={set.name}
        view={set.placement}
        onView={onPlacement}
      />

      <ItemChecksLine checks={checks} hasBackup={!!set.lastBackup} onChanged={onChecksChanged} />

      {/* Backups disclosure with the last-backup date on its row, as on the
          container card. */}
      <FileSetRestorePanel
        set={set}
        hostMountRoot={hostMountRoot}
        restoreFolder={restoreFolder}
        t={t}
        onSetsChanged={onRefresh}
        preselect={restoreRequest?.snapshot}
        preselectAt={restoreRequest?.at}
        trailing={
          // A column, so the note that something else is running sits above
          // the date it qualifies. At rest only the date shows.
          <span className="ms-auto shrink-0 flex flex-col items-end text-xs whitespace-nowrap">
            {running.active && !progress?.active && (
              <span className="text-carbon-textMuted">{t(busyPhraseKey(running.phase))}</span>
            )}
            <span className="text-carbon-textMuted">
              {`${t("containers.lastBackup")}: ${
                set.lastBackup ? formatTs(set.lastBackup) : t("containers.never")
              }`}
            </span>
          </span>
        }
      />

      {/* Keyed by fileSetEditorKey, because the editor seeds from its
          mount-time props and has to remount after a path edit. */}
      {!noPath && (
        <FileSetFoldersEditor
          key={fileSetEditorKey(set)}
          set={set}
          hostMountRoot={hostMountRoot}
          t={t}
          anomaly={anomaly}
          anomalyEnabled={anomalyEnabled}
        />
      )}

      {/* Pinned to the card's bottom edge. */}
      {progress && (
        <ProgressBar
          percent={progress.percent}
          active={progress.active}
          label={progress.phase === "restore" ? t("common.restoring") : t("common.backingUp")}
        />
      )}
      {/* Stops a running backup, next to the bar that shows it. A restore has
          its own cancel in the Backups panel, with its own warning about a
          half-restored target. */}
      {progress && progress.active && progress.phase !== "restore" && (
        <div className="flex justify-end">
          <BackupCancelButton cancelKey={`files:${set.name}`} name={set.name} t={t} />
        </div>
      )}
      {confirmDialog}
    </div>
  );
}

export function Files() {
  const { t } = useT();
  const anomalies = useAnomalyItems();
  const anomalyEnabled = useAnomalySummary().summary?.enabled ?? false;
  const itemChecks = useItemChecks();
  const restoreRequest = useRestoreRequest();
  const { push } = useToast();
  // Any backup, restore or replication in flight disables the bulk buttons.
  const running = anyActive(useProgress());
  const [sets, setSets] = useState<FileSetView[]>([]);
  const [hostMountRoot, setHostMountRoot] = useState("/host/user");
  const [restoreFolder, setRestoreFolder] = useState(DEFAULT_RESTORE_FOLDER);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  // null = closed; "new" = create dialog; a view = edit dialog for that set.
  const [dialog, setDialog] = useState<"new" | FileSetView | null>(null);
  // Pre-fill for the create dialog when it was opened through "Add preset:
  // Host system config"; null for a plain "Add folder set".
  const [presetSeed, setPresetSeed] = useState<{
    name: string;
    path: string;
    excludes: string[];
  } | null>(null);
  // The "Host system config" preset for this platform. It stays null until
  // loaded or after a failed fetch, and the preset button stays hidden.
  const [preset, setPreset] = useState<FileSetPresetResponse | null>(null);
  const [discovering, setDiscovering] = useState(false);
  const [shakeDiscover, setShakeDiscover] = useState(0);
  const [backupAllBusy, setBackupAllBusy] = useState(false);
  const [shakeBackupAll, setShakeBackupAll] = useState(0);

  // Only the newest read may land; an older answer arriving late would undo the
  // placement a card wrote while it was in flight.
  const read = useRef(0);

  function loadSets() {
    const n = ++read.current;
    return listFileSets()
      .then((res) => {
        if (n !== read.current) return;
        if (res.ok) {
          setSets(res.fileSets ?? []);
          // Clear the message of an earlier failed load.
          setError(null);
        } else setError(res.error ?? t("files.loadSetsFailed"));
      })
      .catch(() => {
        if (n === read.current) setError(t("files.loadSetsFailed"));
      });
  }

  useEffect(() => {
    // Loading waits for both fetches: the restore controls seed their target
    // folder from restoreFolder once at mount, so they must not mount before
    // the settings arrive.
    const sets = loadSets();
    const settings = getSettings()
      .then((res) => {
        if (res.hostMountRoot) setHostMountRoot(res.hostMountRoot);
        if (res.settings?.restoreFolder) setRestoreFolder(res.settings.restoreFolder);
      })
      .catch(() => undefined);
    // A slow or failed preset lookup never holds up the page; the preset
    // button just stays hidden.
    void getFileSetPreset()
      .then((res) => {
        if (res.ok) setPreset(res);
      })
      .catch(() => undefined);
    void Promise.all([sets, settings]).finally(() => setLoading(false));
  }, []); // eslint-disable-line react-hooks/exhaustive-deps -- t() is only read to build a failure message; re-fetching on a language switch would be a wasted round-trip

  // loadSets is stable for this page's lifetime; as a dependency it would
  // subscribe again on every render.
  useEffect(() => {
    const offs = [subscribeRepos(() => void loadSets()), subscribePlacement(() => void loadSets())];
    return () => offs.forEach((off) => off());
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  function placeSet(id: string, next: PlacementView) {
    setSets((prev) => prev.map((s) => (s.id === id ? { ...s, placement: next } : s)));
  }

  // The switch and the Pause button store the same flag, and both read it back
  // from this list. The reload brings the schedule sentence, which the server
  // words from the flag.
  function enabledSaved(id: string, enabled: boolean) {
    setSets((prev) => prev.map((s) => (s.id === id ? { ...s, enabled } : s)));
    void loadSets();
  }

  /** Opens the create dialog pre-filled with the "Host system config" preset,
   *  once it has loaded and is offered for this platform. */
  function handleAddPreset() {
    if (!preset?.offered) return;
    setPresetSeed({ name: preset.name, path: preset.path, excludes: preset.excludes });
    setDialog("new");
  }

  /** Opens a blank create dialog, so no preset seed from an earlier open
   *  leaks in. */
  function handleAddBlank() {
    setPresetSeed(null);
    setDialog("new");
  }

  async function handleDiscover() {
    setDiscovering(true);
    try {
      const res = await discoverFiles();
      // Both outcomes reload the list. The named repositories are searched
      // before the domain's own, so a failed pass can still have written rows,
      // and this page does not poll.
      if (res.skipped?.length) {
        // "+0" looks the same whether everything was read or a repository was
        // switched off, unresolvable or on a share that did not mount.
        push(t("common.discoverSkipped").replace("{list}", res.skipped.join(", ")), "warn");
      }
      if (res.ok) {
        push(`+${res.discovered ?? 0}`, "success");
      } else {
        push(res.error ?? t("common.discoverFailed"), "fail");
        setShakeDiscover((n) => n + 1);
      }
      await loadSets();
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.discoverFailed"), "fail");
      setShakeDiscover((n) => n + 1);
    } finally {
      setDiscovering(false);
    }
  }

  // "Back up all now" starts the server-side batch (batch:files) for every
  // enabled set with a source folder; progress shows on the cards.
  const backupableIds = sets.filter((s) => s.enabled && s.path !== "").map((s) => s.id);

  // The empty state has its own Add buttons, so the header ones wait for the
  // first set.
  const showEmptyState = !loading && !error && sets.length === 0;

  async function handleBackupAll() {
    setBackupAllBusy(true);
    try {
      const res = await backupFilesAll(backupableIds);
      if (res.ok) {
        push(t("containers.batchStarted"), "success");
      } else {
        push(res.error ?? t("settings.error"), "fail");
        setShakeBackupAll((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      setShakeBackupAll((n) => n + 1);
    } finally {
      setBackupAllBusy(false);
    }
  }

  return (
    <div className={PAGE_SHELL_RESPONSIVE}>
      {/* Heading with Discover, for disaster recovery, and the Add actions. */}
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div>
          <PageTitle>{t("files.title")}</PageTitle>
          <OffsiteIndicator domain="files" />
        </div>
        <div className="flex min-w-0 max-w-full items-center justify-end gap-2 flex-wrap max-md:w-full max-md:justify-start">
          <Button
            key={shakeDiscover}
            label={t("containers.discover")}
            labelKey="containers.discover"
            tone="neutral"
            onClick={() => void handleDiscover()}
            disabled={discovering}
            busy={discovering}
            title={t("files.discoverHint")}
            className={shakeDiscover ? "glim-shake" : ""}
          />
          {/* Offered on generic hosts and TrueNAS only; Unraid has the flash
              domain for host config. */}
          {!showEmptyState && preset?.offered && (
            <Button
              label={t("files.addPreset")}
              labelKey="files.addPreset"
              tone="neutral"
              onClick={handleAddPreset}
              title={t("files.addPresetHint")}
            />
          )}
          {!showEmptyState && (
            <Button
              label={t("files.addSet")}
              labelKey="files.addSet"
              tone="accent"
              onClick={handleAddBlank}
            />
          )}
        </div>
      </div>

      {loading && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
      )}
      {error && <p className="text-sm text-statusFail">{error}</p>}

      {/* Hue 0 cannot collide with a card's, since this only shows while the
          list is empty. glim-hue gives the Add buttons the accent, which
          glim-notch-card alone does not. The card centres its content, which
          collapses the h2 to nothing, so insetStart={6} places the notch
          against the card itself. */}
      {showEmptyState && (
        <div
          className="relative glim-notch-card glim-hue bg-carbon-surface rounded-card p-6 text-center flex flex-col items-center gap-3"
          style={hueVars(0) as CSSProperties}
        >
          <h2 className="flex items-center">
            <Badge tone="heading" size="heading" wrap hueIndex={0} insetStart={6}>
              {t("files.setsTitle")}
              <InfoBubble tip={t("files.empty")} onAccent />
            </Badge>
          </h2>
          <EmptyStateIcon icon={IconFiles} />
          <div className="flex items-center gap-2 flex-wrap justify-center">
            {preset?.offered && (
              <Button
                label={t("files.addPreset")}
                labelKey="files.addPreset"
                tone="neutral"
                onClick={handleAddPreset}
                title={t("files.addPresetHint")}
              />
            )}
            <Button
              label={t("files.addSet")}
              labelKey="files.addSet"
              tone="accent"
              onClick={handleAddBlank}
            />
          </div>
        </div>
      )}

      {!loading && sets.length > 0 && (
        <div className="flex items-center gap-3 flex-wrap">
          <Button
            key={shakeBackupAll}
            label={t("files.backupAll")}
            labelKey="files.backupAll"
            hueIndex={BULK_HUE.backup}
            tone="accent"
            onClick={() => void handleBackupAll()}
            disabled={backupAllBusy || running.active || backupableIds.length === 0}
            className={shakeBackupAll ? "glim-shake" : ""}
          />
          {!backupAllBusy && running.active && (
            <span className="text-xs text-carbon-textMuted">
              {t(busyPhraseKey(running.phase))}
            </span>
          )}
        </div>
      )}

      {!loading && sets.length > 0 && (
        <div className="flex flex-col gap-3 glim-content-fade">
          {sets.map((s, i) => (
            <FileSetRow
              key={s.id}
              set={s}
              hostMountRoot={hostMountRoot}
              restoreFolder={restoreFolder}
              t={t}
              onRefresh={() => void loadSets()}
              onEdit={() => setDialog(s)}
              onPlacement={(next) => placeSet(s.id, next)}
              index={i}
              anomaly={anomalies.find("files", s.id)}
              anomalyEnabled={anomalyEnabled}
              checks={itemChecks.find("files", s.id)}
              onChecksChanged={itemChecks.reload}
              restoreRequest={restoreRequest.item === s.name ? restoreRequest : undefined}
              onEnabledSaved={(enabled) => enabledSaved(s.id, enabled)}
            />
          ))}
        </div>
      )}

      {dialog !== null && (
        <FileSetDialog
          initial={dialog === "new" ? null : dialog}
          presetSeed={dialog === "new" ? presetSeed : null}
          hostMountRoot={hostMountRoot}
          t={t}
          onClose={() => {
            setDialog(null);
            setPresetSeed(null);
          }}
          onSaved={() => {
            setDialog(null);
            setPresetSeed(null);
            void loadSets();
          }}
        />
      )}
    </div>
  );
}
