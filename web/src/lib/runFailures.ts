import type { Run } from "./api";

// The kinds the error badge counts, and the outcome each one can clear. A dump
// and an import stand apart from the item's backup, so a failed dump does not
// hide a failed backup of the same container. A saved dump is left out: writing
// a copy of an old dump into a folder says nothing about the database.
const ERROR_CLASSES: Record<string, string> = {
  backup: "item",
  restore: "item",
  update: "item",
  dbdump: "dbdump",
  dbimport: "dbimport",
};

/** How many failures the error badge stands for. */
export function unresolvedErrorCount(runs: Run[]): number {
  return failedRunsNeedingAttention(runs).length;
}

/**
 * The failures that still ask for someone: the last completed run per target
 * and class, where that run failed and nobody acknowledged it. `runs` arrives
 * newest first, and a running one is skipped so a retry in flight does not
 * hide the result before it.
 */
export function failedRunsNeedingAttention(runs: Run[]): Run[] {
  const latest = new Map<string, Run>();
  for (const r of runs) {
    const cls = ERROR_CLASSES[r.kind];
    if (!cls) continue;
    if (r.status === "running") continue;
    if (r.acknowledged) continue;
    const key = `${r.targetId}|${cls}`;
    if (!latest.has(key)) latest.set(key, r);
  }
  return Array.from(latest.values()).filter((r) => r.status === "failed");
}
