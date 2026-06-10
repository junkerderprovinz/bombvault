import { useEffect, useState } from "react";
import { listVMs, backupVMNow, restoreVM, listVMSnapshots, setVMInclude } from "../lib/api";
import type { VM, Snapshot } from "../lib/api";
import { useT } from "../lib/i18n";

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
  const lower = state.toLowerCase();
  const cls =
    lower === "running"
      ? "bg-[#1c3a2a] text-[#6fdc8c] border border-[#2a5540]"
      : lower === "shut off" || lower === "shutoff" || lower === "stopped"
      ? "bg-[#3a1c1c] text-[#ff8389] border border-[#5a2a2a]"
      : "bg-carbon-surface2 text-carbon-textSub border border-carbon-border";
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${cls}`}>
      {state}
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

const SORT_LABELS: Record<SortKey, string> = {
  name: "Name (A–Z)",
  status: "Status",
};

function SortControl({
  value,
  onChange,
}: {
  value: SortKey;
  onChange: (k: SortKey) => void;
}) {
  return (
    <div className="flex items-center gap-2 flex-wrap">
      <span className="text-xs text-carbon-textMuted">Sort:</span>
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
          {SORT_LABELS[k]}
        </button>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// VM-aware IncludeToggle variant
// ---------------------------------------------------------------------------

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

function VMBackupButton({
  name,
  t,
  onBackedUp,
}: {
  name: string;
  t: T;
  onBackedUp?: () => void;
}) {
  type BackupState =
    | { phase: "idle" }
    | { phase: "pending" }
    | { phase: "success"; snapshotId?: string }
    | { phase: "error"; message: string };

  const [state, setState] = useState<BackupState>({ phase: "idle" });

  async function handleBackup() {
    setState({ phase: "pending" });
    try {
      const res = await backupVMNow(name);
      if (res.ok) {
        setState({ phase: "success", snapshotId: res.snapshotId });
        onBackedUp?.();
        setTimeout(() => setState({ phase: "idle" }), 4000);
      } else {
        setState({ phase: "error", message: res.error ?? "Backup failed" });
      }
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Network error";
      setState({ phase: "error", message: msg });
    }
  }

  const isPending = state.phase === "pending";

  return (
    <div className="flex flex-col gap-1 items-start">
      <button
        onClick={() => void handleBackup()}
        disabled={isPending}
        className="inline-flex items-center gap-1.5 rounded-lg bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50 disabled:cursor-not-allowed"
      >
        {isPending ? (
          <>
            <span
              className="h-3 w-3 rounded-full border-2 border-t-transparent animate-spin inline-block"
              style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
            />
            Backing up…
          </>
        ) : (
          t("containers.backupNow")
        )}
      </button>
      {state.phase === "success" && (
        <span className="text-xs text-[#6fdc8c]">
          ✓ Done
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
  t,
}: {
  snap: Snapshot;
  vmName: string;
  t: T;
}) {
  const [confirmed, setConfirmed] = useState(false);
  type RestoreState =
    | { phase: "idle" }
    | { phase: "pending" }
    | { phase: "success" }
    | { phase: "error"; message: string };
  const [restoreState, setRestoreState] = useState<RestoreState>({ phase: "idle" });

  async function handleRestore() {
    if (!confirmed) return;
    setRestoreState({ phase: "pending" });
    try {
      const res = await restoreVM(vmName, snap.id, true);
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
              Restoring…
            </>
          ) : (
            t("snapshots.restore")
          )}
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
    </div>
  );
}

function VMRestorePanel({ name, t }: { name: string; t: T }) {
  const [open, setOpen] = useState(false);
  const [snapshots, setSnapshots] = useState<Snapshot[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setLoading(true);
    setError(null);
    listVMSnapshots(name)
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
          {loading && (
            <p className="py-3 text-xs text-carbon-textMuted">Loading backups…</p>
          )}
          {error && (
            <p className="py-3 text-xs text-[#ff8389]">{error}</p>
          )}
          {!loading && !error && snapshots.length === 0 && (
            <p className="py-3 text-xs text-carbon-textMuted">{t("snapshots.none")}</p>
          )}
          {!loading &&
            snapshots.map((snap) => (
              <VMSnapshotRow key={snap.id} snap={snap} vmName={name} t={t} />
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
  return (
    <div className="bg-carbon-surface rounded-card border border-carbon-border p-4 flex flex-col gap-3">
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
          <label className="flex items-center gap-2 cursor-pointer">
            <VMIncludeToggle name={vm.name} initial={vm.includeInSchedule} />
            <span className="text-xs text-carbon-textSub">
              {t("containers.includeInSchedule")}
            </span>
          </label>
          <div className="ml-auto flex flex-col items-end">
            <VMBackupButton name={vm.name} t={t} onBackedUp={onRefresh} />
          </div>
        </div>
      )}

      {/* Backups / Restore disclosure */}
      <VMRestorePanel name={vm.name} t={t} />
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

  function backupSelected() {
    void runBulk((name) => backupVMNow(name));
  }

  function restoreSelected() {
    if (!window.confirm(t("vms.restoreSelectedConfirm"))) return;
    void runBulk((name) => restoreVM(name, "latest", true));
  }

  return (
    <div className="flex flex-col gap-6 max-w-5xl">
      {/* Page heading */}
      <div>
        <h1 className="text-2xl font-semibold text-carbon-text">
          {t("vms.title")}
        </h1>
        <p className="mt-1 text-sm text-carbon-textSub">
          {t("vms.subtitle")}
        </p>
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

      {/* Sort + select-all controls */}
      {!loading && vms.length > 0 && (
        <div className="flex items-center gap-x-6 gap-y-2 flex-wrap">
          <SortControl value={sortKey} onChange={handleSortChange} />
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
