import type { useT } from "./i18n";

type T = ReturnType<typeof useT>["t"];

/**
 * relativeTime renders a unix timestamp as a localized "time ago" string such
 * as "5 minutes ago".
 */
export function relativeTime(t: T, unix: number): string {
  const diff = Math.floor((Date.now() - unix * 1000) / 1000);
  if (diff < 60) return t("time.justNow");
  if (diff < 3600) {
    const n = Math.floor(diff / 60);
    return t("time.minutesAgo", n);
  }
  if (diff < 86400) {
    const n = Math.floor(diff / 3600);
    return t("time.hoursAgo", n);
  }
  const n = Math.floor(diff / 86400);
  return t("time.daysAgo", n);
}

/**
 * formatTs renders a unix timestamp as a localized date and time, or a dash
 * when the value is missing.
 */
export function formatTs(unix: number | null | undefined): string {
  if (!unix) return "—";
  return new Date(unix * 1000).toLocaleString();
}

/**
 * formatDuration renders a whole-second span compactly and plural-free, e.g.
 * "12s", "3m 5s" or "1h 2m". A negative or non-finite input yields "".
 */
export function formatDuration(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return "";
  const s = Math.floor(seconds);
  if (s < 60) return `${s}s`;
  if (s < 3600) return `${Math.floor(s / 60)}m ${s % 60}s`;
  return `${Math.floor(s / 3600)}h ${Math.floor((s % 3600) / 60)}m`;
}

/**
 * elapsedSince renders the span from a backend-stamped `startedAt` (Unix
 * seconds) to `nowMs` (epoch milliseconds) via formatDuration. It returns ""
 * when `startedAt` is missing, not positive, or in the future (clock skew).
 *
 * The backend sends 0 for an unknown start (see progress.go's Event), and a 0
 * would otherwise render as the age of the Unix epoch; formatDuration only
 * rejects a start in the future.
 */
/**
 * formatMillis is formatDuration for a span measured in milliseconds. Under a
 * second it keeps the milliseconds, which whole seconds would round to "0s".
 */
export function formatMillis(ms: number): string {
  if (Number.isFinite(ms) && ms >= 0 && ms < 1000) return `${Math.round(ms)}ms`;
  return formatDuration(ms / 1000);
}

export function elapsedSince(startedAt: number | undefined, nowMs: number): string {
  if (typeof startedAt !== "number" || !Number.isFinite(startedAt) || startedAt <= 0) return "";
  return formatDuration((nowMs - startedAt * 1000) / 1000);
}

/**
 * formatClockTime renders a unix timestamp as a 24-hour local clock ("HH:MM" or
 * "HH:MM:SS") whatever the browser's locale, so every line of the activity log
 * has the same shape.
 */
export function formatClockTime(unix: number, withSeconds = true): string {
  const d = new Date(unix * 1000);
  const hh = String(d.getHours()).padStart(2, "0");
  const mm = String(d.getMinutes()).padStart(2, "0");
  if (!withSeconds) return `${hh}:${mm}`;
  const ss = String(d.getSeconds()).padStart(2, "0");
  return `${hh}:${mm}:${ss}`;
}
