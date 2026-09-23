import { useEffect, useRef, useState } from "react";
import { listSnapshots, restore, listSnapshotFiles, restoreContainerFiles, restoreContainerToPath, diffSnapshots, tagSnapshot, getSettings } from "../lib/api";
import type { Snapshot, FileEntry, SnapshotDiff } from "../lib/api";
import type { useT } from "../lib/i18n";
import { Advanced, useAdvanced } from "../lib/advanced";
import { useBackupWatch } from "../lib/backupWatch";
import { useProgress, anyActive, busyPhraseKey } from "../lib/progress";
import { SNAPSHOT_MISSING } from "../lib/timeline";
import { RestoreProgress } from "./restore/RestoreProgress";
import { RestoreAction } from "./restore/RestoreAction";
import { FolderBrowser } from "./FolderBrowser";
import { RecentRunsList } from "./RecentRunsList";
import { SnapshotFileTree } from "./SnapshotFileTree";
import { loadErrorMessage } from "../lib/errors";
import { useConfirm } from "../lib/useConfirm";
import { useToast } from "../lib/toast";
import { Button } from "./Button";
import { SelectField } from "./SelectField";
import { IconRestore } from "./Sidebar";
import { IconDisclosure } from "./IconDisclosure";
import { Timeline, type TimelinePick } from "./timeline/Timeline";

type T = ReturnType<typeof useT>["t"];

// humanBytes formats a byte count with a binary (1024) unit and one decimal
// (mirrors the Dashboard's storage card so sizes read the same everywhere).
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

// displayTags drops internal marker tags and shows only user-facing tags as chips.
// The ownership tag (container:<name>) is an implementation detail every snapshot
// carries, "p1" is an internal orchestrator marker and "bv:direct" marks a
// snapshot written straight into a direct repository; all three are noise in the
// UI, so they're hidden here. They stay in restic's metadata untouched.
const INTERNAL_TAGS = new Set(["p1", "bv:direct"]);
export function displayTags(tags: string[], containerName: string): string[] {
  const owner = `container:${containerName}`;
  return tags.filter((tg) => tg !== owner && !INTERNAL_TAGS.has(tg));
}

// SnapshotFileBrowser lists a snapshot's files for multi-select restore: tick any
// files/folders (a collapsible folder tree when unfiltered, or a flat matched list
// while filtering), choose a destination (in place, or an alternate folder), then
// restore the whole selection at once.
function SnapshotFileBrowser({
  containerName,
  snapshotId,
  source,
  hostMountRoot,
  defaultFolder,
  onMissing,
  t,
}: {
  containerName: string;
  snapshotId: string;
  source: string;
  hostMountRoot: string;
  defaultFolder: string;
  onMissing: () => void;
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

  // Fire-and-watch (see useBackupWatch): the server validates + resolves the
  // target synchronously, acks with {started, target}, and runs the restic work
  // detached — so a long restore survives this panel (or the whole browser)
  // going away; the run history is the source of truth for the outcome.
  const cancelledRef = useRef(false);
  const { state: restoreState, fire, reset, isPending } = useBackupWatch({
    progressKey: `container:${containerName}`,
    kind: "restore",
    start: async () => {
      const paths = [...selected];
      const targetPath = dest === "toFolder" ? folder.trim() : "";
      const res = await restoreContainerFiles(containerName, snapshotId, paths, targetPath, true, source);
      if (res.code === SNAPSHOT_MISSING) onMissing();
      if (res.ok) setRestoredTarget(res.target ?? "");
      return res;
    },
    matchRun: (r) => r.domain === "container" && r.target === containerName,
    cancelledRef,
  });
  const progressMap = useProgress();
  const prog = progressMap[`container:${containerName}`];
  // Busy-guard: block a new restore while any OTHER backup/restore/replication
  // runs (this item's own in-flight op is covered by isPending, never blocked).
  const running = anyActive(progressMap);
  const blockedByOther = running.active && !isPending;
  const { confirm, confirmDialog } = useConfirm();

  useEffect(() => {
    setLoading(true);
    listSnapshotFiles(containerName, snapshotId, source)
      .then((res) => {
        // #129 — show the server's own reason (e.g. a stale repo lock) when it
        // sent one; the generic message is only for a plain network failure.
        if (res.ok) setFiles(res.files ?? []);
        else setError(loadErrorMessage(res, t("files.loadFailed")));
      })
      .catch(() => setError(t("files.loadFailed")))
      .finally(() => setLoading(false));
  }, [containerName, snapshotId, source, t]);

  // toggle flips one path in the selection set; a new selection clears any prior
  // result banner so it can't linger over a fresh, unrun selection.
  function toggle(p: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(p)) next.delete(p);
      else next.add(p);
      return next;
    });
    reset();
  }

  // Changing the destination or the target folder also invalidates a prior result
  // banner, so a stale "Restored to …" can't linger over a different, unrun choice.
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
    // In place overwrites the live files, so keep the explicit confirm.
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

      {/* Destination + restore-selected action — shown once something is ticked. */}
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
  t: T;
  // installed=false marks a not-installed (orphan) container: when it has a
  // config-only backup (no snapshots) it can be recreated from the saved config.
  installed?: boolean;
  /** Whether the panel's content is expanded. GlimStone follow-up round (jdp,
   *  live-review: "Können wir hier Buttons machen die alle in einer Zeile
   *  stehen?" — Containers.tsx's five stacked disclosure triggers, this one
   *  included, became one shared row of chip buttons): the trigger row (the
   *  chevron button + the "Backups" label, formerly rendered by this
   *  component itself) moved up into ContainerRow's own shared Selector strip
   *  — see that call site's own comment. This component no longer owns an
   *  `open` boolean or renders a trigger of its own; it is purely the content
   *  pane, shown or hidden by the CALLER's own state, the same "controlled,
   *  not self-toggling" shape FoldersEditor/StopContainersEditor/
   *  ExcludesEditor/HooksEditor (Containers.tsx) all took on in the same
   *  pass. `lastBackupText` (the flush-right "Letztes Backup: …" fact that
   *  used to share the trigger's own line) moved with the trigger — the
   *  caller renders it directly next to the shared button row instead, since
   *  it is always-visible summary data, not part of this expandable content. */
  open: boolean;
}

