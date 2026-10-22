import { describe, expect, it } from "vitest";
import type { StorageLocationSection } from "./api";
import { en, type TranslationKey } from "./i18n";
import type { ProgressState } from "./progress";
import { eraseKey, liveRun, moveKey, moveText, ownFolderKey, sectionMoveRun, storageRunning } from "./storageRuns";

const t = ((key: TranslationKey) => en[key]) as Parameters<typeof moveText>[0];

const state = (over: Partial<ProgressState> = {}): ProgressState => ({ phase: "replicate", percent: 0, active: true, lastSeen: 0, ...over });

describe("liveRun", () => {
  it("is the run under its key while it runs", () => {
    const run = state({ percent: 40 });
    expect(liveRun({ [moveKey("t1")]: run }, moveKey("t1"))).toBe(run);
  });

  it("is over once the run ended, though its entry lingers", () => {
    expect(liveRun({ [moveKey("t1")]: state({ finished: true }) }, moveKey("t1"))).toBeNull();
    expect(liveRun({}, eraseKey("box"))).toBeNull();
  });

  it("does not take another target's move for this one", () => {
    expect(liveRun({ [moveKey("t2")]: state() }, moveKey("t1"))).toBeNull();
  });
});

describe("storageRunning", () => {
  it("counts a move and a deletion, and nothing else", () => {
    expect(storageRunning({ [moveKey("t1")]: state() })).toBe(true);
    expect(storageRunning({ [eraseKey("box")]: state({ phase: "maintenance" }) })).toBe(true);
    expect(storageRunning({ "offsite:flash": state(), "prune:flash": state({ phase: "maintenance" }) })).toBe(false);
  });

  it("still counts a run that lingers, so its end is seen when the entry goes", () => {
    expect(storageRunning({ [eraseKey("box")]: state({ finished: true }) })).toBe(true);
  });

  it("counts the copy before an own folder moves", () => {
    expect(storageRunning({ [ownFolderKey("containers")]: state() })).toBe(true);
  });
});

describe("sectionMoveRun", () => {
  const home = { domain: "containers", use: "home", where: "user/backups/containers" } as StorageLocationSection;

  it("follows the copy before the section's own folder leaves on its home row", () => {
    const run = state({ snapshotIndex: 1, snapshotTotal: 4 });
    expect(sectionMoveRun({ [ownFolderKey("containers")]: run }, home)).toBe(run);
    expect(sectionMoveRun({ [ownFolderKey("vms")]: run }, home)).toBeNull();
  });

  it("leaves a named repository's row alone and follows a copy by its target", () => {
    const run = state();
    expect(sectionMoveRun({ [ownFolderKey("containers")]: run }, { ...home, repoId: "nas" })).toBeNull();
    expect(sectionMoveRun({ [moveKey("t1")]: run }, { ...home, use: "copy", targetId: "t1" })).toBe(run);
  });
});

describe("moveText", () => {
  it("says that the section moves while restic has not counted yet", () => {
    expect(moveText(t, state())).toBe("Moving");
  });

  it("names the backup the copy has reached", () => {
    expect(moveText(t, state({ snapshotIndex: 3, snapshotTotal: 12 }))).toBe("Moving · Backup 3 of 12");
  });
});
