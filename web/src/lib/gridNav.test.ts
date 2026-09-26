import { describe, expect, it } from "vitest";
import { nextGridIndex, type TileBox } from "./gridNav";

/** Tiles 50 wide in rows of `columns`, 60 apart, the way a wrapping grid lays them out. */
function grid(count: number, columns: number, top = 0): TileBox[] {
  return Array.from({ length: count }, (_, i) => ({
    top: top + Math.floor(i / columns) * 60,
    left: (i % columns) * 60,
    width: 50,
  }));
}

describe("nextGridIndex", () => {
  const boxes = grid(10, 4);

  it("steps sideways in reading order and stops at the ends", () => {
    expect(nextGridIndex("ArrowRight", 0, boxes, false)).toBe(1);
    expect(nextGridIndex("ArrowRight", 3, boxes, false)).toBe(4);
    expect(nextGridIndex("ArrowRight", 9, boxes, false)).toBe(9);
    expect(nextGridIndex("ArrowLeft", 0, boxes, false)).toBe(0);
  });

  it("follows the reading direction under rtl", () => {
    expect(nextGridIndex("ArrowLeft", 1, boxes, true)).toBe(2);
    expect(nextGridIndex("ArrowRight", 1, boxes, true)).toBe(0);
  });

  it("moves up and down by rows, to the nearest tile of a shorter row", () => {
    expect(nextGridIndex("ArrowDown", 1, boxes, false)).toBe(5);
    expect(nextGridIndex("ArrowDown", 7, boxes, false)).toBe(9);
    expect(nextGridIndex("ArrowUp", 9, boxes, false)).toBe(5);
    expect(nextGridIndex("ArrowUp", 2, boxes, false)).toBe(2);
    expect(nextGridIndex("ArrowDown", 9, boxes, false)).toBe(9);
  });

  it("crosses into the next group, which starts a row of its own", () => {
    const groups = [...grid(3, 4), ...grid(4, 4, 200)];
    expect(nextGridIndex("ArrowDown", 2, groups, false)).toBe(5);
    expect(nextGridIndex("ArrowUp", 3, groups, false)).toBe(0);
  });

  it("jumps to the ends, and starts at the first tile with nothing focused", () => {
    expect(nextGridIndex("Home", 6, boxes, false)).toBe(0);
    expect(nextGridIndex("End", 0, boxes, false)).toBe(9);
    expect(nextGridIndex("ArrowDown", -1, boxes, false)).toBe(0);
    expect(nextGridIndex("End", 0, [], false)).toBe(-1);
  });
});
