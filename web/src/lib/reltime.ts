import type { useT } from "./i18n";

type T = ReturnType<typeof useT>["t"];

/**
 * relativeTime renders a unix timestamp as a short, localized "time ago" string
 * (e.g. "5m ago" / "vor 5 Min."), shared by the dashboard protection card, the
 * runs list and the drills line so the wording is consistent everywhere.
 */
export function relativeTime(t: T, unix: number): string {
  const diff = Math.floor((Date.now() - unix * 1000) / 1000);
  if (diff < 60) return t("time.justNow");
  if (diff < 3600) return t("time.minutesAgo").replace("{n}", String(Math.floor(diff / 60)));
  if (diff < 86400) return t("time.hoursAgo").replace("{n}", String(Math.floor(diff / 3600)));
  return t("time.daysAgo").replace("{n}", String(Math.floor(diff / 86400)));
}
