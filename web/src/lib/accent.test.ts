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

describe("softTint", () => {
  it("derives a 14%-alpha rgba from the given hex", () => {
    expect(softTint("#FCC419")).toBe("rgba(252, 196, 25, 0.14)");
  });

  it("falls back to the default accent's tint for an unparseable hex", () => {
    expect(softTint("nope")).toBe(softTint(DEFAULT_ACCENT));
  });
});
