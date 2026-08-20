// GlimStone form-engine #1 — --accent-contrast must be COMPUTED from the
// chosen accent's sRGB luminance, not hard-coded. Before this fix,
// applyAccent() never touched --accent-contrast at all, so every custom
// accent silently kept whatever ink colour the CSS default happened to have
// baked in (#161616 in both themes) regardless of whether that accent was
// actually dark or light. These tests pin the real bug: a dark accent must
// resolve to light ink, and a light accent must resolve to dark ink.
import { describe, expect, it } from "vitest";
import { contrastOn, softTint, DEFAULT_ACCENT, DEFAULT_ACCENT_CONTRAST } from "./accent";

describe("contrastOn", () => {
  it("puts light ink on a dark custom accent", () => {
    expect(contrastOn("#0A0A2A")).toBe("#FFFFFF");
  });

  it("puts dark ink on a light custom accent", () => {
    expect(contrastOn("#FFF7CC")).toBe("#161616");
  });

  it("matches the existing default ink for the built-in default accent", () => {
    // Regression guard: DEFAULT_ACCENT_CONTRAST must stay the actual
    // computed value for DEFAULT_ACCENT, not just a number nobody checks.
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

// Regression coverage for the fixed-0.55-luminance-threshold bug: these are
// exactly the 5 of RAINBOW's 8 hues (lib/appearance.ts) a live contrast
// review measured getting white ink at 2.35-3.04:1 (WCAG 1.4.3 needs 4.5:1)
// under the old `> 0.55` cutoff, when #161616 gives every one of them
// 5.95-7.70:1. Each case asserts BOTH the winning ink AND the actual
// contrast ratio against it, independently recomputed here rather than by
// reusing accent.ts's own luminance/contrastRatio helpers — a fix that just
// nudges the threshold to a different-but-still-wrong constant would still
// return "#161616" (passing a hex-only assertion) while failing contrast,
// so the ratio check is the one that actually catches that regression.
describe("contrastOn — rainbow hues previously broken by the fixed 0.55 threshold", () => {
  // Standard WCAG relative luminance + contrast ratio, computed independently
  // of accent.ts's own implementation.
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

  const PREVIOUSLY_BROKEN = [
    "#FF8389", // red 30
    "#FF832B", // orange 40
    "#1D99F3", // blue
    "#BE95FF", // purple 30
    "#FF7EB6", // magenta 30
  ];

  it.each(PREVIOUSLY_BROKEN)("resolves %s to dark ink at >=4.5:1, not white at <4.5:1", (hex) => {
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
