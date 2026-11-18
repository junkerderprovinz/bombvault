import { describe, expect, it } from "vitest";
import type { HostSnapPlan, HostSnapshot } from "./api";
import {
  codeKey,
  defaultName,
  freedEstimate,
  groupSnapshots,
  kindStates,
  needsQcow2,
  planOf,
  sleepingPools,
  sourceLabel,
  stillAsleep,
  validName,
  wokenPools,
} from "./hostSnapshots";

function snap(over: Partial<HostSnapshot>): HostSnapshot {
  return {
    kind: "zfs",
    source: "tank/apps",
    name: "a",
    label: "a",
    createdAt: 0,
    usedBytes: 0,
    referencedBytes: 0,
    newSinceBytes: -1,
    holds: [],
    clones: [],
    owner: "",
    readOnly: true,
    caps: { rename: true, hold: true, delete: true, estimate: true },
    ...over,
  };
}

describe("groupSnapshots", () => {
  it("puts each source's snapshots together, ZFS first, in the server's order", () => {
    const groups = groupSnapshots([
      snap({ kind: "vm", source: "win11", name: "s" }),
      snap({ name: "new" }),
      snap({ kind: "btrfs", fs: "/mnt/cache", source: "appdata", name: "x" }),
      snap({ name: "old" }),
    ]);
    expect(groups.map((g) => g.kind)).toEqual(["zfs", "btrfs", "vm"]);
    expect(groups[0].snapshots.map((s) => s.name)).toEqual(["new", "old"]);
  });
});

describe("freedEstimate", () => {
  it("adds what btrfs reports to ZFS's own figure", () => {
    const got = freedEstimate([snap({}), snap({ kind: "btrfs", usedBytes: 100 })], 900);
    expect(got).toEqual({ bytes: 1000, complete: true });
  });

  it("says it is a floor when part of the selection reports no size", () => {
    expect(freedEstimate([snap({ kind: "vm", usedBytes: -1 })], 0).complete).toBe(false);
    expect(freedEstimate([snap({ kind: "btrfs", usedBytes: -1 })], 50)).toEqual({ bytes: 50, complete: false });
  });
});

describe("pools", () => {
  const pools = [
    { kind: "zfs" as const, name: "tank", state: "awake" as const, read: true, woke: false },
    { kind: "zfs" as const, name: "disk1", state: "asleep" as const, read: false, woke: false },
    { kind: "zfs" as const, name: "disk2", state: "asleep" as const, read: true, woke: true },
    { kind: "zfs" as const, name: "disk3", state: "asleep" as const, read: true, woke: false },
  ];
  it("tells unread, woken and still sleeping pools apart", () => {
    expect(sleepingPools(pools).map((p) => p.name)).toEqual(["disk1"]);
    expect(wokenPools(pools).map((p) => p.name)).toEqual(["disk2"]);
    expect(stillAsleep(pools).map((p) => p.name)).toEqual(["disk3"]);
  });
});

describe("kindStates", () => {
  it("sets a server without a tool apart from one that failed and from no host", () => {
    expect(kindStates({ zfs: { code: "ok" }, btrfs: { code: "missing" }, vm: { code: "vm-unreachable" } })).toEqual({
      missing: ["btrfs", "vm"],
      failed: [],
      noHost: false,
    });
    expect(kindStates({ zfs: { code: "ssh-auth" }, btrfs: { code: "ssh-auth" } }).noHost).toBe(true);
    expect(kindStates({ zfs: { code: "zfs-permission", detail: "no" } }).failed).toEqual([
      { kind: "zfs", code: "zfs-permission", detail: "no" },
    ]);
  });
});

describe("names", () => {
  it("offers a UTC name and checks one the way the server does", () => {
    expect(defaultName(new Date(Date.UTC(2026, 10, 17, 9, 5, 3)))).toBe("manual-20261117090503");
    expect(validName("before-update")).toBe(true);
    for (const bad of ["", "-r", "a b", "a/b", "a@b", ".."]) expect(validName(bad)).toBe(false);
  });

  it("labels a btrfs source by its filesystem", () => {
    const t = (k: string) => k;
    expect(sourceLabel("btrfs", "/mnt/cache", "appdata", t)).toBe("/mnt/cache/appdata");
    expect(sourceLabel("btrfs", "/mnt/cache", "", t)).toBe("/mnt/cache (hostSnap.topLevel)");
    expect(sourceLabel("zfs", undefined, "tank/apps", t)).toBe("tank/apps");
  });

  it("finds the plan by the start of its id and has a sentence for every code", () => {
    const plan = { id: "1a2b3c4d5e" } as HostSnapPlan;
    expect(planOf(snap({ planId: "1a2b3c4d" }), [plan])).toBe(plan);
    expect(planOf(snap({}), [plan])).toBeUndefined();
    expect(codeKey("busy")).toBe("hostSnap.code.busy");
    expect(codeKey("something new")).toBe("hostSnap.code.failed");
  });
});

describe("needsQcow2", () => {
  it("names the VMs with raw disks and nothing else", () => {
    const src = (kind: "zfs" | "vm", source: string, reason?: string) => ({
      kind,
      source,
      group: "",
      usedBytes: -1,
      newSinceLatestBytes: -1,
      canRecursive: false,
      canCreate: !reason,
      reason,
    });
    expect(needsQcow2([src("vm", "win11", "needs-qcow2"), src("vm", "debian"), src("zfs", "tank")])).toEqual(["win11"]);
  });
});
