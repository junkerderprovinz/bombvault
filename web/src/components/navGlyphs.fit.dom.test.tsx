// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// Optical sizing of the Local/Off-site pair ([242]).
//
// jdp: "der glyph ist im vergleich zu offsite glyph viel zu gross, können wir
// nicht ein schönen festplatten bzw HDD glyph verwenden?"
//
// He was right about the symptom and it was NOT the box: both glyphs render
// into the same 20px square. It was the ink. The cloud is hand-drawn with air
// around it and covers 10.4 x 8.2 of the 14-unit grid; Streamline draws to the
// edge, so hard-disk covers 12 x 14 — near enough double the ink in an
// identical box. So IconLocal is scaled into the cloud's envelope by
// gen_glyphs.py.
//
// Two things are worth pinning, and neither is "the file renders":
//
//   1. The transform SURVIVES a regeneration. gen_glyphs.py rewrites this file
//      wholesale, and a swapped source file or a dropped FIT_TO_CLOUD entry
//      would put the old, too-big drive back with nothing failing.
//   2. The arithmetic is the one that was intended. A transform that merely
//      EXISTS proves nothing — a wrong scale still renders. So the numbers are
//      recomputed here from the two measured ink boxes and compared against
//      what the generator emitted, which is what makes this a test rather than
//      a snapshot of whatever happened to be produced.
//
// The ink boxes below are measurements, taken with getBBox in a browser on the
// real markup. They are deliberately NOT derived from the viewBox: a path's
// drawn extent has no necessary relationship to it, and assuming otherwise is
// precisely the bug this fixes.
// ---------------------------------------------------------------------------
import { expect, it } from "vitest";
import { render } from "@testing-library/react";
import { IconCloud, IconLocal } from "./navGlyphs";

/** Measured ink, not viewBox: [x, y, width, height] on the 14-unit grid. */
const CLOUD_INK = [2.2, 2.7, 10.4, 8.2] as const;
const HARD_DISK_INK = [1.0, 0.0, 12.0, 14.0] as const;

function expectedFit() {
  const [sx, sy, sw, sh] = HARD_DISK_INK;
  const [tx, ty, tw, th] = CLOUD_INK;
  const scale = Math.min(tw / sw, th / sh);
  return {
    scale,
    dx: tx + tw / 2 - (sx + sw / 2) * scale,
    dy: ty + th / 2 - (sy + sh / 2) * scale,
  };
}

function transformOf(ui: React.ReactElement): string {
  const { container } = render(ui);
  const g = container.querySelector("svg > g[transform]");
  return g?.getAttribute("transform") ?? "";
}

it("scales the Local drive into the cloud's ink envelope", () => {
  const t = transformOf(<IconLocal />);
  const m = /^translate\((-?[\d.]+) (-?[\d.]+)\) scale\(([\d.]+)\)$/.exec(t);
  expect(m, `IconLocal lost its fit transform (got ${JSON.stringify(t)})`).not.toBeNull();

  const want = expectedFit();
  const [, dx, dy, scale] = m!;
  // 1e-5 because the generator writes six decimals, not a float literal.
  expect(Number(scale)).toBeCloseTo(want.scale, 5);
  expect(Number(dx)).toBeCloseTo(want.dx, 5);
  expect(Number(dy)).toBeCloseTo(want.dy, 5);
});

it("leaves the cloud itself unscaled, since it defines the envelope", () => {
  // Fitting the reference to itself would be a no-op transform at best and a
  // slow drift at worst, each regeneration nudging the pair a little smaller.
  expect(transformOf(<IconCloud />)).toBe("");
});

it("gives the pair the same drawn height", () => {
  // The actual promise to the eye. A row of icons is read off its height, so
  // matching heights is what "same size" means here — the fitted drive comes
  // out NARROWER than the cloud, and that is correct, not a rounding error.
  const { scale } = expectedFit();
  const drawnHeight = HARD_DISK_INK[3] * scale;
  expect(drawnHeight).toBeCloseTo(CLOUD_INK[3], 5);
  expect(HARD_DISK_INK[2] * scale).toBeLessThan(CLOUD_INK[2]);
});
