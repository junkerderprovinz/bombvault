// One finding in the list: how bad it is, which item it is about, what
// happened, and what can be done about it. The figures behind it open in a
// window.

import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";

import { Badge } from "../Badge";
import { Button } from "../Button";
import { IconCompare, IconInfo } from "../glyphs";
import { InfoBubble } from "../InfoBubble";
import {
  ANOMALY_SEVERITY_LABEL,
  ANOMALY_STATE_LABEL,
  anomalyEntryName,
  anomalyItemPath,
  anomalyRestorePath,
  anomalySeverityTone,
  anomalyShortLine,
  anomalyWhere,
  type TranslateAnomaly,
} from "../../lib/anomalies";
import type { AnomalySeverity, AnomalyView } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { isolateLtr } from "../../lib/ltrFragments";
import { formatTs } from "../../lib/reltime";
import { findingHasChanges } from "./FindingChanges";

/** Closes findings one way or the other and says whether it went through. */
export type SettleFindings = (list: AnomalyView[], how: "acknowledge" | "expected") => Promise<boolean>;

// The bar on the leading edge, which flips sides with the reading direction.
const EDGE: Record<AnomalySeverity, string> = {
  critical:
    "bg-statusFailBgSoft shadow-[inset_3px_0_0_var(--status-fail-solid)] rtl:shadow-[inset_-3px_0_0_var(--status-fail-solid)]",
  warning:
    "bg-carbon-surface2 shadow-[inset_3px_0_0_var(--status-warn-solid)] rtl:shadow-[inset_-3px_0_0_var(--status-warn-solid)]",
  info: "bg-carbon-surface2 shadow-[inset_3px_0_0_var(--carbon-surface3)] rtl:shadow-[inset_-3px_0_0_var(--carbon-surface3)]",
};

export function findingWhen(unix: number): string {
  return isolateLtr(formatTs(unix));
}

/** The dump or the dataset a finding is about, beside the item's name. */
export function SeriesTag({ a, t }: { a: AnomalyView; t: TranslateAnomaly }) {
  if (a.scopeKind === "dump") {
    return (
      <Badge tone="neutral" size="small" className="shrink-0">
        {t("anomaly.items.dumpSeries")}
      </Badge>
    );
  }
  if (a.scopeKind !== "zfsds" || !a.part) return null;
  return (
    <span dir="ltr" className="min-w-0 font-mono text-xs text-carbon-textSub text-start wrap-anywhere">
      {a.part}
    </span>
  );
}

