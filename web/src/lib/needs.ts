// What needs the person: the list behind the overview's first card, the
// number in the status head and a count in the navigation.

import type { AnomalyView, BackupItem, DeferredBackup, Run, StorageLocation, StorageLocationSection } from "./api";
import { protection } from "./backupList";
import { isFreshInstall } from "./freshInstall";
import { tallyEntries } from "./protectionTally";
import { failedRunsNeedingAttention } from "./runFailures";

export type Need =
  /** APP_KEY stopped opening the key this server took over, so no backup runs. */
  | { is: "keyLocked" }
  /** A run that failed and was neither repeated with success nor acknowledged.
   *  `item` is the entry it ran for, where the list of entries still has it. */
  | { is: "failedRun"; run: Run; item?: BackupItem }
  /** An entry no schedule reaches, whatever old backups it has. */
  | { is: "unprotected"; item: BackupItem }
  /** A container whose backup waits while its app is busy. Nothing has to be
   *  done, but the backup is late until it goes through. */
  | { is: "deferred"; item: BackupItem; deferral: DeferredBackup }
  /** The off-site copies to one storage location that have failed since they
   *  last worked. `since` is when the first of them began, in Unix seconds. */
  | { is: "failedCopy"; location: StorageLocation; sections: StorageLocationSection[]; since: number }
  | { is: "anomaly"; finding: AnomalyView }
  | { is: "recoveryKit" };

export interface NeedsInput {
  /** The rows of GET /api/items. */
  items: BackupItem[];
  /** The runs of GET /api/runs, newest first. */
  runs: Run[];
  /** The locations of GET /api/storage/locations. */
  locations: StorageLocation[];
  /** The open findings while detection is on, else none. */
  anomalies: AnomalyView[];
  /** Encryption is on and nobody has confirmed the recovery kit is stored. */
  kitDue: boolean;
  /** GET /api/emergency names the backup key "locked". */
  keyLocked?: boolean;
}

// A copy that is switched off is not tried again, so its last failure waits
// for nobody.
function copyFailing(section: StorageLocationSection): boolean {
  const last = section.lastCopy;
  return section.use === "copy" && section.enabled && !!last && !last.ok && !last.acknowledged;
}

function failedCopies(locations: StorageLocation[]): Need[] {
  return locations.flatMap((location): Need[] => {
    const sections = location.sections.filter(copyFailing);
    if (sections.length === 0) return [];
    const since = Math.min(...sections.map((section) => section.lastCopy?.failingSince ?? 0));
    return [{ is: "failedCopy", location, sections, since }];
  });
}

/**
 * needsOf lists what waits for the person, most urgent first. An entry whose
 * last run failed stands once, as that failure, also when it has no backup.
 * An entry the person said not to back up waits for nobody, a failed run of
 * it included.
 * Before the first backup the entries without a schedule and the recovery kit
 * are the first steps, not something that went wrong.
 */
export function needsOf({ items, runs, locations, anomalies, kitDue, keyLocked = false }: NeedsInput): Need[] {
  const failed = failedRunsNeedingAttention(runs)
    .map((run): Need & { is: "failedRun" } => ({
      is: "failedRun",
      run,
      item: items.find((item) => item.runs.some((r) => r.id === run.id)),
    }))
    .filter((need) => !need.item?.backupDeclined);
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
  const deferred = items.flatMap((item): Need[] =>
    item.deferral && !withFailure.has(item) && !item.backupDeclined ? [{ is: "deferred", item, deferral: item.deferral }] : []
  );
  return [
    // First, because every backup is refused until it is solved.
    ...(keyLocked ? [{ is: "keyLocked" } as const] : []),
    ...failed,
    ...unprotected,
    ...deferred,
    ...failedCopies(locations),
    ...loud,
    ...(kitDue && !fresh ? [{ is: "recoveryKit" } as const] : []),
  ];
}
