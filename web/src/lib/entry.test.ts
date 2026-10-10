import { describe, expect, it } from "vitest";
import type { BackupItem, Run } from "./api";
import {
  HISTORY_DAYS,
  barHeight,
  entryReason,
  entryTabPath,
  historyDays,
  parseEntryTab,
  recommendedRun,
} from "./entry";

const NOON = new Date(2026, 9, 6, 12, 0, 0).getTime();
const DAY = 86400;
const at = (daysAgo: number) => Math.floor(NOON / 1000) - daysAgo * DAY;

function item(over: Partial<BackupItem> = {}): BackupItem {
  return {
    kind: "container",
    key: "plex",
    name: "plex",
    included: true,
    paused: false,
    effectiveSchedule: { kind: "everything", spec: "daily@03:00", alsoSpec: "" },
    lastBackup: at(0),
    lastRunStatus: "success",
    sourceBytes: 1024,
    state: "running",
    installed: true,
    runs: [],
    ...over,
  };
}

function run(id: string, daysAgo: number, over: Partial<Run> = {}): Run {
  return {
    id,
    targetId: "tg",
    kind: "backup",
    status: "success",
    startedAt: at(daysAgo),
    finishedAt: at(daysAgo) + 40,
    snapshotId: `snap-${id}`,
    bytes: 100,
    error: "",
    acknowledged: false,
    target: "plex",
    domain: "container",
    ...over,
  };
}

describe("the open tab", () => {
  it("is Backups unless the address names another", () => {
    expect(parseEntryTab("")).toBe("backups");
    expect(parseEntryTab("?tab=settings")).toBe("settings");
    expect(parseEntryTab("?tab=verify")).toBe("verify");
    expect(parseEntryTab("?tab=nonsense")).toBe("backups");
  });

  it("goes into the address without dropping a restore request", () => {
    expect(entryTabPath("container", "my app", "settings")).toBe("/backups/container/my%20app?tab=settings");
    expect(entryTabPath("flash", "flash", "verify", "?restore=abc&at=17")).toBe(
      "/backups/flash/flash?restore=abc&at=17&tab=verify",
    );
    expect(entryTabPath("flash", "flash", "backups", "?tab=verify&restore=abc")).toBe(
      "/backups/flash/flash?restore=abc",
    );
  });
});

describe("the line under the name", () => {
  it("names the last backup of a protected entry", () => {
    expect(entryReason(item())).toEqual({ is: "lastBackup", at: at(0) });
  });

  it("names the last good backup when the last run failed", () => {
    expect(entryReason(item({ lastRunStatus: "failed", lastBackup: at(1) }))).toEqual({
      is: "lastFailed",
      lastGood: at(1),
    });
  });

  it("says a scheduled entry without a backup waits for its next run", () => {
    expect(entryReason(item({ lastBackup: 0, lastRunStatus: "" }))).toEqual({ is: "nextRun" });
  });

  it("tells a paused entry from one whose schedule is off", () => {
    expect(entryReason(item({ lastBackup: 0, included: false }))).toEqual({ is: "pausedNever" });
    expect(
      entryReason(item({ lastBackup: 0, effectiveSchedule: { kind: "none", spec: "", alsoSpec: "", reason: "domain-off" } })),
    ).toEqual({ is: "notScheduled" });
  });

  it("says of an entry that is gone only that", () => {
    expect(entryReason(item({ installed: false, lastRunStatus: "failed" }))).toEqual({ is: "gone" });
  });
});

describe("the backup history", () => {
  const none = () => false;

  it("covers two weeks ending today, with the empty days in place", () => {
    const days = historyDays([run("a", 0), run("b", 3)], none, NOON);
    expect(days).toHaveLength(HISTORY_DAYS);
    expect(new Date(days.at(-1)!.start).getDate()).toBe(6);
    expect(days.map((d) => d.runs.length)).toEqual([0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 1]);
  });

  it("adds up what the backups of one day wrote", () => {
    const days = historyDays([run("a", 1, { bytes: 300 }), run("b", 1, { bytes: 200 })], none, NOON);
    expect(days.at(-2)!.bytes).toBe(500);
  });

  it("leaves out runs that left no restore point and runs older than two weeks", () => {
    const runs = [
      run("failed", 0, { status: "failed" }),
      run("dump", 0, { kind: "dbdump" }),
      run("restore", 0, { kind: "restore" }),
      run("old", 20),
      run("import", 2, { kind: "import" }),
    ];
    const days = historyDays(runs, none, NOON);
    expect(days.flatMap((d) => d.runs.map((r) => r.id))).toEqual(["import"]);
  });

  it("recommends the newest backup no finding was raised on", () => {
    const runs = [run("a", 0), run("b", 1), run("c", 2)];
    const flagged = (r: Run) => r.id === "a";
    expect(recommendedRun(runs, flagged)?.id).toBe("b");
    const days = historyDays(runs, flagged, NOON);
    expect(days.at(-1)).toMatchObject({ flagged: true, recommended: false });
    expect(days.at(-2)).toMatchObject({ flagged: false, recommended: true });
  });

  it("keeps a quiet day visible beside a large one", () => {
    expect(barHeight(1_000_000, 1_000_000)).toBe(100);
    expect(barHeight(10_000, 1_000_000)).toBe(10);
    expect(barHeight(0, 1_000_000)).toBe(8);
    expect(barHeight(0, 0)).toBe(8);
  });
});
