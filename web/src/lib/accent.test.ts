import { describe, expect, it } from "vitest";
import {
  contrastOn,
  softTint,
  DEFAULT_ACCENT,
  DEFAULT_ACCENT_CONTRAST,
  DEFAULT_ACCENT_PRESETS,
  isValidAccentPresets,
} from "./accent";

describe("contrastOn", () => {
  it("puts light ink on a dark custom accent", () => {
    expect(contrastOn("#0A0A2A")).toBe("#FFFFFF");
  });

  it("puts dark ink on a light custom accent", () => {
    expect(contrastOn("#FFF7CC")).toBe("#161616");
  });

  it("gives the default accent DEFAULT_ACCENT_CONTRAST", () => {
    expect(contrastOn(DEFAULT_ACCENT)).toBe(DEFAULT_ACCENT_CONTRAST);
  });

  it("resolves pure black to light ink and pure white to dark ink", () => {
    expect(contrastOn("#000000")).toBe("#FFFFFF");
    expect(contrastOn("#FFFFFF")).toBe("#161616");
  });

  it("falls back to the default contrast for an unparseable hex", () => {
    expect(contrastOn("not-a-color")).toBe(DEFAULT_ACCENT_CONTRAST);
    expect(contrastOn("#12345")).toBe(DEFAULT_ACCENT_CONTRAST);
  });
});

// A luminance cutoff at 0.55 gives these rainbow hues white ink at 2.35 to
// 3.04:1, while #161616 reaches 5.95 to 7.70:1. The ratio is computed here
// without accent.ts's helpers, so the check cannot share a bug with them.
describe("contrastOn on mid-luminance rainbow hues", () => {
  function relLuminance(hex: string): number {
    const n = parseInt(hex.slice(1), 16);
    const channels = [(n >> 16) & 255, (n >> 8) & 255, n & 255];
    const [r, g, b] = channels.map((c) => {
      const v = c / 255;
      return v <= 0.04045 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4);
    });
    return 0.2126 * r + 0.7152 * g + 0.0722 * b;
  }
  function wcagContrastRatio(hexA: string, hexB: string): number {
    const l1 = relLuminance(hexA);
    const l2 = relLuminance(hexB);
    return (Math.max(l1, l2) + 0.05) / (Math.min(l1, l2) + 0.05);
  }

  const MID_HUES = [
    "#FF8389", // red 30
    "#FF832B", // orange 40
    "#1D99F3", // blue
    "#BE95FF", // purple 30
    "#FF7EB6", // magenta 30
  ];

  it.each(MID_HUES)("resolves %s to dark ink at >=4.5:1, not white at <4.5:1", (hex) => {
    const ink = contrastOn(hex);
    expect(ink).toBe("#161616");
    expect(wcagContrastRatio(hex, ink)).toBeGreaterThanOrEqual(4.5);
  });
});

describe("softTint", () => {
  it("derives a 14%-alpha rgba from the given hex", () => {
    expect(softTint("#FCC419")).toBe("rgba(252, 196, 25, 0.14)");
  });

  it("falls back to the default accent's tint for an unparseable hex", () => {
    expect(softTint("nope")).toBe(softTint(DEFAULT_ACCENT));
  });
});

// Storage is covered in accent.dom.test.ts.
describe("DEFAULT_ACCENT_PRESETS", () => {
  it("is a fixed set of eight valid, unique hex colours", () => {
    expect(DEFAULT_ACCENT_PRESETS).toHaveLength(8);
    for (const hex of DEFAULT_ACCENT_PRESETS) {
      expect(hex).toMatch(/^#[0-9a-fA-F]{6}$/);
    }
    expect(new Set(DEFAULT_ACCENT_PRESETS.map((h) => h.toLowerCase())).size).toBe(8);
  });

  it("includes the default accent, so one preset always matches it", () => {
    expect(DEFAULT_ACCENT_PRESETS).toContain(DEFAULT_ACCENT);
  });

  it("starts with the five GlimStone presets in their shared order", () => {
    expect(DEFAULT_ACCENT_PRESETS.slice(0, 5)).toEqual([
      "#FCC419",
      "#1D99F3",
      "#6FDC8C",
      "#FF8389",
      "#BE95FF",
    ]);
  });
});

describe("isValidAccentPresets", () => {
  it("accepts the built-in default preset set", () => {
    expect(isValidAccentPresets(DEFAULT_ACCENT_PRESETS)).toBe(true);
  });

  it("accepts any full set of 8 valid hex colours", () => {
    const custom = ["#111111", "#222222", "#333333", "#444444", "#555555", "#666666", "#777777", "#888888"];
    expect(isValidAccentPresets(custom)).toBe(true);
  });

  it("rejects the whole set when one entry is invalid", () => {
    const almostAllValid = [...DEFAULT_ACCENT_PRESETS.slice(0, 7), "javascript:alert(1)"];
    expect(isValidAccentPresets(almostAllValid)).toBe(false);
  });

  it("rejects a set with the wrong length, short or long", () => {
    expect(isValidAccentPresets(DEFAULT_ACCENT_PRESETS.slice(0, 7))).toBe(false);
    expect(isValidAccentPresets([...DEFAULT_ACCENT_PRESETS, "#000000"])).toBe(false);
    expect(isValidAccentPresets([])).toBe(false);
  });
});
