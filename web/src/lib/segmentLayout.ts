// How the segments of a horizontal selector share their one row ("The one
// horizontal selector" in design-language.md). The module has no imports, so
// a check can run it in Node without a bundler.

/**
 * How a strip holds its labels on its one row: at its own widths while they
 * fit there, at each label's one-line width while they fit together, as
 * glyphs alone after that, and last in even shares with the long labels cut
 * short.
 */
export type SelectorFit = "even" | "content" | "squeezed" | "truncated";

/**
 * SegmentFit is how a strip holds its labels, and for each segment whether
 * its label is cut short.
 */
export interface SegmentFit {
  fit: SelectorFit;
  cut: boolean[];
}

/**
 * clientWidth rounds the room to a whole pixel, so a row that fits exactly can
 * come out half a pixel short.
 */
const ROUNDING = 1;

/**
 * segmentFit decides how segments whose labels need `oneLine` pixels on one
 * line share `room`. `even` is the width each one takes in the strip's own
 * layout, pinned or an even share, and null for a strip whose segments hug
 * their labels. `glyphs` says every segment may show its glyph alone.
 */
export function segmentFit(
  room: number,
  even: number | null,
  oneLine: readonly number[],
  gap: number,
  glyphs: boolean,
): SegmentFit {
  const count = oneLine.length;
  const free = room + ROUNDING - gap * (count - 1);
  const whole = (fit: SelectorFit): SegmentFit => ({ fit, cut: oneLine.map(() => false) });
  if (even !== null && even * count <= free && oneLine.every((w) => w <= even + ROUNDING)) return whole("even");
  if (oneLine.reduce((sum, w) => sum + w, 0) <= free) return whole("content");
  if (glyphs) return whole("squeezed");
  const share = (room - gap * (count - 1)) / count;
  return { fit: "truncated", cut: oneLine.map((w) => w > share + ROUNDING) };
}
