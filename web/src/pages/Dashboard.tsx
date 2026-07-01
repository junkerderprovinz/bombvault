import { useEffect, useState } from "react";
import { listRuns, getSpike, listContainers, listVMs, getSettings, getStatus, getHistory, getStats, recoveryKitUrl, ackRecoveryKit } from "../lib/api";
import type { Run, SpikeCheck, Container, Settings, DomainStatus, HistoryDay, DayStat, RepoStat } from "../lib/api";
import { useT } from "../lib/i18n";
import { useAdvanced } from "../lib/advanced";
import { OffsiteIndicator } from "../components/OffsiteIndicator";
import { formatCadence } from "../components/CadenceBuilder";
import { relativeTime } from "../lib/reltime";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function formatTs(unix: number | null | undefined): string {
  if (!unix) return "—";
  return new Date(unix * 1000).toLocaleString();
}

// humanBytes formats a byte count with a binary (1024) unit and one decimal.
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

// ---------------------------------------------------------------------------
// Stat cards row
// ---------------------------------------------------------------------------

interface StatData {
  containers: number;
  vms: number;
  activeJobs: number;
  pausedJobs: number;
  errors: number;
  missingContainers: number;
  missingVMs: number;
}

function StatCard({
  label,
  value,
  danger,
}: {
  label: string;
  value: number;
  danger?: boolean;
}) {
  return (
    <div className="bg-carbon-surface rounded-card border border-carbon-border px-4 py-3 flex flex-col gap-1">
      <span
        className={`text-2xl font-bold tabular-nums ${
          danger && value > 0 ? "text-[#ff8389]" : "text-carbon-text"
        }`}
      >
        {value}
      </span>
      <span className="text-xs text-carbon-textMuted">{label}</span>
    </div>
  );
}

