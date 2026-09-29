// The navigation math of Selector. Focus, clicks and the RTL read are tested in
// Selector.dom.test.tsx.
import { describe, expect, it } from "vitest";
import {
  MAX_PINNED_WIDTH,
  MIN_PINNED_WIDTH,
  nextFocusIndex,
  pinnedWidth,
  rovedIndex,
  stepFor,
  type SelectorNavKey,
} from "./Selector";

describe("stepFor", () => {
  it("ArrowRight steps +1 in LTR", () => {
    expect(stepFor("ArrowRight", false)).toBe(1);
  });

  it("ArrowLeft steps -1 in LTR", () => {
    expect(stepFor("ArrowLeft", false)).toBe(-1);
  });

  it("ArrowRight steps -1 under RTL (Arabic, Hebrew)", () => {
    expect(stepFor("ArrowRight", true)).toBe(-1);
  });

  it("ArrowLeft steps +1 under RTL", () => {
    expect(stepFor("ArrowLeft", true)).toBe(1);
  });

  it("returns 0 for Home and End, which nextFocusIndex handles as jumps", () => {
    expect(stepFor("Home", false)).toBe(0);
    expect(stepFor("End", true)).toBe(0);
  });
});

describe("nextFocusIndex", () => {
  it("Home always jumps to 0, regardless of current position or direction", () => {
    expect(nextFocusIndex("Home", 4, 6, false)).toBe(0);
    expect(nextFocusIndex("Home", 0, 6, true)).toBe(0);
  });

  it("End always jumps to the last index", () => {
    expect(nextFocusIndex("End", 0, 6, false)).toBe(5);
    expect(nextFocusIndex("End", 2, 6, true)).toBe(5);
  });

  it("ArrowRight moves forward one in LTR", () => {
    expect(nextFocusIndex("ArrowRight", 1, 5, false)).toBe(2);
  });

  it("ArrowLeft moves backward one in LTR", () => {
    expect(nextFocusIndex("ArrowLeft", 1, 5, false)).toBe(0);
  });

  it("ArrowRight moves backward under RTL", () => {
    expect(nextFocusIndex("ArrowRight", 2, 5, true)).toBe(1);
  });

  it("ArrowLeft moves forward under RTL", () => {
    expect(nextFocusIndex("ArrowLeft", 2, 5, true)).toBe(3);
  });

  it("wraps from the last item back to the first on ArrowRight (LTR)", () => {
    expect(nextFocusIndex("ArrowRight", 4, 5, false)).toBe(0);
  });

  it("wraps from the first item back to the last on ArrowLeft (LTR)", () => {
    expect(nextFocusIndex("ArrowLeft", 0, 5, false)).toBe(4);
  });

  it("wraps correctly under RTL too (ArrowRight from index 0 lands on the last item)", () => {
    expect(nextFocusIndex("ArrowRight", 0, 5, true)).toBe(4);
  });

  it("starts at the first item when nothing in the strip currently has focus (current = -1)", () => {
    expect(nextFocusIndex("ArrowRight", -1, 5, false)).toBe(0);
    expect(nextFocusIndex("ArrowLeft", -1, 5, true)).toBe(0);
  });

  it("returns -1 for an empty strip, for every key", () => {
    const keys: SelectorNavKey[] = ["ArrowRight", "ArrowLeft", "Home", "End"];
    for (const key of keys) {
      expect(nextFocusIndex(key, -1, 0, false)).toBe(-1);
    }
  });

  it("a single-item strip stays put on either arrow key (wraps to itself)", () => {
    expect(nextFocusIndex("ArrowRight", 0, 1, false)).toBe(0);
    expect(nextFocusIndex("ArrowLeft", 0, 1, false)).toBe(0);
  });
});

describe("rovedIndex", () => {
  it("prefers the active item when it isn't disabled", () => {
    expect(rovedIndex([false, false, false], 1)).toBe(1);
  });

  it("falls back to the first enabled item when the active item is disabled", () => {
    // Files.tsx disables its active "original" chip when there is no target
    // path, and the tab stop still has to be reachable.
    expect(rovedIndex([false, true, false], 1)).toBe(0);
  });

  it("falls back to the first enabled item when nothing is active (-1)", () => {
    expect(rovedIndex([true, false, false], -1)).toBe(1);
  });

  it("falls back to index 0 when every item is disabled", () => {
    expect(rovedIndex([true, true, true], -1)).toBe(0);
    expect(rovedIndex([true, true, true], 1)).toBe(0);
  });

  it("falls back to index 0 for an empty items list", () => {
    expect(rovedIndex([], -1)).toBe(0);
  });
});

describe("pinnedWidth", () => {
  const GAP = 3.2;

  it("takes the shared floor while the row holds that many", () => {
    expect(pinnedWidth(90, 3, 900, GAP)).toBe(MIN_PINNED_WIDTH);
  });

  it("takes the widest label where it needs more than the floor", () => {
    expect(pinnedWidth(260, 3, 900, GAP)).toBe(260);
  });

  it("gives the floor up for an even share of a phone's row", () => {
    // A 390px phone: the Theme card's groove holds 311.6px.
    expect(pinnedWidth(97, 2, 311.6, GAP)).toBe(154);
  });

  it("never goes below the widest label to fit a share", () => {
    expect(pinnedWidth(120, 4, 311.6, GAP)).toBe(120);
  });

  it("stops at the cap for a label longer than it", () => {
    expect(pinnedWidth(500, 3, 1400, GAP)).toBe(MAX_PINNED_WIDTH);
  });
});