export function FindingLine({
  a,
  t,
  closed = false,
  lead = false,
  siblings,
  onSettle,
  onDetails,
}: {
  a: AnomalyView;
  t: TranslateAnomaly;
  /** A settled or resolved finding, which offers its details and nothing else. */
  closed?: boolean;
  /** The first finding of the list, whose way out is the page's one filled button. */
  lead?: boolean;
  /** Every finding of the same item in the list, on the first of several. */
  siblings?: AnomalyView[];
  onSettle?: SettleFindings;
  onDetails: (compare: boolean) => void;
}) {
  const { lang } = useT();
  const navigate = useNavigate();
  const [busy, setBusy] = useState(false);

  const name = anomalyEntryName(a, t);
  const itemPath = a.targetId ? anomalyItemPath(a) : undefined;
  const restorePath = a.severity === "critical" ? anomalyRestorePath(a) : null;
  const recovered = a.severity === "critical" && a.recoveredAt > 0;
  const closedAt = a.ackedAt || a.resolvedAt;

  async function settle(list: AnomalyView[], how: "acknowledge" | "expected") {
    if (!onSettle) return;
    setBusy(true);
    try {
      await onSettle(list, how);
    } finally {
      setBusy(false);
    }
  }

  const facts = closed
    ? [
        closedAt > 0 && t("anomaly.closedAt").replace("{date}", findingWhen(closedAt)),
        a.ackNote && `${t("anomaly.noteLabel")}: ${a.ackNote}`,
      ]
    : [
        anomalyWhere(a, t),
        a.occurrences > 1 && t("anomaly.occurrences", a.occurrences),
        a.lastGood &&
          t("anomaly.figure")
            .replace("{label}", t("anomaly.detail.lastGood"))
            .replace("{value}", findingWhen(a.lastGood.at)),
      ];

  return (
    <li
      id={`finding-${a.id}`}
      className={`flex min-w-0 scroll-mt-10 flex-col gap-1.5 rounded-card px-4 pb-3.5 pt-3 max-sm:px-3 ${EDGE[a.severity]}`}
    >
      <div className="flex flex-wrap items-center gap-2">
        {closed && (
          <Badge tone={a.state === "resolved" ? "ok" : "neutral"} size="small" className="shrink-0">
            {t(ANOMALY_STATE_LABEL[a.state])}
          </Badge>
        )}
        <Badge tone={anomalySeverityTone(a.severity)} size="small" className="shrink-0">
          {t(ANOMALY_SEVERITY_LABEL[a.severity])}
        </Badge>
        {itemPath ? (
          <Link
            to={itemPath}
            className="min-w-0 text-[15px] font-semibold text-carbon-text hover:text-accentText hover:underline wrap-anywhere"
          >
            {name}
          </Link>
        ) : (
          <span className="min-w-0 text-[15px] font-semibold text-carbon-text wrap-anywhere">{name}</span>
        )}
        <SeriesTag a={a} t={t} />
        {recovered && !closed && (
          <span className="inline-flex shrink-0 items-center gap-1">
            <Badge tone="ok" size="small">
              {t("anomaly.recovered")}
            </Badge>
            <InfoBubble tip={t("anomaly.recoveredHint").replace("{date}", findingWhen(a.recoveredAt))} />
          </span>
        )}
        <span className="ms-auto shrink-0 text-[13px] text-carbon-textMuted">{findingWhen(a.lastSeenAt)}</span>
      </div>

      <p className="text-sm text-carbon-text wrap-anywhere">{anomalyShortLine(a, t, lang).text}</p>
      <p className="text-[13px] text-carbon-textMuted wrap-anywhere">{facts.filter(Boolean).join(" · ")}</p>
      {closed && a.stillPresent && <p className="text-[13px] text-carbon-textMuted">{t("anomaly.stillPresent")}</p>}
      {!closed && a.retentionHeld && <RetentionNote t={t} />}

      <div className="mt-1.5 flex flex-wrap items-center justify-between gap-2">
        <span className="flex flex-wrap items-center gap-2 max-sm:w-full">
          {!closed && onSettle && (
            <>
              <Button
                label={t("anomaly.action.acknowledge")}
                labelKey="anomaly.action.acknowledge"
                onClick={() => void settle([a], "acknowledge")}
                disabled={busy}
                hint={t("anomaly.acknowledgeHint")}
                className="glim-btn-wrap"
              />
              {a.expectable && (
                <Button
                  label={t("anomaly.action.expected")}
                  labelKey="anomaly.action.expected"
                  onClick={() => void settle([a], "expected")}
                  disabled={busy}
                  hint={t("anomaly.expectedHint")}
                  className="glim-btn-wrap"
                />
              )}
              {siblings && (
                <Button
                  label={t("anomaly.action.acknowledgeAll").replace("{n}", siblings.length.toLocaleString())}
                  labelKey="anomaly.action.acknowledgeAll"
                  onClick={() => void settle(siblings, "acknowledge")}
                  disabled={busy}
                  className="glim-btn-wrap"
                />
              )}
            </>
          )}
        </span>
        <span className="flex flex-wrap items-center justify-end gap-2 max-sm:w-full">
          <Button
            label={t("fleet.details")}
            labelKey="fleet.details"
            glyph={<IconInfo />}
            onClick={() => onDetails(false)}
            className="glim-btn-wrap"
          />
          {!closed && restorePath && a.lastGood ? (
            <Button
              label={t("anomaly.action.restoreLastGood").replace("{date}", findingWhen(a.lastGood.at))}
              labelKey="anomaly.action.restoreLastGood"
              tone={lead ? "accent" : "neutral"}
              onClick={() => navigate(restorePath)}
              className="glim-btn-wrap"
            />
          ) : (
            !closed &&
            findingHasChanges(a) && (
              <Button
                label={t("anomaly.changes.title")}
                labelKey="anomaly.changes.title"
                glyph={<IconCompare />}
                tone={lead ? "accent" : "neutral"}
                onClick={() => onDetails(true)}
                className="glim-btn-wrap"
              />
            )
          )}
        </span>
      </div>
    </li>
  );
}

/** Old backups of the series stay until the finding is settled. */
export function RetentionNote({ t }: { t: TranslateAnomaly }) {
  return (
    <p className="inline-flex items-center gap-1 self-start text-xs text-statusWarn">
      {t("anomaly.retentionKept")}
      <InfoBubble tip={t("anomaly.retentionPaused")} />
    </p>
  );
}
