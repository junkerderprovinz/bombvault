// Files backs up arbitrary folders as file sets. It follows VMs.tsx: one card
// per set with an include-in-schedule switch, a backup button that watches the
// progress key "files:<name>", and a Backups panel that restores either in
// place (after a confirm) or into a folder. Sets are added and edited in a
// dialog with a folder picker and one exclude pattern per line.

import { useEffect, useId, useRef, useState, type CSSProperties, type ReactNode } from "react";
import { createPortal } from "react-dom";
import {
  listFileSets,
  createFileSet,
  patchFileSet,
  deleteFileSet,
  deleteFileSetBackups,
  backupFileSet,
  backupFilesAll,
  fileSetSnapshots,
  restoreFileSet,
  listSnapshotFilesFileSet,
  restoreFileSetFiles,
  discoverFiles,
  deleteSnapshot,
  getSettings,
  getFileSetPreset,
} from "../lib/api";
import type { AnomalyItem, BrowseResponse, FileSetView, Snapshot, FileEntry, FileSetPresetResponse } from "../lib/api";
import { applyToggle, browseRelToHost, splitFlatSet, toFlatList } from "../lib/selectionTree";
import { SelectionTree } from "../components/SelectionTree";
import { RepoPicker } from "../components/RepoPicker";
import { SourceToggle, type RepoSource } from "../components/SourceToggle";
import { PAGE_SHELL } from "../lib/pageShell";
import { OffsiteIndicator } from "../components/OffsiteIndicator";
import { EffectiveScheduleLine } from "../components/EffectiveScheduleLine";
import { FolderBrowser } from "../components/FolderBrowser";
import { DEFAULT_RESTORE_FOLDER } from "../components/RestorePanel";
import { SnapshotFileTree } from "../components/SnapshotFileTree";
import { BackupCancelButton } from "../components/BackupCancelButton";
import { ProgressBar } from "../components/ProgressBar";
import { RecentRunsList } from "../components/RecentRunsList";
import { MissingRestorePoint, restorePointOf } from "../components/restore/MissingRestorePoint";
import { RestoreProgress } from "../components/restore/RestoreProgress";
import { EmptyStateIcon } from "../components/EmptyStateIcon";
import { IconBackupNow, IconFiles, IconPencil, IconTrash } from "../components/Sidebar";
import { BULK_HUE } from "../lib/bulkHue";
import { useT } from "../lib/i18n";
import { Advanced, useAdvanced } from "../lib/advanced";
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
import { ItemAnomalyBadge } from "../components/ItemAnomalyBadge";
import { ItemAnomalySettings } from "../components/ItemAnomalySettings";
import { findingSnapshotId } from "../lib/anomalies";
import { useAnomalyItems, useAnomalySummary, useOpenAnomalies } from "../lib/useAnomalies";
import { useRestoreRequest, type RestoreRequest } from "../lib/restoreRequest";

type T = ReturnType<typeof useT>["t"];

function formatTs(unix: number | null | undefined): string {
  if (!unix) return "—";
  return new Date(unix * 1000).toLocaleString();
}

