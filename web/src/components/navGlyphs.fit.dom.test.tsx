// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// The Local / Off-site pair, and why it is sized the way it is ([242], [281]).
//
// Two rounds of jdp looking at the same switch:
//
//   [242] "der glyph ist im vergleich zu offsite glyph viel zu groß"
//   [281] "das offsite icon ist wenn es so klein ist sehr schlecht erkennbar,
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
//   1. Each half carries a fit transform recomputed here from its measured
//      ink, never snapshotted. A transform that merely EXISTS proves nothing,
//      since a wrong scale renders perfectly well.
//   2. Both land on the same width and the same centre. That is what "the pair
//      looks deliberate" reduces to in numbers, and it is the thing that broke
//      in [242].
//   3. That shared width is close to the full grid, which is what [285] was
//      about: matching the two halves to each other is not enough if the
//      result is still the smallest thing in a strip of other glyphs.
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
const LOCAL_DRIVE_INK = [2.2, 4, 9.6, 6] as const;

/** The shared frame, mirroring gen_glyphs.py's PAIR_WIDTH / PAIR_CENTRE. */
const PAIR_WIDTH = 13.8;
const PAIR_CENTRE = [7, 7] as const;

/** The grid both halves are drawn on, for the "fills its box" check. */
const GRID = 14;

function transformOf(ui: React.ReactElement): string {
  const { container } = render(ui);
  return container.querySelector("svg > g[transform]")?.getAttribute("transform") ?? "";
}

/** The transform gen_glyphs.py should have emitted for a given ink box. */
function expectFitted(t: string, ink: readonly [number, number, number, number], what: string) {
  const m = /^translate\((-?[\d.]+) (-?[\d.]+)\) scale\(([\d.]+)\)$/.exec(t);
  expect(m, `${what} lost its fit transform (got ${JSON.stringify(t)})`).not.toBeNull();
  const [sx, sy, sw, sh] = ink;
  const scale = PAIR_WIDTH / sw;
  const [, dx, dy, got] = m!;
  // 1e-5 because the generator writes six decimals, not a float literal.
  expect(Number(got)).toBeCloseTo(scale, 5);
  expect(Number(dx)).toBeCloseTo(PAIR_CENTRE[0] - (sx + sw / 2) * scale, 5);
  expect(Number(dy)).toBeCloseTo(PAIR_CENTRE[1] - (sy + sh / 2) * scale, 5);
}

it("scales the off-site cloud onto the pair's width and centre", () => {
  expectFitted(transformOf(<IconCloud />), FA_CLOUD_INK, "IconCloud");
});

it("scales the local bars by the same rule, not by redrawn coordinates", () => {
  // Both halves go through one constant. Hard-coding the enlarged rects would
  // have worked too, and would have been the second place to forget when the
  // width moves again.
  expectFitted(transformOf(<IconLocal />), LOCAL_DRIVE_INK, "IconLocal");
});

it("emits the same cloud for the tab and for the controls", () => {
  // These drifted apart once already, which is why the generator defines the
  // markup once and emits it twice.
  expect(transformOf(<IconTabOffsite />)).toBe(transformOf(<IconCloud />));
});

it("lands both halves on one width and one centre", () => {
  // The promise to the eye, expressed in the grid's own units.
  for (const ink of [FA_CLOUD_INK, LOCAL_DRIVE_INK]) {
    const scale = PAIR_WIDTH / ink[2];
    expect(ink[2] * scale).toBeCloseTo(PAIR_WIDTH, 5);
    expect(ink[3] * scale).toBeLessThan(GRID); // still inside the box
  }
  // A cloud is taller than two bars, and that difference is correct: matching
  // widths is the rule, matching heights would make the cloud the narrower one.
  const cloudH = FA_CLOUD_INK[3] * (PAIR_WIDTH / FA_CLOUD_INK[2]);
  const barsH = LOCAL_DRIVE_INK[3] * (PAIR_WIDTH / LOCAL_DRIVE_INK[2]);
  expect(cloudH).toBeGreaterThan(barsH);
});

it("fills the box, so the pair is not the smallest thing in a tab strip", () => {
  // [285]: matching the halves to each other is necessary and not sufficient.
  // At 9.6 of 14 they agreed with each other and still read small beside the
  // rail's glyphs, which fill 98-100%.
  expect(PAIR_WIDTH / GRID).toBeGreaterThan(0.95);
});
