// What the page of one backup entry works out before it draws: which tab is
// open, what the line under the name says, and how the runs of the last two
// weeks become the bars of the history.

import type {
  BackupChange,
  BackupItem,
  Container,
  FileSetView,
  PlacementView,
  Run,
  Settings,
  TimelineMark,
  TimelinePlace,
  TimelineRow,
  VM,
  ZFSDatasetView,
} from "./api";
import { entryPath, type EntryKind } from "./backupEntry";
import { inSchedule, isGone, protection } from "./backupList";
import { autoMark } from "./timeline";

export const ENTRY_TABS = ["backups", "settings", "verify"] as const;

export type EntryTab = (typeof ENTRY_TABS)[number];

const TAB_PARAM = "tab";

/** parseEntryTab reads the open tab out of ?tab=. Anything else is Backups. */
export function parseEntryTab(search: string): EntryTab {
  const value = new URLSearchParams(search).get(TAB_PARAM);
  return ENTRY_TABS.find((tab) => tab === value) ?? "backups";
}

/** entryTabPath keeps what else the address carries, which is a finding's
 *  request for a restore. */
export function entryTabPath(kind: EntryKind, key: string, tab: EntryTab, search = ""): string {
  const params = new URLSearchParams(search);
  if (tab === "backups") params.delete(TAB_PARAM);
  else params.set(TAB_PARAM, tab);
  const query = params.toString();
  return entryPath(kind, key) + (query ? `?${query}` : "");
}

/** What only the entry's own kind knows about it. */
export type EntryDetail =
  | {
      kind: "container";
      container: Container;
      /** Every installed container, for the picker of containers to stop. */
      installed: Container[];
      /** The not-installed entries this one can take over by hand. */
      linkCandidates: string[];
      /** False while Docker does not answer. The container is then what
       *  BombVault keeps of it, and its state says nothing. */
      hostAnswered: boolean;
    }
  | { kind: "vm"; vm: VM; linkCandidates: string[]; hostAnswered: boolean }
  | { kind: "files"; set: FileSetView }
  | { kind: "zfs"; dataset: ZFSDatasetView }
  | { kind: "flash" }
  | { kind: "config" };

export interface Entry {
  item: BackupItem;
  detail: EntryDetail;
  settings: Settings;
  hostMountRoot: string;
}

/** withPlacement puts the placement a change answered with into the entry,
 *  so the cards show it before the next read. */
export function withPlacement(entry: Entry, placement: PlacementView): Entry {
  const { detail } = entry;
  switch (detail.kind) {
    case "container":
      return { ...entry, detail: { ...detail, container: { ...detail.container, placement } } };
    case "vm":
      return { ...entry, detail: { ...detail, vm: { ...detail.vm, placement } } };
    case "files":
      return { ...entry, detail: { ...detail, set: { ...detail.set, placement } } };
    default:
      return entry;
  }
}

/** hostSilent says that Docker or libvirt did not answer for the entry's
 *  container or VM. */
export function hostSilent(detail: EntryDetail): boolean {
  return (detail.kind === "container" || detail.kind === "vm") && !detail.hostAnswered;
}

/** The one sentence about an entry's protection that leads the line under
 *  its name. */
export type EntryReason =
  | { is: "gone" }
  | { is: "declined" }
  | { is: "lastFailed"; lastGood: number }
  | { is: "lastBackup"; at: number }
  | { is: "pausedNever" }
  | { is: "notScheduled" }
  | { is: "nextRun" };

export function entryReason(item: BackupItem): EntryReason {
  if (isGone(item)) return { is: "gone" };
  if (item.backupDeclined) return { is: "declined" };
  if (item.lastBackup > 0) {
    return item.lastRunStatus === "failed"
      ? { is: "lastFailed", lastGood: item.lastBackup }
      : { is: "lastBackup", at: item.lastBackup };
  }
  if (protection(item) === "pending") return { is: "nextRun" };
  return inSchedule(item) ? { is: "notScheduled" } : { is: "pausedNever" };
}

export const HISTORY_DAYS = 14;

export interface HistoryDay {
  /** Local midnight that starts the day, in milliseconds. */
  start: number;
  /** What the day's backups added to the repository. */
  bytes: number;
  /** The day's backups, newest first. */
  runs: Run[];
  flagged: boolean;
  recommended: boolean;
}

const WROTE = new Set(["backup", "import"]);

function dayStart(ms: number): number {
  const d = new Date(ms);
  return new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
}

/** backupRuns are the runs that left a restore point, newest first. */
export function backupRuns(runs: Run[]): Run[] {
  return runs.filter((run) => WROTE.has(run.kind) && run.status === "success");
}

/**
 * recommendedRun is the backup to restore from when nothing says otherwise:
 * the newest one no open finding was raised on.
 */
export function recommendedRun(runs: Run[], isFlagged: (run: Run) => boolean): Run | undefined {
  return backupRuns(runs).find((run) => !isFlagged(run));
}

/**
 * historyDays sorts the backups of the last two weeks into days, oldest
 * first and ending today. A day without a backup stays in the row, so a gap
 * shows as a gap.
 */
