import { describe, expect, it } from "vitest";
import { textInkOn } from "./accent";
import { PROVIDER_TILES } from "./providerMarks";
import { REGISTRY_KINDS } from "./registryMarks";

function luminance(hex: string): number {
  const [r, g, b] = [1, 3, 5].map((i) => {
    const v = parseInt(hex.slice(i, i + 2), 16) / 255;
    return v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function contrast(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}

describe("textInkOn", () => {
  it("keeps white on a deep colour and the dark ink on a light one", () => {
    expect(textInkOn("#123f6d")).toBe("#FFFFFF");
    expect(textInkOn("#01cd3e")).toBe("#161616");
  });

  it("falls back to black on a mid tone where neither ink reaches 4.5:1", () => {
    expect(textInkOn("#0082c9")).toBe("#000000");
  });
});

describe("a lit brand tile", () => {
  it.each(Object.entries(PROVIDER_TILES))("%s holds its words at 4.5:1", (_, { tile, ink }) => {
    expect(contrast(tile, ink)).toBeGreaterThanOrEqual(4.5);
  });

  it.each(REGISTRY_KINDS.map((k) => [k.name, k.tile] as const))("%s holds its words at 4.5:1", (_, tile) => {
    expect(contrast(tile, textInkOn(tile))).toBeGreaterThanOrEqual(4.5);
  });
});
