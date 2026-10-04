// @vitest-environment jsdom
// Imported artwork uses its viewBox very differently (Font Awesome fills it,
// Tabler pads two units on every side), so each glyph's viewBox is cropped to
// its measured ink and squared, and preserveAspectRatio scales the longer side
// to fill the box. The crop is recomputed here from the ink recorded in
// scripts/glyphs.json rather than snapshotted, because a wrong crop still
// renders, just at the wrong size.
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { render } from "@testing-library/react";
import type { ComponentType } from "react";
import * as action from "./glyphs";
import * as nav from "./navGlyphs";

type Box = [number, number, number, number];

const ASSORTMENT: { name: string; ink: Box }[] = JSON.parse(
  readFileSync(join(__dirname, "..", "..", "..", "scripts", "glyphs.json"), "utf8"),
);
const INK = new Map(ASSORTMENT.map((e) => [e.name, e.ink]));

const GLYPHS = Object.entries({ ...action, ...nav })
  .filter(([name]) => name.startsWith("Icon"))
  .map(([name, Glyph]) => [name, Glyph as ComponentType] as const);

function croppedBox([x, y, w, h]: Box): Box {
  const side = Math.max(w, h);
  return [x + w / 2 - side / 2, y + h / 2 - side / 2, side, side];
}

function svgOf(Glyph: ComponentType): SVGSVGElement {
  const { container } = render(<Glyph />);
  const svg = container.querySelector("svg");
  if (!svg) throw new Error("no <svg> rendered");
  return svg;
}

function viewBoxOf(Glyph: ComponentType): number[] {
  return (svgOf(Glyph).getAttribute("viewBox") ?? "").trim().split(/\s+/).map(Number);
}

describe("generated glyphs", () => {
  it("finds both generated modules", () => {
    expect(GLYPHS.length).toBeGreaterThan(60);
  });

  it.each(GLYPHS)("%s is a glyph of the shared assortment", (name) => {
    expect(INK.has(name), `${name} is not in scripts/glyphs.json`).toBe(true);
  });

  // glyphs.json keeps three decimals, so the crop is compared to a thousandth
  // of a unit, far below a pixel at 16px.
  it.each(GLYPHS)("crops %s to a box recomputed from its measured ink", (name, Glyph) => {
    const got = viewBoxOf(Glyph);
    expect(got, `${name} has no viewBox`).toHaveLength(4);
    const want = croppedBox(INK.get(name)!);
    got.forEach((v, i) => expect(Math.abs(v - want[i]), `${name} viewBox[${i}]`).toBeLessThan(0.002));
  });

  it.each(GLYPHS)("gives %s a square box, so its aspect ratio survives", (name, Glyph) => {
    const [, , w, h] = viewBoxOf(Glyph);
    expect(w, `${name} is not square`).toBeCloseTo(h, 5);
  });

  it.each(GLYPHS)("renders %s as a hidden, ink-coloured 16px glyph", (_name, Glyph) => {
    const svg = svgOf(Glyph);
    expect(svg.getAttribute("width")).toBe("16");
    expect(svg.getAttribute("height")).toBe("16");
    expect(svg.getAttribute("fill")).toBe("currentColor");
    expect(svg.getAttribute("aria-hidden")).toBe("true");
    expect(svg.querySelector("desc, title")).toBeNull();
  });

  it("mirrors exactly the glyphs that point along the reading direction", () => {
    const mirrored = GLYPHS.filter(([, Glyph]) => svgOf(Glyph).classList.contains("rtl:-scale-x-100"))
      .map(([name]) => name)
      .sort();
    expect(mirrored).toEqual(["IconBack", "IconForward", "IconSignIn", "IconSignOut"]);
  });

  it("never gives two meanings the same drawing", () => {
    // The Off-site tab is the off-site meaning itself, so it shares the cloud.
    const seen = new Map<string, string>();
    for (const [name, Glyph] of GLYPHS) {
      if (name === "IconTabOffsite") continue;
      const drawing = svgOf(Glyph).innerHTML;
      expect(seen.get(drawing), `${name} draws the same as ${seen.get(drawing)}`).toBeUndefined();
      seen.set(drawing, name);
    }
  });

  it("emits the same cloud for the off-site tab and the off-site control", () => {
    expect(svgOf(nav.IconTabOffsite).outerHTML).toEqual(svgOf(nav.IconCloud).outerHTML);
  });
});