// FileSetEnabledToggle is the file-set copy of components/IncludeToggle.tsx.
function FileSetEnabledToggle({ id, initial }: { id: string; initial: boolean }) {
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
  t,
}: {
  set: FileSetView;
  snapshotId: string;
  source: RepoSource;
  hostMountRoot: string;
  restoreFolder: string;
  otherActive: { active: boolean; phase?: string };
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
      if (res.ok) setRestoredTarget(res.target ?? "");
      return res;
    },
  });
  const prog = useProgress()[progressKey];
  const blockedByOther = otherActive.active && !isPending;

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
          <div className="flex items-center gap-2">
            <Button
              label={t("files.restoreSelected").replace("{n}", String(count))}
              labelKey="files.restoreSelected"
              glyph={<IconRestore />}
              tone="accent"
              onClick={handleRestoreSelected}
              disabled={isPending || blockedByOther || !folder.trim()}
              busy={isPending}
              title={isPending ? t("common.restoring") : undefined}
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
  t,
  trailing,
}: {
  set: FileSetView;
  snapshotId: string;
  source: RepoSource;
  hostMountRoot: string;
  restoreFolder: string;
  otherActive: { active: boolean; phase?: string };
  t: T;
  /** Rendered at the end of the destination row, after the Restore button. It
   *  is a prop because it has to join this component's own flex row. */
  trailing?: ReactNode;
}) {
  // Without a path the server cannot restore in place, so only a folder works.
  const noPath = set.path === "";
  const [dest, setDest] = useState<RestoreDest>(noPath ? "folder" : "original");
  // Selecting files is an advanced option. The flag is read here because the
  // Selector takes a flat items list that an <Advanced> wrapper cannot filter.
  const { advanced } = useAdvanced();
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
    start: () =>
      restoreFileSet(set.id, snapshotId, true, dest === "folder" ? targetPath : "", source),
  });
  const prog = useProgress()[progressKey];
  const blockedByOther = otherActive.active && !isPending;
  const { confirm, confirmDialog } = useConfirm();

  // An old result would describe another destination, so a new choice clears
  // it (a no-op while a restore runs).
  useEffect(() => reset(), [dest, targetPath, reset]);

  async function handleRestore() {
    if (dest === "original" && !(await confirm(t("files.restoreOriginalConfirm")))) return;
    if (dest === "folder" && targetPath.trim() === "") return;
    void fire();
  }

  const destItems: SelectorItem[] = [
    { id: "original", label: t("files.restoreOriginal"), disabled: noPath, title: noPath ? t("files.noPathHint") : undefined },
    { id: "folder", label: t("files.restoreToFolder") },
    ...(advanced ? [{ id: "select", label: t("files.restoreSelectFiles") }] : []),
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
          onChange={(id) => setDest(id as RestoreDest)}
          disabled={isPending}
        />
        {/* Selecting files brings its own controls in FileSetFileBrowser. */}
        {dest !== "select" && (
          <Button
            label={t("snapshots.restore")}
            labelKey="snapshots.restore"
            tone="accent"
            onClick={() => void handleRestore()}
            disabled={isPending || blockedByOther || (dest === "folder" && targetPath.trim() === "")}
            busy={isPending}
            title={isPending ? t("common.restoring") : undefined}
            className="shrink-0"
          />
        )}
        {blockedByOther && dest !== "select" && (
          <span className="text-caption text-carbon-textMuted shrink-0">
            {t(busyPhraseKey(otherActive.phase))}
          </span>
        )}
        {/* Pushed to the far end, so the delete badge does not read as a
            further restore step. */}
        {trailing && <span className="ms-auto flex items-center">{trailing}</span>}
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
          t={t}
        />
      )}
      {confirmDialog}
    </div>
  );
}

