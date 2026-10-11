import { describe, expect, it } from "vitest";

import type { ProgressState } from "../../lib/progress";
import { tallyEntries } from "../../lib/protectionTally";
import { headlineOf, liveRunOf, passOf, passRuns, passShare, stationOf } from "./headState";
import { domain, item, run } from "./overview.testsupport";
import { sectionsOf } from "./protectionChecks";

const live = (over: Partial<ProgressState> = {}): ProgressState => ({
  phase: "backup",
  percent: 0,
  active: true,
  lastSeen: 0,
  ...over,
});

describe("headlineOf", () => {
  const backedUp = [item()];
  const green = sectionsOf([domain()], backedUp, {}, null);
  const head = (over: Partial<Parameters<typeof headlineOf>[0]> = {}) =>
    headlineOf({ needs: 0, tally: tallyEntries(backedUp), sections: green, notes: 0, ...over });

  it("says everything is backed up only when nothing is left to say", () => {
    expect(head()).toBe("allGood");
  });

  it("puts what needs the person before everything else", () => {
    expect(head({ needs: 2, tally: tallyEntries([]) })).toBe("needs");
  });

  it("waits for the first backup while no entry has one", () => {
    expect(head({ tally: tallyEntries([item({ lastBackup: 0 })]) })).toBe("waiting");
    expect(head({ tally: tallyEntries([]) })).toBe("waiting");
  });

  it("says not everything is protected while an entry waits", () => {
    expect(head({ tally: tallyEntries([item(), item({ key: "new", lastBackup: 0 })]) })).toBe("partly");
  });

  it("never rounds an open note up to all good", () => {
    expect(head({ notes: 1 })).toBe("notes");
  });

  it("never rounds a section's warning up to all good", () => {
    const overdueCopy = sectionsOf([domain({ replicationState: "overdue" })], backedUp, {}, null);
    expect(head({ sections: overdueCopy })).toBe("notes");
    const noOffsite = sectionsOf([domain({ offsiteConfigured: false })], backedUp, {}, null);
    expect(head({ sections: noOffsite })).toBe("notes");
  });
});

describe("stationOf with the step the server names", () => {
  it("stands where the server says a container or a VM is", () => {
    expect(stationOf("container", live({ step: "stop" }))).toBe("stop");
    expect(stationOf("container", live({ step: "read" }))).toBe("read");
    expect(stationOf("vm", live({ step: "start", percent: 100 }))).toBe("start");
  });

  it("does not move on by the percentage while the server still stops the container", () => {
    expect(stationOf("container", live({ step: "stop", percent: 40 }))).toBe("stop");
  });

  it("keeps reading until the server says the containers start again", () => {
    expect(stationOf("container", live({ step: "read", percent: 100, committed: true }))).toBe("read");
  });

  it("copies while an off-site copy runs, whatever step its frame carries", () => {
    expect(stationOf(null, live({ phase: "replicate", step: "read" }))).toBe("copy");
  });
});

describe("stationOf without a step", () => {
  it("stands at the stop until a container's backup starts reading", () => {
    expect(stationOf("container", live())).toBe("stop");
    expect(stationOf("vm", live())).toBe("stop");
  });

  it("reads once the percentage moves", () => {
    expect(stationOf("container", live({ percent: 12 }))).toBe("read");
  });

  it("reads from the start for a kind that is not stopped", () => {
    expect(stationOf("files", live())).toBe("read");
    expect(stationOf("flash", live())).toBe("read");
  });

  it("reads while a database dump streams, which reports no percentage", () => {
    expect(stationOf("container", live({ stage: "dbdump" }))).toBe("read");
  });

  it("starts again once the backup is written", () => {
    expect(stationOf("container", live({ percent: 100, committed: true }))).toBe("start");
  });

  it("copies while an off-site copy runs", () => {
    expect(stationOf(null, live({ phase: "replicate", percent: 40 }))).toBe("copy");
  });
});

