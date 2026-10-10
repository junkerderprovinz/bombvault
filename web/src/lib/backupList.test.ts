import { describe, expect, it } from "vitest";
import type { BackupItem, BackupItemRun } from "./api";
import {
  DEFAULT_VIEW,
  NOT_INSTALLED,
  addTarget,
  entriesOf,
  entryTarget,
  filterEntries,
  itemProgressKey,
  listedItems,
  parseTile,
  protection,
  rowStatus,
  runStrip,
  showsSelf,
  sortItems,
  tileEntries,
  tilePath,
  type ListView,
} from "./backupList";

function item(over: Partial<BackupItem> = {}): BackupItem {
  return {
    kind: "container",
    key: "plex",
    name: "plex",
    included: true,
    paused: false,
    effectiveSchedule: { kind: "everything", spec: "daily@03:00", alsoSpec: "" },
    lastBackup: 1_700_000_000,
    lastRunStatus: "success",
    sourceBytes: 1024,
    installed: true,
    runs: [],
    ...over,
  };
}

function run(status: string, over: Partial<BackupItemRun> = {}): BackupItemRun {
  return { id: `run-${status}`, kind: "backup", status, startedAt: 1_700_000_000, ...over };
}

const names = (items: BackupItem[]) => items.map((i) => i.name);
const ctx = { nameOf: (i: BackupItem) => i.name, isRunning: () => false, lang: "en" };

describe("the chosen tile", () => {
  it("is read from ?kind= and written back to the same address", () => {
    for (const tile of ["container", "vm", "flash", "files", "zfs", NOT_INSTALLED] as const) {
      expect(parseTile(new URLSearchParams(tilePath(tile).split("?")[1]).get("kind"))).toBe(tile);
    }
    expect(tilePath("all")).toBe("/backups");
    expect(tilePath("files")).toBe("/backups?kind=files");
  });

  it("falls back to All for no kind, an unknown one and the self-backup, which has no tile", () => {
    expect(parseTile(null)).toBe("all");
    expect(parseTile("tapes")).toBe("all");
    expect(parseTile("config")).toBe("all");
  });

  it("stands for the entries of its kind, or for what is gone from the host", () => {
    const entries = [
      item({ name: "plex" }),
      item({ name: "pihole", installed: false }),
      item({ kind: "vm", name: "win10", installed: false }),
      item({ kind: "files", name: "Documents", installed: undefined }),
    ];
    expect(names(tileEntries(entries, "all"))).toEqual(["plex", "pihole", "win10", "Documents"]);
    expect(names(tileEntries(entries, "container"))).toEqual(["plex", "pihole"]);
    expect(names(tileEntries(entries, NOT_INSTALLED))).toEqual(["pihole", "win10"]);
  });
});

describe("what the list holds", () => {
  it("drops an entry of a switched-off kind that has neither a backup nor a run", () => {
    const items = [
      item({ name: "never-touched", kindDisabled: true, lastBackup: 0, lastRunStatus: "" }),
      item({ name: "has-backup", kindDisabled: true }),
      item({ name: "only-failed", kindDisabled: true, lastBackup: 0, runs: [run("failed")] }),
      item({ name: "enabled", lastBackup: 0 }),
    ];
    expect(names(listedItems(items))).toEqual(["has-backup", "only-failed", "enabled"]);
  });

  it("does not count BombVault's own container as an entry", () => {
    expect(names(entriesOf([item({ name: "plex" }), item({ name: "bombvault", self: true })]))).toEqual(["plex"]);
  });
});

describe("where things lead", () => {
  it("opens the page of the entry's kind", () => {
    expect(entryTarget(item())).toBe("/containers");
    expect(entryTarget(item({ kind: "zfs" }))).toBe("/zfs");
    expect(entryTarget(item({ kind: "config" }))).toBe("/config");
  });

  it("adds on the page of the chosen kind, else on the first kind that is added by hand and switched on", () => {
    expect(addTarget("zfs", { files: true, zfs: true })).toBe("/zfs");
    expect(addTarget("files", {})).toBe("/files");
    expect(addTarget("all", { files: true, zfs: true })).toBe("/files");
    expect(addTarget("container", { zfs: true })).toBe("/zfs");
    expect(addTarget("all", { container: true })).toBeNull();
  });

  it("finds a running backup under the key the server reports it by", () => {
    expect(itemProgressKey(item({ kind: "vm", key: "win11", name: "Windows 11" }))).toBe("vm:win11");
    expect(itemProgressKey(item({ kind: "files", key: "a1b2", name: "Documents" }))).toBe("files:Documents");
    expect(itemProgressKey(item({ kind: "flash", key: "flash", name: "flash" }))).toBe("flash");
  });
});

