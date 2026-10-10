import { useNavigate } from "react-router-dom";

import type { useT } from "../../lib/i18n";
import { NOT_INSTALLED, inSchedule, tilePath } from "../../lib/backupList";
import type { Tally } from "../../lib/protectionTally";
import { Card } from "./Card";
import { StatCard } from "./StatCard";

/**
 * StatCardsRow is the card of plain numbers. Each one counts the same entries
 * as every other card, and the two that lead somewhere are buttons.
 */
export function StatCardsRow({
  t,
  hueIndex,
  containers,
  vms,
  all,
  errors,
  onErrors,
}: {
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
  containers: Tally;
  vms: Tally;
  all: Tally;
  /** Failed runs that are neither repeated with success nor acknowledged. */
  errors: number;
  onErrors: () => void;
}) {
  const navigate = useNavigate();
  const scheduled = all.entries.filter(inSchedule).length;
  return (
    <Card title={t("dashboard.blockStats")} hint={t("overview.stats.hint")} hueIndex={hueIndex}>
      <div className="grid grid-cols-[repeat(auto-fit,minmax(7.5rem,1fr))] gap-3">
        <StatCard label={t("dashboard.statContainers")} value={containers.total} />
        <StatCard label={t("dashboard.statVMs")} value={vms.total} />
        <StatCard label={t("dashboard.statErrors")} value={errors} danger onClick={errors > 0 ? onErrors : undefined} />
        <StatCard label={t("dashboard.statActiveJobs")} value={scheduled} />
        <StatCard label={t("dashboard.statPausedJobs")} value={all.total - scheduled} />
        <StatCard
          label={t("containers.notInstalled")}
          value={all.gone}
          onClick={all.gone > 0 ? () => navigate(tilePath(NOT_INSTALLED)) : undefined}
        />
      </div>
    </Card>
  );
}
