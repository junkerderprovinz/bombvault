import { describe, expect, it } from "vitest";
import { computeBubblePosition } from "./bubblePosition";

const VIEWPORT = { width: 1024, height: 768 };
const SMALL_BUBBLE = { width: 200, height: 60 };

describe("computeBubblePosition: horizontal clamp", () => {
  it("centres on the trigger when there's room on both sides", () => {
    const trigger = { left: 500, right: 540, top: 100, bottom: 120 };
    const pos = computeBubblePosition(trigger, SMALL_BUBBLE, VIEWPORT);
    expect(pos.left).toBe(520); // trigger centre, untouched
  });

  it("clamps a right-edge trigger so the bubble's right side stays inside the viewport", () => {
    // Centring alone would put left at 1010 and half the bubble past the edge.
    const trigger = { left: 1000, right: 1020, top: 100, bottom: 120 };
    const pos = computeBubblePosition(trigger, SMALL_BUBBLE, VIEWPORT);
    // The bubble is centred on `left` with translateX(-50%), so its right
    // edge is left + halfWidth.
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

describe("computeBubblePosition: vertical flip", () => {
  it("opens below the trigger when there is room", () => {
    const trigger = { left: 100, right: 140, top: 100, bottom: 120 };
    const pos = computeBubblePosition(trigger, SMALL_BUBBLE, VIEWPORT);
    expect(pos.above).toBe(false);
    expect(pos.top).toBe(120 + 8); // trigger.bottom + margin
  });

  it("flips above the trigger when a tall bubble opening below would clip the viewport's bottom edge", () => {
    // A long tip wraps into a tall bubble on a trigger near the bottom.
    const trigger = { left: 300, right: 340, top: 700, bottom: 720 };
    const tallBubble = { width: 260, height: 140 };
    const pos = computeBubblePosition(trigger, tallBubble, VIEWPORT);
    expect(pos.above).toBe(true);
    expect(pos.top).toBe(700 - 8 - 140); // trigger.top - margin - height
    expect(pos.top).toBeGreaterThanOrEqual(0);
    expect(pos.top + tallBubble.height).toBeLessThanOrEqual(VIEWPORT.height);
  });

  it("stays below when the trigger is near the top, even if the bottom clips", () => {
    // Flipping into negative space would only trade one clipped edge for
    // another.
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

describe("computeBubblePosition: combined edge cases", () => {
  it("clamps horizontally and flips vertically in the bottom-right corner", () => {
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

describe("computeBubblePosition: vertical clamping", () => {
  it("pulls a bubble back inside when it fits but would overhang the bottom", () => {
    // Fits the viewport, but neither the gap below the trigger nor the one
    // above it. Consumers are position:fixed, so an overhang could not be
    // scrolled into view.
    const trigger = { left: 300, right: 340, top: 300, bottom: 320 };
    const bubble = { width: 260, height: 600 };
    const pos = computeBubblePosition(trigger, bubble, VIEWPORT);
    expect(pos.top).toBeGreaterThanOrEqual(8);
    expect(pos.top + bubble.height).toBeLessThanOrEqual(VIEWPORT.height);
  });

  it("leaves a bubble taller than the viewport exactly where it was", () => {
    // Clipping is unavoidable here, and moving the bubble up would cover the
    // trigger as well.
    const trigger = { left: 300, right: 340, top: 4, bottom: 24 };
    const bubble = { width: 260, height: 900 };
    const pos = computeBubblePosition(trigger, bubble, VIEWPORT);
    expect(pos.top).toBe(24 + 8);
  });
});