// FileSetSnapshotRow and FileSetRestorePanel follow VMSnapshotRow and
// VMRestorePanel.
function FileSetSnapshotRow({
  snap,
  set,
  source,
  hostMountRoot,
  restoreFolder,
  flagged,
  preselected,
  onDeleted,
  t,
}: {
  snap: Snapshot;
  set: FileSetView;
  source: RepoSource;
  hostMountRoot: string;
  restoreFolder: string;
  /** An open data-loss finding was raised on this snapshot. */
  flagged: boolean;
  /** A finding's restore link asked for this snapshot, so the row stands out
   *  from its neighbours. */
  preselected: boolean;
  onDeleted: () => void;
  t: T;
}) {
  const progressMap = useProgress();
  const running = anyActive(progressMap);
  // Delete only waits for this set's own backup or restore, as in the VM panel.
  const busy = progressMap[`files:${set.name}`]?.active ?? false;
  const [deleting, setDeleting] = useState(false);
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [shake, setShake] = useState(0);

  async function handleDelete() {
    if (!(await confirm(t("snapshots.deleteConfirm"), { confirmKey: "snapshots.delete" }))) return;
    setDeleting(true);
    try {
      const res = await deleteSnapshot("files", snap.id, source);
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

  return (
    // py-1.5 keeps the row at 44px around the 32px icon badge, as in
    // RestorePanel.tsx and Config.tsx.
    <div
      className={`flex flex-col gap-1 py-1.5 border-b border-carbon-border last:border-0${
        preselected ? " bg-carbon-surface2 px-2 rounded-control" : ""
      }`}
    >
      {/* The date sits under the id, so it reads as a property of it. */}
      <div className="flex flex-col items-start">
        <span className="flex items-center gap-2">
          <span dir="ltr" className="font-mono text-start text-carbon-text text-xs">
            {snap.id.slice(0, 8)}
          </span>
          {flagged && (
            <Badge tone="fail" size="small">
              {t("anomaly.snapshotFlagged")}
            </Badge>
          )}
        </span>
        <span className="text-carbon-textMuted text-xs">
          {new Date(snap.time).toLocaleString()}
          {snap.tags && snap.tags.length > 0 && (
            <span className="hidden sm:inline">{` · ${snap.tags.join(", ")}`}</span>
          )}
        </span>
      </div>
      {/* Everything a snapshot does sits in one row, delete last. The badge
          takes the hue of FileSetRow's card and no red of its own; the glyph,
          the tip and the confirm dialog carry its meaning. */}
      <div>
        <FileSetRestoreControl
          set={set}
          snapshotId={snap.id}
          source={source}
          hostMountRoot={hostMountRoot}
          restoreFolder={restoreFolder}
          otherActive={running}
          t={t}
          trailing={
            <Button
              key={shake}
              label={t("snapshots.delete")}
              labelKey="snapshots.delete"
              glyph={<IconTrash />}
              tone="accent"
              onClick={() => void handleDelete()}
              disabled={deleting || busy}
              className={`shrink-0${shake ? " glim-shake" : ""}`}
            />
          }
        />
      </div>
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
  const [source, setSource] = useState<RepoSource>("local");
  const [snapshots, setSnapshots] = useState<Snapshot[]>([]);
  const { flagged } = useOpenAnomalies();
  // Before the first answer an empty list would report a linked backup as gone.
  const [loading, setLoading] = useState(true);
  // A failed list load replaces the whole list, so it stays inline rather
  // than going into a toast.
  const [error, setError] = useState<string | null>(null);

  const [reloadTick, setReloadTick] = useState(0);
  const [deletingAll, setDeletingAll] = useState(false);
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [shakeDeleteAll, setShakeDeleteAll] = useState(0);

  useEffect(() => {
    if (!open) return;
    // An answer that arrives after the next switch is dropped, and a failure
    // empties the list, whose rows belong to the source just left.
    let current = true;
    const fail = (message: string) => {
      if (!current) return;
      setSnapshots([]);
      setError(message);
    };
    setLoading(true);
    setError(null);
    fileSetSnapshots(set.id, source)
      .then((res) => {
        if (!res.ok) return fail(res.error ?? t("common.loadBackupsFailed"));
        if (current) setSnapshots(res.snapshots ?? []);
      })
      .catch(() => fail(t("common.loadBackupsFailed")))
      .finally(() => {
        if (current) setLoading(false);
      });
    return () => {
      current = false;
    };
  }, [open, set.id, source, reloadTick]); // eslint-disable-line react-hooks/exhaustive-deps -- t() is only read to build a failure message; re-fetching on a language switch would be a wasted round-trip

  // A failure goes to a toast: the reload that follows it would clear an
  // inline error straight away.
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
          <div className="flex items-center gap-2 py-2 border-b border-carbon-border">
              {/* The source toggle and its hint are advanced; basic mode uses local. */}
              <Advanced>
                <span className="flex items-center gap-1 text-xs text-carbon-textMuted">
                  {t("source.label")}
                  <InfoBubble tip={t("source.hint")} />
                </span>
                <SourceToggle source={source} onChange={setSource} disabled={loading} domain="files" />
              </Advanced>
              {/* Delete-all acts on the local repository and forgets the set,
                  so it only shows with the local source. */}
              {source === "local" && snapshots.length > 0 && (
                // A neutral full-size Button like the container card's
                // delete-all; bombvault/no-status-color-on-control keeps red
                // off it. glyphFor gives snapshots.deleteAll its trash glyph.
                // The in-flight wording goes in the title, so the label does
                // not resize the button mid-action.
                <Button
                  key={shakeDeleteAll}
                  label={t("snapshots.deleteAll")}
                  labelKey="snapshots.deleteAll"
                  tone="neutral"
                  onClick={() => void handleDeleteAll()}
                  disabled={deletingAll || loading}
                  busy={deletingAll}
                  title={deletingAll ? t("snapshots.deletingAll") : undefined}
                  className={`ms-auto${shakeDeleteAll ? " glim-shake" : ""}`}
                />
              )}
          </div>
          <RecentRunsList name={set.name} domain="files" t={t} />
          {loading && (
            <p className="py-3 text-xs text-carbon-textMuted">{t("common.loadingBackups")}</p>
          )}
          {error && <p className="py-3 text-xs text-statusFail">{error}</p>}
          {!loading && !error && (
            <MissingRestorePoint
              requested={preselect}
              requestedAt={preselectAt}
              points={snapshots.map(restorePointOf)}
              t={t}
            />
          )}
          {!loading && !error && snapshots.length === 0 && (
            <p className="py-3 text-xs text-carbon-textMuted">{t("snapshots.none")}</p>
          )}
          {!loading &&
            snapshots.map((snap) => (
              <FileSetSnapshotRow
                key={snap.id}
                snap={snap}
                set={set}
                source={source}
                hostMountRoot={hostMountRoot}
                restoreFolder={restoreFolder}
                flagged={flagged.has(findingSnapshotId(snap))}
                preselected={findingSnapshotId(snap) === preselect}
                onDeleted={() => setReloadTick((n) => n + 1)}
                t={t}
              />
            ))}
        </div>
      )}
      {confirmDialog}
    </div>
  );
}

