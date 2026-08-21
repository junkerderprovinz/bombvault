// ---------------------------------------------------------------------------
// offsiteStatusText — pure-function tests for the tiering logic behind
// OffsiteIndicator's live status text (issue #159). Runs against the REAL
// `en` translation table (not a stub), so a drift between a key's placeholder
// tokens and what this function actually replaces would show up here as a
// literal "{index}"/"{total}"/"{percent}" surviving in the output.
//
// See OffsiteIndicator.dom.test.tsx for the full-stack DOM/SSE integration
// coverage of the same behaviour.
// ---------------------------------------------------------------------------
import { describe, expect, it } from "vitest";
import { offsiteStatusText } from "./OffsiteIndicator";
import { en } from "../lib/i18n";
import type { TranslationKey } from "../lib/i18n";
import type { ProgressState } from "../lib/progress";

const t = (key: TranslationKey): string => en[key];

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
    // snapshotIndex 0 means "not attributed to a live snapshot yet" — must NOT
    // render "snapshot 0 of ...".
    expect(offsiteStatusText(t, state({ snapshotIndex: 0, snapshotTotal: 4, percent: 40 }), "")).toBe("Replicating…");
  });

  it("falls back when the backend could not estimate a snapshot total", () => {
    // snapshotTotal 0/absent is the wire's honest "unknown" (see
    // api.progBeginCopySink). Without a denominator there is no run-level
    // percentage to state, and dividing by the live index instead would claim
    // ~93% for a run that has barely started.
    expect(offsiteStatusText(t, state({ snapshotIndex: 15, percent: 55 }), "1m 2s")).toBe("Replicating… (1m 2s)");
  });

  it("shows the RUN-LEVEL percent tier once a real per-snapshot signal exists", () => {
    // 1 whole snapshot done + 62.6% of the second, out of 4 => 40.65% => 41.
    // The old rendering printed 63 here — snapshot 2's own pack progress — in
    // parentheses right after "2 of 4", which reads as a claim about the run.
    const got = offsiteStatusText(t, state({ snapshotIndex: 2, snapshotTotal: 4, percent: 62.6 }), "");
    expect(got).toBe("Replicating… 41% overall (snapshot 2 of 4)");
  });

  it("renders issue #159's reported 15-of-126-at-55% case as ~12%, never 55%", () => {
    const got = offsiteStatusText(t, state({ snapshotIndex: 15, snapshotTotal: 126, percent: 55 }), "1h 7m");
    expect(got).toBe("Replicating… 12% overall (snapshot 15 of 126) · 1h 7m");
    expect(got).not.toContain("55");
  });

  it("combines the run-level percent tier with a live duration when both are known", () => {
    const got = offsiteStatusText(t, state({ snapshotIndex: 1, snapshotTotal: 1, percent: 5 }), "13s");
    expect(got).toBe("Replicating… 5% overall (snapshot 1 of 1) · 13s");
  });

  it("widens the displayed total to at least the live index if the estimate undercounted", () => {
    // The backend's upfront candidate count is a best-effort ESTIMATE (restic
    // never reports a real total) — if the live index ever exceeds it, the
    // displayed total must widen to match rather than claim fewer snapshots
    // than are visibly running (e.g. "snapshot 3 of 2").
    const got = offsiteStatusText(t, state({ snapshotIndex: 3, snapshotTotal: 2, percent: 10 }), "");
    expect(got).toBe("Replicating… 70% overall (snapshot 3 of 3)");
  });

  it("clamps an out-of-range percent, and never sits at 100% while still running", () => {
    // 100 would park the readout at "done" through the retention/unlock tail of
    // the run — the exact "is it stuck?" impression #159 was reported about.
    expect(offsiteStatusText(t, state({ snapshotIndex: 1, snapshotTotal: 1, percent: 142 }), "")).toContain("99% overall");
    expect(offsiteStatusText(t, state({ snapshotIndex: 1, snapshotTotal: 1, percent: -5 }), "")).toContain("0% overall");
  });

  it("never renders a literal placeholder token (parity with the real translation)", () => {
    const got = offsiteStatusText(t, state({ snapshotIndex: 2, snapshotTotal: 4, percent: 50 }), "1m");
    expect(got).not.toMatch(/\{[a-zA-Z]+\}/);
  });
});
