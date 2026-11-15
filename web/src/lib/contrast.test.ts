import { describe, expect, it } from "vitest";
import { DEFAULT_ACCENT_PRESETS } from "./accent";
import { MIN_TEXT_CONTRAST, accentHovers, accentInks, contrastOn, contrastRatio } from "./contrast";
import { darkenTo, INK_LIGHTNESS, lightenTo, srgbOf } from "./oklab";

// The colours a picker or a device can hand the engine that the presets never
// come near: the two ends, the grey in the middle, and two saturated hues.
const HARD = {
  nearWhite: "#FAFAFA",
  nearBlack: "#0A0A0A",
  midGrey: "#777777",
  blue: "#0000FF",
  yellow: "#FFFF00",
};

// --accent-soft over a ground: 14% of the accent.
function wash(ground: string, accent: string): string {
  const channels = (h: string) => [1, 3, 5].map((i) => parseInt(h.slice(i, i + 2), 16));
  const a = channels(accent);
  return "#" + channels(ground).map((v, i) => Math.round(v + (a[i] - v) * 0.14).toString(16).padStart(2, "0")).join("");
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
    const light = inks.light ?? darkenTo(hex, INK_LIGHTNESS.light);
    const dark = inks.dark ?? lightenTo(hex, INK_LIGHTNESS.dark);
    expect(contrastRatio(light, "#e8e8e8")).toBeGreaterThanOrEqual(MIN_TEXT_CONTRAST);
    expect(contrastRatio(light, "#ffffff")).toBeGreaterThanOrEqual(MIN_TEXT_CONTRAST);
    expect(contrastRatio(dark, "#262626")).toBeGreaterThanOrEqual(MIN_TEXT_CONTRAST);
    // OLED's card.
    expect(contrastRatio(dark, "#161616")).toBeGreaterThanOrEqual(MIN_TEXT_CONTRAST);
  });

  it.each(Object.entries(HARD))("reads at 4.5:1 on its own wash on both themes for %s", (_name, hex) => {
    const inks = accentInks(hex);
    expect(contrastRatio(inks.light ?? darkenTo(hex, INK_LIGHTNESS.light), wash("#ffffff", hex))).toBeGreaterThanOrEqual(MIN_TEXT_CONTRAST);
    expect(contrastRatio(inks.dark ?? lightenTo(hex, INK_LIGHTNESS.dark), wash("#262626", hex))).toBeGreaterThanOrEqual(MIN_TEXT_CONTRAST);
  });

  it("needs nothing set for any colour across the picker", () => {
    // Twelve hues at four chromas on four lightness steps.
    const sweep = [0.45, 0.6, 0.75, 0.9].flatMap((l) =>
      [0.02, 0.08, 0.2, 0.3].flatMap((c) =>
        Array.from({ length: 12 }, (_, i) => srgbOf([l, c * Math.cos((i * Math.PI) / 6), c * Math.sin((i * Math.PI) / 6)])),
      ),
    );
    expect(sweep.filter((hex) => accentInks(hex).light || accentInks(hex).dark)).toEqual([]);
  });

  it("sets nothing for a value that is not a colour", () => {
    expect(accentInks("system")).toEqual({ light: undefined, dark: undefined });
  });
});

// The hover as index.css mixes it: 85% of the fill, the rest the text colour.
function hoverMix(hex: string, text: string): string {
  const parts = (h: string) => {
    const n = parseInt(h.slice(1), 16);
    return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
  };
  const f = parts(hex);
  const t = parts(text);
  return "#" + f.map((v, i) => Math.round(v + (t[i] - v) * 0.15).toString(16).padStart(2, "0")).join("");
}

describe("the hover of an accent fill", () => {
  it("needs nothing set for any preset", () => {
    for (const hex of DEFAULT_ACCENT_PRESETS) expect(accentHovers(hex)).toEqual({ light: undefined, dark: undefined });
  });

  it.each(Object.entries(HARD))("keeps the fill's ink at 4.5:1 under the pointer on both themes for %s", (_name, hex) => {
    const ink = contrastOn(hex);
    const hovers = accentHovers(hex);
    expect(contrastRatio(ink, hovers.light ?? hoverMix(hex, "#161616"))).toBeGreaterThanOrEqual(MIN_TEXT_CONTRAST);
    expect(contrastRatio(ink, hovers.dark ?? hoverMix(hex, "#f4f4f4"))).toBeGreaterThanOrEqual(MIN_TEXT_CONTRAST);
  });

  it("steps a mid-grey away from its dark ink on the light theme instead of toward it", () => {
    const light = accentHovers(HARD.midGrey).light;
    expect(light).toBeDefined();
    expect(contrastRatio(light!, "#ffffff")).toBeLessThan(contrastRatio(HARD.midGrey, "#ffffff"));
  });

  it("sets nothing for a value that is not a colour", () => {
    expect(accentHovers("system")).toEqual({ light: undefined, dark: undefined });
  });
});
