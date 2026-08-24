// ---------------------------------------------------------------------------
// Files page (#62) — first-class file-set backups ("point BombVault at any
// folder"). Modeled on VMs.tsx, the closest per-item domain page: one card per
// file set with an include-in-schedule switch, a fire-and-watch backup button
// (progress key "files:<name>"), and an expandable Backups panel whose restore
// control offers "original location" (confirm-gated, in place) vs "to a folder"
// (non-destructive extract via FolderBrowser). Add/edit runs in a dialog with a
// FolderBrowser path picker and an excludes textarea (one pattern per line).
// ---------------------------------------------------------------------------

import { useEffect, useRef, useState, type CSSProperties } from "react";
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
import type { FileSetView, Snapshot, FileEntry, FileSetPresetResponse } from "../lib/api";
import { SourceToggle, type RepoSource } from "../components/SourceToggle";
import { OffsiteIndicator } from "../components/OffsiteIndicator";
import { FolderBrowser } from "../components/FolderBrowser";
import { DEFAULT_RESTORE_FOLDER } from "../components/RestorePanel";
import { SnapshotFileTree } from "../components/SnapshotFileTree";
import { ProgressBar } from "../components/ProgressBar";
import { RecentRunsList } from "../components/RecentRunsList";
import { RestoreProgress } from "../components/restore/RestoreProgress";
import { EmptyStateIcon } from "../components/EmptyStateIcon";
import { IconFiles } from "../components/Sidebar";
import { useT } from "../lib/i18n";
import { Advanced, useAdvanced } from "../lib/advanced";
import { useProgress, anyActive, busyPhraseKey } from "../lib/progress";
import { useBackupWatch } from "../lib/backupWatch";
import { loadErrorMessage } from "../lib/errors";
import { useConfirm } from "../lib/useConfirm";
import { hueVars, rainbowAt } from "../lib/appearance";
import { Selector, type SelectorItem } from "../components/Selector";
import { useRainbow } from "../lib/useRainbow";
import { Badge } from "../components/Badge";
import { InfoBubble } from "../components/InfoBubble";
import { Toggle } from "../components/Toggle";
import { CheckDraw } from "../components/CheckDraw";
import { useToast } from "../lib/toast";

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
  const { t } = useT();
  const { push } = useToast();
  const [enabled, setEnabled] = useState(initial);
  const [busy, setBusy] = useState(false);
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): a
  // failed toggle toasts AND shakes, same mechanism as ToggleRow's shakeNonce.
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
    <div className="flex flex-col items-end gap-1">
      <Toggle
        key={shake}
        hideLabel
        label={t("containers.includeInSchedule")}
        checked={enabled}
        onChange={(next) => void handleChange(next)}
        disabled={busy}
        className={shake ? "glim-shake" : undefined}
      />
    </div>
  );
}

// ---------------------------------------------------------------------------
// Backup button (fire-and-watch, mirrors VMBackupButton)
// ---------------------------------------------------------------------------

// GlimStone follow-up pass (v8.0.0) audit note: the state.phase "success"/
// "error" result below is deliberately NOT migrated to a toast, unlike this
// file's other flash sites (FileSetEnabledToggle/FileSetDialog above). Exact
// same reasoning as Containers.tsx's BackupButton / VMs.tsx's VMBackupButton:
// it's driven by the SHARED lib/backupWatch.ts useBackupWatch hook (kind
// defaults to "backup" here, which already self-clears after 4s —
// SUCCESS_CLEAR_MS, effectively already toast-like), but the identical state
// shape also backs RESTORE outcomes elsewhere, which are explicitly STICKY BY
// DESIGN. Splitting that shared, cross-file state machine's rendering by kind
// is a hook-level architecture change, not the local flash-swap this pass
// does everywhere else — left as its own deliberate follow-up.
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
        className="inline-flex items-center gap-1.5 rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50 disabled:cursor-not-allowed"
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
        <span className="text-xs text-statusFail max-w-[18rem] wrap-break-word">
          {state.message}
        </span>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Selective restore — tick individual files/folders from a snapshot and restore
