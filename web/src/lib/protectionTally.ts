// How many entries are protected. Every card that shows such a number takes it
// from here, so no two of them can disagree.

import type { BackupItem } from "./api";
import type { EntryKind } from "./backupEntry";
import { entriesOf, isGone, protection } from "./backupList";

export interface Tally {
  /** What is counted: the installed entries of every switched-on kind. */
  entries: BackupItem[];
  total: number;
  ok: number;
  pending: number;
  none: number;
  /** Entries whose container or VM is gone from the host. They are not counted. */
  gone: number;
}

/** tallyEntries counts the rows of GET /api/items, all kinds or one. */
export function tallyEntries(items: BackupItem[], kind?: EntryKind): Tally {
  const all = entriesOf(items).filter((item) => !item.kindDisabled && (!kind || item.kind === kind));
  const entries = all.filter((item) => !isGone(item));
  const count = (state: ReturnType<typeof protection>) => entries.filter((item) => protection(item) === state).length;
  return {
    entries,
    total: entries.length,
    ok: count("ok"),
    pending: count("pending"),
    none: count("none"),
    gone: all.length - entries.length,
  };
}

/** The newest successful backup among the counted entries, 0 for none. */
export function lastBackupOf(tally: Tally): number {
  return tally.entries.reduce((last, item) => Math.max(last, item.lastBackup), 0);
}
