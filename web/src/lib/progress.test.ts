// offsiteRunProgress is the one place the off-site run percentage is derived,
// shared by ActivityLog's live line and OffsiteIndicator.
//
// Node environment: importing ./progress only defines functions, the
// EventSource is created inside them.
import { describe, expect, it } from "vitest";
import { anyActive, busyPhraseKey, offsiteRunProgress, parseProgressFrame } from "./progress";
import type { ProgressState } from "./progress";

function state(over: Partial<ProgressState>): ProgressState {
  return { phase: "replicate", percent: 0, active: true, lastSeen: 0, ...over };
}

describe("offsiteRunProgress", () => {
  it("returns null when there is no live snapshot signal at all", () => {
    expect(offsiteRunProgress(undefined)).toBeNull();
    expect(offsiteRunProgress(state({}))).toBeNull();
  });

  it("returns null for a snapshotIndex of 0 (not attributed to a snapshot yet)", () => {
    expect(offsiteRunProgress(state({ snapshotIndex: 0, snapshotTotal: 4, percent: 40 }))).toBeNull();
  });

  it("returns null when the backend reported no snapshot total to divide by", () => {
    // No total means the backend could not estimate one (api.progBeginCopySink).
    // Dividing by the live index instead would read (15-1+0.55)/15 = 97% for a
    // run that has barely started.
    expect(offsiteRunProgress(state({ snapshotIndex: 15, percent: 55 }))).toBeNull();
    expect(offsiteRunProgress(state({ snapshotIndex: 15, snapshotTotal: 0, percent: 55 }))).toBeNull();
  });

  // 15 of 126 at 55% is not 55% of the run: 55 is snapshot 15's own pack-copy
  // progress, which restarts at 0 for every snapshot. 14 whole snapshots plus
  // 55% of the 15th, out of 126.
  it("folds the current snapshot's own progress into the snapshot count", () => {
    expect(offsiteRunProgress(state({ snapshotIndex: 15, snapshotTotal: 126, percent: 55 }))).toEqual({
      percent: 12,
      index: 15,
      total: 126,
    });
  });

  it("agrees with the plain k/N fraction it is rendered next to", () => {
    // "12% overall (snapshot 15 of 126)" has to agree with itself: 15/126 is
    // 11.9%.
    const run = offsiteRunProgress(state({ snapshotIndex: 15, snapshotTotal: 126, percent: 55 }));
    const naive = Math.round((15 / 126) * 100);
    expect(Math.abs((run?.percent ?? 0) - naive)).toBeLessThanOrEqual(1);
  });

  it("widens an undercounting estimate to the live index", () => {
    // "snapshot 3 of 2" would be worse than a slightly optimistic total.
    expect(offsiteRunProgress(state({ snapshotIndex: 3, snapshotTotal: 2, percent: 10 }))).toEqual({
      percent: 70,
      index: 3,
      total: 3,
    });
  });

  it("clamps an out-of-range percent and never reports 100 while still running", () => {
    // A bar parked at 100% through the retention and unlock tail of a run
    // looks stuck.
    expect(offsiteRunProgress(state({ snapshotIndex: 1, snapshotTotal: 1, percent: 142 }))?.percent).toBe(99);
    expect(offsiteRunProgress(state({ snapshotIndex: 1, snapshotTotal: 1, percent: -5 }))?.percent).toBe(0);
    expect(offsiteRunProgress(state({ snapshotIndex: 126, snapshotTotal: 126, percent: 100 }))?.percent).toBe(99);
  });

  it("rejects non-finite values instead of rendering NaN", () => {
    expect(offsiteRunProgress(state({ snapshotIndex: 2, snapshotTotal: 4, percent: NaN }))).toBeNull();
    expect(offsiteRunProgress(state({ snapshotIndex: NaN, snapshotTotal: 4, percent: 50 }))).toBeNull();
    expect(offsiteRunProgress(state({ snapshotIndex: 2, snapshotTotal: Infinity, percent: 50 }))).toBeNull();
  });

  it("advances monotonically across a snapshot boundary instead of sawtoothing", () => {
    // A per-snapshot percentage swings from 0 to 100 once per snapshot, which
    // makes a healthy hour-long run look broken.
    const seq = [
      offsiteRunProgress(state({ snapshotIndex: 3, snapshotTotal: 10, percent: 10 }))!.percent,
      offsiteRunProgress(state({ snapshotIndex: 3, snapshotTotal: 10, percent: 90 }))!.percent,
      offsiteRunProgress(state({ snapshotIndex: 4, snapshotTotal: 10, percent: 0 }))!.percent,
      offsiteRunProgress(state({ snapshotIndex: 4, snapshotTotal: 10, percent: 50 }))!.percent,
    ];
    for (let i = 1; i < seq.length; i++) {
      expect(seq[i]).toBeGreaterThanOrEqual(seq[i - 1]);
    }
  });
});

describe("parseProgressFrame", () => {
  it("carries the dump stage and its byte count", () => {
    expect(parseProgressFrame('{"key":"container:immich","phase":"backup","percent":0,"active":true,"stage":"dbdump","bytes":4096}')).toEqual({
      key: "container:immich",
      phase: "backup",
      percent: 0,
      active: true,
      startedAt: undefined,
      snapshotIndex: undefined,
      snapshotTotal: undefined,
      stage: "dbdump",
      bytes: 4096,
    });
  });

  it("carries the save and import stages", () => {
    expect(parseProgressFrame('{"key":"container:immich","phase":"restore","active":true,"stage":"dbdumpsave"}')?.stage).toBe("dbdumpsave");
    expect(parseProgressFrame('{"key":"container:immich","phase":"restore","active":true,"stage":"dbimport"}')?.stage).toBe("dbimport");
  });

  it("drops a stage this client does not know", () => {
    const frame = parseProgressFrame('{"key":"container:immich","phase":"backup","active":true,"stage":"dbvacuum"}');
    expect(frame?.stage).toBeUndefined();
  });

  it("keeps rejecting a frame without a key", () => {
    expect(parseProgressFrame('{"phase":"backup","active":true}')).toBeNull();
    expect(parseProgressFrame("not json")).toBeNull();
  });
});

describe("busyPhraseKey", () => {
  it("words the phase when nothing more precise is running", () => {
    expect(busyPhraseKey("backup")).toBe("common.backupRunning");
    expect(busyPhraseKey("restore")).toBe("common.restoreRunning");
    expect(busyPhraseKey("replicate")).toBe("common.replicateRunning");
  });

  it("names the database step instead of the phase around it", () => {
    expect(busyPhraseKey("backup", "dbdump")).toBe("dbdump.busyDumping");
    expect(busyPhraseKey("restore", "dbdumpsave")).toBe("dbdump.busySaving");
    expect(busyPhraseKey("restore", "dbimport")).toBe("dbdump.busyImporting");
  });
});

describe("anyActive", () => {
  it("reports the stage of the run it found, so the hint can name it", () => {
    expect(anyActive({ "container:immich": { phase: "backup", active: true, stage: "dbdump" } })).toEqual({
      active: true,
      phase: "backup",
      stage: "dbdump",
    });
  });
});