// just those into a chosen folder (#65). Mirrors the container SnapshotFileBrowser,
// reusing the shared SnapshotFileTree; scoped to the file-set routes and always
// non-destructive (into a folder), so no in-place confirm is needed here.
// ---------------------------------------------------------------------------

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
  // Same #69 fix as FileSetRestoreControl: seed from the global default instead
  // of an empty string that only ever showed the FolderBrowser's placeholder.
  const [folder, setFolder] = useState(restoreFolder);
  const [restoredTarget, setRestoredTarget] = useState("");

  const progressKey = `files:${set.name}`;
  // The SAME ref instance flows to useBackupWatch AND (via RestoreProgress) to the
  // cancel button — see FileSetRestoreControl / RestoreAction. Never split it.
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
        // #129 — show the server's own reason (e.g. a stale repo lock) when it
        // sent one; the generic message is only for a plain network failure.
        if (res.ok) setFiles(res.files ?? []);
        else setError(loadErrorMessage(res, t("files.loadFailed")));
      })
      .catch(() => setError(t("files.loadFailed")))
      .finally(() => setLoading(false));
  }, [set.id, snapshotId, source, t]);

  // A new selection / target clears any prior result banner so it can't linger
  // over a fresh, unrun choice.
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

      {/* Target folder + restore-selected action — shown once something is ticked. */}
      {count > 0 && (
        <div className="border-t border-carbon-border pt-2 flex flex-col gap-2">
          <FolderBrowser
            label={t("restore.targetPath")}
            value={folder}
            hostMountRoot={hostMountRoot}
            onChange={pickFolder}
          />
          <div className="flex items-center gap-2">
            <button
              onClick={handleRestoreSelected}
              disabled={isPending || blockedByOther || !folder.trim()}
              className="shrink-0 inline-flex items-center rounded-control bg-accent px-2.5 py-1 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-40 disabled:cursor-not-allowed"
            >
              {isPending ? t("common.restoring") : t("files.restoreSelected").replace("{n}", String(count))}
            </button>
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

// ---------------------------------------------------------------------------
// Restore control — "original location" (confirm, in place) vs "to a folder" vs
// "select files" (selective, #65)
// ---------------------------------------------------------------------------

type RestoreDest = "original" | "folder" | "select";

function FileSetRestoreControl({
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
  // A path-less discovered set can only restore into a chosen folder — the
  // server refuses an in-place restore when it doesn't know the original path.
  const noPath = set.path === "";
  const [dest, setDest] = useState<RestoreDest>(noPath ? "folder" : "original");
  // "Select files" (the #65 selective restore) is an advanced option; basic
  // mode keeps the whole-set original / to-folder pair. Read directly (rather
  // than staying inside an <Advanced> JSX wrapper) because the destination
  // Selector below needs to decide, in JS, whether "select" belongs in its
  // items array at all — Selector renders a flat items list, not children a
  // wrapper component could conditionally swallow.
  const { advanced } = useAdvanced();
  // Seeded from the operator's global "Default restore folder" setting, exactly
  // like the container restore panel — was hardcoded to "" (#69), which left the
  // FolderBrowser showing only its generic placeholder example text instead of
  // a real usable default.
  const [targetPath, setTargetPath] = useState(restoreFolder);

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
  const { confirm, confirmDialog } = useConfirm();

  // A stale success/error banner would misdescribe a different destination —
  // clear it when the choice changes (no-op while a restore is in flight).
  useEffect(() => reset(), [dest, targetPath, reset]);

  async function handleRestore() {
    if (dest === "original" && !(await confirm(t("files.restoreOriginalConfirm")))) return;
    if (dest === "folder" && targetPath.trim() === "") return;
    void fire();
  }

  // Destination choice, on the shared Selector component (GlimStone
  // form-engine Phase 2, Task 3). "select" only enters the items array in
  // advanced mode — see the `advanced` comment above.
  const destItems: SelectorItem[] = [
    { id: "original", label: t("files.restoreOriginal"), disabled: noPath, title: noPath ? t("files.noPathHint") : undefined },
    { id: "folder", label: t("files.restoreToFolder") },
    ...(advanced ? [{ id: "select", label: t("files.restoreSelectFiles") }] : []),
  ];

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-2 flex-wrap">
        {/* label reuses the existing "Restore" string rather than a new
            aria-only i18n key: this strip's own three item labels already
            describe the actual choice ("Restore to original location" /
            "Restore to a folder" / "Select files"), and this repo's i18n
            convention (see lib/i18n.ts's 26-locale parity test) requires a
            brand-new key to land in every locale in the same pass — not
            worth doing for a screen-reader-only group name that "Restore"
            already names clearly enough in context. */}
        <Selector
          items={destItems}
          label={t("snapshots.restore")}
          select="one"
          active={dest}
          onChange={(id) => setDest(id as RestoreDest)}
          disabled={isPending}
        />
        {/* The whole-set restore button + its own picker/progress; the selective
            mode renders its own controls below (FileSetFileBrowser). */}
        {dest !== "select" && (
          <button
            onClick={() => void handleRestore()}
            disabled={isPending || blockedByOther || (dest === "folder" && targetPath.trim() === "")}
            className="inline-flex items-center gap-1.5 rounded-control bg-accent px-2.5 py-1 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-40 disabled:cursor-not-allowed shrink-0"
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
          t={t}
        />
      )}
      {confirmDialog}
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
  restoreFolder,
  onDeleted,
  t,
}: {
  snap: Snapshot;
  set: FileSetView;
  source: RepoSource;
  hostMountRoot: string;
  restoreFolder: string;
  onDeleted: () => void;
  t: T;
}) {
  const progressMap = useProgress();
  const running = anyActive(progressMap);
  // Delete is guarded only against THIS set's own in-flight backup/restore, not
  // any global activity (mirrors the VM panel's rationale).
  const busy = progressMap[`files:${set.name}`]?.active ?? false;
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
      const res = await deleteSnapshot("files", snap.id, source);
      if (res.ok) onDeleted();
      else {
        push(res.error ?? "Delete failed", "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : "Delete failed", "fail");
      setShake((n) => n + 1);
    } finally {
      setDeleting(false);
    }
  }

  return (
    <div className="flex flex-col gap-1 py-2.5 border-b border-carbon-border last:border-0">
      <div className="flex items-center gap-3 text-sm">
        <span dir="ltr" className="font-mono text-start text-carbon-text text-xs w-20 shrink-0">
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
          key={shake}
          onClick={() => void handleDelete()}
          disabled={deleting || busy}
          title={t("snapshots.delete")}
          className={`shrink-0 rounded-control px-2 py-1 text-xs text-carbon-textSub hover:bg-statusFailBg hover:text-statusFail transition-colors disabled:opacity-50${
            shake ? " glim-shake" : ""
          }`}
        >
          {deleting ? "…" : t("snapshots.delete")}
        </button>
      </div>
      {/* Restore control, indented under the id column to match the row. */}
      <div className="ps-24">
        <FileSetRestoreControl
          set={set}
          snapshotId={snap.id}
          source={source}
          hostMountRoot={hostMountRoot}
          restoreFolder={restoreFolder}
          otherActive={running}
          t={t}
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
}: {
  set: FileSetView;
  hostMountRoot: string;
  restoreFolder: string;
  t: T;
  /** Delete-all forgets the whole set — the parent must reload the list. */
  onSetsChanged: () => void;
}) {
  const [open, setOpen] = useState(false);
  const [source, setSource] = useState<RepoSource>("local");
  const [snapshots, setSnapshots] = useState<Snapshot[]>([]);
  const [loading, setLoading] = useState(false);
  // Section-load error (list failed to load) — NOT migrated to a toast
  // (GlimStone follow-up pass, v8.0.0 audit note): it replaces the whole
  // snapshot-list content area, the same "the section failed to load"
  // structural condition as Files()'s own page-level `error`, not a one-shot
  // button-click confirmation. handleDeleteAll's own one-shot failure below
  // is the bug fix — it used to share this exact state slot (see comment
  // there).
  const [error, setError] = useState<string | null>(null);

  const [reloadTick, setReloadTick] = useState(0);
  const [deletingAll, setDeletingAll] = useState(false);
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): shake
  // the "Delete all" control on a failed delete, alongside the toast below.
  const [shakeDeleteAll, setShakeDeleteAll] = useState(0);

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

  // BUG FIX (GlimStone follow-up pass, v8.0.0): "Delete all" is a one-shot
  // action failure — it used to be routed through the section-load `error`
  // above via setError(), but the failure branch below also bumps
  // reloadTick to refresh the (still-existing) snapshot list, which re-fires
  // the load effect above and clears `error` again almost immediately
  // (setError(null) at the top of that effect) — so a delete-all failure was
  // already near-invisible before this fix, the EXACT same dead-error-
  // display bug 43c6b49 found and fixed in VMs.tsx's
  // VMRestorePanel.handleDeleteAll. A toast survives that reload.
  async function handleDeleteAll() {
    // TODO(#follow-up): richer stake-detail copy ("N snapshots, X GB") belongs
    // here once it ships (deferred — new interpolated i18n keys across all 25
    // non-English locales, out of scope for this window.confirm() → dialog
    // mechanism swap, form-engine Task 7). This is the highest-value site for
    // it: an irreversible bulk delete of every backup this set has. Same
    // flagged follow-up as Containers.tsx's deleteBackupsConfirm and
    // VMs.tsx's deleteAllConfirm.
    if (!(await confirm(t("files.deleteBackupsConfirm")))) return;
    setDeletingAll(true);
    deleteFileSetBackups(set.id)
      .then((res) => {
        if (!res.ok) {
          push(res.error ?? "Failed to delete backups", "fail");
          setShakeDeleteAll((n) => n + 1);
          setReloadTick((n) => n + 1);
          return;
        }
        // The set itself was forgotten along with its snapshots — reload the
        // whole list so the card disappears instead of going stale.
        onSetsChanged();
      })
      .catch(() => {
        push("Failed to delete backups", "fail");
        setShakeDeleteAll((n) => n + 1);
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
          className={`transition-transform ${open ? "rotate-90" : "rtl:rotate-180"}`}
        >
          <path fill="currentColor" d="M4 1.3 8.5 6 4 10.7Z" />
        </svg>
        {t("snapshots.title")}
      </button>

      {open && (
        <div className="mt-2 rounded-card bg-carbon-background px-3 py-1">
          <div className="flex flex-col gap-1 py-2 border-b border-carbon-border">
            <div className="flex items-center gap-2">
              {/* Source (Local / Off-site) toggle is advanced; basic mode uses local. */}
              <Advanced>
                <span className="text-xs text-carbon-textMuted">{t("source.label")}</span>
                <SourceToggle source={source} onChange={setSource} disabled={loading} domain="files" />
              </Advanced>
              {/* Delete-all acts on the LOCAL repo (and forgets the set), so it
                  is only offered while the local source is shown. */}
              {source === "local" && snapshots.length > 0 && (
                // Task 5 (rule 13): was a plain underline-on-hover text
                // button; already correctly fault-red per "the destructive
                // control is always the fault colour" (Destructive actions).
                <Badge
                  key={shakeDeleteAll}
                  as="button"
                  onClick={() => void handleDeleteAll()}
                  disabled={deletingAll || loading}
                  tone="fail"
                  size="small"
                  className={`ms-auto${shakeDeleteAll ? " glim-shake" : ""}`}
                >
                  {deletingAll ? t("snapshots.deletingAll") : t("snapshots.deleteAll")}
                </Badge>
              )}
            </div>
            <p className="text-caption text-carbon-textMuted">{t("source.hint")}</p>
          </div>
          <RecentRunsList name={set.name} domain="files" t={t} />
          {loading && (
            <p className="py-3 text-xs text-carbon-textMuted">{t("common.loadingBackups")}</p>
          )}
          {error && <p className="py-3 text-xs text-statusFail">{error}</p>}
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

// ---------------------------------------------------------------------------
// Add / edit dialog
// ---------------------------------------------------------------------------

function FileSetDialog({
  initial,
  presetSeed,
  hostMountRoot,
  t,
  onClose,
  onSaved,
}: {
  /** null = create a new set; a view = edit that set. */
  initial: FileSetView | null;
  /** Pre-fill values for a NEW set opened via "Add preset: Host system
   *  config" (#134 — the files domain's flash-domain analogue on
   *  generic/TrueNAS). Ignored when `initial` is set (editing an existing
   *  set never seeds from a preset). Still just a starting point — every
   *  field stays fully editable before Save, same as a blank create. */
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
  const [saving, setSaving] = useState(false);
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): shake
  // the Save button alongside the toast on a failed save.
  const [shake, setShake] = useState(0);

  const canSave = name.trim() !== "" && path.trim() !== "" && !saving;

  // GlimStone follow-up pass (v8.0.0): the "error" flash below is now a toast
  // — same shape as Settings.tsx's CloudCredSetsCard.save() (a dialog editor
  // that closes on success via onSaved(), so a toast is the only outcome
  // notice left, success or failure).
  async function handleSave() {
    if (!canSave) return;
    setSaving(true);
    const excludes = excludesText
      .split("\n")
      .map((line) => line.trim())
      .filter((line) => line !== "");
    try {
      const res = initial
        ? await patchFileSet(initial.id, { name: name.trim(), path: path.trim(), excludes, enabled })
        : await createFileSet({ name: name.trim(), path: path.trim(), excludes, enabled });
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

  // Portal to <body> so the fixed overlay can never be trapped by an ancestor's
  // CSS transform (belt-and-braces with the bv-page-in keyframe fix, #62).
  //
  // GlimStone follow-up pass (jdp live review: "wird das Fenster zu weit oben
  // eingeblendet, dort sitzt der Cardtitelbadge nicht richtig"): was
  // `items-start` (top-anchored, only the backdrop's own `p-4` = 16px above
  // the relative shell) — for THIS dialog's actual short, single-screen
  // content that left the heading Badge's own -11px notch poking up to just
  // ~5px below the literal browser-viewport edge (measured live), reading as
  // a flat rectangle jammed into the corner rather than a notch with any
  // breathing room. `items-center` is the same fix ConfirmDialog.tsx/
  // WhatsNewDialog.tsx/ErrorDetailPanel.tsx already use for their own
  // tone="heading" notch — safe here for the identical reason theirs is
  // safe: the visible box below is capped at `max-h-[90vh]`, strictly under
  // the 100vh flex container, so a centred item's top offset is always
  // positive (never negative/off-screen) regardless of content height —
  // short content (like this one) gets comfortable margin on all sides,
  // and content that grows toward the 90vh cap still centres safely with
  // `overflow-y-auto` on the backdrop covering the rest. Same family, same
  // fix still owed to Receiver.tsx's ReceiverDialog and Fleet.tsx's own two
  // `items-start` dialogs (identical copy-pasted shell) — flagged, not
  // touched here (scoped to the Ordner tab this review round covered).
  return createPortal(
    <div
      className="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto bg-black/60 p-4"
      onClick={onClose}
    >
      {/* GlimStone follow-up pass ("half-overlap card notch"): non-scrolling
          `relative` shell wraps the scrollable dialog box — see
          Receiver.tsx's ReceiverDialog for the identical split and why. */}
      <div className="relative w-full max-w-lg">
      <h2 className="flex items-center">
        <Badge tone="heading" size="heading" wrap>{initial ? t("files.editSet") : t("files.addSet")}</Badge>
      </h2>
      <div
        role="dialog"
        aria-modal="true"
        aria-label={initial ? t("files.editSet") : t("files.addSet")}
        onClick={(e) => e.stopPropagation()}
        className="w-full max-h-[90vh] overflow-y-auto rounded-card bg-carbon-surface p-5 flex flex-col gap-4 shadow-2xl"
      >
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
            className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 bv-field-focus"
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
            className="rounded-control bg-carbon-surface2 text-carbon-text text-sm font-mono px-3 py-1.5 bv-field-focus text-start"
          />
          <p className="text-caption text-carbon-textMuted">{t("files.excludesHint")}</p>
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

        <div className="flex items-center justify-end gap-2 pt-1">
          <button
            onClick={onClose}
            disabled={saving}
            className="inline-flex items-center rounded-control bg-carbon-surface2 px-3 py-1.5 text-xs font-medium text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors disabled:opacity-50"
          >
            {t("files.cancel")}
          </button>
          <button
            key={shake}
            onClick={() => void handleSave()}
            disabled={!canSave}
            className={`inline-flex items-center rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50${
              shake ? " glim-shake" : ""
            }`}
          >
            {saving ? t("common.saving") : t("settings.save")}
          </button>
        </div>
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
  restoreFolder,
  t,
  onRefresh,
  onEdit,
  index,
}: {
  set: FileSetView;
  hostMountRoot: string;
  restoreFolder: string;
  t: T;
  onRefresh: () => void;
  onEdit: () => void;
  /** Position in the rendered list — the rainbow palette position (GlimStone
   *  form-engine Phase 2, Task 2). Assigned by LIST INDEX, never a hash of
   *  `set.id`/name — see the caller below. */
  index: number;
}) {
  const progressMap = useProgress();
  const progress = progressMap[`files:${set.name}`];
  const running = anyActive(progressMap);
  const [removing, setRemoving] = useState(false);
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): a
  // failed action toasts AND shakes its button.
  const [shake, setShake] = useState(0);

  const noPath = set.path === "";
  const pathMissing = !noPath && !set.pathExists;

  async function handleRemove() {
    if (!(await confirm(t("files.deleteSetConfirm")))) return;
    setRemoving(true);
    try {
      const res = await deleteFileSet(set.id);
      if (res.ok) onRefresh();
      else {
        push(res.error ?? "Remove failed", "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : "Remove failed", "fail");
      setShake((n) => n + 1);
    } finally {
      setRemoving(false);
    }
  }

  return (
    <div
      style={{ ...hueVars(rainbowAt(index)), "--row-i": String(index) } as CSSProperties}
      // glim-tint washes the card (trap #2 — without it this card shows
      // almost no colour at rest); glim-active while THIS set's own
      // backup/restore is actively running — mirrors ContainerRow/VMRow.
      // bv-stagger-row (GlimStone motion-engine animation 3) — see
      // ContainerRow's identical comment.
      className={`relative overflow-hidden bg-carbon-surface rounded-card p-4 flex flex-col gap-3 glim-hue glim-tint bv-stagger-row ${
        progress?.active ? "glim-active" : ""
      }`}
    >
      {/* Top row: name + chips, path, last backup */}
      <div className="flex items-start gap-3 flex-wrap">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="font-semibold text-carbon-text text-sm truncate">
              {set.name}
            </span>
            {set.excludes.length > 0 && (
              // GlimStone completeness sweep: was a hand-rolled span byte-
              // identical to Badge's own tone="neutral" medium-stage classes
              // (bg-carbon-surface2/text-carbon-textSub, rounded-control) —
              // exactly the drift Badge.tsx exists to prevent. `wrap` matches
              // this straggler's original un-clipped, content-grows sizing
              // (no fixed height, just px-2/py-0.5) more closely than the
              // default fixed-height stage would.
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

        {/* Last backup */}
        <div className="text-end shrink-0">
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
            className="inline-flex items-center rounded-control bg-carbon-surface2 px-3 py-1.5 text-xs font-medium text-carbon-text hover:bg-carbon-hover transition-colors"
          >
            {t("files.editSet")}
          </button>
          <button
            key={shake}
            onClick={() => void handleRemove()}
            disabled={removing}
            className={`inline-flex items-center rounded-control bg-statusFailBg px-3 py-1.5 text-xs font-medium text-statusFail hover:bg-statusFailBgHover transition-colors disabled:opacity-50${
              shake ? " glim-shake" : ""
            }`}
          >
            {removing ? t("dashboard.checking") : t("files.deleteSet")}
          </button>
        </div>
        <div className="ms-auto flex flex-col items-end">
          <FileSetBackupButton set={set} t={t} onBackedUp={onRefresh} running={running} />
        </div>
      </div>

      {/* Backups / Restore disclosure */}
      <FileSetRestorePanel
        set={set}
        hostMountRoot={hostMountRoot}
        restoreFolder={restoreFolder}
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
      {confirmDialog}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Files page
// ---------------------------------------------------------------------------

export function Files() {
  const { t } = useT();
  const { push } = useToast();
  // One subscription for the whole list rather than one per row — see
  // Containers.tsx's identical call for the same reasoning.
  useRainbow();
  // Broader "something is running" signal: any backup/restore/replication in
  // flight disables the bulk start buttons + shows a hint.
  const running = anyActive(useProgress());
  const [sets, setSets] = useState<FileSetView[]>([]);
  const [hostMountRoot, setHostMountRoot] = useState("/host/user");
  const [restoreFolder, setRestoreFolder] = useState(DEFAULT_RESTORE_FOLDER);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  // null = closed; "new" = create dialog; a view = edit dialog for that set.
  const [dialog, setDialog] = useState<"new" | FileSetView | null>(null);
  // Pre-fill values for the create dialog when opened via "Add preset: Host
  // system config" (null for a plain "Add folder set"). Only meaningful while
  // dialog === "new"; cleared alongside it.
  const [presetSeed, setPresetSeed] = useState<{
    name: string;
    path: string;
    excludes: string[];
  } | null>(null);
  // The "Host system config" preset suggestion for the current platform
  // (#134, files domain's flash-domain analogue). null until loaded or on a
  // failed fetch — either way the preset button stays hidden, never a
  // half-working affordance.
  const [preset, setPreset] = useState<FileSetPresetResponse | null>(null);
  const [discovering, setDiscovering] = useState(false);
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): shake
  // the Discover button on a failed discover, alongside its existing toast —
  // same mechanism as Containers.tsx's/VMs.tsx's identical shakeDiscover.
  const [shakeDiscover, setShakeDiscover] = useState(0);
  const [backupAllBusy, setBackupAllBusy] = useState(false);
  // Same shake-on-failure treatment for the "back up all" batch start — mirrors
  // Containers.tsx's backupSelected/shakeBackupSelected.
  const [shakeBackupAll, setShakeBackupAll] = useState(0);

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
    // Gate the loading flag on BOTH fetches (Promise.all, not two independent
    // .finally()s): the file-set restore controls below seed their target-folder
    // state from restoreFolder ONCE at mount (React only reads a useState
    // initialiser the first render), so if they mounted before this settings
    // fetch resolved they'd permanently miss the real default and fall back to
    // the generic placeholder example instead — same class of bug as #69.
    const sets = loadSets();
    const settings = getSettings()
      .then((res) => {
        if (res.hostMountRoot) setHostMountRoot(res.hostMountRoot);
        if (res.settings?.restoreFolder) setRestoreFolder(res.settings.restoreFolder);
      })
      .catch(() => undefined);
    // Independent of the two fetches above: a failed/slow preset lookup must
    // never block the page (loading gate stays on sets+settings only) — the
    // preset button just stays hidden until it resolves.
    void getFileSetPreset()
      .then((res) => {
        if (res.ok) setPreset(res);
      })
      .catch(() => undefined);
    void Promise.all([sets, settings]).finally(() => setLoading(false));
  }, []);

  /** Opens the create dialog pre-filled with the "Host system config" preset
   *  (still fully editable — Save persists through the SAME create-file-set
   *  endpoint as a blank "Add folder set"). No-op until the preset has
   *  loaded and is offered for this platform. */
  function handleAddPreset() {
    if (!preset?.offered) return;
    setPresetSeed({ name: preset.name, path: preset.path, excludes: preset.excludes });
    setDialog("new");
  }

  /** Opens a blank create dialog — used by both "Add folder set" entry
   *  points so a stale preset seed from a previous open can never leak in. */
  function handleAddBlank() {
    setPresetSeed(null);
    setDialog("new");
  }

  // GlimStone follow-up pass (v8.0.0): the "+N" / error note never
  // auto-cleared (it stuck around next to the Discover button until the next
  // click) — now a toast, mirroring Containers.tsx's/VMs.tsx's identical
  // handleDiscover.
  async function handleDiscover() {
    setDiscovering(true);
    try {
      const res = await discoverFiles();
      if (res.ok) {
        push(`+${res.discovered ?? 0}`, "success");
        await loadSets();
      } else {
        push(res.error ?? "Discover failed", "fail");
        setShakeDiscover((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : "Discover failed", "fail");
      setShakeDiscover((n) => n + 1);
    } finally {
      setDiscovering(false);
    }
  }

  // "Back up all now" fires the SERVER-SIDE batch (batch:files) for every
  // enabled set that has a source folder; per-set progress shows on the cards.
  const backupableIds = sets.filter((s) => s.enabled && s.path !== "").map((s) => s.id);

  // jdp live review ("Ordnerset hinzufügen Button rechts oben kann weg, der
  // ist redundant"): the empty-state Card below already carries its own
  // prominent "Add folder set" (+ "Add preset", where offered) CTA, so
  // showing the identical pair a second time in the top-right actions bar
  // was pure duplication — confirmed both call the exact same handlers
  // (handleAddPreset/handleAddBlank). Gate the top-right pair on NOT being in
  // that empty state; once a set exists the empty-state Card stops rendering
  // and the top-right pair is the page's only entry point again, so "Add" is
  // never unreachable. Mirrors the loading/error/empty guard already used
  // for the empty-state block itself below.
  const showEmptyState = !loading && !error && sets.length === 0;

  // GlimStone follow-up pass (v8.0.0): same "+N"/error note migrated off a
  // stuck local span onto a toast — mirrors Containers.tsx's backupSelected
  // (push + shakeBackupSelected).
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
    <div className="flex flex-col gap-6 max-w-5xl">
      {/* Page heading + Discover (disaster-recovery) + Add actions */}
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-2xl font-semibold text-carbon-text">{t("files.title")}</h1>
          <p className="mt-1 text-sm text-carbon-textSub">{t("files.subtitle")}</p>
          <div className="mt-2"><OffsiteIndicator domain="files" /></div>
        </div>
        <div className="flex items-center gap-2 shrink-0 flex-wrap">
          <button
            key={shakeDiscover}
            onClick={() => void handleDiscover()}
            disabled={discovering}
            title={t("files.discoverHint")}
            className={`inline-flex items-center rounded-control bg-carbon-surface2 px-3 py-1.5 text-xs font-medium text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors disabled:opacity-50${
              shakeDiscover ? " glim-shake" : ""
            }`}
          >
            {discovering ? t("containers.discovering") : t("containers.discover")}
          </button>
          {/* Generic/TrueNAS-only one-click starting point (#134): Unraid
              already has the dedicated flash domain for host-level config, so
              preset stays null (never offered) there. Also hidden in the
              empty state — see showEmptyState's own comment above. */}
          {!showEmptyState && preset?.offered && (
            <button
              onClick={handleAddPreset}
              title={t("files.addPresetHint")}
              className="inline-flex items-center rounded-control bg-carbon-surface2 px-3 py-1.5 text-xs font-medium text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors"
            >
              {t("files.addPreset")}
            </button>
          )}
          {!showEmptyState && (
            <button
              onClick={handleAddBlank}
              className="inline-flex items-center rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity"
            >
              {t("files.addSet")}
            </button>
          )}
        </div>
      </div>

      {loading && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
      )}
      {error && <p className="text-sm text-statusFail">{error}</p>}

      {/* Empty state — the "no separate file-backup tool needed" pitch.
          GlimStone follow-up pass (jdp live review: "Die Card im Ordner-Tab
          hat keine Cardtitelbadge mit dem Infotext der in der Card steht"):
          this card had no heading at all — just the icon, the permanent
          pitch paragraph, and the buttons — the one Card-shaped box on this
          page that never got the tone="heading" notch every other Card in
          the app carries. `relative glim-notch-card` (no separate inner
          overflow-hidden box needed — unlike Config.tsx's backupTitle split,
          this card was never `overflow-hidden` to begin with, so a single
          div can host both the badge's positioned ancestor and the visible
          surface, matching Config.tsx's own snapshotsTitle card). The old
          permanent `<p>{t("files.empty")}</p>` reads once and then costs
          vertical space forever (rule 8) — moved verbatim onto the new
          heading Badge as an `onAccent` InfoBubble instead, same content,
          zero new i18n keys for the body (mirrors Flash.tsx's
          backupTitle/restoreNote pass). hueIndex={0}: the only tone="heading"
          notch on this page's own body (the dialog's h2 badge deliberately
          carries no hueIndex, same as every other dialog title in the app),
          and mutually exclusive with FileSetRow's OWN rainbowAt(index) tint
          (this card only renders while the list is empty, i.e. never
          alongside a single FileSetRow), so there is no position to collide
          with.
          insetStart={6} (GlimStone follow-up pass, jdp: "Files/Ordner-Tab:
          Cardtitelbadge falsch platziert" — a SECOND, distinct root-cause
          mechanism from the split-notch one Badge.tsx's own `insetStart` doc
          otherwise documents: this card's `relative` ancestor and its p-6
          padded content ARE the same single div (no structural split), so
          the static-position fallback should already be correct here — but
          this parent is ALSO `text-center flex flex-col items-center`
          (centering the icon/button below), and the `<h2>` above them has NO
          in-flow content of its own once its only child (the notch Badge)
          becomes `position: absolute` — an h2 with nothing left in flow
          collapses to a 0×0 box, which `items-center` then centers
          horizontally in the card rather than stretching to the padding
          edge. The static position then resolves against that zero-width,
          CENTERED h2, landing the badge at the card's horizontal centre
          (measured live: 488px right of the p-6 content edge) instead of
          flush with it — confirmed identical on Fleet.tsx's and
          Receiver.tsx's own empty-state Cards, which share this exact
          `text-center items-center` recipe. `insetStart={6}` sidesteps the
          collapsed-h2 quirk entirely: it's a real `start-6` CSS offset
          resolved against the outer `relative` box directly, so it doesn't
          care what the h2 collapsed to. */}
      {showEmptyState && (
        <div className="relative glim-notch-card bg-carbon-surface rounded-card p-6 text-center flex flex-col items-center gap-3">
          <h2 className="flex items-center">
            <Badge tone="heading" size="heading" wrap hueIndex={0} insetStart={6}>
              {t("files.setsTitle")}
              <InfoBubble tip={t("files.empty")} onAccent />
            </Badge>
          </h2>
          <EmptyStateIcon icon={IconFiles} />
          <div className="flex items-center gap-2 flex-wrap justify-center">
            {preset?.offered && (
              <button
                onClick={handleAddPreset}
                title={t("files.addPresetHint")}
                className="inline-flex items-center rounded-control bg-carbon-surface2 px-3 py-1.5 text-xs font-medium text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors"
              >
                {t("files.addPreset")}
              </button>
            )}
            <button
              onClick={handleAddBlank}
              className="inline-flex items-center rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity"
            >
              {t("files.addSet")}
            </button>
          </div>
        </div>
      )}

      {/* Bulk "back up all" bar */}
      {!loading && sets.length > 0 && (
        <div className="flex items-center gap-3 flex-wrap">
          <button
            key={shakeBackupAll}
            onClick={() => void handleBackupAll()}
            disabled={backupAllBusy || running.active || backupableIds.length === 0}
            className={`inline-flex items-center rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50${
              shakeBackupAll ? " glim-shake" : ""
            }`}
          >
            {t("files.backupAll")}
          </button>
          {!backupAllBusy && running.active && (
            <span className="text-xs text-carbon-textMuted">
              {t(busyPhraseKey(running.phase))}
            </span>
          )}
        </div>
      )}

      {/* File-set cards */}
      {!loading && sets.length > 0 && (
        <div className="flex flex-col gap-3 bv-content-fade">
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
            />
          ))}
        </div>
      )}

      {/* Add / edit dialog */}
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
