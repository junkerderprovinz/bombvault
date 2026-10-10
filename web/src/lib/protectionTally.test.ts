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
    effectiveSchedule: { kind: "domain", spec: "daily 03:00", alsoSpec: "" },
    lastBackup: 1_700_000_000,
    lastRunStatus: "success",
    sourceBytes: 1,
    installed: true,
    runs: [],
    ...over,
  };
}

const never = { lastBackup: 0, lastRunStatus: "" };
const excluded = { included: false, effectiveSchedule: { kind: "none", spec: "", alsoSpec: "", reason: "excluded" } } as const;

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

  it("keeps an entry with a backup protected after it was taken out of the schedule", () => {
    expect(tallyEntries([item(excluded)])).toMatchObject({ total: 1, ok: 1, none: 0 });
  });

  it("leaves out what is gone from the host and says how many", () => {
    const tally = tallyEntries([item({}), item({ key: "old", installed: false })]);
    expect(tally).toMatchObject({ total: 1, ok: 1, gone: 1 });
    expect(tally.entries.map((e) => e.key)).toEqual(["plex"]);
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
});