describe("liveRunOf", () => {
  it("takes its station from the step of the run it follows", () => {
    expect(liveRunOf({ "container:plex": live({ percent: 30, step: "stop" }) })).toMatchObject({ station: "stop" });
  });

  it("follows the entry a pass is at, not the pass", () => {
    const pass = live({ percent: 20, done: 1, total: 5, unit: "items" });
    expect(liveRunOf({ everything: pass })).toBeNull();
    expect(liveRunOf({ everything: pass, "vm:win": live() })).toMatchObject({ kind: "vm", name: "win" });
  });

  it("is null while nothing runs", () => {
    expect(liveRunOf({})).toBeNull();
    expect(liveRunOf({ "container:plex": live({ active: false }) })).toBeNull();
  });

  it("follows a container by its name", () => {
    expect(liveRunOf({ "container:plex": live({ percent: 30 }) })).toMatchObject({
      kind: "container",
      name: "plex",
      percent: 30,
      station: "read",
      stops: true,
    });
  });

  it("keeps a colon inside a name", () => {
    expect(liveRunOf({ "files:Fotos: Urlaub": live() })).toMatchObject({ kind: "files", name: "Fotos: Urlaub", stops: false });
  });

  it("has no name for a kind with one entry", () => {
    expect(liveRunOf({ flash: live() })).toMatchObject({ kind: "flash", name: "" });
  });

  it("prefers a backup to an off-site copy", () => {
    const run = liveRunOf({
      "offsite:containers": live({ phase: "replicate" }),
      "vm:win": live(),
    });
    expect(run).toMatchObject({ kind: "vm", name: "win" });
  });

  it("follows an off-site copy through its snapshots", () => {
    const run = liveRunOf({
      "offsite:containers": live({ phase: "replicate", percent: 50, snapshotIndex: 2, snapshotTotal: 4 }),
    });
    expect(run).toMatchObject({ kind: null, name: "containers", station: "copy", percent: 38, step: { index: 2, total: 4 } });
  });

  it("follows the move of a section like a copy, without a section to name", () => {
    const run = liveRunOf({ "move:t-box-flash": live({ phase: "replicate", percent: 0, snapshotIndex: 3, snapshotTotal: 12 }) });
    expect(run).toMatchObject({ kind: null, name: "", station: "copy", step: { index: 3, total: 12 } });
  });

  it("leaves a restore and maintenance alone", () => {
    expect(liveRunOf({ "container:plex": live({ phase: "restore" }), "prune:containers": live({ phase: "maintenance" }) })).toBeNull();
  });

  it("lets go of a run that has ended and only lingers", () => {
    expect(liveRunOf({ "container:plex": live({ percent: 100, finished: true }) })).toBeNull();
  });
});

describe("the pass over everything", () => {
  const counting = (done: number, total: number, over: Partial<ProgressState> = {}) => ({
    everything: live({ done, total, unit: "items", ...over }),
  });

  it("is null while no pass runs", () => {
    expect(passOf({})).toBeNull();
    expect(passOf({ "container:plex": live() })).toBeNull();
  });

  it("counts the entries it is through", () => {
    expect(passOf(counting(3, 14))).toEqual({ done: 3, total: 14 });
  });

  it("starts at none, because the server leaves a zero out", () => {
    expect(passOf({ everything: live({ total: 14, unit: "items" }) })).toEqual({ done: 0, total: 14 });
  });

  it("ends with its last frame, not when that frame is dropped", () => {
    expect(passOf(counting(14, 14, { percent: 100, finished: true }))).toBeNull();
  });

  it("shares out the ring by the entries behind it", () => {
    expect(passShare({ done: 3, total: 12 }, null)).toBe(25);
  });

  it("adds the part of the entry being backed up", () => {
    const at = liveRunOf({ "container:plex": live({ percent: 50 }) });
    expect(passShare({ done: 3, total: 12 }, at)).toBeCloseTo(29.17, 2);
  });

  it("does not count an off-site copy as part of an entry", () => {
    const copy = liveRunOf({ "offsite:containers": live({ phase: "replicate", percent: 50 }) });
    expect(passShare({ done: 3, total: 12 }, copy)).toBe(25);
  });

  it("shows an empty ring for a pass with nothing to back up", () => {
    expect(passShare({ done: 0, total: 0 }, null)).toBe(0);
  });

  it("never passes the full ring when the total was corrected downwards", () => {
    expect(passShare({ done: 5, total: 4 }, null)).toBe(100);
  });
});

describe("passRuns", () => {
  const parent = run({ id: "all", domain: "everything", targetId: "everything", status: "running", finishedAt: null });
  const child = (id: string, over: Partial<Parameters<typeof run>[0]> = {}) => run({ id, groupId: "all", ...over });

  it("lists the backups the running pass has finished, oldest first", () => {
    const runs = [child("c", { status: "running", finishedAt: null }), child("b", { status: "failed" }), child("a"), parent];
    expect(passRuns(runs).map((r) => [r.id, r.status])).toEqual([
      ["a", "success"],
      ["b", "failed"],
    ]);
  });

  it("leaves out a dump that ran for one of its entries", () => {
    expect(passRuns([child("d", { kind: "dbdump" }), child("a"), parent]).map((r) => r.id)).toEqual(["a"]);
  });

  it("leaves out runs of an earlier pass and runs outside one", () => {
    const runs = [child("a"), run({ id: "solo" }), run({ id: "old", groupId: "before" }), parent];
    expect(passRuns(runs).map((r) => r.id)).toEqual(["a"]);
  });

  it("is empty once the pass is over", () => {
    expect(passRuns([child("a"), { ...parent, status: "success", finishedAt: 1 }])).toEqual([]);
  });
});
