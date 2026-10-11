import { useAttentionCount, useFreshInstall } from "./attention";
import { useT } from "./i18n";
import type { NavCountKind } from "./navModel";
import { useLoudAnomalies } from "./useAnomalies";

export interface NavCount {
  value: number;
  /** The figure said in words, because the bare number names nothing. */
  label: string;
  /** The status the figure stands for, for a surface that shows it as one. */
  tone: "fail" | "warn";
}

/**
 * useNavCounts resolves the figures the navigation entries carry, once for
 * every surface that shows them. Before the first backup they are all zero:
 * nothing can need the person yet.
 */
export function useNavCounts(): Record<NavCountKind, NavCount> {
  const { t } = useT();
  const fresh = useFreshInstall();
  const needs = useAttentionCount();
  const loud = useLoudAnomalies();
  const attention = fresh ? 0 : needs;
  const anomalies = fresh ? { ...loud, count: 0 } : loud;
  return {
    attention: {
      value: attention,
      label: t("nav.attentionCountAria").replace("{n}", attention.toLocaleString()),
      tone: "fail",
    },
    anomalies: {
      value: anomalies.count,
      label: t("anomaly.navCountAria").replace("{n}", anomalies.count.toLocaleString()),
      tone: anomalies.tone,
    },
  };
}
