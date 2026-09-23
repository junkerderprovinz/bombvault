// Placement math for the shared `.glim-bubble` tooltip and info bubble,
// ported from GlimStone's reference/tooltip.ts with the same 8px viewport
// margin and the same clamp-then-flip order. Callers do the one DOM read
// (getBoundingClientRect, offsetWidth, offsetHeight) and pass plain numbers,
// so the math is tested without a DOM.

export interface BubbleTriggerRect {
  left: number;
  right: number;
  top: number;
  bottom: number;
}

export interface BubbleSize {
  width: number;
  height: number;
}

export interface Viewport {
  width: number;
  height: number;
}

export interface BubblePosition {
  left: number;
  top: number;
  /** True when the bubble opens above the trigger, because opening below
   *  would clip the viewport's bottom edge and there is room above. */
  above: boolean;
}

/** The 8px viewport margin of reference/tooltip.ts. */
export const BUBBLE_VIEWPORT_MARGIN = 8;

/**
 * computeBubblePosition clamps the bubble horizontally into the viewport,
 * keeping a margin, and flips it above the trigger when opening below would
 * clip the bottom edge and there is room above. A trigger at the very top
 * keeps opening downward, since flipping into negative space only trades one
 * clipped edge for another.
 *
 * `bubble` is the rendered size (offsetWidth, offsetHeight), because the
 * height depends on how many lines the tip wraps to. Measure it in a
 * useLayoutEffect so the corrected position lands before the first paint.
 */
export function computeBubblePosition(
  trigger: BubbleTriggerRect,
  bubble: BubbleSize,
  viewport: Viewport,
  margin: number = BUBBLE_VIEWPORT_MARGIN
): BubblePosition {
  const centerX = trigger.left + (trigger.right - trigger.left) / 2;
  const halfWidth = bubble.width / 2;
  const left = Math.max(
    margin + halfWidth,
    Math.min(viewport.width - margin - halfWidth, centerX)
  );

  const opensBelowClips = trigger.bottom + margin + bubble.height > viewport.height;
  const roomAbove = trigger.top - margin - bubble.height >= 0;
  const above = opensBelowClips && roomAbove;
  const unclamped = above ? trigger.top - margin - bubble.height : trigger.bottom + margin;
  // A bubble that fits the viewport but neither gap is pulled back inside:
  // every consumer is position:fixed, so an overhang could not be scrolled
  // into view. A bubble taller than the viewport clips wherever it goes, and
  // moving it up would only cover the trigger as well, so it stays put.
  const fits = bubble.height + 2 * margin <= viewport.height;
  const top = fits
    ? Math.max(margin, Math.min(viewport.height - margin - bubble.height, unclamped))
    : unclamped;

  return { left, top, above };
}
