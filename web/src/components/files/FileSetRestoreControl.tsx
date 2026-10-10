import { useEffect, useRef, useState } from "react";
import { listSnapshotFilesFileSet, restoreFileSet, restoreFileSetFiles } from "../../lib/api";
import type { FileEntry, FileSetView } from "../../lib/api";
import { useBackupWatch } from "../../lib/backupWatch";
import { loadErrorMessage } from "../../lib/errors";
import type { useT } from "../../lib/i18n";
import { busyPhraseKey, useProgress } from "../../lib/progress";
import { SNAPSHOT_MISSING } from "../../lib/timeline";
import { useConfirm } from "../../lib/useConfirm";
import { checkRestoreOnce, restoreBlockReason, useRestoreCheck } from "../../lib/useRestoreCheck";
import { Button } from "../Button";
import { FolderBrowser } from "../FolderBrowser";
import { RestoreCheckPanel } from "../restore/RestoreCheckPanel";
import { RestoreProgress } from "../restore/RestoreProgress";
import { Selector, type SelectorItem } from "../Selector";
import { IconRestore } from "../Sidebar";
import { SnapshotFileTree } from "../SnapshotFileTree";
import type { RepoSource } from "../SourceToggle";

type T = ReturnType<typeof useT>["t"];

// FileSetFileBrowser restores ticked files and folders from a snapshot into a
// folder. It never writes in place, so it asks nothing.
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
  // Shared with the cancel button inside RestoreProgress.
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
        // The server's own reason, such as a stale repo lock, says more.
        if (res.ok) setFiles(res.files ?? []);
        else setError(loadErrorMessage(res, t("files.loadFailed")));
      })
      .catch(() => setError(t("files.loadFailed")))
      .finally(() => setLoading(false));
  }, [set.id, snapshotId, source, t]);

  // The last result described the previous selection or folder.
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

export function FileSetRestoreControl({
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
  const [targetPath, setTargetPath] = useState(restoreFolder);

  const progressKey = `files:${set.name}`;
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

  // An old result would describe another destination.
  useEffect(() => reset(), [dest, targetPath, reset]);

  // The check runs on the click. In place it always asks, into a folder it
  // speaks up only when the restore cannot work.
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
