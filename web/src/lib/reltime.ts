import type { useT } from "./i18n";

type T = ReturnType<typeof useT>["t"];

/**
 * relativeTime renders a unix timestamp as a fully written-out, localized
 * "time ago" string (e.g. "5 minutes ago" / "vor 5 Minuten"), shared by the
 * dashboard protection card, the runs list and the drills line so the wording
 * is consistent everywhere. Singular counts (n === 1) use dedicated keys so the
 * grammar stays correct ("1 minute ago" vs "5 minutes ago").
 */
export function relativeTime(t: T, unix: number): string {
  const diff = Math.floor((Date.now() - unix * 1000) / 1000);
  if (diff < 60) return t("time.justNow");
  if (diff < 3600) {
    const n = Math.floor(diff / 60);
    return n === 1 ? t("time.minuteAgo") : t("time.minutesAgo").replace("{n}", String(n));
  }
  if (diff < 86400) {
    const n = Math.floor(diff / 3600);
    return n === 1 ? t("time.hourAgo") : t("time.hoursAgo").replace("{n}", String(n));
  }
  const n = Math.floor(diff / 86400);
  return n === 1 ? t("time.dayAgo") : t("time.daysAgo").replace("{n}", String(n));
}

/**
 * formatTs renders a unix timestamp as a localized date + time, or "—" when the
 * value is missing. Shared by the dashboard cards, the runs list and the
 * per-domain recent-runs list so absolute times read the same everywhere.
 */
export function formatTs(unix: number | null | undefined): string {
  if (!unix) return "—";
  return new Date(unix * 1000).toLocaleString();
}

/**
 * formatDuration renders a whole-second span compactly and plural-free
 * (e.g. "12s", "3m 5s", "1h 2m"). A negative or non-finite input yields ""
 * so a missing/older start time never produces a broken duration.
 */
export function formatDuration(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return "";
  const s = Math.floor(seconds);
  if (s < 60) return `${s}s`;
  if (s < 3600) return `${Math.floor(s / 60)}m ${s % 60}s`;
  return `${Math.floor(s / 3600)}h ${Math.floor((s % 3600) / 60)}m`;
}

/**
 * formatClockTime renders a unix timestamp as a fixed 24-hour local clock face
 * ("HH:MM" or "HH:MM:SS"), independent of the browser's locale. The dashboard
 * activity log (a flat, docker-logs-style line list) wants a stable, always
 * 24-hour timestamp per line — not a locale-dependent 12/24-hour format that
 * `toLocaleTimeString` would silently vary by browser language.
 */
export function formatClockTime(unix: number, withSeconds = true): string {
  const d = new Date(unix * 1000);
  const hh = String(d.getHours()).padStart(2, "0");
  const mm = String(d.getMinutes()).padStart(2, "0");
  if (!withSeconds) return `${hh}:${mm}`;
  const ss = String(d.getSeconds()).padStart(2, "0");
  return `${hh}:${mm}:${ss}`;
}
