import { useCallback, useEffect, useRef, useState } from "react";
import type { CSSProperties } from "react";
import { Link } from "react-router-dom";
import { hueVars, rainbowAt } from "../lib/appearance";
import { listRuns, getSpike, listContainers, listVMs, getSettings, getStatus, getHistory, getStats, downloadRecoveryKit, ackRecoveryKit, runDrill } from "../lib/api";
import type { Run, SpikeCheck, Container, Settings, DomainStatus, HistoryDay, DayStat, RepoStat, StorageForecast } from "../lib/api";
import { ErrorDetailPanel } from "../components/ErrorDetailPanel";
import { useT } from "../lib/i18n";
import { PAGE_SHELL } from "../lib/pageShell";
import { useAdvanced } from "../lib/advanced";
import { OffsiteIndicator } from "../components/OffsiteIndicator";
import { formatCadence, parseCadenceString } from "../components/CadenceBuilder";
import type { CadenceState } from "../components/CadenceBuilder";
import { cronPeriodSeconds } from "../lib/cron";
import { relativeTime, formatTs, formatDuration } from "../lib/reltime";
import { isFreshInstall } from "../lib/freshInstall";
import { useDashboardLayout, CustomizableBlock, type BlockDragHandlers } from "../lib/dashboardLayout";
import { ActivityLog } from "../components/ActivityLog";
import { Badge, type BadgeTone } from "../components/Badge";
import { IconPencil } from "../components/Sidebar";
import { IconTipButton } from "../components/IconTipButton";
import { Selector } from "../components/Selector";
// humanBytes (binary 1024 units, one decimal) moved to lib/forecast so the
// storage forecast line shares the exact formatter of the size column.
import { buildForecastLine, humanBytes, type ResolveForecast } from "../lib/forecast";
import type { TranslationKey } from "../lib/i18n";

// Same cadence as ActivityLog's own runs polling (web/src/components/ActivityLog.tsx)
// so the summary tier's "Last result" cell and the Activity Log never disagree
// about which domain is currently running.
const SUMMARY_RUNS_POLL_MS = 10000;

// ---------------------------------------------------------------------------
// Run kind/target label helpers — shared by every dashboard card that renders
// a Run's kind and target (RunsCard, SummaryTier's "Last result" cell). A
// prune/verify run's targetId IS the domain literal it ran against
// ("containers"/"vms"/"files", or store.FlashTargetID/ConfigTargetID —
// "flash"/"config" — see internal/api/service.go domainRunTargetID), never a
// resolvable item id, so it needs its own kind label + domain-name
// resolution instead of falling through to the generic backup/restore/update
// display (which would otherwise show it mislabeled as "Restore" with a
// blank/truncated target — #run-activity-log finding 1).
// ---------------------------------------------------------------------------

function runDomainLabel(t: ReturnType<typeof useT>["t"], domain: string): string {
  switch (domain) {
    case "containers":
      return t("activityLog.domainContainers");
    case "vms":
      return t("activityLog.domainVMs");
    case "flash":
      return t("activityLog.domainFlash");
    case "config":
      return t("activityLog.domainConfig");
    case "files":
      return t("activityLog.domainFiles");
    default:
      return domain;
  }
}

function runKindLabel(t: ReturnType<typeof useT>["t"], kind: string): string {
  switch (kind) {
    case "backup":
      return t("run.kindBackup");
    case "restore":
      return t("run.kindRestore");
    case "update":
      return t("run.kindUpdate");
    case "prune":
      return t("activityLog.typePrune");
    case "verify":
      return t("activityLog.typeVerify");
    case "offsite":
      return t("activityLog.typeOffsite");
    case "drill":
      return t("activityLog.jobDrill");
    case "drdrill":
      return t("run.kindDRDrill");
    case "tamper":
      return t("activityLog.jobTamper");
    case "export":
      return t("run.kindExport");
    default:
      // An unknown future kind shows its raw literal rather than a wrong label.
      return kind;
  }
}

// isDomainOpRunKind mirrors the backend's domainRunTargetID users: these kinds
// carry the DOMAIN literal (or the flash/config singleton id) in targetId, never
// a resolvable item id.
function isDomainOpRunKind(kind: string): boolean {
  return kind === "prune" || kind === "verify" || kind === "offsite" || kind === "drill" || kind === "drdrill" || kind === "tamper" || kind === "export";
}

// runTargetText resolves what to show in a run's "target" column. Domain-op
// runs (prune/verify/offsite/drill/tamper/export) carry the domain literal in
// targetId (see above) — reuse the same domain-name keys the activity log uses
// instead of the generic target/targetId fallback, which would show the raw
// literal (e.g. "containers…") since it is never in the backend's
// target-name map.
function runTargetText(t: ReturnType<typeof useT>["t"], run: Run): string {
  if (isDomainOpRunKind(run.kind)) {
    return runDomainLabel(t, run.targetId);
  }
  return run.target || `${run.targetId.slice(0, 12)}…`;
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
  onClick,
}: {
  label: string;
  value: number;
  danger?: boolean;
  /** When set, the card becomes a real button (pointer cursor, hover, focus
   *  ring) — used to open the error-detail panel from the errors tile. */
  onClick?: () => void;
}) {
  const base = "bg-carbon-surface rounded-card px-4 py-3 flex flex-col gap-1 min-w-0 overflow-hidden";
  const inner = (
    <>
      <span
        className={`text-2xl font-bold tabular-nums ${
          danger && value > 0 ? "text-statusFail" : "text-carbon-text"
        }`}
      >
        {value}
      </span>
      <span className="text-xs text-carbon-textMuted wrap-break-word leading-tight">{label}</span>
    </>
  );
  if (onClick) {
    return (
      <button
        type="button"
        onClick={onClick}
        className={`${base} text-start cursor-pointer hover:bg-carbon-hover motion-safe:transition-colors focus:outline-solid focus:outline-2 focus:outline-(--focus-ring)`}
      >
        {inner}
      </button>
    );
  }
  return <div className={base}>{inner}</div>;
}

// computeStatData fetches the four inputs the stat cards need and derives the
// tile values. Extracted from the component so it can be re-run on demand (after
// the error panel acknowledges failures) as well as on mount. Rejects if any
// fetch rejects — the caller then leaves the last known data in place.
async function computeStatData(): Promise<StatData> {
  const [contRes, settingsRes, runsRes, vmsRes] = await Promise.all([
    listContainers(),
    getSettings(),
    listRuns(),
    listVMs(),
  ]);
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
  const activeJobs = schedEnabled ? installed.filter((c) => c.includeInSchedule).length : 0;
  const pausedJobs = !schedEnabled ? installed.filter((c) => c.includeInSchedule).length : 0;
  // Scoped to backup/restore/update kinds — a failed prune/verify
  // (maintenance) run is surfaced in the Activity Log, not here, so this
  // badge keeps its original "backup/restore failures" meaning (#3).
  //
  // Reflects the LAST completed run per item (#100), not a cumulative
  // count of every failure ever recorded — a target that has since
  // backed up (or restored/updated) successfully must drop out. `runs`
  // arrives newest-first, so the first non-"running" run seen per
  // targetId is that item's latest completed outcome; "running" runs
  // are skipped so an in-flight retry doesn't hide the prior result.
  // Acknowledged failures (#126) are skipped too, so resolving an error in the
  // detail panel drops the target out of the badge just like a later success.
  const latestCompletedByTarget = new Map<string, (typeof runs)[number]>();
  for (const r of runs) {
    if (r.kind !== "backup" && r.kind !== "restore" && r.kind !== "update") continue;
    if (r.status === "running") continue;
    if (r.acknowledged) continue;
    if (!latestCompletedByTarget.has(r.targetId)) {
      latestCompletedByTarget.set(r.targetId, r);
    }
  }
  const errors = Array.from(latestCompletedByTarget.values()).filter((r) => r.status === "failed").length;

  return {
    containers: installed.length,
    vms: vmsInstalled.length,
    activeJobs,
    pausedJobs,
    errors,
    missingContainers: notInstalled.length,
    missingVMs: vmsMissing.length,
  };
}

function StatCardsRow({ t, advanced }: { t: ReturnType<typeof useT>["t"]; advanced: boolean }) {
  const [data, setData] = useState<StatData | null>(null);
  const [errorPanelOpen, setErrorPanelOpen] = useState(false);

  useEffect(() => {
    let active = true;
    computeStatData()
      .then((d) => {
        if (active) setData(d);
      })
      .catch(() => {
        // Non-fatal: stat cards stay null (not rendered)
      });
    return () => {
      active = false;
    };
  }, []);

  // Re-run after the error panel acknowledges failures so the errors tile
  // reflects the new count without a page reload.
  const refresh = useCallback(() => {
    computeStatData()
      .then((d) => setData(d))
      .catch(() => {
        /* non-fatal — keep the current tile values */
      });
  }, []);

  if (!data) return null;

  return (
    <>
      <div className={`grid grid-cols-2 gap-3 ${advanced ? "sm:grid-cols-4 lg:grid-cols-7" : "sm:grid-cols-3"}`}>
        <StatCard label={t("dashboard.statContainers")} value={data.containers} />
        <StatCard label={t("dashboard.statVMs")} value={data.vms} />
        {advanced && (
          <>
            <StatCard label={t("dashboard.statActiveJobs")} value={data.activeJobs} />
            <StatCard label={t("dashboard.statPausedJobs")} value={data.pausedJobs} />
          </>
        )}
        {/* Clickable only when there are errors to show — opens the detail panel. */}
        <StatCard
          label={t("dashboard.statErrors")}
          value={data.errors}
          danger
          onClick={data.errors > 0 ? () => setErrorPanelOpen(true) : undefined}
        />
        {advanced && (
          <>
            <StatCard label={t("dashboard.statMissingContainers")} value={data.missingContainers} danger />
            <StatCard label={t("dashboard.statMissingVMs")} value={data.missingVMs} />
          </>
        )}
      </div>
      {errorPanelOpen && (
        <ErrorDetailPanel onClose={() => setErrorPanelOpen(false)} onChanged={refresh} />
      )}
    </>
  );
}

