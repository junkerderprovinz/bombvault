// ---------------------------------------------------------------------------
// bubblePosition — pure placement math, exercised directly with plain
// numbers (no DOM, no React renderer), matching this repo's established
// no-jsdom pattern for pure logic (Selector.test.ts, appearance.test.ts).
// ---------------------------------------------------------------------------
import { describe, expect, it } from "vitest";
import { computeBubblePosition } from "./bubblePosition";

const VIEWPORT = { width: 1024, height: 768 };
const SMALL_BUBBLE = { width: 200, height: 60 };

describe("computeBubblePosition — horizontal clamp", () => {
  it("centres on the trigger when there's room on both sides", () => {
    const trigger = { left: 500, right: 540, top: 100, bottom: 120 };
    const pos = computeBubblePosition(trigger, SMALL_BUBBLE, VIEWPORT);
    expect(pos.left).toBe(520); // trigger centre, untouched
  });

  it("clamps a right-edge trigger so the bubble's right side stays inside the viewport", () => {
    // Trigger hugging the right edge — reproduces the reported
    // "Wiederherstellungskit" overflow shape: naive centring would put
    // left at ~1010, pushing half the 200px-wide bubble past width 1024.
    const trigger = { left: 1000, right: 1020, top: 100, bottom: 120 };
    const pos = computeBubblePosition(trigger, SMALL_BUBBLE, VIEWPORT);
    // Bubble is translateX(-50%)'d around `left`, so its right edge is
    // left + halfWidth — must stay within margin of the viewport edge.
    expect(pos.left + SMALL_BUBBLE.width / 2).toBeLessThanOrEqual(VIEWPORT.width - 8);
    expect(pos.left).toBe(1024 - 8 - 100); // viewport.width - margin - halfWidth
  });

  it("clamps a left-edge trigger so the bubble's left side stays inside the viewport", () => {
    const trigger = { left: 5, right: 25, top: 100, bottom: 120 };
    const pos = computeBubblePosition(trigger, SMALL_BUBBLE, VIEWPORT);
    expect(pos.left - SMALL_BUBBLE.width / 2).toBeGreaterThanOrEqual(8);
    expect(pos.left).toBe(8 + 100); // margin + halfWidth
  });

  it("a wide bubble on a narrow trigger clamps the same way, using the bubble's own real width", () => {
    const trigger = { left: 990, right: 1010, top: 50, bottom: 70 };
    const wideBubble = { width: 280, height: 100 }; // the CSS max-width
    const pos = computeBubblePosition(trigger, wideBubble, VIEWPORT);
    expect(pos.left + wideBubble.width / 2).toBeLessThanOrEqual(VIEWPORT.width - 8);
  });
});

describe("computeBubblePosition — vertical flip", () => {
  it("opens below the trigger by default (matches the pre-fix behaviour when there's room)", () => {
    const trigger = { left: 100, right: 140, top: 100, bottom: 120 };
    const pos = computeBubblePosition(trigger, SMALL_BUBBLE, VIEWPORT);
    expect(pos.above).toBe(false);
    expect(pos.top).toBe(120 + 8); // trigger.bottom + margin
  });

  it("flips above the trigger when a tall bubble opening below would clip the viewport's bottom edge", () => {
    // Reproduces the actual recovery.why case: a long tip wraps to a tall
    // bubble, and the trigger sits low enough in a scrolled page that
    // opening downward would run past the window's bottom edge.
    const trigger = { left: 300, right: 340, top: 700, bottom: 720 };
    const tallBubble = { width: 260, height: 140 };
    const pos = computeBubblePosition(trigger, tallBubble, VIEWPORT);
    expect(pos.above).toBe(true);
    expect(pos.top).toBe(700 - 8 - 140); // trigger.top - margin - height
    expect(pos.top).toBeGreaterThanOrEqual(0);
    expect(pos.top + tallBubble.height).toBeLessThanOrEqual(VIEWPORT.height);
  });

  it("stays below (does NOT flip) when the trigger is pinned near the very top, even if opening below would clip the bottom", () => {
    // A trigger with no room above must keep opening downward — flipping
    // into negative space would just trade one clipped edge for another
    // (reference/tooltip.ts's own documented edge case).
    const trigger = { left: 300, right: 340, top: 4, bottom: 24 };
    const tallBubble = { width: 260, height: 900 }; // taller than the whole viewport
    const pos = computeBubblePosition(trigger, tallBubble, VIEWPORT);
    expect(pos.above).toBe(false);
    expect(pos.top).toBe(24 + 8);
  });

  it("stays below when the bubble fits comfortably even though the trigger is near the bottom", () => {
    const trigger = { left: 300, right: 340, top: 730, bottom: 750 };
    const tinyBubble = { width: 120, height: 10 };
    const pos = computeBubblePosition(trigger, tinyBubble, VIEWPORT);
    expect(pos.above).toBe(false);
  });
});

describe("computeBubblePosition — combined edge cases", () => {
  it("clamps horizontally AND flips vertically at once for a trigger pinned to the bottom-right corner", () => {
    const trigger = { left: 1000, right: 1020, top: 740, bottom: 760 };
    const bubble = { width: 260, height: 120 };
    const pos = computeBubblePosition(trigger, bubble, VIEWPORT);
    expect(pos.left + bubble.width / 2).toBeLessThanOrEqual(VIEWPORT.width - 8);
    expect(pos.above).toBe(true);
    expect(pos.top).toBeGreaterThanOrEqual(0);
    expect(pos.top + bubble.height).toBeLessThanOrEqual(VIEWPORT.height);
  });

  it("a custom margin is honoured", () => {
    const trigger = { left: 1000, right: 1020, top: 100, bottom: 120 };
    const pos = computeBubblePosition(trigger, SMALL_BUBBLE, VIEWPORT, 20);
    expect(pos.left).toBe(1024 - 20 - 100);
  });
});
