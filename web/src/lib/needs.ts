// What needs the person: the list behind the overview's first card, the
// number in the status head and a count in the navigation.

import type { AnomalyView, BackupItem, Run } from "./api";
import { protection } from "./backupList";
import { isFreshInstall } from "./freshInstall";
import { tallyEntries } from "./protectionTally";
import { failedRunsNeedingAttention } from "./runFailures";

export type Need =
  /** A run that failed and was neither repeated with success nor acknowledged.
   *  `item` is the entry it ran for, where the list of entries still has it. */
  | { is: "failedRun"; run: Run; item?: BackupItem }
  /** An entry no schedule reaches and that has no backup. */
  | { is: "unprotected"; item: BackupItem }
  | { is: "anomaly"; finding: AnomalyView }
  | { is: "recoveryKit" };

export interface NeedsInput {
  /** The rows of GET /api/items. */
  items: BackupItem[];
  /** The runs of GET /api/runs, newest first. */
  runs: Run[];
  /** The open findings while detection is on, else none. */
  anomalies: AnomalyView[];
  /** Encryption is on and nobody has confirmed the recovery kit is stored. */
  kitDue: boolean;
}

/**
 * needsOf lists what waits for the person, most urgent first. An entry whose
 * last run failed stands once, as that failure, also when it has no backup.
 * Before the first backup the entries without a schedule and the recovery kit
 * are the first steps, not something that went wrong.
 */
export function needsOf({ items, runs, anomalies, kitDue }: NeedsInput): Need[] {
  const failed = failedRunsNeedingAttention(runs).map((run): Need & { is: "failedRun" } => ({
    is: "failedRun",
    run,
    item: items.find((item) => item.runs.some((r) => r.id === run.id)),
  }));
  const fresh = isFreshInstall(items);
  const withFailure = new Set(failed.map((need) => need.item));
  const unprotected = fresh
    ? []
    : tallyEntries(items)
        .entries.filter((item) => protection(item) === "none" && !withFailure.has(item))
        .map((item): Need => ({ is: "unprotected", item }));
  const loud = anomalies
    .filter((finding) => finding.severity !== "info")
    .map((finding): Need => ({ is: "anomaly", finding }));
  return [...failed, ...unprotected, ...loud, ...(kitDue && !fresh ? [{ is: "recoveryKit" } as const] : [])];
}
