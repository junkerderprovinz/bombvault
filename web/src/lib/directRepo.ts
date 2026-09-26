import type { OffsiteTarget } from "./api";

type Retention = Pick<OffsiteTarget, "retentionKeepLast" | "retentionKeepDaily" | "retentionKeepWeekly" | "retentionKeepMonthly">;

/** retentionLowered is the server's rule: all zero keeps everything and the
 *  dimensions add up, so any one that shrinks keeps less. */
export function retentionLowered(before: Retention, after: Retention): boolean {
  const values = (r: Retention) => [r.retentionKeepLast, r.retentionKeepDaily, r.retentionKeepWeekly, r.retentionKeepMonthly];
  const b = values(before);
  const a = values(after);
  if (a.every((n) => n === 0)) return false;
  if (b.every((n) => n === 0)) return true;
  return a.some((n, i) => n < b[i]);
}
