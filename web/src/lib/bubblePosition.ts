// ---------------------------------------------------------------------------
// bubblePosition — pure placement math for the shared `.glim-bubble` tooltip/
// info-bubble chrome (design-language.md "The tooltip and info bubble";
// GlimStone's reference/tooltip.ts is the canonical implementation this is
// ported from, pixel-for-pixel: same 8px viewport margin, same clamp-then-
// flip order).
//
// Bug this exists to fix: both call sites that render a `.glim-bubble`
// (InfoBubble.tsx's (i) icon and Selector.tsx's SelectorTab `tip`) used to
// position the bubble by centring it on the trigger and always opening
// downward, with NO viewport awareness at all — `{ left: r.left + r.width/2,
// top: r.bottom + 6 }`, full stop. A trigger near the right edge pushed the
// (translateX(-50%)-centred) bubble half off-screen horizontally; a trigger
// low enough on a tall page (or with a long enough tip, like recovery.why's
// multi-sentence explanation) pushed it off the bottom vertically. Reported
// live against the "Wiederherstellungskit" ("Recovery kit") bubble in
// Settings.tsx's encryption section, but the bug was in the shared engine,
// not that one call site — so the fix lives here, once, for every bubble in
// the app.
//
// Pure and DOM-free on purpose (no jsdom needed to exercise it — see
// bubblePosition.test.ts), matching this repo's own established split for
// positioning/navigation math (Selector.tsx's stepFor/nextFocusIndex/
// rovedIndex in Selector.test.ts): callers do the one DOM read
// (getBoundingClientRect/offsetWidth/offsetHeight) and hand plain numbers in.
// ---------------------------------------------------------------------------

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
  /** True when the bubble was flipped to open above the trigger instead of
   *  below it, because opening below would have clipped the viewport's
   *  bottom edge and there was actually room above to flip into. */
  above: boolean;
}

/** Matches reference/tooltip.ts's own 8px viewport margin exactly. */
export const BUBBLE_VIEWPORT_MARGIN = 8;

/**
 * computeBubblePosition — clamp horizontally into the viewport (with a
 * margin, never flush against the edge), and flip above the trigger when
 * opening below would clip the viewport's bottom edge AND there's actually
 * room above to flip into (reference/tooltip.ts's own documented behaviour:
 * "a trigger pinned to the very top of the page keeps opening downward
 * regardless" — flipping into negative space would just trade one clipped
 * edge for another).
 *
 * `bubble` must be the bubble's REAL rendered size (offsetWidth/
 * offsetHeight), not an assumed constant — the actual height depends on how
 * many lines the tip text wraps to, which depends on the tip's own length,
 * not just the CSS max-width. Callers measure this after the bubble has
 * mounted (e.g. in a `useLayoutEffect`, so the corrected position lands
 * before the first paint rather than causing a visible jump).
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
  const top = above ? trigger.top - margin - bubble.height : trigger.bottom + margin;

  return { left, top, above };
}