// ---------------------------------------------------------------------------
// Status chip — statusTone maps a raw status string to the shared Badge's
// tone; statusLabel (defined right after it) maps that same string to
// translated, badge-length text. Every call site renders both together —
// `<Badge tone={statusTone(s)}>{statusLabel(s, t)}</Badge>` — instead of the
// tone alone.
//
// Task 9 fix: until this task, every one of these Badges rendered the raw
// English status word verbatim (`{overallStatus}` / `{chipFor(c)}` /
// `{chipForRpo(...)}` / `{protectionChip(...)}` / `{run.status}` /
// `{newestRun.status}`) — the exact untranslated-badge-text bug class Phase 1
// Task 5 already fixed for SpikePanel.tsx's OK/FAIL/INFO chips, sitting the
// whole time right next to an already-translated sibling label
// (overallLabel/rpoLabel/protLabel/healthLabel) that made the raw word's
// presence obvious on close reading. A prior version of this exact comment
// claimed the opposite — that showing the raw word was deliberate because
// "these are backend-sourced run-status words, not prose to translate" — but
// that rationale doesn't hold: `run.statusRunning`/`Success`/`Failed` already
// existed as real, fully-translated keys in all 26 locales (added for this
// exact fix, then never wired up), and `chipFor`/`chipForRpo`/`protectionChip`
// below don't return backend text at all — they're this file's own derived
// vocabulary. Task 7 (fifth hue) only ever touched statusTone's tone mapping,
// never what the Badge's children rendered, so the bug (and the incorrect
// comment defending it) survived that task untouched.
//
// KNOWN LIMITATION, documented on purpose (spec-compliance review of Task 7,
// not fixed in that task — see index.css's matching comment on
// --status-warn-text's dark value for the full writeup): tone="warn" and
// tone="active" render as near-identical amber in BOTH themes — dark with
// the DEFAULT accent (#f1c21b vs #FCC419, RGB-distance ~11, 1.05:1 between
// them), light on EVERY accent (#8e6a00 vs a static #7a5c00, hue 44.8° vs
// 45.2°, 1.25:1, with chip backgrounds only ~10 apart).
// SummaryTier below is a real, live site where both can appear in the same
// row at once — the "Overall health" cell showing tone="warn" (an RPO
// lapsing) next to "Last result" showing tone="active" (a run literally
// running). Not a bare SC 1.4.1 violation (each badge's own text still
// differs), but a real glance-level regression.
// Only DARK theme has a mitigation: 4 of Settings.tsx's 5 ACCENT_PRESETS
// aren't gold/yellow and don't collide, and dark's --accent-text follows
// whichever the user picked. Light's --accent-text is a static #7a5c00 that
// does NOT follow the accent, so light-theme users see this on every preset.
// Left unresolved rather than force a disproportionate fix (recolouring warn off Carbon's
// actual yellow token, changing the app's default accent, or adding a new
// icon system to Badge all reach well past a contrast-arithmetic bugfix) —
// flagged for a future task. Task 8 (focus system) was checked against this,
// since it also works the hue-vs-accent boundary via [data-rainbow]
// .glim-hue's --item-hue-ring — no shared fix: that mechanism only ever
// touches outline colour on :focus-visible, never badge fill/text colour,
// so it doesn't reach statusTone's tone="warn"/tone="active" at all. Still
// open for whichever task picks it up next.
// ---------------------------------------------------------------------------

function statusTone(status: string): BadgeTone {
  switch (status.toLowerCase()) {
    case "success":
    case "ok":
      return "ok";
    case "failed":
    case "degraded":
      return "fail";
    // Genuine activity (Task 7: resolve the fifth hue) — a run that is
    // literally in progress right now. "active" is the accent-soft Badge
    // tone, not a solid fill: this list can show several running rows at
    // once (independent domains backing up concurrently), and rule 3's
    // "at most one solid accent" cap doesn't apply to a soft/tinted chip
    // reading at the same weight as its ok/fail/warn siblings.
    case "running":
    case "checking":
      return "active";
    // The literal backend/derived string "info" (chipForRpo's "warn" SLA
    // lapse, protectionChip's "amber" aggregate, chipFor's best-effort
    // check failure) always meant a real caution, never activity — routes
    // to warn, matching SpikePanel.tsx's own hard-coded tone="warn" for the
    // identical best-effort-fail case. Unaffected by the "active" rename
    // above; this is a separate switch arm.
    case "info":
      return "warn";
    // A skip is neither success nor failure: a muted, neutral chip so a removed
    // container's scheduled target reads as "intentionally not run", distinct
    // from green success and red failure (#57).
    case "skipped":
      return "neutral";
    default:
      return "neutral";
  }
}

// statusLabel is statusTone's translation-side twin (Task 9). It keys off
// the SAME tone bucket statusTone already computes — not the raw string a
// second time — so every raw word any helper below can produce (chipFor:
// ok/info/failed — chipForRpo: success/info/failed/neutral — protectionChip:
// ok/info/failed/neutral — run.status/newestRun.status: success/failed/
// running/skipped) resolves to translated, badge-length text without a
// second switch to keep in sync. Reuses spike.ok/spike.fail/spike.info —
// SpikePanel.tsx's own OK/FAIL/INFO wording (Phase 1 Task 5) — for the
// ok/fail/warn buckets, and run.statusRunning for active (added alongside
// statusSuccess/statusFailed back when this bug was first anticipated, then
// never wired up until now). neutral/default gets the one genuinely new key
// this task adds, run.statusSkipped, translated into all 26 locales.
function statusLabel(status: string, t: ReturnType<typeof useT>["t"]): string {
  // Checked BEFORE the tone lookup, because "cancelled" and "skipped" share the
  // neutral tone and the tone is all the switch below can see. A restore the
  // user cancelled therefore read "Skipped" in Last Result and in the run
  // history — a different claim about a different event, and the one the user
  // themselves had just caused.
  if (status === "cancelled") return t("run.statusCancelled");
  switch (statusTone(status)) {
    case "ok":
      return t("spike.ok");
    case "fail":
      return t("spike.fail");
    case "warn":
      return t("spike.info");
    case "active":
      return t("run.statusRunning");
    default:
      return t("run.statusSkipped");
  }
}

// ---------------------------------------------------------------------------
// Card wrapper
// ---------------------------------------------------------------------------

