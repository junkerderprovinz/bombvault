import { describe, expect, it } from "vitest";
import { perRowFor, segmentLayout } from "./segmentLayout";

const GAP = 3.2;
const fixed = (...widths: number[]) => widths.map((w) => ({ oneLine: w, narrowest: w }));

describe("perRowFor", () => {
  it("keeps every segment on one row while they fit", () => {
    expect(perRowFor(3, 900, 200, GAP)).toBe(3);
  });

  it("splits six that fit four to a row three and three", () => {
    expect(perRowFor(6, 4 * 200 + 3 * GAP + 50, 200, GAP)).toBe(3);
  });

  it("splits seven that fit four to a row four and three", () => {
    expect(perRowFor(7, 4 * 200 + 3 * GAP + 50, 200, GAP)).toBe(4);
  });

  it("puts one segment on a row when not even two fit", () => {
    expect(perRowFor(3, 250, 200, GAP)).toBe(1);
  });
});

describe("segmentLayout", () => {
  it("keeps the pinned width while every label fits it", () => {
    expect(segmentLayout(900, 200, fixed(120, 180, 90), GAP)).toEqual({ byContent: false, perRow: 3 });
  });

  it("lays the segments out by content when a label would wrap and they fit side by side", () => {
    const segments = [
      { oneLine: 420, narrowest: 180 },
      { oneLine: 90, narrowest: 90 },
      { oneLine: 90, narrowest: 90 },
    ];
    expect(segmentLayout(700, 200, segments, GAP)).toEqual({ byContent: true, perRow: 3 });
  });

  it("goes back to even rows at the pinned width when they do not fit side by side either", () => {
    const segments = [
      { oneLine: 420, narrowest: 180 },
      { oneLine: 300, narrowest: 300 },
      { oneLine: 90, narrowest: 90 },
    ];
    expect(segmentLayout(700, 200, segments, GAP)).toEqual({ byContent: false, perRow: 3 });
    expect(segmentLayout(500, 200, segments, GAP)).toEqual({ byContent: false, perRow: 2 });
  });

  it("keeps the pinned width for a single word longer than it, which cannot wrap", () => {
    expect(segmentLayout(900, 200, fixed(260, 90), GAP)).toEqual({ byContent: false, perRow: 2 });
  });
});
