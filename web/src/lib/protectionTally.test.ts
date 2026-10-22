import { describe, expect, it } from "vitest";

import type { BackupItem } from "./api";
import { lastBackupOf, tallyEntries } from "./protectionTally";

function item(over: Partial<BackupItem>): BackupItem {
  return {
    kind: "container",
    key: "plex",
    name: "plex",
    included: true,
    paused: false,
    effectiveSchedule: { kind: "domain", spec: "daily 03:00" },
    lastBackup: 1_700_000_000,
    lastRunStatus: "success",
    sourceBytes: 1,
    installed: true,
    runs: [],
    ...over,
  };
}

const never = { lastBackup: 0, lastRunStatus: "" };
const excluded = { included: false, effectiveSchedule: { kind: "none", spec: "", reason: "excluded" } } as const;

describe("tallyEntries", () => {
  it("counts all six kinds", () => {
    const kinds = ["container", "vm", "files", "zfs", "flash", "config"] as const;
    const tally = tallyEntries(kinds.map((kind) => item({ kind, key: kind })));
    expect(tally.total).toBe(6);
    expect(tally.ok).toBe(6);
  });

  it("tells protected, waiting and not protected apart", () => {
    const tally = tallyEntries([
      item({ key: "backed-up" }),
      item({ key: "scheduled", ...never }),
      item({ key: "left-out", ...never, ...excluded }),
    ]);
    expect(tally).toMatchObject({ total: 3, ok: 1, pending: 1, none: 1 });
  });

  it("does not count an old backup as protection once the entry is out of every schedule", () => {
    expect(tallyEntries([item(excluded)])).toMatchObject({ total: 1, ok: 0, none: 1 });
    expect(tallyEntries([item({ paused: true })])).toMatchObject({ total: 1, ok: 0, none: 1 });
    expect(
      tallyEntries([item({ effectiveSchedule: { kind: "none", spec: "", reason: "schedule-off" } })])
    ).toMatchObject({ total: 1, ok: 0, none: 1 });
  });

  it("leaves out what is gone from the host and says how many", () => {
    const tally = tallyEntries([item({}), item({ key: "old", installed: false })]);
    expect(tally).toMatchObject({ total: 1, ok: 1, gone: 1 });
    expect(tally.entries.map((e) => e.key)).toEqual(["plex"]);
  });

  it("leaves out what the person said not to back up and lists it apart", () => {
    const set = item({ key: "scratch", ...never, ...excluded, backupDeclined: true });
    const tally = tallyEntries([item({}), set, item({ key: "left-out", ...never, ...excluded })]);
    expect(tally).toMatchObject({ total: 2, ok: 1, none: 1 });
    expect(tally.declined).toEqual([set]);
  });

  it("does not count an entry set aside as protected by the backups it still has", () => {
    const tally = tallyEntries([item({ ...excluded, backupDeclined: true })]);
    expect(tally).toMatchObject({ total: 0, ok: 0, none: 0 });
    expect(tally.declined).toHaveLength(1);
  });

  it("lists nothing as set aside that is gone or of a switched-off kind", () => {
    const tally = tallyEntries([
      item({ key: "old", installed: false, backupDeclined: true }),
      item({ kind: "vm", key: "win", kindDisabled: true, backupDeclined: true }),
    ]);
    expect(tally.declined).toEqual([]);
    expect(tally.gone).toBe(1);
  });

  it("leaves out BombVault's own container", () => {
    expect(tallyEntries([item({ key: "bombvault", self: true, ...never, ...excluded })]).total).toBe(0);
  });

  it("leaves out the entries of a switched-off kind", () => {
    const tally = tallyEntries([item({}), item({ kind: "vm", key: "win", kindDisabled: true })]);
    expect(tally.total).toBe(1);
  });

  it("counts one kind when asked for it", () => {
    const items = [item({}), item({ kind: "vm", key: "win", ...never })];
    expect(tallyEntries(items, "vm")).toMatchObject({ total: 1, ok: 0, pending: 1 });
    expect(tallyEntries(items, "zfs").total).toBe(0);
  });

  it("adds up to the total", () => {
    const tally = tallyEntries([
      item({}),
      item({ key: "a", ...never }),
      item({ key: "b", ...never, ...excluded }),
      item({ key: "c", installed: false }),
    ]);
    expect(tally.ok + tally.pending + tally.none).toBe(tally.total);
  });
});

describe("lastBackupOf", () => {
  it("is the newest backup of a counted entry", () => {
    const tally = tallyEntries([item({ lastBackup: 100 }), item({ key: "b", lastBackup: 300 }), item({ key: "gone", installed: false, lastBackup: 900 })]);
    expect(lastBackupOf(tally)).toBe(300);
  });

  it("is 0 where nothing was backed up", () => {
    expect(lastBackupOf(tallyEntries([]))).toBe(0);
  });

  it("counts the replica of an entry that only replicates, and no other", () => {
    const zfs = { kind: "zfs", ...never } as const;
    const tally = tallyEntries([
      item({ lastBackup: 100 }),
      item({ ...zfs, key: "only", replica: { lastAt: 300, lastStatus: "success", only: true } }),
      item({ ...zfs, key: "extra", replica: { lastAt: 900, lastStatus: "success" } }),
    ]);
    expect(lastBackupOf(tally)).toBe(300);
  });
});

describe("an entry that only replicates", () => {
  const only = (lastAt: number) =>
    item({ kind: "zfs", key: `z${lastAt}`, ...never, replica: { lastAt, lastStatus: lastAt ? "success" : "", only: true } });

  it("waits for its replica, not for a backup, and is protected once it has one", () => {
    const tally = tallyEntries([only(0), only(300), item({ key: "scheduled", ...never })]);
    expect(tally).toMatchObject({ total: 3, ok: 1, pending: 2, replicaPending: 1 });
  });
});
