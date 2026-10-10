// What an item's backups measured, as a line over the range detection holds
// usual for it. Opened from a finding, the run that finding was raised on is
// marked.

import { useEffect, useState } from "react";

import { Button } from "../Button";
import { InfoBubble } from "../InfoBubble";
import { anomalyDeviation, anomalySentence, type TranslateAnomaly } from "../../lib/anomalies";
import {
  CURVE_BOX,
  curveLayout,
  curvePath,
  curveSource,
  type CurveQuantity,
  type CurveSource,
} from "../../lib/anomalyCurve";
import { getAnomalyItemSeries, type AnomalySeries, type AnomalyView } from "../../lib/api";
import { humanBytes } from "../../lib/forecast";
import { useT, type TranslationKey } from "../../lib/i18n";
import { isolateLtr } from "../../lib/ltrFragments";
import { formatMillis } from "../../lib/reltime";

type Tone = "fail" | "warn" | "accent" | "quiet";

const STROKE: Record<Tone, string> = {
  fail: "stroke-statusFailSolid",
  warn: "stroke-statusWarnSolid",
  accent: "stroke-accent",
  quiet: "stroke-carbon-textMuted",
};
const FILL: Record<Tone, string> = {
  fail: "fill-statusFailSolid",
  warn: "fill-statusWarnSolid",
  accent: "fill-accent",
  quiet: "fill-carbon-surface2",
};
const INK: Record<Tone, string> = {
  fail: "fill-statusFail",
  warn: "fill-statusWarn",
  accent: "fill-accentText",
  quiet: "fill-carbon-textSub",
};

function titleKey({ series, quantity }: CurveSource): TranslationKey {
  const dump = series.scopeKind === "dump";
  switch (quantity.quantity) {
    case "newDataBytes":
      return "anomaly.curve.newData";
    case "sourceFiles":
      return "anomaly.curve.sourceFiles";
    case "resticMs":
      return dump ? "anomaly.curve.dumpDuration" : "anomaly.curve.duration";
    default:
      return dump ? "anomaly.curve.dumpBytes" : "anomaly.curve.sourceBytes";
  }
}

function formatValue(quantity: CurveQuantity, value: number): string {
  if (quantity === "resticMs") return isolateLtr(formatMillis(value));
  if (quantity === "sourceFiles") return isolateLtr(Math.round(value).toLocaleString());
  return isolateLtr(humanBytes(value));
}

function shortDate(unix: number): string {
  return new Date(unix * 1000).toLocaleDateString(undefined, { day: "2-digit", month: "2-digit" });
}

/** The usual range in words, or how long it takes until there is one. */
function rangeText({ quantity: q }: CurveSource, t: TranslateAnomaly): string | null {
  if (q.learning && !q.band) return t("anomaly.curve.learning").replace("{needed}", q.needed.toLocaleString());
  const low = q.band?.low ? formatValue(q.quantity, q.band.low.threshold) : null;
  const high = q.band?.high ? formatValue(q.quantity, q.band.high.threshold) : null;
  if (low && high) return t("anomaly.curve.between").replace("{low}", low).replace("{high}", high);
  if (high) return t("anomaly.curve.upTo").replace("{value}", high);
  if (low) return t("anomaly.curve.above").replace("{value}", low);
  return null;
}

type Load = { state: "loading" } | { state: "failed" } | { state: "ready"; series: AnomalySeries[] };

export function SeriesCurve({
  targetId,
  finding,
  t,
}: {
  targetId: string;
  /** The finding whose quantity and run the curve shows. */
  finding?: AnomalyView;
  t: TranslateAnomaly;
}) {
  const [load, setLoad] = useState<Load>({ state: "loading" });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let active = true;
    setLoad({ state: "loading" });
    getAnomalyItemSeries(targetId)
      .then((res) => {
        if (active) setLoad(res.ok ? { state: "ready", series: res.series } : { state: "failed" });
      })
      .catch(() => {
        if (active) setLoad({ state: "failed" });
      });
    return () => {
      active = false;
    };
  }, [targetId, attempt]);

  if (load.state === "loading") {
    return (
      <p role="status" className="text-xs text-carbon-textMuted">
        {t("dashboard.checking")}
      </p>
    );
  }
  if (load.state === "failed") {
    return (
      <div className="flex flex-wrap items-center gap-2">
        <p className="text-xs text-statusWarn">{t("anomaly.curve.loadFailed")}</p>
        <Button label={t("anomaly.retry")} labelKey="anomaly.retry" onClick={() => setAttempt((n) => n + 1)} />
      </div>
    );
  }
  const source = curveSource(load.series, finding);
  return source ? <Curve source={source} finding={finding} t={t} /> : null;
}

