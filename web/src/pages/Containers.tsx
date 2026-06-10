import { useEffect, useState } from "react";
import { listContainers, deleteBackups, backupNow, restore, discover } from "../lib/api";
import type { Container } from "../lib/api";
import { useT } from "../lib/i18n";
import { BackupButton } from "../components/BackupButton";
import { RestorePanel } from "../components/RestorePanel";
import { IncludeToggle } from "../components/IncludeToggle";

type T = ReturnType<typeof useT>["t"];

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function formatTs(unix: number | null | undefined): string {
  if (!unix) return "—";
  return new Date(unix * 1000).toLocaleString();
}

// ---------------------------------------------------------------------------
// State chip
// ---------------------------------------------------------------------------

function StateChip({ state }: { state: string }) {
  const lower = state.toLowerCase();
  const cls =
    lower === "running"
      ? "bg-[#1c3a2a] text-[#6fdc8c] border border-[#2a5540]"
      : lower === "exited" || lower === "stopped"
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

type SortKey = "name" | "status" | "ip";

const SORT_STORAGE_KEY = "bv-containers-sort";

function loadSortKey(): SortKey {
  const v = localStorage.getItem(SORT_STORAGE_KEY);
  if (v === "name" || v === "status" || v === "ip") return v;
  return "name";
}

/** Parse an IP like "192.168.1.5" into a numeric tuple for numeric-aware sort. */
function ipToTuple(ip: string): number[] {
  if (!ip) return [Infinity];
  const parts = ip.split(".").map(Number);
  if (parts.length !== 4 || parts.some(isNaN)) return [Infinity];
  return parts;
}

function compareIPs(a: string, b: string): number {
  if (!a && !b) return 0;
  if (!a) return 1;
  if (!b) return -1;
  const ta = ipToTuple(a);
  const tb = ipToTuple(b);
  for (let i = 0; i < 4; i++) {
    const d = (ta[i] ?? 0) - (tb[i] ?? 0);
    if (d !== 0) return d;
  }
  return 0;
}

function sortContainers(containers: Container[], key: SortKey): Container[] {
  const copy = [...containers];
  switch (key) {
    case "name":
      return copy.sort((a, b) =>
        a.name.localeCompare(b.name, undefined, { sensitivity: "base" })
      );
    case "status": {
      const rank = (c: Container) => (c.state.toLowerCase() === "running" ? 0 : 1);
      return copy.sort((a, b) => {
        const r = rank(a) - rank(b);
        if (r !== 0) return r;
        return a.name.localeCompare(b.name, undefined, { sensitivity: "base" });
      });
    }
    case "ip":
      return copy.sort((a, b) => {
        const cmp = compareIPs(a.ip, b.ip);
        if (cmp !== 0) return cmp;
        return a.name.localeCompare(b.name, undefined, { sensitivity: "base" });
      });
  }
}

const SORT_LABELS: Record<SortKey, string> = {
  name: "Name (A–Z)",
  status: "Status",
  ip: "IP",
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
      {(["name", "status", "ip"] as SortKey[]).map((k) => (
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
// Installed / not-installed filter
// ---------------------------------------------------------------------------

type FilterKey = "all" | "installed" | "notInstalled";

const FILTER_STORAGE_KEY = "bv-containers-filter";

function loadFilterKey(): FilterKey {
  const v = localStorage.getItem(FILTER_STORAGE_KEY);
  if (v === "all" || v === "installed" || v === "notInstalled") return v;
  return "all";
}

function FilterControl({
  value,
  onChange,
  t,
}: {
  value: FilterKey;
  onChange: (k: FilterKey) => void;
  t: T;
}) {
  const labels: Record<FilterKey, string> = {
    all: t("containers.filterAll"),
    installed: t("containers.filterInstalled"),
    notInstalled: t("containers.notInstalled"),
  };
  return (
    <div className="flex items-center gap-2 flex-wrap">
      <span className="text-xs text-carbon-textMuted">{t("containers.filter")}</span>
      {(["all", "installed", "notInstalled"] as FilterKey[]).map((k) => (
        <button
          key={k}
          onClick={() => onChange(k)}
          className={`rounded-lg px-3 py-1 text-xs font-medium transition-colors ${
            value === k
              ? "bg-accent text-accentContrast"
              : "bg-carbon-surface2 text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
          }`}
        >
          {labels[k]}
        </button>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Container row
// ---------------------------------------------------------------------------

// DeleteBackupsButton permanently forgets all backups of a (usually
// no-longer-installed) container and refreshes the list on success.
function DeleteBackupsButton({
  name,
  t,
  onDeleted,
}: {
  name: string;
  t: T;
  onDeleted: () => void;
}) {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleDelete() {
    if (!window.confirm(t("containers.deleteBackupsConfirm"))) return;
    setPending(true);
    setError(null);
    try {
      const res = await deleteBackups(name);
      if (res.ok) onDeleted();
      else setError(res.error ?? "Delete failed");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Delete failed");
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="flex flex-col gap-1">
      <button
        onClick={() => void handleDelete()}
        disabled={pending}
        className="inline-flex items-center gap-2 rounded-lg bg-[#3a1c1c] px-3 py-1.5 text-xs font-medium text-[#ff8389] hover:bg-[#4a2424] transition-colors disabled:opacity-50"
      >
        {pending ? t("dashboard.checking") : t("containers.deleteBackups")}
      </button>
      {error && <p className="text-xs text-[#ff8389]">{error}</p>}
    </div>
  );
}

function ContainerRow({
  container,
  t,
  onDeleted,
  selected,
  onToggleSelect,
}: {
  container: Container;
  t: T;
  onDeleted: () => void;
  selected?: boolean;
  onToggleSelect?: () => void;
}) {
  const installed = container.installed;
  return (
    <div className="bg-carbon-surface rounded-card border border-carbon-border p-4 flex flex-col gap-3">
      {/* Top row */}
      <div className="flex items-start gap-3 flex-wrap">
        {/* Multi-select checkbox (installed containers only) */}
        {onToggleSelect && (
          <input
            type="checkbox"
            checked={!!selected}
            onChange={onToggleSelect}
            aria-label={`Select ${container.name}`}
            className="mt-1 h-4 w-4 shrink-0 cursor-pointer"
            style={{ accentColor: "var(--accent)" }}
          />
        )}
        {/* Name + image */}
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="font-semibold text-carbon-text text-sm truncate">
              {container.name}
            </span>
            {installed ? (
              <StateChip state={container.state} />
            ) : (
              <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-carbon-surface2 text-carbon-textSub border border-carbon-border">
                {t("containers.notInstalled")}
              </span>
            )}
            {container.ip && (
              <span className="text-xs text-carbon-textMuted font-mono">{container.ip}</span>
            )}
          </div>
          {container.image && (
            <p className="text-xs text-carbon-textMuted mt-0.5 truncate">{container.image}</p>
          )}
        </div>

        {/* Last backup */}
        <div className="text-right shrink-0">
          <p className="text-xs text-carbon-textMuted">{t("containers.lastBackup")}</p>
          <p className="text-xs text-carbon-textSub">
            {container.lastBackup ? formatTs(container.lastBackup) : t("containers.never")}
          </p>
        </div>
      </div>

      {/* Actions row — include toggle on the left, backup button on the right. */}
      <div className="flex items-start justify-between gap-4 flex-wrap">
        {installed ? (
          <>
            {/* Include toggle */}
            <label className="flex items-center gap-2 cursor-pointer">
              <IncludeToggle name={container.name} initial={container.includeInSchedule} />
              <span className="text-xs text-carbon-textSub">
                {t("containers.includeInSchedule")}
              </span>
            </label>

            {/* Backup button (right) — refreshes the list so "last backup" updates */}
            <div className="ml-auto flex flex-col items-end">
              <BackupButton name={container.name} t={t} onBackedUp={onDeleted} />
            </div>
          </>
        ) : (
          /* Not installed: can't back up; offer delete-all-backups instead. */
          <DeleteBackupsButton name={container.name} t={t} onDeleted={onDeleted} />
        )}
      </div>

      {/* Backups / Restore disclosure (works even when not installed) */}
      <RestorePanel name={container.name} t={t} />
    </div>
  );
}

// ---------------------------------------------------------------------------
// Containers page
// ---------------------------------------------------------------------------

export function Containers() {
  const { t } = useT();
  const [containers, setContainers] = useState<Container[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [sortKey, setSortKey] = useState<SortKey>(loadSortKey);
  const [filterKey, setFilterKey] = useState<FilterKey>(loadFilterKey);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [bulkBusy, setBulkBusy] = useState(false);
  const [bulkMsg, setBulkMsg] = useState<string | null>(null);
  const [discovering, setDiscovering] = useState(false);
  const [discoverMsg, setDiscoverMsg] = useState<string | null>(null);

  function loadContainers() {
    return listContainers()
      .then((res) => {
        if (res.ok) setContainers(res.containers ?? []);
        else setError("Failed to load containers");
      })
      .catch(() => setError("Failed to load containers"));
  }

  useEffect(() => {
    void loadContainers().finally(() => setLoading(false));
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  function handleSortChange(k: SortKey) {
    setSortKey(k);
    localStorage.setItem(SORT_STORAGE_KEY, k);
  }

  function handleFilterChange(k: FilterKey) {
    setFilterKey(k);
    localStorage.setItem(FILTER_STORAGE_KEY, k);
  }

  const sorted = sortContainers(containers, sortKey);
  const live = sorted.filter((c) => c.installed);
  const orphans = sorted.filter((c) => !c.installed);

  function toggleSelect(name: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(name)) next.delete(name);
      else next.add(name);
      return next;
    });
  }

  const allLiveSelected = live.length > 0 && live.every((c) => selected.has(c.name));
  function toggleSelectAll() {
    setSelected(allLiveSelected ? new Set() : new Set(live.map((c) => c.name)));
  }

  // Run an action over every selected container, then refresh + clear.
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
    void loadContainers();
  }

  function backupSelected() {
    void runBulk((name) => backupNow(name));
  }

  function restoreSelected() {
    if (!window.confirm(t("containers.restoreSelectedConfirm"))) return;
    void runBulk((name) => restore(name, "latest", true));
  }

  async function handleDiscover() {
    setDiscovering(true);
    setDiscoverMsg(null);
    try {
      const res = await discover();
      if (res.ok) {
        setDiscoverMsg(`+${res.discovered ?? 0}`);
        await loadContainers();
      } else {
        setDiscoverMsg(res.error ?? "Discover failed");
      }
    } catch (err) {
      setDiscoverMsg(err instanceof Error ? err.message : "Discover failed");
    } finally {
      setDiscovering(false);
    }
  }

  return (
    <div className="flex flex-col gap-6 max-w-5xl">
      {/* Page heading + Discover (disaster-recovery) action */}
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-2xl font-semibold text-carbon-text">
            {t("containers.title")}
          </h1>
          <p className="mt-1 text-sm text-carbon-textSub">
            Manage container backups, schedules, and restores.
          </p>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          {discoverMsg && (
            <span className="text-xs text-carbon-textSub">{discoverMsg}</span>
          )}
          <button
            onClick={() => void handleDiscover()}
            disabled={discovering}
            title={t("containers.discoverHint")}
            className="inline-flex items-center rounded-lg bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
          >
            {discovering ? t("containers.discovering") : t("containers.discover")}
          </button>
        </div>
      </div>

      {/* Container list */}
      {loading && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
      )}
      {error && (
        <p className="text-sm text-[#ff8389]">{error}</p>
      )}
      {!loading && !error && containers.length === 0 && (
        <div className="bg-carbon-surface rounded-card border border-carbon-border p-6 text-center">
          <p className="text-sm text-carbon-textMuted">
            No containers found. Is Docker running?
          </p>
        </div>
      )}
      {/* Controls: filter (installed / not installed) + sort. */}
      {!loading && containers.length > 0 && (
        <div className="flex items-center gap-x-6 gap-y-2 flex-wrap">
          <FilterControl value={filterKey} onChange={handleFilterChange} t={t} />
          <SortControl value={sortKey} onChange={handleSortChange} />
          {filterKey !== "notInstalled" && live.length > 0 && (
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

      {/* Bulk action bar — appears when one or more containers are selected. */}
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
            {t("containers.backupSelected")}
          </button>
          <button
            onClick={restoreSelected}
            disabled={bulkBusy}
            className="inline-flex items-center rounded-lg bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
          >
            {t("containers.restoreSelected")}
          </button>
          <button
            onClick={() => setSelected(new Set())}
            disabled={bulkBusy}
            className="text-xs text-carbon-textMuted hover:text-carbon-text transition-colors disabled:opacity-50"
          >
            {t("containers.clearSelection")}
          </button>
          {bulkBusy && <span className="text-xs text-carbon-textMuted">{t("containers.working")}</span>}
          {!bulkBusy && bulkMsg && <span className="text-xs text-carbon-textSub">{bulkMsg}</span>}
        </div>
      )}

      {!loading && filterKey !== "notInstalled" && live.length > 0 && (
        <div className="flex flex-col gap-3">
          {live.map((c) => (
            <ContainerRow
              key={c.name}
              container={c}
              t={t}
              onDeleted={() => void loadContainers()}
              selected={selected.has(c.name)}
              onToggleSelect={() => toggleSelect(c.name)}
            />
          ))}
        </div>
      )}

      {/* Not-installed containers that still have backups. */}
      {!loading && filterKey !== "installed" && orphans.length > 0 && (
        <div className="flex flex-col gap-3">
          <div>
            <h2 className="text-sm font-semibold text-carbon-textSub uppercase tracking-widest">
              {t("containers.notInstalledTitle")}
            </h2>
            <p className="mt-1 text-xs text-carbon-textMuted">
              {t("containers.notInstalledHint")}
            </p>
          </div>
          {orphans.map((c) => (
            <ContainerRow key={c.name} container={c} t={t} onDeleted={() => void loadContainers()} />
          ))}
        </div>
      )}
    </div>
  );
}
