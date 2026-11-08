// Where a carried item belongs while it is dragged through a list ("Reordering
// by dragging" in design-language.md). It takes plain boxes, so a drag hook
// measures once and the tests need no DOM.

/** A box as `getBoundingClientRect` or the offset properties measure it. */
export interface DropRect {
  left: number;
  top: number;
  width: number;
  height: number;
}

/**
 * DropAxis is the way a list runs: along a row, down a column, or as a grid
 * whose items wrap into lines read one after the other.
 */
export type DropAxis = "x" | "y" | "grid";

export interface DropOptions {
  /** "y" when left out. */
  axis?: DropAxis;
  /** The lines read from the right. */
  rtl?: boolean;
}

/**
 * dropIndex is where the carried item goes among `others`, the list's other
 * items in their order. `held` is its unscaled box where it is held, and `from`
 * its place in the layout `others` were measured in. It passes a neighbour
 * once its edge in the direction of travel, from `from` towards `held`, crosses
 * the neighbour's middle, so an item of any size reaches both ends of the list.
 * In a grid its top or bottom edge decides the line the same way, and along
 * that line its start or end edge decides the place.
 */
export function dropIndex(
  held: DropRect,
  from: DropRect,
  others: readonly DropRect[],
  { axis = "y", rtl = false }: DropOptions = {},
): number {
  if (axis === "y") return before(others.map(down), leading(down(held), down(from)));
  // Mirrored, a line that reads from the right reads from the left. The sides
  // are copied by name, since a DOMRect keeps them as getters a spread drops.
  const read = (r: DropRect): DropRect =>
    rtl ? { left: -r.left - r.width, top: r.top, width: r.width, height: r.height } : r;
  if (axis === "x") return before(others.map(read).map(across), leading(across(read(held)), across(read(from))));
  return inGrid(read(held), read(from), others.map(read));
}

interface Span {
  start: number;
  size: number;
}

const across = (r: DropRect): Span => ({ start: r.left, size: r.width });
const down = (r: DropRect): Span => ({ start: r.top, size: r.height });

/** The edge an item leads with on its way from `from` to `at`. */
function leading(at: Span, from: Span): number {
  return at.start < from.start ? at.start : at.start + at.size;
}

/** How many of `spans` have their middle before `edge`. */
function before(spans: readonly Span[], edge: number): number {
  return spans.filter((s) => s.start + s.size / 2 < edge).length;
}

interface Line {
  top: number;
  bottom: number;
  items: Span[];
}

/** The boxes in reading order, split where one starts below the line so far. */
function linesOf(boxes: readonly DropRect[]): Line[] {
  const lines: Line[] = [];
  for (const b of boxes) {
    const line = lines[lines.length - 1];
    if (line && b.top < line.bottom) {
      line.top = Math.min(line.top, b.top);
      line.bottom = Math.max(line.bottom, b.top + b.height);
      line.items.push(across(b));
    } else {
      lines.push({ top: b.top, bottom: b.top + b.height, items: [across(b)] });
    }
  }
  return lines;
}

function inGrid(held: DropRect, from: DropRect, others: readonly DropRect[]): number {
  const lines = linesOf(others);
  const middle = (l: Line) => (l.top + l.bottom) / 2;
  const count = (n: number) => lines.slice(0, n).reduce((sum, l) => sum + l.items.length, 0);

  // The lines wholly above the item's place, and the line it shares with
  // others. An item alone on its line shares none.
  const gap = from.top + from.height / 2;
  const above = lines.filter((l) => l.bottom <= gap).length;
  const own = lines[above] && lines[above].top <= gap ? above : -1;

  // Moved onto another line, the item travels backward or forward in reading
  // order whatever way it went across, so that decides the edge along it.
  let line = own;
  let edge = leading(across(held), across(from));
  if (held.top < from.top) {
    const up = lines.findIndex((l, i) => i < above && middle(l) > held.top);
    if (up >= 0) [line, edge] = [up, held.left];
  } else if (held.top > from.top) {
    const start = own >= 0 ? own + 1 : above;
    let next = -1;
    for (const [i, l] of lines.entries()) {
      if (i < start) continue;
      if (middle(l) >= held.top + held.height) break;
      next = i;
    }
    if (next >= 0) [line, edge] = [next, held.left + held.width];
  }
  const target = lines[line];
  return target ? count(line) + before(target.items, edge) : count(above);
}
