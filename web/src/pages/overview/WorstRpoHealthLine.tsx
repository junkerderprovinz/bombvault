import type { DomainStatus } from "../../lib/api";
import type { useT } from "../../lib/i18n";
import { statusLabel, statusTone } from "../../lib/runDisplay";
import { Badge } from "../../components/Badge";
import { chipForRpo, worstRpoLabel, worstRpoStatus } from "./rpo";

/** The worst-RPO health line: a Badge plus plain-text label from the shared
 *  worstRpoStatus derivation, written once for its two consumers (the summary
 *  tier's health cell and the phone storage block's health row) so the two
 *  surfaces cannot disagree on what repo health says. Four-status language
 *  throughout: Badge + text label, never color alone. */
export function WorstRpoHealthLine({
  t,
  domains,
  loading,
}: {
  t: ReturnType<typeof useT>["t"];
  domains: DomainStatus[];
  /** True until the caller's /api/status load settles; the line reads
   *  "checking" rather than guessing from an empty list. */
  loading: boolean;
}) {
  if (loading) {
    return <span className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</span>;
  }
  const health = worstRpoStatus(domains);
  return (
    <>
      {health !== "off" && (
        <Badge tone={statusTone(chipForRpo(health))}>{statusLabel(chipForRpo(health), t)}</Badge>
      )}
      <span className="text-sm text-carbon-text truncate min-w-0">{worstRpoLabel(t, health)}</span>
    </>
  );
}
