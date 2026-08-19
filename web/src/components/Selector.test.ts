// ---------------------------------------------------------------------------
// Selector — pure navigation math (GlimStone form-engine Phase 2, Task 3).
//
// Covers stepFor/nextFocusIndex/rovedIndex directly, with plain numbers and
// booleans — no DOM, no React renderer, matching this repo's established
// no-jsdom pattern for pure logic (Badge.test.ts, appearance.test.ts). The
// DOM-touching half (real focus movement, the RTL getComputedStyle() read,
// clicking/onChange wiring) is covered separately in Selector.dom.test.tsx.
// ---------------------------------------------------------------------------
import { describe, expect, it } from "vitest";
import { nextFocusIndex, rovedIndex, stepFor, type SelectorNavKey } from "./Selector";

describe("stepFor — direction of \"further along the strip\"", () => {
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

  it("Home/End don't step — they jump, handled separately by nextFocusIndex", () => {
    expect(stepFor("Home", false)).toBe(0);
    expect(stepFor("End", true)).toBe(0);
  });
});

describe("nextFocusIndex — roving-tabindex target for a keypress", () => {
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

  it("ArrowRight moves BACKWARD under RTL — the direction flips, not just the label", () => {
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

describe("rovedIndex — which item holds tabIndex 0", () => {
  it("prefers the active item when it isn't disabled", () => {
    expect(rovedIndex([false, false, false], 1)).toBe(1);
  });

  it("falls back to the first enabled item when the active item is disabled", () => {
    // Mirrors Files.tsx's destChip: the active "original" chip goes disabled
    // (no target path), so roving tabindex must land somewhere reachable.
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