function Curve({ source, finding, t }: { source: CurveSource; finding?: AnomalyView; t: TranslateAnomaly }) {
  const { lang } = useT();
  const { series, quantity } = source;
  const { left, right, axis, top, bottom } = CURVE_BOX;
  const layout = curveLayout(quantity, series.failed, finding?.runId);
  const { line, marked } = layout;
  const last = line.length - 1;

  const settled = !!finding && finding.state === "open" && finding.recoveredAt > 0;
  const tone: Tone =
    !finding || finding.state !== "open" || settled
      ? "quiet"
      : finding.severity === "critical"
        ? "fail"
        : finding.severity === "warning"
          ? "warn"
          : "accent";
  // The line up to the marked run keeps the accent, and what follows takes
  // the finding's colour, so the run reads as the point where it left.
  const loud = tone !== "quiet" && marked >= 0;
  const deviation = finding && marked >= 0 ? anomalyDeviation(finding, t, lang) : null;
  const range = rangeText(source, t);
  const title = t(titleKey(source));

  let label = null;
  if (deviation) {
    const [x, y] = line[marked];
    const onLeft = x > CURVE_BOX.width / 2;
    const at = { x: onLeft ? x - 14 : x + 14, textAnchor: onLeft ? ("end" as const) : ("start" as const) };
    // A run that stayed near the one before it would put the figure on the
    // line, so there the figure stands above both.
    const before = line[Math.max(marked - 1, 0)][1];
    const beside = Math.abs(y - before) < 26 ? Math.min(y, before) - 24 : y + 4;
    const baseline = Math.min(Math.max(beside, top + 10), bottom - 12);
    label = (
      <g className="glim-curve-mark">
        <text {...at} y={baseline} className={`text-[15px] font-bold tabular-nums ${INK[loud ? tone : "quiet"]}`}>
          {deviation.figure}
        </text>
        <text {...at} y={baseline + 14} className="fill-carbon-textSub text-[11.5px]">
          {deviation.versus}
        </text>
      </g>
    );
  }

  return (
    <div className="flex min-w-0 flex-col gap-2 rounded-card bg-carbon-surface2 px-2 py-3 sm:px-4">
      <div className="flex flex-wrap items-center gap-1.5 px-1.5 text-[13px] font-semibold text-carbon-textSub sm:px-0">
        <span>{title}</span>
        {series.scopeKind === "zfsds" && (
          <span dir="ltr" className="min-w-0 font-mono font-normal text-start wrap-anywhere">
            {series.part}
          </span>
        )}
        <InfoBubble tip={t("anomaly.curve.hint")} />
      </div>

      {/* The time axis runs left to right in every language, and an anchor
          would flip with the reading direction. */}
      <svg
        viewBox={`0 0 ${CURVE_BOX.width} ${CURVE_BOX.height}`}
        role="img"
        aria-label={finding ? `${title}. ${anomalySentence(finding, t, lang)}` : title}
        className="mx-auto block w-full max-w-[26rem] overflow-visible [direction:ltr]"
      >
        {layout.band && (
          <rect
            data-part="band"
            x={left - 6}
            y={layout.band.y}
            width={right - left + 12}
            height={layout.band.height}
            rx="5"
            className="fill-carbon-surface3"
          />
        )}
        <path d={`M${left - 6} ${axis}H${right + 6}`} className="stroke-carbon-surface3" />
        {layout.ticks.map((tick) => (
          <text
            key={tick.x}
            x={tick.x}
            y={axis + 16}
            textAnchor={tick.anchor}
            className="fill-carbon-textMuted text-[10.5px] tabular-nums"
          >
            {shortDate(tick.at)}
          </text>
        ))}
        {layout.toLearn.map(([x, y]) => (
          <circle
            key={x}
            data-part="to-learn"
            cx={x}
            cy={y}
            r="3.5"
            strokeDasharray="2 3"
            className="fill-none stroke-carbon-textMuted"
          />
        ))}

        <path
          data-part="line"
          d={curvePath(loud ? line.slice(0, marked) : line)}
          pathLength="1"
          className="glim-curve-line fill-none stroke-accent stroke-[2.2] [stroke-linecap:round] [stroke-linejoin:round]"
        />
        {loud && (
          <path
            data-part="jump"
            d={curvePath(line.slice(Math.max(marked - 1, 0)))}
            pathLength="1"
            className={`glim-curve-line glim-curve-jump fill-none stroke-[2.2] [stroke-linecap:round] [stroke-linejoin:round] ${STROKE[tone]}`}
          />
        )}

        <g className="glim-curve-mark">
          {layout.crosses.map(([x, y], i) => (
            <path
              key={i}
              data-part="failed"
              d="M-4 -4l8 8m0 -8l-8 8"
              transform={`translate(${x} ${y})`}
              className="fill-none stroke-carbon-text stroke-[2.2] [stroke-linecap:round]"
            />
          ))}
          {marked !== last && (
            <circle
              cx={line[last][0]}
              cy={line[last][1]}
              r="3.5"
              className={`stroke-carbon-surface2 stroke-2 ${settled ? "fill-statusOkSolid" : "fill-accent"}`}
            />
          )}
          {marked >= 0 && (
            <circle
              data-part="marked"
              cx={line[marked][0]}
              cy={line[marked][1]}
              r={loud ? 5 : 4.5}
              className={`stroke-2 ${FILL[loud ? tone : "quiet"]} ${loud ? "stroke-carbon-surface2" : STROKE.quiet}`}
            />
          )}
        </g>
        {label}
      </svg>

      <div className="flex flex-wrap justify-center gap-x-4 gap-y-1 text-xs text-carbon-textMuted">
        {range && (
          <span className="inline-flex items-center gap-1.5">
            <i
              aria-hidden="true"
              className={`h-2.5 w-3.5 flex-none rounded-[3px] ${
                layout.band ? "bg-carbon-surface3" : "shadow-[inset_0_0_0_1px_var(--carbon-text-muted)]"
              }`}
            />
            {t("anomaly.figure").replace("{label}", t("anomaly.curve.range")).replace("{value}", range)}
          </span>
        )}
        <span className="inline-flex items-center gap-1.5">
          <i aria-hidden="true" className="h-[3px] w-3.5 flex-none rounded-sm bg-accent" />
          {t("anomaly.detail.observed")}
        </span>
        {settled && (
          <span className="inline-flex items-center gap-1.5">
            <i aria-hidden="true" className="h-2 w-2 flex-none rounded-full bg-statusOkSolid" />
            {t("anomaly.curve.settled")}
          </span>
        )}
      </div>
    </div>
  );
}