function Card({
  title,
  children,
  action,
  hueIndex,
}: {
  title: string;
  children: React.ReactNode;
  action?: React.ReactNode;
  /** Rainbow position for THIS Card's own heading notch — GlimStone
   *  follow-up pass, jdp's live review of this page specifically: "Dashboard:
   *  Cardtitelbadges sind falsch platziert. Alle sind nicht im
   *  Regenbogenmodus." Every one of this page's Cards was still the ORIGINAL
   *  Task 5 flat-accent-only heading, never migrated to the `hueIndex` opt-in
   *  Settings.tsx's own Card already got — same prop, same Badge.tsx
   *  mechanism, just never threaded through on this file. Assigned by the
   *  caller's own running `nextHue()` counter, in the CURRENT rendered order
   *  of the user's customizable/reorderable block layout (see the main
   *  component's own `hueSeq`/`nextHue` comment for why that has to be a
   *  counter passed through the block-render callbacks rather than a static
   *  per-Card literal, the way Settings.tsx's own fixed tab layout can get
   *  away with). Omit for a genuine singleton — same rule as Settings.tsx's
   *  Card. */
  hueIndex?: number;
}) {
  return (
    // GlimStone follow-up pass (live-review round, "half-overlap card
    // notch"): the heading Badge is now `position: absolute`, straddling
    // THIS card's own top edge (see Badge.tsx's badgeClassName comment) —
    // it needs a `relative` ancestor whose edge IS the card's real visual
    // edge, which the bg-carbon-surface box below can no longer provide on
    // its own: that box's own `overflow-hidden` (there so a Card containing
    // a ProgressBar clips the bar's square ends to the card's rounded
    // corners — see ProgressBar.tsx) would just as happily clip the badge's
    // own -11px poke above it. So `relative` moves to this new, purely
    // structural OUTER div (no bg/radius/shadow of its own — all of that
    // stays on the inner div below), with the badge rendering as its direct
    // child: it escapes the inner div's clipping entirely while still
    // measuring its offset against a box whose top edge is pixel-identical
    // to the visual card's own (this outer div has no padding/border, so it
    // hugs the inner div exactly).
    // `glim-notch-card` (same live-review round's rainbow-hue follow-up as
    // Settings.tsx's own Card — index.css's `[data-rainbow="reactive"]
    // .glim-notch-card:hover .glim-notch-hue` rule keys off this marker):
    // this page's Card never carried it before now — a genuine instance of
    // the SAME gap jdp is naming here, not a new one invented for this
    // fix — a hued heading on this page would have lit up on hovering only
    // its own ~22px glyph, not this card's whole body, in reactive mode.
    //
    // insetStart={5} (GlimStone follow-up pass, jdp emphatic: "Dashboard-
    // Badges sind immer noch falsch platziert, links buendig mit der Card")
    // — the outer div above is deliberately unpadded (that's the whole point
    // of this split, see above), which left the badge's horizontal position
    // — the CSS static-position fallback Badge.tsx uses by default — flush
    // with THIS outer div's bare edge instead of the inner p-5 box's content
    // edge below. `insetStart={5}` states the inner box's own p-5 explicitly
    // on the Badge instead of leaving it to be re-derived from ambient DOM
    // shape — see Badge.tsx's own `insetStart` doc for the full mechanism
    // and the four OTHER call sites (SummaryCell below, ActivityLog.tsx,
    // Flash.tsx's and Config.tsx's backup Cards) that independently hit the
    // identical mismatch.
    //
    // `.glim-hue` ALSO added (rainbow-mode completeness sweep, jdp live
    // review: "Es sind nicht alle Buttons in den Regenbogen-Modus
    // eingepflegt"): `glim-notch-card` alone never redefines
    // --accent/--focus-ring, only the reactive-mode hover reveal, so every
    // action button/control rendered as this Card's `children` (e.g.
    // RansomwareCard's "Run off-site DR check" button) stayed the flat theme
    // accent regardless of rainbow, even though this SAME Card's own
    // heading notch was already correctly hued. Same hueIndex prop the
    // Badge already uses — custom properties cascade to every descendant
    // once redefined here, no per-button change needed at any of this
    // Card's many call sites.
    <div
      className={`relative glim-notch-card${hueIndex !== undefined ? " glim-hue" : ""}`}
      style={hueIndex !== undefined ? (hueVars(rainbowAt(hueIndex)) as CSSProperties) : undefined}
    >
      <h2 className="flex items-center">
        <Badge tone="heading" size="heading" wrap hueIndex={hueIndex} insetStart={5}>{title}</Badge>
      </h2>
      <div className="bg-carbon-surface rounded-card p-5 flex flex-col gap-4 overflow-hidden">
        {/* action used to share a `justify-between` row with the <h2> above,
            pinned to the row's far end opposite the title. Now that the
            title lives outside this div entirely, `justify-end` replaces
            `justify-between` (which needs 2+ items to do anything — with
            only `action` left in the row, `justify-between` would dock it
            to the START instead of the end it always visually occupied). */}
        {action && <div className="flex justify-end">{action}</div>}
        {children}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Spike status card
// ---------------------------------------------------------------------------

function SpikeCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
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
        setError(err instanceof Error ? err.message : t("common.checkFailed"));
        setAllOk(false);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, []); // eslint-disable-line react-hooks/exhaustive-deps -- t() is only read to build a failure message; re-fetching on a language switch would be a wasted round-trip

  const hasRun = !loading && allOk !== null;
  const overallStatus = allOk ? "ok" : "degraded";
  const overallLabel = allOk ? t("dashboard.allOk") : t("dashboard.degraded");

  // A best-effort (optional) check that fails is informational, not a failure.
  const chipFor = (c: SpikeCheck) => (c.OK ? "ok" : c.BestEffort ? "info" : "failed");

  return (
    <Card title={t("spike.title")} hueIndex={hueIndex}>
      {loading && (
        <p className="text-xs text-carbon-textMuted">{t("dashboard.checking")}</p>
      )}

      {hasRun && (
        <div className="flex items-center gap-2">
          <span className="text-xs text-carbon-textMuted">{t("spike.overall")}</span>
          <Badge tone={statusTone(overallStatus)}>{statusLabel(overallStatus, t)}</Badge>
          <span className="text-sm text-carbon-text">{overallLabel}</span>
        </div>
      )}

      {error && (
        <p className="text-xs text-statusFail">{error}</p>
      )}

      {checks && checks.length > 0 && (
        <div className="divide-y divide-carbon-border">
          {checks.map((c) => (
            <div key={c.Name} className="flex items-center gap-3 py-2 text-sm">
              <Badge tone={statusTone(chipFor(c))}>{statusLabel(chipFor(c), t)}</Badge>
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

// chipForRpo maps an RPO status to a statusTone/Badge color variant.
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
  hueIndex,
}: {
  t: ReturnType<typeof useT>["t"];
  domains: DomainStatus[];
  loading: boolean;
  hueIndex?: number;
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
    <Card title={t("dashboard.protectionTitle")} hueIndex={hueIndex}>
      {loading && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
      )}
      {!loading && domains.length > 0 && (
        <div className="@container divide-y divide-carbon-border bv-content-fade">
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
                {/* Two-mode row layout (#66 follow-up). WIDE card (container query
                    @[44rem] on the rows wrapper): every row lays its cells on the
                    SAME shared grid track template so the same kind of info sits in
                    the same column down the whole card, regardless of which cells a
                    row populates: [domain] [status] [schedule] [last run] [verified]
                    [off-site verified] [off-site DR]. Fixed tracks for the
                    fixed-width columns, fr tracks for the text/badges so a long
                    badge (e.g. "proven restorable from off-site") wraps inside its
                    column instead of overflowing the card, and absent badges just
                    leave their column blank without re-flowing the others; the
                    three badge columns get a readable floor width (not minmax(0,…))
                    so they wrap at word boundaries instead of being squeezed thin
                    enough to hyphenate mid-word. NARROW card: the row falls back to
                    a wrapped flex stack — the domain name reads as a full-width
                    heading (basis-full) and the remaining cells flow underneath —
                    so a half-width card never grows per-row horizontal
                    scrollbars. */}
                <div className="flex flex-wrap items-center gap-x-3 gap-y-1 py-0.5 @[44rem]:grid @[44rem]:gap-3 @[44rem]:grid-cols-[7rem_minmax(0,1fr)_minmax(0,1fr)_5rem_minmax(6.5rem,1fr)_minmax(7.5rem,1.2fr)_minmax(7.5rem,1.6fr)]">
                  <span
                    className={`col-start-1 basis-full @[44rem]:basis-auto min-w-0 truncate font-medium ${
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
                        <span className="shrink-0">
                          <Badge tone={statusTone(chipForRpo(d.status))}>{statusLabel(chipForRpo(d.status), t)}</Badge>
                        </span>
                        <span className="min-w-0 truncate text-carbon-text">
                          {rpoLabel(d.status)}
                        </span>
                      </div>
                      {/* Col 3 — schedule cadence. A domain with no cadence of
                          its own can still be covered by the whole-server
                          "Backup Everything" pass, and then it is the pass's
                          cadence that applies — naming it keeps the row from
                          contradicting the domain's own card, which correctly
                          shows no schedule (#177). */}
                      <span className="col-start-3 min-w-0 truncate text-carbon-textMuted text-xs">
                        {d.coveredBy
                          ? t("dashboard.rpoViaEverything").replace("{cadence}", formatCadence(d.coveredBy, t, lang))
                          : formatCadence(d.schedule, t, lang)}
                      </span>
                      {/* Col 4 — last successful run. */}
                      <span
                        className="col-start-4 text-start @[44rem]:text-end text-carbon-textMuted text-xs"
                        title={formatTs(d.lastSuccess)}
                      >
                        {d.lastSuccess ? relativeTime(t, d.lastSuccess) : t("containers.never")}
                      </span>
                      {/* Col 5 — local-verify shield badge. */}
                      {d.lastVerified ? (
                        <div className="col-start-5 min-w-0">
                          <Badge
                            tone={d.lastVerifiedOK ? "ok" : "fail"}
                            wrap
                            className="max-w-full"
                            title={`${t("verify.shield")} · ${formatTs(d.lastVerified)}`}
                          >
                            {d.lastVerifiedOK ? "✓" : "✗"} {t("verify.shield")} {relativeTime(t, d.lastVerified)}
                          </Badge>
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
                          <Badge
                            tone={d.lastOffsiteSubsetOK ? "ok" : "fail"}
                            wrap
                            className="max-w-full"
                            title={`${t("drill.offsiteVerified")} · ${formatTs(d.lastOffsiteSubsetAt)}`}
                          >
                            {d.lastOffsiteSubsetOK ? "✓" : "✗"} {t("drill.offsiteVerified")} {relativeTime(t, d.lastOffsiteSubsetAt)}
                          </Badge>
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
                          <Badge
                            tone="ok"
                            wrap
                            className="max-w-full"
                            title={`${t("drill.provenOffsite")} · ${formatTs(d.lastDrDrillAt)}`}
                          >
                            ✓ {t("drill.provenOffsite")} · {relativeTime(t, d.lastDrDrillAt)}
                          </Badge>
                        ) : drFailed ? (
                          // RED — a recorded off-site DR drill FAILED (scheduled or a
                          // manual run). Always shown; the opt-out never masks a real
                          // failure — only "never drilled" goes neutral below.
                          <Badge
                            tone="fail"
                            wrap
                            className="max-w-full"
                            title={
                              d.drillDetail
                                ? `${t("drill.checkOffsiteDr")} · ${t("drill.failReasonPrefix")} ${d.drillDetail} · ${formatTs(d.lastDrDrillAt)}`
                                : `${t("drill.provenOffsite")} · ${formatTs(d.lastDrDrillAt)}`
                            }
                          >
                            ✗ {t("drill.provenOffsite")} · {relativeTime(t, d.lastDrDrillAt)}
                          </Badge>
                        ) : drUnscheduled ? (
                          // NEUTRAL — off-site DR not scheduled (manual only) and nothing
                          // failing to show: muted, never red. File's no-claim styling.
                          <Badge tone="neutral" wrap className="max-w-full" title={t("drill.manualOnlyTitle")}>
                            {t("drill.manualOnly")}
                          </Badge>
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
                  <div className="flex flex-wrap items-center gap-2 ps-1">
                    {drFailed && d.drillDetail && (
                      <span className="text-xs text-statusFail wrap-break-word" title={d.drillDetail}>
                        {t("drill.checkOffsiteDr")} · {t("drill.failReasonPrefix")} {d.drillDetail}
                      </span>
                    )}
                    {d.offsiteConfigured && (
                      <button
                        type="button"
                        onClick={() => runOffsiteDr(d.domain)}
                        disabled={drRunning === d.domain}
                        className="rounded-control bg-carbon-surface2 px-2 py-1 text-xs text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
                      >
                        {drRunning === d.domain
                          ? t("drill.runningOffsiteDr")
                          : d.lastDrDrillAt && d.lastDrDrillOK
                            ? t("drill.rerunOffsiteDr")
                            : t("drill.runOffsiteDr")}
                      </button>
                    )}
                    {drRunError[d.domain] && (
                      <span className="text-xs text-statusFail wrap-break-word">✗ {drRunError[d.domain]}</span>
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

// protectionChip maps the red/amber/green aggregate to a statusTone/Badge variant.
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
  hueIndex,
}: {
  t: ReturnType<typeof useT>["t"];
  domains: DomainStatus[];
  loading: boolean;
  hueIndex?: number;
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
    <Card title={t("ransomware.title")} hueIndex={hueIndex}>
      {loading && <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>}
      {!loading && (
      <div className="bv-content-fade">
      {shown.map((d) => {
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
                <Badge tone={statusTone(protectionChip(d.protection))}>{statusLabel(protectionChip(d.protection), t)}</Badge>
                <span className="text-sm text-carbon-textSub">{protLabel(d.protection)}</span>
              </div>
              <div className="flex flex-col gap-0.5 ps-1">
                {rows.map((row) => {
                  const icon =
                    row.state === "ok" ? "✓" : row.state === "amber" ? "!" : row.state === "bad" ? "✗" : "—";
                  const iconColor =
                    row.state === "ok"
                      ? "text-statusOk"
                      : row.state === "amber"
                        ? "text-statusWarn"
                        : row.state === "bad"
                          ? "text-statusFail"
                          : "text-carbon-textMuted";
                  const labelColor =
                    row.state === "amber"
                      ? "text-statusWarn"
                      : row.state === "muted"
                        ? "text-carbon-textMuted"
                        : "text-carbon-textSub";
                  return (
                    <div key={row.key} className="flex flex-col gap-0.5">
                      <div className="flex items-center gap-2 text-sm">
                        <span className={`w-4 shrink-0 text-center ${iconColor}`}>{icon}</span>
                        {row.state === "bad" ? (
                          // Task 5 (rule 13) deliberate exception, documented
                          // rather than converted: this is a status-LIST-ROW
                          // label that only sometimes (state === "bad") also
                          // navigates — its non-clickable siblings above/below
                          // render at the SAME plain text-sm size (the `else`
                          // branch right below). Forcing only the clickable
                          // state into a fixed-height Badge chip would make
                          // row height/typography jump depending on which
                          // domain is currently faulted — a worse, more
                          // visible inconsistency than the plain-link issue
                          // rule 13 targets (which is about a link sitting
                          // among ALREADY-badge-styled siblings; nothing else
                          // in this row is a badge). The semantic fault-red
                          // colour + hover underline already signals both
                          // "this is wrong" and "this is clickable" without
                          // breaking row alignment.
                          <Link to="/settings#offsite" className="text-statusFail hover:underline flex-1 truncate min-w-0">
                            {row.label}
                          </Link>
                        ) : (
                          <span className={`flex-1 truncate min-w-0 ${labelColor}`}>{row.label}</span>
                        )}
                        {row.at !== undefined && (
                          <span className="text-xs text-carbon-textMuted shrink-0">{ageText(row.at)}</span>
                        )}
                      </div>
                      {/* WHICH check + WHY it failed (off-site DR reason from /api/status). */}
                      {row.detail && (
                        <span className="text-xs text-statusFail wrap-break-word ps-6" title={row.detail}>
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
      </div>
      )}
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Recent Runs card
// ---------------------------------------------------------------------------

function RunsCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  const [runs, setRuns] = useState<Run[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [day, setDay] = useState("all");

  useEffect(() => {
    listRuns()
      .then((res) => {
        if (res.ok) setRuns(res.runs ?? []);
        else setError(t("dashboard.loadRunsFailed"));
      })
      .catch(() => setError(t("dashboard.loadRunsFailed")))
      .finally(() => setLoading(false));
  }, []); // eslint-disable-line react-hooks/exhaustive-deps -- t() is only read to build a failure message; re-fetching on a language switch would be a wasted round-trip

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
    <Card title={t("run.historyTitle")} hueIndex={hueIndex}>
      {loading && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
      )}
      {error && <p className="text-sm text-statusFail">{error}</p>}
      {!loading && !error && runs.length === 0 && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.noRuns")}</p>
      )}
      {runs.length > 0 && (
        <div className="bv-content-fade">
          {/* Day filter */}
          <div className="flex items-center gap-2 mb-2">
            <label className="text-xs text-carbon-textMuted">{t("run.filterDay")}</label>
            <select
              value={day}
              onChange={(e) => setDay(e.target.value)}
              className="rounded-control bg-carbon-surface2 px-2 py-1 text-xs text-carbon-text bv-field-focus"
            >
              <option value="all">{t("run.allDays")}</option>
              {days.map((d) => (
                <option key={d} value={d}>{d}</option>
              ))}
            </select>
          </div>
          {/* Scrollable list — all runs in the window (filtered by day) */}
          <div className="divide-y divide-carbon-border max-h-128 overflow-y-auto pe-2">
            {shown.map((run) => {
              const dur = run.finishedAt != null ? formatDuration(run.finishedAt - run.startedAt) : "";
              return (
              <div key={run.id} className="flex flex-col gap-0.5 py-2.5 text-sm">
                <div className="flex items-center gap-3">
                  <Badge tone={statusTone(run.status)}>{statusLabel(run.status, t)}</Badge>
                  <span className="text-carbon-text font-medium w-16 shrink-0 truncate">
                    {runKindLabel(t, run.kind)}
                  </span>
                  <span className="text-carbon-text flex-1 truncate min-w-0">
                    {runTargetText(t, run)}
                  </span>
                  {/* Start → end + duration, with the relative age underneath (#45/#50). */}
                  <span className="flex flex-col items-end shrink-0 text-xs leading-tight">
                    <span className="text-carbon-textSub whitespace-nowrap">
                      {formatTs(run.startedAt)}
                      {run.finishedAt != null && (
                        <>
                          {" "}
                          <span className="inline-block rtl:-scale-x-100">→</span> {formatTs(run.finishedAt)}
                        </>
                      )}
                    </span>
                    <span className="text-carbon-textMuted whitespace-nowrap">
                      {dur ? `(${dur}) · ` : ""}
                      {relativeTime(t, run.startedAt)}
                    </span>
                  </span>
                </div>
                {run.status === "failed" && run.error && (
                  <p dir="ltr" className="ps-16 text-xs text-statusFail wrap-break-word text-start">{run.error}</p>
                )}
                {run.status === "skipped" && run.error && (
                  <p dir="ltr" className="ps-16 text-xs text-carbon-textMuted wrap-break-word text-start">{run.error}</p>
                )}
              </div>
              );
            })}
          </div>
        </div>
      )}
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Last Backups card
// ---------------------------------------------------------------------------

function LastBackupsCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
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
    <Card title={t("dashboard.lastBackups")} hueIndex={hueIndex}>
      {loading && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
      )}
      {!loading && containers.length === 0 && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.noContainers")}</p>
      )}

      {withBackups.length > 0 && (
        <div className="divide-y divide-carbon-border bv-content-fade">
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
                <div className="w-2 h-2 rounded-full bg-statusOkSolid shrink-0" />
                <span className="text-carbon-text font-medium flex-1 truncate min-w-0">{c.name}</span>
                {hasStart ? (
                  <span className="text-carbon-textMuted text-xs shrink-0 text-end">
                    {formatTs(c.lastBackupStarted)} <span className="inline-block rtl:-scale-x-100">→</span> {formatTs(c.lastBackup)}
                    {duration && (
                      <span className="ms-1" title={t("dashboard.duration")} aria-label={t("dashboard.duration")}>
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
// runs; no runs → neutral carbon surface. Colors come from the theme vars in
// index.css (#105) so the scale stays legible in light mode too.
function cellColor(stat: DayStat | undefined): string {
  if (!stat || (stat.ok === 0 && stat.failed === 0)) return "var(--carbon-surface2, #262626)";
  if (stat.failed > 0) return "var(--status-fail-solid, #ff8389)";
  // All ok — deeper green for more runs that day.
  if (stat.ok >= 3) return "var(--heat-ok-3, #42be65)";
  if (stat.ok === 2) return "var(--heat-ok-2, #6fdc8c)";
  return "var(--heat-ok-1, #a7f0ba)";
}

// mondayIndex returns 0..6 for Mon..Sun (JS getDay() is 0=Sun..6=Sat).
function mondayIndex(d: Date): number {
  return (d.getDay() + 6) % 7;
}

function HealthHeatmapCard({
  t,
  selectedDay,
  onSelectDay,
  hueIndex,
}: {
  t: ReturnType<typeof useT>["t"];
  /** The Activity Log's active day filter (ISO YYYY-MM-DD, local) — the
   *  matching cell renders an accent outline; clicking it again clears. */
  selectedDay: string | null;
  /** Fired with a cell's local ISO day — the Dashboard toggles the Activity
   *  Log day filter and scrolls the log into view. Zero-run days fire too:
   *  the log then honestly shows nothing for that day. */
  onSelectDay: (isoDay: string) => void;
  hueIndex?: number;
}) {
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
    // Walk calendar days (setDate), not fixed 24h steps: a millisecond walk
    // lands on the same local date twice on the 25h DST fall-back day, which
    // would emit a duplicate cell and shift the whole grid.
    const cur = new Date(first);
    while (cur <= last) {
      const iso = cur.toLocaleDateString("en-CA"); // YYYY-MM-DD, local
      const hd = byDate.get(iso);
      cells.push({ key: iso, date: iso, stat: statFor(hd) });
      cur.setDate(cur.getDate() + 1);
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

  // REVERSED (jdp, live-review, extremely emphatic — "Es soll immer alles in
  // die Farb- und Formengine integriert werden!! IMMER!!"): this used to
  // carry `hue={false}`, justified as "a small, fixed set of 5 where each
  // entry already has its own durable identity, and a 5-way rainbow strip
  // competing with the heatmap's own fixed red/green state hues would hurt
  // legibility for no tracking benefit." That reasoning is exactly the kind
  // of self-authored aesthetic exception jdp has now ruled out categorically
  // — a plausible-sounding taste judgement is never grounds to unilaterally
  // exclude a control from the colour engine, no matter how reasonable it
  // reads in isolation. This strip is a genuine "select one of several"
  // Selector like every other hue-enabled one in this app, so it gets the
  // same default `hue` (true) as the rest — no opt-out prop at all now.
  //   KNOWN COINCIDENCE, not a reason to exclude: RAINBOW[0] (#FF8389) and
  // RAINBOW[3] (#6FDC8C) happen to match this page's own fixed --status-fail/
  // --status-ok hues in dark theme (see lib/appearance.ts's own documented
  // KNOWN LIMITATION for the full writeup) — a coincidence, not a WCAG
  // failure (every cell still carries its own count as text, not colour
  // alone), and not grounds for a fresh opt-out either.
  const toggle = (
    <Selector
      items={(["containers", "vms", "flash", "config", "files"] as HeatDomain[]).map((d) => ({
        id: d,
        label: domainLabel(d),
      }))}
      label={t("dashboard.healthTitle")}
      select="one"
      active={domain}
      onChange={(id) => setDomain(id as HeatDomain)}
      size="sm"
      plain
    />
  );

  return (
    <Card title={t("dashboard.healthTitle")} action={toggle} hueIndex={hueIndex}>
      {loading && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
      )}
      {!loading && days.length > 0 && (
        <div className="flex flex-col gap-2 bv-content-fade">
          <div className="flex gap-1 overflow-x-auto">
            {weeks.map((week, wi) => (
              <div key={wi} className="flex flex-col gap-1">
                {week.map((cell) => {
                  if (!cell.date) {
                    return <div key={cell.key} className="w-[11px] h-[11px]" />;
                  }
                  const date = cell.date;
                  const stat = cell.stat ?? { ok: 0, failed: 0 };
                  const active = selectedDay === date;
                  // A real <button> for free keyboard operability (Enter/
                  // Space) — Tailwind's preflight strips the browser button
                  // chrome, so only the cell's own size/fill classes remain.
                  // The tooltip stays the plain "<date>: N ok, N failed" data
                  // line (it doubles as the accessible name).
                  //
                  // bv-convention-exception: control-reads-engine-tokens --
                  // an 11px heat-map cell, not a control with chrome. The
                  // shape engine's control radius is 10px in `round`, which
                  // on an 11px box is a disc, and the grid stops reading as
                  // a grid. `rounded-xs` (2px) is also exactly what the four
                  // legend swatches below already use, so the cells and
                  // their legend stay one shape instead of drifting apart.
                  return (
                    <button
                      key={cell.key}
                      type="button"
                      onClick={() => onSelectDay(date)}
                      aria-pressed={active}
                      className={`w-[11px] h-[11px] rounded-xs cursor-pointer focus:outline-solid focus:outline-2 focus:outline-(--focus-ring) ${
                        active ? "outline-solid outline-2 outline-accent" : ""
                      }`}
                      style={{ backgroundColor: cellColor(cell.stat) }}
                      title={`${date}: ${stat.ok} ok, ${stat.failed} failed`}
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
            <span className="w-[11px] h-[11px] rounded-xs" style={{ backgroundColor: "var(--heat-ok-1, #a7f0ba)" }} />
            <span className="w-[11px] h-[11px] rounded-xs" style={{ backgroundColor: "var(--heat-ok-2, #6fdc8c)" }} />
            <span className="w-[11px] h-[11px] rounded-xs" style={{ backgroundColor: "var(--heat-ok-3, #42be65)" }} />
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
    // Task 7: was text-statusInfo (the old fifth hue). glimstone/docs/
    // design-language.md's Charts section is explicit here — "one colour
    // source: the accent, never rainbow or status hues" — so this isn't a
    // semantic judgment call like the rest of Task 7, it's the spec's own
    // fixed rule for every hand-drawn chart in the app. text-accentText, not
    // the flat text-accent: a spec-compliance review measured the flat
    // accent gold at 1.61:1 against this card's light-theme background —
    // the trend line effectively disappeared, badly under the 3:1 non-text
    // minimum. See index.css's --accent-text comment for the fix.
    <span className="text-accentText shrink-0">
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
  /** Growth + time-to-full forecast riding the same /api/stats response. */
  forecast: StorageForecast | null;
}

function StorageCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  const [data, setData] = useState<DomainStats[] | null>(null);
  const [loading, setLoading] = useState(true);

  // Resolves a translation key (+ optional {placeholder} params) for the
  // forecast line — the same injected seam ActivityLog uses, so
  // buildForecastLine stays pure and testable without an I18nProvider.
  const resolveForecast: ResolveForecast = (key, params) => {
    let s = t(key as TranslationKey);
    if (params) {
      for (const [name, value] of Object.entries(params)) s = s.split(`{${name}}`).join(value);
    }
    return s;
  };

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
            forecast: res.ok ? (res.forecast ?? null) : null,
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
    <Card title={t("dashboard.storageTitle")} hueIndex={hueIndex}>
      {loading && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
      )}
      {!loading && !anyData && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.noStats")}</p>
      )}
      {!loading && anyData && data && (
        <div className="divide-y divide-carbon-border bv-content-fade">
          {data.map((d) => {
            const has = d.latest != null;
            const dedup =
              d.latest && d.latest.restoreSize > 0 && d.latest.rawSize > 0
                ? `${(d.latest.restoreSize / d.latest.rawSize).toFixed(1)}x`
                : "—";
            // Compact per-domain forecast line (growth/week + time-to-full +
            // free space) from the same /api/stats response. Null when the
            // backend could determine nothing — then no line renders at all.
            const forecastLine = buildForecastLine(d.forecast, resolveForecast);
            return (
              <div key={d.domain} className="flex flex-col gap-0.5 py-2.5 min-w-0">
                <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm min-w-0">
                  <span
                    className={`font-medium w-28 shrink-0 truncate ${
                      has ? "text-carbon-text" : "text-carbon-textMuted"
                    }`}
                  >
                    {domainLabel(d.domain)}
                  </span>
                  {has && d.latest ? (
                    <>
                      <span className="text-carbon-text tabular-nums w-20 shrink-0 text-end">
                        {humanBytes(d.latest.rawSize)}
                      </span>
                      <span className="text-carbon-textMuted text-xs shrink-0 w-24 truncate">
                        {t("dashboard.dedup")} {dedup}
                      </span>
                      <span className="text-carbon-textMuted text-xs shrink-0 w-24 truncate">
                        {d.latest.snapshots} {t("dashboard.snapshotsLabel")}
                      </span>
                      <span className="ms-auto shrink-0">
                        <Sparkline values={d.stats.map((s) => s.rawSize)} />
                      </span>
                    </>
                  ) : (
                    <span className="text-xs text-carbon-textMuted flex-1">
                      {t("dashboard.noStats")}
                    </span>
                  )}
                </div>
                {forecastLine && (
                  <p className="ps-1 text-xs text-carbon-textSub wrap-break-word">
                    {forecastLine.growth}
                    {forecastLine.growth && (forecastLine.projection || forecastLine.free) ? " · " : ""}
                    {forecastLine.projection && (
                      /* Near-term projection (< 8 weeks to full) flips to the
                         existing warn text token; otherwise it stays muted. */
                      <span className={forecastLine.warn ? "text-statusWarn" : undefined}>
                        {forecastLine.projection}
                      </span>
                    )}
                    {forecastLine.projection && forecastLine.free ? " · " : ""}
                    {forecastLine.free}
                  </p>
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
  // Backend refusal text from the fetch-based kit download (null = no error).
  const [kitError, setKitError] = useState<string | null>(null);

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
    <div className="rounded-card bg-statusWarnBg px-4 py-3 flex flex-col gap-2">
      {/* Task 5 (rule 11): deliberately NOT a heading badge, and the one
          outermost <h2> on this page that isn't — see Badge.tsx's file header
          for the shared reasoning. Short version: this panel's own surface is
          already a filled status wash (bg-statusWarnBg), so a "filled" badge
          on top of it has nothing to fill against. Measured on the live page,
          badge-fill vs. this panel: accent-soft 1.06:1 light / 1.39:1 dark,
          warn-strong 1.00:1 light (the two warn-bg tokens share one value in
          light mode) / 1.11:1 dark. Either way the fill reads as invisible,
          so the badge would look like plain text wearing extra padding while
          also throwing away the text-statusWarn colour that currently carries
          the alert's meaning (8.62:1 against the panel). Rule 11's filled
          badge presumes a neutral card surface underneath; this alert isn't
          one. Revisit only if a genuine "badge on a status surface" token
          pair ever exists. */}
      <h2 className="text-sm font-semibold text-statusWarn">
        {t("recovery.nagTitle")}
      </h2>
      <p className="text-xs text-statusWarn leading-relaxed">
        {t("recovery.nagBody")}
      </p>
      <div className="flex flex-wrap items-center gap-2">
        {/* Fetch-based download (mirrors Settings): a raw <a download> would save
            the 403 refusal body as the .md file when auth is off — the backend
            fails closed for this export, so surface its message instead (#A1). */}
        <button
          type="button"
          onClick={() => void downloadRecoveryKit().then(setKitError)}
          className="rounded-control bg-carbon-surface3 hover:bg-carbon-border px-3 py-1.5 text-sm text-carbon-text transition-colors"
        >
          {t("recovery.download")}
        </button>
        {kitError && (
          <span className="text-xs text-statusFail wrap-break-word">✗ {kitError}</span>
        )}
        <button
          type="button"
          onClick={dismiss}
          disabled={dismissing}
          className="rounded-control px-3 py-1.5 text-sm text-carbon-textSub hover:text-carbon-text transition-colors disabled:opacity-50"
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
    <div className="bg-carbon-surface rounded-card p-5 flex items-center gap-4">
      <div className="flex-1 flex flex-col gap-1.5">
        <p className="text-sm text-carbon-text">{t("recovery.freshNudge")}</p>
        {/* Task 5 (rule 13): was a plain underline-on-hover text link, styled
            with the raw accent colour and no fill at all. This card's own one
            call-to-action functions as a primary action (rule 3 allows
            exactly one solid-accent primary action per page/card), so it
            takes the SAME filled rounded-control/bg-accent/text-accentContrast
            treatment every other primary button in this app already uses
            (e.g. Config.tsx's Save button) — matching an established idiom
            rather than routing through Badge's tone system, which has no
            "primary CTA" tone of its own and isn't the right place to invent
            one for a single call site. */}
        <Link
          to="/recovery"
          className="self-start inline-flex items-center gap-1 rounded-control bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity"
        >
          {t("recovery.freshNudgeCta")} <span className="inline-block rtl:-scale-x-100">→</span>
        </Link>
      </div>
      <button
        type="button"
        onClick={onDismiss}
        aria-label={t("common.close")}
        className="shrink-0 rounded-control px-2 py-1 text-sm text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors"
      >
        ✕
      </button>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Summary tier — a compact three-cell overview above the detail cards. It
// reuses the same Card/StatCard surface + Badge visual language as the
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
    case "cron": {
      // Raw cron cadence (#107): approximate the fire interval from the gap
      // between its first two fires (mirrors the backend's PeriodSeconds).
      // 0 = not computable → Infinity so it never falsely wins "soonest".
      const secs = cronPeriodSeconds(s.cron);
      return secs > 0 ? secs / 86400 : Infinity;
    }
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

function SummaryCell({
  label,
  children,
  hueIndex,
}: {
  label: string;
  children: React.ReactNode;
  /** Rainbow position for THIS cell's own heading notch — see Card's own
   *  `hueIndex` doc above for the full history. SummaryTier (this cell's one
   *  caller) assigns all three of its cells consecutive positions, each
   *  resolved by ITS OWN caller's `nextHue()` at the same synchronous point
   *  every other block's heading is (see SummaryTier's own `healthHueIndex`
   *  doc for why that has to be a plain number, not a function passed down
   *  for this cell's caller to call later), so the three cells read as one
   *  genuine equal-member set even though each is its own standalone box. */
  hueIndex?: number;
}) {
  return (
    // GlimStone follow-up pass ("half-overlap card notch"): same outer/inner
    // split as Card() above — `relative` moves to this structural outer div
    // so the heading Badge's -11px poke above the card isn't clipped by the
    // inner div's own overflow-hidden (#98's fix for a status chip + a
    // relative-time pair that doesn't fit on one line in this narrow
    // sm:grid-cols-3 cell — unrelated to the heading, but sharing the same
    // box before this pass). `min-w-0` stays on the outer too: it's a grid
    // item, and Chromium/Firefox's grid-track sizing reads min-width off
    // whatever box IS the direct grid child, which is now this outer div.
    // `glim-notch-card` — same rainbow-hue hover-reveal gap as Card() above,
    // never carried here either.
    //
    // insetStart={5} — the EXACT same bare-outer-div/padded-inner-div split
    // as Card() above, so the EXACT same bug: the badge's own -11px poke
    // needed to escape this cell's inner overflow-hidden box (see the
    // comment above), leaving its horizontal static position measured
    // against this now-unpadded OUTER div instead of the inner p-5 box's own
    // content edge — this specific cell (Gesamtzustand/Nächstes Backup/
    // Letztes Ergebnis) is the one jdp measured live and flagged by name
    // ("Dashboard-Badges... links buendig mit der Card"). See Badge.tsx's
    // own `insetStart` doc for the mechanism this fixes at the source
    // instead of re-patching per Card.
    //
    // `.glim-hue` ALSO added, same rainbow-mode completeness fix as Card()
    // above (jdp live review): this cell's own status Badges read
    // tone={statusTone(...)} — load-bearing status signals that read
    // --status-* tokens, never --accent — so they stay untouched; this only
    // matters for anything focusable/accent-coloured a future SummaryCell
    // child might add.
    <div
      className={`relative glim-notch-card min-w-0${hueIndex !== undefined ? " glim-hue" : ""}`}
      style={hueIndex !== undefined ? (hueVars(rainbowAt(hueIndex)) as CSSProperties) : undefined}
    >
      {/* Task 5 (rule 11): each SummaryCell is its own standalone
          bg-carbon-surface rounded-card box — not nested inside an
          already-badged heading — so it gets the same Badge-in-<h2>
          treatment as every other outermost Card heading on this page (see
          Card just above and Badge.tsx's file header). `wrap` + max-w-full
          because this sits in a narrow sm:grid-cols-3 cell. */}
      <h2 className="flex items-center min-w-0">
        <Badge tone="heading" size="heading" wrap className="max-w-full" hueIndex={hueIndex} insetStart={5}>{label}</Badge>
      </h2>
      {/* p-5 (was px-4 py-3, GlimStone follow-up pass, jdp emphatic: "Die
          Cards von Gesamtzustand, Nächstes Backup, Letztes Ergebnis größer
          machen damit der Inhalt nicht so knapp unter dem Cardtitelbadge
          klebt" — make these 3 cards bigger, the content sticks too close
          under the badge). The badge's own -50%-translate overlap always
          pokes exactly 11px into whatever box sits below it (half of the
          22px heading stage — see Badge.tsx's badgeClassName comment), so
          the CLEARANCE between the badge's bottom edge and the first line of
          real content is simply this box's own top padding minus that fixed
          11px. py-3's 12px top padding left only ~1px of clearance — the "OK
          Aktuell" text visibly touching the badge, confirmed live via
          getBoundingClientRect (11px overlap vs. 12px padding). Card()
          above uses p-5 (20px) for the exact same badge, giving it ~9px of
          breathing room — the "comfortable" feel every other Dashboard card
          already has. Matching that value here (rather than inventing a new
          number) makes these three cells read as the same weight of card as
          their neighbours, not a cramped miniature of one. */}
      <div className="bg-carbon-surface rounded-card p-5 flex flex-col gap-2 min-w-0 overflow-hidden">
        {/* flex-wrap so a value that cannot fit on one line (e.g. status chip + a
            relative time in a narrow half-width cell) drops to a second line and stays
            fully readable, instead of being hard-clipped by overflow-hidden (#98). */}
        <div className="flex flex-wrap items-center gap-x-2 gap-y-1 min-h-7 min-w-0">{children}</div>
      </div>
    </div>
  );
}

function SummaryTier({
  t,
  lang,
  domains,
  loading,
  newestRun,
  healthHueIndex,
  nextBackupHueIndex,
  lastResultHueIndex,
}: {
  t: ReturnType<typeof useT>["t"];
  lang: string;
  domains: DomainStatus[];
  loading: boolean;
  newestRun: Run | null;
  /** This tier's three cells' own rainbow positions — see the main Dashboard
   *  component's own `hueSeq`/`nextHue` comment. Plain numbers, each computed
   *  by the CALLER's `nextHue()` at the SAME synchronous point every other
   *  block's own `hueIndex={nextHue()}` is (not a `nextHue` function passed
   *  down for this component to call from its OWN body): a first version of
   *  this fix did exactly that, and it broke, live — React doesn't call a
   *  child function component's body until AFTER the parent's own render
   *  function has already returned, so `nextHue()` calls made from inside
   *  THIS component's body ran strictly after every sibling block's own
   *  direct `nextHue()` call already consumed slots 0-7, landing this tier's
   *  three cells on indices 8/9/10 (wrapping back to red/orange/yellow)
   *  instead of the 0/1/2 they visually occupy first on the page — caught
   *  live via getComputedStyle against the real deployed container, not by
   *  reading the code. Passing three already-resolved numbers sidesteps the
   *  whole "when does React actually call this component" question: the
   *  values are fixed the instant `nextHue()` runs, in the caller's own
   *  array order, same as every other block. */
  healthHueIndex?: number;
  nextBackupHueIndex?: number;
  lastResultHueIndex?: number;
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
      <SummaryCell label={t("dashboard.summaryHealth")} hueIndex={healthHueIndex}>
        {loading ? (
          <span className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</span>
        ) : (
          <>
            {health !== "off" && <Badge tone={statusTone(chipForRpo(health))}>{statusLabel(chipForRpo(health), t)}</Badge>}
            <span className="text-sm text-carbon-text truncate min-w-0">{healthLabel}</span>
          </>
        )}
      </SummaryCell>

      {/* Next backup — soonest scheduled cadence as human text (not a countdown) */}
      <SummaryCell label={t("dashboard.summaryNextBackup")} hueIndex={nextBackupHueIndex}>
        {loading ? (
          <span className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</span>
        ) : (
          <span className="text-sm text-carbon-text truncate min-w-0">
            {nextCadence || t("dashboard.rpoOff")}
          </span>
        )}
      </SummaryCell>

      {/* Last result — the newest run: status chip + target + relative time */}
      <SummaryCell label={t("dashboard.summaryLastResult")} hueIndex={lastResultHueIndex}>
        {newestRun ? (
          <>
            <Badge tone={statusTone(newestRun.status)}>{statusLabel(newestRun.status, t)}</Badge>
            <span className="text-sm text-carbon-text flex-1 truncate min-w-0">
              {runTargetText(t, newestRun)}
            </span>
            <span
              className="text-xs text-carbon-textMuted shrink-0 whitespace-nowrap"
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
  // newest-first, so runs[0] is the latest. Polled (not fetched once) so the
  // cell doesn't freeze on whatever domain happened to be running at page
  // load — mirrors ActivityLog's own listRuns polling (same cadence) so both
  // widgets stay in sync (#158: card stuck on a finished run while the
  // Activity Log had already moved on to the next domain).
  const [runs, setRuns] = useState<Run[]>([]);
  useEffect(() => {
    let active = true;
    const load = () => {
      listRuns()
        .then((res) => {
          if (active && res.ok) setRuns(res.runs ?? []);
        })
        .catch(() => {/* non-fatal — summary "Last result" falls back to empty */});
    };
    load();
    const id = setInterval(load, SUMMARY_RUNS_POLL_MS);
    return () => {
      active = false;
      clearInterval(id);
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

  // Heatmap → Activity Log drilldown: clicking a heatmap cell narrows the log
  // to that LOCAL calendar day (ISO YYYY-MM-DD — the same en-CA local mapping
  // the heatmap cells are keyed by) and scrolls the log card into view.
  // Clicking the active day again — or the chip's × inside ActivityLog —
  // clears it. State lives here because the two cards are independent,
  // individually hideable dashboard blocks with no other shared parent.
  const [logDayFilter, setLogDayFilter] = useState<string | null>(null);
  const activityLogBlockRef = useRef<HTMLDivElement>(null);
  const selectLogDay = (isoDay: string) => {
    const next = logDayFilter === isoDay ? null : isoDay;
    setLogDayFilter(next);
    if (next === null) return; // toggled off — stay put, nothing to show
    const el = activityLogBlockRef.current;
    if (!el) return; // Activity Log block hidden via customize — filter still set
    const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    el.scrollIntoView({ behavior: reducedMotion ? "auto" : "smooth", block: "start" });
  };

  // Customizable dashboard (#46) — everything below the heading + banners is a
  // reorderable / hideable block, persisted per-browser via useDashboardLayout.
  const [editing, setEditing] = useState(false);

  // Ordered block list. Each block has a stable id, a label, a `render`
  // callback that produces the node (props preserved exactly from the
  // original render) and an advancedOnly flag. advancedOnly blocks are
  // dropped from BOTH the render and the customize list when not in Advanced
  // view — their order/hidden state still persists.
  //
  // `render` — GlimStone follow-up pass, jdp's live review of this page:
  // "Cardtitelbadges sind falsch platziert. Alle sind nicht im
  // Regenbogenmodus." Was `node: React.ReactNode`, a pre-built element
  // constructed eagerly, in this array's own FIXED definition order — which
  // cannot give a Card heading a correct rainbow position, because the
  // block a user actually SEES at position N depends on their own persisted
  // drag-reorder + hide/show state (`visibleBlocks` below), not this
  // array's literal order. Deferred to a function so each Card's hueIndex
  // can be assigned from the shared `nextHue()` counter (declared right
  // before `visibleBlocks.map()` in the JSX below) at the point each block
  // is ACTUALLY rendered, in the user's own current visible order — the
  // exact same "own running counter, consumed in rendered order" contract
  // Settings.tsx's `nextHue()` already uses, just passed through explicitly
  // here instead of called inline, since the render order here isn't a
  // static JSX literal the way Settings.tsx's tab bodies are.
  //   Most blocks consume exactly one hue slot (one Card, one `nextHue()`
  // call); "summary" consumes three (its own three SummaryCells, via the
  // `nextHue` callback threaded into SummaryTier); "stats" consumes none
  // (StatCardsRow's tiles have no heading badge of their own — nothing to
  // hue).
  const blocks: {
    id: string;
    label: string;
    advancedOnly?: boolean;
    render: (nextHue: () => number) => React.ReactNode;
  }[] = [
    {
      id: "summary",
      label: t("dashboard.blockSummary"),
      // Three DIRECT nextHue() calls, eagerly resolved to plain numbers here
      // rather than a `nextHue` function handed to SummaryTier to call from
      // its own component body — see SummaryTier's own `healthHueIndex` doc
      // for the live ordering bug that shape caused (React doesn't invoke a
      // child component's body until after this whole `blocks` array/render
      // pass has already returned, so calls made from inside SummaryTier ran
      // AFTER every sibling block below had already consumed its own slot).
      render: (nextHue) => (
        <SummaryTier
          t={t}
          lang={lang}
          domains={statusDomains}
          loading={statusLoading}
          newestRun={runs[0] ?? null}
          healthHueIndex={nextHue()}
          nextBackupHueIndex={nextHue()}
          lastResultHueIndex={nextHue()}
        />
      ),
    },
    {
      id: "activityLog",
      label: t("activityLog.title"),
      // The wrapper div carries the scroll anchor for the heatmap drilldown
      // (scroll-mt keeps the card heading clear of the viewport's top edge).
      render: (nextHue) => (
        <div ref={activityLogBlockRef} className="scroll-mt-4">
          <ActivityLog
            dayFilter={logDayFilter}
            onClearDayFilter={() => setLogDayFilter(null)}
            hueIndex={nextHue()}
          />
        </div>
      ),
    },
    {
      id: "stats",
      label: t("dashboard.blockStats"),
      render: () => <StatCardsRow t={t} advanced={advanced} />,
    },
    {
      id: "protection",
      label: t("dashboard.protectionTitle"),
      render: (nextHue) => (
        <ProtectionCard t={t} domains={statusDomains} loading={statusLoading} hueIndex={nextHue()} />
      ),
    },
    {
      id: "ransomware",
      label: t("ransomware.title"),
      advancedOnly: true,
      render: (nextHue) => (
        <RansomwareCard t={t} domains={statusDomains} loading={statusLoading} hueIndex={nextHue()} />
      ),
    },
    // Last Backups and Run History are separate blocks (#50 follow-up) so each
    // can be hidden, reordered and read at full width independently.
    {
      id: "lastBackups",
      label: t("dashboard.lastBackups"),
      render: (nextHue) => <LastBackupsCard t={t} hueIndex={nextHue()} />,
    },
    {
      id: "runHistory",
      label: t("run.historyTitle"),
      render: (nextHue) => <RunsCard t={t} hueIndex={nextHue()} />,
    },
    {
      id: "heatmap",
      label: t("dashboard.healthTitle"),
      render: (nextHue) => (
        <HealthHeatmapCard t={t} selectedDay={logDayFilter} onSelectDay={selectLogDay} hueIndex={nextHue()} />
      ),
    },
    {
      id: "storage",
      label: t("dashboard.storageTitle"),
      render: (nextHue) => <StorageCard t={t} hueIndex={nextHue()} />,
    },
    {
      id: "spike",
      label: t("spike.title"),
      advancedOnly: true,
      render: (nextHue) => <SpikeCard t={t} hueIndex={nextHue()} />,
    },
  ];

  const defaultOrder = blocks.map((b) => b.id);
  const { order, hidden, reorder, toggleHidden, toggleWidth, getWidth, reset } =
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
    // GlimStone follow-up pass (live-review round, jdp emphatic: "Die
    // Abstände der Cards passen nicht. Bitte systemweit anpassen!"): this
    // page's own block-to-block rhythm was gap-6 (24px) — measured live via
    // getBoundingClientRect, every adjacent pair of stacked cards sat exactly
    // 24px apart, so it wasn't a mix of ad-hoc values WITHIN this page. The
    // actual drift is against the rest of the app: Settings.tsx's own
    // tab-panels wrapper already settled on gap-10 (40px) as "the same 40px
    // rhythm every Card-to-Card gap already uses" (see that file's own
    // comment, live-review round) and even bumped its heading-to-first-card
    // gap to match it for the identical reason this pass now applies here —
    // a smaller gap right before the first card read as visually mismatched
    // next to the wider rhythm below it. Splitting this outer wrapper into
    // its own gap-6 header/banner group plus an outer gap-10 mirrors that
    // exact two-level structure (Settings' `hueSeq` comment calls out the
    // same shared-counter pattern this page already reuses, for the same
    // reason: matching an established convention beats reinventing one).
    //   PAGE_SHELL (jdp live-review, "Können wir die nicht überall gleich
    // breit machen?"): this page's `gap-10 max-w-6xl` is now that shared
    // constant, unchanged in value — it is the page the app-wide 1152px was
    // chosen FROM, because it owns the only content dense enough to have a
    // measurable opinion about width (a md:grid-cols-2 block grid, 7-column
    // container-query run rows, and the Advanced 7-across stat tier, whose
    // longest German label needs exactly 136px of a 136px cell at 1024px —
    // zero slack). Swapping the literal for the constant is what stops the
    // other nine pages drifting away from it again. See lib/pageShell.ts.
    //   The nested gap-6 group below stays: heading + banner are a tight pair
    // that deliberately sits closer than the 40px Card rhythm, the same
    // two-level shape Settings.tsx uses for its heading + tab strip.
    <div className={PAGE_SHELL}>
      <div className="flex flex-col gap-6">
      {/* Page heading — fixed (contextual, not customizable). The pencil in the
          top-right corner toggles the customize/edit mode.
            That pencil is `h-8 w-8` + centring, not the `p-2` it used to size
          itself with. It is a square icon-only badge by every other measure
          (the same rounded-control tile, the same bg-carbon-surface2/hover
          recipe as Settings' Registry add/remove and FolderBrowser's browse
          badge), but it derived its own footprint from padding around an 18px
          glyph and landed on 34px — measured live — where every other square
          icon badge in the app is 32px. Two pixels, but exactly the drift the
          one-size rule exists to stop: a call site sizing itself from its own
          contents instead of taking the shared number. See Badge.tsx's "ONE
          SIZE FOR SQUARE ICON BADGES" block. */}
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
        {/* Real `.glim-bubble` tooltip, not the OS's native `title=` balloon
            (whole-app sweep). This was the LAST icon-only control in the app
            still naming itself with a bare `title=`/`aria-label` pair —
            measured live on the deployed container: 32px, rounded-control,
            and `title` present, while every other icon-only trigger (every
            Badge `tip`, FolderBrowser's browse badge, Settings' registry and
            copy badges, PathModeSwitch's and SourceToggle's segments) already
            rendered the shared bubble. IconTipButton.tsx's own header is
            explicit that a stray native `title=` on an icon-only trigger is
            precisely the anti-pattern that file exists to replace, and
            design-language's tooltip section calls the bubble unconditional
            for a control with no visible text.
              It stays a hand-rolled `h-8 w-8` button rather than becoming a
            Badge, for the reason already recorded below: its background
            legitimately flips between two states, and Badge's icon-only
            tone="active" is unconditionally accent-filled. IconTipButton
            takes the className verbatim, so both states survive byte-identical
            — only the tooltip mechanism changes. `aria-pressed` is threaded
            through IconTipButton's new optional prop so the toggle state is
            not lost in the swap (this is a toggle, not a one-shot action).
              32px and `rounded-control` are unchanged, so it still matches
            every other square icon control app-wide and still tracks the
            shape engine. */}
        <IconTipButton
          onClick={() => setEditing((v) => !v)}
          tip={editing ? t("dashboard.customizeDone") : t("dashboard.customize")}
          ariaPressed={editing}
          className={`shrink-0 inline-flex h-8 w-8 items-center justify-center rounded-control motion-safe:transition-colors ${
            editing
              ? "bg-accent text-accentContrast"
              : "bg-carbon-surface2 text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
          }`}
        >
          {/* FILLED pencil (design-language.md "Icon glyphs", rule 218 —
              already a closed silhouette under its old stroke, so it flips
              directly: same path data, `fill="currentColor"`). The old
              facet-highlight line is dropped rather than faked as a
              surface-colour cutout — this button's own background flips
              between `bg-carbon-surface2` and `bg-accent` (the `editing`
              state above), so a cutout hard-coded to one of those two colours
              would show a visible mismatched patch in the other; the plain
              pencil silhouette alone already reads clearly as "edit" without
              it.
                This used to be an inline `<svg>` right here, and it was the
              app's ONLY pencil. Files.tsx's own "Ordner-Set bearbeiten" badge
              needed the same glyph (jdp's icon-badge round for that tab), and
              the standing instruction there was to reuse this one rather than
              draw a second — so the path moved verbatim into Sidebar.tsx's
              shared icon set as IconPencil and this call site now renders that
              component. Same silhouette; the only difference is that IconPencil
              crops its viewBox to the ink (see its own comment for the measured
              numbers and why), so this button's glyph goes from 11.34 × 10.58
              px of ink at 18px to 12.20 × 11.39 px at the app's standard 16px
              icon-badge glyph size — sub-pixel-per-axis in practice, and it
              brings this one in line with every other 16px glyph in a 32px
              tile. The button itself is untouched: it stays a hand-rolled
              `h-8 w-8` rather than a Badge, because its background legitimately
              flips between two states (`bg-accent` while editing,
              `bg-carbon-surface2` at rest) and Badge's icon-only tone="active"
              is unconditionally accent-filled. */}
          <IconPencil />
        </IconTipButton>
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
            className="self-start rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text motion-safe:transition-colors"
          >
            {t("dashboard.resetLayout")}
          </button>
          <p className="text-xs text-carbon-textMuted">{t("dashboard.customizeHint")}</p>
        </div>
      )}
      </div>

      {/* Ordered, visible blocks in a responsive grid: full-width cards span
          both columns, half-width cards flow two-per-row (request B). Below
          the md breakpoint everything stacks in a single column regardless of
          width. The col-span lives on this wrapper div (not on
          CustomizableBlock's own root) so it applies in BOTH edit mode (where
          CustomizableBlock renders its control-bar div) and view mode (where
          it renders only `<>{children}</>`). In edit mode each block carries a
          control bar + native drag-and-drop; otherwise the card renders
          plainly. Dragging still reorders the flat `order` array — the grid
          simply derives each cell's span from order + width. */}
      {/* hueSeq/nextHue — SAME page-wide running-counter pattern as
          Settings.tsx's own `nextHue()` (see that file's own `hueSeq`
          comment), just declared here instead of at the top of the return:
          it must be freshly reset to 0 on every render (a stale count would
          drift every heading's colour after any state change) AND consumed
          in the ACTUAL rendered order of `visibleBlocks` below — the user's
          own current drag-reorder/hide-show layout, not this file's fixed
          `blocks` definition order (GlimStone follow-up pass, jdp: "Alle
          sind nicht im Regenbogenmodus" — every heading on this page was
          still the flat, un-hued Task-5 default; see the `blocks` array's
          own `render` comment above for why a plain per-block literal index
          can't do this and a shared counter can). Each block's own `render`
          callback calls this DIRECTLY, once per real heading badge it owns,
          right here in this synchronous pass (most blocks once; "summary"
          three times, back-to-back, for its own three SummaryCells; "stats"
          zero times — StatCardsRow has no heading to hue) — never by handing
          the `nextHue` function itself to a child component to call later
          from its own body: React doesn't actually invoke a child function
          component until after THIS WHOLE render pass has returned, so a
          call made from inside a child runs strictly after every direct
          call made here, landing on the wrong (already-past-the-end)
          indices — caught live the first time this shipped (see
          SummaryTier's own `healthHueIndex` doc for the exact live numbers)
          and fixed by resolving all three of its indices to plain numbers
          right here, the same way every other block already does. */}
      <div className="grid grid-cols-1 gap-10 md:grid-cols-2">
        {(() => {
          let hueSeq = 0;
          const nextHue = () => hueSeq++;
          return visibleBlocks.map((b, i) => (
            <div
              key={b.id}
              className={getWidth(b.id) === "half" ? "md:col-span-1" : "md:col-span-2"}
            >
              <CustomizableBlock
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
                width={getWidth(b.id)}
                onToggleWidth={() => toggleWidth(b.id)}
                t={t}
              >
                {b.render(nextHue)}
              </CustomizableBlock>
            </div>
          ));
        })()}
      </div>

      {/* Hidden-cards tray — only while editing and something is hidden. */}
      {editing && hiddenBlocks.length > 0 && (
        <div className="relative flex flex-col gap-3 rounded-card border border-dashed border-carbon-border p-4">
          <h2 className="flex items-center">
            <Badge tone="heading" size="heading" wrap>{t("dashboard.hiddenCards")}</Badge>
          </h2>
          <div className="flex flex-wrap gap-2">
            {hiddenBlocks.map((b) => (
              <div
                key={b.id}
                className="flex items-center gap-2 rounded-control bg-carbon-surface2 px-2.5 py-1.5"
              >
                <span className="max-w-48 truncate text-xs text-carbon-textSub">
                  {b.label}
                </span>
                <button
                  type="button"
                  onClick={() => toggleHidden(b.id)}
                  aria-label={`${t("dashboard.showCard")} ${b.label}`}
                  className="rounded-control px-2 py-0.5 text-xs text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text motion-safe:transition-colors"
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
