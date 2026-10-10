// What BombVault knows about one item and how closely it watches it: the
// item's own sensitivity and notifications, its learning progress, what its
// backups measured, the usual figures of the item and of each series inside
// it, and what was marked as expected.

import { useState } from "react";

import { Badge } from "../Badge";
import { Button } from "../Button";
import { InfoBubble } from "../InfoBubble";
import { ItemAnomalySettings, type AnomalyGlobals } from "../ItemAnomalySettings";
import {
  ANOMALY_CHANGED_EVENT,
  ANOMALY_FAMILY_LABEL,
  anomalyErrorText,
  anomalyLearningText,
  type TranslateAnomaly,
} from "../../lib/anomalies";
import {
  forgetAnomalyExpectation,
  type AnomalyItem,
  type AnomalySeriesInfo,
  type AnomalyView,
} from "../../lib/api";
import { humanBytes } from "../../lib/forecast";
import { isolateLtr } from "../../lib/ltrFragments";
import { formatMillis, formatTs } from "../../lib/reltime";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { SeriesCurve } from "./SeriesCurve";

function typicalSize(t: TranslateAnomaly, sourceBytes: number, resticMs: number): string {
  return t("anomaly.items.typicalSize")
    .replace("{size}", isolateLtr(humanBytes(sourceBytes)))
    .replace("{duration}", isolateLtr(formatMillis(resticMs)));
}

/** How far an item has come, in the words the learning rings use. */
export function learningProgressText(t: TranslateAnomaly, learning: AnomalyItem["learning"]): string {
  if (learning.noData) return t("anomaly.noData");
  if (learning.samples >= learning.needed) return t("anomaly.learningDone");
  return t("anomaly.learningProgress")
    .replace("{n}", learning.samples.toLocaleString())
    .replace("{needed}", learning.needed.toLocaleString());
}

/** The figures of a series that has one rule set rather than all of them. */
function SeriesLine({ t, label, series, ltr }: { t: TranslateAnomaly; label: string; series: AnomalySeriesInfo; ltr?: boolean }) {
  return (
    <div className="flex flex-wrap items-baseline gap-x-3 text-xs text-carbon-textSub">
      <span
        dir={ltr ? "ltr" : undefined}
        className={`min-w-0 text-carbon-text text-start wrap-anywhere${ltr ? " font-mono" : ""}`}
      >
        {label}
      </span>
      <span>{anomalyLearningText(t, series.learning.samples, series.learning.needed, false)}</span>
      {series.typical.sourceBytes !== null && series.typical.resticMs !== null && (
        <span>{typicalSize(t, series.typical.sourceBytes, series.typical.resticMs)}</span>
      )}
      {series.retentionHeld && <span className="text-statusWarn">{t("anomaly.retentionPaused")}</span>}
    </div>
  );
}

function LearningState({ t, item }: { t: TranslateAnomaly; item: AnomalyItem }) {
  const { learning } = item;
  if (!item.scheduled) return <span className="text-xs text-carbon-textSub">{t("anomaly.items.notScheduled")}</span>;
  if (learning.noData) return <span className="text-xs text-carbon-textSub">{t("anomaly.noData")}</span>;
  if (learning.samples >= learning.needed) {
    return (
      <span className="inline-flex items-center gap-2 text-[13px] text-carbon-textSub">
        <i aria-hidden="true" className="h-[7px] w-[7px] rounded-full bg-statusOkSolid" />
        {t("anomaly.learningDone")}
      </span>
    );
  }
  return (
    <span className="inline-flex items-center gap-1.5">
      <Badge tone="active" size="small" wrap>
        {learningProgressText(t, learning)}
      </Badge>
      <InfoBubble
        tip={t("anomaly.items.learningDetail")
          .replace("{newData}", learning.newData.toLocaleString())
          .replace("{source}", learning.source.toLocaleString())
          .replace("{duration}", learning.duration.toLocaleString())
          .replace("{needed}", learning.needed.toLocaleString())}
      />
    </span>
  );
}