// Exported for the page's dom tests, which render it against a mocked client.
export function FileSetDialog({
  initial,
  presetSeed,
  hostMountRoot,
  t,
  onClose,
  onSaved,
}: {
  /** null = create a new set; a view = edit that set. */
  initial: FileSetView | null;
  /** Pre-fill for a new set opened through "Add preset: Host system config",
   *  the counterpart of the flash domain on generic hosts and TrueNAS.
   *  Ignored when editing; every field stays editable. */
  presetSeed: { name: string; path: string; excludes: string[] } | null;
  hostMountRoot: string;
  t: T;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { push } = useToast();
  const [name, setName] = useState(initial?.name ?? presetSeed?.name ?? "");
  const [path, setPath] = useState(initial?.path ?? presetSeed?.path ?? "");
  const [excludesText, setExcludesText] = useState(
    (initial?.excludes ?? presetSeed?.excludes ?? []).join("\n")
  );
  const [enabled, setEnabled] = useState(initial?.enabled ?? true);
  // Empty means the Folders repository, which is where a new set starts.
  const [repo, setRepo] = useState(initial?.repo ?? "");
  // A set with backups keeps its repository and its name, since nothing moves
  // or re-tags its snapshots. Locked here as well as refused by the server, so
  // the reason shows before the attempt.
  const hasBackups = Boolean(initial) && (initial?.lastBackup ?? 0) > 0;
  const [saving, setSaving] = useState(false);
  const [shake, setShake] = useState(0);

  const canSave = name.trim() !== "" && path.trim() !== "" && !saving;

  // The dialog closes on success, so every outcome is reported as a toast.
  async function handleSave() {
    if (!canSave) return;
    setSaving(true);
    const excludes = excludesText
      .split("\n")
      .map((line) => line.trim())
      .filter((line) => line !== "");
    try {
      const res = initial
        ? await patchFileSet(initial.id, {
            name: name.trim(),
            path: path.trim(),
            excludes,
            enabled,
            // Sent only when it differs: once the set has backups the server
            // refuses the field, and an ordinary save must still go through.
            ...(repo.trim() !== (initial.repo ?? "").trim() ? { repo: repo.trim() } : {}),
          })
        : await createFileSet({
            name: name.trim(),
            path: path.trim(),
            excludes,
            enabled,
            // A new set has no backups, so the picker is live here and its
            // answer travels with the create.
            repo: repo.trim(),
          });
      if (res.ok) {
        push(t("settings.saved"), "success");
        onSaved();
      } else {
        push(res.error ?? t("settings.error"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      setShake((n) => n + 1);
    } finally {
      setSaving(false);
    }
  }

  // Portal to <body> so no ancestor's CSS transform can trap the fixed
  // overlay. Centred rather than top-anchored, which would push the heading
  // notch against the viewport edge; the box is capped at 90vh, so it never
  // clips.
  return createPortal(
    <div
      className="glim-modal-backdrop fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-4"
      onClick={onClose}
    >
      {/* The heading notch sits on a non-scrolling shell around the
          scrollable box, as in Receiver.tsx's ReceiverDialog. */}
      <div className="relative w-full max-w-lg">
      {/* px-5 matches the box's p-5 so the notch lands where a Card's does;
          FolderBrowser.tsx explains why the notch has no offset of its own. */}
      <h2 className="flex items-center px-5">
        <Badge tone="heading" size="heading" wrap>{initial ? t("files.editSet") : t("files.addSet")}</Badge>
      </h2>
      <div
        role="dialog"
        aria-modal="true"
        aria-label={initial ? t("files.editSet") : t("files.addSet")}
        onClick={(e) => e.stopPropagation()}
        className="w-full max-h-[90vh] overflow-y-auto rounded-card bg-carbon-surface p-5 flex flex-col gap-4 shadow-2xl"
      >
        {/* The name becomes a restic tag, so the server validates it strictly. */}
        <div className="flex flex-col gap-1.5">
          <label className="flex items-center gap-1 text-xs text-carbon-textSub">
            {t("files.name")}
            {hasBackups && <InfoBubble tip={t("files.nameLocked")} />}
          </label>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            disabled={hasBackups}
            spellCheck={false}
            autoComplete="off"
            placeholder="documents"
            className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus disabled:opacity-50"
          />
        </div>

        {/* Source folder (relative subpath under the host mount root) */}
        <div className="flex flex-col gap-1.5">
          <FolderBrowser
            inDialog
            label={t("files.path")}
            value={path}
            hostMountRoot={hostMountRoot}
            onChange={setPath}
          />
          {/* Saving a new path clears the ticked sub-folder selection on the
              server, so the hint says so beforehand, whether or not a
              selection exists. The tree itself lives on the card. */}
          <p className="text-caption text-carbon-textMuted">{t("files.pathChangeHint")}</p>
          <p className="text-caption text-carbon-textMuted">{t("files.pathHint")}</p>
        </div>

        {/* Exclude patterns, one per line */}
        <div className="flex flex-col gap-1.5">
          <label className="text-xs text-carbon-textSub">{t("files.excludes")}</label>
          <textarea
            value={excludesText}
            onChange={(e) => setExcludesText(e.target.value)}
            spellCheck={false}
            rows={4}
            placeholder={"*.tmp\ncache/"}
            dir="ltr"
            className="rounded-control bg-carbon-surface2 text-carbon-text text-sm font-mono px-3 py-1.5 glim-field-focus text-start"
          />
          <p className="text-caption text-carbon-textMuted">{t("files.excludesHint")}</p>
        </div>

        {/* The set's own repository, empty for the Folders repository. The
            locations are defined in Settings and picked here, so a bucket
            path is typed once and corrected in one place. */}
        <RepoPicker
          value={repo}
          onChange={setRepo}
          locked={hasBackups}
          labelKey="files.repo"
          hintKey="files.repoHint"
          defaultLabelKey="files.repoPlaceholder"
          lockedKey="files.repoLocked"
        />

        <ToggleRow checked={enabled} onChange={setEnabled} label={t("files.enabled")} />

        <div className="flex items-center justify-end gap-2 pt-1">
          <Button
            label={t("files.cancel")}
            labelKey="files.cancel"
            tone="neutral"
            onClick={onClose}
            disabled={saving}
          />
          <Button
            key={shake}
            label={t("settings.save")}
            labelKey="settings.save"
            tone="accent"
            onClick={() => void handleSave()}
            disabled={!canSave}
            busy={saving}
            title={saving ? t("common.saving") : undefined}
            className={shake ? "glim-shake" : ""}
          />
        </div>
      </div>
      </div>
    </div>,
    document.body,
  );
}

/** What a queued file-set PATCH was sent for. `pre` and `sent` hold the
 *  toggle's effect, so a failure undoes exactly that toggle (revertFrom)
 *  rather than restoring a snapshot, which would also undo newer toggles
 *  queued behind it. `node` is the row a failure points at: it shakes, and
 *  an empty-selection refusal puts its warn line under it. */
interface FileSetSaveDesc {
  node: string;
  pre: { includes: ReadonlySet<string>; exclusions: ReadonlySet<string> };
  sent: { includes: ReadonlySet<string>; exclusions: ReadonlySet<string> };
}

/** fileSetEditorKey is the remount key for FileSetFoldersEditor: set id, path
 *  and whether a selection is stored. The editor seeds its state from the
 *  mount-time props and never re-syncs, so after a path edit, which clears the
 *  selection on the server, it has to remount; otherwise its next PATCH would
 *  be refused against the new root or bring the cleared entries back. A changed
 *  selection keeps the key, so a list refetch leaves the expansion, the browse
 *  cache and a PATCH in flight alone. */
export function fileSetEditorKey(set: FileSetView): string {
  return `${set.id}:${set.path}:${set.selectedPaths ? "set" : "null"}`;
}

// FileSetFoldersEditor puts the SelectionTree over the set's one root, as a
// disclosure on the set card. It stays out of FileSetDialog, whose full-set
// PATCHes would race the live save queue, and leaves out the container extras
// (CACHEDIR switch, custom rows, Reset). Exported for the page's dom tests.
export function FileSetFoldersEditor({
  set,
  hostMountRoot,
  t,
  anomaly,
  anomalyEnabled = false,
}: {
  set: FileSetView;
  hostMountRoot: string;
  t: T;
  /** What anomaly detection knows about this set, for its own sensitivity
   *  and notification setting under the tree. */
  anomaly?: AnomalyItem;
  anomalyEnabled?: boolean;
}) {
  const regionId = useId();
  const [open, setOpen] = useState(false);
  const noPath = set.path === "";
  // The set's resolved host path, cleaned on both segments (browseRelToHost is
  // the exact inverse of the browse prefix swap the tree performs below).
  const root = noPath ? "" : browseRelToHost(set.path, hostMountRoot);
  // Includes and exclusions in host paths. A set without a stored selection is
  // seeded with an include of its root, so the root renders checked and the
  // preview reads "1 path", which is what its backup covers; nothing is
  // written until the first toggle. The seed is read once per mount, and
  // fileSetEditorKey remounts the editor when it has to change.
  const seed = splitFlatSet(noPath ? [] : (set.selectedPaths ?? [root]));
  const [includes, setIncludes] = useState<Set<string>>(seed.includes);
  const [exclusions, setExclusions] = useState<Set<string>>(seed.exclusions);
  // The save queue reads the selection through this ref; applyMirror keeps it
  // in step with the state the tree renders.
  const mirrorRef = useRef<{ inc: Set<string>; exc: Set<string> }>({ inc: seed.includes, exc: seed.exclusions });
  // Per-row busy and shake state keyed by host path, as in FoldersEditor.
  // blockedPath is the row whose last toggle was refused, for the warn line.
  const [rowBusy, setRowBusy] = useState<Record<string, boolean>>({});
  const [rowShake, setRowShake] = useState<Record<string, number>>({});
  const [blockedPath, setBlockedPath] = useState<string | null>(null);
  // Listings cached for the editor's lifetime, so closing the disclosure
  // keeps them.
  const browseCache = useRef(new Map<string, Promise<BrowseResponse>>());
  const { push } = useToast();
  // A one-deep PATCH queue. While an attempt is in flight, further toggles
  // only update the selection and mark the queue dirty; when it settles, one
  // follow-up sends the latest full list. A slow or failing save can never
  // overwrite a newer toggle.
  const queueRef = useRef<{ inFlight: boolean; dirty: boolean }>({ inFlight: false, dirty: false });
  const pendingDescRef = useRef<FileSetSaveDesc | null>(null);
  // Rows whose toggles are folded into the next attempt's body (an attempt
  // always carries the live mirror); each row's busy flag clears exactly when
  // the attempt that acknowledges its effect settles.
  const pendingRowsRef = useRef<Set<string>>(new Set());

  function applyMirror(inc: Set<string>, exc: Set<string>): void {
    mirrorRef.current = { inc, exc };
    setIncludes(inc);
    setExclusions(exc);
  }

  // Called after the selection has been updated, as in FoldersEditor.
  function scheduleSave(desc: FileSetSaveDesc): void {
    if (queueRef.current.inFlight) {
      queueRef.current.dirty = true;
      pendingDescRef.current = desc; // latest desc wins, matching the drain
      return;
    }
    pendingDescRef.current = desc;
    void attemptSave();
  }

  // The body is built from the current selection when the attempt starts, not
  // from desc.sent, which is stale once later toggles queued behind it. Every
  // selection PATCH goes through here, so no two run at once.
  async function attemptSave(): Promise<void> {
    queueRef.current.inFlight = true;
    const rows = [...pendingRowsRef.current];
    pendingRowsRef.current.clear();
    const desc = pendingDescRef.current;
    pendingDescRef.current = null;
    try {
      const live = mirrorRef.current;
      const r = await patchFileSet(set.id, { selectedPaths: toFlatList(live.inc, live.exc) });
      if (r.ok) {
        push(t("folders.saved"), "success");
      } else {
        // A coded empty-selection refusal also gets the client check's warn
        // line. The client check normally prevents it, but a concurrent
        // writer or a stale anchor can still cause one.
        push(r.error ?? t("settings.error"), "fail");
        if (desc) {
          if (r.code === "empty-selection") setBlockedPath(desc.node);
          revertFrom(desc);
        }
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      if (desc) revertFrom(desc);
    } finally {
      queueRef.current.inFlight = false;
      setRowBusy((b) => {
        const n = { ...b };
        for (const p of rows) n[p] = false;
        return n;
      });
      if (queueRef.current.dirty) {
        queueRef.current.dirty = false;
        if (pendingDescRef.current) void attemptSave();
      }
    }
  }

  // Undoes only the failed toggle on the current selection: removes what it
  // added and adds back what it removed, so a toggle made after it survives.
  function revertFrom(desc: FileSetSaveDesc): void {
    const live = mirrorRef.current;
    const inc = new Set(live.inc);
    const exc = new Set(live.exc);
    for (const p of desc.sent.includes) if (!desc.pre.includes.has(p)) inc.delete(p);
    for (const p of desc.pre.includes) if (!desc.sent.includes.has(p)) inc.add(p);
    for (const p of desc.sent.exclusions) if (!desc.pre.exclusions.has(p)) exc.delete(p);
    for (const p of desc.pre.exclusions) if (!desc.sent.exclusions.has(p)) exc.add(p);
    // A toggle queued behind this save was checked against a selection that
    // still had this attempt's include, so undoing it can leave none. The
    // server never stored that, so fall back to the includes it still has.
    if (inc.size === 0) {
      for (const p of desc.pre.includes) inc.add(p);
    }
    applyMirror(inc, exc);
    setRowShake((s) => ({ ...s, [desc.node]: (s[desc.node] ?? 0) + 1 }));
  }

  // Applies the toggle optimistically to the current selection (the ref, not
  // the state closure) and queues the save. A toggle that would leave the set
  // without includes never reaches the server; its warn line points to
  // deleting the set, since file sets have no Reset.
  function onToggle(hostPath: string): void {
    const pre = { includes: mirrorRef.current.inc, exclusions: mirrorRef.current.exc };
    const next = applyToggle(hostPath, pre.includes, pre.exclusions);
    const unchanged =
      next.includes.size === pre.includes.size &&
      [...next.includes].every((p) => pre.includes.has(p)) &&
      next.exclusions.size === pre.exclusions.size &&
      [...next.exclusions].every((p) => pre.exclusions.has(p));
    if (unchanged) return;
    if (next.includes.size === 0) {
      setBlockedPath(hostPath);
      setRowShake((s) => ({ ...s, [hostPath]: (s[hostPath] ?? 0) + 1 }));
      return;
    }
    setBlockedPath(null);
    applyMirror(next.includes, next.exclusions);
    setRowBusy((b) => ({ ...b, [hostPath]: true }));
    pendingRowsRef.current.add(hostPath);
    scheduleSave({
      node: hostPath,
      pre,
      sent: { includes: next.includes, exclusions: next.exclusions },
    });
  }

  // A set without a path has no tree; the card's noPathHint line stands in.
  // FileSetRow checks this too. The check comes after the hooks.
  if (noPath) return null;

  return (
    <div className="border-t border-carbon-border pt-3">
      <button
        type="button"
        aria-expanded={open}
        aria-controls={regionId}
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-center gap-2 py-1 text-start text-sm text-carbon-textSub"
      >
        <span className="w-4 shrink-0 flex items-center justify-center" aria-hidden="true">
          <svg
            width="10"
            height="10"
            viewBox="0 0 12 12"
            fill="none"
            className={`transition-transform ${open ? "rotate-90" : "rtl:rotate-180"}`}
          >
            <path fill="currentColor" d="M4 1.3 8.5 6 4 10.7Z" />
          </svg>
        </span>
        {t("files.foldersToggle")}
      </button>
      {open && (
        <div id={regionId} role="region" aria-label={t("folders.title")} className="mt-2 flex flex-col gap-2">
          <p className="text-xs text-carbon-textMuted">{t("files.foldersHint")}</p>
          {/* One root: the resolved host path with an empty dest, so the row
              is labelled with the bare path. Its count comes from
              rootIncludeCount, the paths the next backup hands restic. */}
          <SelectionTree
            mounts={[{ source: root, dest: "", selected: true, isAppdata: false, reachable: true }]}
            customPaths={[]}
            includes={includes}
            exclusions={exclusions}
            hostSourceRoot={hostMountRoot}
            containerName={`fileset-${set.id}`}
            browseCache={browseCache.current}
            onToggle={onToggle}
            // customPaths is always [] so the remove chip can never render;
            // the prop stays required on SelectionTreeProps (container shape).
            onRemoveCustom={() => {}}
            busyPaths={new Set(Object.keys(rowBusy).filter((k) => rowBusy[k]))}
            shakeCounts={rowShake}
            blockedPath={blockedPath}
            // Points to deleting the set rather than to a Reset.
            blockedMessage={t("files.emptySelectionBlocked")}
          />
          <ItemAnomalySettings item={anomaly} enabled={anomalyEnabled} t={t} />
        </div>
      )}
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
  index,
  anomaly,
  anomalyEnabled = false,
  restoreRequest,
}: {
  set: FileSetView;
  hostMountRoot: string;
  restoreFolder: string;
  t: T;
  onRefresh: () => void;
  onEdit: () => void;
  /** Rainbow position by list index, not a hash of the id or name. */
  index: number;
  anomaly?: AnomalyItem;
  anomalyEnabled?: boolean;
  /** A finding's restore link for this set: the card opens its backups and
   *  comes into view. */
  restoreRequest?: RestoreRequest;
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
      {/* Top row: name, chips and path, with the action badges. */}
      <div className="flex items-start gap-3 flex-wrap">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="font-semibold text-carbon-text text-sm truncate">
              {set.name}
            </span>
            <ItemAnomalyBadge item={anomaly} enabled={anomalyEnabled} t={t} />
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
            comes first, since it is what the card is for. */}
        <div className="ms-auto flex items-start gap-1.5 shrink-0">
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
          <FileSetEnabledToggle id={set.id} initial={set.enabled} />
        </div>
      </div>

      {/* What the toggle above means for this set, in the same
          server-computed sentence as the Schedules card. */}
      <EffectiveScheduleLine effective={set.effectiveSchedule} />

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

  function loadSets() {
    return listFileSets()
      .then((res) => {
        if (res.ok) {
          setSets(res.fileSets ?? []);
          // Clear the message of an earlier failed load.
          setError(null);
        } else setError(res.error ?? t("files.loadSetsFailed"));
      })
      .catch(() => setError(t("files.loadSetsFailed")));
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
    <div className={PAGE_SHELL}>
      {/* Heading with Discover, for disaster recovery, and the Add actions. */}
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-2xl font-semibold text-carbon-text">{t("files.title")}</h1>
          <p className="mt-1 text-sm text-carbon-textSub">{t("files.subtitle")}</p>
          <div className="mt-2"><OffsiteIndicator domain="files" /></div>
        </div>
        <div className="flex items-center gap-2 shrink-0 flex-wrap">
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
              index={i}
              anomaly={anomalies.find("files", s.id)}
              anomalyEnabled={anomalyEnabled}
              restoreRequest={restoreRequest.item === s.name ? restoreRequest : undefined}
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
