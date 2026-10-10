import type { DomainStatus, Run, ScheduleNext } from "../../lib/api";
import type { useT } from "../../lib/i18n";
import { formatTs, relativeTime } from "../../lib/reltime";
import { runTargetText, statusLabel, statusTone } from "../../lib/runDisplay";
import { Badge } from "../../components/Badge";
import { Card } from "./Card";
import { NextRunCard } from "./NextRunCard";
import { SummaryCell } from "./SummaryCell";
import { WorstRpoHealthLine } from "./WorstRpoHealthLine";

// SummaryTier is the card of three figures: the oldest recovery point, the
// next backup and the last result. It reads what the page has already fetched.
export function SummaryTier({
  t,
  domains,
  scheduleNext,
  loading,
  newestRun,
  hueIndex,
}: {
  t: ReturnType<typeof useT>["t"];
  domains: DomainStatus[];
  loading: boolean;
  newestRun: Run | null;
  hueIndex?: number;
  /** GET /api/schedule/next, soonest first: the scheduler's own answer to
   *  "what fires next", which the activity log and the Unraid widget read as
   *  well (#187). */
  scheduleNext: ScheduleNext[];
}) {
  return (
    <Card title={t("dashboard.blockSummary")} hint={t("overview.summary.hint")} hueIndex={hueIndex}>
      <div className="@container">
        <div className="grid grid-cols-1 gap-4 @[30rem]:grid-cols-3">
          <SummaryCell label={t("dashboard.summaryHealth")}>
            <WorstRpoHealthLine t={t} domains={domains} loading={loading} />
          </SummaryCell>

          <NextRunCard t={t} dense={false} scheduleNext={scheduleNext} domains={domains} loading={loading} />

          <SummaryCell label={t("dashboard.summaryLastResult")}>
            {newestRun ? (
              <>
                <Badge tone={statusTone(newestRun.status)}>{statusLabel(newestRun.status, t)}</Badge>
                <span className="min-w-0 flex-1 truncate text-sm text-carbon-text">{runTargetText(t, newestRun)}</span>
                <span className="shrink-0 whitespace-nowrap text-xs text-carbon-textMuted" title={formatTs(newestRun.startedAt)}>
                  {relativeTime(t, newestRun.startedAt)}
                </span>
              </>
            ) : (
              <span className="text-sm text-carbon-textMuted">{t("dashboard.noRuns")}</span>
            )}
          </SummaryCell>
        </div>
      </div>
    </Card>
  );
}
