// Restoring several entries at once from the Restore page: which entries can
// join, which backup each one takes, and where it goes.

import { BEFORE_RESTORE_TAG, type AnomalyView, type BackupItem, type TimelinePlace, type TimelineRow } from "./api";
import { ENTRY_KIND, type EntryKind } from "./backupEntry";
import { isGone } from "./backupList";
import { rowTime } from "./entry";
import { recommend } from "./restorePlan";
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

/** takenBeforeRestore is a backup BombVault took of what a restore was about
 *  to replace, which every copy of it is tagged with. */
export function takenBeforeRestore(row: TimelineRow): boolean {
  return (
    row.places.length > 0 &&
    row.places.every((mark) => mark.tags.some((tag) => tag === BEFORE_RESTORE_TAG || tag.startsWith(`${BEFORE_RESTORE_TAG}:`)))
  );
}

export interface ManyPick {
  row?: TimelineRow;
  /** The finding that moved the pick off the newest backup. */
  finding?: AnomalyView;
  /** The newest backup is one taken before a restore and was passed over. */
  passedOver: boolean;
}

/** pickRow is the backup an entry takes. At a moment it is the newest from
 *  then or before. Otherwise it is the one the window for a single entry
 *  recommends, among the backups that were not taken before a restore: such a
 *  backup holds the state a restore set out to replace. */
export function pickRow(rows: TimelineRow[], at: number | null, findings: AnomalyView[]): ManyPick {
  if (at !== null) return { row: rowAt(rows, at), passedOver: false };
  const usable = rows.filter((row) => !takenBeforeRestore(row));
  const { index, finding } = recommend(usable, findings);
  return { row: usable[index], finding, passedOver: rows.length > 0 && takenBeforeRestore(rows[0]) };
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

const NOT_STARTED_KEY = "bv-restore-many-left";

/** The entries of a running restore of several that have not started yet, kept
 *  so the Restore page can name them when the window was left mid-run. */
export interface NotStarted {
  names: string[];
  at: number;
}

export function noteNotStarted(names: string[]): void {
  try {
    localStorage.setItem(NOT_STARTED_KEY, JSON.stringify({ names, at: Date.now() } satisfies NotStarted));
  } catch {
    // Without storage the page cannot name them later.
  }
}

export function clearNotStarted(): void {
  try {
    localStorage.removeItem(NOT_STARTED_KEY);
  } catch {
    // nothing kept, nothing to clear
  }
}

export function readNotStarted(): NotStarted | null {
  try {
    const kept = JSON.parse(localStorage.getItem(NOT_STARTED_KEY) ?? "null") as Partial<NotStarted> | null;
    if (!kept || !Array.isArray(kept.names) || kept.names.length === 0) return null;
    return { names: kept.names.filter((n): n is string => typeof n === "string"), at: Number(kept.at) || 0 };
  } catch {
    return null;
  }
}

const RUN_LOCK = "bv-restore-many";

/** holdRunLock tells other tabs a restore of several runs in this one, until
 *  the returned function is called. */
export function holdRunLock(): () => void {
  let release = () => {};
  const held = new Promise<void>((resolve) => (release = resolve));
  void navigator.locks?.request(RUN_LOCK, () => held).catch(() => undefined);
  return release;
}

/** runLockHeld says whether a tab is running a restore of several now. */
export async function runLockHeld(): Promise<boolean> {
  try {
    const state = await navigator.locks?.query();
    return (state?.held ?? []).some((lock) => lock.name === RUN_LOCK);
  } catch {
    return false;
  }
}
