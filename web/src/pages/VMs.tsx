import { useEffect, useRef, useState, type ReactNode } from "react";
import { listVMs, backupVMNow, restoreVM, listVMSnapshots, setVMInclude, setVMIncludeAll, setVMMethod, deleteSnapshot, deleteBackupsVM, forgetVM, discoverVMs, exportVM } from "../lib/api";
import { SourceToggle, type RepoSource } from "../components/SourceToggle";
import { OffsiteIndicator } from "../components/OffsiteIndicator";
import type { VM, Snapshot } from "../lib/api";
import { useT, stateLabel } from "../lib/i18n";
import { Advanced } from "../lib/advanced";
import { ProgressBar } from "../components/ProgressBar";
import { RestoreAction } from "../components/restore/RestoreAction";
import { useProgress, anyActive, busyPhraseKey } from "../lib/progress";
import { useBackupWatch, fireAndWaitRun } from "../lib/backupWatch";

type T = ReturnType<typeof useT>["t"];

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function formatTs(unix: number | null | undefined): string {
  if (!unix) return "—";
  return new Date(unix * 1000).toLocaleString();
}

// ---------------------------------------------------------------------------
// State chip (mirrors Containers.tsx)
// ---------------------------------------------------------------------------

function StateChip({ state }: { state: string }) {
  const { t } = useT();
  const lower = state.toLowerCase();
  const cls =
    lower === "running"
      ? "bg-[#1c3a2a] text-[#6fdc8c] border border-[#2a5540]"
      : lower === "shut off" || lower === "shutoff" || lower === "stopped"
      ? "bg-[#3a1c1c] text-[#ff8389] border border-[#5a2a2a]"
      : "bg-carbon-surface2 text-carbon-textSub border border-carbon-border";
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${cls}`}>
      {stateLabel(t, state)}
    </span>
  );
}

// ---------------------------------------------------------------------------
// Sort control
// ---------------------------------------------------------------------------

type SortKey = "name" | "status";

const SORT_STORAGE_KEY = "bv-vms-sort";

function loadSortKey(): SortKey {
  const v = localStorage.getItem(SORT_STORAGE_KEY);
  if (v === "name" || v === "status") return v;
  return "name";
}

function sortVMs(vms: VM[], key: SortKey): VM[] {
  const copy = [...vms];
  switch (key) {
    case "name":
      return copy.sort((a, b) =>
        a.name.localeCompare(b.name, undefined, { sensitivity: "base" })
      );
    case "status": {
      const rank = (v: VM) => (v.state.toLowerCase() === "running" ? 0 : 1);
      return copy.sort((a, b) => {
        const r = rank(a) - rank(b);
        if (r !== 0) return r;
        return a.name.localeCompare(b.name, undefined, { sensitivity: "base" });
      });
    }
  }
}

const SORT_KEYS = {
  name: "sort.nameAsc",
  status: "sort.status",
} as const;

function SortControl({
  value,
  onChange,
  t,
}: {
  value: SortKey;
  onChange: (k: SortKey) => void;
  t: T;
}) {
  return (
    <div className="flex items-center gap-2 flex-wrap">
      <span className="text-xs text-carbon-textMuted">{t("sort.label")}</span>
      {(["name", "status"] as SortKey[]).map((k) => (
        <button
          key={k}
          onClick={() => onChange(k)}
          className={`rounded-lg px-3 py-1 text-xs font-medium transition-colors ${
            value === k
              ? "bg-accent text-accentContrast"
              : "bg-carbon-surface2 text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
          }`}
        >
          {t(SORT_KEYS[k])}
        </button>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Schedule / backup chip filters (#41)
// ---------------------------------------------------------------------------
// Generic sibling of the sort chips: same chip look + localStorage pattern, but
// parameterised over its option set so the schedule and backup dimensions each
// instantiate it without duplicating the markup. Mirrors Containers.tsx's
// ChipFilter. VMs have NO installed/not-installed FilterControl — the state-
// based live/orphans split already covers that dimension.

type ScheduleFilterKey = "all" | "scheduled" | "notScheduled";
type BackupFilterKey = "all" | "backedUp" | "neverBackedUp";

const SCHEDULE_FILTER_STORAGE_KEY = "bv-vms-schedule-filter";
const BACKUP_FILTER_STORAGE_KEY = "bv-vms-backup-filter";

function loadScheduleFilterKey(): ScheduleFilterKey {
  const v = localStorage.getItem(SCHEDULE_FILTER_STORAGE_KEY);
  if (v === "all" || v === "scheduled" || v === "notScheduled") return v;
  return "all";
}

function loadBackupFilterKey(): BackupFilterKey {
  const v = localStorage.getItem(BACKUP_FILTER_STORAGE_KEY);
  if (v === "all" || v === "backedUp" || v === "neverBackedUp") return v;
  return "all";
}

function ChipFilter<K extends string>({
  label,
  options,
  value,
  onChange,
}: {
  label: string;
  options: { key: K; label: string }[];
  value: K;
  onChange: (k: K) => void;
}) {
  return (
    <div className="flex items-center gap-2 flex-wrap">
      <span className="text-xs text-carbon-textMuted">{label}</span>
      {options.map((o) => (
        <button
          key={o.key}
          onClick={() => onChange(o.key)}
          className={`rounded-lg px-3 py-1 text-xs font-medium transition-colors ${
            value === o.key
              ? "bg-accent text-accentContrast"
              : "bg-carbon-surface2 text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
          }`}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Filter popover (#2.6)
// ---------------------------------------------------------------------------
// Collapses the top controls (search + schedule/backup chips + sort) behind a
// single "Filters" button so the toolbar stays uncluttered. Mirrors the same
// change on Containers so the two pages stay consistent. Closes on outside
// click / Escape; the controls inside keep all their own state + persistence.

function FilterPopover({ t, children }: { t: T; children: ReactNode }) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    function onDown(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") setOpen(false);
    }
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        onClick={() => setOpen((p) => !p)}
        aria-expanded={open}
        className="inline-flex items-center gap-1.5 rounded-lg border border-carbon-border bg-carbon-surface2 px-3 py-1.5 text-xs font-medium text-carbon-text hover:bg-carbon-hover transition-colors"
      >
        <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
          <path d="M1 2.5h10M2.5 6h7M4.5 9.5h3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </svg>
        {t("filter.button")}
      </button>
      {open && (
        <div className="absolute left-0 z-20 mt-2 w-max min-w-[16rem] max-w-[calc(100vw-2rem)] rounded-lg border border-carbon-border bg-carbon-surface p-4 shadow-lg flex flex-col gap-4">
          {children}
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// VM-aware IncludeToggle variant
// ---------------------------------------------------------------------------

// VMMethodSelect picks the per-VM backup method (graceful shutdown vs live
// snapshot) via PATCH /api/vms/{name}.
function VMMethodSelect({
  name,
  initial,
  t,
}: {
  name: string;
  initial: string;
  t: ReturnType<typeof useT>["t"];
}) {
  const [method, setMethod] = useState(initial || "graceful");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleChange(next: string) {
    setBusy(true);
    setError(null);
    try {
      const res = await setVMMethod(name, next);
      if (res.ok) {
        setMethod(next);
      } else {
        // Surface the failure instead of silently reverting — a swallowed error
        // here means the user thinks they switched to live (no downtime) when the
        // VM will actually be shut down at backup time.
        setError(res.error ?? t("vm.method.saveFailed"));
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t("vm.method.saveFailed"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-col items-end gap-1">
      <select
        value={method}
        disabled={busy}
        onChange={(e) => void handleChange(e.target.value)}
        title={t("vm.method.hint")}
        className="rounded border border-carbon-border bg-carbon-surface2 px-2 py-1 text-xs text-carbon-text disabled:opacity-50"
      >
        <option value="graceful">{t("vm.method.graceful")}</option>
        <option value="live">{t("vm.method.live")}</option>
      </select>
      {error && (
        <span className="text-xs text-[#ff8389] max-w-[12rem] text-right leading-tight">
          {error}
        </span>
      )}
    </div>
  );
}

function VMIncludeToggle({
  name,
  initial,
}: {
  name: string;
  initial: boolean;
}) {
  const [enabled, setEnabled] = useState(initial);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Re-seed when the parent passes a fresh value (e.g. after "Include all in
  // schedule" reloads the list). Rows are keyed by name and do not remount, so
  // without this the toggle would keep showing its stale pre-bulk state.
  useEffect(() => setEnabled(initial), [initial]);

  async function handleChange(next: boolean) {
    setBusy(true);
    setError(null);
    try {
      const res = await setVMInclude(name, next);
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
// VM-aware BackupButton variant
// ---------------------------------------------------------------------------

function VMExportButton({ name, t }: { name: string; t: T }) {
  const [state, setState] = useState<"idle" | "pending" | "done" | "error">("idle");
  const [msg, setMsg] = useState<string | null>(null);
  async function run() {
    setState("pending");
    setMsg(null);
    try {
      const r = await exportVM(name);
      if (r.ok) {
        setState("done");
        setMsg(r.path ?? null);
      } else {
        setState("error");
        setMsg(r.error ?? t("settings.error"));
      }
    } catch (err) {
      setState("error");
      setMsg(err instanceof Error ? err.message : t("settings.error"));
    }
  }
  return (
    <div className="flex flex-col items-start gap-1">
      <button
        onClick={() => void run()}
        disabled={state === "pending"}
        className="inline-flex items-center gap-1.5 rounded-lg border border-carbon-border bg-carbon-surface2 px-3 py-1.5 text-xs font-medium text-carbon-text hover:bg-carbon-hover transition-colors disabled:opacity-50"
      >
        {state === "pending" ? "…" : t("export.button")}
      </button>
      {state === "done" && (
        <span className="text-xs text-[#6fdc8c] break-all">{t("export.exportedTo")} {msg}</span>
      )}
      {state === "error" && <span className="text-xs text-[#ff8389] break-all">{msg}</span>}
    </div>
  );
}

function VMBackupButton({
  name,
  t,
  onBackedUp,
  running,
}: {
  name: string;
  t: T;
  onBackedUp?: () => void;
  /** "Something is running" signal (anyActive): busy-guards this backup while
   *  another op runs, but never for its OWN in-flight backup (isPending). */
  running?: { active: boolean; phase?: string };
}) {
  // Fire-and-watch (see useBackupWatch): the server backs the VM up detached and
  // answers immediately, so we watch the "vm:<name>" progress + recorded run for
  // the outcome instead of awaiting the whole backup.
  const { state, fire, isPending } = useBackupWatch({
    progressKey: `vm:${name}`,
    start: () => backupVMNow(name),
    matchRun: (r) => r.domain === "vm" && r.target === name,
    onDone: onBackedUp,
  });
  const blockedByOther = !!running?.active && !isPending;

  return (
    <div className="flex flex-col gap-1 items-start">
      <button
        onClick={() => void fire()}
        disabled={isPending || blockedByOther}
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
      {/* A backup/restore/replication elsewhere blocks a new VM backup — say why. */}
      {blockedByOther && (
        <span className="text-xs text-carbon-textMuted">{t(busyPhraseKey(running?.phase))}</span>
      )}
      {/* Plain export is an advanced-only extra. */}
      <Advanced><VMExportButton name={name} t={t} /></Advanced>
      {state.phase === "success" && (
        <span className="text-xs text-[#6fdc8c]">
          ✓ {t("common.done")}
          {state.phase === "success" && state.snapshotId && (
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
// VM-aware RestorePanel variant
// ---------------------------------------------------------------------------

function VMSnapshotRow({
  snap,
  vmName,
  source,
  onDeleted,
  t,
}: {
  snap: Snapshot;
  vmName: string;
  source: RepoSource;
  onDeleted: () => void;
  t: T;
}) {
  const progressMap = useProgress();
  // Busy-guard handed to the shared RestoreAction: block a new restore while any
  // OTHER backup/restore/replication runs (this VM's own in-flight restore is
  // covered inside RestoreAction via isPending, never self-blocked).
  const running = anyActive(progressMap);
  const [deleting, setDeleting] = useState(false);
  const [deleteErr, setDeleteErr] = useState<string | null>(null);

  async function handleDelete() {
    if (!window.confirm(t("snapshots.deleteConfirm"))) return;
    setDeleting(true);
    setDeleteErr(null);
    try {
      const res = await deleteSnapshot("vms", snap.id, source);
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
          disabled={deleting || running.active}
          title={t("snapshots.delete")}
          className="shrink-0 rounded-lg border border-carbon-border px-2 py-1 text-xs text-carbon-textSub hover:bg-[#3a1c1c] hover:text-[#ff8389] transition-colors disabled:opacity-50"
        >
          {deleting ? "…" : t("snapshots.delete")}
        </button>
      </div>
      {/* Restore control (confirm + leave-stopped + progress banner), indented
          under the id column (pl-24) to match the row's content alignment. */}
      <div className="pl-24">
        <RestoreAction
          domain="vm"
          name={vmName}
          snapshotId={snap.id}
          source={source}
          otherActive={running}
          successMessage={t("restore.completeVM")}
          t={t}
        />
      </div>
      {deleteErr && <p className="text-xs text-[#ff8389] pl-24 break-words">{deleteErr}</p>}
    </div>
  );
}

function VMRestorePanel({ name, t }: { name: string; t: T }) {
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
    listVMSnapshots(name, source)
      .then((res) => {
        if (res.ok) setSnapshots(res.snapshots ?? []);
        else setError(res.error ?? "Failed to load backups");
      })
      .catch(() => setError("Failed to load backups"))
      .finally(() => setLoading(false));
  }, [open, name, source, reloadTick]);

  function handleDeleteAll() {
    if (!window.confirm(t("snapshots.deleteAllConfirm"))) return;
    setDeletingAll(true);
    setError(null);
    deleteBackupsVM(name, source)
      .then((res) => {
        if (!res.ok) setError(res.error ?? "Failed to delete backups");
      })
      .catch(() => setError("Failed to delete backups"))
      .finally(() => {
        setDeletingAll(false);
        setReloadTick((n) => n + 1);
      });
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
              {snapshots.length > 0 && (
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
          {loading && (
            <p className="py-3 text-xs text-carbon-textMuted">{t("common.loadingBackups")}</p>
          )}
          {error && (
            <p className="py-3 text-xs text-[#ff8389]">{error}</p>
          )}
          {!loading && !error && snapshots.length === 0 && (
            <p className="py-3 text-xs text-carbon-textMuted">{t("snapshots.none")}</p>
          )}
          {!loading &&
            snapshots.map((snap) => (
              <VMSnapshotRow
                key={snap.id}
                snap={snap}
                vmName={name}
                source={source}
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
// VM row
// ---------------------------------------------------------------------------

function VMRow({
  vm,
  t,
  onRefresh,
  selected,
  onToggleSelect,
}: {
  vm: VM;
  t: T;
  onRefresh: () => void;
  selected?: boolean;
  onToggleSelect?: () => void;
}) {
  const installed = vm.state !== "not-installed";
  const progressMap = useProgress();
  const progress = progressMap[`vm:${vm.name}`];
  // "Something is running" across any domain — busy-guards this row's own VM
  // backup (its OWN in-flight backup is handled by isPending inside the button).
  const running = anyActive(progressMap);
  return (
    <div className="relative overflow-hidden bg-carbon-surface rounded-card border border-carbon-border p-4 flex flex-col gap-3">
      {/* Top row */}
      <div className="flex items-start gap-3 flex-wrap">
        {/* Multi-select checkbox (installed VMs only) */}
        {onToggleSelect && (
          <input
            type="checkbox"
            checked={!!selected}
            onChange={onToggleSelect}
            aria-label={`Select ${vm.name}`}
            className="mt-1 h-4 w-4 shrink-0 cursor-pointer"
            style={{ accentColor: "var(--accent)" }}
          />
        )}
        {/* Name + state */}
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="font-semibold text-carbon-text text-sm truncate">
              {vm.name}
            </span>
            {installed ? (
              <StateChip state={vm.state} />
            ) : (
              <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-carbon-surface2 text-carbon-textSub border border-carbon-border">
                {t("containers.notInstalled")}
              </span>
            )}
          </div>
        </div>

        {/* Last backup */}
        <div className="text-right shrink-0">
          <p className="text-xs text-carbon-textMuted">{t("containers.lastBackup")}</p>
          <p className="text-xs text-carbon-textSub">
            {vm.lastBackup ? formatTs(vm.lastBackup) : t("containers.never")}
          </p>
        </div>
      </div>

      {/* Actions row */}
      {installed && (
        <div className="flex items-start justify-between gap-4 flex-wrap">
          <div className="flex items-center gap-4 flex-wrap">
            <label className="flex items-center gap-2 cursor-pointer">
              <VMIncludeToggle name={vm.name} initial={vm.includeInSchedule} />
              <span className="text-xs text-carbon-textSub">
                {t("containers.includeInSchedule")}
              </span>
            </label>
            {/* Backup method (graceful / live) — always visible; it decides VM downtime. */}
            <label className="flex items-center gap-2">
              <span className="text-xs text-carbon-textSub">{t("vm.method")}</span>
              <VMMethodSelect name={vm.name} initial={vm.method} t={t} />
            </label>
          </div>
          <div className="ml-auto flex flex-col items-end">
            <VMBackupButton name={vm.name} t={t} onBackedUp={onRefresh} running={running} />
          </div>
        </div>
      )}

      {/* Not installed: offer to clear the stale entry (also stops the scheduler
          retrying a deleted VM). Deleting actual backups stays in the panel below. */}
      {!installed && (
        <div className="flex justify-end">
          <VMForgetButton name={vm.name} t={t} onForgotten={onRefresh} />
        </div>
      )}

      {/* Backups / Restore disclosure */}
      <VMRestorePanel name={vm.name} t={t} />

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

// VMForgetButton clears a no-longer-installed VM's stale entry (its target row),
// for a deleted VM that has no backups left — answering "how do I remove this".
function VMForgetButton({
  name,
  t,
  onForgotten,
}: {
  name: string;
  t: T;
  onForgotten: () => void;
}) {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleForget() {
    if (!window.confirm(t("vms.removeEntryConfirm"))) return;
    setPending(true);
    setError(null);
    try {
      const res = await forgetVM(name);
      if (res.ok) onForgotten();
      else setError(res.error ?? "Remove failed");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Remove failed");
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="flex flex-col items-end gap-1">
      <button
        onClick={() => void handleForget()}
        disabled={pending}
        className="inline-flex items-center gap-2 rounded-lg bg-[#3a1c1c] px-3 py-1.5 text-xs font-medium text-[#ff8389] hover:bg-[#4a2424] transition-colors disabled:opacity-50"
      >
        {pending ? t("dashboard.checking") : t("vms.removeEntry")}
      </button>
      {error && <p className="text-xs text-[#ff8389]">{error}</p>}
    </div>
  );
}

// ScheduleIncludeAllControl is the one-click header control: "Include all in
// schedule" / "Exclude all" for every known VM, refreshing the list so each
// row's include toggle reflects the new state.
function ScheduleIncludeAllControl({
  t,
  onChanged,
}: {
  t: T;
  onChanged: () => void;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function run(include: boolean) {
    setBusy(true);
    setError(null);
    try {
      const res = await setVMIncludeAll(include);
      if (res.ok) onChanged();
      else setError(res.error ?? t("settings.error"));
    } catch (err) {
      setError(err instanceof Error ? err.message : t("settings.error"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex items-center gap-2 flex-wrap">
      <button
        onClick={() => void run(true)}
        disabled={busy}
        className="inline-flex items-center rounded-lg bg-accent px-3 py-1 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
      >
        {t("schedule.includeAll")}
      </button>
      <button
        onClick={() => void run(false)}
        disabled={busy}
        className="inline-flex items-center rounded-lg bg-carbon-surface2 px-3 py-1 text-xs font-medium text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors disabled:opacity-50"
      >
        {t("schedule.excludeAll")}
      </button>
      {error && <span className="text-xs text-[#ff8389]">{error}</span>}
    </div>
  );
}

// ---------------------------------------------------------------------------
// VMs page
// ---------------------------------------------------------------------------

export function VMs() {
  const { t } = useT();
  // Broader "something is running" signal: any backup/restore/replication in
  // flight disables the bulk start buttons + shows a hint.
  const running = anyActive(useProgress());
  const [vms, setVMs] = useState<VM[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [sortKey, setSortKey] = useState<SortKey>(loadSortKey);
  const [search, setSearch] = useState("");
  const [scheduleFilter, setScheduleFilter] = useState<ScheduleFilterKey>(loadScheduleFilterKey);
  const [backupFilter, setBackupFilter] = useState<BackupFilterKey>(loadBackupFilterKey);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [bulkBusy, setBulkBusy] = useState(false);
  const [bulkMsg, setBulkMsg] = useState<string | null>(null);
  const [discovering, setDiscovering] = useState(false);
  const [discoverMsg, setDiscoverMsg] = useState<string | null>(null);

  async function handleDiscover() {
    setDiscovering(true);
    setDiscoverMsg(null);
    try {
      const res = await discoverVMs();
      if (res.ok) {
        setDiscoverMsg(`+${res.discovered ?? 0}`);
        await loadVMs();
      } else {
        setDiscoverMsg(res.error ?? "Discover failed");
      }
    } catch (err) {
      setDiscoverMsg(err instanceof Error ? err.message : "Discover failed");
    } finally {
      setDiscovering(false);
    }
  }

  function loadVMs() {
    return listVMs()
      .then((res) => {
        if (res.ok) setVMs(res.vms ?? []);
        else setError("Failed to load VMs");
      })
      .catch(() => setError("Failed to load VMs"));
  }

  useEffect(() => {
    void loadVMs().finally(() => setLoading(false));
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  function handleSortChange(k: SortKey) {
    setSortKey(k);
    localStorage.setItem(SORT_STORAGE_KEY, k);
  }

  function handleScheduleFilterChange(k: ScheduleFilterKey) {
    setScheduleFilter(k);
    localStorage.setItem(SCHEDULE_FILTER_STORAGE_KEY, k);
  }

  function handleBackupFilterChange(k: BackupFilterKey) {
    setBackupFilter(k);
    localStorage.setItem(BACKUP_FILTER_STORAGE_KEY, k);
  }

  // Compose search (#40) + schedule/backup chips (#41) into one predicate applied
  // BEFORE sort + the live/orphans split, so they combine. VMs have no image, so
  // the search matches the name only.
  const query = search.trim().toLowerCase();
  const filtered = vms.filter((v) => {
    if (query && !v.name.toLowerCase().includes(query)) return false;
    if (scheduleFilter === "scheduled" && !v.includeInSchedule) return false;
    if (scheduleFilter === "notScheduled" && v.includeInSchedule) return false;
    if (backupFilter === "backedUp" && v.lastBackup == null) return false;
    if (backupFilter === "neverBackedUp" && v.lastBackup != null) return false;
    return true;
  });

  const sorted = sortVMs(filtered, sortKey);
  const live = sorted.filter((v) => v.state !== "not-installed");
  const orphans = sorted.filter((v) => v.state === "not-installed");

  // When the list has VMs but the filters excluded them all, show a no-match hint
  // (distinct from the "no VMs at all" empty state, which keys off vms.length).
  const noMatch = vms.length > 0 && live.length === 0 && orphans.length === 0;

  function toggleSelect(name: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(name)) next.delete(name);
      else next.add(name);
      return next;
    });
  }

  const allLiveSelected = live.length > 0 && live.every((v) => selected.has(v.name));
  function toggleSelectAll() {
    setSelected(allLiveSelected ? new Set() : new Set(live.map((v) => v.name)));
  }

  // Keep the selection in sync with what's actually visible: when a search or
  // filter hides a previously-selected VM, drop it, so the bulk-bar count stays
  // honest and a bulk action — including the DESTRUCTIVE "Restore selected" —
  // can never overwrite a VM the user can no longer see.
  useEffect(() => {
    setSelected((prev) => {
      if (prev.size === 0) return prev;
      const visible = new Set(live.map((v) => v.name));
      let changed = false;
      const next = new Set<string>();
      for (const n of prev) {
        if (visible.has(n)) next.add(n);
        else changed = true;
      }
      return changed ? next : prev;
    });
  }, [search, scheduleFilter, backupFilter, vms]); // eslint-disable-line react-hooks/exhaustive-deps

  async function runBulk(action: (name: string) => Promise<{ ok: boolean }>) {
    setBulkBusy(true);
    setBulkMsg(null);
    let ok = 0;
    let fail = 0;
    for (const name of selected) {
      try {
        const res = await action(name);
        if (res.ok) ok++;
        else fail++;
      } catch {
        fail++;
      }
    }
    setBulkBusy(false);
    setBulkMsg(`${ok} ok, ${fail} failed`);
    setSelected(new Set());
    void loadVMs();
  }

  // Single VM backups AND restores are ASYNC and share the server's
  // single-flight guard, so firing them in a tight loop would make every call
  // after the first hit "already running". Run the bulk serially via
  // fireAndWaitRun: it fires one run (retrying briefly while the previous VM's
  // guard is still releasing), then waits for the NEW recorded run to finish
  // before the next — correlated by run id, never by the client clock.
  function backupSelected() {
    void runBulk((name) =>
      fireAndWaitRun({
        kind: "backup",
        matchRun: (r) => r.domain === "vm" && r.target === name,
        start: () => backupVMNow(name),
      })
    );
  }

  function restoreSelected() {
    if (!window.confirm(t("vms.restoreSelectedConfirm"))) return;
    void runBulk((name) =>
      fireAndWaitRun({
        kind: "restore",
        matchRun: (r) => r.domain === "vm" && r.target === name,
        start: () => restoreVM(name, "latest", true),
      })
    );
  }

  return (
    <div className="flex flex-col gap-6 max-w-5xl">
      {/* Page heading + Discover (disaster-recovery) action */}
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-2xl font-semibold text-carbon-text">
            {t("vms.title")}
          </h1>
          <p className="mt-1 text-sm text-carbon-textSub">
            {t("vms.subtitle")}
          </p>
          <div className="mt-2"><OffsiteIndicator domain="vms" /></div>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          {discoverMsg && (
            <span className="text-xs text-carbon-textSub">{discoverMsg}</span>
          )}
          <button
            onClick={() => void handleDiscover()}
            disabled={discovering}
            title={t("vms.discoverHint")}
            className="inline-flex items-center rounded-lg bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
          >
            {discovering ? t("containers.discovering") : t("containers.discover")}
          </button>
        </div>
      </div>

      {loading && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
      )}
      {error && (
        <p className="text-sm text-[#ff8389]">{error}</p>
      )}
      {!loading && !error && vms.length === 0 && (
        <div className="bg-carbon-surface rounded-card border border-carbon-border p-6 text-center">
          <p className="text-sm text-carbon-textMuted">{t("vms.empty")}</p>
        </div>
      )}

      {/* Controls: Filters popover (search + schedule/backup filters + sort) + select-all. */}
      {!loading && vms.length > 0 && (
        <div className="flex items-center gap-x-6 gap-y-2 flex-wrap">
          <FilterPopover t={t}>
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={t("vms.searchPlaceholder")}
              spellCheck={false}
              autoComplete="off"
              className="w-full rounded-lg bg-carbon-surface2 border border-carbon-border text-carbon-text text-sm px-3 py-1.5 focus:outline-none focus:border-[#78a9ff]"
            />
            <ChipFilter<ScheduleFilterKey>
              label={t("filter.schedule")}
              value={scheduleFilter}
              onChange={handleScheduleFilterChange}
              options={[
                { key: "all", label: t("filter.all") },
                { key: "scheduled", label: t("filter.scheduled") },
                { key: "notScheduled", label: t("filter.notScheduled") },
              ]}
            />
            <ChipFilter<BackupFilterKey>
              label={t("filter.backup")}
              value={backupFilter}
              onChange={handleBackupFilterChange}
              options={[
                { key: "all", label: t("filter.all") },
                { key: "backedUp", label: t("filter.backedUp") },
                { key: "neverBackedUp", label: t("filter.neverBackedUp") },
              ]}
            />
            <SortControl value={sortKey} onChange={handleSortChange} t={t} />
          </FilterPopover>
          {live.length > 0 && (
            <label className="flex items-center gap-2 text-xs text-carbon-textSub cursor-pointer">
              <input
                type="checkbox"
                checked={allLiveSelected}
                onChange={toggleSelectAll}
                className="h-4 w-4 cursor-pointer"
                style={{ accentColor: "var(--accent)" }}
              />
              {t("containers.selectAll")}
            </label>
          )}
          {live.length > 0 && (
            <div className="ml-auto">
              <ScheduleIncludeAllControl t={t} onChanged={() => void loadVMs()} />
            </div>
          )}
        </div>
      )}

      {/* Bulk action bar */}
      {!loading && selected.size > 0 && (
        <div className="flex items-center gap-3 flex-wrap rounded-lg border border-carbon-border bg-carbon-surface2 px-3 py-2">
          <span className="text-xs text-carbon-textSub">
            {selected.size} {t("containers.selectedCount")}
          </span>
          <button
            onClick={backupSelected}
            disabled={bulkBusy || running.active}
            className="inline-flex items-center rounded-lg bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
          >
            {t("vms.backupSelected")}
          </button>
          {/* Bulk restore is advanced-only; bulk backup stays basic. */}
          <Advanced>
            <button
              onClick={restoreSelected}
              disabled={bulkBusy || running.active}
              className="inline-flex items-center rounded-lg bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
            >
              {t("vms.restoreSelected")}
            </button>
          </Advanced>
          <button
            onClick={() => setSelected(new Set())}
            disabled={bulkBusy}
            className="text-xs text-carbon-textMuted hover:text-carbon-text transition-colors disabled:opacity-50"
          >
            {t("containers.clearSelection")}
          </button>
          {bulkBusy && (
            <span className="text-xs text-carbon-textMuted">{t("containers.working")}</span>
          )}
          {!bulkBusy && running.active && (
            <span className="text-xs text-carbon-textMuted">
              {t(busyPhraseKey(running.phase))}
            </span>
          )}
          {!bulkBusy && bulkMsg && (
            <span className="text-xs text-carbon-textSub">{bulkMsg}</span>
          )}
        </div>
      )}

      {/* Live VMs */}
      {!loading && live.length > 0 && (
        <div className="flex flex-col gap-3">
          {live.map((v) => (
            <VMRow
              key={v.name}
              vm={v}
              t={t}
              onRefresh={() => void loadVMs()}
              selected={selected.has(v.name)}
              onToggleSelect={() => toggleSelect(v.name)}
            />
          ))}
        </div>
      )}

      {/* Orphan VMs — no longer defined on the host but still have backups */}
      {!loading && orphans.length > 0 && (
        <div className="flex flex-col gap-3">
          <div>
            <h2 className="text-sm font-semibold text-carbon-textSub uppercase tracking-widest">
              {t("containers.notInstalledTitle")}
            </h2>
            <p className="mt-1 text-xs text-carbon-textMuted">
              {t("vms.notInstalledHint")}
            </p>
          </div>
          {orphans.map((v) => (
            <VMRow key={v.name} vm={v} t={t} onRefresh={() => void loadVMs()} />
          ))}
        </div>
      )}

      {/* No VM matches the active search / schedule / backup filters. */}
      {!loading && !error && noMatch && (
        <p className="text-sm text-carbon-textMuted">{t("filter.noMatch")}</p>
      )}
    </div>
  );
}