// RecreateButton recreates a not-installed container from its saved definition
// (a config-only backup has no restic snapshot to restore). Calls the normal
// restore with "latest", which the backend resolves to a recreate-only restore.
//
// Fire-and-watch (see useBackupWatch): the POST is only the async ACK — the
// recreate runs detached on the server, so the real outcome (the recorded run)
// must be watched. Treating the ack as final rendered detached failures green.
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

// RestoreToFolder extracts a whole snapshot into an ALTERNATE folder under the
// host mount — non-destructive: the running container is never touched. It uses
// the shared FolderBrowser (a folder-tree picker) pre-filled with the default
// restore folder, calls restoreContainerToPath, and shows the resolved target
// path on success (errors inline).
function RestoreToFolder({
  containerName,
  snapshotId,
  source,
  hostMountRoot,
  defaultFolder,
  onMissing,
  t,
}: {
  containerName: string;
  snapshotId: string;
  source: string;
  hostMountRoot: string;
  defaultFolder: string;
  onMissing: () => void;
  t: T;
}) {
  const [path, setPath] = useState(defaultFolder);
  const [target, setTarget] = useState("");

  // Fire-and-watch (see useBackupWatch): the server validates + resolves the
  // target synchronously, acks with {started, target}, and runs the (possibly
  // multi-hour) extraction detached — issue #24: awaiting it held the request
  // open until the browser/proxy dropped it, killing restic mid-restore. The
  // run history is the source of truth; closing the panel is safe.
  const cancelledRef = useRef(false);
  const { state, fire, reset, isPending } = useBackupWatch({
    progressKey: `container:${containerName}`,
    kind: "restore",
    start: async () => {
      const p = path.trim();
      const res = await restoreContainerToPath(containerName, snapshotId, p, source);
      if (res.code === SNAPSHOT_MISSING) onMissing();
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
    reset(); // a stale "Restored to …" must not linger over a different, unrun choice
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

// snapLabel renders a snapshot's short id + time for the compare selects.
function snapLabel(snap: Snapshot): string {
  return `${snap.id.slice(0, 8)} · ${new Date(snap.time).toLocaleString()}`;
}

// CompareSnapshots is a collapsible "Compare" panel: pick two snapshots (two
// selects, defaulting to the newest pair) and show the diff summary of what
// changed between them (restic diff). Visually consistent with the Files /
// Restore-to-folder panels.
function CompareSnapshots({ containerName, t }: { containerName: string; t: T }) {
  const [open, setOpen] = useState(false);
  const [snapshots, setSnapshots] = useState<Snapshot[]>([]);
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [diff, setDiff] = useState<SnapshotDiff | null>(null);
  const { push } = useToast();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): a
  // failed action toasts AND shakes its button, layered ON TOP of this
  // button's own pre-existing sticky inline error (kept deliberately — see
  // this component's header comment above).
  const [shake, setShake] = useState(0);

  // renderActions only ever gets one picked mark at a time, so compare reads
  // its own snapshot list at the container's own location once it opens, and
  // seeds the default pair (older "from" → newer "to") from what comes back.
  useEffect(() => {
    if (!open) return;
    listSnapshots(containerName, "local")
      .then((res) => {
        const list = res.ok ? (res.snapshots ?? []) : [];
        setSnapshots(list);
        setFrom(list[1]?.id ?? "");
        setTo(list[0]?.id ?? "");
        setDiff(null);
        setError(null);
      })
      .catch(() => setSnapshots([]));
  }, [open, containerName]);

  // GlimStone follow-up pass (v8.0.0) audit note: `diff`/`error` below are
  // deliberately NOT migrated to a toast, unlike this file's SnapshotTags.submit
  // sibling. A successful compare renders
  // a real comparison RESULT the user reads at their own pace — added/changed/
  // removed file counts and byte totals — not a one-shot completion ping; the
  // same "reference value" reasoning ExportButton and RestoreProgress's
  // restored-to path already established. `error` stays paired with it for
  // the same reason ExportButton/VMExportButton's own error stays inline next
  // to their "done" result: the two are mutually exclusive views of the SAME
  // last-compare outcome (a fresh run clears whichever one is showing), so
  // splitting them onto different UI surfaces (one ephemeral toast, one
  // sticky inline result) would read as inconsistent.
  async function run() {
    if (!from || !to || from === to) return;
    setLoading(true);
    setError(null);
    setDiff(null);
    try {
      const res = await diffSnapshots(containerName, from, to, "local");
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
            {/* Compare-direction arrow: implies reading order (from → to), so
                it mirrors under RTL — an inline-block wrapper so scaleX(-1)
                flips the glyph shape itself, not the layout position. */}
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

// SnapshotTags renders a snapshot's (non-ownership) tags as small chips plus a
// tiny inline "add tag" input. On submit it calls tagSnapshot and asks the
// parent to refresh so the new chip appears.
function SnapshotTags({
  tags,
  snapshotId,
  containerName,
  source,
  onTagged,
  t,
}: {
  tags: string[];
  snapshotId: string;
  containerName: string;
  source: string;
  onTagged: () => void;
  t: T;
}) {
  const [adding, setAdding] = useState(false);
  const [value, setValue] = useState("");
  const [busy, setBusy] = useState(false);
  const { push } = useToast();
  const shown = displayTags(tags, containerName);

  // NOTE (Task 2 audit, GlimStone standing rule sweep): deliberately NOT given
  // a `.glim-shake` here, unlike this file's other fixes — there is no
  // dedicated submit button, only this input's onBlur, and the established
  // shake mechanism replays by giving the element a fresh `key` (forcing an
  // unmount+remount). Unmounting a FOCUSED input fires a native blur first,
  // which would re-invoke submit() with the same still-bad value — a
  // shake-triggered infinite retry loop. The toast (below, pre-existing)
  // still fires; the animation is the one piece left as a follow-up pending a
  // non-remount replay mechanism (e.g. a rAF class-remove-then-readd).
  async function submit() {
    const tag = value.trim();
    if (!tag) {
      setAdding(false);
      return;
    }
    setBusy(true);
    try {
      const res = await tagSnapshot(containerName, snapshotId, [tag], source);
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
      {shown.map((tg) => (
        <span
          key={tg}
          className="inline-flex items-center rounded-control bg-carbon-surface3 px-1.5 py-0.5 text-caption text-carbon-textSub"
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

function SnapshotActions({
  pick,
  containerName,
  hostMountRoot,
  defaultFolder,
  t,
}: {
  pick: TimelinePick;
  containerName: string;
  hostMountRoot: string;
  defaultFolder: string;
  t: T;
}) {
  const { advanced } = useAdvanced();
  const running = anyActive(useProgress());
  const [showRestore, setShowRestore] = useState(false);
  // In basic mode only the in-place restore is offered; the mode radios (files /
  // to-folder) are advanced. Pin the mode to "inPlace" so the panel always renders.
  const [mode, setMode] = useState<RestoreMode>("inPlace");
  const effectiveMode: RestoreMode = advanced ? mode : "inPlace";
  // Group name so the three radios are mutually exclusive PER row.
  const radioName = `restore-mode-${pick.row.key}`;

  return (
    <>
      {/* Tags (chips + inline add-tag) — ownership tag hidden. Advanced only. */}
      <Advanced>
        <div className="hidden sm:flex">
          <SnapshotTags
            tags={pick.mark.tags}
            snapshotId={pick.snapshotId}
            containerName={containerName}
            source={pick.source}
            onTagged={pick.refresh}
            t={t}
          />
        </div>
      </Advanced>
      {/* Square icon badge, no hueIndex needed: this row sits inside
          ContainerRow's own `.glim-hue` element, so the ambient rainbow
          position already applies (FoldersEditor's Save/Add badges use the
          same mechanism). The timeline row itself carries id, time and
          delete; this toggle only opens the inline restore panel. */}
      <Button
        label={t("restore.open")}
        labelKey="restore.open"
        glyph={<IconRestore />}
        tone="accent"
        onClick={() => setShowRestore((p) => !p)}
      />
      {showRestore && (
        <div className="basis-full mt-1 rounded-card bg-carbon-surface2 p-3 flex flex-col gap-3 text-xs">
          {/* Mode radios (Individual files / To a folder) are advanced; in basic
              mode only the in-place restore below is shown. */}
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
                snapshotId={pick.snapshotId}
                source={pick.source}
                otherActive={running}
                successMessage={t("restore.completeContainer")}
                onMissing={pick.onMissing}
                t={t}
              />
            </div>
          )}
          {effectiveMode === "files" && (
            <div className="border-t border-carbon-border pt-2">
              <SnapshotFileBrowser
                containerName={containerName}
                snapshotId={pick.snapshotId}
                source={pick.source}
                hostMountRoot={hostMountRoot}
                defaultFolder={defaultFolder}
                onMissing={pick.onMissing}
                t={t}
              />
            </div>
          )}
          {effectiveMode === "toFolder" && (
            <div className="border-t border-carbon-border pt-2">
              <RestoreToFolder
                containerName={containerName}
                snapshotId={pick.snapshotId}
                source={pick.source}
                hostMountRoot={hostMountRoot}
                defaultFolder={defaultFolder}
                onMissing={pick.onMissing}
                t={t}
              />
            </div>
          )}
        </div>
      )}
    </>
  );
}

// DEFAULT_RESTORE_FOLDER is the fallback pre-fill for the restore-to-folder
// picker when the settings value is empty (matches the backend column default).
// Exported so every restore-to-folder picker in the app (containers here, file
// sets in Files.tsx) shares the exact same fallback instead of drifting apart.
export const DEFAULT_RESTORE_FOLDER = "user/bombvault/restore";

export function RestorePanel({ name, t, installed = true, open }: RestorePanelProps) {
  // Restore-to-folder needs the default folder + host mount root to seed the
  // FolderBrowser. Fetched once the panel is opened (not on mount).
  const [restoreFolder, setRestoreFolder] = useState(DEFAULT_RESTORE_FOLDER);
  const [hostMountRoot, setHostMountRoot] = useState("/host/user");

  // Load the default restore folder + host mount root the first time the panel
  // is opened, so the restore-to-folder picker can pre-fill them.
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

  if (!open) return null;

  return (
    <div className="mt-2 rounded-card bg-carbon-background px-3 py-1">
      <RecentRunsList name={name} domain="container" t={t} />
      <Advanced>
        <CompareSnapshots containerName={name} t={t} />
      </Advanced>
      <Timeline
        domain="containers"
        itemKey={name}
        itemName={name}
        open={open}
        header={(rows, places) => {
          // A place nobody has read may hold every backup this container has,
          // so neither sentence below is true yet: the container would be
          // called config-only, or offered a recreate from an empty local place.
          const answered = places.length > 0 && places.every((p) => p.state === "read");
          if (rows.length > 0 || !answered) return null;
          if (installed) return <p className="py-2 text-xs text-carbon-textMuted">{t("snapshots.configOnlyHint")}</p>;
          return <RecreateButton name={name} source="local" t={t} />;
        }}
        renderActions={(pick) => (
          <SnapshotActions
            pick={pick}
            containerName={name}
            hostMountRoot={hostMountRoot}
            defaultFolder={restoreFolder}
            t={t}
          />
        )}
      />
    </div>
  );
}
