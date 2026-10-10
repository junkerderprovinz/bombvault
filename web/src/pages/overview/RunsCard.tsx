import { useState } from "react";
import type { CSSProperties } from "react";
import type { AnomalyView, Run } from "../../lib/api";
import { hueVars } from "../../lib/appearance";
import { humanBytes } from "../../lib/forecast";
import type { useT } from "../../lib/i18n";
import { formatDuration, formatTs, relativeTime } from "../../lib/reltime";
import { runTargetText, statusLabel, statusTone } from "../../lib/runDisplay";
import { runKindLabel } from "../../lib/runKind";
import { isOwnReason, isWarningNote, RunReasonText } from "../../lib/runReason";
import { useOpenAnomalies } from "../../lib/useAnomalies";
import { Badge } from "../../components/Badge";
import { ErrorDetailPanel } from "../../components/ErrorDetailPanel";
import { MobileSectionLabel } from "../../components/mobile/MobileSectionLabel";
import { RunAnomalyBadge } from "../../components/RunAnomalyBadge";
import { SelectField } from "../../components/SelectField";
import { Card } from "./Card";
import { FailureCounter } from "./FailureCounter";
import { failedRunsNeedingAttention } from "./runFailures";

/** The scrubbed, direction-fixed note a run shows under its summary: the
 *  reason a run failed or was skipped, or what a successful one has to add
 *  (which folder an import kept, a dump that reached one database only).
 *  One definition for both run-row faces, so the desktop history card and
 *  the phone glance list cannot drift on what a run says. dir stays "ltr"
 *  for a restic, rclone or Docker message and follows the page for one of
 *  our own sentences. */
function RunReasonLine({
  t,
  run,
  dense,
}: {
  t: ReturnType<typeof useT>["t"];
  run: Run;
  /** The phone face seats the line inside its tap target, where only
   *  phrasing content may go, and needs its own top margin. */
  dense?: boolean;
}) {
  if (!run.error) return null;
  let tone: string;
  if (run.status === "failed") tone = "text-statusFail";
  else if (run.status === "skipped") tone = "text-carbon-textMuted";
  else if (run.status === "success") tone = isWarningNote(run.error) ? "text-statusWarn" : "text-carbon-textMuted";
  else return null;
  const Tag = dense ? "span" : "p";
  return (
    <Tag
      dir={isOwnReason(run.error) ? undefined : "ltr"}
      className={`${dense ? "mt-0.5 " : ""}block text-xs wrap-break-word text-start ${tone}`}
    >
      <RunReasonText reason={run.error} t={t} />
    </Tag>
  );
}

/** One run row for both faces of RunsCard: the status badge, the kind/target
 *  summary, the relative age, and the reason line are written here once, so
 *  the two densities cannot disagree on what a row says. The faces stay two
 *  arrangements of that one content: the phone row is a >=44px touch target
 *  whose tap opens the run detail sheet (min-h-[2.75rem], relative age, byte
 *  volume), the desktop row is a static grid line with exact timestamps (the
 *  precise clock lives in the run detail the phone taps through to). */
function RunRow({
  t,
  run,
  dense,
  hueIndex,
  onTap,
  anomalies,
}: {
  t: ReturnType<typeof useT>["t"];
  run: Run;
  /** The phone glance face. */
  dense?: boolean;
  /** The phone row's position in the block's hue rotation. */
  hueIndex?: number;
  /** Phone only: opens the page-hosted run detail sheet for this run. */
  onTap?: () => void;
  /** Open anomaly findings raised on this run; the desktop row marks them. */
  anomalies?: AnomalyView[];
}) {
  const badge = <Badge tone={statusTone(run.status)}>{statusLabel(run.status, t)}</Badge>;
  const kind = runKindLabel(t, run.kind);
  const target = runTargetText(t, run);
  const age = relativeTime(t, run.startedAt);
  if (dense) {
    return (
      <button
        type="button"
        onClick={onTap}
        aria-label={`${statusLabel(run.status, t)} · ${kind} ${target}`}
        className="flex min-h-[2.75rem] w-full items-center gap-2 rounded-control bg-carbon-surface2 px-2 py-2 text-start glim-hue"
        style={hueVars(hueIndex ?? 0) as CSSProperties}
      >
        <span className="shrink-0">{badge}</span>
        <span className="min-w-0 flex-1">
          <span className="block truncate text-sm font-semibold text-carbon-text">
            {kind} · {target}
          </span>
          <span className="mt-0.5 block truncate text-xs text-carbon-textMuted">
            {age}
            {run.bytes > 0 ? ` · ${humanBytes(run.bytes)}` : ""}
          </span>
          <RunReasonLine t={t} run={run} dense />
        </span>
      </button>
    );
  }
  const dur = run.finishedAt != null ? formatDuration(run.finishedAt - run.startedAt) : "";
  return (
    <div className="flex flex-col gap-0.5 rounded-control bg-carbon-surface2 px-2 py-2.5 text-sm">
      <div className="flex items-center gap-3">
        {badge}
        {/* Room for two lines: "Database dump" and its translations do not
            fit one line at 360px. */}
        <span className="text-carbon-text font-medium min-w-16 max-w-32 shrink-0 break-words">{kind}</span>
        <span className="text-carbon-text flex-1 truncate min-w-0">{target}</span>
        <RunAnomalyBadge findings={anomalies} t={t} />
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
            {age}
          </span>
        </span>
      </div>
      <RunReasonLine t={t} run={run} />
    </div>
  );
}

