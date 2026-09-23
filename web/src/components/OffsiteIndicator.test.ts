// Uses the real `en` table, so a placeholder that offsiteStatusText does not
// replace shows up as a literal "{index}" in the output.
import { describe, expect, it } from "vitest";
import { offsiteStatusText } from "./OffsiteIndicator";
import { countText, en } from "../lib/i18n";
import type { TranslationKey } from "../lib/i18n";
import type { ProgressState } from "../lib/progress";

const t = (key: TranslationKey, n?: number): string => countText(en[key], "en", n);

function state(over: Partial<ProgressState>): ProgressState {
  return { phase: "replicate", percent: 0, active: true, lastSeen: 0, ...over };
}

describe("offsiteStatusText", () => {
  it("returns the plain label when nothing is known", () => {
    expect(offsiteStatusText(t, undefined, "")).toBe("Replicating…");
  });

  it("returns the duration-only text when a duration is known but no snapshot index", () => {
    expect(offsiteStatusText(t, state({ startedAt: 100 }), "1m 2s")).toBe("Replicating… (1m 2s)");
  });

  it("ignores a snapshotIndex of 0 (not yet known) even if percent is a number", () => {
    expect(offsiteStatusText(t, state({ snapshotIndex: 0, snapshotTotal: 4, percent: 40 }), "")).toBe("Replicating…");
  });

  it("falls back when the backend could not estimate a snapshot total", () => {
    // No total means the backend could not estimate one (api.progBeginCopySink).
    // Dividing by the live index instead would claim about 93% here.
    expect(offsiteStatusText(t, state({ snapshotIndex: 15, percent: 55 }), "1m 2s")).toBe("Replicating… (1m 2s)");
  });

  it("shows the run-level percent once a per-snapshot signal exists", () => {
    // One snapshot done plus 62.6% of the second, out of 4: 40.65%, shown as 41.
    const got = offsiteStatusText(t, state({ snapshotIndex: 2, snapshotTotal: 4, percent: 62.6 }), "");
    expect(got).toBe("Replicating… 41% overall (snapshot 2 of 4)");
  });

  it("shows snapshot 15 of 126 at 55% as 12% overall, not 55%", () => {
    const got = offsiteStatusText(t, state({ snapshotIndex: 15, snapshotTotal: 126, percent: 55 }), "1h 7m");
    expect(got).toBe("Replicating… 12% overall (snapshot 15 of 126) · 1h 7m");
    expect(got).not.toContain("55");
  });

  it("combines the run-level percent tier with a live duration when both are known", () => {
    const got = offsiteStatusText(t, state({ snapshotIndex: 1, snapshotTotal: 1, percent: 5 }), "13s");
    expect(got).toBe("Replicating… 5% overall (snapshot 1 of 1) · 13s");
  });

  it("widens the displayed total to at least the live index if the estimate undercounted", () => {
    // The total is the backend's estimate (restic reports none), so a live
    // index past it widens the total instead of showing "snapshot 3 of 2".
    const got = offsiteStatusText(t, state({ snapshotIndex: 3, snapshotTotal: 2, percent: 10 }), "");
    expect(got).toBe("Replicating… 70% overall (snapshot 3 of 3)");
  });

  it("clamps an out-of-range percent, and never sits at 100% while still running", () => {
    // 100 would read as done through the retention and unlock tail of the run.
    expect(offsiteStatusText(t, state({ snapshotIndex: 1, snapshotTotal: 1, percent: 142 }), "")).toContain("99% overall");
    expect(offsiteStatusText(t, state({ snapshotIndex: 1, snapshotTotal: 1, percent: -5 }), "")).toContain("0% overall");
  });

  it("never renders a literal placeholder token (parity with the real translation)", () => {
    const got = offsiteStatusText(t, state({ snapshotIndex: 2, snapshotTotal: 4, percent: 50 }), "1m");
    expect(got).not.toMatch(/\{[a-zA-Z]+\}/);
  });
});
