import type { DomainStatus, Run, ScheduleNext } from "../../lib/api";
import type { useT } from "../../lib/i18n";
import { formatTs, relativeTime } from "../../lib/reltime";
import { runTargetText, statusLabel, statusTone } from "../../lib/runDisplay";
import { Badge } from "../../components/Badge";
import { NextRunCard } from "./NextRunCard";
import { SummaryCell } from "./SummaryCell";
import { WorstRpoHealthLine } from "./WorstRpoHealthLine";

// SummaryTier is the three-cell overview above the detail cards. It reads the
// status domains and the newest run the page has already fetched.
export function SummaryTier({
  t,
  domains,
  scheduleNext,
  loading,
  newestRun,
  healthHueIndex,
  nextBackupHueIndex,
  lastResultHueIndex,
}: {
  t: ReturnType<typeof useT>["t"];
  domains: DomainStatus[];
  loading: boolean;
  newestRun: Run | null;
  /** The rainbow positions of the three cells, as plain numbers the page
   *  resolves with its own nextHue() calls. Handing the nextHue function down
   *  instead would not work: React runs a child component's body after the
   *  parent's render has returned, so calls made in here would come after
   *  every sibling block's and land the cells behind them in the rotation. */
  healthHueIndex?: number;
  nextBackupHueIndex?: number;
  lastResultHueIndex?: number;
  /** GET /api/schedule/next, soonest first: the scheduler's own answer to
   *  "what fires next", which the activity log and the Unraid widget read as
   *  well (#187). */
  scheduleNext: ScheduleNext[];
}) {
  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
      {/* Overall health: the worst RPO status across enabled domains */}
      <SummaryCell label={t("dashboard.summaryHealth")} hueIndex={healthHueIndex}>
        <WorstRpoHealthLine t={t} domains={domains} loading={loading} />
      </SummaryCell>

      {/* Next backup: the soonest fire time from the scheduler, as a countdown */}
      <NextRunCard
        t={t}
        dense={false}
        hueIndex={nextBackupHueIndex}
        scheduleNext={scheduleNext}
        domains={domains}
        loading={loading}
      />

      {/* Last result: the newest run's status chip, target and relative time */}
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
