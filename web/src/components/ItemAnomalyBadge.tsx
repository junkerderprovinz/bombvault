import { Link } from "react-router-dom";

import { Badge } from "./Badge";
import {
  anomalyDomainsLabel,
  anomalyLearningText,
  anomalySeverityTone,
  itemOpenCounts,
  worstSeverity,
  type TranslateAnomaly,
} from "../lib/anomalies";
import type { AnomalyItem } from "../lib/api";

/**
 * ItemAnomalyBadge is the mark beside an item's name: the open findings, or
 * how far the history still has to go before they can be trusted. The learning
 * caption is hidden while detection is off, since nothing is learning then,
 * while the counts already found stay readable.
 */
export function ItemAnomalyBadge({
  item,
  enabled,
  t,
}: {
  item?: AnomalyItem;
  enabled: boolean;
  t: TranslateAnomaly;
}) {
  if (!item) return null;

  const counts = itemOpenCounts(item);
  const open = counts.critical + counts.warning + counts.info;
  const worst = worstSeverity(counts);
  if (open > 0 && worst) {
    return (
      <Link
        to={`/anomalies?scope=item:${encodeURIComponent(item.targetId)}#findings`}
        aria-label={t("anomaly.itemBadgeAria")
          .replace("{name}", item.name || anomalyDomainsLabel(item.domain, t))
          .replace("{n}", open.toLocaleString())}
      >
        <Badge tone={anomalySeverityTone(worst)} size="small" shape="pill">
          {open}
        </Badge>
      </Link>
    );
  }

  const { samples, needed, noData } = item.learning;
  if (!enabled || !item.scheduled || (samples >= needed && !noData)) return null;
  return (
    <span className="text-xs text-carbon-textSub">
      {anomalyLearningText(t, samples, needed, noData)}
    </span>
  );
}
