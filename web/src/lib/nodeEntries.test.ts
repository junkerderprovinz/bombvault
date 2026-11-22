import { describe, expect, it } from "vitest";
import type { Container, FileSetView } from "./api";
import { rowStatus } from "./backupList";
import { containerItem, fileSetItem, nodeEntryPath, parseNodeEntryPath } from "./nodeEntries";

const container = {
  name: "nextcloud",
  image: "nextcloud:29",
  state: "running",
  ip: "172.18.0.4",
  installed: true,
  includeInSchedule: true,
  lastBackup: 1_700_000_000,
  stack: "cloud",
} as Container;

const fileSet = {
  id: "s1",
  name: "Documents",
  enabled: false,
  lastBackup: 0,
  effectiveSchedule: { kind: "none", spec: "", reason: "excluded" },
} as FileSetView;

describe("node entries", () => {
  it("lists a container of a node the way the list shows its own", () => {
    expect(containerItem(container)).toMatchObject({
      kind: "container",
      key: "nextcloud",
      name: "nextcloud",
      included: true,
      lastBackup: 1_700_000_000,
      state: "running",
      installed: true,
      ip: "172.18.0.4",
      image: "nextcloud:29",
      runs: [],
    });
  });

  it("counts a container that was never backed up as not backed up", () => {
    expect(containerItem({ ...container, lastBackup: null }).lastBackup).toBe(0);
  });

  it("lists a folder set by its id and keeps whether it is in the schedule", () => {
    expect(fileSetItem(fileSet)).toMatchObject({ kind: "files", key: "s1", name: "Documents", included: false });
    expect(rowStatus(fileSetItem(fileSet), 0)).toEqual({ is: "notProtected" });
  });

  it("counts a folder set the node backed up as protected, whatever schedule the node reports", () => {
    const backedUp = { ...fileSet, enabled: true, lastBackup: 1_700_000_000, effectiveSchedule: { kind: "none", spec: "", reason: "domainOff" } } as FileSetView;
    expect(rowStatus(fileSetItem(backedUp), 0)).toEqual({ is: "last", quiet: true });
  });

  it("addresses an entry under its node and reads the address back", () => {
    const path = nodeEntryPath("n 1", "container", "plex/tv");
    expect(path).toBe("/nodes/n%201/backups/container/plex%2Ftv");
    expect(parseNodeEntryPath("n 1", "container", "plex/tv")).toEqual({ node: "n 1", kind: "container", key: "plex/tv" });
    expect(parseNodeEntryPath("n1", "vm", "win")).toBeNull();
  });
});
