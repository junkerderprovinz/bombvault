import { useEffect, useState } from "react";
import { listContainers } from "../lib/api";
import type { Container } from "../lib/api";
import { useT } from "../lib/i18n";
import { BackupButton } from "../components/BackupButton";
import { RestorePanel } from "../components/RestorePanel";
import { IncludeToggle } from "../components/IncludeToggle";

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
              ? "bg-carbon-surface3 text-carbon-text"
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
// Container row
// ---------------------------------------------------------------------------

function ContainerRow({
  container,
  t,
}: {
  container: Container;
  t: ReturnType<typeof useT>["t"];
}) {
  return (
    <div className="bg-carbon-surface rounded-card border border-carbon-border p-4 flex flex-col gap-3">
      {/* Top row */}
      <div className="flex items-start gap-3 flex-wrap">
        {/* Name + image */}
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="font-semibold text-carbon-text text-sm truncate">
              {container.name}
            </span>
            <StateChip state={container.state} />
            {container.ip && (
              <span className="text-xs text-carbon-textMuted font-mono">{container.ip}</span>
            )}
          </div>
          <p className="text-xs text-carbon-textMuted mt-0.5 truncate">{container.image}</p>
        </div>

        {/* Last backup */}
        <div className="text-right shrink-0">
          <p className="text-xs text-carbon-textMuted">{t("containers.lastBackup")}</p>
          <p className="text-xs text-carbon-textSub">
            {container.lastBackup ? formatTs(container.lastBackup) : t("containers.never")}
          </p>
        </div>
      </div>

      {/* Actions row */}
      <div className="flex items-start gap-4 flex-wrap">
        {/* Include toggle */}
        <label className="flex items-center gap-2 cursor-pointer">
          <IncludeToggle name={container.name} initial={container.includeInSchedule} />
          <span className="text-xs text-carbon-textSub">
            {t("containers.includeInSchedule")}
          </span>
        </label>

        {/* Backup button */}
        <BackupButton name={container.name} t={t} />
      </div>

      {/* Snapshots / Restore disclosure */}
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

  useEffect(() => {
    listContainers()
      .then((res) => {
        if (res.ok) setContainers(res.containers ?? []);
        else setError("Failed to load containers");
      })
      .catch(() => setError("Failed to load containers"))
      .finally(() => setLoading(false));
  }, []);

  function handleSortChange(k: SortKey) {
    setSortKey(k);
    localStorage.setItem(SORT_STORAGE_KEY, k);
  }

  const sorted = sortContainers(containers, sortKey);

  return (
    <div className="flex flex-col gap-6 max-w-5xl">
      {/* Page heading */}
      <div>
        <h1 className="text-2xl font-semibold text-carbon-text">
          {t("containers.title")}
        </h1>
        <p className="mt-1 text-sm text-carbon-textSub">
          Manage container backups, schedules, and restores.
        </p>
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
      {!loading && containers.length > 0 && (
        <div className="flex flex-col gap-3">
          <SortControl value={sortKey} onChange={handleSortChange} />
          {sorted.map((c) => (
            <ContainerRow key={c.name} container={c} t={t} />
          ))}
        </div>
      )}
    </div>
  );
}