export function historyDays(runs: Run[], isFlagged: (run: Run) => boolean, now = Date.now()): HistoryDay[] {
  const today = new Date(dayStart(now));
  const days: HistoryDay[] = Array.from({ length: HISTORY_DAYS }, (_, i) => ({
    // Counted in calendar days, which a change of the clock leaves alone.
    start: new Date(today.getFullYear(), today.getMonth(), today.getDate() - (HISTORY_DAYS - 1 - i)).getTime(),
    bytes: 0,
    runs: [],
    flagged: false,
    recommended: false,
  }));
  const byStart = new Map(days.map((day) => [day.start, day]));
  const recommended = recommendedRun(runs, isFlagged);
  for (const run of backupRuns(runs)) {
    const day = byStart.get(dayStart(run.startedAt * 1000));
    if (!day) continue;
    day.runs.push(run);
    day.bytes += run.bytes;
    if (isFlagged(run)) day.flagged = true;
    if (run === recommended) day.recommended = true;
  }
  return days;
}

/** barHeight is a day's share of the tallest day, in percent. The root keeps
 *  a quiet day visible next to the one that brought a whole library. */
export function barHeight(bytes: number, largest: number): number {
  if (largest <= 0) return 8;
  return Math.max(8, Math.round((Math.sqrt(bytes) / Math.sqrt(largest)) * 100));
}

// The tags BombVault writes itself, by prefix and in whole. The server refuses
// to set or clear them by hand, because retention, the database dumps and a
// VM restore read a meaning from them.
const OWN_TAG_PREFIXES = [
  "container:",
  "vm:",
  "fileset:",
  "stack:",
  "vmrun:",
  "formerly:",
  "dbdump:",
  "dbengine:",
  "dbimage:",
  "dbversion:",
  "dbname:",
  "bvrun:",
  "zfs:",
  "import:",
  "blocks-partial:",
  "bv:",
];
const IMPORTED_TAG = "imported";
const OWN_TAGS = new Set([IMPORTED_TAG, "blocks", "live", "p1", "p2"]);

export function isOwnTag(tag: string): boolean {
  return OWN_TAGS.has(tag) || OWN_TAG_PREFIXES.some((prefix) => tag.startsWith(prefix));
}

/** personTags are the tags among these that a person set. */
export function personTags(tags: string[]): string[] {
  return tags.filter((tag) => !isOwnTag(tag));
}

/** rowTags are the tags a backup shows in the list: the ones a person set at
 *  any of its places, and that it came in through an import. */
export function rowTags(row: TimelineRow): string[] {
  const all = new Set(row.places.flatMap((mark) => mark.tags));
  return [...all].filter((tag) => tag === IMPORTED_TAG || !isOwnTag(tag));
}

/** parseTags reads a typed list of tags, which a comma separates because no
 *  tag can hold one. */
export function parseTags(text: string): string[] {
  return [...new Set(text.split(",").map((tag) => tag.trim()))].filter((tag) => tag !== "");
}

/**
 * actionMark is where a tag, a comparison or an export of a backup works: the
 * place a restore would take it from, else the first place that holds it.
 */
export function actionMark(row: TimelineRow, places: TimelinePlace[]): TimelineMark {
  return autoMark(row, places) ?? row.places[0];
}

/** comparable are the backups that can be compared with one another at a
 *  place, which are the ones that place holds. */
export function comparable(rows: TimelineRow[], place: string): TimelineRow[] {
  return rows.filter((row) => row.places.some((mark) => mark.place === place));
}

export interface DiffGroups {
  added: BackupChange[];
  removed: BackupChange[];
  changed: BackupChange[];
}

/** diffGroups sorts the listed paths of a comparison by what happened to them. */
export function diffGroups(changes: BackupChange[]): DiffGroups {
  const groups: DiffGroups = { added: [], removed: [], changed: [] };
  for (const change of changes) {
    if (change.modifier === "+") groups.added.push(change);
    else if (change.modifier === "-") groups.removed.push(change);
    else groups.changed.push(change);
  }
  return groups;
}

/** rowHolds says whether an id names the backup: its key, or one of the
 *  snapshots a place holds of it. */
export function rowHolds(row: TimelineRow, snapshotId: string): boolean {
  return row.key === snapshotId || row.places.some((mark) => mark.snapshotIds.includes(snapshotId));
}

/** rowOfRun is the row of the backup list a run wrote. A copy at a target
 *  carries an id of its own, so every id of the row is looked at. */
export function rowOfRun(rows: TimelineRow[], run: Run): TimelineRow | undefined {
  return run.snapshotId === "" ? undefined : rows.find((row) => rowHolds(row, run.snapshotId));
}

export function runOfRow(runs: Run[], row: TimelineRow): Run | undefined {
  return runs.find((run) => run.snapshotId !== "" && rowHolds(row, run.snapshotId));
}

/** rowTime is when a backup was taken, in Unix seconds. */
export function rowTime(row: TimelineRow): number {
  return Math.floor(Date.parse(row.time) / 1000);
}
