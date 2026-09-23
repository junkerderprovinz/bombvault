// @vitest-environment jsdom
// Imported artwork uses its viewBox very differently (Font Awesome fills it,
// Tabler pads two units on every side), so each glyph's viewBox is cropped to
// its measured ink and squared, and preserveAspectRatio scales the longer side
// to fill the box. The crop is recomputed here rather than snapshotted,
// because a wrong crop still renders, just at the wrong size.
//
// The ink boxes were measured with getBBox in a browser, not read off the
// viewBox: two source files carry a transparent bounding path, dropped on
// import, which is why IconCopy measures 20 of its 24 units.
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
  // Kept at the precision getBBox reported rather than rounded to the 2/20 it
  // nearly is, so a later measurement has something exact to compare with.
  IconCheckCircle: [2, 1.9934, 20.0078, 20.0143],
  IconTabIntegrity: [3, 1, 18, 22],
  IconTabStorage: [0, 0, 448, 512],
  // The source declares `0 0 492 492` and the ink is a 368.7 square inside it;
  // uncropped, the mark would render at three quarters of its neighbours' size.
  IconSave: [61.8, 62.4, 368.7, 368.7],
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
  // The side can be right while the origin is not, which renders the glyph at
  // the right size but off-centre.
  for (const [name] of GLYPHS) {
    const [x, y, w, h] = INK[name];
    const [bx, by, bw, bh] = croppedBox(INK[name]);
    expect(bx + bw / 2, `${name} horizontally off-centre`).toBeCloseTo(x + w / 2, 5);
    expect(by + bh / 2, `${name} vertically off-centre`).toBeCloseTo(y + h / 2, 5);
  }
});
