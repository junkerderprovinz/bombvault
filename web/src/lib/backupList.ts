// What the Backups page works out from GET /api/items before it draws a row:
// which entries a tile stands for, what state an entry is in, and how the
// list is filtered and ordered.

import type { BackupItem } from "./api";
import { ENTRY_KIND, ENTRY_LIST_PATH, entryListPath, entryPath, progressKey, type EntryKind } from "./backupEntry";
import type { ProgressState } from "./progress";

/** The kinds with a tile of their own, in the order the tiles stand. The
 *  self-backup is one entry and shows under All. */
export const TILE_KINDS = ["container", "vm", "flash", "files", "zfs"] as const satisfies readonly EntryKind[];

export type TileKind = (typeof TILE_KINDS)[number];

export const NOT_INSTALLED = "not-installed";

export type Tile = "all" | TileKind | typeof NOT_INSTALLED;

/** parseTile reads the chosen tile out of ?kind=. Anything else is All. */
export function parseTile(value: string | null): Tile {
  if (value === NOT_INSTALLED) return NOT_INSTALLED;
  return TILE_KINDS.find((kind) => kind === value) ?? "all";
}

export function tilePath(tile: Tile): string {
  if (tile === "all") return entryListPath();
  if (tile === NOT_INSTALLED) return `${ENTRY_LIST_PATH}?kind=${NOT_INSTALLED}`;
  return entryListPath(tile);
}

/** entryTarget is where a row leads: the page of its entry. */
export function entryTarget(item: BackupItem): string {
  return entryPath(item.kind, item.key);
}

/** The two kinds added by hand. Every other kind is found on the server. */
export const ADDED_KINDS = ["files", "zfs"] as const satisfies readonly EntryKind[];

export type AddedKind = (typeof ADDED_KINDS)[number];

/**
 * addStart is what the Add action opens: the form of the tile's kind, the
 * choice between the two, or nothing when neither kind is switched on.
 */
export function addStart(tile: Tile, enabled: Partial<Record<EntryKind, boolean>>): AddedKind | "choose" | null {
  if (tile === "files" || tile === "zfs") return tile;
  return ADDED_KINDS.some((kind) => enabled[kind]) ? "choose" : null;
}

/** itemId names an entry among all kinds, as a selection and a list key do. */
export function itemId(item: BackupItem): string {
  return `${item.kind}:${item.key}`;
}

/** The key a running backup or restore of the item reports under. Containers
 *  and VMs report under their key, folder sets and ZFS items under their name. */
export function itemProgressKey(item: BackupItem): string {
  return progressKey(item.kind, ENTRY_KIND[item.kind].keyedBy === "name" ? item.key : item.name);
}

/** runsNow narrows a progress entry to one that shows a backup or a restore
 *  of the item under way. */
export function runsNow(progress: ProgressState | undefined): progress is ProgressState {
  return !!progress?.active && (progress.phase === "backup" || progress.phase === "restore");
}

export function isGone(item: BackupItem): boolean {
  return item.installed === false;
}

/** inSchedule is false for an entry taken out of the schedule or whose own
 *  schedule is off. */
export function inSchedule(item: BackupItem): boolean {
  return item.included && !item.paused;
}

/**
 * listedItems drops what the list has nothing to say about: an entry of a
 * switched-off kind that was never backed up and never ran. Docker reports
 * every container on the host, also while containers are not backed up at all.
 */
export function listedItems(items: BackupItem[]): BackupItem[] {
  return items.filter((item) => !item.kindDisabled || item.lastBackup > 0 || item.runs.length > 0);
}

/** entriesOf leaves out BombVault's own container, which is listed but is
 *  not an entry anything can be done with. */
export function entriesOf(items: BackupItem[]): BackupItem[] {
  return items.filter((item) => !item.self);
}

export function tileEntries(entries: BackupItem[], tile: Tile): BackupItem[] {
  if (tile === "all") return entries;
  if (tile === NOT_INSTALLED) return entries.filter(isGone);
  return entries.filter((item) => item.kind === tile);
}

export type Protection = "ok" | "pending" | "none";

/**
 * protection is how an entry stands. It is protected while a schedule reaches
 * it and a backup of it exists. One no schedule reaches is not, whatever old
 * backups it has, and neither is one the person said not to back up. The
 * server counts by the same rule in GET /api/coverage.
 */
export function protection(item: BackupItem): Protection {
  if (item.backupDeclined || isGone(item) || neverRuns(item)) return "none";
  return item.lastBackup > 0 ? "ok" : "pending";
}

/** The one thing a row's status says when no backup of it is running. */
export type RowStatus =
  | { is: "failed" }
  | { is: "anomalies"; count: number }
  /** The date of the last backup. `quiet` is the protected entry; without it
   *  the date is all that is left of an entry BombVault has stopped backing up. */
  | { is: "last"; quiet: boolean }
  | { is: "notProtected" }
  | { is: "waiting" };