export function RunsCard({
  t,
  hueIndex,
  dense,
  runs,
  loading,
  failed,
  refreshRuns,
  onOpenRun,
}: {
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
  /** True = the phone glance surface; false = the desktop history card. */
  dense: boolean;
  /** The page's polled listRuns result (newest-first); the same list the
   *  summary tier's "Last result" cell reads. */
  runs: Run[];
  /** True until the page's first runs load settles. */
  loading: boolean;
  /** True when the page's last runs load failed (the load-failure copy). */
  failed: boolean;
  /** Re-reads the page's runs list; fired after the error panel resolves a
   *  failure, so acknowledged failures drop out of the counter live. */
  refreshRuns: () => void;
  /** Phone only: opens the page-hosted run detail sheet for a tapped row. */
  onOpenRun?: (run: Run) => void;
}) {
  const [day, setDay] = useState("all");
  const [panelOpen, setPanelOpen] = useState(false);
  const { byRunId } = useOpenAnomalies();
  // The failures the counter still counts (shared with the stat tier's
  // errors tile; one derivation, see its own doc).
  const failures = failedRunsNeedingAttention(runs);

  if (dense) {
    // The four most recent runs, newest first, through the shared RunRow so
    // the glance face and the desktop history card say the same thing about
    // a run (the tap opens the page-hosted RunDetailSheet; no route).
    const recent = runs.slice(0, 4);
    return (
      <section className="relative glim-notch-card glim-hue" style={hueVars(hueIndex ?? 0) as CSSProperties}>
        <MobileSectionLabel t={t} labelKey="dashboard.recentRuns" />
        <div className="rounded-card bg-carbon-surface p-2 pt-5">
          {failures.length > 0 && (
            <FailureCounter t={t} count={failures.length} dense hueIndex={0} onOpen={() => setPanelOpen(true)} />
          )}
          {panelOpen && (
            <ErrorDetailPanel onClose={() => setPanelOpen(false)} onChanged={refreshRuns} />
          )}
          {loading ? (
            <p className="px-2 py-2 text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
          ) : (
            <>
              {failed && (
                // The load-failure copy, never "No runs yet": an empty list
                // from a failed read must not impersonate an answer.
                <p className="px-2 py-2 text-sm text-statusFail">{t("dashboard.loadRunsFailed")}</p>
              )}
              {!failed && recent.length === 0 && (
                <p className="px-2 py-2 text-sm text-carbon-textMuted">{t("dashboard.noRuns")}</p>
              )}
              {recent.length > 0 && (
                // Rows separated by shade (soft tiles), never divider lines.
                // The counter row owns rotation position 0; the run rows
                // follow in display order, so a row keeps its hue whether or
                // not the counter row is present above it.
                <div className="flex flex-col gap-1">
                  {recent.map((run, i) => (
                    <RunRow key={run.id} t={t} run={run} dense hueIndex={i + 1} onTap={() => onOpenRun?.(run)} />
                  ))}
                </div>
              )}
            </>
          )}
        </div>
      </section>
    );
  }

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
      {failed && <p className="text-sm text-statusFail">{t("dashboard.loadRunsFailed")}</p>}
      {!loading && !failed && runs.length === 0 && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.noRuns")}</p>
      )}
      {runs.length > 0 && (
        <div className="glim-content-fade">
          {/* Day filter */}
          <div className="flex items-center gap-2 mb-2">
            <label className="text-xs text-carbon-textMuted">{t("run.filterDay")}</label>
            <SelectField
              value={day}
              onChange={setDay}
              label={t("run.filterDay")}
              options={[
                { value: "all", label: t("run.allDays") },
                ...days.map((d) => ({ value: d, label: d })),
              ]}
              className="rounded-control bg-carbon-surface2 px-2 py-1 text-xs text-carbon-text glim-field-focus"
            />
          </div>
          {/* Scrollable list of all runs in the window, filtered by day */}
          <div className="flex flex-col gap-1 max-h-128 overflow-y-auto pe-2">
            {shown.map((run) => (
              <RunRow key={run.id} t={t} run={run} anomalies={byRunId.get(run.id)} />
            ))}
          </div>
        </div>
      )}
    </Card>
  );
}
