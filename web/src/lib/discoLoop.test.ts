import { describe, expect, it } from "vitest";
import { RAINBOW } from "./appearance";
import { buildLoop, colourAt, oklch } from "./discoLoop";

function oklab(hex: string): number[] {
  const { l, c, h } = oklch(hex);
  return [l, c * Math.cos((h * Math.PI) / 180), c * Math.sin((h * Math.PI) / 180)];
}

function distance(x: string, y: string): number {
  const a = oklab(x);
  const b = oklab(y);
  return Math.hypot(a[0] - b[0], a[1] - b[1], a[2] - b[2]);
}

describe("the disco loop", () => {
  const loop = buildLoop(RAINBOW);
  const length = loop.at[RAINBOW.length];

  it("passes through every palette colour, in order", () => {
    RAINBOW.forEach((colour, k) => {
      expect(colourAt(loop, loop.at[k])).toBe(colour);
    });
  });

  it("closes on the first colour and wraps either way", () => {
    expect(colourAt(loop, length)).toBe(RAINBOW[0]);
    expect(colourAt(loop, length * 2 + 0.1)).toBe(colourAt(loop, 0.1));
    expect(colourAt(loop, -0.1)).toBe(colourAt(loop, length - 0.1));
  });

  // The palette's neighbours sit unevenly on the wheel. The same time for
  // every gap would run the glide three times as fast across the widest as
  // across the narrowest, and change its pace at every colour. A sample that
  // straddles a palette colour cuts its corner and measures short, hence the
  // slack.
  it("covers equal distances in equal times", () => {
    const samples = 48;
    const steps: number[] = [];
    for (let i = 0; i < samples; i++) {
      const here = colourAt(loop, (length * i) / samples);
      const next = colourAt(loop, (length * (i + 1)) / samples);
      steps.push(distance(here, next));
    }
    expect(Math.max(...steps) / Math.min(...steps)).toBeLessThan(1.5);
  });

  it("turns round the wheel instead of cutting through grey", () => {
    const sunflower = oklch(RAINBOW[2]);
    const green = oklch(RAINBOW[3]);
    const halfway = oklch(colourAt(loop, (loop.at[2] + loop.at[3]) / 2));
    expect(halfway.c).toBeGreaterThan(Math.min(sunflower.c, green.c) * 0.9);
  });

  it("keeps the other colour's hue on a glide to or from grey", () => {
    const palette = ["#FFFFFF", "#FF0000", ...RAINBOW.slice(2)];
    const greyLoop = buildLoop(palette);
    const halfway = oklch(colourAt(greyLoop, greyLoop.at[1] / 2));
    expect(Math.abs(halfway.h - oklch("#FF0000").h)).toBeLessThan(2);
  });

  it("stands still for a palette of one colour", () => {
    const flat = buildLoop(Array(8).fill("#3DDBD9"));
    expect(colourAt(flat, 0)).toBe("#3DDBD9");
    expect(colourAt(flat, 5)).toBe("#3DDBD9");
  });
});
