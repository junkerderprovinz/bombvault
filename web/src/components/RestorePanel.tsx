import { useEffect, useMemo, useState } from "react";
import { listSnapshots, restore, listSnapshotFiles, restoreContainerFile, restoreContainerToPath, deleteSnapshot, diffSnapshots, tagSnapshot } from "../lib/api";
import type { Snapshot, FileEntry, SnapshotDiff } from "../lib/api";
import type { useT } from "../lib/i18n";
import { SourceToggle, type RepoSource } from "./SourceToggle";

type T = ReturnType<typeof useT>["t"];

// Cap the rendered file list — an appdata snapshot can hold thousands of nodes;
// rendering them all would jank the UI. Users narrow with the filter box.
const FILE_DISPLAY_CAP = 500;

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

// displayTags drops the internal ownership tag (container:<name>) and shows only
// any OTHER tags as chips — the ownership tag is an implementation detail every
// snapshot carries, so showing it would just be noise.
function displayTags(snap: Snapshot, containerName: string): string[] {
  const owner = `container:${containerName}`;
  return (snap.tags ?? []).filter((tg) => tg !== owner);
}

// ---------------------------------------------------------------------------
// File tree (collapsible folder browser for file-level restore)
// ---------------------------------------------------------------------------

interface TreeNode {
  name: string;
  path: string; // full absolute path (matches FileEntry.path)
  type: "dir" | "file";
  children: Map<string, TreeNode>;
}

// buildTree turns restic's flat file list into a nested folder tree keyed by
// path segment, so the browser can be expanded/collapsed like a file manager.
function buildTree(files: FileEntry[]): TreeNode {
  const root: TreeNode = { name: "", path: "", type: "dir", children: new Map() };
  for (const f of files) {
    const segs = f.path.split("/").filter(Boolean);
    let node = root;
    let acc = "";
    segs.forEach((seg, i) => {
      acc += "/" + seg;
      let child = node.children.get(seg);
      if (!child) {
        const isLast = i === segs.length - 1;
        const type: "dir" | "file" = isLast && f.type === "file" ? "file" : "dir";
        child = { name: seg, path: acc, type, children: new Map() };
        node.children.set(seg, child);
      }
      node = child;
    });
  }
  return root;
}

// FileRow restores a single file/dir back to its original location (in-place).
function FileRow({
  containerName,
  snapshotId,
  file,
  source,
  t,
}: {
  containerName: string;
  snapshotId: string;
  file: FileEntry;
  source: string;
  t: T;
}) {
  const [state, setState] = useState<RestoreState>({ phase: "idle" });

  async function handleRestore() {
    if (!window.confirm(t("files.restoreConfirm"))) return;
    setState({ phase: "pending" });
    try {
      const res = await restoreContainerFile(containerName, snapshotId, file.path, true, source);
      if (res.ok) setState({ phase: "success" });
      else setState({ phase: "error", message: res.error ?? "Restore failed" });
    } catch (err) {
      setState({ phase: "error", message: err instanceof Error ? err.message : "Network error" });
    }
  }

  return (
    <div className="flex items-center gap-2 py-1 text-xs border-b border-carbon-border last:border-0">
      <span className="font-mono text-carbon-textSub flex-1 truncate" title={file.path}>
        {file.type === "dir" ? "📁 " : ""}
        {file.path}
      </span>
      {state.phase === "success" ? (
        <span className="text-[#6fdc8c] shrink-0">✓ {t("files.restored")}</span>
      ) : state.phase === "error" ? (
        <span className="text-[#ff8389] shrink-0 max-w-[14rem] truncate" title={state.message}>
          {state.message}
        </span>
      ) : (
        <button
          onClick={() => void handleRestore()}
          disabled={state.phase === "pending"}
          className="shrink-0 rounded bg-carbon-surface3 px-2 py-0.5 text-carbon-text hover:bg-carbon-hover transition-colors disabled:opacity-50"
        >
          {state.phase === "pending" ? "…" : t("files.restore")}
        </button>
      )}
    </div>
  );
}

// sortNodes orders tree children: directories first, then alphabetically.
function sortNodes(a: TreeNode, b: TreeNode): number {
  if (a.type !== b.type) return a.type === "dir" ? -1 : 1;
  return a.name.localeCompare(b.name, undefined, { sensitivity: "base" });
}

