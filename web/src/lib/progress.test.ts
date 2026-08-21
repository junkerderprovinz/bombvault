// ---------------------------------------------------------------------------
// offsiteRunProgress — the single place the off-site run-level percentage is
// derived (issue #159). Both surfaces that show it (ActivityLog's live line and
// OffsiteIndicator) call this, so the arithmetic is pinned here once instead of
// twice in two component tests that could drift apart the way the strings did.
//
// Pure logic, node environment: importing ./progress only defines the module's
// functions — its EventSource plumbing all lives inside them.
// ---------------------------------------------------------------------------
import { describe, expect, it } from "vitest";
import { offsiteRunProgress } from "./progress";
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
    // The honest "could not estimate" (api.progBeginCopySink). Dividing by the
    // live index instead would compute (15-1+0.55)/15 = 97% for a run that has
    // barely started — the single worst thing this helper could do.
    expect(offsiteRunProgress(state({ snapshotIndex: 15, percent: 55 }))).toBeNull();
    expect(offsiteRunProgress(state({ snapshotIndex: 15, snapshotTotal: 0, percent: 55 }))).toBeNull();
  });

  // The exact frame from issue #159's report. 15 of 126 at 55% is NOT 55% of
  // the run — 55 is snapshot 15's own pack-copy progress, which restarts at 0
  // for every snapshot. 14 whole snapshots plus 55% of the 15th, out of 126.
  it("folds the current snapshot's own progress into the snapshot count", () => {
    expect(offsiteRunProgress(state({ snapshotIndex: 15, snapshotTotal: 126, percent: 55 }))).toEqual({
      percent: 12,
      index: 15,
      total: 126,
    });
  });

  it("agrees with the plain k/N fraction it is rendered next to", () => {
    // The whole point of the new phrasing: "12% overall (snapshot 15 of 126)"
    // must corroborate itself. 15/126 = 11.9%, and the derived figure is 12%.
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
    // Parking at 100% through the retention/unlock tail of a run is exactly the
    // "is it stuck?" impression #159 was reported about.
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
    // The old readout swung 0 -> 100 once per snapshot while "of N" crawled,
    // which is what made a healthy hour-long run look broken.
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