describe("protection", () => {
  it("is given once a backup exists", () => {
    expect(protection(item())).toBe("ok");
    expect(protection(item({ installed: false }))).toBe("ok");
  });

  it("is pending while the first scheduled run is still to come", () => {
    expect(protection(item({ lastBackup: 0 }))).toBe("pending");
  });

  it("is missing when nothing will run", () => {
    expect(protection(item({ lastBackup: 0, included: false }))).toBe("none");
    expect(protection(item({ lastBackup: 0, paused: true }))).toBe("none");
    expect(protection(item({ lastBackup: 0, effectiveSchedule: { kind: "none", spec: "", alsoSpec: "" } }))).toBe("none");
    expect(protection(item({ lastBackup: 0, installed: false }))).toBe("none");
  });
});

describe("the one status of a row", () => {
  it("puts a failed last run before everything else", () => {
    expect(rowStatus(item({ lastRunStatus: "failed" }), 2)).toEqual({ is: "failed" });
  });

  it("then names open anomalies", () => {
    expect(rowStatus(item(), 2)).toEqual({ is: "anomalies", count: 2 });
  });

  it("shows the last backup of a protected entry", () => {
    expect(rowStatus(item(), 0)).toEqual({ is: "last", quiet: true });
  });

  it("warns about an entry nothing will back up and waits for one that is scheduled", () => {
    expect(rowStatus(item({ lastBackup: 0, included: false }), 0)).toEqual({ is: "notProtected" });
    expect(rowStatus(item({ lastBackup: 0 }), 0)).toEqual({ is: "waiting" });
  });

  it("leaves only the date on an entry that is gone or whose kind is off, failed or not", () => {
    expect(rowStatus(item({ installed: false, lastRunStatus: "failed" }), 0)).toEqual({ is: "last", quiet: false });
    expect(rowStatus(item({ kindDisabled: true }), 0)).toEqual({ is: "last", quiet: false });
  });
});

describe("the run strip", () => {
  it("is fourteen squares, oldest first, grey where nothing ran", () => {
    const strip = runStrip(item({ runs: [run("success"), run("failed")] }), () => false);
    expect(strip).toHaveLength(14);
    expect(strip.slice(0, 12)).toEqual(Array(12).fill("none"));
    expect(strip.slice(12)).toEqual(["fail", "ok"]);
  });

  it("colours a run an anomaly was raised on, whatever its outcome", () => {
    const runs = [run("success", { id: "a" }), run("success", { id: "b" })];
    expect(runStrip(item({ runs }), (id) => id === "b").slice(12)).toEqual(["warn", "ok"]);
  });

  it("keeps a cancelled and a skipped run grey", () => {
    expect(runStrip(item({ runs: [run("cancelled"), run("skipped")] }), () => false).slice(12)).toEqual(["none", "none"]);
  });

  it("gives no square to a database dump, a restore or a run that is still going", () => {
    const runs = [
      run("running"),
      run("success", { kind: "dbdump" }),
      run("failed", { kind: "restore" }),
      run("success", { kind: "import" }),
      run("success"),
    ];
    expect(runStrip(item({ runs }), () => false).filter((tone) => tone !== "none")).toEqual(["ok", "ok"]);
  });
});

describe("filtering", () => {
  const entries = [
    item({ name: "plex" }),
    item({ name: "paused", included: false }),
    item({ name: "own-off", paused: true }),
    item({ name: "never", lastBackup: 0 }),
    item({ name: "pihole", installed: false }),
    item({ kind: "files", name: "Documents", installed: undefined }),
  ];
  const view = (over: Partial<ListView>): ListView => ({ ...DEFAULT_VIEW, ...over });
  const left = (v: ListView, query = "") => names(filterEntries(entries, "all", v, query, (i) => i.name));

  it("by schedule", () => {
    expect(left(view({ schedule: "scheduled" }))).toEqual(["plex", "never", "pihole", "Documents"]);
    expect(left(view({ schedule: "paused" }))).toEqual(["paused", "own-off"]);
  });

  it("by backup", () => {
    expect(left(view({ backup: "never" }))).toEqual(["never"]);
    expect(left(view({ backup: "backedUp" }))).not.toContain("never");
  });

  it("by whether the container or VM is still installed", () => {
    expect(left(view({ installed: "notInstalled" }))).toEqual(["pihole"]);
    expect(left(view({ installed: "installed" }))).not.toContain("pihole");
  });

  it("by a search in the name, whatever its case", () => {
    expect(left(DEFAULT_VIEW, "  DOC ")).toEqual(["Documents"]);
  });

  it("inside the chosen tile only", () => {
    expect(names(filterEntries(entries, "files", DEFAULT_VIEW, "", (i) => i.name))).toEqual(["Documents"]);
  });
});

