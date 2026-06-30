import { useEffect, useState } from "react";
import { listVMs, backupVMNow, restoreVM, listVMSnapshots, setVMInclude, setVMIncludeAll, setVMMethod, deleteSnapshot, deleteBackupsVM, forgetVM, discoverVMs, exportVM, listRuns } from "../lib/api";
import { SourceToggle, type RepoSource } from "../components/SourceToggle";
import { OffsiteIndicator } from "../components/OffsiteIndicator";
import type { VM, Snapshot } from "../lib/api";
import { useT, stateLabel } from "../lib/i18n";
import { ProgressBar } from "../components/ProgressBar";
import { useProgress } from "../lib/progress";
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
          enabled ? "bg-[#6fdc8c]" : "bg-carbon-surface3"
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
}: {
  name: string;
  t: T;
  onBackedUp?: () => void;
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

  return (
    <div className="flex flex-col gap-1 items-start">
      <button
        onClick={() => void fire()}
        disabled={isPending}
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
      <VMExportButton name={name} t={t} />
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
  const [confirmed, setConfirmed] = useState(false);
  type RestoreState =
    | { phase: "idle" }
    | { phase: "pending" }
    | { phase: "success" }
    | { phase: "error"; message: string };
  const [restoreState, setRestoreState] = useState<RestoreState>({ phase: "idle" });
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

  async function handleRestore() {
    if (!confirmed) return;
    setRestoreState({ phase: "pending" });
    try {
      const res = await restoreVM(vmName, snap.id, true, source);
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
        <button
          onClick={() => void handleDelete()}
          disabled={deleting || isPending}
          title={t("snapshots.delete")}
          className="shrink-0 rounded-lg border border-carbon-border px-2 py-1 text-xs text-carbon-textSub hover:bg-[#3a1c1c] hover:text-[#ff8389] transition-colors disabled:opacity-50"
        >
          {deleting ? "…" : t("snapshots.delete")}
        </button>
      </div>
      {restoreState.phase === "success" && (
        <p className="text-xs text-[#6fdc8c] pl-24">
          Restore complete — VM disks have been replaced.
        </p>
      )}
      {restoreState.phase === "error" && (
        <p className="text-xs text-[#ff8389] pl-24 break-words">
          {restoreState.message}
        </p>
      )}
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
              <span className="text-xs text-carbon-textMuted">{t("source.label")}</span>
              <SourceToggle source={source} onChange={setSource} disabled={loading} />
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
  const progress = useProgress()[`vm:${vm.name}`];
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
            <label className="flex items-center gap-2">
              <span className="text-xs text-carbon-textSub">{t("vm.method")}</span>
              <VMMethodSelect name={vm.name} initial={vm.method} t={t} />
            </label>
          </div>
          <div className="ml-auto flex flex-col items-end">
            <VMBackupButton name={vm.name} t={t} onBackedUp={onRefresh} />
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
        <ProgressBar percent={progress.percent} active={progress.active} />
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
        className="inline-flex items-center rounded-lg bg-carbon-surface2 border border-carbon-border px-3 py-1 text-xs font-medium text-carbon-text hover:bg-carbon-hover transition-colors disabled:opacity-50"
      >
        {t("schedule.includeAll")}
      </button>
      <button
        onClick={() => void run(false)}
        disabled={busy}
        className="inline-flex items-center rounded-lg bg-carbon-surface2 border border-carbon-border px-3 py-1 text-xs font-medium text-carbon-textSub hover:bg-carbon-hover transition-colors disabled:opacity-50"
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
  const [vms, setVMs] = useState<VM[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [sortKey, setSortKey] = useState<SortKey>(loadSortKey);
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

  const sorted = sortVMs(vms, sortKey);
  const live = sorted.filter((v) => v.state !== "not-installed");
  const orphans = sorted.filter((v) => v.state === "not-installed");

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

  // Single VM backups are now ASYNC and share the server's single-backup guard,
  // so firing them in a tight loop would make every call after the first hit
  // "a backup is already running". Run the bulk serially: snapshot this VM's
  // existing run ids, fire (retrying briefly while the previous VM's guard is
  // still releasing — the guard clears just after the run goes terminal, not the
  // instant we observe it), then wait for the NEW run to finish before the next.
  // Correlate by a new run id, never by the client clock (skew matched the wrong
  // or last run).
  async function backupOneAndWait(name: string): Promise<{ ok: boolean }> {
    const isVMRun = (rr: { kind: string; domain?: string; target?: string }) =>
      rr.kind === "backup" && rr.domain === "vm" && rr.target === name;
    let baseline = new Set<string>();
    try {
      const before = await listRuns();
      baseline = new Set((before.runs ?? []).filter(isVMRun).map((rr) => rr.id));
    } catch {
      // ignore — fall back to the first terminal run for this VM.
    }
    // Fire, retrying only while the guard is still busy from the previous VM.
    const fireDeadline = Date.now() + 30 * 1000;
    for (;;) {
      const res = await backupVMNow(name);
      if (res.ok) break;
      const busy = (res.error ?? "").toLowerCase().includes("already running");
      if (!busy || Date.now() > fireDeadline) return { ok: false };
      await new Promise((r) => setTimeout(r, 1000));
    }
    // Poll the recorded run until this VM's NEW backup reaches a terminal state.
    const deadline = Date.now() + 13 * 60 * 60 * 1000; // past the 12h server cap
    for (;;) {
      await new Promise((r) => setTimeout(r, 2000));
      try {
        const runs = await listRuns();
        const run = runs.runs?.find((rr) => isVMRun(rr) && !baseline.has(rr.id));
        if (run && run.status === "success") return { ok: true };
        if (run && run.status === "failed") return { ok: false };
      } catch {
        // transient — keep polling
      }
      if (Date.now() > deadline) return { ok: false };
    }
  }

  function backupSelected() {
    void runBulk((name) => backupOneAndWait(name));
  }

  function restoreSelected() {
    if (!window.confirm(t("vms.restoreSelectedConfirm"))) return;
    void runBulk((name) => restoreVM(name, "latest", true));
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

      {/* One-click include/exclude every VM in the schedule. */}
      {!loading && !error && live.length > 0 && (
        <ScheduleIncludeAllControl t={t} onChanged={() => void loadVMs()} />
      )}

      {/* Sort + select-all controls */}
      {!loading && vms.length > 0 && (
        <div className="flex items-center gap-x-6 gap-y-2 flex-wrap">
          <SortControl value={sortKey} onChange={handleSortChange} t={t} />
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
            disabled={bulkBusy}
            className="inline-flex items-center rounded-lg bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
          >
            {t("vms.backupSelected")}
          </button>
          <button
            onClick={restoreSelected}
            disabled={bulkBusy}
            className="inline-flex items-center rounded-lg bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
          >
            {t("vms.restoreSelected")}
          </button>
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
    </div>
  );
}
