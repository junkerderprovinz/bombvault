// A finding's details: what was measured, how that sits against the usual
// range, where its backup differs from the one before, and every way to act
// on it.

import { useNavigate } from "react-router-dom";

import { Badge } from "../Badge";
import { Button } from "../Button";
import { IconCompare, IconEye } from "../glyphs";
import { IconAnomalies } from "../navGlyphs";
import {
  ANOMALY_SEVERITY_LABEL,
  ANOMALY_STATE_LABEL,
  anomalyEntryName,
  anomalyFigures,
  anomalyItemPath,
  anomalyRestorePath,
  anomalySentence,
  anomalySeverityTone,
  type AnomalyFigure,
  type TranslateAnomaly,
} from "../../lib/anomalies";
import { findingHasCurve } from "../../lib/anomalyCurve";
import type { AnomalyView } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { AnomalyWindow } from "./AnomalyWindow";
import { FindingChanges, findingHasChanges } from "./FindingChanges";
import { findingWhen, RetentionNote, SeriesTag, type SettleFindings } from "./FindingLine";
import { SeriesCurve } from "./SeriesCurve";

function Figures({ rows }: { rows: AnomalyFigure[] }) {
  return (
    <dl className="grid grid-cols-[max-content_minmax(0,1fr)] gap-x-6 gap-y-1.5 text-sm max-sm:grid-cols-[minmax(0,2fr)_minmax(0,3fr)] max-sm:gap-x-3">
      {rows.map(([term, value]) => (
        <div key={term} className="contents">
          <dt className="text-carbon-textMuted">{term}</dt>
          <dd className="font-semibold tabular-nums text-carbon-text wrap-anywhere">{value}</dd>
        </div>
      ))}
    </dl>
  );
}

export function FindingWindow({
  a,
  t,
  comparing,
  onCompare,
  onMonitoring,
  onSettle,
  onClose,
}: {
  a: AnomalyView;
  t: TranslateAnomaly;
  /** Whether the comparison with the backup before is showing. */
  comparing: boolean;
  onCompare: () => void;
  /** Opens the item's monitoring, where detection knows the item. */
  onMonitoring?: () => void;
  onSettle: SettleFindings;
  onClose: () => void;
}) {
  const { lang } = useT();
  const navigate = useNavigate();
  const open = a.state === "open";
  const figures = anomalyFigures(a, t, lang, findingWhen);
  const main: AnomalyFigure[] = a.lastGood
    ? [...figures.main, [t("anomaly.detail.lastGood"), findingWhen(a.lastGood.at)]]
    : figures.main;
  const itemPath = a.targetId ? anomalyItemPath(a) : undefined;
  const restorePath = anomalyRestorePath(a);
  const recovered = a.severity === "critical" && a.recoveredAt > 0;
  const closedAt = a.ackedAt || a.resolvedAt;

  const leave = (to: string) => {
    onClose();
    navigate(to);
  };

  return (
    <AnomalyWindow
      title={<span className="min-w-0 normal-case tracking-normal wrap-anywhere">{anomalyEntryName(a, t)}</span>}
      onClose={onClose}
    >
      <div className="flex flex-wrap items-center gap-2 text-sm text-carbon-textMuted">
        <span>{findingWhen(a.lastSeenAt)}</span>
        <SeriesTag a={a} t={t} />
        <span className="ms-auto flex items-center gap-2">
          {!open && (
            <Badge tone={a.state === "resolved" ? "ok" : "neutral"} size="small">
              {t(ANOMALY_STATE_LABEL[a.state])}
            </Badge>
          )}
          <Badge tone={anomalySeverityTone(a.severity)} size="small">
            {t(ANOMALY_SEVERITY_LABEL[a.severity])}
          </Badge>
        </span>
      </div>

      <p className="text-sm text-carbon-text wrap-anywhere">{anomalySentence(a, t, lang)}</p>

      {findingHasCurve(a) && <SeriesCurve targetId={a.targetId} finding={a} t={t} />}

      <Figures rows={main} />
      <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 px-4 py-3">
        <h3 className="text-caption font-semibold uppercase tracking-[.12em] text-carbon-textMuted">
          {t("anomaly.detail.more")}
        </h3>
        <Figures rows={figures.more} />
      </div>

      {recovered && open && (
        <p className="text-xs text-carbon-textSub">
          {t("anomaly.recoveredHint").replace("{date}", findingWhen(a.recoveredAt))}
        </p>
      )}
      {open && a.retentionHeld && <RetentionNote t={t} />}

      {open && comparing && <FindingChanges a={a} t={t} />}
      {!open && (
        <p className="flex flex-wrap items-baseline gap-x-3 text-xs text-carbon-textSub">
          {closedAt > 0 && <span>{t("anomaly.closedAt").replace("{date}", findingWhen(closedAt))}</span>}
          {a.ackNote && (
            <span className="min-w-0 wrap-anywhere">
              {t("anomaly.noteLabel")}: {a.ackNote}
            </span>
          )}
          {a.stillPresent && <span>{t("anomaly.stillPresent")}</span>}
        </p>
      )}

      {open && (
        <div className="flex flex-wrap items-center gap-2 md:justify-end">
          {findingHasChanges(a) && !comparing && (
            <Button
              label={t("anomaly.changes.title")}
              labelKey="anomaly.changes.title"
              glyph={<IconCompare />}
              onClick={onCompare}
              className="glim-btn-wrap"
            />
          )}
          <Button
            label={t("anomaly.action.acknowledge")}
            labelKey="anomaly.action.acknowledge"
            onClick={() => void onSettle([a], "acknowledge")}
            hint={t("anomaly.acknowledgeHint")}
            className="glim-btn-wrap"
          />
          {a.expectable && (
            <Button
              label={t("anomaly.action.expected")}
              labelKey="anomaly.action.expected"
              onClick={() => void onSettle([a], "expected")}
              hint={t("anomaly.expectedHint")}
              className="glim-btn-wrap"
            />
          )}
          {onMonitoring && (
            <Button
              label={t("anomaly.card.monitoring")}
              labelKey="anomaly.card.monitoring"
              glyph={<IconAnomalies />}
              onClick={onMonitoring}
              className="glim-btn-wrap"
            />
          )}
          {itemPath && (
            <Button
              label={t("anomaly.openItem")}
              labelKey="anomaly.openItem"
              glyph={<IconEye />}
              onClick={() => leave(itemPath)}
              className="glim-btn-wrap"
            />
          )}
          {restorePath && a.lastGood && (
            <Button
              label={t("anomaly.action.restoreLastGood").replace("{date}", findingWhen(a.lastGood.at))}
              labelKey="anomaly.action.restoreLastGood"
              tone={a.severity === "critical" ? "accent" : "neutral"}
              onClick={() => leave(restorePath)}
              className="glim-btn-wrap"
            />
          )}
        </div>
      )}
    </AnomalyWindow>
  );
}
