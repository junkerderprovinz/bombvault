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
export function Vessel({ capacity }: { capacity: StorageLocationCapacity }) {
  const { t } = useT();
  const id = useId();
  const fill = fillOf(capacity);
  const measured = fill.state === "measured";
  const tip = tipText(t, fill);
  const bubble = useTipBubble(tip);
  const level = measured ? FLOOR - fill.share * (FLOOR - BRIM) : UNSIZED_LEVEL;
  const warn = measured && fill.warn;
  const top = measured ? forecastText(t, fill.weeks) || t("storage.vessel.full") : t("storage.vessel.unknown");
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
        className={`glim-pic inline-flex min-w-0 max-w-full items-stretch gap-2 rounded-control text-carbon-textSub${warn ? " glim-vessel-warn" : ""}`}
      >
        <svg viewBox="0 0 34 40" width="34" height="40" aria-hidden="true" className="flex-none overflow-visible">
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
              <path
                className="glim-vessel-liquid"
                d={`M-6 ${level}q3 -1.6 6 0t6 0 6 0 6 0 6 0 6 0 6 0 6 0V40H-6z`}
              />
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
          <span className={warn ? "text-statusWarn" : "text-carbon-textMuted"}>{top}</span>
          <span className="text-sm tabular-nums text-carbon-textSub">{amount}</span>
        </span>
      </span>
      {bubble.bubble}
    </>
  );
}