export function ItemMonitoring({
  t,
  item,
  globals,
  enabled,
  finding,
}: {
  t: TranslateAnomaly;
  item: AnomalyItem;
  globals: AnomalyGlobals;
  enabled: boolean;
  /** The finding whose quantity the curve shows instead of the item's size. */
  finding?: AnomalyView;
}) {
  const { confirm, confirmDialog } = useConfirm();
  const { push } = useToast();
  const [forgotten, setForgotten] = useState<string[]>([]);

  async function forget(family: string, scopeKind: string, part: string) {
    if (!(await confirm(t("anomaly.expectation.forgetConfirm"), { confirmKey: "anomaly.expectation.forget" }))) {
      return;
    }
    const res = await forgetAnomalyExpectation(item.targetId, family, scopeKind, part);
    if (!res.ok) {
      push(anomalyErrorText(res.code, t), "fail");
      return;
    }
    setForgotten((prev) => [...prev, `${scopeKind}:${part}:${family}`]);
    window.dispatchEvent(new Event(ANOMALY_CHANGED_EVENT));
  }

  const typical = item.typical;
  const expectations = item.expectations.filter((e) => !forgotten.includes(`${e.scopeKind}:${e.part}:${e.family}`));

  return (
    <div className="flex flex-col gap-4">
      <ItemAnomalySettings item={item} enabled={enabled} globals={globals} t={t} />

      <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-1.5">
        <span className="inline-flex items-center gap-1.5 text-sm font-semibold text-carbon-text">
          {t("anomaly.learn.title")}
          <InfoBubble tip={t("anomaly.learningHint").replace("{needed}", item.learning.needed.toLocaleString())} />
        </span>
        <LearningState t={t} item={item} />
      </div>

      <SeriesCurve targetId={item.targetId} finding={finding} t={t} />

      <div className="flex flex-col gap-2">
        {typical.sourceBytes !== null && typical.resticMs !== null && (
          <p className="text-xs text-carbon-textSub">
            {typical.newDataBytes !== null
              ? t("anomaly.items.typical")
                  .replace("{size}", isolateLtr(humanBytes(typical.sourceBytes)))
                  .replace("{newData}", isolateLtr(humanBytes(typical.newDataBytes)))
                  .replace("{duration}", isolateLtr(formatMillis(typical.resticMs)))
              : typicalSize(t, typical.sourceBytes, typical.resticMs)}
          </p>
        )}
        {item.dump && <SeriesLine t={t} label={t("anomaly.items.dumpSeries")} series={item.dump} />}
        {item.datasets.map((series) => (
          <SeriesLine key={series.part} t={t} label={series.part} series={series} ltr />
        ))}
        {item.selectionSince > 0 && (
          <p className="text-xs text-carbon-textSub">
            {t("anomaly.expectation.selectionSince").replace("{date}", formatTs(item.selectionSince))}
          </p>
        )}
      </div>

      {expectations.length > 0 && (
        <div className="flex flex-col gap-2">
          <h3 className="text-[13px] font-semibold text-carbon-textSub">{t("anomaly.markedExpected")}</h3>
          {expectations.map((e) => (
            <div
              key={`${e.scopeKind}:${e.part}:${e.family}`}
              className="flex flex-wrap items-center gap-2 text-xs text-carbon-textSub"
            >
              {/* An expectation of a dump or a dataset says which series it
                  belongs to, or it would read as the item's own. */}
              {e.scopeKind === "zfsds" && (
                <span dir="ltr" className="min-w-0 font-mono text-carbon-text text-start wrap-anywhere">
                  {e.part}
                </span>
              )}
              {e.scopeKind === "dump" && <span className="text-carbon-text">{t("anomaly.items.dumpSeries")}</span>}
              <span>
                {e.ceiling > 0
                  ? t("anomaly.expectation.ceiling")
                      .replace("{family}", t(ANOMALY_FAMILY_LABEL[e.family] ?? "anomaly.family.newData"))
                      .replace("{bytes}", humanBytes(e.ceiling))
                  : t("anomaly.expectation.since")
                      .replace("{family}", t(ANOMALY_FAMILY_LABEL[e.family] ?? "anomaly.family.newData"))
                      .replace("{date}", formatTs(e.sinceAt))}
              </span>
              <Button
                label={t("anomaly.expectation.forget")}
                labelKey="anomaly.expectation.forget"
                onClick={() => void forget(e.family, e.scopeKind, e.part)}
                className="ms-auto"
              />
            </div>
          ))}
        </div>
      )}
      {confirmDialog}
    </div>
  );
}
