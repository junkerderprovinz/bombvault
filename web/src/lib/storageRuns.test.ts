import { describe, expect, it } from "vitest";
import { en, type TranslationKey } from "./i18n";
import type { ProgressState } from "./progress";
import { eraseKey, liveRun, moveKey, moveText, storageRunning } from "./storageRuns";

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
});

describe("moveText", () => {
  it("says that the section moves while restic has not counted yet", () => {
    expect(moveText(t, state())).toBe("Moving");
  });

  it("names the backup the copy has reached", () => {
    expect(moveText(t, state({ snapshotIndex: 3, snapshotTotal: 12 }))).toBe("Moving · Backup 3 of 12");
  });
});
