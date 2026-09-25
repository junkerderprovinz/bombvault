import { useEffect, useRef, useState } from "react";
import { listSnapshots, restore, listSnapshotFiles, restoreContainerFiles, restoreContainerToPath, deleteSnapshot, diffSnapshots, tagSnapshot, getSettings } from "../lib/api";
import type { Snapshot, FileEntry, SnapshotDiff } from "../lib/api";
import type { useT } from "../lib/i18n";
import { Advanced, useAdvanced } from "../lib/advanced";
import { useBackupWatch } from "../lib/backupWatch";
import { useProgress, anyActive, busyPhraseKey } from "../lib/progress";
import { RestoreProgress } from "./restore/RestoreProgress";
import { RestoreAction } from "./restore/RestoreAction";
import { SourceToggle, type RepoSource } from "./SourceToggle";
import { FolderBrowser } from "./FolderBrowser";
import { RecentRunsList } from "./RecentRunsList";
import { SnapshotFileTree } from "./SnapshotFileTree";
import { loadErrorMessage } from "../lib/errors";
import { useConfirm } from "../lib/useConfirm";
import { useToast } from "../lib/toast";
import { Button } from "./Button";
import { SelectField } from "./SelectField";
import { InfoBubble } from "./InfoBubble";
import { IconRestore, IconTrash } from "./Sidebar";
import { IconDisclosure } from "./IconDisclosure";

type T = ReturnType<typeof useT>["t"];

