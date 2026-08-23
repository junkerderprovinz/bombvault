// ---------------------------------------------------------------------------
// TimePicker — pure placement/parsing math, exercised directly with plain
// numbers (no DOM, no React renderer), matching this repo's established
// no-jsdom pattern for pure logic (Selector.test.ts, bubblePosition.test.ts).
// Real DOM/keyboard/popover behaviour lives in TimePicker.dom.test.tsx.
// ---------------------------------------------------------------------------
import { describe, expect, it } from "vitest";
import { formatTime, minutesFor, nearestStep, parseTime } from "./TimePicker";

describe("parseTime", () => {
  it("parses a well-formed HH:MM value", () => {
    expect(parseTime("14:30")).toEqual({ hour: 14, minute: 30 });
    expect(parseTime("02:00")).toEqual({ hour: 2, minute: 0 });
    expect(parseTime("23:59")).toEqual({ hour: 23, minute: 59 });
  });

  it("accepts a single-digit hour", () => {
    expect(parseTime("4:05")).toEqual({ hour: 4, minute: 5 });
  });

  it("clamps an out-of-range hour/minute instead of producing an invalid time", () => {
    expect(parseTime("99:99")).toEqual({ hour: 23, minute: 59 });
  });

  it("defaults to 00:00 for empty or malformed input, never throws", () => {
    expect(parseTime("")).toEqual({ hour: 0, minute: 0 });
    expect(parseTime("not-a-time")).toEqual({ hour: 0, minute: 0 });
    expect(parseTime("  ")).toEqual({ hour: 0, minute: 0 });
  });
});

describe("formatTime", () => {
  it("zero-pads both fields", () => {
    expect(formatTime(2, 0)).toBe("02:00");
    expect(formatTime(9, 5)).toBe("09:05");
  });

  it("round-trips through parseTime", () => {
    const { hour, minute } = parseTime("17:45");
    expect(formatTime(hour, minute)).toBe("17:45");
  });
});

describe("minutesFor", () => {
  it("defaults to 5-minute steps", () => {
    expect(minutesFor(5)).toEqual([0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55]);
  });

  it("supports a 1-minute step (every minute)", () => {
    expect(minutesFor(1)).toHaveLength(60);
    expect(minutesFor(1)[1]).toBe(1);
  });

  it("supports a coarser 15-minute step", () => {
    expect(minutesFor(15)).toEqual([0, 15, 30, 45]);
  });

  it("clamps a zero/negative/absurd step to a sane 1-59 range, falling back to 5", () => {
    expect(minutesFor(0)).toEqual(minutesFor(5));
    expect(minutesFor(-3)).toEqual(minutesFor(5));
  });
});

describe("nearestStep", () => {
  const steps = minutesFor(5);

  it("returns the exact value when it's already a valid step", () => {
    expect(nearestStep(steps, 30)).toBe(30);
  });

  it("rounds to the nearest available step for an off-grid value", () => {
    expect(nearestStep(steps, 32)).toBe(30);
    expect(nearestStep(steps, 33)).toBe(35);
  });

  it("resolves an exact tie to the smaller/earlier option", () => {
    // 27.5 is impossible with integers, but 2/3 between two steps close
    // enough to be meaningfully tested: distance to 25 and 30 from 27 is 2
    // and 3 — not a tie. Use a coarse 10-step table for a real tie at 5.
    expect(nearestStep([0, 10], 5)).toBe(0);
  });

  it("never goes out of range at the ends", () => {
    expect(nearestStep(steps, 0)).toBe(0);
    expect(nearestStep(steps, 59)).toBe(55);
  });
});
