import { describe, expect, it } from "vitest";
import type { AnomalyView, TimelineRow } from "./api";
import {
  emergencyPath,
  parseEmergencyWay,
  DEFAULT_RESTORE_FOLDER,
  backsUpFirst,
  defaultWhere,
  findingsOf,
  recommend,
  requestedRow,
  restoreFolderFor,
  whatsFor,
  wheresFor,
} from "./restorePlan";

const HERE = { hasOriginal: true, writable: false, oneDataset: false };
const GONE = { ...HERE, hasOriginal: false };

function row(key: string, daysAgo: number, copies: string[] = []): TimelineRow {
  return {
    key,
    time: new Date(Date.UTC(2026, 8, 30 - daysAgo)).toISOString(),
    places: [{ place: "local", snapshotIds: [key, ...copies], tags: [] }],
  };
}

const at = (r: TimelineRow) => Date.parse(r.time) / 1000;

function finding(over: Partial<AnomalyView>): AnomalyView {
  return { id: "a", severity: "critical", domain: "container", name: "plex", targetId: "t1", ...over } as AnomalyView;
}

const ROWS = [row("d", 0), row("c", 1), row("b", 2), row("a", 3)];

describe("what a kind can bring back", () => {
  it.each([
    ["container", false, ["all", "files"]],
    ["container", true, ["all", "files", "dumps"]],
    ["vm", false, ["all"]],
    ["files", false, ["all", "files"]],
    ["zfs", false, ["all", "files"]],
    ["flash", false, ["all", "plugins"]],
  ] as const)("%s (database: %s)", (kind, database, whats) => {
    expect(whatsFor(kind, { database })).toEqual(whats);
  });
});

describe("where a restore can go", () => {
  it.each([
    ["container", "all", HERE, ["beside", "replace", "folder"]],
    ["container", "all", GONE, ["replace", "folder"]],
    ["container", "files", HERE, ["replace", "folder"]],
    ["container", "dumps", HERE, []],
    ["vm", "all", HERE, ["beside", "replace"]],
    ["vm", "all", GONE, ["replace"]],
    ["files", "all", HERE, ["beside", "replace", "folder"]],
    ["files", "all", GONE, ["folder"]],
    ["files", "files", HERE, ["folder"]],
    ["flash", "all", HERE, ["zip"]],
    ["flash", "plugins", HERE, []],
    ["zfs", "all", { ...HERE, writable: true, oneDataset: true }, ["replace", "newDataset", "folder"]],
    ["zfs", "all", HERE, ["folder"]],
    ["zfs", "files", { ...HERE, writable: true, oneDataset: true }, ["replace"]],
    ["zfs", "files", HERE, []],
    ["config", "all", HERE, []],
  ] as const)("%s, %s", (kind, what, facts, wheres) => {
    expect(wheresFor(kind, what, facts)).toEqual(wheres);
  });

  it("starts on the target that leaves the original alone", () => {
    expect(defaultWhere(["beside", "replace", "folder"], true)).toBe("beside");
    expect(defaultWhere(["replace", "newDataset", "folder"], true)).toBe("newDataset");
    expect(defaultWhere(["replace"], true)).toBe("replace");
    expect(defaultWhere([], true)).toBeUndefined();
  });

  it("starts on bringing back an entry that is gone, which replaces nothing", () => {
    expect(defaultWhere(["replace", "folder"], false)).toBe("replace");
    expect(defaultWhere(["folder"], false)).toBe("folder");
  });

  it("backs up first only where a whole entry is replaced", () => {
    expect(backsUpFirst("container", "all", "replace", true)).toBe(true);
    expect(backsUpFirst("container", "files", "replace", true)).toBe(false);
    expect(backsUpFirst("container", "all", "beside", true)).toBe(false);
    expect(backsUpFirst("container", "all", "replace", false)).toBe(false);
    expect(backsUpFirst("zfs", "all", "replace", true)).toBe(false);
  });
});

