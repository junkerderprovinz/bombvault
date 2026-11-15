import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { DEFAULT_ACCENT_PRESETS } from "./accent";
import { RAINBOW } from "./appearance";
import { contrastRatio } from "./contrast";
import { darkenTo, INK_LIGHTNESS, lightenTo, oklabOf, srgbOf } from "./oklab";

const CSS = readFileSync(join(dirname(fileURLToPath(import.meta.url)), "..", "index.css"), "utf8");

describe("OKLab", () => {
  it("puts white and black at the ends of the lightness axis with no colour", () => {
    const [lw, aw, bw] = oklabOf("#FFFFFF");
    expect(Math.abs(lw - 1)).toBeLessThan(1e-4);
    expect(Math.abs(aw)).toBeLessThan(1e-4);
    expect(Math.abs(bw)).toBeLessThan(1e-4);
  });

  it("gives a colour back as it went in", () => {
    for (const hex of ["#FCC419", "#1D99F3", "#161616", "#E8E8E8", "#FF0000"]) expect(srgbOf(oklabOf(hex))).toBe(hex);
  });

  it("clips a colour outside sRGB per channel, as the browser paints it", () => {
    expect(srgbOf([0.7, 0.3, 0])).toMatch(/^#FF[0-9A-F]{4}$/);
  });

  it("darkens onto the step and keeps the hue", () => {
    for (const hex of DEFAULT_ACCENT_PRESETS) {
      const [, a0, b0] = oklabOf(hex);
      const [l, a, b] = oklabOf(darkenTo(hex, INK_LIGHTNESS.light));
      expect(Math.abs(l - INK_LIGHTNESS.light), hex).toBeLessThan(0.005);
      expect(Math.abs(Math.atan2(b, a) - Math.atan2(b0, a0)), hex).toBeLessThan(0.02);
    }
  });

  it("leaves an accent darker than the light step as its own ink", () => {
    expect(darkenTo("#0000FF", INK_LIGHTNESS.light)).toBe("#0000FF");
  });

  it("lifts Blue onto the dark step and leaves a bright accent alone", () => {
    const [l] = oklabOf(lightenTo("#1D99F3", INK_LIGHTNESS.dark));
    expect(Math.abs(l - INK_LIGHTNESS.dark)).toBeLessThan(0.005);
    expect(lightenTo("#FCC419", INK_LIGHTNESS.dark)).toBe("#FCC419");
  });

  it("brings every preset and rainbow ink within a point of the same contrast on white", () => {
    const ratios = [...DEFAULT_ACCENT_PRESETS, ...RAINBOW].map((h) => contrastRatio(darkenTo(h, INK_LIGHTNESS.light), "#FFFFFF"));
    expect(Math.max(...ratios) - Math.min(...ratios), ratios.map((r) => r.toFixed(2)).join(" ")).toBeLessThan(1);
  });

  it("uses the steps the stylesheet declares", () => {
    const values = (name: string) =>
      [...new Set([...CSS.matchAll(new RegExp(`${name}:\\s*([\\d.]+);`, "g"))].map((m) => Number(m[1])))];
    expect(values("--ink-l-light")).toEqual([INK_LIGHTNESS.light]);
    expect(values("--ink-l-dark")).toEqual([INK_LIGHTNESS.dark]);
  });
});
