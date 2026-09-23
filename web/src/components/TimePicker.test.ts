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

  it("clamps an out-of-range hour and minute", () => {
    expect(parseTime("99:99")).toEqual({ hour: 23, minute: 59 });
  });

  it("defaults to 00:00 for empty or malformed input", () => {
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

  it("supports a 1-minute step", () => {
    expect(minutesFor(1)).toHaveLength(60);
    expect(minutesFor(1)[1]).toBe(1);
  });

  it("supports a coarser 15-minute step", () => {
    expect(minutesFor(15)).toEqual([0, 15, 30, 45]);
  });

  it("falls back to 5 for a zero or negative step", () => {
    expect(minutesFor(0)).toEqual(minutesFor(5));
    expect(minutesFor(-3)).toEqual(minutesFor(5));
  });
});

describe("nearestStep", () => {
  const steps = minutesFor(5);

  it("returns a value that is already on the grid", () => {
    expect(nearestStep(steps, 30)).toBe(30);
  });

  it("rounds an off-grid value to the nearest step", () => {
    expect(nearestStep(steps, 32)).toBe(30);
    expect(nearestStep(steps, 33)).toBe(35);
  });

  it("resolves a tie to the earlier option", () => {
    // A 5-minute grid has no integer tie, so this uses steps of 10.
    expect(nearestStep([0, 10], 5)).toBe(0);
  });

  it("stays in range at the ends", () => {
    expect(nearestStep(steps, 0)).toBe(0);
    expect(nearestStep(steps, 59)).toBe(55);
  });
});