export function rowStatus(item: BackupItem, openAnomalies: number): RowStatus {
  const gone = isGone(item);
  if (!gone && item.lastRunStatus === "failed") return { is: "failed" };
  if (openAnomalies > 0) return { is: "anomalies", count: openAnomalies };
  if (gone || item.kindDisabled) return { is: "last", quiet: false };
  switch (protection(item)) {
    case "none":
      return { is: "notProtected" };
    case "pending":
      return { is: "waiting" };
    default:
      return { is: "last", quiet: true };
  }
}

export type RunTone = "ok" | "warn" | "fail" | "none";

export const STRIP_LENGTH = 14;

// A restore changes nothing in the repository, so it takes no square. The
// newest square is then the run lastRunStatus speaks about.
const STRIP_KINDS = new Set(["backup", "import"]);

/**
 * runStrip is the tone of each of the last runs, oldest first and padded at
 * the old end, so a young entry reads as new and not as missed. A cancelled or
 * skipped run wrote nothing and stays grey.
 */
export function runStrip(item: BackupItem, hasAnomaly: (runId: string) => boolean): RunTone[] {
  const tones = item.runs
    .filter((run) => STRIP_KINDS.has(run.kind) && run.status !== "running")
    .slice(0, STRIP_LENGTH)
    .map((run): RunTone => {
      if (hasAnomaly(run.id)) return "warn";
      if (run.status === "success") return "ok";
      return run.status === "failed" ? "fail" : "none";
    })
    .reverse();
  return [...Array<RunTone>(STRIP_LENGTH - tones.length).fill("none"), ...tones];
}

export const SCHEDULE_FILTERS = ["all", "scheduled", "paused"] as const;
export const BACKUP_FILTERS = ["all", "backedUp", "never"] as const;
export const INSTALLED_FILTERS = ["all", "installed", "notInstalled"] as const;

export const SORT_FIELDS = ["name", "status", "ip", "lastBackup", "size", "schedule"] as const;

export type SortField = (typeof SORT_FIELDS)[number];

export interface ListSort {
  field: SortField;
  /** Each field has a natural order: A to Z, problems first, lowest address
   *  first, newest first, largest first, scheduled first. */
  reversed: boolean;
}

export interface ListView {
  schedule: (typeof SCHEDULE_FILTERS)[number];
  backup: (typeof BACKUP_FILTERS)[number];
  installed: (typeof INSTALLED_FILTERS)[number];
  sort: ListSort;
}

export const DEFAULT_VIEW: ListView = {
  schedule: "all",
  backup: "all",
  installed: "all",
  sort: { field: "name", reversed: false },
};

const VIEW_KEY = "bv-backups-view";

function oneOf<T extends string>(valid: readonly T[], value: unknown, fallback: T): T {
  return valid.find((v) => v === value) ?? fallback;
}

/** loadListView reads the filters and the sort this browser last used. The
 *  search is not stored and starts empty. */
export function loadListView(): ListView {
  let stored: Partial<ListView> = {};
  try {
    stored = JSON.parse(localStorage.getItem(VIEW_KEY) ?? "{}") ?? {};
  } catch {
    // Without storage, or with something else under the key, the list opens unfiltered.
  }
  return {
    schedule: oneOf(SCHEDULE_FILTERS, stored.schedule, DEFAULT_VIEW.schedule),
    backup: oneOf(BACKUP_FILTERS, stored.backup, DEFAULT_VIEW.backup),
    installed: oneOf(INSTALLED_FILTERS, stored.installed, DEFAULT_VIEW.installed),
    sort: {
      field: oneOf(SORT_FIELDS, stored.sort?.field, DEFAULT_VIEW.sort.field),
      reversed: stored.sort?.reversed === true,
    },
  };
}

export function saveListView(view: ListView): void {
  try {
    localStorage.setItem(VIEW_KEY, JSON.stringify(view));
  } catch {
    // The view then lasts until the page is left.
  }
}

export function isFiltered(view: ListView): boolean {
  return view.schedule !== "all" || view.backup !== "all" || view.installed !== "all";
}

function matchesView(item: BackupItem, view: ListView): boolean {
  if (view.schedule !== "all" && inSchedule(item) !== (view.schedule === "scheduled")) return false;
  if (view.backup !== "all" && item.lastBackup > 0 !== (view.backup === "backedUp")) return false;
  if (view.installed !== "all" && isGone(item) !== (view.installed === "notInstalled")) return false;
  return true;
}

export interface SortContext {
  /** The name a row shows, which for the flash drive and the self-backup is
   *  worded by the page. */
  nameOf: (item: BackupItem) => string;
  isRunning: (item: BackupItem) => boolean;
  lang: string;
}

function statusRank(item: BackupItem, ctx: SortContext): number {
  if (ctx.isRunning(item)) return 0;
  if (item.lastRunStatus === "failed") return 1;
  return item.lastBackup > 0 ? 3 : 2;
}

function neverRuns(item: BackupItem): boolean {
  return !inSchedule(item) || item.effectiveSchedule.kind === "none";
}

// Entries with the same schedule stand together, and what never runs comes last.
function compareSchedule(a: BackupItem, b: BackupItem): number {
  if (neverRuns(a) || neverRuns(b)) return Number(neverRuns(a)) - Number(neverRuns(b));
  return a.effectiveSchedule.spec.localeCompare(b.effectiveSchedule.spec);
}

