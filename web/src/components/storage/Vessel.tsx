import { useId, type CSSProperties } from "react";
import type { StorageLocationCapacity } from "../../lib/api";
import { FORECAST_CAP_WEEKS } from "../../lib/forecast";
import { useT } from "../../lib/i18n";
import { amountText, fillOf, type Fill } from "../../lib/storageLocations";
import { useTipBubble } from "../../lib/useTipBubble";

type T = ReturnType<typeof useT>["t"];

const BRIM = 8;
const FLOOR = 35.5;
// Where the liquid stands in a vessel nobody knows the size of.
const UNSIZED_LEVEL = 27;
// A wave running up a wall, drawn at the left one and mirrored at the right.
const CREST = "M-5 .6V-1H0C.9 -.55 2.7 -.03 6 0V.6z";
// The quiet bubbles and the ones that burst above the surface, as x and
// radius. The stylesheet times each by its place among its siblings.
const BUBBLES = [
  [10.5, 0.5],
  [16.5, 0.45],
  [24.5, 0.7],
];
const POPS = [
  [13.5, 0.7],
  [19, 0.6],
  [21, 0.65],
];

function forecastText(t: T, weeks: number | undefined): string {
  if (weeks === undefined) return "";
  if (weeks > FORECAST_CAP_WEEKS) return t("storage.vessel.fullOverYear");
  return t("storage.vessel.fullInWeeks", weeks);
}

function tipText(t: T, fill: Fill): string {
  if (fill.state === "unsupported") return t("storage.vessel.unsupportedTip");
  if (fill.state === "unmeasured") return t("storage.vessel.unmeasuredTip");
  const pct = String(Math.round(fill.share * 100));
  const forecast = forecastText(t, fill.weeks);
  return forecast
    ? t("storage.vessel.usedForecast").replace("{pct}", pct).replace("{full}", forecast)
    : t("storage.vessel.used").replace("{pct}", pct);
}

/**
 * Vessel shows how full a storage location is: the liquid stands at the used
 * share and a dashed line marks where the location is full. A location that
 * reports no size has no such line and open walls above the liquid.
 */
export function Vessel({
  capacity,
  forecast = true,
}: {
  capacity: StorageLocationCapacity;
  /** Whether the line at the brim says when the location is full. Off where
   *  the forecast stands beside the vessel already. */
  forecast?: boolean;
}) {
  const { t } = useT();
  const id = useId();
  const fill = fillOf(capacity);
  const measured = fill.state === "measured";
  const tip = tipText(t, fill);
  const bubble = useTipBubble(tip);
  const level = measured ? FLOOR - fill.share * (FLOOR - BRIM) : UNSIZED_LEVEL;
  const warn = measured && fill.warn;
  const top = measured
    ? (forecast && forecastText(t, fill.weeks)) || t("storage.vessel.full")
    : t("storage.vessel.unknown");
  const amount = amountText(t, capacity);
  const wallsFrom = measured ? 3 : 17;

  return (
    <>
      <span
        ref={bubble.ref}
        tabIndex={0}
        aria-label={tip}
        aria-describedby={bubble.describedBy}
        {...bubble.handlers}
        className={`glim-pic glim-vessel inline-flex min-w-0 max-w-full items-stretch gap-2 rounded-control text-carbon-textSub${warn ? " glim-vessel-warn" : ""}`}
      >
        <svg
          viewBox="0 0 34 40"
          width="34"
          height="40"
          aria-hidden="true"
          className="flex-none overflow-visible"
          style={{ "--liquid-depth": (FLOOR - level).toFixed(2) } as CSSProperties}
        >
          <clipPath id={`${id}-in`}>
            <rect className="glim-vessel-shape" x="7.5" y="-8" width="19" height="43.5" rx="4.5" />
          </clipPath>
          <clipPath id={`${id}-top`}>
            <rect x="0" y="3" width="34" height="37" />
          </clipPath>
          <clipPath id={`${id}-walls`}>
            <rect x="0" y={wallsFrom} width="34" height={40 - wallsFrom} />
          </clipPath>
          <rect
            className="glim-vessel-shape glim-vessel-back"
            x="7.5"
            y="-8"
            width="19"
            height="43.5"
            rx="4.5"
            clipPath={`url(#${id}-top)`}
          />
          <g clipPath={`url(#${id}-in)`}>
            <g className="glim-vessel-rise" style={{ "--vessel-rise": `${FLOOR - level + 3}px` } as CSSProperties}>
              <g transform={`translate(0 ${level})`}>
                <path className="glim-vessel-liquid glim-vessel-liquid-back glim-slosh-back" d="M-20 -.5H54V45H-20z" />
                <g className="glim-slosh">
                  <path className="glim-vessel-liquid" d="M-20 0H54V45H-20z" />
                  <path className="glim-vessel-liquid glim-slosh-hump" d="M11 .6V0C13 0 14.5 -1 17 -1S21 0 23 0V.6z" />
                  <g transform="translate(7.5 0)">
                    <path className="glim-vessel-liquid glim-slosh-crest glim-slosh-crest-left" d={CREST} />
                  </g>
                  <g transform="translate(26.5 0) scale(-1 1)">
                    <path className="glim-vessel-liquid glim-slosh-crest glim-slosh-crest-right" d={CREST} />
                  </g>
                </g>
                <g>
                  {BUBBLES.map(([cx, r]) => (
                    <circle key={cx} className="glim-slosh-bubble" cx={cx} cy="0" r={r} />
                  ))}
                </g>
                <g>
                  {POPS.map(([cx, r]) => (
                    <g key={cx} className="glim-slosh-pop">
                      <circle className="glim-slosh-pop-bubble" cx={cx} cy="0" r={r} />
                      <circle className="glim-slosh-pop-ring" cx={cx} cy="0" r={r} />
                      <circle className="glim-slosh-pop-drop" cx={cx} cy="0" r=".3" />
                      <circle className="glim-slosh-pop-drop" cx={cx} cy="0" r=".3" />
                      <circle className="glim-slosh-pop-drop" cx={cx} cy="0" r=".3" />
                    </g>
                  ))}
                </g>
              </g>
            </g>
          </g>
          <rect
            className="glim-vessel-shape glim-vessel-glass"
            x="6"
            y="-8"
            width="22"
            height="45"
            rx="6"
            clipPath={`url(#${id}-walls)`}
          />
          {measured ? (
            <path className="glim-vessel-full" d={`M2 ${BRIM}H32`} />
          ) : (
            <path className="glim-vessel-glass glim-vessel-glass-open" d="M6 15V3M28 15V3" />
          )}
        </svg>
        <span className="flex min-w-0 flex-col justify-between text-start text-xs leading-4 md:min-w-[10.5rem]">
          <span className={warn && forecast ? "text-statusWarn" : "text-carbon-textMuted"}>{top}</span>
          <span className="text-sm tabular-nums text-carbon-textSub">{amount}</span>
        </span>
      </span>
      {bubble.bubble}
    </>
  );
}
