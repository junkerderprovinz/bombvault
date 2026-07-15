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

  useEffect(() => {
    setLoading(true);
    listSnapshotFiles(containerName, snapshotId, source)
      .then((res) => {
        if (res.ok) setFiles(res.files ?? []);
        else setError(t("files.loadFailed"));
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

  function handleRestoreSelected() {
    if (selected.size === 0) return;
    if (dest === "toFolder" && !folder.trim()) return;
    // In place overwrites the live files, so keep the explicit confirm.
    if (dest === "inPlace" && !window.confirm(t("files.restoreConfirm"))) return;
    void fire();
  }

  const count = selected.size;

  return (
    <div className="mt-1 rounded-lg border border-carbon-border bg-carbon-surface2 p-2 flex flex-col gap-2">
      <p className="text-[11px] text-carbon-textMuted">{t("files.selectHint")}</p>
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
            <button
              onClick={handleRestoreSelected}
              disabled={isPending || blockedByOther || (dest === "toFolder" && !folder.trim())}
              className="shrink-0 inline-flex items-center rounded-lg bg-accent px-2.5 py-1 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-40 disabled:cursor-not-allowed"
            >
              {isPending ? t("common.restoring") : t("files.restoreSelected").replace("{n}", String(count))}
            </button>
            {blockedByOther && (
              <span className="text-[11px] text-carbon-textMuted">{t(busyPhraseKey(running.phase))}</span>
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
    </div>
  );
}

interface RestorePanelProps {
  name: string;
  t: T;
  // installed=false marks a not-installed (orphan) container: when it has a
  // config-only backup (no snapshots) it can be recreated from the saved config.
  installed?: boolean;
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
  function handle() {
    if (!window.confirm(t("snapshots.recreateConfirm"))) return;
    void fire();
  }
  return (
    <div className="flex flex-col gap-1 py-2">
      <button
        onClick={handle}
        disabled={isPending || blockedByOther || state.phase === "success"}
        className="self-start inline-flex items-center rounded-lg bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
      >
        {isPending ? t("common.restoring") : t("snapshots.recreate")}
      </button>
      {blockedByOther && (
        <span className="text-[11px] text-carbon-textMuted">{t(busyPhraseKey(running.phase))}</span>
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
    <div className="mt-1 rounded-lg border border-carbon-border bg-carbon-surface2 p-2 flex flex-col gap-1.5">
      <p className="text-[11px] text-carbon-textMuted">{t("restore.toFolderHint")}</p>
      <FolderBrowser
        label={t("restore.targetPath")}
        value={path}
        hostMountRoot={hostMountRoot}
        onChange={pickPath}
      />
      <div className="flex items-center gap-2">
        <button
          onClick={() => void fire()}
          disabled={!path.trim() || isPending || blockedByOther || done}
          className="shrink-0 inline-flex items-center rounded-lg bg-accent px-2.5 py-1 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-40 disabled:cursor-not-allowed"
        >
          {isPending ? t("common.restoring") : t("restore.confirm")}
        </button>
        {blockedByOther && (
          <span className="text-[11px] text-carbon-textMuted">{t(busyPhraseKey(running.phase))}</span>
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

  async function run() {
    if (!from || !to || from === to) return;
    setLoading(true);
    setError(null);
    setDiff(null);
    try {
      const res = await diffSnapshots(containerName, from, to, source);
      if (res.ok && res.diff) setDiff(res.diff);
      else setError(res.error ?? "Compare failed");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Network error");
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
    "rounded bg-carbon-background border border-carbon-border text-carbon-text text-xs px-2 py-1 focus:outline-none focus:border-[#78a9ff] max-w-[16rem] truncate";

  return (
    <div className="py-2 border-b border-carbon-border">
      <button
        onClick={() => setOpen((p) => !p)}
        className="flex items-center gap-1.5 text-xs text-carbon-textSub hover:text-carbon-text transition-colors"
      >
        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" className={`transition-transform ${open ? "rotate-90" : ""}`}>
          <path d="M4 2l4 4-4 4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
        {t("snapshot.compare")}
      </button>
      {open && (
        <div className="mt-2 rounded-lg border border-carbon-border bg-carbon-surface2 p-2 flex flex-col gap-2">
          <p className="text-[11px] text-carbon-textMuted">{t("snapshot.pickTwo")}</p>
          <div className="flex items-center gap-2 flex-wrap">
            <select value={from} onChange={(e) => setFrom(e.target.value)} disabled={loading} className={selectCls}>
              {snapshots.map((s) => (
                <option key={s.id} value={s.id}>{snapLabel(s)}</option>
              ))}
            </select>
            <span className="text-xs text-carbon-textMuted">→</span>
            <select value={to} onChange={(e) => setTo(e.target.value)} disabled={loading} className={selectCls}>
              {snapshots.map((s) => (
                <option key={s.id} value={s.id}>{snapLabel(s)}</option>
              ))}
            </select>
            <button
              onClick={() => void run()}
              disabled={loading || !from || !to || from === to}
              className="inline-flex items-center rounded-lg bg-accent px-2.5 py-1 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-40 disabled:cursor-not-allowed"
            >
              {loading ? "…" : t("snapshot.compare")}
            </button>
          </div>
          {error && <p className="text-xs text-[#ff8389] break-words">{error}</p>}
          {diff && (
            <p className="text-xs text-carbon-text font-mono break-words" title={summary}>
              <span className="text-[#6fdc8c]">+{diff.addedFiles}</span> {t("snapshot.added")} ({humanBytes(diff.addedBytes)}),{" "}
              <span className="text-carbon-textSub">~{diff.changedFiles}</span> {t("snapshot.changed")},{" "}
              <span className="text-[#ff8389]">-{diff.removedFiles}</span> {t("snapshot.removed")} ({humanBytes(diff.removedBytes)})
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
  const [err, setErr] = useState<string | null>(null);
  const tags = displayTags(snap, containerName);

  async function submit() {
    const tag = value.trim();
    if (!tag) {
      setAdding(false);
      return;
    }
    setBusy(true);
    setErr(null);
    try {
      const res = await tagSnapshot(containerName, snap.id, [tag], source);
      if (res.ok) {
        setValue("");
        setAdding(false);
        onTagged();
      } else {
        setErr(res.error ?? "Failed");
      }
    } catch (e) {
      setErr(e instanceof Error ? e.message : "Network error");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex items-center gap-1 flex-wrap">
      {tags.map((tg) => (
        <span
          key={tg}
          className="inline-flex items-center rounded bg-carbon-surface3 px-1.5 py-0.5 text-[10px] text-carbon-textSub"
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
          className="w-24 rounded bg-carbon-background border border-carbon-border text-carbon-text text-[10px] px-1.5 py-0.5 focus:outline-none focus:border-[#78a9ff]"
        />
      ) : (
        <button
          onClick={() => setAdding(true)}
          title={t("snapshot.addTag")}
          className="inline-flex items-center rounded border border-carbon-border px-1.5 py-0.5 text-[10px] text-carbon-textMuted hover:bg-carbon-hover hover:text-carbon-text transition-colors"
        >
          + {t("snapshot.tags")}
        </button>
      )}
      {err && <span className="text-[10px] text-[#ff8389]">{err}</span>}
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
  const [deleteErr, setDeleteErr] = useState<string | null>(null);

  async function handleDelete() {
    if (!window.confirm(t("snapshots.deleteConfirm"))) return;
    setDeleting(true);
    setDeleteErr(null);
    try {
      const res = await deleteSnapshot("containers", snap.id, source);
      if (res.ok) onDeleted();
      else setDeleteErr(res.error ?? "Delete failed");
    } catch (err) {
      setDeleteErr(err instanceof Error ? err.message : "Delete failed");
    } finally {
      setDeleting(false);
    }
  }

  // Group name so the three radios are mutually exclusive PER snapshot.
  const radioName = `restore-mode-${snap.id}`;

  return (
    <div className="flex flex-col gap-1 py-2.5 border-b border-carbon-border last:border-0">
      <div className="flex items-center gap-3 text-sm">
        {/* Snapshot ID */}
        <span className="font-mono text-carbon-text text-xs w-20 shrink-0">
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

        {/* Consolidated restore toggle: opens the inline panel with 3 modes */}
        <button
          onClick={() => setShowRestore((p) => !p)}
          className={`shrink-0 rounded-lg border border-carbon-border px-2.5 py-1 text-xs transition-colors ${
            showRestore ? "bg-carbon-surface3 text-carbon-text" : "text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
          }`}
        >
          {t("restore.open")}
        </button>

        {/* Delete this backup (restic forget) */}
        <button
          onClick={() => void handleDelete()}
          disabled={deleting || busy}
          title={t("snapshots.delete")}
          className="shrink-0 rounded-lg border border-carbon-border px-2 py-1 text-xs text-carbon-textSub hover:bg-[#3a1c1c] hover:text-[#ff8389] transition-colors disabled:opacity-50"
        >
          {deleting ? "…" : t("snapshots.delete")}
        </button>
      </div>
      {deleteErr && <p className="text-xs text-[#ff8389] pl-24 break-words">{deleteErr}</p>}

      {/* Inline restore panel: radio-selected mode + the UI for that mode. */}
      {showRestore && (
        <div className="mt-1 rounded-lg border border-carbon-border bg-carbon-surface2 p-3 flex flex-col gap-3 text-xs">
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
              <p className="text-[11px] text-carbon-textMuted">{t("restore.inPlaceHint")}</p>
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
    </div>
  );
}

// DEFAULT_RESTORE_FOLDER is the fallback pre-fill for the restore-to-folder
// picker when the settings value is empty (matches the backend column default).
const DEFAULT_RESTORE_FOLDER = "user/bombvault/restore";

export function RestorePanel({ name, t, installed = true }: RestorePanelProps) {
  const [open, setOpen] = useState(false);
  const [source, setSource] = useState<RepoSource>("local");
  const [snapshots, setSnapshots] = useState<Snapshot[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // Restore-to-folder needs the default folder + host mount root to seed the
  // FolderBrowser. Fetched once the panel is opened (not on mount).
  const [restoreFolder, setRestoreFolder] = useState(DEFAULT_RESTORE_FOLDER);
  const [hostMountRoot, setHostMountRoot] = useState("/host/user");

  function toggle() {
    setOpen((prev) => !prev);
  }

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
        else setError(res.error ?? "Failed to load backups");
      })
      .catch(() => setError("Failed to load backups"))
      .finally(() => setLoading(false));
  }, [open, name, source, reloadTick]);

  return (
    <div className="mt-1">
      <button
        onClick={toggle}
        className="flex items-center gap-1.5 text-xs text-carbon-textSub hover:text-carbon-text transition-colors"
      >
        <svg
          width="12"
          height="12"
          viewBox="0 0 12 12"
          fill="none"
          className={`transition-transform ${open ? "rotate-90" : ""}`}
        >
          <path
            d="M4 2l4 4-4 4"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
        {t("snapshots.title")}
      </button>

      {open && (
        <div className="mt-2 rounded-lg border border-carbon-border bg-carbon-background px-3 py-1">
          {/* Source (Local / Off-site) toggle is advanced; basic mode uses local. */}
          <Advanced>
            <div className="flex flex-col gap-1 py-2 border-b border-carbon-border">
              <div className="flex items-center gap-2">
                <span className="text-xs text-carbon-textMuted">{t("source.label")}</span>
                <SourceToggle source={source} onChange={setSource} disabled={loading} />
              </div>
              <p className="text-[11px] text-carbon-textMuted">{t("source.hint")}</p>
            </div>
          </Advanced>
          <RecentRunsList name={name} domain="container" t={t} />
          {loading && (
            <p className="py-3 text-xs text-carbon-textMuted">{t("common.loadingBackups")}</p>
          )}
          {error && (
            <p className="py-3 text-xs text-[#ff8389]">{error}</p>
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
      )}
    </div>
  );
}
