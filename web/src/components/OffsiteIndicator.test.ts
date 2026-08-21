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
    expect(offsiteStatusText(t, state({ snapshotIndex: 0, percent: 40 }), "")).toBe("Replicating…");
  });

  it("shows the live snapshot/percent tier once a real per-snapshot signal exists", () => {
    const got = offsiteStatusText(t, state({ snapshotIndex: 2, snapshotTotal: 4, percent: 62.6 }), "");
    expect(got).toBe("Replicating snapshot 2 of 4 (63%)"); // rounded
  });

  it("combines the snapshot/percent tier with a live duration when both are known", () => {
    const got = offsiteStatusText(t, state({ snapshotIndex: 1, snapshotTotal: 1, percent: 5 }), "13s");
    expect(got).toBe("Replicating snapshot 1 of 1 (5%) · 13s");
  });

  it("widens the displayed total to at least the live index if the estimate undercounted", () => {
    // The backend's upfront candidate count is a best-effort ESTIMATE (restic
    // never reports a real total) — if the live index ever exceeds it, the
    // displayed total must widen to match rather than claim fewer snapshots
    // than are visibly running (e.g. "snapshot 3 of 2").
    const got = offsiteStatusText(t, state({ snapshotIndex: 3, snapshotTotal: 2, percent: 10 }), "");
    expect(got).toBe("Replicating snapshot 3 of 3 (10%)");
  });

  it("clamps an out-of-range percent into 0..100 defensively", () => {
    expect(offsiteStatusText(t, state({ snapshotIndex: 1, snapshotTotal: 1, percent: 142 }), "")).toContain("(100%)");
    expect(offsiteStatusText(t, state({ snapshotIndex: 1, snapshotTotal: 1, percent: -5 }), "")).toContain("(0%)");
  });

  it("never renders a literal placeholder token (parity with the real translation)", () => {
    const got = offsiteStatusText(t, state({ snapshotIndex: 2, snapshotTotal: 4, percent: 50 }), "1m");
    expect(got).not.toMatch(/\{[a-zA-Z]+\}/);
  });
});
