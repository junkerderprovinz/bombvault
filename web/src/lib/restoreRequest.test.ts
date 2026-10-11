import { describe, expect, it } from "vitest";
import type { Run } from "./api";
import { readRestoreRequest, runRestorePath } from "./restoreRequest";

function run(over: Partial<Run> = {}): Run {
  return {
    id: "r1",
    targetId: "tg-plex",
    kind: "backup",
    status: "success",
    startedAt: 1_700_000_000,
    finishedAt: 1_700_000_060,
    snapshotId: "abc123",
    bytes: 1,
    error: "",
    acknowledged: false,
    target: "plex",
    domain: "container",
    ...over,
  };
}

describe("the restore link of a run", () => {
  it.each([
    ["a container by its name", run(), "/backups/container/plex"],
    ["a VM by its libvirt name", run({ domain: "vm", target: "win11" }), "/backups/vm/win11"],
    ["a folder set by its id", run({ domain: "files", target: "Photos", targetId: "set-1" }), "/backups/files/set-1"],
    ["a ZFS item by its id", run({ domain: "zfs", target: "tank/apps", targetId: "zfs-1" }), "/backups/zfs/zfs-1"],
    ["the flash drive", run({ domain: "flash", target: "flash", targetId: "flash" }), "/backups/flash/flash"],
  ])("names %s", (_, r, page) => {
    expect(runRestorePath(r)).toBe(`${page}?restore=abc123&at=1700000060`);
  });

  it("is read back as the request it was written as", () => {
    const [, search] = (runRestorePath(run()) ?? "").split("?");
    expect(readRestoreRequest(`?${search}`)).toMatchObject({ snapshot: "abc123", at: 1_700_000_060, dump: false });
  });

  it.each([
    ["failed", run({ status: "failed" })],
    ["wrote no snapshot", run({ snapshotId: "" })],
    ["was a restore", run({ kind: "restore" })],
    ["covered everything", run({ domain: "everything" })],
  ])("is missing for a run that %s", (_, r) => {
    expect(runRestorePath(r)).toBeNull();
  });
});
