// ---------------------------------------------------------------------------
// Files page (#62) — first-class file-set backups ("point BombVault at any
// folder"). Modeled on VMs.tsx, the closest per-item domain page: one card per
// file set with an include-in-schedule switch, a fire-and-watch backup button
// (progress key "files:<name>"), and an expandable Backups panel whose restore
// control offers "original location" (confirm-gated, in place) vs "to a folder"
// (non-destructive extract via FolderBrowser). Add/edit runs in a dialog with a
// FolderBrowser path picker and an excludes textarea (one pattern per line).
// ---------------------------------------------------------------------------

import { useEffect, useRef, useState } from "react";
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
  discoverFiles,
  deleteSnapshot,
  getSettings,
} from "../lib/api";
import type { FileSetView, Snapshot } from "../lib/api";
import { SourceToggle, type RepoSource } from "../components/SourceToggle";
import { OffsiteIndicator } from "../components/OffsiteIndicator";
import { FolderBrowser } from "../components/FolderBrowser";
import { ProgressBar } from "../components/ProgressBar";
import { RecentRunsList } from "../components/RecentRunsList";
import { RestoreProgress } from "../components/restore/RestoreProgress";
import { useT } from "../lib/i18n";
import { Advanced } from "../lib/advanced";
import { useProgress, anyActive, busyPhraseKey } from "../lib/progress";
import { useBackupWatch } from "../lib/backupWatch";

type T = ReturnType<typeof useT>["t"];

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function formatTs(unix: number | null | undefined): string {
  if (!unix) return "—";
  return new Date(unix * 1000).toLocaleString();
}

// ---------------------------------------------------------------------------
// Include-in-schedule toggle (mirrors VMIncludeToggle, PATCHes {enabled})
// ---------------------------------------------------------------------------

