import type { useT } from "../../lib/i18n";
import { formatTs } from "../../lib/reltime";

type T = ReturnType<typeof useT>["t"];

/** A backup as the notice compares it: the id a finding names it by, and
 *  when it was taken in Unix seconds. */
export type RestorePointTime = { id: string; at: number };

/** A listed snapshot or dump under the id findings use for it. */
export function restorePointOf(s: { id: string; time: string; original?: string }): RestorePointTime {
  return { id: s.original || s.id, at: Math.floor(Date.parse(s.time) / 1000) };
}

/** The backup closest in time to `at`, or the newest one without a time. */
export function nearestRestorePoint<P extends RestorePointTime>(points: P[], at: number): P | undefined {
  const target = at > 0 ? at : Number.MAX_SAFE_INTEGER;
  let best: P | undefined;
  for (const p of points) {
    if (!best || Math.abs(p.at - target) < Math.abs(best.at - target)) best = p;
  }
  return best;
}

/**
 * MissingRestorePoint speaks up when a finding's restore link names a backup
 * the list no longer holds. Retention may have removed it since the finding
 * was raised, and a list without the promised row reads as if the link had
 * worked.
 */
export function MissingRestorePoint({
  requested,
  requestedAt,
  points,
  t,
}: {
  requested: string;
  requestedAt: number;
  points: RestorePointTime[];
  t: T;
}) {
  if (!requested || points.some((p) => p.id === requested)) return null;
  const nearest = nearestRestorePoint(points, requestedAt);
  const when = requestedAt > 0 ? formatTs(requestedAt) : requested.slice(0, 8);
  return (
    <p role="status" className="py-2 text-xs text-statusWarn">
      {t("restore.missingPoint").replace("{date}", when)}
      {nearest && ` ${t("restore.nearestPoint").replace("{date}", formatTs(nearest.at))}`}
    </p>
  );
}
