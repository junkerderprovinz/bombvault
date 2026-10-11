import { describe, expect, it } from "vitest";
import type { BackupItem, Run, TimelinePlace, TimelineRow } from "./api";
import {
  HISTORY_DAYS,
  actionMark,
  barHeight,
  comparable,
  diffGroups,
  entryReason,
  entryTabPath,
  historyDays,
  isOwnTag,
  parseEntryTab,
  parseTags,
  personTags,
  recommendedRun,
  rowHolds,
  rowTags,
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
    effectiveSchedule: { kind: "everything", spec: "daily@03:00" },
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
      entryReason(item({ lastBackup: 0, effectiveSchedule: { kind: "none", spec: "", reason: "domain-off" } })),
    ).toEqual({ is: "notScheduled" });
  });

  it("says of an entry the person set aside that it is excluded, with or without a backup", () => {
    expect(entryReason(item({ included: false, backupDeclined: true }))).toEqual({ is: "declined" });
    expect(entryReason(item({ lastBackup: 0, included: false, backupDeclined: true }))).toEqual({ is: "declined" });
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

describe("the tags of a backup", () => {
  const row = (...tags: string[][]): TimelineRow => ({
    key: "k",
    time: "2026-10-06T03:47:00Z",
    places: tags.map((list, i) => ({ place: i === 0 ? "local" : `offsite:t${i}`, snapshotIds: [`s${i}`], tags: list })),
  });

  it("tells a person's tags from the ones BombVault writes itself", () => {
    const own = ["container:plex", "vmrun:7", "bvrun:9", "dbdump:plex", "zfs:tank/a", "bv:direct", "p1", "blocks", "imported"];
    expect(own.filter(isOwnTag)).toEqual(own);
    expect(personTags([...own, "before-upgrade", "keep"])).toEqual(["before-upgrade", "keep"]);
  });

  it("shows a row the tags of every place once, and that it was imported", () => {
    expect(rowTags(row(["container:plex", "keep", "imported"], ["container:plex", "keep", "p1"]))).toEqual([
      "keep",
      "imported",
    ]);
  });

  it("reads typed tags apart at the comma and drops what is empty or double", () => {
    expect(parseTags(" keep ,before-upgrade,, keep")).toEqual(["keep", "before-upgrade"]);
    expect(parseTags("  ")).toEqual([]);
  });
});

describe("the place an action on a backup works at", () => {
  const local: TimelinePlace = { place: "local", label: "", kind: "home", remote: false, enabled: true, appendOnly: false, state: "read" };
  const box: TimelinePlace = { ...local, place: "offsite:t1", label: "Box", kind: "target", remote: true };
  const mark = (place: string) => ({ place, snapshotIds: [place], tags: [] });
  const both: TimelineRow = { key: "a", time: "2026-10-06T03:47:00Z", places: [mark(box.place), mark(local.place)] };
  const away: TimelineRow = { key: "b", time: "2026-10-05T03:47:00Z", places: [mark(box.place)] };

  it("is the entry's own place before a copy, and a copy when that is all there is", () => {
    expect(actionMark(both, [local, box]).place).toBe("local");
    expect(actionMark(away, [local, box]).place).toBe(box.place);
  });

  it("falls back to the first place while none of them is read", () => {
    expect(actionMark(away, [local, { ...box, state: "unchecked" }]).place).toBe(box.place);
  });

  it("offers for a comparison only the backups one place holds", () => {
    expect(comparable([both, away], "local").map((r) => r.key)).toEqual(["a"]);
    expect(comparable([both, away], box.place).map((r) => r.key)).toEqual(["a", "b"]);
  });

  it("finds a backup by its key and by the id a place holds it under", () => {
    expect(rowHolds(both, "a")).toBe(true);
    expect(rowHolds(both, "local")).toBe(true);
    expect(rowHolds(both, "other")).toBe(false);
  });
});

describe("the paths of a comparison", () => {
  it("are sorted by what happened to them", () => {
    const groups = diffGroups([
      { path: "/new", modifier: "+" },
      { path: "/gone", modifier: "-" },
      { path: "/edited", modifier: "M" },
      { path: "/retyped", modifier: "T" },
    ]);
    expect(groups.added.map((c) => c.path)).toEqual(["/new"]);
    expect(groups.removed.map((c) => c.path)).toEqual(["/gone"]);
    expect(groups.changed.map((c) => c.path)).toEqual(["/edited", "/retyped"]);
  });
});
