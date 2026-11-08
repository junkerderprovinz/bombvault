import { expect, test } from "vitest";
import { dropIndex, type DropAxis, type DropRect } from "./dropIndex";

// Boxes of these sizes side by side from 0, along a row or down a column.
function lay(axis: 'x' | 'y', sizes: readonly number[]): DropRect[] {
  let at = 0;
  return sizes.map((size) => {
    const box = axis === 'x' ? { left: at, top: 0, width: size, height: 40 } : { left: 0, top: at, width: 200, height: size };
    at += size;
    return box;
  });
}

// Where item `i` of `boxes` lands when it is carried to `at` along the axis.
function carry(boxes: readonly DropRect[], i: number, at: number, axis: DropAxis = 'x', rtl = false): number {
  const from = boxes[i];
  const held = axis === 'y' ? { ...from, top: at } : { ...from, left: at };
  return dropIndex(held, from, boxes.filter((_, j) => j !== i), { axis, rtl });
}

// The sizes in the order a carried item at `index` leaves them.
function moved(sizes: readonly number[], i: number, index: number): number[] {
  const rest = sizes.filter((_, j) => j !== i);
  rest.splice(index, 0, sizes[i]);
  return rest;
}

test('an item of equal size passes its neighbour once it covers half of it, at the same point both ways', () => {
  const row = lay('x', [100, 100, 100]);
  expect(carry(row, 0, 50)).toBe(0);
  expect(carry(row, 0, 51)).toBe(1);
  expect(carry(row, 0, 151)).toBe(2);
  expect(carry(row, 2, 151)).toBe(2);
  expect(carry(row, 2, 149)).toBe(1);
  expect(carry(row, 2, 49)).toBe(0);
});

test('a wide column reaches the first and the last place from the middle and back', () => {
  const sizes = [80, 100, 300, 80];
  expect(carry(lay('x', sizes), 2, 0)).toBe(0);
  expect(carry(lay('x', sizes), 2, 260)).toBe(3);
  const first = moved(sizes, 2, 0);
  expect(carry(lay('x', first), 0, 260)).toBe(3);
  const last = moved(sizes, 2, 3);
  expect(carry(lay('x', last), 3, 0)).toBe(0);
});

test('a tall row reaches the top and the bottom of a column', () => {
  const sizes = [40, 40, 200, 40];
  expect(carry(lay('y', sizes), 2, 0, 'y')).toBe(0);
  expect(carry(lay('y', sizes), 2, 120, 'y')).toBe(3);
  expect(carry(lay('y', moved(sizes, 2, 0)), 0, 120, 'y')).toBe(3);
  expect(carry(lay('y', moved(sizes, 2, 3)), 3, 0, 'y')).toBe(0);
});

test('a narrow item passes a wide neighbour only beyond its middle, either way', () => {
  const before = lay('x', [80, 300, 80]);
  expect(carry(before, 0, 149)).toBe(0);
  expect(carry(before, 0, 151)).toBe(1);
  const after = lay('x', [300, 80, 80]);
  expect(carry(after, 1, 151)).toBe(1);
  expect(carry(after, 1, 149)).toBe(0);
});

test('an item stays put until its leading edge reaches the middle of a neighbour', () => {
  // The narrow item sits at 100..180 between a 100 and a 300 wide column.
  const row = lay('x', [100, 80, 300]);
  for (let x = 51; x < 250; x++) expect(carry(row, 1, x), `held at ${x}`).toBe(1);
  expect(carry(row, 1, 49)).toBe(0);
  expect(carry(row, 1, 251)).toBe(2);
});

test('the order a position asks for asks for itself once the others have moved', () => {
  const sizes = [60, 300, 100, 80, 160];
  for (let i = 0; i < sizes.length; i++) {
    for (let x = -50; x <= 750; x += 5) {
      const index = carry(lay('x', sizes), i, x);
      expect(carry(lay('x', moved(sizes, i, index)), index, x), `item ${i} held at ${x}`).toBe(index);
    }
  }
});

test('a vertical list places its rows by their top and bottom edges', () => {
  const column = lay('y', [40, 40, 40]);
  expect(carry(column, 0, 20, 'y')).toBe(0);
  expect(carry(column, 0, 21, 'y')).toBe(1);
  expect(carry(column, 2, 59, 'y')).toBe(1);
  expect(carry(column, 2, 19, 'y')).toBe(0);
});

