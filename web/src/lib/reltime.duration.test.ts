import { describe, expect, it } from "vitest";
import { formatDuration } from "./reltime";

describe("formatDuration", () => {
  it("counts seconds, minutes and hours below a day", () => {
    expect(formatDuration(12)).toBe("12s");
    expect(formatDuration(185)).toBe("3m 5s");
    expect(formatDuration(86_399)).toBe("23h 59m");
  });

  it("counts days and hours from a day on", () => {
    expect(formatDuration(86_400)).toBe("1d 0h");
    // A yearly schedule eleven days after its date.
    expect(formatDuration(30_714_120)).toBe("355d 11h");
  });

  it("is empty for a negative or non-finite span", () => {
    expect(formatDuration(-1)).toBe("");
    expect(formatDuration(Number.NaN)).toBe("");
  });
});