describe("the folder a restore starts with", () => {
  it("is the one of the section, then the shared one, then the default", () => {
    const shared = { restoreFolder: "user/restore" };
    expect(restoreFolderFor({ ...shared, restoreFolders: { vms: "user/restore/vms" } }, "vm")).toBe("user/restore/vms");
    expect(restoreFolderFor({ ...shared, restoreFolders: { vms: "user/restore/vms" } }, "container")).toBe("user/restore");
    expect(restoreFolderFor({ restoreFolder: "" }, "files")).toBe(DEFAULT_RESTORE_FOLDER);
  });
});

describe("the findings of an entry", () => {
  it("are the open ones that are more than a note, found by name or by id", () => {
    const list = [
      finding({ id: "1" }),
      finding({ id: "2", severity: "info" }),
      finding({ id: "3", name: "sonarr" }),
      finding({ id: "4", domain: "files", name: "Photos", targetId: "set-1" }),
      finding({ id: "5", domain: "flash", name: "", targetId: "flash" }),
    ];
    expect(findingsOf(list, "container", "plex").map((a) => a.id)).toEqual(["1"]);
    expect(findingsOf(list, "files", "set-1").map((a) => a.id)).toEqual(["4"]);
    expect(findingsOf(list, "flash", "flash").map((a) => a.id)).toEqual(["5"]);
  });
});

describe("the recommended backup", () => {
  it("is the newest one while nothing is open", () => {
    expect(recommend(ROWS, [])).toEqual({ index: 0 });
  });

  it("is the last good one a finding names", () => {
    const a = finding({ lastGood: { runId: "r", snapshotId: "b", at: at(ROWS[2]) } });
    expect(recommend(ROWS, [a])).toEqual({ index: 2, finding: a });
  });

  it("is found by the id of a copy at another storage location", () => {
    const rows = [row("d", 0), row("c", 1, ["c-copy"])];
    const a = finding({ lastGood: { runId: "r", snapshotId: "c-copy", at: at(rows[1]) } });
    expect(recommend(rows, [a]).index).toBe(1);
  });

  it("is the newest one from before, when the named one is no longer kept", () => {
    const a = finding({ lastGood: { runId: "r", snapshotId: "pruned", at: at(ROWS[1]) + 60 } });
    expect(recommend(ROWS, [a]).index).toBe(1);
  });

  it("takes the earliest last good backup of several findings", () => {
    const late = finding({ id: "late", lastGood: { runId: "r", snapshotId: "c", at: at(ROWS[1]) } });
    const early = finding({ id: "early", lastGood: { runId: "r", snapshotId: "a", at: at(ROWS[3]) } });
    expect(recommend(ROWS, [late, early])).toEqual({ index: 3, finding: early });
  });

  it("is the one before the oldest backup a finding was raised on", () => {
    const a = finding({ flaggedSnapshots: ["d", "c"] });
    expect(recommend(ROWS, [a])).toEqual({ index: 2, finding: a });
  });

  it("stays the newest when every backup was flagged", () => {
    expect(recommend(ROWS, [finding({ flaggedSnapshots: ["a"] })])).toEqual({ index: 0 });
  });
});

describe("the backup a link asks for", () => {
  it("is found by any of its ids", () => {
    const rows = [row("d", 0), row("c", 1, ["c-copy"])];
    expect(requestedRow(rows, "c-copy", 0)).toEqual({ row: rows[1], missing: false });
  });

  it("falls back to the one closest in time, and says so", () => {
    expect(requestedRow(ROWS, "gone", at(ROWS[2]) + 3600)).toEqual({ row: ROWS[2], missing: true });
  });

  it("falls back to the newest without a time", () => {
    expect(requestedRow(ROWS, "gone", 0)).toEqual({ row: ROWS[0], missing: true });
  });
});

describe("a link into the emergency window", () => {
  it("names the step and, where one is meant, the way in", () => {
    expect(emergencyPath("settings")).toBe("/restore?emergency=settings");
    expect(emergencyPath("access", "group")).toBe("/restore?emergency=access&way=group");
  });

  it("is read back, and anything else opens on no way", () => {
    expect(parseEmergencyWay("group")).toBe("group");
    expect(parseEmergencyWay("kit")).toBe("kit");
    expect(parseEmergencyWay("pair")).toBeNull();
    expect(parseEmergencyWay(null)).toBeNull();
  });
});