test('a right-to-left row reads its items from the right', () => {
  // Three 100 wide items, the first at the right.
  const row = [200, 100, 0].map((left) => ({ left, top: 0, width: 100, height: 40 }));
  expect(carry(row, 0, 151, 'x', true)).toBe(0);
  expect(carry(row, 0, 149, 'x', true)).toBe(1);
  expect(carry(row, 0, 0, 'x', true)).toBe(2);
});

// Six 100 by 60 tiles, three to a line with a gap of 10, read line by line.
const TILES = [0, 1, 2, 3, 4, 5].map((n) => ({ left: (n % 3) * 110, top: Math.floor(n / 3) * 70, width: 100, height: 60 }));

function carryTile(tiles: readonly DropRect[], i: number, left: number, top: number, rtl = false): number {
  return dropIndex({ ...tiles[i], left, top }, tiles[i], tiles.filter((_, j) => j !== i), { axis: 'grid', rtl });
}

test('a tile carried onto another line lands where it is held', () => {
  expect(carryTile(TILES, 2, 0, 70)).toBe(3);
  expect(carryTile(TILES, 3, 220, 0)).toBe(2);
  expect(carryTile(TILES, 5, 0, 0)).toBe(0);
  expect(carryTile(TILES, 0, 220, 70)).toBe(5);
});

test('a tile stays on its line until its top or bottom edge passes the middle of the next', () => {
  expect(carryTile(TILES, 0, 0, 34)).toBe(0);
  expect(carryTile(TILES, 4, 110, 36)).toBe(4);
  expect(carryTile(TILES, 0, 30, 5)).toBe(0);
  expect(carryTile(TILES, 0, 70, 0)).toBe(1);
});

test('a tile reaches the free place at the end of a short last line', () => {
  const five = TILES.slice(0, 5);
  expect(carryTile(five, 0, 220, 70)).toBe(4);
});

test('a tile alone on the last line stays last until it leaves the line', () => {
  const four = TILES.slice(0, 4);
  expect(carryTile(four, 3, 150, 70)).toBe(3);
  expect(carryTile(four, 3, 0, 0)).toBe(0);
});

test("a tile's place asks for itself once the others have moved", () => {
  const five = TILES.slice(0, 5);
  for (let i = 0; i < five.length; i++) {
    for (let left = -50; left <= 300; left += 5) {
      for (let top = -30; top <= 120; top += 5) {
        const index = carryTile(five, i, left, top);
        const at = { ...five[index], left, top };
        const settled = dropIndex(at, five[index], five.filter((_, j) => j !== index), { axis: 'grid' });
        expect(settled, `tile ${i} held at ${left}, ${top}`).toBe(index);
      }
    }
  }
});

test('a right-to-left grid reads each line from the right', () => {
  const mirrored = TILES.map((t) => ({ ...t, left: 220 - t.left }));
  expect(carryTile(mirrored, 0, 150, 0, true)).toBe(1);
  expect(carryTile(mirrored, 0, 0, 70, true)).toBe(5);
});

// DOMRect keeps its sides on the prototype, so a copy made with spread has none of them.
class MeasuredBox {
  #left: number;
  #top: number;
  #width: number;
  #height: number;
  constructor(r: DropRect) {
    this.#left = r.left;
    this.#top = r.top;
    this.#width = r.width;
    this.#height = r.height;
  }
  get left() { return this.#left; }
  get top() { return this.#top; }
  get width() { return this.#width; }
  get height() { return this.#height; }
}

test('measured boxes place a right to left row and grid as plain boxes do', () => {
  const row = lay('x', [300, 100, 100]).map((b) => ({ ...b, left: 500 - b.left - b.width }));
  const measured = row.map((b) => new MeasuredBox(b));
  for (const at of [0, 120, 250, 380]) {
    const plain = dropIndex({ ...row[0], left: at }, row[0], row.slice(1), { axis: 'x', rtl: true });
    const real = dropIndex(new MeasuredBox({ ...row[0], left: at }), measured[0], measured.slice(1), { axis: 'x', rtl: true });
    expect(real, `held at ${at}`).toBe(plain);
    const grid = dropIndex(new MeasuredBox({ ...row[0], left: at }), measured[0], measured.slice(1), { axis: 'grid', rtl: true });
    expect(grid, `grid, held at ${at}`).toBe(plain);
  }
});
