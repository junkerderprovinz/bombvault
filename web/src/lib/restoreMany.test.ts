import { describe, expect, it } from "vitest";
import type { BackupItem, TimelinePlace, TimelineRow } from "./api";
import { commonTimes, downloadsOnly, joinsMany, rowAt, runTarget, snapshotOf, whereFor } from "./restoreMany";

function item(over: Partial<BackupItem> = {}): BackupItem {
  return {
    kind: "container",
    key: "plex",
    name: "plex",
    included: true,
    paused: false,
    effectiveSchedule: { kind: "everything", spec: "daily@03:00" },
    lastBackup: 1_700_000_000,
    lastRunStatus: "success",
    sourceBytes: 1024,
    installed: true,
    runs: [],
    ...over,
  };
}

function row(key: string, unix: number, place = "local"): TimelineRow {
  return { key, time: new Date(unix * 1000).toISOString(), places: [{ place, snapshotIds: [`${key}-snap`], tags: [] }] };
}

function place(id: string, remote: boolean): TimelinePlace {
  return { place: id, label: id, kind: remote ? "target" : "home", remote, enabled: true, appendOnly: false, state: "read" };
}

const PLACES = [place("offsite:b2", true), place("local", false)];

describe("restoring several entries", () => {
  it("takes containers, VMs, folders and the flash, and leaves ZFS and the configuration to their own windows", () => {
    expect(joinsMany(item())).toBe(true);
    expect(joinsMany(item({ kind: "vm" }))).toBe(true);
    expect(joinsMany(item({ kind: "files", installed: undefined }))).toBe(true);
    expect(joinsMany(item({ kind: "flash", key: "flash" }))).toBe(true);
    expect(joinsMany(item({ installed: false }))).toBe(true);
    expect(joinsMany(item({ kind: "zfs" }))).toBe(false);
    expect(joinsMany(item({ kind: "config", key: "config" }))).toBe(false);
    expect(joinsMany(item({ lastBackup: 0 }))).toBe(false);
    expect(joinsMany(item({ kindDisabled: true }))).toBe(false);
  });

  it("offers the flash as a download only", () => {
    expect(downloadsOnly(item({ kind: "flash", key: "flash" }))).toBe(true);
    expect(downloadsOnly(item({ kind: "vm" }))).toBe(false);
  });

  it("creates a container that is gone in its place, whatever target was picked", () => {
    expect(whereFor(item(), "beside")).toBe("beside");
    expect(whereFor(item({ installed: false }), "beside")).toBe("replace");
    expect(whereFor(item({ kind: "files", installed: undefined }), "beside")).toBe("beside");
  });

  it("matches the run of a folder set by its name and the others by their key", () => {
    expect(runTarget(item())).toBe("plex");
    expect(runTarget(item({ kind: "files", key: "set-1", name: "Documents" }))).toBe("Documents");
  });

  it("takes the newest backup, or the newest one from a moment or before", () => {
    const rows = [row("c", 300), row("b", 200), row("a", 100)];
    expect(rowAt(rows, null)?.key).toBe("c");
    expect(rowAt(rows, 250)?.key).toBe("b");
    expect(rowAt(rows, 200)?.key).toBe("b");
    expect(rowAt(rows, 50)).toBeUndefined();
  });

  it("lists every moment of the picked entries once, newest first", () => {
    expect(commonTimes([[row("c", 300), row("a", 100)], [row("b", 200), row("x", 100)]])).toEqual([300, 200, 100]);
  });

  it("restores from the storage location the timeline ranks first", () => {
    const both: TimelineRow = { ...row("c", 300), places: [row("c", 300).places[0], { place: "offsite:b2", snapshotIds: ["far"], tags: [] }] };
    expect(snapshotOf(both, PLACES)).toEqual({ snapshotId: "c-snap", source: "local" });
    expect(snapshotOf({ ...both, places: [] }, PLACES)).toBeNull();
  });
});
