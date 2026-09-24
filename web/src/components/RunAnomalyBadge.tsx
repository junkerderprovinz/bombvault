import { Badge } from "./Badge";
import { InfoBubble } from "./InfoBubble";
import {
  anomalySentence,
  anomalySeverityTone,
  sortOpenAnomalies,
  type TranslateAnomaly,
} from "../lib/anomalies";
import type { AnomalyView } from "../lib/api";
import { useT } from "../lib/i18n";

/**
 * RunAnomalyBadge marks a run an open finding was raised on or last seen in.
 * The sentences sit in a bubble beside it rather than in a title, which touch
 * and keyboard cannot reach.
 */
export function RunAnomalyBadge({ findings, t }: { findings?: AnomalyView[]; t: TranslateAnomaly }) {
  const { lang } = useT();
  if (!findings || findings.length === 0) return null;
  const sorted = sortOpenAnomalies(findings);
  return (
    <span className="inline-flex shrink-0 items-center gap-1">
      <Badge tone={anomalySeverityTone(sorted[0].severity)} size="small">
        {t("anomaly.runBadge")}
      </Badge>
      <InfoBubble tip={sorted.map((a) => anomalySentence(a, t, lang)).join(" ")} />
    </span>
  );
}
