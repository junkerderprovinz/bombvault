import { describe, expect, it } from "vitest";
import { DEFAULT_ACCENT_PRESETS } from "./accent";
import { MIN_TEXT_CONTRAST, accentInks, contrastOn, contrastRatio } from "./contrast";

// The colours a picker or a device can hand the engine that the presets never
// come near: the two ends, the grey in the middle, and two saturated hues.
const HARD = {
  nearWhite: "#FAFAFA",
  nearBlack: "#0A0A0A",
  midGrey: "#777777",
  blue: "#0000FF",
  yellow: "#FFFF00",
};

// The light ink as index.css mixes it: 45% of the accent, the rest black.
function lightMix(hex: string): string {
  const n = parseInt(hex.slice(1), 16);
  const c = [(n >> 16) & 255, (n >> 8) & 255, n & 255].map((v) => Math.round(v * 0.45));
  return "#" + c.map((v) => v.toString(16).padStart(2, "0")).join("");
}

describe("the ink on a fill", () => {
  it.each(Object.entries(HARD))("reaches 4.5:1 on %s", (_name, hex) => {
    expect(contrastRatio(hex, contrastOn(hex))).toBeGreaterThanOrEqual(MIN_TEXT_CONTRAST);
  });

  it("goes past the near-black toward black on a mid-tone neither ink reads on", () => {
    const ink = contrastOn(HARD.midGrey);
    expect(ink).not.toBe("#161616");
    expect(ink).not.toBe("#FFFFFF");
    expect(contrastRatio(HARD.midGrey, ink)).toBeGreaterThanOrEqual(MIN_TEXT_CONTRAST);
  });

  it("keeps the near-black on every preset", () => {
    for (const hex of DEFAULT_ACCENT_PRESETS) expect(contrastOn(hex)).toBe("#161616");
  });
});

describe("accent text", () => {
  it("needs nothing set for any preset", () => {
    for (const hex of DEFAULT_ACCENT_PRESETS) expect(accentInks(hex)).toEqual({ light: undefined, dark: undefined });
  });

  it.each(Object.entries(HARD))("reaches 4.5:1 on both themes for %s", (_name, hex) => {
    const inks = accentInks(hex);
    const light = inks.light ?? lightMix(hex);
    const dark = inks.dark ?? hex;
    expect(contrastRatio(light, "#e8e8e8")).toBeGreaterThanOrEqual(MIN_TEXT_CONTRAST);
    expect(contrastRatio(light, "#ffffff")).toBeGreaterThanOrEqual(MIN_TEXT_CONTRAST);
    expect(contrastRatio(dark, "#262626")).toBeGreaterThanOrEqual(MIN_TEXT_CONTRAST);
    // OLED's card.
    expect(contrastRatio(dark, "#161616")).toBeGreaterThanOrEqual(MIN_TEXT_CONTRAST);
  });

  it("moves an accent only as far as it has to", () => {
    const dark = accentInks(HARD.blue).dark!;
    expect(contrastRatio(dark, "#262626")).toBeLessThan(MIN_TEXT_CONTRAST + 0.2);
  });

  it("sets nothing for a value that is not a colour", () => {
    expect(accentInks("system")).toEqual({ light: undefined, dark: undefined });
  });
});