// humanBytes formats a byte count in binary units with one decimal, like the
// Dashboard's storage card.
function humanBytes(n: number): string {
  if (!n || n <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let v = n;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${i === 0 ? v : v.toFixed(1)} ${units[i]}`;
}

// displayTags hides the ownership tags under the entry's own or a former name,
// the formerly: takeover marker and the orchestrator's internal marker tags.
const INTERNAL_TAGS = new Set(["p1"]);
function displayTags(snap: Snapshot, containerName: string, aliases: string[]): string[] {
  const owners = new Set([containerName, ...aliases].map((n) => `container:${n}`));
  return (snap.tags ?? []).filter((tg) => !owners.has(tg) && !INTERNAL_TAGS.has(tg) && !tg.startsWith("formerly:"));
}

// SnapshotFileBrowser restores ticked files and folders from a snapshot, in
// place or into another folder.
function SnapshotFileBrowser({
  containerName,
  snapshotId,
  source,
  hostMountRoot,
  defaultFolder,
  t,
}: {
  containerName: string;
  snapshotId: string;
  source: string;
  hostMountRoot: string;
  defaultFolder: string;
  t: T;
}) {
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState("");
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [dest, setDest] = useState<"inPlace" | "toFolder">("inPlace");
  const [folder, setFolder] = useState(defaultFolder);
  const [restoredTarget, setRestoredTarget] = useState("");

  // The server acks with {started, target} and runs the restore detached, so
  // it survives the panel or the browser closing. useBackupWatch reads the
  // outcome from the run history.
  const cancelledRef = useRef(false);
  const { state: restoreState, fire, reset, isPending } = useBackupWatch({
    progressKey: `container:${containerName}`,
    kind: "restore",
    start: async () => {
      const paths = [...selected];
      const targetPath = dest === "toFolder" ? folder.trim() : "";
      const res = await restoreContainerFiles(containerName, snapshotId, paths, targetPath, true, source);
      if (res.ok) setRestoredTarget(res.target ?? "");
      return res;
    },
    matchRun: (r) => r.domain === "container" && r.target === containerName,
    cancelledRef,
  });
  const progressMap = useProgress();
  const prog = progressMap[`container:${containerName}`];
  // Any other running backup, restore or replication blocks a new restore.
  // This item's own run shows as isPending instead.
  const running = anyActive(progressMap);
  const blockedByOther = running.active && !isPending;
  const { confirm, confirmDialog } = useConfirm();

  useEffect(() => {
    setLoading(true);
    listSnapshotFiles(containerName, snapshotId, source)
      .then((res) => {
        if (res.ok) setFiles(res.files ?? []);
        else setError(loadErrorMessage(res, t("files.loadFailed")));
      })
      .catch(() => setError(t("files.loadFailed")))
      .finally(() => setLoading(false));
  }, [containerName, snapshotId, source, t]);

  // toggle, pickDest and pickFolder clear the last result, which described the
  // previous choice.
  function toggle(p: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(p)) next.delete(p);
      else next.add(p);
      return next;
    });
    reset();
  }

  function pickDest(d: "inPlace" | "toFolder") {
    setDest(d);
    reset();
  }
  function pickFolder(v: string) {
    setFolder(v);
    reset();
  }

  async function handleRestoreSelected() {
    if (selected.size === 0) return;
    if (dest === "toFolder" && !folder.trim()) return;
    // Restoring in place overwrites the live files, so it asks first.
    if (dest === "inPlace" && !(await confirm(t("files.restoreConfirm")))) return;
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
          <div className="flex flex-col gap-1.5">
            <label className="flex items-center gap-2 cursor-pointer text-carbon-text">
              <input
                type="radio"
                name={`files-dest-${snapshotId}`}
                checked={dest === "inPlace"}
                onChange={() => pickDest("inPlace")}
                style={{ accentColor: "var(--accent)" }}
              />
              {t("files.dest.inPlace")}
            </label>
            <label className="flex items-center gap-2 cursor-pointer text-carbon-text">
              <input
                type="radio"
                name={`files-dest-${snapshotId}`}
                checked={dest === "toFolder"}
                onChange={() => pickDest("toFolder")}
                style={{ accentColor: "var(--accent)" }}
              />
              {t("files.dest.toFolder")}
            </label>
          </div>
          {dest === "toFolder" && (
            <FolderBrowser
              label={t("restore.targetPath")}
              value={folder}
              hostMountRoot={hostMountRoot}
              onChange={pickFolder}
            />
          )}
          <div className="flex items-center gap-2">
            <Button
              label={t("files.restoreSelected").replace("{n}", String(count))}
              labelKey="files.restoreSelected"
              glyph={<IconRestore />}
              tone="accent"
              onClick={() => void handleRestoreSelected()}
              disabled={isPending || blockedByOther || (dest === "toFolder" && !folder.trim())}
              busy={isPending}
              title={isPending ? t("common.restoring") : undefined}
              className="shrink-0"
            />
            {blockedByOther && (
              <span className="text-caption text-carbon-textMuted">{t(busyPhraseKey(running.phase))}</span>
            )}
          </div>
          <RestoreProgress
            state={restoreState}
            isPending={isPending}
            prog={prog}
            cancelKey={`container:${containerName}`}
            inPlace={dest === "inPlace"}
            name={containerName}
            cancelledRef={cancelledRef}
            successMessage={
              restoredTarget
                ? t("restore.restoredTo").replace("{path}", restoredTarget)
                : t("files.restoredInPlace")
            }
            t={t}
          />
        </div>
      )}
      {confirmDialog}
    </div>
  );
}

interface RestorePanelProps {
  name: string;
  /** The entry's former names, whose ownership tags are hidden like its own. */
  aliases?: string[];
  t: T;
  // False for a container that is not installed. With a config-only backup
  // it can be recreated from the saved definition.
  installed?: boolean;
  /** Whether the panel is shown. The caller owns the toggle. */
  open: boolean;
}

// RecreateButton recreates a container that is not installed from its saved
// definition, since a config-only backup has no snapshot to restore. The
// backend turns a restore of "latest" into a recreate. The POST only
// acknowledges the start; the result comes from the recorded run.
function RecreateButton({ name, source, t }: { name: string; source: string; t: T }) {
  const cancelledRef = useRef(false);
  const { state, fire, isPending } = useBackupWatch({
    progressKey: `container:${name}`,
    kind: "restore",
    start: () => restore(name, "latest", true, source),
    matchRun: (r) => r.domain === "container" && r.target === name,
    cancelledRef,
  });
  const progressMap = useProgress();
  const prog = progressMap[`container:${name}`];
  const running = anyActive(progressMap);
  const blockedByOther = running.active && !isPending;
  const { confirm, confirmDialog } = useConfirm();
  async function handle() {
    if (!(await confirm(t("snapshots.recreateConfirm")))) return;
    void fire();
  }
  return (
    <div className="flex flex-col gap-1 py-2">
      <Button
        label={t("snapshots.recreate")}
        labelKey="snapshots.recreate"
        tone="accent"
        onClick={() => void handle()}
        disabled={isPending || blockedByOther || state.phase === "success"}
        busy={isPending}
        title={isPending ? t("common.restoring") : undefined}
        className="self-start"
      />
      {blockedByOther && (
        <span className="text-caption text-carbon-textMuted">{t(busyPhraseKey(running.phase))}</span>
      )}
      <RestoreProgress
        state={state}
        isPending={isPending}
        prog={prog}
        cancelKey={`container:${name}`}
        inPlace
        name={name}
        cancelledRef={cancelledRef}
        successMessage={t("restore.recreateComplete")}
        t={t}
      />
      {confirmDialog}
    </div>
  );
}

// RestoreToFolder extracts a whole snapshot into another folder under the host
// mount and leaves the running container alone.
function RestoreToFolder({
  containerName,
  snapshotId,
  source,
  hostMountRoot,
  defaultFolder,
  t,
}: {
  containerName: string;
  snapshotId: string;
  source: string;
  hostMountRoot: string;
  defaultFolder: string;
  t: T;
}) {
  const [path, setPath] = useState(defaultFolder);
  const [target, setTarget] = useState("");

  // Runs detached like the file restore: an extraction can take hours, and a
  // request held open that long gets dropped by the browser or a proxy.
  const cancelledRef = useRef(false);
  const { state, fire, reset, isPending } = useBackupWatch({
    progressKey: `container:${containerName}`,
    kind: "restore",
    start: async () => {
      const p = path.trim();
      const res = await restoreContainerToPath(containerName, snapshotId, p, source);
      if (res.ok) setTarget(res.target ?? p);
      return res;
    },
    matchRun: (r) => r.domain === "container" && r.target === containerName,
    cancelledRef,
  });
  const progressMap = useProgress();
  const prog = progressMap[`container:${containerName}`];
  const running = anyActive(progressMap);
  const blockedByOther = running.active && !isPending;

  function pickPath(v: string) {
    setPath(v);
    reset(); // the last result described the previous folder
  }

  const done = state.phase === "success";
  return (
    <div className="mt-1 rounded-card bg-carbon-background p-2 flex flex-col gap-1.5">
      <p className="text-caption text-carbon-textMuted">{t("restore.toFolderHint")}</p>
      <FolderBrowser
        label={t("restore.targetPath")}
        value={path}
        hostMountRoot={hostMountRoot}
        onChange={pickPath}
      />
      <div className="flex items-center gap-2">
        <Button
          label={t("common.confirm")}
          labelKey="common.confirm"
          tone="accent"
          onClick={() => void fire()}
          disabled={!path.trim() || isPending || blockedByOther || done}
          busy={isPending}
          title={isPending ? t("common.restoring") : undefined}
          className="shrink-0"
        />
        {blockedByOther && (
          <span className="text-caption text-carbon-textMuted">{t(busyPhraseKey(running.phase))}</span>
        )}
      </div>
      <RestoreProgress
        state={state}
        isPending={isPending}
        prog={prog}
        cancelKey={`container:${containerName}`}
        inPlace={false}
        name={containerName}
        cancelledRef={cancelledRef}
        successMessage={t("restore.restoredTo").replace("{path}", target)}
        t={t}
      />
    </div>
  );
}

// snapLabel renders a snapshot's short id and time for the compare selects.
function snapLabel(snap: Snapshot): string {
  return `${snap.id.slice(0, 8)} · ${new Date(snap.time).toLocaleString()}`;
}

// CompareSnapshots shows what changed between two snapshots (restic diff),
// starting with the newest pair.
function CompareSnapshots({
  snapshots,
  containerName,
  source,
  t,
}: {
  snapshots: Snapshot[];
  containerName: string;
  source: string;
  t: T;
}) {
  const [open, setOpen] = useState(false);
  const [from, setFrom] = useState(snapshots[1]?.id ?? "");
  const [to, setTo] = useState(snapshots[0]?.id ?? "");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [diff, setDiff] = useState<SnapshotDiff | null>(null);
  const { push } = useToast();
  // Bumping shake remounts the button, which replays .glim-shake.
  const [shake, setShake] = useState(0);

  // Switching between the local and off-site repo loads a different snapshot
  // list, and the server rejects IDs left over from the other one.
  useEffect(() => {
    setFrom(snapshots[1]?.id ?? "");
    setTo(snapshots[0]?.id ?? "");
    setDiff(null);
    setError(null);
  }, [snapshots]);

  // The diff stays inline because it is read at leisure, and a failed compare
  // shows its error in the same spot as well as in a toast.
  async function run() {
    if (!from || !to || from === to) return;
    setLoading(true);
    setError(null);
    setDiff(null);
    try {
      const res = await diffSnapshots(containerName, from, to, source);
      if (res.ok && res.diff) {
        setDiff(res.diff);
      } else {
        const message = res.error ?? t("common.compareFailed");
        setError(message);
        push(message, "fail");
        setShake((n) => n + 1);
      }
    } catch (e) {
      const message = e instanceof Error ? e.message : t("common.networkError");
      setError(message);
      push(message, "fail");
      setShake((n) => n + 1);
    } finally {
      setLoading(false);
    }
  }

  const summary = diff
    ? t("snapshot.diffSummary")
        .replace("{addedFiles}", String(diff.addedFiles))
        .replace("{addedBytes}", humanBytes(diff.addedBytes))
        .replace("{changedFiles}", String(diff.changedFiles))
        .replace("{removedFiles}", String(diff.removedFiles))
        .replace("{removedBytes}", humanBytes(diff.removedBytes))
    : "";

  const selectCls =
    "rounded-control bg-carbon-surface3 text-carbon-text text-xs px-2 py-1 glim-field-focus-well max-w-[16rem] truncate";

  return (
    <div className="py-2 border-b border-carbon-border">
      <Button
        label={t("snapshot.compare")}
        labelKey="snapshot.compare"
        tone="neutral"
        onClick={() => setOpen((p) => !p)}
        glyph={<IconDisclosure open={open} />}
      />
      {open && (
        <div className="mt-2 rounded-card bg-carbon-surface2 p-2 flex flex-col gap-2">
          <p className="text-caption text-carbon-textMuted">{t("snapshot.pickTwo")}</p>
          <div className="flex items-center gap-2 flex-wrap">
            <SelectField
              value={from}
              onChange={setFrom}
              label={t("snapshot.compareFrom")}
              options={snapshots.map((s) => ({ value: s.id, label: snapLabel(s) }))}
              disabled={loading}
              className={selectCls}
            />
            {/* The arrow follows reading order, so it flips under RTL.
                inline-block lets the scale apply to the glyph itself. */}
            <span className="inline-block text-xs text-carbon-textMuted rtl:-scale-x-100">→</span>
            <SelectField
              value={to}
              onChange={setTo}
              label={t("snapshot.compareTo")}
              options={snapshots.map((s) => ({ value: s.id, label: snapLabel(s) }))}
              disabled={loading}
              className={selectCls}
            />
            <Button
              key={shake}
              label={t("snapshot.compare")}
              labelKey="snapshot.compare"
              tone="accent"
              onClick={() => void run()}
              disabled={loading || !from || !to || from === to}
              busy={loading}
              title={loading ? "…" : undefined}
              className={shake ? "glim-shake" : ""}
            />
          </div>
          {error && <p className="text-xs text-statusFail wrap-break-word">{error}</p>}
          {diff && (
            <p className="text-xs text-carbon-text font-mono wrap-break-word" title={summary}>
              <span className="text-statusOk">+{diff.addedFiles}</span> {t("snapshot.added")} ({humanBytes(diff.addedBytes)}),{" "}
              <span className="text-carbon-textSub">~{diff.changedFiles}</span> {t("snapshot.changed")},{" "}
              <span className="text-statusFail">-{diff.removedFiles}</span> {t("snapshot.removed")} ({humanBytes(diff.removedBytes)})
            </p>
          )}
        </div>
      )}
    </div>
  );
}

// SnapshotTags shows a snapshot's user tags as chips, with an inline input to
// add one.
function SnapshotTags({
  snap,
  containerName,
  aliases,
  source,
  onTagged,
  t,
}: {
  snap: Snapshot;
  containerName: string;
  aliases: string[];
  source: string;
  onTagged: () => void;
  t: T;
}) {
  const [adding, setAdding] = useState(false);
  const [value, setValue] = useState("");
  const [busy, setBusy] = useState(false);
  const { push } = useToast();
  const tags = displayTags(snap, containerName, aliases);

  // A failed tag only toasts. The shake replays by remounting the element,
  // and remounting this focused input fires blur, which would submit the same
  // bad value again in a loop.
  async function submit() {
    const tag = value.trim();
    if (!tag) {
      setAdding(false);
      return;
    }
    setBusy(true);
    try {
      const res = await tagSnapshot(containerName, snap.id, [tag], source);
      if (res.ok) {
        setValue("");
        setAdding(false);
        onTagged();
      } else {
        push(res.error ?? t("common.actionFailed"), "fail");
      }
    } catch (e) {
      push(e instanceof Error ? e.message : t("common.networkError"), "fail");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex items-center gap-1 flex-wrap">
      {tags.map((tg) => (
        <span
          key={tg}
          className="inline-flex items-center rounded-pill bg-carbon-surface3 px-1.5 py-0.5 text-caption text-carbon-textSub"
        >
          {tg}
        </span>
      ))}
      {adding ? (
        <input
          type="text"
          value={value}
          autoFocus
          disabled={busy}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") void submit();
            else if (e.key === "Escape") {
              setAdding(false);
              setValue("");
            }
          }}
          onBlur={() => void submit()}
          placeholder={t("snapshot.addTag")}
          spellCheck={false}
          className="w-24 rounded-control bg-carbon-surface2 text-carbon-text text-caption px-1.5 py-0.5 glim-field-focus"
        />
      ) : (
        <Button
          label={t("snapshot.tags")}
          labelKey="snapshot.addTag"
          tone="neutral"
          onClick={() => setAdding(true)}
          title={t("snapshot.addTag")}
        />
      )}
    </div>
  );
}

// RestoreMode selects which of the three restore flows the inline panel shows.
type RestoreMode = "inPlace" | "files" | "toFolder";

function SnapshotRow({
  snap,
  containerName,
  aliases,
  source,
  hostMountRoot,
  defaultFolder,
  onDeleted,
  onTagged,
  t,
}: {
  snap: Snapshot;
  containerName: string;
  aliases: string[];
  source: RepoSource;
  hostMountRoot: string;
  defaultFolder: string;
  onDeleted: () => void;
  onTagged: () => void;
  t: T;
}) {
  const { advanced } = useAdvanced();
  const progressMap = useProgress();
  const running = anyActive(progressMap);
  // Delete waits only for this container's own backup or restore, not for
  // unrelated activity.
  const busy = progressMap[`container:${containerName}`]?.active ?? false;
  const [showRestore, setShowRestore] = useState(false);
  // Basic mode offers only the in-place restore.
  const [mode, setMode] = useState<RestoreMode>("inPlace");
  const effectiveMode: RestoreMode = advanced ? mode : "inPlace";
  const [deleting, setDeleting] = useState(false);
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  // Bumping shake remounts the delete button, which replays .glim-shake.
  const [shake, setShake] = useState(0);

  async function handleDelete() {
    if (!(await confirm(t("snapshots.deleteConfirm"), { confirmKey: "snapshots.delete" }))) return;
    setDeleting(true);
    try {
      const res = await deleteSnapshot("containers", snap.id, source);
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

  // One radio group per snapshot.
  const radioName = `restore-mode-${snap.id}`;

  return (
    // With the 32px icon badges, py-1.5 keeps the collapsed row at 44px, the
    // same as Config.tsx's ConfigSnapshotRow.
    <div className="flex flex-col gap-1 py-1.5 border-b border-carbon-border last:border-0">
      <div className="flex items-center gap-3 text-sm">
        <span dir="ltr" className="font-mono text-start text-carbon-text text-xs w-20 shrink-0">
          {snap.id.slice(0, 8)}
        </span>
        <span className="text-carbon-textMuted text-xs flex-1">
          {new Date(snap.time).toLocaleString()}
        </span>
        <Advanced>
          <div className="hidden sm:flex">
            <SnapshotTags snap={snap} containerName={containerName} aliases={aliases} source={source} onTagged={onTagged} t={t} />
          </div>
        </Advanced>

        {/* No hueIndex on these buttons: the row sits inside ContainerRow's
            .glim-hue element, so the accent already resolves to the row's
            colour. */}
        <Button
          label={t("restore.open")}
          labelKey="restore.open"
          glyph={<IconRestore />}
          tone="accent"
          onClick={() => setShowRestore((p) => !p)}
        />

        {/* Not red: the trash glyph, the tooltip and the confirm dialog carry
            the destructive meaning. */}
        <Button
          key={shake}
          label={t("snapshots.delete")}
          labelKey="snapshots.delete"
          glyph={<IconTrash />}
          tone="accent"
          onClick={() => void handleDelete()}
          disabled={deleting || busy}
          className={shake ? "glim-shake" : ""}
        />
      </div>

      {showRestore && (
        <div className="mt-1 rounded-card bg-carbon-surface2 p-3 flex flex-col gap-3 text-xs">
          <Advanced>
            <div className="flex flex-col gap-1.5">
              <label className="flex items-center gap-2 cursor-pointer text-carbon-text">
                <input
                  type="radio"
                  name={radioName}
                  checked={mode === "inPlace"}
                  onChange={() => setMode("inPlace")}
                  style={{ accentColor: "var(--accent)" }}
                />
                {t("restore.mode.inPlace")}
              </label>
              <label className="flex items-center gap-2 cursor-pointer text-carbon-text">
                <input
                  type="radio"
                  name={radioName}
                  checked={mode === "files"}
                  onChange={() => setMode("files")}
                  style={{ accentColor: "var(--accent)" }}
                />
                {t("restore.mode.files")}
              </label>
              <label className="flex items-center gap-2 cursor-pointer text-carbon-text">
                <input
                  type="radio"
                  name={radioName}
                  checked={mode === "toFolder"}
                  onChange={() => setMode("toFolder")}
                  style={{ accentColor: "var(--accent)" }}
                />
                {t("restore.mode.toFolder")}
              </label>
            </div>
          </Advanced>

          {effectiveMode === "inPlace" && (
            <div className="flex flex-col gap-2 border-t border-carbon-border pt-2">
              <p className="text-caption text-carbon-textMuted">{t("restore.inPlaceHint")}</p>
              <RestoreAction
                domain="container"
                name={containerName}
                snapshotId={snap.id}
                source={source}
                otherActive={running}
                successMessage={t("restore.completeContainer")}
                t={t}
              />
            </div>
          )}

          {effectiveMode === "files" && (
            <div className="border-t border-carbon-border pt-2">
              <SnapshotFileBrowser
                containerName={containerName}
                snapshotId={snap.id}
                source={source}
                hostMountRoot={hostMountRoot}
                defaultFolder={defaultFolder}
                t={t}
              />
            </div>
          )}

          {effectiveMode === "toFolder" && (
            <div className="border-t border-carbon-border pt-2">
              <RestoreToFolder
                containerName={containerName}
                snapshotId={snap.id}
                source={source}
                hostMountRoot={hostMountRoot}
                defaultFolder={defaultFolder}
                t={t}
              />
            </div>
          )}
        </div>
      )}
      {confirmDialog}
    </div>
  );
}

// DEFAULT_RESTORE_FOLDER pre-fills every restore-to-folder picker when the
// setting is empty. It matches the backend column default.
export const DEFAULT_RESTORE_FOLDER = "user/bombvault/restore";

export function RestorePanel({ name, aliases = [], t, installed = true, open }: RestorePanelProps) {
  const [source, setSource] = useState<RepoSource>("local");
  const [snapshots, setSnapshots] = useState<Snapshot[]>([]);
  const [loading, setLoading] = useState(false);
  // A failed load stays inline rather than in a toast: it describes the
  // section, not a one-off action.
  const [error, setError] = useState<string | null>(null);
  const [restoreFolder, setRestoreFolder] = useState(DEFAULT_RESTORE_FOLDER);
  const [hostMountRoot, setHostMountRoot] = useState("/host/user");

  const [reloadTick, setReloadTick] = useState(0);

  // Seeds the restore-to-folder pickers once the panel opens.
  useEffect(() => {
    if (!open) return;
    getSettings()
      .then((res) => {
        if (res.ok) {
          setRestoreFolder(res.settings.restoreFolder || DEFAULT_RESTORE_FOLDER);
          if (res.hostMountRoot) setHostMountRoot(res.hostMountRoot);
        }
      })
      .catch(() => undefined);
  }, [open]);

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
    listSnapshots(name, source)
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
  }, [open, name, source, reloadTick]); // eslint-disable-line react-hooks/exhaustive-deps -- t() is only read to build a failure message; re-fetching on a language switch would be a wasted round-trip

  if (!open) return null;

  return (
    <div className="mt-2 rounded-card bg-carbon-background px-3 py-1">
      {/* Basic mode always reads the local repo. */}
      <Advanced>
        <div className="flex items-center gap-2 py-2 border-b border-carbon-border">
          <span className="flex items-center gap-1 text-xs text-carbon-textMuted">
            {t("source.label")}
            <InfoBubble tip={t("source.hint")} />
          </span>
          <SourceToggle source={source} onChange={setSource} disabled={loading} domain="containers" />
        </div>
      </Advanced>
      <RecentRunsList name={name} domain="container" t={t} />
      {loading && (
        <p className="py-3 text-xs text-carbon-textMuted">{t("common.loadingBackups")}</p>
      )}
      {error && (
        <p className="py-3 text-xs text-statusFail">{error}</p>
      )}
      {!loading && !error && snapshots.length === 0 && (
        <div className="py-3 flex flex-col gap-1">
          <p className="text-xs text-carbon-textMuted">{t("snapshots.none")}</p>
          {/* A config-only backup has no snapshot. A removed container can be
              recreated from it; an installed one only gets the explanation. */}
          {installed ? (
            <p className="text-xs text-carbon-textMuted">{t("snapshots.configOnlyHint")}</p>
          ) : (
            <RecreateButton name={name} source={source} t={t} />
          )}
        </div>
      )}
      <Advanced when={!loading && !error && snapshots.length >= 2}>
        <CompareSnapshots snapshots={snapshots} containerName={name} source={source} t={t} />
      </Advanced>
      {!loading && snapshots.map((snap) => (
        <SnapshotRow
          key={snap.id}
          snap={snap}
          containerName={name}
          aliases={aliases}
          source={source}
          hostMountRoot={hostMountRoot}
          defaultFolder={restoreFolder}
          onDeleted={() => setReloadTick((n) => n + 1)}
          onTagged={() => setReloadTick((n) => n + 1)}
          t={t}
        />
      ))}
    </div>
  );
}
