// Restoring several entries at once from the Restore page: which entries can
// join, which backup each one takes, and where it goes.

import type { BackupItem, TimelinePlace, TimelineRow } from "./api";
import { ENTRY_KIND, type EntryKind } from "./backupEntry";
import { isGone } from "./backupList";
import { rowTime } from "./entry";
import { autoMark, newestId, sourceOfPlace } from "./timeline";

/** Why an entry with backups still cannot join a restore of several. A ZFS
 *  restore asks for a dataset and a target of its own, and the configuration
 *  comes back through the emergency wizard. */
export type LeftOut = "zfs" | "config";

export function leftOut(kind: EntryKind): LeftOut | null {
  return kind === "zfs" || kind === "config" ? kind : null;
}

/** joinsMany is true for an entry that can be restored together with others. */
export function joinsMany(item: BackupItem): boolean {
  return item.lastBackup > 0 && !item.kindDisabled && leftOut(item.kind) === null;
}

/** A flash backup is never written back over the live stick, so in a restore
 *  of several it comes as a ZIP to download beside the others. */
export function downloadsOnly(item: BackupItem): boolean {
  return item.kind === "flash";
}

/** The name the recorded run of a restore carries for the entry. */
export function runTarget(item: BackupItem): string {
  return ENTRY_KIND[item.kind].keyedBy === "name" ? item.key : item.name;
}

export type ManyWhere = "beside" | "replace";

/** Where one entry goes. Without the container or VM on the host there is
 *  nothing beside which to put it, so it is created again in its place. */
export function whereFor(item: BackupItem, where: ManyWhere): ManyWhere {
  return isGone(item) ? "replace" : where;
}

/** rowAt is the backup an entry takes for a moment: its newest, or its newest
 *  from that moment or before. Rows come newest first. */
export function rowAt(rows: TimelineRow[], at: number | null): TimelineRow | undefined {
  if (at === null) return rows[0];
  return rows.find((row) => rowTime(row) <= at);
}

/** The moments to restore to, newest first: the times of every backup of the
 *  picked entries, each once. */
export function commonTimes(timelines: TimelineRow[][]): number[] {
  const times = new Set(timelines.flatMap((rows) => rows.map(rowTime)));
  return [...times].sort((a, b) => b - a);
}

/** The snapshot and the storage location a backup is restored from. */
export function snapshotOf(row: TimelineRow, places: TimelinePlace[]): { snapshotId: string; source: string } | null {
  const mark = autoMark(row, places) ?? row.places[0];
  if (!mark) return null;
  return { snapshotId: newestId(mark), source: sourceOfPlace(mark.place) };
}