function FileSetEnabledToggle({ id, initial }: { id: string; initial: boolean }) {
  const [enabled, setEnabled] = useState(initial);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Re-seed when the parent passes a fresh value (rows are keyed by id and do
  // not remount, so a list reload must reach the toggle).
  useEffect(() => setEnabled(initial), [initial]);

  async function handleChange(next: boolean) {
    setBusy(true);
    setError(null);
    try {
      const res = await patchFileSet(id, { enabled: next });
      if (res.ok) {
        setEnabled(next);
      } else {
        setError(res.error ?? "Failed to update schedule");
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to update schedule");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-col items-end gap-1">
      <button
        role="switch"
        aria-label="Include in schedule"
        aria-checked={enabled}
        disabled={busy}
        onClick={() => void handleChange(!enabled)}
        title="Include in schedule"
        className={`relative inline-flex h-5 w-9 shrink-0 items-center rounded-full transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#78a9ff] disabled:opacity-50 ${
          enabled ? "bg-accent" : "bg-carbon-surface3"
        }`}
      >
        <span
          className={`inline-block h-3.5 w-3.5 rounded-full bg-carbon-background transition-transform ${
            enabled ? "translate-x-[18px]" : "translate-x-[3px]"
          }`}
        />
      </button>
      {error && (
        <span className="text-xs text-[#ff8389] max-w-[12rem] text-right leading-tight">
          {error}
        </span>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Backup button (fire-and-watch, mirrors VMBackupButton)
// ---------------------------------------------------------------------------

function FileSetBackupButton({
  set,
  t,
  onBackedUp,
  running,
}: {
  set: FileSetView;
  t: T;
  onBackedUp?: () => void;
  /** "Something is running" signal (anyActive): busy-guards this backup while
   *  another op runs, but never for its OWN in-flight backup (isPending). */
  running?: { active: boolean; phase?: string };
}) {
  const { state, fire, isPending } = useBackupWatch({
    progressKey: `files:${set.name}`,
    start: () => backupFileSet(set.id),
    matchRun: (r) => r.domain === "files" && r.target === set.name,
    onDone: onBackedUp,
  });
  const blockedByOther = !!running?.active && !isPending;
  // A path-less discovered set has nothing to back up until a folder is set
  // (the server would refuse anyway) — restore-to-folder still works below.
  const noPath = set.path === "";

  return (
    <div className="flex flex-col gap-1 items-start">
      <button
        onClick={() => void fire()}
        disabled={isPending || blockedByOther || noPath}
        title={noPath ? t("files.noPathHint") : undefined}
        className="inline-flex items-center gap-1.5 rounded-lg bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50 disabled:cursor-not-allowed"
      >
        {isPending ? (
          <>
            <span
              className="h-3 w-3 rounded-full border-2 border-t-transparent animate-spin inline-block"
              style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
            />
            {t("common.backingUp")}
          </>
        ) : (
          t("containers.backupNow")
        )}
      </button>
      {blockedByOther && (
        <span className="text-xs text-carbon-textMuted">{t(busyPhraseKey(running?.phase))}</span>
      )}
      {state.phase === "success" && (
        <span className="text-xs text-[#6fdc8c]">
          ✓ {t("common.done")}
          {state.snapshotId && (
            <span className="font-mono ml-1 text-carbon-textMuted">
              {state.snapshotId.slice(0, 8)}
            </span>
          )}
        </span>
      )}
      {state.phase === "error" && (
        <span className="text-xs text-[#ff8389] max-w-[18rem] break-words">
          {state.message}
        </span>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Restore control — "original location" (confirm, in place) vs "to a folder"
// ---------------------------------------------------------------------------

type RestoreDest = "original" | "folder";

function FileSetRestoreControl({
  set,
  snapshotId,
  source,
  hostMountRoot,
  otherActive,
  t,
}: {
  set: FileSetView;
  snapshotId: string;
  source: RepoSource;
  hostMountRoot: string;
  otherActive: { active: boolean; phase?: string };
  t: T;
}) {
  // A path-less discovered set can only restore into a chosen folder — the
  // server refuses an in-place restore when it doesn't know the original path.
  const noPath = set.path === "";
  const [dest, setDest] = useState<RestoreDest>(noPath ? "folder" : "original");
  const [targetPath, setTargetPath] = useState("");

  const progressKey = `files:${set.name}`;
  // The SAME ref instance flows to useBackupWatch AND (via RestoreProgress) to
  // RestoreCancelButton — see RestoreAction's header note. Never split it.
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

  // A stale success/error banner would misdescribe a different destination —
  // clear it when the choice changes (no-op while a restore is in flight).
  useEffect(() => reset(), [dest, targetPath, reset]);

  function handleRestore() {
    if (dest === "original" && !window.confirm(t("files.restoreOriginalConfirm"))) return;
    if (dest === "folder" && targetPath.trim() === "") return;
    void fire();
  }

  const destChip = (key: RestoreDest, label: string, disabled = false) => (
    <button
      onClick={() => setDest(key)}
      disabled={disabled || isPending}
      title={disabled ? t("files.noPathHint") : undefined}
      className={`rounded-lg px-3 py-1 text-xs font-medium transition-colors disabled:opacity-40 disabled:cursor-not-allowed ${
        dest === key
          ? "bg-accent text-accentContrast"
          : "bg-carbon-surface2 text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
      }`}
    >
      {label}
    </button>
  );

  return (
    <div className="flex flex-col gap-2">
      {/* Destination choice */}
      <div className="flex items-center gap-2 flex-wrap">
        {destChip("original", t("files.restoreOriginal"), noPath)}
        {destChip("folder", t("files.restoreToFolder"))}
        <button
          onClick={handleRestore}
          disabled={isPending || blockedByOther || (dest === "folder" && targetPath.trim() === "")}
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
        {blockedByOther && (
          <span className="text-[11px] text-carbon-textMuted shrink-0">
            {t(busyPhraseKey(otherActive.phase))}
          </span>
        )}
      </div>
      {/* Target folder picker for the non-destructive extract */}
      {dest === "folder" && (
        <FolderBrowser
          label={t("restore.targetPath")}
          value={targetPath}
          hostMountRoot={hostMountRoot}
          onChange={setTargetPath}
        />
      )}
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
    </div>
  );
}

// ---------------------------------------------------------------------------
// Snapshot row + Backups panel (mirror VMSnapshotRow / VMRestorePanel)
// ---------------------------------------------------------------------------

function FileSetSnapshotRow({
  snap,
  set,
  source,
  hostMountRoot,
  onDeleted,
  t,
}: {
  snap: Snapshot;
  set: FileSetView;
  source: RepoSource;
  hostMountRoot: string;
  onDeleted: () => void;
  t: T;
}) {
  const progressMap = useProgress();
  const running = anyActive(progressMap);
  // Delete is guarded only against THIS set's own in-flight backup/restore, not
  // any global activity (mirrors the VM panel's rationale).
  const busy = progressMap[`files:${set.name}`]?.active ?? false;
  const [deleting, setDeleting] = useState(false);
  const [deleteErr, setDeleteErr] = useState<string | null>(null);

  async function handleDelete() {
    if (!window.confirm(t("snapshots.deleteConfirm"))) return;
    setDeleting(true);
    setDeleteErr(null);
    try {
      const res = await deleteSnapshot("files", snap.id, source);
      if (res.ok) onDeleted();
      else setDeleteErr(res.error ?? "Delete failed");
    } catch (err) {
      setDeleteErr(err instanceof Error ? err.message : "Delete failed");
    } finally {
      setDeleting(false);
    }
  }

  return (
    <div className="flex flex-col gap-1 py-2.5 border-b border-carbon-border last:border-0">
      <div className="flex items-center gap-3 text-sm">
        <span className="font-mono text-carbon-text text-xs w-20 shrink-0">
          {snap.id.slice(0, 8)}
        </span>
        <span className="text-carbon-textMuted text-xs flex-1">
          {new Date(snap.time).toLocaleString()}
        </span>
        {snap.tags && snap.tags.length > 0 && (
          <span className="text-carbon-textMuted text-xs hidden sm:block">
            {snap.tags.join(", ")}
          </span>
        )}
        <button
          onClick={() => void handleDelete()}
          disabled={deleting || busy}
          title={t("snapshots.delete")}
          className="shrink-0 rounded-lg border border-carbon-border px-2 py-1 text-xs text-carbon-textSub hover:bg-[#3a1c1c] hover:text-[#ff8389] transition-colors disabled:opacity-50"
        >
          {deleting ? "…" : t("snapshots.delete")}
        </button>
      </div>
      {/* Restore control, indented under the id column to match the row. */}
      <div className="pl-24">
        <FileSetRestoreControl
          set={set}
          snapshotId={snap.id}
          source={source}
          hostMountRoot={hostMountRoot}
          otherActive={running}
          t={t}
        />
      </div>
      {deleteErr && <p className="text-xs text-[#ff8389] pl-24 break-words">{deleteErr}</p>}
    </div>
  );
}

function FileSetRestorePanel({
  set,
  hostMountRoot,
  t,
  onSetsChanged,
}: {
  set: FileSetView;
  hostMountRoot: string;
  t: T;
  /** Delete-all forgets the whole set — the parent must reload the list. */
  onSetsChanged: () => void;
}) {
  const [open, setOpen] = useState(false);
  const [source, setSource] = useState<RepoSource>("local");
  const [snapshots, setSnapshots] = useState<Snapshot[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [reloadTick, setReloadTick] = useState(0);
  const [deletingAll, setDeletingAll] = useState(false);

  useEffect(() => {
    if (!open) return;
    setLoading(true);
    setError(null);
    fileSetSnapshots(set.id, source)
      .then((res) => {
        if (res.ok) setSnapshots(res.snapshots ?? []);
        else setError(res.error ?? "Failed to load backups");
      })
      .catch(() => setError("Failed to load backups"))
      .finally(() => setLoading(false));
  }, [open, set.id, source, reloadTick]);

  function handleDeleteAll() {
    if (!window.confirm(t("files.deleteBackupsConfirm"))) return;
    setDeletingAll(true);
    setError(null);
    deleteFileSetBackups(set.id)
      .then((res) => {
        if (!res.ok) {
          setError(res.error ?? "Failed to delete backups");
          setReloadTick((n) => n + 1);
          return;
        }
        // The set itself was forgotten along with its snapshots — reload the
        // whole list so the card disappears instead of going stale.
        onSetsChanged();
      })
      .catch(() => {
        setError("Failed to delete backups");
        setReloadTick((n) => n + 1);
      })
      .finally(() => setDeletingAll(false));
  }

  return (
    <div className="mt-1">
      <button
        onClick={() => setOpen((prev) => !prev)}
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
              {/* Source (Local / Off-site) toggle is advanced; basic mode uses local. */}
              <Advanced>
                <span className="text-xs text-carbon-textMuted">{t("source.label")}</span>
                <SourceToggle source={source} onChange={setSource} disabled={loading} />
              </Advanced>
              {/* Delete-all acts on the LOCAL repo (and forgets the set), so it
                  is only offered while the local source is shown. */}
              {source === "local" && snapshots.length > 0 && (
                <button
                  onClick={handleDeleteAll}
                  disabled={deletingAll || loading}
                  className="ml-auto text-[11px] text-[#ff8389] hover:underline disabled:opacity-50 disabled:no-underline"
                >
                  {deletingAll ? t("snapshots.deletingAll") : t("snapshots.deleteAll")}
                </button>
              )}
            </div>
            <p className="text-[11px] text-carbon-textMuted">{t("source.hint")}</p>
          </div>
          <RecentRunsList name={set.name} domain="files" t={t} />
          {loading && (
            <p className="py-3 text-xs text-carbon-textMuted">{t("common.loadingBackups")}</p>
          )}
          {error && <p className="py-3 text-xs text-[#ff8389]">{error}</p>}
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
                onDeleted={() => setReloadTick((n) => n + 1)}
                t={t}
              />
            ))}
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Add / edit dialog
// ---------------------------------------------------------------------------

function FileSetDialog({
  initial,
  hostMountRoot,
  t,
  onClose,
  onSaved,
}: {
  /** null = create a new set; a view = edit that set. */
  initial: FileSetView | null;
  hostMountRoot: string;
  t: T;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [name, setName] = useState(initial?.name ?? "");
  const [path, setPath] = useState(initial?.path ?? "");
  const [excludesText, setExcludesText] = useState((initial?.excludes ?? []).join("\n"));
  const [enabled, setEnabled] = useState(initial?.enabled ?? true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const canSave = name.trim() !== "" && path.trim() !== "" && !saving;

  async function handleSave() {
    if (!canSave) return;
    setSaving(true);
    setError(null);
    const excludes = excludesText
      .split("\n")
      .map((line) => line.trim())
      .filter((line) => line !== "");
    try {
      const res = initial
        ? await patchFileSet(initial.id, { name: name.trim(), path: path.trim(), excludes, enabled })
        : await createFileSet({ name: name.trim(), path: path.trim(), excludes, enabled });
      if (res.ok) {
        onSaved();
      } else {
        setError(res.error ?? t("settings.error"));
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t("settings.error"));
    } finally {
      setSaving(false);
    }
  }

  // Portal to <body> so the fixed overlay can never be trapped by an ancestor's
  // CSS transform (belt-and-braces with the bv-page-in keyframe fix, #62).
  return createPortal(
    <div
      className="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-black/60 p-4"
      onClick={onClose}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label={initial ? t("files.editSet") : t("files.addSet")}
        onClick={(e) => e.stopPropagation()}
        className="w-full max-w-lg max-h-[90vh] overflow-y-auto rounded-card border border-carbon-border bg-carbon-surface p-5 flex flex-col gap-4"
      >
        <h2 className="text-lg font-semibold text-carbon-text">
          {initial ? t("files.editSet") : t("files.addSet")}
        </h2>

        {/* Name — feeds the restic tag, so the server validates it strictly. */}
        <div className="flex flex-col gap-1.5">
          <label className="text-xs text-carbon-textSub">{t("files.name")}</label>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            spellCheck={false}
            autoComplete="off"
            placeholder="documents"
            className="rounded-lg bg-carbon-surface2 border border-carbon-border text-carbon-text text-sm px-3 py-1.5 focus:outline-none focus:border-[#78a9ff]"
          />
        </div>

        {/* Source folder (relative subpath under the host mount root) */}
        <div className="flex flex-col gap-1.5">
          <FolderBrowser
            label={t("files.path")}
            value={path}
            hostMountRoot={hostMountRoot}
            onChange={setPath}
          />
          <p className="text-[11px] text-carbon-textMuted">{t("files.pathHint")}</p>
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
            className="rounded-lg bg-carbon-surface2 border border-carbon-border text-carbon-text text-sm font-mono px-3 py-1.5 focus:outline-none focus:border-[#78a9ff]"
          />
          <p className="text-[11px] text-carbon-textMuted">{t("files.excludesHint")}</p>
        </div>

        {/* Include in schedule */}
        <label className="flex items-center gap-2 text-xs text-carbon-textSub cursor-pointer">
          <input
            type="checkbox"
            checked={enabled}
            onChange={(e) => setEnabled(e.target.checked)}
            className="h-4 w-4 cursor-pointer"
            style={{ accentColor: "var(--accent)" }}
          />
          {t("files.enabled")}
        </label>

        {error && <p className="text-xs text-[#ff8389] break-words">{error}</p>}

        <div className="flex items-center justify-end gap-2 pt-1">
          <button
            onClick={onClose}
            disabled={saving}
            className="inline-flex items-center rounded-lg bg-carbon-surface2 px-3 py-1.5 text-xs font-medium text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors disabled:opacity-50"
          >
            {t("files.cancel")}
          </button>
          <button
            onClick={() => void handleSave()}
            disabled={!canSave}
            className="inline-flex items-center rounded-lg bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
          >
            {saving ? t("common.saving") : t("settings.save")}
          </button>
        </div>
      </div>
    </div>,
    document.body,
  );
}

// ---------------------------------------------------------------------------
// File-set row
// ---------------------------------------------------------------------------

function FileSetRow({
  set,
  hostMountRoot,
  t,
  onRefresh,
  onEdit,
}: {
  set: FileSetView;
  hostMountRoot: string;
  t: T;
  onRefresh: () => void;
  onEdit: () => void;
}) {
  const progressMap = useProgress();
  const progress = progressMap[`files:${set.name}`];
  const running = anyActive(progressMap);
  const [removing, setRemoving] = useState(false);
  const [removeErr, setRemoveErr] = useState<string | null>(null);

  const noPath = set.path === "";
  const pathMissing = !noPath && !set.pathExists;

  async function handleRemove() {
    if (!window.confirm(t("files.deleteSetConfirm"))) return;
    setRemoving(true);
    setRemoveErr(null);
    try {
      const res = await deleteFileSet(set.id);
      if (res.ok) onRefresh();
      else setRemoveErr(res.error ?? "Remove failed");
    } catch (err) {
      setRemoveErr(err instanceof Error ? err.message : "Remove failed");
    } finally {
      setRemoving(false);
    }
  }

  return (
    <div className="relative overflow-hidden bg-carbon-surface rounded-card border border-carbon-border p-4 flex flex-col gap-3">
      {/* Top row: name + chips, path, last backup */}
      <div className="flex items-start gap-3 flex-wrap">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="font-semibold text-carbon-text text-sm truncate">
              {set.name}
            </span>
            {set.excludes.length > 0 && (
              <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-carbon-surface2 text-carbon-textSub border border-carbon-border">
                {t("files.excludesCount").replace("{n}", String(set.excludes.length))}
              </span>
            )}
            {/* Source-folder problems, loudest first: no folder at all (discovered
                set), then folder configured but missing on disk. */}
            {noPath && (
              <span
                title={t("files.noPathHint")}
                className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-[#3a2f1c] text-[#f1c21b] border border-[#5a4a2a]"
              >
                {t("files.noPath")}
              </span>
            )}
            {pathMissing && (
              <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-[#3a1c1c] text-[#ff8389] border border-[#5a2a2a]">
                {t("files.pathMissing")}
              </span>
            )}
          </div>
          {!noPath && (
            <p className="mt-1 text-xs font-mono text-carbon-textMuted truncate">
              {hostMountRoot}/{set.path}
            </p>
          )}
          {noPath && (
            <p className="mt-1 text-xs text-carbon-textMuted">{t("files.noPathHint")}</p>
          )}
        </div>

        {/* Last backup */}
        <div className="text-right shrink-0">
          <p className="text-xs text-carbon-textMuted">{t("containers.lastBackup")}</p>
          <p className="text-xs text-carbon-textSub">
            {set.lastBackup ? formatTs(set.lastBackup) : t("containers.never")}
          </p>
        </div>
      </div>

      {/* Actions row */}
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div className="flex items-center gap-4 flex-wrap">
          <label className="flex items-center gap-2 cursor-pointer">
            <FileSetEnabledToggle id={set.id} initial={set.enabled} />
            <span className="text-xs text-carbon-textSub">{t("files.enabled")}</span>
          </label>
          <button
            onClick={onEdit}
            className="inline-flex items-center rounded-lg border border-carbon-border bg-carbon-surface2 px-3 py-1.5 text-xs font-medium text-carbon-text hover:bg-carbon-hover transition-colors"
          >
            {t("files.editSet")}
          </button>
          <button
            onClick={() => void handleRemove()}
            disabled={removing}
            className="inline-flex items-center rounded-lg bg-[#3a1c1c] px-3 py-1.5 text-xs font-medium text-[#ff8389] hover:bg-[#4a2424] transition-colors disabled:opacity-50"
          >
            {removing ? t("dashboard.checking") : t("files.deleteSet")}
          </button>
          {removeErr && <span className="text-xs text-[#ff8389]">{removeErr}</span>}
        </div>
        <div className="ml-auto flex flex-col items-end">
          <FileSetBackupButton set={set} t={t} onBackedUp={onRefresh} running={running} />
        </div>
      </div>

      {/* Backups / Restore disclosure */}
      <FileSetRestorePanel
        set={set}
        hostMountRoot={hostMountRoot}
        t={t}
        onSetsChanged={onRefresh}
      />

      {/* Live backup/restore progress, pinned to the card's bottom edge */}
      {progress && (
        <ProgressBar
          percent={progress.percent}
          active={progress.active}
          label={progress.phase === "restore" ? t("common.restoring") : t("common.backingUp")}
        />
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Files page
// ---------------------------------------------------------------------------

export function Files() {
  const { t } = useT();
  // Broader "something is running" signal: any backup/restore/replication in
  // flight disables the bulk start buttons + shows a hint.
  const running = anyActive(useProgress());
  const [sets, setSets] = useState<FileSetView[]>([]);
  const [hostMountRoot, setHostMountRoot] = useState("/host/user");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  // null = closed; "new" = create dialog; a view = edit dialog for that set.
  const [dialog, setDialog] = useState<"new" | FileSetView | null>(null);
  const [discovering, setDiscovering] = useState(false);
  const [discoverMsg, setDiscoverMsg] = useState<string | null>(null);
  const [backupAllBusy, setBackupAllBusy] = useState(false);
  const [backupAllMsg, setBackupAllMsg] = useState<string | null>(null);

  function loadSets() {
    return listFileSets()
      .then((res) => {
        if (res.ok) {
          setSets(res.fileSets ?? []);
          // Clear any stale banner from a previous failed load — a later success
          // must not leave "Failed to load file sets" up while the UI works.
          setError(null);
        } else setError(res.error ?? "Failed to load file sets");
      })
      .catch(() => setError("Failed to load file sets"));
  }

  useEffect(() => {
    void loadSets().finally(() => setLoading(false));
    getSettings()
      .then((res) => {
        if (res.hostMountRoot) setHostMountRoot(res.hostMountRoot);
      })
      .catch(() => undefined);
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  async function handleDiscover() {
    setDiscovering(true);
    setDiscoverMsg(null);
    try {
      const res = await discoverFiles();
      if (res.ok) {
        setDiscoverMsg(`+${res.discovered ?? 0}`);
        await loadSets();
      } else {
        setDiscoverMsg(res.error ?? "Discover failed");
      }
    } catch (err) {
      setDiscoverMsg(err instanceof Error ? err.message : "Discover failed");
    } finally {
      setDiscovering(false);
    }
  }

  // "Back up all now" fires the SERVER-SIDE batch (batch:files) for every
  // enabled set that has a source folder; per-set progress shows on the cards.
  const backupableIds = sets.filter((s) => s.enabled && s.path !== "").map((s) => s.id);

  async function handleBackupAll() {
    setBackupAllBusy(true);
    setBackupAllMsg(null);
    try {
      const res = await backupFilesAll(backupableIds);
      setBackupAllMsg(res.ok ? t("containers.batchStarted") : res.error ?? t("settings.error"));
    } catch (err) {
      setBackupAllMsg(err instanceof Error ? err.message : t("settings.error"));
    } finally {
      setBackupAllBusy(false);
    }
  }

  return (
    <div className="flex flex-col gap-6 max-w-5xl">
      {/* Page heading + Discover (disaster-recovery) + Add actions */}
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-2xl font-semibold text-carbon-text">{t("files.title")}</h1>
          <p className="mt-1 text-sm text-carbon-textSub">{t("files.subtitle")}</p>
          <div className="mt-2"><OffsiteIndicator domain="files" /></div>
        </div>
        <div className="flex items-center gap-2 shrink-0 flex-wrap">
          {discoverMsg && (
            <span className="text-xs text-carbon-textSub">{discoverMsg}</span>
          )}
          <button
            onClick={() => void handleDiscover()}
            disabled={discovering}
            title={t("files.discoverHint")}
            className="inline-flex items-center rounded-lg bg-carbon-surface2 border border-carbon-border px-3 py-1.5 text-xs font-medium text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors disabled:opacity-50"
          >
            {discovering ? t("containers.discovering") : t("containers.discover")}
          </button>
          <button
            onClick={() => setDialog("new")}
            className="inline-flex items-center rounded-lg bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity"
          >
            {t("files.addSet")}
          </button>
        </div>
      </div>

      {loading && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
      )}
      {error && <p className="text-sm text-[#ff8389]">{error}</p>}

      {/* Empty state — the "no separate file-backup tool needed" pitch */}
      {!loading && !error && sets.length === 0 && (
        <div className="bg-carbon-surface rounded-card border border-carbon-border p-6 text-center flex flex-col items-center gap-3">
          <p className="text-sm text-carbon-textMuted max-w-xl">{t("files.empty")}</p>
          <button
            onClick={() => setDialog("new")}
            className="inline-flex items-center rounded-lg bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity"
          >
            {t("files.addSet")}
          </button>
        </div>
      )}

      {/* Bulk "back up all" bar */}
      {!loading && sets.length > 0 && (
        <div className="flex items-center gap-3 flex-wrap">
          <button
            onClick={() => void handleBackupAll()}
            disabled={backupAllBusy || running.active || backupableIds.length === 0}
            className="inline-flex items-center rounded-lg bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
          >
            {t("files.backupAll")}
          </button>
          {!backupAllBusy && running.active && (
            <span className="text-xs text-carbon-textMuted">
              {t(busyPhraseKey(running.phase))}
            </span>
          )}
          {backupAllMsg && (
            <span className="text-xs text-carbon-textSub">{backupAllMsg}</span>
          )}
        </div>
      )}

      {/* File-set cards */}
      {!loading && sets.length > 0 && (
        <div className="flex flex-col gap-3">
          {sets.map((s) => (
            <FileSetRow
              key={s.id}
              set={s}
              hostMountRoot={hostMountRoot}
              t={t}
              onRefresh={() => void loadSets()}
              onEdit={() => setDialog(s)}
            />
          ))}
        </div>
      )}

      {/* Add / edit dialog */}
      {dialog !== null && (
        <FileSetDialog
          initial={dialog === "new" ? null : dialog}
          hostMountRoot={hostMountRoot}
          t={t}
          onClose={() => setDialog(null)}
          onSaved={() => {
            setDialog(null);
            void loadSets();
          }}
        />
      )}
    </div>
  );
}
