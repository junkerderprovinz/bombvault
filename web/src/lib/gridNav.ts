import { useRef, useState, type KeyboardEvent } from "react";

// Keyboard movement through a listbox of tiles. The grid is one tab stop and
// the keys only move focus; Enter or Space picks the focused tile as its own
// click. Rows are read from where the tiles stand, so a grid may wrap at any
// width and hold several groups one after another.

export type GridKey = "ArrowLeft" | "ArrowRight" | "ArrowUp" | "ArrowDown" | "Home" | "End";

const GRID_KEYS: readonly string[] = ["ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown", "Home", "End"];

/** Where a tile stands on screen. */
export interface TileBox {
  top: number;
  left: number;
  width: number;
}

// Tiles in one row share their top edge; a pixel of slack absorbs rounding.
const SAME_ROW = 1;

/**
 * nextGridIndex is the tile focus moves to from `current`. Left and right step
 * through the list in reading order and stop at its ends; up and down go to the
 * tile of the next row above or below whose centre is nearest. With nothing
 * focused yet it starts at the first tile; an empty grid answers -1.
 */
export function nextGridIndex(key: GridKey, current: number, boxes: TileBox[], rtl: boolean): number {
  const count = boxes.length;
  if (count === 0) return -1;
  if (key === "Home") return 0;
  if (key === "End") return count - 1;
  const here = boxes[current];
  if (!here) return 0;
  if (key === "ArrowLeft" || key === "ArrowRight") {
    const step = (key === "ArrowRight") !== rtl ? 1 : -1;
    return Math.min(count - 1, Math.max(0, current + step));
  }
  const down = key === "ArrowDown";
  const tops = boxes
    .map((b) => b.top)
    .filter((top) => (down ? top > here.top + SAME_ROW : top < here.top - SAME_ROW));
  if (tops.length === 0) return current;
  const row = down ? Math.min(...tops) : Math.max(...tops);
  const centre = (b: TileBox) => b.left + b.width / 2;
  let best = current;
  let bestDistance = Infinity;
  boxes.forEach((b, i) => {
    if (Math.abs(b.top - row) > SAME_ROW) return;
    const distance = Math.abs(centre(b) - centre(here));
    if (distance < bestDistance) {
      best = i;
      bestDistance = distance;
    }
  });
  return best;
}

/**
 * useGridNav gives a listbox of `count` tiles its keys and its one tab stop,
 * which rests on the tile last focused, else on `selected`, else the first.
 * Spread `onKeyDown` on the listbox and `tileProps(i)` on each tile.
 */
export function useGridNav(count: number, selected: number) {
  const tiles = useRef<(HTMLElement | null)[]>([]);
  const [focused, setFocused] = useState(-1);
  const stop = focused >= 0 && focused < count ? focused : selected >= 0 && selected < count ? selected : 0;

  function onKeyDown(e: KeyboardEvent<HTMLElement>) {
    // Alt with an arrow is the browser's back and forward.
    if (!GRID_KEYS.includes(e.key) || e.altKey || e.ctrlKey || e.metaKey) return;
    const nodes = tiles.current.slice(0, count);
    const boxes = nodes.map((node) => {
      const r = node?.getBoundingClientRect();
      return { top: r?.top ?? 0, left: r?.left ?? 0, width: r?.width ?? 0 };
    });
    const at = nodes.findIndex((node) => node === document.activeElement);
    const rtl = getComputedStyle(e.currentTarget).direction === "rtl";
    const next = nextGridIndex(e.key as GridKey, at < 0 ? stop : at, boxes, rtl);
    if (next < 0) return;
    e.preventDefault();
    setFocused(next);
    nodes[next]?.focus();
  }

  function tileProps(i: number) {
    return {
      ref: (node: HTMLElement | null) => {
        tiles.current[i] = node;
      },
      tabIndex: i === stop ? 0 : -1,
      onFocus: () => setFocused(i),
    };
  }

  return { onKeyDown, tileProps };
}
