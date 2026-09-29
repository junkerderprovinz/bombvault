// What BombVault knows about one item and how closely it watches it: learning
// progress, the usual figures of the item and of each series inside it, what
// was marked as expected, and the item's own sensitivity and notifications.

import { useState } from "react";

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
import { forgetAnomalyExpectation, type AnomalyItem, type AnomalySeriesInfo } from "../../lib/api";
import { humanBytes } from "../../lib/forecast";
import { isolateLtr } from "../../lib/ltrFragments";
import { formatMillis, formatTs } from "../../lib/reltime";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { useRemoteView } from "../../lib/remoteView";

function typicalSize(t: TranslateAnomaly, sourceBytes: number, resticMs: number): string {
  return t("anomaly.items.typicalSize")
    .replace("{size}", isolateLtr(humanBytes(sourceBytes)))
    .replace("{duration}", isolateLtr(formatMillis(resticMs)));
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

export function ItemMonitoring({
  t,
  item,
  globals,
  enabled,
  id,
}: {
  t: TranslateAnomaly;
  item: AnomalyItem;
  globals: AnomalyGlobals;
  enabled: boolean;
  id?: string;
}) {
  const { confirm, confirmDialog } = useConfirm();
  const { push } = useToast();
  const { remote } = useRemoteView();
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

  return (
    <div id={id} className="flex flex-col gap-2 glim-content-fade">
      <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1 text-xs text-carbon-textSub">
        {!item.scheduled ? (
          <span>{t("anomaly.items.notScheduled")}</span>
        ) : (
          <span className="inline-flex items-center gap-1">
            {anomalyLearningText(t, item.learning.samples, item.learning.needed, item.learning.noData)}
            <InfoBubble
              tip={t("anomaly.items.learningDetail")
                .replace("{newData}", item.learning.newData.toLocaleString())
                .replace("{source}", item.learning.source.toLocaleString())
                .replace("{duration}", item.learning.duration.toLocaleString())
                .replace("{needed}", item.learning.needed.toLocaleString())}
            />
          </span>
        )}
        {typical.sourceBytes !== null && typical.resticMs !== null && (
          <span>
            {typical.newDataBytes !== null
              ? t("anomaly.items.typical")
                  .replace("{size}", isolateLtr(humanBytes(typical.sourceBytes)))
                  .replace("{newData}", isolateLtr(humanBytes(typical.newDataBytes)))
                  .replace("{duration}", isolateLtr(formatMillis(typical.resticMs)))
              : typicalSize(t, typical.sourceBytes, typical.resticMs)}
          </span>
        )}
      </div>

      {item.dump && <SeriesLine t={t} label={t("anomaly.items.dumpSeries")} series={item.dump} />}
      {item.datasets.map((series) => (
        <SeriesLine key={series.part} t={t} label={series.part} series={series} ltr />
      ))}

      {item.selectionSince > 0 && (
        <p className="text-xs text-carbon-textSub">
          {t("anomaly.expectation.selectionSince").replace("{date}", formatTs(item.selectionSince))}
        </p>
      )}

      {item.expectations
        .filter((e) => !forgotten.includes(`${e.scopeKind}:${e.part}:${e.family}`))
        .map((e) => (
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
            {/* Forgetting an expectation changes how detection judges this
                item from here on, the same as its sensitivity below, so it
                stays local with it. */}
            {!remote && (
              <Button
                label={t("anomaly.expectation.forget")}
                labelKey="anomaly.expectation.forget"
                onClick={() => void forget(e.family, e.scopeKind, e.part)}
              />
            )}
          </div>
        ))}

      {!remote && <ItemAnomalySettings item={item} enabled={enabled} globals={globals} t={t} />}
      {confirmDialog}
    </div>
  );
}
