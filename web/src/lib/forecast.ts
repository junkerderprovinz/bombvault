// The Storage card's forecast line, built from the "forecast" object in
// GET /api/stats (internal/api/forecast.go). Translation goes through an
// injected resolver, as in activityLog.ts, so this is testable without an
// I18nProvider.

import type { StorageForecast } from "./api";

/** ResolveForecast translates a key and fills its {placeholder} params. The app
 *  passes a closure over useT()'s t; tests pass a stub. */
export type ResolveForecast = (key: string, params?: Record<string, string>) => string;

/** humanBytes formats a byte count with a binary (1024) unit and one decimal,
 *  like every storage figure on the dashboard. Zero and negatives collapse to
 *  "0 B"; a signed display is up to the caller. */
export function humanBytes(n: number): string {
  if (!n || n <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let v = n;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${i === 0 ? v : v.toFixed(1)} ${units[i]}`;
}

/** Below this many weeksToFull the projection renders in the warn token. */
export const FORECAST_WARN_WEEKS = 8;

/** Above this many weeksToFull the projection caps at "full in > 1 year". */
export const FORECAST_CAP_WEEKS = 52;

/** ForecastLine holds the localized segments of the forecast line. A null
 *  segment is not rendered. */
export interface ForecastLine {
  /** Growth trend, e.g. "Growing 1.2 GB/week" (or the shrinking variant). */
  growth: string | null;
  /** Time-to-full projection, e.g. "Repo volume full in ~6 weeks". */
  projection: string | null;
  /** Free space on the repo volume, e.g. "1.9 TB free". */
  free: string | null;
  /** True when weeksToFull < FORECAST_WARN_WEEKS; the projection is then shown
   *  in the statusWarn colour. */
  warn: boolean;
}

/**
 * buildForecastLine maps a forecast object to its segments, or returns null
 * when it carries nothing to show. Negative growth uses the shrinking wording
 * with the unsigned amount. The backend sends weeksToFull only for positive
 * growth and known free space; it is rounded to whole weeks with a floor of 1,
 * which has its own count-neutral key, and reads "full in > 1 year" beyond
 * FORECAST_CAP_WEEKS.
 */
export function buildForecastLine(
  forecast: StorageForecast | null | undefined,
  resolve: ResolveForecast
): ForecastLine | null {
  if (!forecast) return null;
  const growthBytes = forecast.growthBytesPerWeek;
  const freeBytes = forecast.freeBytes;
  const weeksToFull = forecast.weeksToFull;

  const growth =
    growthBytes == null
      ? null
      : growthBytes < 0
        ? resolve("dashboard.forecastShrink", { bytes: humanBytes(-growthBytes) })
        : resolve("dashboard.forecastGrowth", { bytes: humanBytes(growthBytes) });

  let projection: string | null = null;
  let warn = false;
  if (weeksToFull != null) {
    warn = weeksToFull < FORECAST_WARN_WEEKS;
    if (weeksToFull > FORECAST_CAP_WEEKS) {
      projection = resolve("dashboard.forecastFullOverYear");
    } else {
      const weeks = Math.max(1, Math.round(weeksToFull));
      projection =
        weeks === 1
          ? resolve("dashboard.forecastFullOneWeek")
          : resolve("dashboard.forecastFull", { weeks: String(weeks) });
    }
  }

  const free = freeBytes == null ? null : resolve("dashboard.forecastFree", { bytes: humanBytes(freeBytes) });

  if (growth == null && projection == null && free == null) return null;
  return { growth, projection, free, warn };
}
