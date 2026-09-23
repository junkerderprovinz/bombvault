// The hex and HSV conversions behind ColorPickerSwatch. The drag handlers and
// the hex field both go through them, so a lossy round trip would move the
// picker's dots away from the colour the caller receives.
import { describe, expect, it } from "vitest";
import { hexToHsv, hsvToHex, normalizeHex } from "./ColorPickerPopover";

describe("hexToHsv / hsvToHex round trip", () => {
  it.each([
    "#fcc419", // default accent (yellow)
    "#1d99f3", // blue
    "#6fdc8c", // green
    "#ff8389", // red
    "#be95ff", // purple
    "#000000",
    "#ffffff",
    "#808080",
])("round-trips %s through hexToHsv -> hsvToHex unchanged", (hex) => {
    const hsv = hexToHsv(hex);
    expect(hsv).not.toBeNull();
    expect(hsvToHex(hsv!.h, hsv!.s, hsv!.v)).toBe(hex);
  });

  it("accepts an uppercase or bare (no #) hex the same way", () => {
    const withHash = hexToHsv("#2F6FEB");
    const bare = hexToHsv("2f6feb");
    expect(withHash).toEqual(bare);
  });

  it("returns null for an unparseable value", () => {
    expect(hexToHsv("not-a-color")).toBeNull();
    expect(hexToHsv("#12345")).toBeNull();
    expect(hexToHsv("")).toBeNull();
  });
});

describe("hsvToHex", () => {
  it("produces pure red/green/blue at their canonical hues", () => {
    expect(hsvToHex(0, 1, 1)).toBe("#ff0000");
    expect(hsvToHex(120, 1, 1)).toBe("#00ff00");
    expect(hsvToHex(240, 1, 1)).toBe("#0000ff");
  });

  it("zero saturation is a neutral grey regardless of hue", () => {
    expect(hsvToHex(200, 0, 0.5)).toBe(hsvToHex(10, 0, 0.5));
  });

  it("zero value is always black regardless of hue/saturation", () => {
    expect(hsvToHex(90, 1, 0)).toBe("#000000");
  });
});

describe("normalizeHex", () => {
  it("adds a leading # and lowercases", () => {
    expect(normalizeHex("2F6FEB")).toBe("#2f6feb");
  });

  it("accepts an already-prefixed value unchanged apart from case", () => {
    expect(normalizeHex("#2F6FEB")).toBe("#2f6feb");
  });

  it("trims surrounding whitespace", () => {
    expect(normalizeHex("  2f6feb  ")).toBe("#2f6feb");
  });

  it("rejects anything that isn't exactly 6 hex digits", () => {
    expect(normalizeHex("2f6fe")).toBeNull();
    expect(normalizeHex("2f6febb")).toBeNull();
    expect(normalizeHex("zzzzzz")).toBeNull();
    expect(normalizeHex("")).toBeNull();
  });
});
