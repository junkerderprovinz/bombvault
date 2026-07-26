// ---------------------------------------------------------------------------
// forecast — pure formatting/selection for the Storage card's forecast line
// (the "forecast" object riding GET /api/stats; backend contract in
// internal/api/forecast.go). Framework-free like activityLog.ts: the only i18n
// dependency is an injected resolver, so the growth/projection/free selection
// is unit-testable without a live I18nProvider.
// ---------------------------------------------------------------------------

import type { StorageForecast } from "./api";

/**
 * Resolves a translation key (+ optional {placeholder} params) into a
 * localized, interpolated string — same seam as activityLog's ResolveName.
 * The real implementation closes over `useT()`'s `t`; tests pass a stub.
 */
export type ResolveForecast = (key: string, params?: Record<string, string>) => string;

/** humanBytes formats a byte count with a binary (1024) unit and one decimal.
 *  The shared formatter for the dashboard's storage figures (moved out of
 *  Dashboard.tsx so the forecast line reads exactly like the size column).
 *  Zero and negatives collapse to "0 B" — a signed display is the caller's
 *  job (buildForecastLine picks growth vs shrink wording instead). */
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

/**
 * The rendered segments of the Storage card's compact forecast line. Each
 * segment is already localized; a null segment is simply not rendered. `warn`
 * marks a near-term projection (weeksToFull < FORECAST_WARN_WEEKS) — the
 * component then paints the projection segment with the statusWarn text token.
 */
export interface ForecastLine {
  /** Growth trend, e.g. "Growing 1.2 GB/week" (or the shrinking variant). */
  growth: string | null;
  /** Time-to-full projection, e.g. "Repo volume full in ~6 weeks". */
  projection: string | null;
  /** Free space on the repo volume, e.g. "1.9 TB free". */
  free: string | null;
  /** True when weeksToFull < FORECAST_WARN_WEEKS. */
  warn: boolean;
}

/**
 * buildForecastLine maps a /api/stats forecast object to the display segments,
 * or null when there is nothing to show (absent/null forecast, or one carrying
 * no known field) — the card then renders no forecast line at all.
 *
 *   - growth: present whenever growthBytesPerWeek is known. Negative growth
 *     picks the "shrinking" wording with the unsigned magnitude; zero reads as
 *     growing "0 B"/week (flat).
 *   - projection: present whenever weeksToFull is known (the backend only
 *     sends it for positive growth + known free space). Rounded to whole
 *     weeks with a floor of 1 ("~1 week" has its own count-neutral key), and
 *     capped: beyond FORECAST_CAP_WEEKS it reads "full in > 1 year".
 *   - free: present whenever freeBytes is known.
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
