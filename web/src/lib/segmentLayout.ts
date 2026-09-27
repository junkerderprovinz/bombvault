// How the pinned segments of a horizontal selector share the room they have
// ("The one horizontal selector" in design-language.md). The module has no
// imports, so a check can run it in Node without a bundler.

/**
 * SegmentWidths is how wide a segment is with its label on one line, and with
 * the label broken wherever it can break.
 */
export interface SegmentWidths {
  oneLine: number;
  narrowest: number;
}

/**
 * SegmentLayout is `perRow` pinned segments to a row or, with `byContent`, all
 * of them on one row, each as wide as its label and an even share of the rest.
 */
export interface SegmentLayout {
  byContent: boolean;
  perRow: number;
}

/**
 * clientWidth rounds the room to a whole pixel, so a row that fits exactly can
 * come out half a pixel short.
 */
const ROUNDING = 1;

/**
 * perRowFor is how many pinned segments share a row: all of them while they
 * fit, otherwise as even a share as the rows allow, so six that fit four to a
 * row go three and three and seven go four and three ("A selector that wraps
 * fills its box").
 */
export function perRowFor(count: number, room: number, width: number, gap: number): number {
  const fit = Math.max(1, Math.floor((room + gap) / (width + gap)));
  if (count <= fit) return count;
  const rows = Math.ceil(count / fit);
  return Math.ceil(count / rows);
}

/**
 * segmentLayout decides how segments pinned to `pinned` pixels fill `room`.
 * While no label wraps at the pinned width, they keep it. Where one would, and
 * the segments fit side by side at their one-line widths, they share the row
 * by those widths instead. Only when they do not fit that way either do they
 * wrap into even rows at the pinned width.
 *
 * A label of one word cannot wrap. Too long for the pinned width, it runs into
 * the segment's padding, and the selector keeps its pinned widths.
 */
export function segmentLayout(room: number, pinned: number, segments: SegmentWidths[], gap: number): SegmentLayout {
  const count = segments.length;
  const wraps = segments.some((s) => s.oneLine > pinned && s.narrowest < s.oneLine);
  const inline = segments.reduce((sum, s) => sum + s.oneLine, 0) + gap * (count - 1);
  if (wraps && inline <= room + ROUNDING) return { byContent: true, perRow: count };
  return { byContent: false, perRow: perRowFor(count, room, pinned, gap) };
}
