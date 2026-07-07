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