describe("sorting", () => {
  const sorted = (items: BackupItem[], field: ListView["sort"]["field"], reversed = false, over = {}) =>
    names(sortItems(items, { field, reversed }, { ...ctx, ...over }));

  it("by name in both directions, numbers in their natural order", () => {
    const items = [item({ name: "node10" }), item({ name: "Alpha" }), item({ name: "node2" })];
    expect(sorted(items, "name")).toEqual(["Alpha", "node2", "node10"]);
    expect(sorted(items, "name", true)).toEqual(["node10", "node2", "Alpha"]);
  });

  it("by status: running, failed, never backed up, fine", () => {
    const items = [
      item({ name: "fine" }),
      item({ name: "never", lastBackup: 0, lastRunStatus: "" }),
      item({ name: "failed", lastRunStatus: "failed" }),
      item({ name: "running" }),
    ];
    const isRunning = (i: BackupItem) => i.name === "running";
    expect(sorted(items, "status", false, { isRunning })).toEqual(["running", "failed", "never", "fine"]);
    expect(sorted(items, "status", true, { isRunning })).toEqual(["fine", "never", "failed", "running"]);
  });

  it("by last backup, newest first, and by size, largest first", () => {
    const items = [
      item({ name: "old", lastBackup: 100, sourceBytes: 5 }),
      item({ name: "new", lastBackup: 300, sourceBytes: null }),
      item({ name: "mid", lastBackup: 200, sourceBytes: 50 }),
    ];
    expect(sorted(items, "lastBackup")).toEqual(["new", "mid", "old"]);
    expect(sorted(items, "lastBackup", true)).toEqual(["old", "mid", "new"]);
    expect(sorted(items, "size")).toEqual(["mid", "old", "new"]);
  });

  it("by schedule, with what never runs last and equal schedules side by side", () => {
    const at = (spec: string) => ({ kind: "domain" as const, spec, alsoSpec: "" });
    const items = [
      item({ name: "off", included: false }),
      item({ name: "b-4am", effectiveSchedule: at("daily@04:00") }),
      item({ name: "a-3am", effectiveSchedule: at("daily@03:00") }),
      item({ name: "a-4am", effectiveSchedule: at("daily@04:00") }),
    ];
    expect(sorted(items, "schedule")).toEqual(["a-3am", "a-4am", "b-4am", "off"]);
  });

  it("keeps what is gone from the host after what is installed, in every order", () => {
    const items = [item({ name: "a-gone", installed: false }), item({ name: "z-here" }), item({ name: "b-here" })];
    expect(sorted(items, "name")).toEqual(["b-here", "z-here", "a-gone"]);
    expect(sorted(items, "name", true)).toEqual(["z-here", "b-here", "a-gone"]);
  });
});

describe("BombVault's own container", () => {
  const self = item({ name: "bombvault", self: true, lastBackup: 0 });

  it("shows under All and under Containers", () => {
    expect(showsSelf(self, "all", DEFAULT_VIEW, "")).toBe(true);
    expect(showsSelf(self, "container", DEFAULT_VIEW, "bomb")).toBe(true);
    expect(showsSelf(self, "vm", DEFAULT_VIEW, "")).toBe(false);
    expect(showsSelf(undefined, "all", DEFAULT_VIEW, "")).toBe(false);
  });

  it("leaves when a filter or the search rules it out", () => {
    expect(showsSelf(self, "all", { ...DEFAULT_VIEW, schedule: "scheduled" }, "")).toBe(false);
    expect(showsSelf(self, "all", { ...DEFAULT_VIEW, backup: "never" }, "")).toBe(false);
    expect(showsSelf(self, "all", { ...DEFAULT_VIEW, installed: "notInstalled" }, "")).toBe(false);
    expect(showsSelf(self, "all", { ...DEFAULT_VIEW, installed: "installed" }, "")).toBe(true);
    expect(showsSelf(self, "all", DEFAULT_VIEW, "plex")).toBe(false);
  });
});
