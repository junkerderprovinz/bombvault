// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// The Local / Off-site pair, and why it is sized the way it is ([242], [267]).
//
// Two rounds of jdp looking at the same switch:
//
//   [242] "der glyph ist im vergleich zu offsite glyph viel zu groß"
//   [267] "das offsite icon ist wenn es so klein ist sehr schlecht erkennbar,
//          es muss einfacher sein"
//
// The first was about ink AREA: two glyphs in identical 20px boxes are not the
// same size if one is drawn to the edge and the other has air around it. The
// second was about FEATURE size, which no amount of scaling fixes. One unit of
// the 14-unit grid is 1.43px at 20px, so detail thinner than roughly 1.5 units
// merges into its neighbour: the old cloud's humps rose 1.3 units and read as
// a dome, and Streamline's hard-disk had an interior arm well under a unit.
//
// Hence today's pair: Font Awesome's cloud (one closed path, deep valleys) and
// two plain bars for local storage. What this file pins is the sizing contract
// between them, because that is the part a regeneration can silently break:
//
//   1. The cloud carries a fit transform, recomputed here from its measured
//      ink rather than snapshotted. A transform that merely EXISTS proves
//      nothing, since a wrong scale renders perfectly well.
//   2. The bars are drawn at the pair's width already, so they must NOT carry
//      one. A transform appearing there would mean someone fitted a glyph
//      twice.
//   3. Both are centred on the same point and span the same width. That is
//      what "the pair looks deliberate" reduces to in numbers, and it is the
//      thing that broke in [242].
//
// jsdom has no getBBox, so the ink boxes below are measurements taken in a
// real browser and the assertions work on attributes. That split is fine: the
// arithmetic is what regresses, and the arithmetic is testable here.
// ---------------------------------------------------------------------------
import { expect, it } from "vitest";
import { render } from "@testing-library/react";
import { IconCloud, IconLocal, IconTabOffsite } from "./navGlyphs";

/** Measured, not derived from any viewBox: [x, y, width, height]. */
const FA_CLOUD_INK = [0, 32, 640, 448] as const;

/** The shared frame, mirroring gen_glyphs.py's PAIR_WIDTH / PAIR_CENTRE. */
const PAIR_WIDTH = 9.6;
const PAIR_CENTRE = [7, 7] as const;

function transformOf(ui: React.ReactElement): string {
  const { container } = render(ui);
  return container.querySelector("svg > g[transform]")?.getAttribute("transform") ?? "";
}

it("scales the off-site cloud onto the pair's width and centre", () => {
  const t = transformOf(<IconCloud />);
  const m = /^translate\((-?[\d.]+) (-?[\d.]+)\) scale\(([\d.]+)\)$/.exec(t);
  expect(m, `IconCloud lost its fit transform (got ${JSON.stringify(t)})`).not.toBeNull();

  const [sx, sy, sw, sh] = FA_CLOUD_INK;
  const scale = PAIR_WIDTH / sw;
  const [, dx, dy, got] = m!;
  // 1e-5 because the generator writes six decimals, not a float literal.
  expect(Number(got)).toBeCloseTo(scale, 5);
  expect(Number(dx)).toBeCloseTo(PAIR_CENTRE[0] - (sx + sw / 2) * scale, 5);
  expect(Number(dy)).toBeCloseTo(PAIR_CENTRE[1] - (sy + sh / 2) * scale, 5);
});

it("emits the same cloud for the tab and for the controls", () => {
  // These drifted apart once already, which is why the generator defines the
  // markup once and emits it twice.
  expect(transformOf(<IconTabOffsite />)).toBe(transformOf(<IconCloud />));
});

it("leaves the local bars untransformed, since they are drawn to size", () => {
  expect(transformOf(<IconLocal />)).toBe("");
});

it("gives both halves the same width and centre", () => {
  const { container } = render(<IconLocal />);
  const rects = [...container.querySelectorAll("rect")];
  expect(rects.length).toBe(2);

  const num = (el: Element, a: string) => Number(el.getAttribute(a));
  const left = Math.min(...rects.map((r) => num(r, "x")));
  const right = Math.max(...rects.map((r) => num(r, "x") + num(r, "width")));
  const top = Math.min(...rects.map((r) => num(r, "y")));
  const bottom = Math.max(...rects.map((r) => num(r, "y") + num(r, "height")));

  expect(right - left).toBeCloseTo(PAIR_WIDTH, 5);
  expect((left + right) / 2).toBeCloseTo(PAIR_CENTRE[0], 5);
  expect((top + bottom) / 2).toBeCloseTo(PAIR_CENTRE[1], 5);

  // And the cloud lands on that same centre, by the transform checked above.
  const drawnHeight = FA_CLOUD_INK[3] * (PAIR_WIDTH / FA_CLOUD_INK[2]);
  expect(drawnHeight).toBeGreaterThan(bottom - top); // a cloud is taller than two bars
  expect(drawnHeight).toBeLessThan(14); // and still inside the grid
});
