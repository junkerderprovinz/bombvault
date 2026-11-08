import { describe, expect, it } from "vitest";
import { segmentFit } from "./segmentLayout";

const GAP = 3.2;

describe("segmentFit", () => {
  it("keeps the pinned width while the segments fit side by side at it", () => {
    expect(segmentFit(900, 200, [120, 180, 90], GAP, false)).toEqual({ fit: "even", cut: [false, false, false] });
  });

  it("shares the row by the labels when the pinned widths do not fit", () => {
    expect(segmentFit(360, 200, [100, 110, 120], GAP, false).fit).toBe("content");
  });

  it("shares the row by the labels when one is longer than the pinned width", () => {
    expect(segmentFit(900, 200, [90, 260, 90], GAP, false).fit).toBe("content");
  });

  it("keeps the labels of a hugging strip while they fit", () => {
    expect(segmentFit(400, null, [80, 90, 100], GAP, false).fit).toBe("content");
  });

  it("drops the labels for glyphs when they do not fit and every segment has one", () => {
    expect(segmentFit(300, 200, [140, 150, 160], GAP, true)).toEqual({ fit: "squeezed", cut: [false, false, false] });
  });

  it("shares the row evenly and cuts only the labels too long for a share when a segment has no glyph", () => {
    // (300 - 2 * 3.2) / 3 leaves each segment 97.87px.
    expect(segmentFit(300, 200, [60, 150, 97], GAP, false)).toEqual({ fit: "truncated", cut: [false, true, false] });
  });

  it("does not cut a row that fits to the pixel short over clientWidth rounding", () => {
    expect(segmentFit(299, null, [100, 100, 99.6], 0, false).fit).toBe("content");
  });
});
