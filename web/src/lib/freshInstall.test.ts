// @vitest-environment jsdom
import { beforeEach, describe, expect, it } from "vitest";

import type { BackupItem } from "./api";
import { isFreshInstall, rememberInUse, seenInUse } from "./freshInstall";

const entry = (over: Partial<BackupItem>) => ({ kind: "container", key: "plex", lastBackup: 0, ...over }) as BackupItem;

describe("isFreshInstall", () => {
  it("holds while no entry has a backup", () => {
    expect(isFreshInstall([entry({}), entry({ kind: "vm", key: "win11" })])).toBe(true);
  });

  it("holds for a server with nothing to back up", () => {
    expect(isFreshInstall([])).toBe(true);
  });

  it("ends with the first backup of any entry", () => {
    expect(isFreshInstall([entry({}), entry({ kind: "flash", key: "flash", lastBackup: 1_700_000_000 })])).toBe(false);
  });

  it("does not come back when the backed-up entry is gone from the host or its section is off", () => {
    expect(isFreshInstall([entry({ lastBackup: 1_700_000_000, installed: false })])).toBe(false);
    expect(isFreshInstall([entry({ lastBackup: 1_700_000_000, kindDisabled: true })])).toBe(false);
  });

  it("is not ended by a run that failed", () => {
    expect(isFreshInstall([entry({ lastRunStatus: "failed" })])).toBe(true);
  });

  it("ends with the first replica of an entry that only replicates", () => {
    const replicated = entry({ kind: "zfs", key: "tank/media", replica: { lastAt: 1_700_000_000, lastStatus: "success", only: true } });
    expect(isFreshInstall([replicated])).toBe(false);
  });

  it("is not ended by a replica that never succeeded", () => {
    expect(isFreshInstall([entry({ kind: "zfs", key: "tank/media", replica: { lastAt: 0, lastStatus: "failed", only: true } })])).toBe(true);
  });
});

describe("seenInUse", () => {
  beforeEach(() => localStorage.clear());

  it("holds once the entries were read with a backup among them", () => {
    expect(seenInUse()).toBe(false);
    rememberInUse(true);
    expect(seenInUse()).toBe(true);
  });

  it("ends when the entries are read without any backup again", () => {
    rememberInUse(true);
    rememberInUse(false);
    expect(seenInUse()).toBe(false);
  });
});
