import type { Run } from "./api";

// The kinds a failure counts for, and the outcome each one can clear. A dump
// and an import stand apart from the item's backup: a failed dump must not hide
// a failed backup of the same container, and neither of them cancels the other.
// A saved dump is missing on purpose, since writing a copy of an old dump into
// a folder says nothing about the state of the database (#3).
const ERROR_CLASSES: Record<string, string> = {
  backup: "item",
  restore: "item",
  update: "item",
  dbdump: "dbdump",
  dbimport: "dbimport",
};

/**
 * failedRunsNeedingAttention is the last completed run per target and class,
 * where that run failed (#100). A target that has since succeeded drops out,
 * and so does an acknowledged failure (#126). `runs` arrives newest first, and
 * a still running one is skipped so an in-flight retry does not hide the
 * previous result.
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
