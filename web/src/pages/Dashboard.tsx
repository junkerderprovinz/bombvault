import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { listRuns, getSpike, listContainers, listVMs, getSettings, getStatus, getHistory, getStats, recoveryKitUrl, ackRecoveryKit, runDrill } from "../lib/api";
import type { Run, SpikeCheck, Container, Settings, DomainStatus, HistoryDay, DayStat, RepoStat } from "../lib/api";
import { useT } from "../lib/i18n";
import { useAdvanced } from "../lib/advanced";
import { OffsiteIndicator } from "../components/OffsiteIndicator";
import { formatCadence, parseCadenceString } from "../components/CadenceBuilder";
import type { CadenceState } from "../components/CadenceBuilder";
import { relativeTime, formatTs, formatDuration } from "../lib/reltime";
import { isFreshInstall } from "../lib/freshInstall";
import { useDashboardLayout, CustomizableBlock, type BlockDragHandlers } from "../lib/dashboardLayout";

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

function StatCardsRow({ t, advanced }: { t: ReturnType<typeof useT>["t"]; advanced: boolean }) {
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
    <div className={`grid grid-cols-2 gap-3 ${advanced ? "sm:grid-cols-4 lg:grid-cols-7" : "sm:grid-cols-3"}`}>
      <StatCard label={t("dashboard.statContainers")} value={data.containers} />
      <StatCard label={t("dashboard.statVMs")} value={data.vms} />
      {advanced && (
        <>
          <StatCard label={t("dashboard.statActiveJobs")} value={data.activeJobs} />
          <StatCard label={t("dashboard.statPausedJobs")} value={data.pausedJobs} />
        </>
      )}
      <StatCard label={t("dashboard.statErrors")} value={data.errors} danger />
      {advanced && (
        <>
          <StatCard label={t("dashboard.statMissingContainers")} value={data.missingContainers} danger />
          <StatCard label={t("dashboard.statMissingVMs")} value={data.missingVMs} />
        </>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Status chip
// ---------------------------------------------------------------------------

function StatusChip({
  status,
}: {
  status: "success" | "failed" | "running" | "ok" | "degraded" | "checking" | "skipped" | string;
}) {
  const map: Record<string, string> = {
    success: "bg-[#1c3a2a] text-[#6fdc8c] border border-[#2a5540]",
    ok:      "bg-[#1c3a2a] text-[#6fdc8c] border border-[#2a5540]",
    failed:  "bg-[#3a1c1c] text-[#ff8389] border border-[#5a2a2a]",
    degraded:"bg-[#3a1c1c] text-[#ff8389] border border-[#5a2a2a]",
    running: "bg-[#1c2a3a] text-[#78a9ff] border border-[#2a3a5a]",
    checking:"bg-[#1c2a3a] text-[#78a9ff] border border-[#2a3a5a]",
    info:    "bg-[#2a2a1c] text-[#f1c21b] border border-[#4a4a2a]",
    // A skip is neither success nor failure: a muted, neutral chip so a removed
    // container's scheduled target reads as "intentionally not run", distinct
    // from green success and red failure (#57).
    skipped: "bg-[#2a2a2e] text-[#a8a8b0] border border-[#3a3a40]",
  };
  const cls = map[status.toLowerCase()] ?? "bg-carbon-surface2 text-carbon-textSub border border-carbon-border";
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded-sm text-xs font-medium ${cls}`}>
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

function ProtectionCard({
  t,
  domains,
  loading,
}: {
  t: ReturnType<typeof useT>["t"];
  domains: DomainStatus[];
  loading: boolean;
}) {
  const { lang } = useT();

  // Manual off-site DR run, triggered from a failing DR row so a pass clears the
  // red. `drRunning` is the domain whose DR check is in flight; `drRunError`
  // holds the last returned failure detail per domain (shown next to the button).
  const [drRunning, setDrRunning] = useState<string | null>(null);
  const [drRunError, setDrRunError] = useState<Record<string, string>>({});

  // Keep a domain's transient manual-run message only where the Run-DR button is
  // actually reachable (a DR-capable, non-off domain with an off-site repo), and
  // drop it elsewhere so a refetch (including one triggered by ANOTHER domain's
  // run) can't resurface a stale error. The next run for that domain clears it.
  useEffect(() => {
    setDrRunError((prev) => {
      const next: Record<string, string> = {};
      for (const d of domains) {
        const drCapable = d.domain === "containers" || d.domain === "flash" || d.domain === "files";
        const reachable = drCapable && d.status !== "off" && d.offsiteConfigured;
        if (reachable && prev[d.domain] !== undefined) next[d.domain] = prev[d.domain];
      }
      return Object.keys(next).length === Object.keys(prev).length ? prev : next;
    });
  }, [domains]);

  const runOffsiteDr = (domain: string) => {
    setDrRunning(domain);
    setDrRunError((e) => {
      const next = { ...e };
      delete next[domain];
      return next;
    });
    void runDrill(domain, "offsite", "dr")
      .then((res) => {
        // A drill that actually ran (pass OR fail) is recorded and surfaced by the
        // status refetch below via d.drillDetail — don't duplicate it here. Only a
        // run that produced NO recorded row (e.g. the repo was busy) needs its own
        // transient message next to the button.
        if (!res.ok && !res.drill) {
          setDrRunError((e) => ({ ...e, [domain]: res.error ?? t("verify.failed") }));
        }
      })
      .catch((err) => {
        setDrRunError((e) => ({
          ...e,
          [domain]: err instanceof Error ? err.message : t("verify.failed"),
        }));
      })
      .finally(() => {
        setDrRunning((cur) => (cur === domain ? null : cur));
        // Refetch the shared /api/status so a pass clears the red DR pill + reason
        // (the Dashboard page listens for this event and reloads getStatus()).
        window.dispatchEvent(new Event("bv:settings-changed"));
      });
  };

  const domainLabel = (domain: string): string => {
    switch (domain) {
      case "containers":
        return t("dashboard.domainContainers");
      case "vms":
        return t("dashboard.domainVMs");
      case "flash":
        return t("dashboard.domainFlash");
      case "files":
        return t("dashboard.domainFiles");
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
            // Only containers, flash + files ever run an off-site DR drill
            // (schedule.go drillTasks / runDRDrill). VMs + config can have an
            // off-site repo but cannot be DR-drilled, so they must show NO DR
            // pill or Run-DR button.
            const drCapable = d.domain === "containers" || d.domain === "flash" || d.domain === "files";
            // Off-site DR opt-out (#37): the scheduled DR drill is turned off for a
            // DR-capable domain that HAS an off-site repo. The pill then reads NEUTRAL
            // ("manual only") — but only when there is no failing result to show.
            const drUnscheduled = drCapable && d.offsiteConfigured && !d.offsiteDrillScheduled;
            // The red "proven restorable off-site" state: a recorded off-site DR drill
            // that failed. A real failure (scheduled OR a manual run) is ALWAYS shown,
            // never masked by the opt-out — only "never drilled" goes neutral.
            const drFailed = !off && drCapable && d.lastDrDrillAt > 0 && !d.lastDrDrillOK;
            return (
              <div key={d.domain} className="flex flex-col gap-1 py-2.5 text-sm">
                {/* Shared column grid (#66) — every row lays its cells on the SAME
                    track template so the same kind of info sits in the same column
                    down the whole card, regardless of which cells a row populates:
                    [domain] [status] [schedule] [last run] [verified] [off-site
                    verified] [off-site DR]. Fixed tracks for the fixed-width columns,
                    fr tracks for the text/badges so a long badge (e.g. "proven
                    restorable from off-site") wraps inside its column instead of
                    overflowing the card, and absent badges just leave their column
                    blank without re-flowing the others. The three badge columns get a
                    readable floor width (not minmax(0,…)) so they wrap at word
                    boundaries instead of being squeezed thin enough to hyphenate
                    mid-word; overflow-x-auto on the row is the fallback once the
                    floors no longer fit a narrow viewport. */}
                <div className="grid items-center gap-3 overflow-x-auto grid-cols-[7rem_minmax(0,1fr)_minmax(0,1fr)_5rem_minmax(6.5rem,1fr)_minmax(7.5rem,1.2fr)_minmax(7.5rem,1.6fr)]">
                  <span
                    className={`col-start-1 min-w-0 truncate font-medium ${
                      off ? "text-carbon-textMuted" : "text-carbon-text"
                    }`}
                  >
                    {domainLabel(d.domain)}
                  </span>
                  {off ? (
                    <span className="col-start-2 col-span-6 min-w-0 truncate text-xs text-carbon-textMuted">
                      {t("dashboard.rpoOff")}
                    </span>
                  ) : (
                    <>
                      {/* Col 2 — status: the RPO chip + its label kept together so the
                          pill never drifts from the words it qualifies. */}
                      <div className="col-start-2 flex min-w-0 items-center gap-2">
                        <StatusChip status={chipForRpo(d.status)} />
                        <span className="min-w-0 truncate text-carbon-text">
                          {rpoLabel(d.status)}
                        </span>
                      </div>
                      {/* Col 3 — schedule cadence. */}
                      <span className="col-start-3 min-w-0 truncate text-carbon-textMuted text-xs">
                        {formatCadence(d.schedule, t, lang)}
                      </span>
                      {/* Col 4 — last successful run. */}
                      <span
                        className="col-start-4 text-right text-carbon-textMuted text-xs"
                        title={formatTs(d.lastSuccess)}
                      >
                        {d.lastSuccess ? relativeTime(t, d.lastSuccess) : t("containers.never")}
                      </span>
                      {/* Col 5 — local-verify shield badge. */}
                      {d.lastVerified ? (
                        <div className="col-start-5 min-w-0">
                          <span
                            title={`${t("verify.shield")} · ${formatTs(d.lastVerified)}`}
                            className={`inline-flex max-w-full items-center gap-1 px-1.5 py-0.5 rounded text-xs font-medium break-normal ${
                              d.lastVerifiedOK
                                ? "bg-[#1c3a2a] text-[#6fdc8c] border border-[#2a5540]"
                                : "bg-[#3a1c1c] text-[#ff8389] border border-[#5a2a2a]"
                            }`}
                          >
                            {d.lastVerifiedOK ? "✓" : "✗"} {t("verify.shield")} {relativeTime(t, d.lastVerified)}
                          </span>
                        </div>
                      ) : null}
                      {/* Col 6 — Off-site SUBSET badge (#63) — the off-site integrity
                          check (`restic check --read-data-subset` against the off-site
                          repo). Mirrors the local-verify shield above (same pills)
                          and is the ONLY off-site drill VMs can run (DR restores
                          are refused for them), so it shows for EVERY domain with
                          a recorded run — alongside, never instead of, the DR pill
                          below. */}
                      {d.lastOffsiteSubsetAt ? (
                        <div className="col-start-6 min-w-0">
                          <span
                            title={`${t("drill.offsiteVerified")} · ${formatTs(d.lastOffsiteSubsetAt)}`}
                            className={`inline-flex max-w-full items-center gap-1 px-1.5 py-0.5 rounded text-xs font-medium break-normal ${
                              d.lastOffsiteSubsetOK
                                ? "bg-[#1c3a2a] text-[#6fdc8c] border border-[#2a5540]"
                                : "bg-[#3a1c1c] text-[#ff8389] border border-[#5a2a2a]"
                            }`}
                          >
                            {d.lastOffsiteSubsetOK ? "✓" : "✗"} {t("drill.offsiteVerified")} {relativeTime(t, d.lastOffsiteSubsetAt)}
                          </span>
                        </div>
                      ) : null}
                      {/* Col 7 — Off-site restorability (DR) badge — mirrors the
                          local-verify shield above (same pills), but proves the backup
                          is recoverable from the OFF-SITE repo (a real DR sandbox
                          restore). Only containers + flash + files ever run a DR drill,
                          so VMs/config never show this pill (empty column). On a
                          failure the tooltip names WHICH check + the reason. */}
                      <div className="col-start-7 min-w-0">
                        {drCapable && d.lastDrDrillAt && d.lastDrDrillOK ? (
                          // GREEN — proven restorable off-site. A real passed run (even
                          // a MANUAL one) is honest proof, so it's kept even when the
                          // scheduled DR drill is opted out.
                          <span
                            title={`${t("drill.provenOffsite")} · ${formatTs(d.lastDrDrillAt)}`}
                            className="inline-flex max-w-full items-center gap-1 px-1.5 py-0.5 rounded-sm text-xs font-medium break-normal bg-[#1c3a2a] text-[#6fdc8c] border border-[#2a5540]"
                          >
                            ✓ {t("drill.provenOffsite")} · {relativeTime(t, d.lastDrDrillAt)}
                          </span>
                        ) : drFailed ? (
                          // RED — a recorded off-site DR drill FAILED (scheduled or a
                          // manual run). Always shown; the opt-out never masks a real
                          // failure — only "never drilled" goes neutral below.
                          <span
                            title={
                              d.drillDetail
                                ? `${t("drill.checkOffsiteDr")} · ${t("drill.failReasonPrefix")} ${d.drillDetail} · ${formatTs(d.lastDrDrillAt)}`
                                : `${t("drill.provenOffsite")} · ${formatTs(d.lastDrDrillAt)}`
                            }
                            className="inline-flex max-w-full items-center gap-1 px-1.5 py-0.5 rounded-sm text-xs font-medium break-normal bg-[#3a1c1c] text-[#ff8389] border border-[#5a2a2a]"
                          >
                            ✗ {t("drill.provenOffsite")} · {relativeTime(t, d.lastDrDrillAt)}
                          </span>
                        ) : drUnscheduled ? (
                          // NEUTRAL — off-site DR not scheduled (manual only) and nothing
                          // failing to show: muted, never red. File's no-claim styling.
                          <span
                            title={t("drill.manualOnlyTitle")}
                            className="inline-flex max-w-full items-center gap-1 px-1.5 py-0.5 rounded-sm text-xs font-medium break-normal bg-carbon-surface2 text-carbon-textMuted border border-carbon-border"
                          >
                            {t("drill.manualOnly")}
                          </span>
                        ) : null}
                      </div>
                    </>
                  )}
                </div>
                {/* Manual off-site DR: the "Run off-site DR check" button is always
                    reachable for a configured domain (so a manual run works when
                    opted out AND when currently green), while the red WHICH-check +
                    WHY reason stays gated to an actual scheduled failure (drFailed).
                    Only the off-site DR row drives that red — a local subset pass
                    can't clear it, so we run {offsite,dr} explicitly. */}
                {!off && drCapable && (drFailed || d.offsiteConfigured) && (
                  <div className="flex flex-wrap items-center gap-2 pl-1">
                    {drFailed && d.drillDetail && (
                      <span className="text-xs text-[#ff8389] wrap-break-word" title={d.drillDetail}>
                        {t("drill.checkOffsiteDr")} · {t("drill.failReasonPrefix")} {d.drillDetail}
                      </span>
                    )}
                    {d.offsiteConfigured && (
                      <button
                        type="button"
                        onClick={() => runOffsiteDr(d.domain)}
                        disabled={drRunning === d.domain}
                        className="rounded-md border border-carbon-border bg-carbon-surface2 px-2 py-1 text-xs text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
                      >
                        {drRunning === d.domain
                          ? t("drill.runningOffsiteDr")
                          : d.lastDrDrillAt && d.lastDrDrillOK
                            ? t("drill.rerunOffsiteDr")
                            : t("drill.runOffsiteDr")}
                      </button>
                    )}
                    {drRunError[d.domain] && (
                      <span className="text-xs text-[#ff8389] wrap-break-word">✗ {drRunError[d.domain]}</span>
                    )}
                  </div>
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
// Ransomware protection card
// ---------------------------------------------------------------------------

// protectionChip maps the red/amber/green aggregate to a StatusChip variant.
function protectionChip(level: string): string {
  switch (level) {
    case "green":
      return "ok";
    case "amber":
      return "info";
    case "red":
      return "failed";
    default:
      return "neutral";
  }
}

// A checklist row: "ok" (proven, green), "amber" (a currency lapse that mirrors
// the chip's amber — stale/overdue), "bad" (a red gap → deep-links to Settings),
// or "muted" (not applicable / never run — no claim made, so not a failure).
type RowState = "ok" | "amber" | "bad" | "muted";

function RansomwareCard({
  t,
  domains,
  loading,
}: {
  t: ReturnType<typeof useT>["t"];
  domains: DomainStatus[];
  loading: boolean;
}) {
  // Pure renderer: every row is derived from the extended /api/status domain
  // fields (tamperState/replicationState/drillState/encryptionOn/pruneStrategySet),
  // which the backend computes from the SAME inputs as the aggregate chip — so a
  // row can never contradict it, and the card needs no /api/settings round-trip.
  const domainLabel = (domain: string): string => {
    switch (domain) {
      case "containers":
        return t("dashboard.domainContainers");
      case "vms":
        return t("dashboard.domainVMs");
      case "flash":
        return t("dashboard.domainFlash");
      case "files":
        return t("dashboard.domainFiles");
      default:
        return domain;
    }
  };

  const protLabel = (level: string): string => {
    switch (level) {
      case "green":
        return t("ransomware.protGreen");
      case "amber":
        return t("ransomware.protAmber");
      default:
        return t("ransomware.protRed");
    }
  };

  // In scope: enabled domains that carry a protection posture (protection != "").
  const shown = domains.filter((d) => d.enabled && d.protection !== "");
  // Render nothing at all when no domain is in scope (nobody has off-site yet).
  if (!loading && shown.length === 0) return null;

  const ageText = (at: number): string => (at > 0 ? relativeTime(t, at) : t("containers.never"));

  // appendOnly/replication/drill rows are pure maps of the backend state string
  // (which is kept consistent with the chip). The ✓/!/✗/— icon + label + color all
  // follow the state, so a red/never row never reads "verified".
  const appendOnlyRow = (d: DomainStatus): { label: string; state: RowState; at?: number } => {
    switch (d.tamperState) {
      case "ok":
        return { label: t("ransomware.appendOnlyVerified"), state: "ok", at: d.lastTamperAt };
      case "stale":
        return { label: t("ransomware.appendOnlyStale"), state: "amber", at: d.lastTamperAt };
      case "failed":
        return { label: t("ransomware.appendOnlyFailed"), state: "bad", at: d.lastTamperAt };
      case "never":
        return { label: t("ransomware.appendOnlyNever"), state: "bad" };
      default:
        return { label: t("ransomware.appendOnlyOff"), state: "muted" };
    }
  };
  const replicationRow = (d: DomainStatus): { label: string; state: RowState; at?: number } => {
    switch (d.replicationState) {
      case "ok":
        return { label: t("ransomware.replicationCurrent"), state: "ok", at: d.lastReplicationAt };
      case "overdue":
        return { label: t("ransomware.replicationOverdue"), state: "amber", at: d.lastReplicationAt };
      case "never":
        return { label: t("ransomware.replicationNever"), state: "muted" };
      default:
        // "" — replication is coupled to each backup (no independent expectation).
        return { label: t("ransomware.replicationCurrent"), state: "muted" };
    }
  };
  const drillRow = (d: DomainStatus): { label: string; state: RowState; at?: number; detail?: string } => {
    switch (d.drillState) {
      case "ok":
        return { label: t("ransomware.drillOffsite"), state: "ok", at: d.lastDrDrillAt };
      case "failed":
        // The latest off-site DR drill FAILED — red, matching the "proven
        // restorable" pill, regardless of how recently it ran. Carry the scrubbed
        // reason so the row can say WHY (and WHICH check) it failed.
        return { label: t("ransomware.drillFailed"), state: "bad", at: d.lastDrDrillAt, detail: d.drillDetail };
      case "overdue":
        return { label: t("ransomware.drillOverdue"), state: "amber", at: d.lastDrDrillAt };
      case "never":
        return { label: t("ransomware.drillNever"), state: "muted" };
      default:
        // "" — no drill schedule set, so no claim.
        return { label: t("ransomware.drillOffsite"), state: "muted" };
    }
  };

  return (
    <Card title={t("ransomware.title")}>
      {loading && <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>}
      {!loading &&
        shown.map((d) => {
          // Each row: label, state, and an optional age stamp. A "bad" row is a red
          // gap the user should fix — it deep-links into Settings. Every state comes
          // from the backend so it cannot diverge from the chip above.
          const ao = appendOnlyRow(d);
          const rep = replicationRow(d);
          const dr = drillRow(d);
          const rows: { key: string; label: string; state: RowState; at?: number; detail?: string }[] = [
            {
              key: "configured",
              label: t("ransomware.configured"),
              state: d.offsiteConfigured ? "ok" : "bad",
            },
            { key: "appendOnly", ...ao },
            { key: "replication", ...rep },
            { key: "drill", ...dr },
            {
              key: "encryption",
              label: t("ransomware.encryptionOn"),
              state: d.encryptionOn ? "ok" : "bad",
            },
            {
              key: "prune",
              label: t("ransomware.pruneStrategy"),
              state: d.pruneStrategySet ? "ok" : "bad",
            },
          ];

          return (
            <div key={d.domain} className="flex flex-col gap-1.5 py-2 border-b border-carbon-border last:border-0">
              <div className="flex items-center gap-2">
                <span className="font-medium text-carbon-text w-28 shrink-0 truncate">
                  {domainLabel(d.domain)}
                </span>
                <StatusChip status={protectionChip(d.protection)} />
                <span className="text-sm text-carbon-textSub">{protLabel(d.protection)}</span>
              </div>
              <div className="flex flex-col gap-0.5 pl-1">
                {rows.map((row) => {
                  const icon =
                    row.state === "ok" ? "✓" : row.state === "amber" ? "!" : row.state === "bad" ? "✗" : "—";
                  const iconColor =
                    row.state === "ok"
                      ? "text-[#6fdc8c]"
                      : row.state === "amber"
                        ? "text-[#f1c21b]"
                        : row.state === "bad"
                          ? "text-[#ff8389]"
                          : "text-carbon-textMuted";
                  const labelColor =
                    row.state === "amber"
                      ? "text-[#f1c21b]"
                      : row.state === "muted"
                        ? "text-carbon-textMuted"
                        : "text-carbon-textSub";
                  return (
                    <div key={row.key} className="flex flex-col gap-0.5">
                      <div className="flex items-center gap-2 text-sm">
                        <span className={`w-4 shrink-0 text-center ${iconColor}`}>{icon}</span>
                        {row.state === "bad" ? (
                          <Link to="/settings#offsite" className="text-[#ff8389] hover:underline flex-1 truncate">
                            {row.label}
                          </Link>
                        ) : (
                          <span className={`flex-1 truncate ${labelColor}`}>{row.label}</span>
                        )}
                        {row.at !== undefined && (
                          <span className="text-xs text-carbon-textMuted shrink-0">{ageText(row.at)}</span>
                        )}
                      </div>
                      {/* WHICH check + WHY it failed (off-site DR reason from /api/status). */}
                      {row.detail && (
                        <span className="text-xs text-[#ff8389] wrap-break-word pl-6" title={row.detail}>
                          {t("drill.checkOffsiteDr")} · {t("drill.failReasonPrefix")} {row.detail}
                        </span>
                      )}
                    </div>
                  );
                })}
              </div>
            </div>
          );
        })}
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
              className="rounded-sm border border-carbon-border bg-carbon-surface2 px-2 py-1 text-xs text-carbon-text"
            >
              <option value="all">{t("run.allDays")}</option>
              {days.map((d) => (
                <option key={d} value={d}>{d}</option>
              ))}
            </select>
          </div>
          {/* Scrollable list — all runs in the window (filtered by day) */}
          <div className="divide-y divide-carbon-border max-h-128 overflow-y-auto pr-2">
            {shown.map((run) => {
              const dur = run.finishedAt != null ? formatDuration(run.finishedAt - run.startedAt) : "";
              return (
              <div key={run.id} className="flex flex-col gap-0.5 py-2.5 text-sm">
                <div className="flex items-center gap-3">
                  <StatusChip status={run.status} />
                  <span className="text-carbon-text font-medium w-16 shrink-0">
                    {run.kind === "backup"
                      ? t("run.kindBackup")
                      : run.kind === "update"
                        ? t("run.kindUpdate")
                        : t("run.kindRestore")}
                  </span>
                  <span className="text-carbon-text flex-1 truncate">
                    {run.target || `${run.targetId.slice(0, 12)}…`}
                  </span>
                  {/* Start → end + duration, with the relative age underneath (#45/#50). */}
                  <span className="flex flex-col items-end shrink-0 text-xs leading-tight">
                    <span className="text-carbon-textSub whitespace-nowrap">
                      {formatTs(run.startedAt)}
                      {run.finishedAt != null ? ` → ${formatTs(run.finishedAt)}` : ""}
                    </span>
                    <span className="text-carbon-textMuted whitespace-nowrap">
                      {dur ? `(${dur}) · ` : ""}
                      {relativeTime(t, run.startedAt)}
                    </span>
                  </span>
                </div>
                {run.status === "failed" && run.error && (
                  <p className="pl-16 text-xs text-[#ff8389] wrap-break-word">{run.error}</p>
                )}
                {run.status === "skipped" && run.error && (
                  <p className="pl-16 text-xs text-carbon-textMuted wrap-break-word">{run.error}</p>
                )}
              </div>
              );
            })}
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
          {withBackups.map((c) => {
            // Older data (or a run before the start time was recorded) has no
            // lastBackupStarted — fall back to just the finish time, never a
            // negative/broken duration.
            const hasStart = c.lastBackupStarted != null && c.lastBackup != null;
            const duration = hasStart
              ? formatDuration((c.lastBackup as number) - (c.lastBackupStarted as number))
              : "";
            return (
              <div key={c.name} className="flex items-center gap-3 py-2.5 text-sm">
                <div className="w-2 h-2 rounded-full bg-[#6fdc8c] shrink-0" />
                <span className="text-carbon-text font-medium flex-1 truncate">{c.name}</span>
                {hasStart ? (
                  <span className="text-carbon-textMuted text-xs shrink-0 text-right">
                    {formatTs(c.lastBackupStarted)} → {formatTs(c.lastBackup)}
                    {duration && (
                      <span className="ml-1" title={t("dashboard.duration")} aria-label={t("dashboard.duration")}>
                        ({duration})
                      </span>
                    )}
                  </span>
                ) : (
                  <span className="text-carbon-textMuted text-xs shrink-0">
                    {formatTs(c.lastBackup)}
                  </span>
                )}
              </div>
            );
          })}
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

type HeatDomain = "containers" | "vms" | "flash" | "config" | "files";

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
      case "config":
        return t("dashboard.domainConfig");
      case "files":
        return t("dashboard.domainFiles");
    }
  };

  const toggle = (
    <div className="flex items-center gap-1">
      {(["containers", "vms", "flash", "config", "files"] as HeatDomain[]).map((d) => (
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
                      className="w-[11px] h-[11px] rounded-xs"
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
            <span className="w-[11px] h-[11px] rounded-xs" style={{ backgroundColor: "var(--carbon-surface2, #262626)" }} />
            <span className="w-[11px] h-[11px] rounded-xs" style={{ backgroundColor: "#a7f0ba" }} />
            <span className="w-[11px] h-[11px] rounded-xs" style={{ backgroundColor: "#6fdc8c" }} />
            <span className="w-[11px] h-[11px] rounded-xs" style={{ backgroundColor: "#42be65" }} />
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

type StorageDomain = "containers" | "vms" | "flash" | "files";

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
    const domains: StorageDomain[] = ["containers", "vms", "flash", "files"];
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
      case "files":
        return t("dashboard.domainFiles");
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

function RecoveryNag({ t, suppressed }: { t: ReturnType<typeof useT>["t"]; suppressed?: boolean }) {
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

  if (suppressed) return null;
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
// Fresh-install nudge — on a brand-new or rebuilt install (no domain has ever
// backed up successfully) point the user at the guided Recovery tab to recover
// their existing backups. Dismissible; the dismissal persists in localStorage.
// The fresh signal is derived purely from the shared /api/status domains the
// dashboard already fetched — no extra round-trip, and nothing is fetched or
// computed once dismissed.
// ---------------------------------------------------------------------------

const RECOVERY_NUDGE_DISMISSED = "bombvault.recoveryNudgeDismissed";

function FreshInstallNudge({
  t,
  domains,
  loading,
  dismissed,
  onDismiss,
}: {
  t: ReturnType<typeof useT>["t"];
  domains: DomainStatus[];
  loading: boolean;
  dismissed: boolean;
  onDismiss: () => void;
}) {
  // Gate: do nothing (and read nothing) once dismissed or while status is still
  // loading. Only then is the fresh predicate evaluated against shared data.
  if (dismissed || loading) return null;
  if (!isFreshInstall(domains)) return null;

  return (
    <div className="bg-carbon-surface rounded-card border border-carbon-border p-5 flex items-center gap-4">
      <div className="flex-1 flex flex-col gap-1.5">
        <p className="text-sm text-carbon-text">{t("recovery.freshNudge")}</p>
        <Link
          to="/recovery"
          className="self-start text-sm font-medium hover:underline"
          style={{ color: "var(--accent)" }}
        >
          {t("recovery.freshNudgeCta")} →
        </Link>
      </div>
      <button
        type="button"
        onClick={onDismiss}
        aria-label={t("common.close")}
        className="shrink-0 rounded-md border border-carbon-border px-2 py-1 text-sm text-carbon-textSub hover:text-carbon-text transition-colors"
      >
        ✕
      </button>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Summary tier — a compact three-cell overview above the detail cards. It
// reuses the same Card/StatCard surface + StatusChip visual language as the
// detail tier below, reading the shared /api/status domains and the newest
// listRuns entry (no extra round-trips beyond the one runs fetch in the parent).
// ---------------------------------------------------------------------------

// cadencePeriodDays approximates how often a parsed cadence fires, in days, so
// the soonest (most frequent) enabled schedule can be picked WITHOUT a live
// next-run timestamp (the backend has none). Smaller = fires sooner; "off"
// yields Infinity so it never wins.
function cadencePeriodDays(s: CadenceState): number {
  switch (s.mode) {
    case "daily":
      return 1;
    case "everyN":
      return Math.max(1, s.intervalDays);
    case "weekly":
      return 7 / Math.max(1, s.weekdays.length);
    default:
      return Infinity;
  }
}

// minutesOfDay turns "HH:MM" into minutes since midnight — a stable tiebreak
// between two equally-frequent schedules (the earlier clock time wins).
function minutesOfDay(hhmm: string): number {
  const m = /^(\d{1,2}):(\d{2})$/.exec(hhmm);
  return m ? parseInt(m[1], 10) * 60 + parseInt(m[2], 10) : 24 * 60;
}

function SummaryCell({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="bg-carbon-surface rounded-card border border-carbon-border px-4 py-3 flex flex-col gap-2">
      <span className="text-xs text-carbon-textMuted uppercase tracking-widest">{label}</span>
      <div className="flex items-center gap-2 min-h-7">{children}</div>
    </div>
  );
}

function SummaryTier({
  t,
  lang,
  domains,
  loading,
  newestRun,
}: {
  t: ReturnType<typeof useT>["t"];
  lang: string;
  domains: DomainStatus[];
  loading: boolean;
  newestRun: Run | null;
}) {
  // Cell 1 — worst RPO status across enabled, non-off domains: any overdue/never
  // is red, else any warn is amber, else any ok is green, else all off = neutral.
  // The representative status reuses chipForRpo + the existing rpo* labels below.
  const active = domains.filter((d) => d.enabled && d.status !== "off");
  const health: "overdue" | "warn" | "ok" | "off" = active.some(
    (d) => d.status === "overdue" || d.status === "never"
  )
    ? "overdue"
    : active.some((d) => d.status === "warn")
      ? "warn"
      : active.some((d) => d.status === "ok")
        ? "ok"
        : "off";
  const healthLabel =
    health === "overdue"
      ? t("dashboard.rpoOverdue")
      : health === "warn"
        ? t("dashboard.rpoWarn")
        : health === "ok"
          ? t("dashboard.rpoOk")
          : t("dashboard.rpoOff");

  // Cell 2 — the soonest (most frequent) enabled schedule, shown as human cadence
  // text (e.g. "Daily 03:00"). NOTE: there is no next-run timestamp on the
  // backend and no client-side cron calculator, so this is deliberately NOT a
  // live countdown — just which enabled schedule fires soonest. Empty when every
  // domain is off/unscheduled, in which case we show the "not scheduled" label.
  const scheduled = domains
    .filter((d) => d.enabled)
    .map((d) => ({ raw: d.schedule, s: parseCadenceString(d.schedule) }))
    .filter((x) => x.s.mode !== "off")
    .sort(
      (a, b) =>
        cadencePeriodDays(a.s) - cadencePeriodDays(b.s) ||
        minutesOfDay(a.s.time) - minutesOfDay(b.s.time)
    );
  const nextCadence = scheduled.length > 0 ? formatCadence(scheduled[0].raw, t, lang) : "";

  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
      {/* Overall health — worst RPO status across enabled domains */}
      <SummaryCell label={t("dashboard.summaryHealth")}>
        {loading ? (
          <span className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</span>
        ) : (
          <>
            {health !== "off" && <StatusChip status={chipForRpo(health)} />}
            <span className="text-sm text-carbon-text truncate">{healthLabel}</span>
          </>
        )}
      </SummaryCell>

      {/* Next backup — soonest scheduled cadence as human text (not a countdown) */}
      <SummaryCell label={t("dashboard.summaryNextBackup")}>
        {loading ? (
          <span className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</span>
        ) : (
          <span className="text-sm text-carbon-text truncate">
            {nextCadence || t("dashboard.rpoOff")}
          </span>
        )}
      </SummaryCell>

      {/* Last result — the newest run: status chip + target + relative time */}
      <SummaryCell label={t("dashboard.summaryLastResult")}>
        {newestRun ? (
          <>
            <StatusChip status={newestRun.status} />
            <span className="text-sm text-carbon-text flex-1 truncate">
              {newestRun.target || `${newestRun.targetId.slice(0, 12)}…`}
            </span>
            <span
              className="text-xs text-carbon-textMuted shrink-0"
              title={formatTs(newestRun.startedAt)}
            >
              {relativeTime(t, newestRun.startedAt)}
            </span>
          </>
        ) : (
          <span className="text-sm text-carbon-textMuted">{t("dashboard.noRuns")}</span>
        )}
      </SummaryCell>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Dashboard page
// ---------------------------------------------------------------------------

export function Dashboard() {
  const { t, lang } = useT();
  const { advanced } = useAdvanced();

  // Single /api/status fetch shared by the Protection + Ransomware cards (no
  // duplicate round-trip — both cards read the same extended domain status).
  const [statusDomains, setStatusDomains] = useState<DomainStatus[]>([]);
  const [statusLoading, setStatusLoading] = useState(true);

  // Newest run for the summary tier's "Last result" cell. listRuns returns
  // newest-first, so runs[0] is the latest. Fetched once here (the detail-tier
  // RunsCard keeps its own fetch for the filtered list).
  const [runs, setRuns] = useState<Run[]>([]);
  useEffect(() => {
    let active = true;
    listRuns()
      .then((res) => {
        if (active && res.ok) setRuns(res.runs ?? []);
      })
      .catch(() => {/* non-fatal — summary "Last result" falls back to empty */});
    return () => {
      active = false;
    };
  }, []);

  // Page-level banners are capped at one: the Fresh-install nudge wins over the
  // Recovery-kit nag. Fresh dismissal persists in localStorage (shared key), and
  // while Fresh is showing the Recovery nag is suppressed.
  const [freshDismissed, setFreshDismissed] = useState(() => {
    try {
      return localStorage.getItem(RECOVERY_NUDGE_DISMISSED) === "1";
    } catch {
      return false;
    }
  });
  const dismissFresh = () => {
    try {
      localStorage.setItem(RECOVERY_NUDGE_DISMISSED, "1");
    } catch {
      /* storage unavailable — dismiss for this session only */
    }
    setFreshDismissed(true);
  };
  const freshShown = !statusLoading && !freshDismissed && isFreshInstall(statusDomains);

  useEffect(() => {
    let active = true;
    const load = () => {
      getStatus()
        .then((res) => {
          if (active && res.ok) setStatusDomains(res.domains ?? []);
        })
        .catch(() => {/* non-fatal */})
        .finally(() => {
          if (active) setStatusLoading(false);
        });
    };
    load();
    // Live-refresh when protection-relevant state changes elsewhere (e.g. a manual
    // restore drill on the Settings page, which dispatches this event) so the
    // scorecard pills reflect the new outcome without a page reload.
    window.addEventListener("bv:settings-changed", load);
    return () => {
      active = false;
      window.removeEventListener("bv:settings-changed", load);
    };
  }, []);

  // Customizable dashboard (#46) — everything below the heading + banners is a
  // reorderable / hideable block, persisted per-browser via useDashboardLayout.
  const [editing, setEditing] = useState(false);

  // Ordered block list. Each block has a stable id, a label, the rendered node
  // (props preserved exactly from the original render) and an advancedOnly flag.
  // advancedOnly blocks are dropped from BOTH the render and the customize list
  // when not in Advanced view — their order/hidden state still persists.
  const blocks: {
    id: string;
    label: string;
    advancedOnly?: boolean;
    node: React.ReactNode;
  }[] = [
    {
      id: "summary",
      label: t("dashboard.blockSummary"),
      node: (
        <SummaryTier
          t={t}
          lang={lang}
          domains={statusDomains}
          loading={statusLoading}
          newestRun={runs[0] ?? null}
        />
      ),
    },
    {
      id: "stats",
      label: t("dashboard.blockStats"),
      node: <StatCardsRow t={t} advanced={advanced} />,
    },
    {
      id: "protection",
      label: t("dashboard.protectionTitle"),
      node: <ProtectionCard t={t} domains={statusDomains} loading={statusLoading} />,
    },
    {
      id: "ransomware",
      label: t("ransomware.title"),
      advancedOnly: true,
      node: <RansomwareCard t={t} domains={statusDomains} loading={statusLoading} />,
    },
    // Last Backups and Run History are separate blocks (#50 follow-up) so each
    // can be hidden, reordered and read at full width independently.
    {
      id: "lastBackups",
      label: t("dashboard.lastBackups"),
      node: <LastBackupsCard t={t} />,
    },
    {
      id: "runHistory",
      label: t("run.historyTitle"),
      node: <RunsCard t={t} />,
    },
    {
      id: "heatmap",
      label: t("dashboard.healthTitle"),
      node: <HealthHeatmapCard t={t} />,
    },
    {
      id: "storage",
      label: t("dashboard.storageTitle"),
      node: <StorageCard t={t} />,
    },
    {
      id: "spike",
      label: t("spike.title"),
      advancedOnly: true,
      node: <SpikeCard t={t} />,
    },
  ];

  const defaultOrder = blocks.map((b) => b.id);
  const { order, hidden, reorder, toggleHidden, reset } =
    useDashboardLayout(defaultOrder);

  // Persisted order → concrete blocks. Unknown/stale ids are guarded out, and
  // advancedOnly blocks are dropped while not in Advanced view.
  const byId = new Map(blocks.map((b) => [b.id, b]));
  const orderedAvailable = order
    .map((id) => byId.get(id))
    .filter(
      (b): b is (typeof blocks)[number] => !!b && (advanced || !b.advancedOnly)
    );
  const visibleBlocks = orderedAvailable.filter((b) => !hidden.has(b.id));
  const hiddenBlocks = orderedAvailable.filter((b) => hidden.has(b.id));

  // Native HTML5 drag-and-drop — the dragged id lives in a ref (no re-render
  // mid-drag); onDrop reorders relative to the drop-target block. The move
  // up/down buttons on each block are the accessible + touch fallback.
  const draggingId = useRef<string | null>(null);
  const dragHandlersFor = (blockId: string): BlockDragHandlers => ({
    onDragStart: (e) => {
      draggingId.current = blockId;
      e.dataTransfer.effectAllowed = "move";
      try {
        e.dataTransfer.setData("text/plain", blockId);
      } catch {
        /* some browsers restrict setData during dragstart — the ref suffices */
      }
    },
    onDragOver: (e) => {
      e.preventDefault();
      e.dataTransfer.dropEffect = "move";
    },
    onDrop: (e) => {
      e.preventDefault();
      let dragged = draggingId.current;
      if (!dragged) {
        try {
          dragged = e.dataTransfer.getData("text/plain") || null;
        } catch {
          dragged = null;
        }
      }
      if (dragged && dragged !== blockId) reorder(dragged, blockId);
      draggingId.current = null;
    },
    onDragEnd: () => {
      draggingId.current = null;
    },
  });

  return (
    <div className="flex flex-col gap-6 max-w-5xl">
      {/* Page heading — fixed (contextual, not customizable). The pencil in the
          top-right corner toggles the customize/edit mode. */}
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-carbon-text">
            {t("dashboard.title")}
          </h1>
          <p className="mt-1 text-sm text-carbon-textSub">
            {t("dashboard.subtitle")}
          </p>
          <div className="mt-2 flex flex-col gap-1">
            <OffsiteIndicator domain="containers" withLabel />
            <OffsiteIndicator domain="vms" withLabel />
            <OffsiteIndicator domain="flash" withLabel />
            <OffsiteIndicator domain="files" withLabel />
          </div>
        </div>
        <button
          type="button"
          onClick={() => setEditing((v) => !v)}
          aria-label={editing ? t("dashboard.customizeDone") : t("dashboard.customize")}
          aria-pressed={editing}
          title={editing ? t("dashboard.customizeDone") : t("dashboard.customize")}
          className={`shrink-0 rounded-md p-2 motion-safe:transition-colors ${
            editing
              ? "bg-accent text-accentContrast"
              : "border border-carbon-border text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
          }`}
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <path
              d="M4 20h4L18.5 9.5a2.121 2.121 0 0 0-3-3L5 17v3z"
              stroke="currentColor"
              strokeWidth="1.6"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
            <path
              d="M13.5 6.5l3 3"
              stroke="currentColor"
              strokeWidth="1.6"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
        </button>
      </div>

      {/* Fresh/rebuilt install nudge to the guided Recovery tab — fixed
          (contextual). Reuses the shared /api/status fetch below. */}
      <FreshInstallNudge
        t={t}
        domains={statusDomains}
        loading={statusLoading}
        dismissed={freshDismissed}
        onDismiss={dismissFresh}
      />

      {/* Recovery-kit nag — fixed (contextual): only while encryption is on and
          the recovery kit is unstored. */}
      <RecoveryNag t={t} suppressed={freshShown} />

      {/* Customize controls — the pencil in the heading toggles edit mode; while
          editing, the Reset button + hint appear here. */}
      {editing && (
        <div className="flex flex-col gap-2">
          <button
            type="button"
            onClick={reset}
            className="self-start rounded-md border border-carbon-border px-3 py-1.5 text-sm text-carbon-textSub hover:text-carbon-text motion-safe:transition-colors"
          >
            {t("dashboard.resetLayout")}
          </button>
          <p className="text-xs text-carbon-textMuted">{t("dashboard.customizeHint")}</p>
        </div>
      )}

      {/* Ordered, visible blocks. In edit mode each carries a control bar +
          native drag-and-drop; otherwise the card renders plainly. */}
      <div className="flex flex-col gap-6">
        {visibleBlocks.map((b, i) => (
          <CustomizableBlock
            key={b.id}
            id={b.id}
            label={b.label}
            index={i}
            total={visibleBlocks.length}
            isFirst={i === 0}
            isLast={i === visibleBlocks.length - 1}
            editing={editing}
            dragHandlers={dragHandlersFor(b.id)}
            /* Move relative to the VISIBLE neighbour (skips hidden / advanced-gated
               blocks in the stored order) so a single press always reorders. */
            onMoveUp={() => {
              if (i > 0) reorder(b.id, visibleBlocks[i - 1].id);
            }}
            onMoveDown={() => {
              if (i < visibleBlocks.length - 1)
                reorder(b.id, visibleBlocks[i + 1].id);
            }}
            onHide={() => toggleHidden(b.id)}
            t={t}
          >
            {b.node}
          </CustomizableBlock>
        ))}
      </div>

      {/* Hidden-cards tray — only while editing and something is hidden. */}
      {editing && hiddenBlocks.length > 0 && (
        <div className="flex flex-col gap-3 rounded-card border border-dashed border-carbon-border p-4">
          <h2 className="text-sm font-semibold uppercase tracking-widest text-carbon-textSub">
            {t("dashboard.hiddenCards")}
          </h2>
          <div className="flex flex-wrap gap-2">
            {hiddenBlocks.map((b) => (
              <div
                key={b.id}
                className="flex items-center gap-2 rounded-md border border-carbon-border bg-carbon-surface2 px-2.5 py-1.5"
              >
                <span className="max-w-48 truncate text-xs text-carbon-textSub">
                  {b.label}
                </span>
                <button
                  type="button"
                  onClick={() => toggleHidden(b.id)}
                  aria-label={`${t("dashboard.showCard")} ${b.label}`}
                  className="rounded-sm px-2 py-0.5 text-xs text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text motion-safe:transition-colors"
                >
                  {t("dashboard.showCard")}
                </button>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