// TreeRow renders one node of the collapsible file tree. Directories expand /
// collapse; any node (file or folder) can be restored to its original location.
function TreeRow({
  containerName,
  snapshotId,
  node,
  depth,
  source,
  t,
}: {
  containerName: string;
  snapshotId: string;
  node: TreeNode;
  depth: number;
  source: string;
  t: T;
}) {
  const [expanded, setExpanded] = useState(depth === 0); // top level open by default
  const [state, setState] = useState<RestoreState>({ phase: "idle" });
  const isDir = node.type === "dir";
  const kids = isDir ? Array.from(node.children.values()).sort(sortNodes) : [];

  async function handleRestore() {
    if (!window.confirm(t("files.restoreConfirm"))) return;
    setState({ phase: "pending" });
    try {
      const res = await restoreContainerFile(containerName, snapshotId, node.path, true, source);
      if (res.ok) setState({ phase: "success" });
      else setState({ phase: "error", message: res.error ?? "Restore failed" });
    } catch (err) {
      setState({ phase: "error", message: err instanceof Error ? err.message : "Network error" });
    }
  }

  return (
    <div>
      <div
        className="flex items-center gap-1 py-0.5 text-xs rounded hover:bg-carbon-hover"
        style={{ paddingLeft: depth * 14 }}
      >
        {isDir ? (
          <button
            onClick={() => setExpanded((e) => !e)}
            className="w-4 shrink-0 text-carbon-textMuted hover:text-carbon-text"
            aria-label={expanded ? "collapse" : "expand"}
          >
            <svg width="10" height="10" viewBox="0 0 12 12" fill="none" className={`transition-transform ${expanded ? "rotate-90" : ""}`}>
              <path d="M4 2l4 4-4 4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </button>
        ) : (
          <span className="w-4 shrink-0" />
        )}
        <span className="shrink-0">{isDir ? "📁" : "📄"}</span>
        <span className="font-mono text-carbon-textSub flex-1 truncate" title={node.path}>
          {node.name}
        </span>
        {state.phase === "success" ? (
          <span className="text-[#6fdc8c] shrink-0">✓ {t("files.restored")}</span>
        ) : state.phase === "error" ? (
          <span className="text-[#ff8389] shrink-0 max-w-[12rem] truncate" title={state.message}>
            {state.message}
          </span>
        ) : (
          <button
            onClick={() => void handleRestore()}
            disabled={state.phase === "pending"}
            className="shrink-0 rounded bg-carbon-surface3 px-2 py-0.5 text-carbon-text hover:bg-carbon-hover transition-colors disabled:opacity-50"
          >
            {state.phase === "pending" ? "…" : t("files.restore")}
          </button>
        )}
      </div>
      {isDir && expanded && kids.map((c) => (
        <TreeRow key={c.path} containerName={containerName} snapshotId={snapshotId} node={c} depth={depth + 1} source={source} t={t} />
      ))}
    </div>
  );
}

