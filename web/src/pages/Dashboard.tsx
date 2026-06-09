import { useEffect, useState } from "react";
import { listRuns, runSpike, getSpike, listContainers } from "../lib/api";
import type { Run, SpikeCheck, Container } from "../lib/api";
import { useT } from "../lib/i18n";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function formatTs(unix: number | null | undefined): string {
  if (!unix) return "—";
  return new Date(unix * 1000).toLocaleString();
}

function relativeTime(unix: number): string {
  const diff = Math.floor((Date.now() - unix * 1000) / 1000);
  if (diff < 60) return "just now";
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  return `${Math.floor(diff / 86400)}d ago`;
}

// ---------------------------------------------------------------------------
// Status chip
// ---------------------------------------------------------------------------

function StatusChip({
  status,
}: {
  status: "success" | "failed" | "running" | "ok" | "degraded" | "checking" | string;
}) {
  const map: Record<string, string> = {
    success: "bg-[#1c3a2a] text-[#6fdc8c] border border-[#2a5540]",
    ok:      "bg-[#1c3a2a] text-[#6fdc8c] border border-[#2a5540]",
    failed:  "bg-[#3a1c1c] text-[#ff8389] border border-[#5a2a2a]",
    degraded:"bg-[#3a1c1c] text-[#ff8389] border border-[#5a2a2a]",
    running: "bg-[#1c2a3a] text-[#78a9ff] border border-[#2a3a5a]",
    checking:"bg-[#1c2a3a] text-[#78a9ff] border border-[#2a3a5a]",
    info:    "bg-[#2a2a1c] text-[#f1c21b] border border-[#4a4a2a]",
  };
  const cls = map[status.toLowerCase()] ?? "bg-carbon-surface2 text-carbon-textSub border border-carbon-border";
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${cls}`}>
      {status}
    </span>
  );
}

// ---------------------------------------------------------------------------
// Card wrapper
// ---------------------------------------------------------------------------

function Card({
  title,
  children,
  action,
}: {
  title: string;
  children: React.ReactNode;
  action?: React.ReactNode;
}) {
  return (
    <div className="bg-carbon-surface rounded-card border border-carbon-border p-5 flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold text-carbon-textSub uppercase tracking-widest">
          {title}
        </h2>
        {action}
      </div>
      {children}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Spike status card
// ---------------------------------------------------------------------------

function SpikeCard({ t }: { t: ReturnType<typeof useT>["t"] }) {
  const [checks, setChecks] = useState<SpikeCheck[] | null>(null);
  const [allOk, setAllOk] = useState<boolean | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function fetchSpike(fn: typeof runSpike) {
    setLoading(true);
    setError(null);
    try {
      const res = await fn();
      setChecks(res.checks);
      setAllOk(res.allOk);
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Check failed";
      setError(msg);
      setChecks(null);
      setAllOk(false);
    } finally {
      setLoading(false);
    }
  }

  // The check is warmed at container startup; load the cached result on mount so
  // the green list shows immediately. The button re-runs the probes fresh.
  const check = () => fetchSpike(runSpike);
  useEffect(() => { void fetchSpike(getSpike); }, []); // eslint-disable-line react-hooks/exhaustive-deps
  const hasRun = !loading && allOk !== null;
  const overallStatus = allOk ? "ok" : "degraded";
  const overallLabel = allOk ? t("dashboard.allOk") : t("dashboard.degraded");

  // A best-effort (optional) check that fails is informational, not a failure.
  const chipFor = (c: SpikeCheck) => (c.OK ? "ok" : c.BestEffort ? "info" : "failed");

  return (
    <Card
      title={t("spike.title")}
      action={
        <button
          onClick={() => void check()}
          disabled={loading}
          className="inline-flex items-center gap-2 rounded-md bg-carbon-surface3 px-3 py-1.5 text-xs font-medium text-carbon-text hover:bg-carbon-hover transition-colors disabled:opacity-50"
        >
          {loading ? (
            <>
              <span className="h-3 w-3 rounded-full border-2 border-[#78a9ff] border-t-transparent animate-spin" />
              {t("dashboard.checking")}
            </>
          ) : (
            t("dashboard.hostIntegrationCheck")
          )}
        </button>
      }
    >
      {hasRun && (
        <div className="flex items-center gap-2">
          <span className="text-xs text-carbon-textMuted">{t("spike.overall")}</span>
          <StatusChip status={overallStatus} />
          <span className="text-sm text-carbon-text">{overallLabel}</span>
        </div>
      )}

      {error && (
        <p className="text-xs text-[#ff8389]">{error}</p>
      )}

      {checks && checks.length > 0 && (
        <div className="divide-y divide-carbon-border">
          {checks.map((c) => (
            <div key={c.Name} className="flex items-center gap-3 py-2 text-sm">
              <StatusChip status={chipFor(c)} />
              <span className="font-mono text-carbon-text w-32 shrink-0">{c.Name}</span>
              <span className="text-carbon-textMuted truncate flex-1">{c.Detail}</span>
              {c.BestEffort && (
                <span className="text-xs text-carbon-textMuted shrink-0">
                  {t("spike.bestEffort")}
                </span>
              )}
            </div>
          ))}
        </div>
      )}
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Recent Runs card
// ---------------------------------------------------------------------------

function RunsCard({ t }: { t: ReturnType<typeof useT>["t"] }) {
  const [runs, setRuns] = useState<Run[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    listRuns()
      .then((res) => {
        if (res.ok) setRuns(res.runs ?? []);
        else setError("Failed to load runs");
      })
      .catch(() => setError("Failed to load runs"))
      .finally(() => setLoading(false));
  }, []);

  const recent = runs.slice(0, 8);

  return (
    <Card title={t("run.historyTitle")}>
      {loading && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
      )}
      {error && <p className="text-sm text-[#ff8389]">{error}</p>}
      {!loading && !error && recent.length === 0 && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.noRuns")}</p>
      )}
      {recent.length > 0 && (
        <div className="divide-y divide-carbon-border">
          {recent.map((run) => (
            <div key={run.id} className="flex items-center gap-3 py-2.5 text-sm">
              <StatusChip status={run.status} />
              <span className="text-carbon-text font-medium w-16 shrink-0">
                {run.kind === "backup" ? t("run.kindBackup") : t("run.kindRestore")}
              </span>
              <span className="text-carbon-textMuted flex-1 truncate text-xs font-mono">
                {run.targetId.slice(0, 12)}…
              </span>
              <span className="text-carbon-textMuted text-xs shrink-0">
                {relativeTime(run.startedAt)}
              </span>
            </div>
          ))}
        </div>
      )}
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Last Backups card
// ---------------------------------------------------------------------------

function LastBackupsCard({ t }: { t: ReturnType<typeof useT>["t"] }) {
  const [containers, setContainers] = useState<Container[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    listContainers()
      .then((res) => {
        if (res.ok) setContainers(res.containers ?? []);
      })
      .catch(() => {/* non-fatal */})
      .finally(() => setLoading(false));
  }, []);

  const withBackups = containers
    .filter((c) => c.lastBackup != null)
    .sort((a, b) => (b.lastBackup ?? 0) - (a.lastBackup ?? 0))
    .slice(0, 6);

  const noBackups = containers
    .filter((c) => c.lastBackup == null)
    .slice(0, 4);

  return (
    <Card title={t("dashboard.lastBackups")}>
      {loading && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
      )}
      {!loading && containers.length === 0 && (
        <p className="text-sm text-carbon-textMuted">No containers found.</p>
      )}

      {withBackups.length > 0 && (
        <div className="divide-y divide-carbon-border">
          {withBackups.map((c) => (
            <div key={c.name} className="flex items-center gap-3 py-2.5 text-sm">
              <div className="w-2 h-2 rounded-full bg-[#6fdc8c] shrink-0" />
              <span className="text-carbon-text font-medium flex-1 truncate">{c.name}</span>
              <span className="text-carbon-textMuted text-xs shrink-0">
                {formatTs(c.lastBackup)}
              </span>
            </div>
          ))}
        </div>
      )}

      {noBackups.length > 0 && (
        <div className="divide-y divide-carbon-border">
          {noBackups.map((c) => (
            <div key={c.name} className="flex items-center gap-3 py-2.5 text-sm">
              <div className="w-2 h-2 rounded-full bg-carbon-surface3 shrink-0" />
              <span className="text-carbon-textMuted flex-1 truncate">{c.name}</span>
              <span className="text-carbon-textMuted text-xs shrink-0">
                {t("containers.never")}
              </span>
            </div>
          ))}
        </div>
      )}
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Dashboard page
// ---------------------------------------------------------------------------

export function Dashboard() {
  const { t } = useT();

  return (
    <div className="flex flex-col gap-6 max-w-5xl">
      {/* Page heading */}
      <div>
        <h1 className="text-2xl font-semibold text-carbon-text">
          {t("dashboard.title")}
        </h1>
        <p className="mt-1 text-sm text-carbon-textSub">
          BombVault — container backup overview
        </p>
      </div>

      {/* 2-column grid for last backups + run history */}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <LastBackupsCard t={t} />
        <RunsCard t={t} />
      </div>

      {/* Spike status — full width */}
      <SpikeCard t={t} />
    </div>
  );
}
