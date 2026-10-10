import { useCallback } from "react";
import { Link } from "react-router-dom";
import { acknowledgeAnomalies, markAnomaliesExpected } from "../../lib/api";
import type { AnomalyActionResult, AnomalySummary, AnomalyView } from "../../lib/api";
import { ANOMALY_CHANGED_EVENT, sortOpenAnomalies } from "../../lib/anomalies";
import type { useT } from "../../lib/i18n";
import { useAnomalySummary, useOpenAnomalies } from "../../lib/useAnomalies";
import { AnomalyRow, type AnomalyAction } from "../../components/AnomalyRow";
import { Card } from "./Card";

/**
 * What the backup history says is out of the ordinary.
 *
 * It sits next to the coverage card because the two answer the same question
 * from opposite ends: that one names what nothing backs up, this one names the
 * backup that ran and came out wrong. Only critical and warning findings reach
 * the card; notes are counted and linked, so a card people read stays a card
 * about things that matter.
 */
export function AnomaliesCard({
  t,
  summary,
  open,
  loading,
  error,
  hueIndex,
  onAcknowledge,
  onExpected,
}: {
  t: ReturnType<typeof useT>["t"];
  summary: AnomalySummary | null;
  open: AnomalyView[];
  loading: boolean;
  /** The summary or the listing was refused or never arrived. */
  error: boolean;
  hueIndex?: number;
  onAcknowledge?: AnomalyAction;
  onExpected?: AnomalyAction;
}) {
  const rows = sortOpenAnomalies(open);
  const criticals = rows.filter((a) => a.severity === "critical").slice(0, 5);
  const warnings = rows.filter((a) => a.severity === "warning").slice(0, 3);
  const loud = summary ? summary.open.critical + summary.open.warning : 0;
  const notShown = Math.max(0, loud - criticals.length - warnings.length);

  return (
    <Card title={t("anomaly.title")} hint={t("anomaly.cardHint")} hueIndex={hueIndex}>
      {loading && !summary ? (
        <p className="text-sm text-carbon-textSub">{t("dashboard.checking")}</p>
      ) : error || !summary ? (
        // A request that did not arrive says nothing about the backups, so it
        // must not borrow the all-clear's wording.
        <p className="text-sm text-statusWarn">{t("anomaly.loadFailed")}</p>
      ) : !summary.enabled ? (
        <div className="flex flex-col gap-1 text-sm text-carbon-textSub">
          <p>{t("anomaly.off")}</p>
          <Link to="/settings/integrity#anomalies" className="text-accentText hover:underline">
            {t("anomaly.openSettings")}
          </Link>
          {open.length > 0 && (
            <Link to="/anomalies" className="text-accentText hover:underline">
              {t("anomaly.showEarlier")}
            </Link>
          )}
        </div>
      ) : criticals.length === 0 && warnings.length === 0 ? (
        <div className="flex flex-col gap-1">
          <p className="text-sm text-carbon-textSub">{t("anomaly.allClear")}</p>
          {summary.learningItems > 0 && (
            <p className="text-xs text-carbon-textSub">
              {t("anomaly.learningCount", summary.learningItems)}
            </p>
          )}
          {summary.open.info > 0 && (
            <Link to="/anomalies" className="text-xs text-carbon-textMuted hover:underline">
              {t("anomaly.notesCount", summary.open.info)}
            </Link>
          )}
          {summary.evalErrors > 0 && (
            <p className="text-xs text-statusWarn">{t("anomaly.evalErrors", summary.evalErrors)}</p>
          )}
        </div>
      ) : (
        <div className="flex flex-col gap-2">
          <p className="flex flex-wrap gap-x-3 text-xs text-carbon-textSub">
            {summary.open.critical > 0 && <span>{t("anomaly.countCritical", summary.open.critical)}</span>}
            {summary.open.warning > 0 && <span>{t("anomaly.countWarning", summary.open.warning)}</span>}
            {summary.retentionHeld > 0 && <span>{t("anomaly.countHeld", summary.retentionHeld)}</span>}
          </p>
          {[...criticals, ...warnings].map((a) => (
            <AnomalyRow
              key={a.id}
              a={a}
              t={t}
              onAcknowledge={onAcknowledge}
              onExpected={onExpected}
            />
          ))}
          <div className="flex flex-wrap items-baseline gap-x-3">
            {notShown > 0 && (
              <span className="text-xs text-carbon-textSub">{t("anomaly.moreCount", notShown)}</span>
            )}
            <Link to="/anomalies" className="text-xs text-accentText hover:underline">
              {t("anomaly.showAll")}
            </Link>
          </div>
        </div>
      )}
      {summary?.enabled && !summary.ready && (
        <p className="text-xs text-carbon-textMuted">{t("anomaly.checking")}</p>
      )}
    </Card>
  );
}

// The card's data. Summary and rows share one generation, so the counts and
// the listed findings always describe the same pass.
export function AnomaliesBlock({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  const { summary, error: summaryFailed, loading } = useAnomalySummary();
  const { list, error: listFailed } = useOpenAnomalies();

  const settle = useCallback(
    (call: (ids: string[]) => Promise<AnomalyActionResult>) => async (a: AnomalyView) => {
      const res = await call([a.id]);
      if (res.ok) window.dispatchEvent(new Event(ANOMALY_CHANGED_EVENT));
      return res;
    },
    []
  );

  return (
    <AnomaliesCard
      t={t}
      summary={summary}
      open={list}
      loading={loading}
      error={summaryFailed || listFailed}
      hueIndex={hueIndex}
      onAcknowledge={settle((ids) => acknowledgeAnomalies(ids))}
      onExpected={settle((ids) => markAnomaliesExpected(ids))}
    />
  );
}
