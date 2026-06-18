import { useEffect, useState } from "react";
import { listSnapshots, restore, listSnapshotFiles, restoreContainerFile } from "../lib/api";
import type { Snapshot, FileEntry } from "../lib/api";
import type { useT } from "../lib/i18n";

type T = ReturnType<typeof useT>["t"];

// Cap the rendered file list — an appdata snapshot can hold thousands of nodes;
// rendering them all would jank the UI. Users narrow with the filter box.
const FILE_DISPLAY_CAP = 500;

// FileRow restores a single file/dir back to its original location (in-place).
function FileRow({
  containerName,
  snapshotId,
  file,
  t,
}: {
  containerName: string;
  snapshotId: string;
  file: FileEntry;
  t: T;
}) {
  const [state, setState] = useState<RestoreState>({ phase: "idle" });

  async function handleRestore() {
    if (!window.confirm(t("files.restoreConfirm"))) return;
    setState({ phase: "pending" });
    try {
      const res = await restoreContainerFile(containerName, snapshotId, file.path, true);
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

// SnapshotFileBrowser lists the files in a snapshot with a filter, for
// file-level restore.
function SnapshotFileBrowser({
  containerName,
  snapshotId,
  t,
}: {
  containerName: string;
  snapshotId: string;
  t: T;
}) {
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState("");

  useEffect(() => {
    setLoading(true);
    listSnapshotFiles(containerName, snapshotId)
      .then((res) => {
        if (res.ok) setFiles(res.files ?? []);
        else setError(t("files.loadFailed"));
      })
      .catch(() => setError(t("files.loadFailed")))
      .finally(() => setLoading(false));
  }, [containerName, snapshotId, t]);

  const q = filter.trim().toLowerCase();
  const matched = q ? files.filter((f) => f.path.toLowerCase().includes(q)) : files;
  const shown = matched.slice(0, FILE_DISPLAY_CAP);

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
      {!loading && !error && matched.length === 0 && (
        <p className="text-xs text-carbon-textMuted">{t("files.none")}</p>
      )}
      {!loading && shown.length > 0 && (
        <div className="max-h-64 overflow-y-auto">
          {shown.map((f) => (
            <FileRow key={f.path} containerName={containerName} snapshotId={snapshotId} file={f} t={t} />
          ))}
        </div>
      )}
      {matched.length > shown.length && (
        <p className="text-xs text-carbon-textMuted">{t("files.more")}</p>
      )}
    </div>
  );
}

interface RestorePanelProps {
  name: string;
  t: T;
}

type RestoreState =
  | { phase: "idle" }
  | { phase: "pending" }
  | { phase: "success" }
  | { phase: "error"; message: string };

function SnapshotRow({
  snap,
  containerName,
  t,
}: {
  snap: Snapshot;
  containerName: string;
  t: T;
}) {
  const [confirmed, setConfirmed] = useState(false);
  const [restoreState, setRestoreState] = useState<RestoreState>({ phase: "idle" });
  const [showFiles, setShowFiles] = useState(false);

  async function handleRestore() {
    if (!confirmed) return;
    setRestoreState({ phase: "pending" });
    try {
      const res = await restore(containerName, snap.id, true);
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
        {/* Tags */}
        {snap.tags && snap.tags.length > 0 && (
          <span className="text-carbon-textMuted text-xs hidden sm:block">
            {snap.tags.join(", ")}
          </span>
        )}

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
      </div>

      {/* File-level restore browser */}
      {showFiles && (
        <SnapshotFileBrowser containerName={containerName} snapshotId={snap.id} t={t} />
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

export function RestorePanel({ name, t }: RestorePanelProps) {
  const [open, setOpen] = useState(false);
  const [snapshots, setSnapshots] = useState<Snapshot[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function toggle() {
    setOpen((prev) => !prev);
  }

  useEffect(() => {
    if (!open) return;
    setLoading(true);
    setError(null);
    listSnapshots(name)
      .then((res) => {
        if (res.ok) setSnapshots(res.snapshots ?? []);
        else setError("Failed to load backups");
      })
      .catch(() => setError("Failed to load backups"))
      .finally(() => setLoading(false));
  }, [open, name]);

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
          {loading && (
            <p className="py-3 text-xs text-carbon-textMuted">{t("common.loadingBackups")}</p>
          )}
          {error && (
            <p className="py-3 text-xs text-[#ff8389]">{error}</p>
          )}
          {!loading && !error && snapshots.length === 0 && (
            <p className="py-3 text-xs text-carbon-textMuted">{t("snapshots.none")}</p>
          )}
          {!loading && snapshots.map((snap) => (
            <SnapshotRow
              key={snap.id}
              snap={snap}
              containerName={name}
              t={t}
            />
          ))}
        </div>
      )}
    </div>
  );
}