// SnapshotFileBrowser lists the files in a snapshot for file-level restore: a
// collapsible folder tree when unfiltered, or a flat matched list while filtering.
function SnapshotFileBrowser({
  containerName,
  snapshotId,
  source,
  t,
}: {
  containerName: string;
  snapshotId: string;
  source: string;
  t: T;
}) {
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState("");

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

  const q = filter.trim().toLowerCase();
  const matched = q ? files.filter((f) => f.path.toLowerCase().includes(q)) : files;
  const shown = matched.slice(0, FILE_DISPLAY_CAP);
  const tree = useMemo(() => buildTree(files), [files]);
  const topLevel = Array.from(tree.children.values()).sort(sortNodes);

  return (
    <div className="mt-1 ml-24 rounded-lg border border-carbon-border bg-carbon-surface2 p-2 flex flex-col gap-2">
      <input
        type="text"
        value={filter}
        onChange={(e) => setFilter(e.target.value)}
        placeholder={t("files.filterPlaceholder")}
        spellCheck={false}
        className="rounded bg-carbon-background border border-carbon-border text-carbon-text text-xs px-2 py-1 focus:outline-none focus:border-[#78a9ff]"
      />
      {loading && <p className="text-xs text-carbon-textMuted">…</p>}
      {error && <p className="text-xs text-[#ff8389]">{error}</p>}
      {!loading && !error && (q ? matched.length === 0 : topLevel.length === 0) && (
        <p className="text-xs text-carbon-textMuted">{t("files.none")}</p>
      )}
      {/* Filtering → flat matched list (easier to scan); otherwise → folder tree. */}
      {!loading && q && shown.length > 0 && (
        <div className="max-h-64 overflow-y-auto">
          {shown.map((f) => (
            <FileRow key={f.path} containerName={containerName} snapshotId={snapshotId} file={f} source={source} t={t} />
          ))}
        </div>
      )}
      {!loading && q && matched.length > shown.length && (
        <p className="text-xs text-carbon-textMuted">{t("files.more")}</p>
      )}
      {!loading && !q && topLevel.length > 0 && (
        <div className="max-h-64 overflow-y-auto">
          {topLevel.map((n) => (
            <TreeRow key={n.path} containerName={containerName} snapshotId={snapshotId} node={n} depth={0} source={source} t={t} />
          ))}
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
function RecreateButton({ name, source, t }: { name: string; source: string; t: T }) {
  const [state, setState] = useState<RestoreState>({ phase: "idle" });
  async function handle() {
    if (!window.confirm(t("snapshots.recreateConfirm"))) return;
    setState({ phase: "pending" });
    try {
      const res = await restore(name, "latest", true, source);
      if (res.ok) setState({ phase: "success" });
      else setState({ phase: "error", message: res.error ?? "Recreate failed" });
    } catch (err) {
      setState({ phase: "error", message: err instanceof Error ? err.message : "Network error" });
    }
  }
  return (
    <div className="flex flex-col gap-1 py-2">
      <button
        onClick={() => void handle()}
        disabled={state.phase === "pending" || state.phase === "success"}
        className="self-start inline-flex items-center rounded-lg bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
      >
        {state.phase === "pending" ? t("common.restoring") : t("snapshots.recreate")}
      </button>
      {state.phase === "success" && (
        <p className="text-xs text-[#6fdc8c]">Recreate started — the container is being recreated.</p>
      )}
      {state.phase === "error" && (
        <p className="text-xs text-[#ff8389] break-words">{state.message}</p>
      )}
    </div>
  );
}

type RestoreState =
  | { phase: "idle" }
  | { phase: "pending" }
  | { phase: "success" }
  | { phase: "error"; message: string };

// RestoreToFolder extracts a whole snapshot into an ALTERNATE folder under the
// host mount — non-destructive: the running container is never touched. It
// reveals a path input + a confirm button, calls restoreContainerToPath, and
// shows the resolved target path on success (errors inline).
function RestoreToFolder({
  containerName,
  snapshotId,
  source,
  t,
}: {
  containerName: string;
  snapshotId: string;
  source: string;
  t: T;
}) {
  const [path, setPath] = useState("");
  const [state, setState] = useState<RestoreState>({ phase: "idle" });
  const [target, setTarget] = useState("");

  async function handle() {
    const p = path.trim();
    if (!p) return;
    setState({ phase: "pending" });
    try {
      const res = await restoreContainerToPath(containerName, snapshotId, p, source);
      if (res.ok) {
        setTarget(res.target ?? p);
        setState({ phase: "success" });
      } else {
        setState({ phase: "error", message: res.error ?? "Restore failed" });
      }
    } catch (err) {
      setState({ phase: "error", message: err instanceof Error ? err.message : "Network error" });
    }
  }

  const isPending = state.phase === "pending";
  return (
    <div className="mt-1 ml-24 rounded-lg border border-carbon-border bg-carbon-surface2 p-2 flex flex-col gap-1.5">
      <p className="text-[11px] text-carbon-textMuted">{t("restore.toFolderHint")}</p>
      <div className="flex items-center gap-2">
        <span className="text-xs text-carbon-textSub shrink-0">{t("restore.targetPath")}</span>
        <input
          type="text"
          value={path}
          onChange={(e) => setPath(e.target.value)}
          placeholder={`user/restore/${containerName}`}
          spellCheck={false}
          disabled={isPending || state.phase === "success"}
          className="flex-1 rounded bg-carbon-background border border-carbon-border text-carbon-text text-xs px-2 py-1 focus:outline-none focus:border-[#78a9ff]"
        />
        <button
          onClick={() => void handle()}
          disabled={!path.trim() || isPending || state.phase === "success"}
          className="shrink-0 inline-flex items-center rounded-lg bg-accent px-2.5 py-1 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-40 disabled:cursor-not-allowed"
        >
          {isPending ? t("common.restoring") : t("restore.confirm")}
        </button>
      </div>
      {state.phase === "success" && (
        <p className="text-xs text-[#6fdc8c] break-words">
          {t("restore.restoredTo").replace("{path}", target)}
        </p>
      )}
      {state.phase === "error" && (
        <p className="text-xs text-[#ff8389] break-words">{state.message}</p>
      )}
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

function SnapshotRow({
  snap,
  containerName,
  source,
  onDeleted,
  onTagged,
  t,
}: {
  snap: Snapshot;
  containerName: string;
  source: string;
  onDeleted: () => void;
  onTagged: () => void;
  t: T;
}) {
  const [confirmed, setConfirmed] = useState(false);
  const [restoreState, setRestoreState] = useState<RestoreState>({ phase: "idle" });
  const [showFiles, setShowFiles] = useState(false);
  const [showRestoreTo, setShowRestoreTo] = useState(false);
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

  async function handleRestore() {
    if (!confirmed) return;
    setRestoreState({ phase: "pending" });
    try {
      const res = await restore(containerName, snap.id, true, source);
      if (res.ok) {
        setRestoreState({ phase: "success" });
      } else {
        setRestoreState({ phase: "error", message: res.error ?? "Restore failed" });
      }
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Network error";
      setRestoreState({ phase: "error", message: msg });
    }
  }

  const isPending = restoreState.phase === "pending";

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
        {/* Tags (chips + inline add-tag) — ownership tag hidden */}
        <div className="hidden sm:flex">
          <SnapshotTags snap={snap} containerName={containerName} source={source} onTagged={onTagged} t={t} />
        </div>

        {/* Confirm checkbox */}
        <label className="flex items-center gap-1.5 text-xs text-carbon-textSub cursor-pointer shrink-0">
          <input
            type="checkbox"
            checked={confirmed}
            onChange={(e) => setConfirmed(e.target.checked)}
            disabled={isPending || restoreState.phase === "success"}
            className="rounded border-carbon-border bg-carbon-surface2 focus:ring-offset-0"
            style={{ accentColor: "var(--accent)" }}
          />
          {t("restore.confirm")}
        </label>

        {/* Files (file-level restore) toggle */}
        <button
          onClick={() => setShowFiles((p) => !p)}
          className={`shrink-0 rounded-lg border border-carbon-border px-2.5 py-1 text-xs transition-colors ${
            showFiles ? "bg-carbon-surface3 text-carbon-text" : "text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
          }`}
        >
          {t("snapshots.files")}
        </button>

        {/* Restore to an alternate folder (non-destructive) toggle */}
        <button
          onClick={() => setShowRestoreTo((p) => !p)}
          title={t("restore.toFolderHint")}
          className={`shrink-0 rounded-lg border border-carbon-border px-2.5 py-1 text-xs transition-colors ${
            showRestoreTo ? "bg-carbon-surface3 text-carbon-text" : "text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
          }`}
        >
          {t("restore.toFolder")}
        </button>

        {/* Restore button */}
        <button
          onClick={() => void handleRestore()}
          disabled={!confirmed || isPending || restoreState.phase === "success"}
          className="inline-flex items-center gap-1.5 rounded-lg bg-accent px-2.5 py-1 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-40 disabled:cursor-not-allowed shrink-0"
        >
          {isPending ? (
            <>
              <span
                className="h-2.5 w-2.5 rounded-full border-2 border-t-transparent animate-spin inline-block"
                style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
              />
              {t("common.restoring")}
            </>
          ) : (
            t("snapshots.restore")
          )}
        </button>

        {/* Delete this backup (restic forget) */}
        <button
          onClick={() => void handleDelete()}
          disabled={deleting || isPending}
          title={t("snapshots.delete")}
          className="shrink-0 rounded-lg border border-carbon-border px-2 py-1 text-xs text-carbon-textSub hover:bg-[#3a1c1c] hover:text-[#ff8389] transition-colors disabled:opacity-50"
        >
          {deleting ? "…" : t("snapshots.delete")}
        </button>
      </div>
      {deleteErr && <p className="text-xs text-[#ff8389] pl-24 break-words">{deleteErr}</p>}

      {/* File-level restore browser */}
      {showFiles && (
        <SnapshotFileBrowser containerName={containerName} snapshotId={snap.id} source={source} t={t} />
      )}

      {/* Restore to an alternate folder (non-destructive) */}
      {showRestoreTo && (
        <RestoreToFolder containerName={containerName} snapshotId={snap.id} source={source} t={t} />
      )}

      {/* Inline result */}
      {restoreState.phase === "success" && (
        <p className="text-xs text-[#6fdc8c] pl-24">
          Restore complete — container is being recreated.
        </p>
      )}
      {restoreState.phase === "error" && (
        <p className="text-xs text-[#ff8389] pl-24 break-words">
          {restoreState.message}
        </p>
      )}
    </div>
  );
}

export function RestorePanel({ name, t, installed = true }: RestorePanelProps) {
  const [open, setOpen] = useState(false);
  const [source, setSource] = useState<RepoSource>("local");
  const [snapshots, setSnapshots] = useState<Snapshot[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function toggle() {
    setOpen((prev) => !prev);
  }

  const [reloadTick, setReloadTick] = useState(0);

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
          <div className="flex flex-col gap-1 py-2 border-b border-carbon-border">
            <div className="flex items-center gap-2">
              <span className="text-xs text-carbon-textMuted">{t("source.label")}</span>
              <SourceToggle source={source} onChange={setSource} disabled={loading} />
            </div>
            <p className="text-[11px] text-carbon-textMuted">{t("source.hint")}</p>
          </div>
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
          {!loading && !error && snapshots.length >= 2 && (
            <CompareSnapshots snapshots={snapshots} containerName={name} source={source} t={t} />
          )}
          {!loading && snapshots.map((snap) => (
            <SnapshotRow
              key={snap.id}
              snap={snap}
              containerName={name}
              source={source}
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
