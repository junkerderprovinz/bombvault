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
import { InfoBubble } from "./InfoBubble";
import { IconRestore, IconTrash } from "./Sidebar";

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
// carries, and "p1" is an internal orchestrator marker — both are noise in the UI,
// so they're hidden here. They stay in restic's metadata untouched.
const INTERNAL_TAGS = new Set(["p1"]);
function displayTags(snap: Snapshot, containerName: string): string[] {
  const owner = `container:${containerName}`;
  return (snap.tags ?? []).filter((tg) => tg !== owner && !INTERNAL_TAGS.has(tg));
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
  // Default to comparing the two most recent snapshots (older "from" → newer "to").
  const [from, setFrom] = useState(snapshots[1]?.id ?? "");
  const [to, setTo] = useState(snapshots[0]?.id ?? "");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [diff, setDiff] = useState<SnapshotDiff | null>(null);
  const { push } = useToast();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): a
  // failed action toasts AND shakes its button, layered ON TOP of this
  // button's own pre-existing sticky inline error (kept deliberately — see
  // this component's header comment above).
  const [shake, setShake] = useState(0);

  // Re-seed the default pair whenever the snapshot set changes (e.g. toggling the
  // Local/Off-site source reloads a different repo's snapshots). Without this, the
  // selects keep stale IDs from the previous repo and Compare is rejected with
  // "snapshot does not belong to this container".
  useEffect(() => {
    setFrom(snapshots[1]?.id ?? "");
    setTo(snapshots[0]?.id ?? "");
    setDiff(null);
    setError(null);
  }, [snapshots]);

  // GlimStone follow-up pass (v8.0.0) audit note: `diff`/`error` below are
  // deliberately NOT migrated to a toast, unlike this file's SnapshotRow.
  // handleDelete / SnapshotTags.submit siblings. A successful compare renders
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
    "rounded-control bg-carbon-surface3 text-carbon-text text-xs px-2 py-1 bv-field-focus-well max-w-[16rem] truncate";

  return (
    <div className="py-2 border-b border-carbon-border">
      <Button
        label={t("snapshot.compare")}
        labelKey="snapshot.compare"
        tone="neutral"
        onClick={() => setOpen((p) => !p)}
      />
      {open && (
        <div className="mt-2 rounded-card bg-carbon-surface2 p-2 flex flex-col gap-2">
          <p className="text-caption text-carbon-textMuted">{t("snapshot.pickTwo")}</p>
          <div className="flex items-center gap-2 flex-wrap">
            <select value={from} onChange={(e) => setFrom(e.target.value)} disabled={loading} className={selectCls}>
              {snapshots.map((s) => (
                <option key={s.id} value={s.id}>{snapLabel(s)}</option>
              ))}
            </select>
            {/* Compare-direction arrow: implies reading order (from → to), so
                it mirrors under RTL — an inline-block wrapper so scaleX(-1)
                flips the glyph shape itself, not the layout position. */}
            <span className="inline-block text-xs text-carbon-textMuted rtl:-scale-x-100">→</span>
            <select value={to} onChange={(e) => setTo(e.target.value)} disabled={loading} className={selectCls}>
              {snapshots.map((s) => (
                <option key={s.id} value={s.id}>{snapLabel(s)}</option>
              ))}
            </select>
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
  snap,
  containerName,
  source,
  onTagged,
  t,
}: {
  snap: Snapshot;
  containerName: string;
  source: string;
  onTagged: () => void;
  t: T;
}) {
  const [adding, setAdding] = useState(false);
  const [value, setValue] = useState("");
  const [busy, setBusy] = useState(false);
  const { push } = useToast();
  const tags = displayTags(snap, containerName);

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
          className="w-24 rounded-control bg-carbon-surface2 text-carbon-text text-caption px-1.5 py-0.5 bv-field-focus"
        />
      ) : (
        <Button
          label={t("snapshot.tags")}
          labelKey="snapshot.tags"
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
  source,
  hostMountRoot,
  defaultFolder,
  onDeleted,
  onTagged,
  t,
}: {
  snap: Snapshot;
  containerName: string;
  source: RepoSource;
  hostMountRoot: string;
  defaultFolder: string;
  onDeleted: () => void;
  onTagged: () => void;
  t: T;
}) {
  const { advanced } = useAdvanced();
  const progressMap = useProgress();
  // Busy-guard handed to the shared RestoreAction: block a new restore while any
  // OTHER backup/restore/replication runs (this snapshot's own in-flight restore
  // is covered inside RestoreAction via isPending, never self-blocked).
  const running = anyActive(progressMap);
  // Delete only needs to be blocked while THIS container's own op is in flight
  // (deleting a snapshot mid-restore/backup of the same repo). An unrelated
  // container's activity must not disable it — so guard on the row-local key,
  // not the global anyActive.
  const busy = progressMap[`container:${containerName}`]?.active ?? false;
  // The consolidated "Restore…" panel: one toggle, three radio-selected modes.
  const [showRestore, setShowRestore] = useState(false);
  // In basic mode only the in-place restore is offered; the mode radios (files /
  // to-folder) are advanced. Pin the mode to "inPlace" so the panel always renders.
  const [mode, setMode] = useState<RestoreMode>("inPlace");
  const effectiveMode: RestoreMode = advanced ? mode : "inPlace";
  const [deleting, setDeleting] = useState(false);
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): a
  // failed delete toasts AND shakes the delete button.
  const [shake, setShake] = useState(0);

  async function handleDelete() {
    if (!(await confirm(t("snapshots.deleteConfirm")))) return;
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

  // Group name so the three radios are mutually exclusive PER snapshot.
  const radioName = `restore-mode-${snap.id}`;

  return (
    // py-1.5, not the py-2.5 this row used to carry: the two square icon
    // badges in it grew 24px → 32px when every square icon badge in the app
    // was unified on one size, and trimming the row's own vertical padding by
    // the matching 4px per side keeps the collapsed row at exactly the 44px
    // it measured before (verified live, before and after). A bigger badge in
    // a list of unchanged density, rather than a list that grew — see
    // Badge.tsx's "ONE SIZE FOR SQUARE ICON BADGES" block. Config.tsx's
    // ConfigSnapshotRow carries the identical pairing for the same reason.
    <div className="flex flex-col gap-1 py-1.5 border-b border-carbon-border last:border-0">
      <div className="flex items-center gap-3 text-sm">
        {/* Snapshot ID */}
        <span dir="ltr" className="font-mono text-start text-carbon-text text-xs w-20 shrink-0">
          {snap.id.slice(0, 8)}
        </span>
        {/* Time */}
        <span className="text-carbon-textMuted text-xs flex-1">
          {new Date(snap.time).toLocaleString()}
        </span>
        {/* Tags (chips + inline add-tag) — ownership tag hidden. Advanced only. */}
        <Advanced>
          <div className="hidden sm:flex">
            <SnapshotTags snap={snap} containerName={containerName} source={source} onTagged={onTagged} t={t} />
          </div>
        </Advanced>

        {/* Consolidated restore toggle: opens the inline panel with 3 modes.
            Square icon badge (icon-badge round, standing rule: every icon
            badge gets real hue integration + a hover tooltip carrying its
            old label) — was a plain text `<button>` reading "Wiederherstellen…".
            `tone="active"` (icon-only → solid `bg-accent`/`text-accentContrast`,
            Badge.tsx's own `isIconOnly && tone==="active"` branch), no
            `hueIndex` needed: this row lives inside ContainerRow's own
            `.glim-hue` element (RestorePanel is one of that row's own
            disclosure panes), so the ambient CSS custom-property cascade
            already resolves `--accent`/`--accent-contrast` to the row's own
            rainbow position — the exact same already-verified mechanism
            FoldersEditor's own Save/Add badges use (see Containers.tsx).
            IconRestore reuses the app's existing circular-sweep "restore"
            glyph family (Sidebar.tsx's own IconRecovery — see that icon's
            own doc comment; Settings.tsx's separate reset glyph,
            IconResetArrow, deliberately diverged from this family for its
            own harder small-badge-beside-colour-swatches legibility case).
            `size="icon"` — the app's one square-icon-badge size (32px). This
            badge was `size="large"` (24px), measured against its own
            pre-conversion `py-1` text-button self: correct for this control
            in isolation, and wrong in the card, where it sat beside the 32px
            Lokal/Offsite pair and the (then) 28px Jetzt-sichern/Export pair.
            The row's own padding dropped `py-2.5` → `py-1.5` in the same
            change, so the snapshot row still measures the 44px it always did
            with a larger badge inside it — see Badge.tsx's "ONE SIZE FOR
            SQUARE ICON BADGES" block. The old "highlighted while open" `bg-carbon-surface3`
            swap is dropped rather than layered onto a second, competing
            background utility class (Tailwind utilities of equal specificity
            resolve by generated-stylesheet order, not by className list
            order — not a safe way to override Badge's own tone fill):
            `aria-expanded` now carries that state instead, matching every
            other disclosure trigger in this app (StackCard's own chevron
            toggle, ExcludesEditor's assistant toggle) that doesn't
            recolour itself when open either — the panel appearing below is
            already the visible feedback. (Badge/IconTipButton don't carry an
            `aria-expanded` passthrough today, same as the plain `<button>`
            this replaces, which never set one either — no regression.) */}
        <Button
          label={t("restore.open")}
          glyph={<IconRestore />}
          tone="accent"
          onClick={() => setShowRestore((p) => !p)}
        />

        {/* Delete this backup (restic forget) — square icon badge, styled
            EXACTLY like the restore badge beside it and like every other
            icon badge in this card.
              jdp, live review: "Der Löschen-Badge ist auch anders eingefärbt,
            soll nicht so sein, ganz normal in die Farbmodi integrieren." He
            is right, and the previous reasoning was the problem. This badge
            was `tone="neutral"` plus `hover:bg-statusFailBg
            hover:text-statusFail` — a flat grey tile at rest that flashed red
            on hover. Two things were wrong with that: `neutral` is one of the
            tones Badge deliberately exempts from rainbow `hueIndex` (they are
            load-bearing STATUS signals), so this badge took no colour-engine
            position at all and stayed grey in every rainbow palette while its
            siblings picked up the row's hue; and the red hover made it the
            one badge in the card with a bespoke colour treatment.
              Now `tone="active"` with no colour override, identical to the
            restore badge it sits next to: icon-only + active resolves to the
            solid `bg-accent`/`text-accentContrast` pair, and this row lives
            inside ContainerRow's own `.glim-hue` element, so the ordinary CSS
            custom-property cascade paints it in the row's own rainbow
            position — no `hueIndex` needed, the same mechanism the restore
            badge and FoldersEditor's badges already use.
              Nothing about the action becomes ambiguous by dropping the red:
            the destructive meaning is carried by the IconTrash glyph and by
            the `tip` bubble (t("snapshots.delete") — "Löschen"), and the
            action still routes through the existing confirm dialog before
            anything is forgotten. This mirrors the already-shipped decision
            that "Deaktivieren" buttons must not be red either.
              `glim-shake` (the system-wide "failed delete shakes its button"
            rule) survives on `className` — that is behaviour, not colour.
            IconTrash reused verbatim (Sidebar.tsx). */}
        <Button
          key={shake}
          label={t("snapshots.delete")}
          glyph={<IconTrash />}
          tone="accent"
          onClick={() => void handleDelete()}
          disabled={deleting || busy}
          className={shake ? "glim-shake" : ""}
        />
      </div>

      {/* Inline restore panel: radio-selected mode + the UI for that mode. */}
      {showRestore && (
        <div className="mt-1 rounded-card bg-carbon-surface2 p-3 flex flex-col gap-3 text-xs">
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

          {/* In place — the destructive recreate (confirm-gated). */}
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

          {/* Individual files — multi-select file restore (in place / to a folder). */}
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

          {/* To a folder — extract into an alternate folder via the tree picker. */}
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

// DEFAULT_RESTORE_FOLDER is the fallback pre-fill for the restore-to-folder
// picker when the settings value is empty (matches the backend column default).
// Exported so every restore-to-folder picker in the app (containers here, file
// sets in Files.tsx) shares the exact same fallback instead of drifting apart.
export const DEFAULT_RESTORE_FOLDER = "user/bombvault/restore";

export function RestorePanel({ name, t, installed = true, open }: RestorePanelProps) {
  const [source, setSource] = useState<RepoSource>("local");
  const [snapshots, setSnapshots] = useState<Snapshot[]>([]);
  const [loading, setLoading] = useState(false);
  // Section-load error — NOT migrated to a toast (GlimStone follow-up pass,
  // v8.0.0 audit note): same structural "the section failed to load"
  // condition as Files.tsx's FileSetRestorePanel, not a one-shot action.
  const [error, setError] = useState<string | null>(null);
  // Restore-to-folder needs the default folder + host mount root to seed the
  // FolderBrowser. Fetched once the panel is opened (not on mount).
  const [restoreFolder, setRestoreFolder] = useState(DEFAULT_RESTORE_FOLDER);
  const [hostMountRoot, setHostMountRoot] = useState("/host/user");

  const [reloadTick, setReloadTick] = useState(0);

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

  useEffect(() => {
    if (!open) return;
    setLoading(true);
    setError(null);
    listSnapshots(name, source)
      .then((res) => {
        if (res.ok) setSnapshots(res.snapshots ?? []);
        else setError(res.error ?? t("common.loadBackupsFailed"));
      })
      .catch(() => setError(t("common.loadBackupsFailed")))
      .finally(() => setLoading(false));
  }, [open, name, source, reloadTick]); // eslint-disable-line react-hooks/exhaustive-deps -- t() is only read to build a failure message; re-fetching on a language switch would be a wasted round-trip

  if (!open) return null;

  return (
    <div className="mt-2 rounded-card bg-carbon-background px-3 py-1">
      {/* Source (Local / Off-site) toggle is advanced; basic mode uses local. */}
      <Advanced>
        {/* `source.hint` as an InfoBubble on the "Quelle" label, not the
            permanent `text-caption` <p> under the row it used to be — rule
            8's "read once, costs vertical space forever" case. Same
            conversion Flash.tsx got in 63f53d5, applied here in the same pass
            as the three other surviving copies (pages/Config.tsx,
            pages/VMs.tsx, pages/Files.tsx) rather than one tab at a time.
            This is the copy that renders on the CONTAINERS tab — it lives in
            this shared panel rather than in Containers.tsx itself, which is
            why grepping pages/ alone misses it.
              The row keeps its own `py-2` and bottom border: nothing here
            changed height, only the <p> beneath it disappeared. The wrapping
            `flex flex-col gap-1` goes with the <p>, having one child left. */}
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
          {/* A config-only backup (stateless container, no data snapshot) has
              no restic snapshot. If the container is gone, offer to recreate
              it from the saved definition; if it's installed, just explain. */}
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
