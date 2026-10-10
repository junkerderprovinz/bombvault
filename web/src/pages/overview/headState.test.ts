import { describe, expect, it } from "vitest";

import type { ProgressState } from "../../lib/progress";
import { tallyEntries } from "../../lib/protectionTally";
import { headlineOf, liveRunOf, stationOf } from "./headState";
import { domain, item } from "./overview.testsupport";
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

describe("stationOf", () => {
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

  it("leaves a restore and maintenance alone", () => {
    expect(liveRunOf({ "container:plex": live({ phase: "restore" }), "prune:containers": live({ phase: "maintenance" }) })).toBeNull();
  });

  it("lets go of a run that has ended and only lingers", () => {
    expect(liveRunOf({ "container:plex": live({ percent: 100, finished: true }) })).toBeNull();
  });
});