// An address orders by its numbers, so .9 stands before .10. Entries without
// one, which is every kind but a running container, come last.
function ipRank(item: BackupItem): number {
  const parts = (item.ip ?? "").split(".").map(Number);
  if (parts.length !== 4 || parts.some(Number.isNaN)) return Number.MAX_SAFE_INTEGER;
  return parts.reduce((sum, part) => sum * 256 + part, 0);
}

function compare(a: BackupItem, b: BackupItem, field: Exclude<SortField, "name">, ctx: SortContext): number {
  switch (field) {
    case "status":
      return statusRank(a, ctx) - statusRank(b, ctx);
    case "ip":
      return ipRank(a) - ipRank(b);
    case "lastBackup":
      return b.lastBackup - a.lastBackup;
    case "size":
      return (b.sourceBytes ?? 0) - (a.sourceBytes ?? 0);
    case "schedule":
      return compareSchedule(a, b);
  }
}

/**
 * sortItems orders the rows. Equal rows fall back to the name, and what is no
 * longer installed always stands after what is.
 */
export function sortItems(items: BackupItem[], sort: ListSort, ctx: SortContext): BackupItem[] {
  const collator = new Intl.Collator(ctx.lang, { numeric: true, sensitivity: "base" });
  const byName = (a: BackupItem, b: BackupItem) => collator.compare(ctx.nameOf(a), ctx.nameOf(b));
  const direction = sort.reversed ? -1 : 1;
  const sorted = [...items].sort((a, b) => {
    const order = sort.field === "name" ? byName(a, b) : compare(a, b, sort.field, ctx);
    return order !== 0 ? order * direction : byName(a, b);
  });
  return [...sorted.filter((item) => !isGone(item)), ...sorted.filter(isGone)];
}

/** filterEntries is what the chosen tile, the filters and the search leave. */
export function filterEntries(
  entries: BackupItem[],
  tile: Tile,
  view: ListView,
  query: string,
  nameOf: (item: BackupItem) => string
): BackupItem[] {
  const q = query.trim().toLowerCase();
  const found = (item: BackupItem) =>
    [nameOf(item), item.image, item.ip, ...(item.formerNames ?? [])].some((text) => text?.toLowerCase().includes(q));
  return tileEntries(entries, tile).filter((item) => matchesView(item, view) && (q === "" || found(item)));
}

/**
 * showsSelf says whether BombVault's own container belongs in the view. It is
 * a container that is installed, has no schedule and no backup, so any filter
 * on those leaves it out.
 */
export function showsSelf(self: BackupItem | undefined, tile: Tile, view: ListView, query: string): self is BackupItem {
  if (!self || (tile !== "all" && tile !== "container")) return false;
  if (view.schedule !== "all" || view.backup !== "all" || view.installed === "notInstalled") return false;
  return self.name.toLowerCase().includes(query.trim().toLowerCase());
}

/**
 * rangeIds is what a Shift+click picks: every row from the one picked last to
 * the clicked one, in the order the list shows them. Without a row picked
 * before, or with one that is out of view, it is the clicked row alone.
 */
export function rangeIds(order: string[], anchor: string | null, id: string): string[] {
  const to = order.indexOf(id);
  const from = anchor === null ? -1 : order.indexOf(anchor);
  if (from < 0 || to < 0) return [id];
  return order.slice(Math.min(from, to), Math.max(from, to) + 1);
}

/** canBackUp is false for what a backup would only skip or refuse. */
export function canBackUp(item: BackupItem): boolean {
  return !isGone(item) && !item.self && !item.kindDisabled;
}

export interface BatchPart {
  kind: EntryKind;
  keys: string[];
}

/** batchParts splits entries into the batches the server runs, one per kind. */
export function batchParts(items: BackupItem[]): BatchPart[] {
  const parts = new Map<EntryKind, string[]>();
  for (const item of items) parts.set(item.kind, [...(parts.get(item.kind) ?? []), item.key]);
  return [...parts].map(([kind, keys]) => ({ kind, keys }));
}

export interface StackGroup {
  project: string;
  members: BackupItem[];
}

/**
 * stacksOf groups the backed-up containers by their compose project and keeps
 * the projects with two or more of them. A container that is gone counts, so
 * a stack wiped from the server can still come back as one.
 */
export function stacksOf(items: BackupItem[], lang: string): StackGroup[] {
  const collator = new Intl.Collator(lang, { numeric: true, sensitivity: "base" });
  const byProject = new Map<string, BackupItem[]>();
  for (const item of items) {
    if (item.kind !== "container" || !item.stack) continue;
    if (!isGone(item) && item.lastBackup === 0) continue;
    byProject.set(item.stack, [...(byProject.get(item.stack) ?? []), item]);
  }
  return [...byProject]
    .filter(([, members]) => members.length > 1)
    .map(([project, members]) => ({ project, members: members.sort((a, b) => collator.compare(a.name, b.name)) }))
    .sort((a, b) => collator.compare(a.project, b.project));
}
