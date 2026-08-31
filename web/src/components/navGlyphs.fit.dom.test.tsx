// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// One sizing rule for every imported glyph ([242], [267], [281], [285], [305]).
//
// Five rounds of jdp looking at icons that were all in identically-sized boxes
// and reporting, correctly every time, that some of them were bigger than
// others. The rule that came out of it:
//
//   What the eye compares is a glyph's INK, not its box. Imported artwork
//   varies wildly in how much of its own viewBox it uses — Font Awesome fills
//   its box edge to edge, Tabler pads by two units on all four sides — so a
//   set assembled from several sources arrives on screen at several sizes
//   unless something normalises it.
//
// The normaliser is a viewBox cropped to the measured ink and squared off.
// `preserveAspectRatio="xMidYMid meet"` (the default) then scales each drawing
// until its longer side fills the box, leaving the aspect ratio alone. Square
// rather than tight on both axes for exactly that reason: a tight rectangle
// would stretch a wide glyph to the height of a tall one.
//
// What is pinned here, and why each part:
//
//   1. The cropped box is RECOMPUTED from the measured ink, not snapshotted.
//      A viewBox that merely differs from the source's proves nothing — a
//      wrong crop renders perfectly well, just at the wrong size.
//   2. Every one of them is square. This is the whole aspect-ratio guarantee,
//      and it is one multiplication away from being silently wrong.
//   3. The off-site tab and the off-site control carry the identical box.
//      Those two drifted apart once already.
//
// The ink boxes below are MEASUREMENTS, taken with getBBox on the real markup
// in a browser. They are deliberately not derived from any viewBox: a path's
// drawn extent and its viewBox have no necessary relationship, and two of the
// source files carry a fully transparent bounding path that makes the viewBox
// actively misleading — dropped on import, which is why `copy` measures 20 of
// its 24 units rather than the full 24.
// ---------------------------------------------------------------------------
import { expect, it } from "vitest";
import { render } from "@testing-library/react";
import {
  IconCheckCircle,
  IconCloud,
  IconCopy,
  IconLocal,
  IconTabIntegrity,
  IconTabOffsite,
  IconTabStorage,
} from "./navGlyphs";
import { IconSave } from "./glyphs";

/** Measured ink: [x, y, width, height], in each source's own units. */
const INK: Record<string, readonly [number, number, number, number]> = {
  IconCloud: [0, 32, 640, 448],
  IconLocal: [0, 32, 512, 448],
  IconCopy: [2, 2, 20, 20],
  IconCheckCircle: [2, 2, 20, 20],
  IconTabIntegrity: [3, 1, 18, 22],
  IconTabStorage: [0, 0, 448, 512],
  IconSave: [0, 32, 448, 448],
};

/** Mirrors gen_glyphs.py's `cropped_box`. */
function croppedBox([x, y, w, h]: readonly [number, number, number, number]) {
  const side = Math.max(w, h);
  return [x + w / 2 - side / 2, y + h / 2 - side / 2, side, side];
}

function viewBoxOf(ui: React.ReactElement): number[] {
  const { container } = render(ui);
  const raw = container.querySelector("svg")?.getAttribute("viewBox") ?? "";
  return raw.trim().split(/\s+/).map(Number);
}

const GLYPHS: [string, React.ReactElement][] = [
  ["IconCloud", <IconCloud />],
  ["IconLocal", <IconLocal />],
  ["IconCopy", <IconCopy />],
  ["IconCheckCircle", <IconCheckCircle />],
  ["IconTabIntegrity", <IconTabIntegrity />],
  ["IconTabStorage", <IconTabStorage />],
  ["IconSave", <IconSave />],
];

it.each(GLYPHS)("crops %s to a box recomputed from its measured ink", (name, ui) => {
  const got = viewBoxOf(ui);
  expect(got, `${name} has no viewBox`).toHaveLength(4);
  const want = croppedBox(INK[name]);
  got.forEach((v, i) => expect(v).toBeCloseTo(want[i], 5));
});

it.each(GLYPHS)("gives %s a square box, so its aspect ratio survives", (name, ui) => {
  const [, , w, h] = viewBoxOf(ui);
  expect(w, `${name} is not square`).toBeCloseTo(h, 5);
});

it("emits the same cloud for the off-site tab and the off-site control", () => {
  expect(viewBoxOf(<IconTabOffsite />)).toEqual(viewBoxOf(<IconCloud />));
});

it("centres each crop on the ink, so nothing sits off to one side", () => {
  // The half of the crop that is easy to get wrong: the SIDE can be right
  // while the origin is not, which renders the glyph correctly sized and
  // visibly off-centre. Checked as "ink centre equals box centre".
  for (const [name] of GLYPHS) {
    const [x, y, w, h] = INK[name];
    const [bx, by, bw, bh] = croppedBox(INK[name]);
    expect(bx + bw / 2, `${name} horizontally off-centre`).toBeCloseTo(x + w / 2, 5);
    expect(by + bh / 2, `${name} vertically off-centre`).toBeCloseTo(y + h / 2, 5);
  }
});