function StatCardsRow({ t }: { t: ReturnType<typeof useT>["t"] }) {
  const [data, setData] = useState<StatData | null>(null);

  useEffect(() => {
    let active = true;
    Promise.all([listContainers(), getSettings(), listRuns(), listVMs()])
      .then(([contRes, settingsRes, runsRes, vmsRes]) => {
        if (!active) return;
        const containers = contRes.ok ? (contRes.containers ?? []) : [];
        const settings: Settings | null = settingsRes.ok ? settingsRes.settings : null;
        const runs = runsRes.ok ? (runsRes.runs ?? []) : [];
        // listVMs fails/returns empty when the VMs domain is off — treat as none.
        const vms = vmsRes.ok ? (vmsRes.vms ?? []) : [];

        const installed = containers.filter((c) => c.installed);
        const notInstalled = containers.filter((c) => !c.installed);
        const vmsInstalled = vms.filter((v) => v.state !== "not-installed");
        const vmsMissing = vms.filter((v) => v.state === "not-installed");
        const schedEnabled = settings ? settings.containersSchedule !== "off" && settings.containersSchedule !== "" : false;
        const activeJobs = schedEnabled
          ? installed.filter((c) => c.includeInSchedule).length
          : 0;
        const pausedJobs = !schedEnabled
          ? installed.filter((c) => c.includeInSchedule).length
          : 0;
        const errors = runs.filter((r) => r.status === "failed").length;

        setData({
          containers: installed.length,
          vms: vmsInstalled.length,
          activeJobs,
          pausedJobs,
          errors,
          missingContainers: notInstalled.length,
          missingVMs: vmsMissing.length,
        });
      })
      .catch(() => {
        // Non-fatal: stat cards stay null (not rendered)
      });
    return () => {
      active = false;
    };
  }, []);

  if (!data) return null;

  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-7">
      <StatCard label={t("dashboard.statContainers")} value={data.containers} />
      <StatCard label={t("dashboard.statVMs")} value={data.vms} />
      <StatCard label={t("dashboard.statActiveJobs")} value={data.activeJobs} />
      <StatCard label={t("dashboard.statPausedJobs")} value={data.pausedJobs} />
      <StatCard label={t("dashboard.statErrors")} value={data.errors} danger />
      <StatCard label={t("dashboard.statMissingContainers")} value={data.missingContainers} danger />
      <StatCard label={t("dashboard.statMissingVMs")} value={data.missingVMs} />
    </div>
  );
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

  // Display-only on the dashboard: load the cached result (warmed at container
  // startup). Running the check lives in Settings, not here.
  useEffect(() => {
    let active = true;
    setLoading(true);
    getSpike()
      .then((res) => {
        if (!active) return;
        setChecks(res.checks);
        setAllOk(res.allOk);
      })
      .catch((err) => {
        if (!active) return;
        setError(err instanceof Error ? err.message : "Check failed");
        setAllOk(false);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, []);

  const hasRun = !loading && allOk !== null;
  const overallStatus = allOk ? "ok" : "degraded";
  const overallLabel = allOk ? t("dashboard.allOk") : t("dashboard.degraded");

  // A best-effort (optional) check that fails is informational, not a failure.
  const chipFor = (c: SpikeCheck) => (c.OK ? "ok" : c.BestEffort ? "info" : "failed");

  return (
    <Card title={t("spike.title")}>
      {loading && (
        <p className="text-xs text-carbon-textMuted">{t("dashboard.checking")}</p>
      )}

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
// Protection (RPO) status card
// ---------------------------------------------------------------------------

// chipForRpo maps an RPO status to a StatusChip color variant.
function chipForRpo(status: string): string {
  switch (status) {
    case "ok":
      return "success";
    case "warn":
      return "info";
    case "overdue":
    case "never":
      return "failed";
    default:
      return "neutral";
  }
}

function ProtectionCard({ t }: { t: ReturnType<typeof useT>["t"] }) {
  const { lang } = useT();
  const [domains, setDomains] = useState<DomainStatus[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let active = true;
    getStatus()
      .then((res) => {
        if (!active) return;
        if (res.ok) setDomains(res.domains ?? []);
      })
      .catch(() => {/* non-fatal */})
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, []);

  const domainLabel = (domain: string): string => {
    switch (domain) {
      case "containers":
        return t("dashboard.domainContainers");
      case "vms":
        return t("dashboard.domainVMs");
      case "flash":
        return t("dashboard.domainFlash");
      default:
        return domain;
    }
  };

  const rpoLabel = (status: string): string => {
    switch (status) {
      case "ok":
        return t("dashboard.rpoOk");
      case "warn":
        return t("dashboard.rpoWarn");
      case "overdue":
        return t("dashboard.rpoOverdue");
      case "never":
        return t("dashboard.rpoNever");
      default:
        return t("dashboard.rpoOff");
    }
  };

  return (
    <Card title={t("dashboard.protectionTitle")}>
      {loading && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
      )}
      {!loading && domains.length > 0 && (
        <div className="divide-y divide-carbon-border">
          {domains.map((d) => {
            const off = d.status === "off";
            return (
              <div key={d.domain} className="flex items-center gap-3 py-2.5 text-sm">
                <span
                  className={`font-medium w-28 shrink-0 truncate ${
                    off ? "text-carbon-textMuted" : "text-carbon-text"
                  }`}
                >
                  {domainLabel(d.domain)}
                </span>
                {off ? (
                  <span className="text-xs text-carbon-textMuted flex-1">
                    {t("dashboard.rpoOff")}
                  </span>
                ) : (
                  <>
                    <StatusChip status={chipForRpo(d.status)} />
                    <span className="text-carbon-text flex-1 truncate">
                      {rpoLabel(d.status)}
                    </span>
                    <span className="text-carbon-textMuted text-xs shrink-0">
                      {formatCadence(d.schedule, t, lang)}
                    </span>
                    <span
                      className="text-carbon-textMuted text-xs shrink-0 w-20 text-right"
                      title={formatTs(d.lastSuccess)}
                    >
                      {d.lastSuccess ? relativeTime(t, d.lastSuccess) : t("containers.never")}
                    </span>
                    {d.lastVerified ? (
                      <span
                        title={`${t("verify.shield")} · ${formatTs(d.lastVerified)}`}
                        className={`inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-xs font-medium shrink-0 ${
                          d.lastVerifiedOK
                            ? "bg-[#1c3a2a] text-[#6fdc8c] border border-[#2a5540]"
                            : "bg-[#3a1c1c] text-[#ff8389] border border-[#5a2a2a]"
                        }`}
                      >
                        {d.lastVerifiedOK ? "✓" : "✗"} {t("verify.shield")} {relativeTime(t, d.lastVerified)}
                      </span>
                    ) : null}
                  </>
                )}
              </div>
            );
          })}
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
  const [day, setDay] = useState("all");

  useEffect(() => {
    listRuns()
      .then((res) => {
        if (res.ok) setRuns(res.runs ?? []);
        else setError("Failed to load runs");
      })
      .catch(() => setError("Failed to load runs"))
      .finally(() => setLoading(false));
  }, []);

  // Local calendar day of a run, used for the day filter + its labels. Runs come
  // newest-first, so the distinct-days list is already in descending order.
  const dayOf = (run: Run) => new Date(run.startedAt * 1000).toLocaleDateString();
  const days: string[] = [];
  for (const run of runs) {
    const d = dayOf(run);
    if (!days.includes(d)) days.push(d);
  }
  const shown = day === "all" ? runs : runs.filter((run) => dayOf(run) === day);

  return (
    <Card title={t("run.historyTitle")}>
      {loading && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
      )}
      {error && <p className="text-sm text-[#ff8389]">{error}</p>}
      {!loading && !error && runs.length === 0 && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.noRuns")}</p>
      )}
      {runs.length > 0 && (
        <>
          {/* Day filter */}
          <div className="flex items-center gap-2 mb-2">
            <label className="text-xs text-carbon-textMuted">{t("run.filterDay")}</label>
            <select
              value={day}
              onChange={(e) => setDay(e.target.value)}
              className="rounded border border-carbon-border bg-carbon-surface2 px-2 py-1 text-xs text-carbon-text"
            >
              <option value="all">{t("run.allDays")}</option>
              {days.map((d) => (
                <option key={d} value={d}>{d}</option>
              ))}
            </select>
          </div>
          {/* Scrollable list — all runs in the window (filtered by day) */}
          <div className="divide-y divide-carbon-border max-h-96 overflow-y-auto">
            {shown.map((run) => (
              <div key={run.id} className="flex flex-col gap-0.5 py-2.5 text-sm">
                <div className="flex items-center gap-3">
                  <StatusChip status={run.status} />
                  <span className="text-carbon-text font-medium w-16 shrink-0">
                    {run.kind === "backup" ? t("run.kindBackup") : t("run.kindRestore")}
                  </span>
                  <span className="text-carbon-text flex-1 truncate">
                    {run.target || `${run.targetId.slice(0, 12)}…`}
                  </span>
                  <span className="text-carbon-textMuted text-xs shrink-0" title={formatTs(run.startedAt)}>
                    {relativeTime(t, run.startedAt)}
                  </span>
                </div>
                {run.status === "failed" && run.error && (
                  <p className="pl-16 text-xs text-[#ff8389] break-words">{run.error}</p>
                )}
              </div>
            ))}
          </div>
        </>
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
// Backup health heatmap (GitHub-contributions style)
// ---------------------------------------------------------------------------

type HeatDomain = "containers" | "vms" | "flash";

// cellColor maps a day's outcome (for the selected domain) to a fill color:
// any failure → red; all-ok → green shades that deepen with more successful
// runs; no runs → neutral carbon surface.
function cellColor(stat: DayStat | undefined): string {
  if (!stat || (stat.ok === 0 && stat.failed === 0)) return "var(--carbon-surface2, #262626)";
  if (stat.failed > 0) return "#ff8389";
  // All ok — darker green for more runs that day.
  if (stat.ok >= 3) return "#42be65";
  if (stat.ok === 2) return "#6fdc8c";
  return "#a7f0ba";
}

// MS_DAY is one day in milliseconds, used to walk the calendar grid.
const MS_DAY = 86400000;

// mondayIndex returns 0..6 for Mon..Sun (JS getDay() is 0=Sun..6=Sat).
function mondayIndex(d: Date): number {
  return (d.getDay() + 6) % 7;
}

function HealthHeatmapCard({ t }: { t: ReturnType<typeof useT>["t"] }) {
  const [days, setDays] = useState<HistoryDay[]>([]);
  const [loading, setLoading] = useState(true);
  const [domain, setDomain] = useState<HeatDomain>("containers");

  useEffect(() => {
    let active = true;
    getHistory(90)
      .then((res) => {
        if (!active) return;
        if (res.ok) setDays(res.days ?? []);
      })
      .catch(() => {/* non-fatal */})
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, []);

  const byDate = new Map(days.map((d) => [d.date, d]));
  const statFor = (d: HistoryDay | undefined): DayStat | undefined =>
    d ? d[domain] : undefined;

  // Build columns of 7 days (Mon..Sun). Lead the first column with empty cells so
  // each row lines up with its weekday. Parse the YYYY-MM-DD as a local date.
  const cells: Array<{ key: string; date?: string; stat?: DayStat }> = [];
  if (days.length > 0) {
    const first = new Date(days[0].date + "T00:00:00");
    const last = new Date(days[days.length - 1].date + "T00:00:00");
    const lead = mondayIndex(first);
    for (let i = 0; i < lead; i++) {
      cells.push({ key: `lead-${i}` });
    }
    for (let ts = first.getTime(); ts <= last.getTime(); ts += MS_DAY) {
      const iso = new Date(ts).toLocaleDateString("en-CA"); // YYYY-MM-DD, local
      const hd = byDate.get(iso);
      cells.push({ key: iso, date: iso, stat: statFor(hd) });
    }
  }

  // Chunk the flat day list into week columns of 7 (Mon..Sun rows).
  const weeks: Array<typeof cells> = [];
  for (let i = 0; i < cells.length; i += 7) {
    weeks.push(cells.slice(i, i + 7));
  }

  const domainLabel = (d: HeatDomain): string => {
    switch (d) {
      case "containers":
        return t("dashboard.domainContainers");
      case "vms":
        return t("dashboard.domainVMs");
      case "flash":
        return t("dashboard.domainFlash");
    }
  };

  const toggle = (
    <div className="flex items-center gap-1">
      {(["containers", "vms", "flash"] as HeatDomain[]).map((d) => (
        <button
          key={d}
          type="button"
          onClick={() => setDomain(d)}
          className={`px-2 py-0.5 rounded text-xs font-medium border ${
            domain === d
              ? "bg-carbon-surface2 text-carbon-text border-carbon-border"
              : "bg-transparent text-carbon-textMuted border-transparent hover:text-carbon-text"
          }`}
        >
          {domainLabel(d)}
        </button>
      ))}
    </div>
  );

  return (
    <Card title={t("dashboard.healthTitle")} action={toggle}>
      {loading && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
      )}
      {!loading && days.length > 0 && (
        <div className="flex flex-col gap-2">
          <div className="flex gap-1 overflow-x-auto">
            {weeks.map((week, wi) => (
              <div key={wi} className="flex flex-col gap-1">
                {week.map((cell) => {
                  if (!cell.date) {
                    return <div key={cell.key} className="w-[11px] h-[11px]" />;
                  }
                  const stat = cell.stat ?? { ok: 0, failed: 0 };
                  return (
                    <div
                      key={cell.key}
                      className="w-[11px] h-[11px] rounded-sm"
                      style={{ backgroundColor: cellColor(cell.stat) }}
                      title={`${cell.date}: ${stat.ok} ok, ${stat.failed} failed`}
                    />
                  );
                })}
              </div>
            ))}
          </div>
          {/* Legend */}
          <div className="flex items-center gap-1.5 text-xs text-carbon-textMuted">
            <span>{t("dashboard.heatLess")}</span>
            <span className="w-[11px] h-[11px] rounded-sm" style={{ backgroundColor: "var(--carbon-surface2, #262626)" }} />
            <span className="w-[11px] h-[11px] rounded-sm" style={{ backgroundColor: "#a7f0ba" }} />
            <span className="w-[11px] h-[11px] rounded-sm" style={{ backgroundColor: "#6fdc8c" }} />
            <span className="w-[11px] h-[11px] rounded-sm" style={{ backgroundColor: "#42be65" }} />
            <span>{t("dashboard.heatMore")}</span>
          </div>
        </div>
      )}
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Sparkline — hand-rolled inline SVG trend line (no charting lib)
// ---------------------------------------------------------------------------

function Sparkline({
  values,
  width = 120,
  height = 28,
}: {
  values: number[];
  width?: number;
  height?: number;
}) {
  // Need at least two points to draw a line.
  if (!values || values.length < 2) return null;

  const min = Math.min(...values);
  const max = Math.max(...values);
  const span = max - min;
  const pad = 2; // keep the stroke off the edges
  const usableH = height - pad * 2;
  const usableW = width - pad * 2;
  const step = usableW / (values.length - 1);

  const points = values
    .map((v, i) => {
      const x = pad + i * step;
      // Flat line when all values are equal (avoid divide-by-zero).
      const y = span === 0 ? height / 2 : pad + usableH - ((v - min) / span) * usableH;
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(" ");

  return (
    <span className="text-[#78a9ff] shrink-0">
      <svg
        width={width}
        height={height}
        viewBox={`0 0 ${width} ${height}`}
        className="block"
        aria-hidden="true"
      >
        <polyline
          points={points}
          fill="none"
          stroke="currentColor"
          strokeWidth={1.5}
          strokeLinejoin="round"
          strokeLinecap="round"
        />
      </svg>
    </span>
  );
}

// ---------------------------------------------------------------------------
// Storage card — repo size + dedup trend per domain
// ---------------------------------------------------------------------------

type StorageDomain = "containers" | "vms" | "flash";

interface DomainStats {
  domain: StorageDomain;
  stats: RepoStat[];
  latest: RepoStat | null;
}

function StorageCard({ t }: { t: ReturnType<typeof useT>["t"] }) {
  const [data, setData] = useState<DomainStats[] | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let active = true;
    const domains: StorageDomain[] = ["containers", "vms", "flash"];
    Promise.all(domains.map((d) => getStats(d, "local", 90)))
      .then((results) => {
        if (!active) return;
        setData(
          results.map((res, i) => ({
            domain: domains[i],
            stats: res.ok ? (res.stats ?? []) : [],
            latest: res.ok ? (res.latest ?? null) : null,
          }))
        );
      })
      .catch(() => {/* non-fatal */})
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, []);

  const domainLabel = (d: StorageDomain): string => {
    switch (d) {
      case "containers":
        return t("dashboard.domainContainers");
      case "vms":
        return t("dashboard.domainVMs");
      case "flash":
        return t("dashboard.domainFlash");
    }
  };

  const anyData = !!data && data.some((d) => d.latest != null);

  return (
    <Card title={t("dashboard.storageTitle")}>
      {loading && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
      )}
      {!loading && !anyData && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.noStats")}</p>
      )}
      {!loading && anyData && data && (
        <div className="divide-y divide-carbon-border">
          {data.map((d) => {
            const has = d.latest != null;
            const dedup =
              d.latest && d.latest.restoreSize > 0 && d.latest.rawSize > 0
                ? `${(d.latest.restoreSize / d.latest.rawSize).toFixed(1)}x`
                : "—";
            return (
              <div key={d.domain} className="flex items-center gap-3 py-2.5 text-sm">
                <span
                  className={`font-medium w-28 shrink-0 truncate ${
                    has ? "text-carbon-text" : "text-carbon-textMuted"
                  }`}
                >
                  {domainLabel(d.domain)}
                </span>
                {has && d.latest ? (
                  <>
                    <span className="text-carbon-text tabular-nums w-20 shrink-0 text-right">
                      {humanBytes(d.latest.rawSize)}
                    </span>
                    <span className="text-carbon-textMuted text-xs shrink-0 w-24">
                      {t("dashboard.dedup")} {dedup}
                    </span>
                    <span className="text-carbon-textMuted text-xs shrink-0 w-24 truncate">
                      {d.latest.snapshots} {t("dashboard.snapshotsLabel")}
                    </span>
                    <span className="flex-1" />
                    <Sparkline values={d.stats.map((s) => s.rawSize)} />
                  </>
                ) : (
                  <span className="text-xs text-carbon-textMuted flex-1">
                    {t("dashboard.noStats")}
                  </span>
                )}
              </div>
            );
          })}
        </div>
      )}
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Recovery-kit nag — shown only when encryption is ON and the kit has not been
// acknowledged. Prompts the user to download + safely store the encryption
// recovery kit so disaster recovery works even without a running BombVault.
// ---------------------------------------------------------------------------

function RecoveryNag({ t }: { t: ReturnType<typeof useT>["t"] }) {
  const [settings, setSettings] = useState<Settings | null>(null);
  const [dismissing, setDismissing] = useState(false);

  useEffect(() => {
    let active = true;
    getSettings()
      .then((res) => {
        if (active && res.ok) setSettings(res.settings);
      })
      .catch(() => {/* non-fatal */});
    return () => {
      active = false;
    };
  }, []);

  if (!settings || !settings.encryptionEnabled || settings.recoveryKitAck) {
    return null;
  }

  const dismiss = () => {
    setDismissing(true);
    void ackRecoveryKit()
      .then((res) => {
        if (res.ok) setSettings({ ...settings, recoveryKitAck: true });
      })
      .catch(() => {/* non-fatal */})
      .finally(() => setDismissing(false));
  };

  return (
    <div className="rounded-card border border-[#4a4a2a] bg-[#2a2a1c] px-4 py-3 flex flex-col gap-2">
      <h2 className="text-sm font-semibold text-[#f1c21b]">
        {t("recovery.nagTitle")}
      </h2>
      <p className="text-xs text-[#f1c21b] leading-relaxed">
        {t("recovery.nagBody")}
      </p>
      <div className="flex flex-wrap items-center gap-2">
        <a
          href={recoveryKitUrl()}
          download="bombvault-recovery-kit.md"
          className="rounded-md bg-carbon-surface3 hover:bg-carbon-border px-3 py-1.5 text-sm text-carbon-text transition-colors"
        >
          {t("recovery.download")}
        </a>
        <button
          type="button"
          onClick={dismiss}
          disabled={dismissing}
          className="rounded-md border border-carbon-border px-3 py-1.5 text-sm text-carbon-textSub hover:text-carbon-text transition-colors disabled:opacity-50"
        >
          {t("recovery.stored")}
        </button>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Dashboard page
// ---------------------------------------------------------------------------

export function Dashboard() {
  const { t } = useT();
  const { advanced } = useAdvanced();

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
        <div className="mt-2 flex flex-col gap-1">
          <OffsiteIndicator domain="containers" withLabel />
          <OffsiteIndicator domain="vms" withLabel />
          <OffsiteIndicator domain="flash" withLabel />
        </div>
      </div>

      {/* Recovery-kit nag — only while encryption is on and the kit is unstored */}
      <RecoveryNag t={t} />

      {/* Stat cards — compact summary row */}
      <StatCardsRow t={t} />

      {/* Protection (RPO) status — "are my backups current?" indicator */}
      <ProtectionCard t={t} />

      {/* 2-column grid for last backups + run history */}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <LastBackupsCard t={t} />
        <RunsCard t={t} />
      </div>

      {/* Backup health heatmap — full width */}
      <HealthHeatmapCard t={t} />

      {/* Storage — repo size + dedup trend per domain — full width */}
      <StorageCard t={t} />

      {/* Spike (host-integration) status is the only advanced-gated dashboard card. */}
      {advanced && <SpikeCard t={t} />}
    </div>
  );
}
