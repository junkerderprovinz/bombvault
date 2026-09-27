import { useCallback, useState } from "react";

// ---------------------------------------------------------------------------
// useLoadMore — the ONE client-side list pagination primitive. Every mobile
// list destination renders `visible` and gates the "Load more" button on
// `hasMore`; no mobile list hand-rolls slicing.
//
// WHY A CONSTANT THRESHOLD, NEVER A GROWING OR VIEWPORT-DERIVED STEP:
// a fixed window keeps "where was that row" answers stable as the user pages
// — with an expanding window every append re-shuffles the row the user is
// looking at. The threshold is the caller's constant (default 20); the hook
// never derives it from viewport height or scroll position.
//
// WHY CLIENT-SIDE ONLY: listRuns() takes no params and web/src/lib/api.ts is
// frozen for this milestone, so there is no server pagination to bind — the
// slice is taken client-side over the array the consumer already holds.
//
// WHY NO IntersectionObserver / scroll listener / sentinel ref, EVER
// (infinite scroll is out of scope by recorded decision): auto-load steals scroll
// anchoring from the user and makes the footer unreachable; the only way the
// window grows is an explicit showMore() press from a hasMore-gated button.
// This file contains no observer and no scroll machinery by construction —
// the source-assert guard treats any appearance as a regression.
//
// RESET-ON-FILTER: typing in a bound search or
// toggling a filter chip resets the visible slice to the initial window. The
// hook resets when the `items` array IDENTITY changes, and consumers pass the
// FILTERED array — so slice and filter can never disagree (the consumer
// derives `visible` from the same array it filtered).
//
// LIVE-FEED PRESERVE KEY: the dashboard activity log (the first live-feed
// consumer) is the first consumer whose array identity changes WITHOUT a
// filter change — its
// merged list re-merges on every runs poll (10s), every SSE progress push and
// every idle-countdown tick. Strict reset-on-identity there collapses the
// reader's page back to 20 rows every few seconds — precisely the "where was
// that row" instability the constant threshold exists to prevent. Such consumers pass an opaque
// `preserveKey` describing ONLY their filter state: while the key is
// unchanged, a new identity is a data REFRESH — the window is kept, new rows
// extend the list at the bottom, and the slice clamps naturally if the list
// shrinks; a key change rewinds exactly like the default contract. Static
// lists (Containers, Files) pass no key and keep the strict identity
// behavior — unchanged from the base semantics above.
// ---------------------------------------------------------------------------

/**
 * The next visible count after one "Load more" press: one threshold added,
 * clamped into [0, total]. Pure so the window math is unit-proven directly
 * (activityLog's extractable-pure-fn model).
 */
export function loadMoreWindow(total: number, visible: number, threshold: number): number {
  return Math.max(0, Math.min(total, visible + threshold));
}

export function useLoadMore<T>(
  items: T[],
  threshold = 20,
  preserveKey?: string
): { visible: T[]; showMore: () => void; hasMore: boolean; reset: () => void } {
  const [count, setCount] = useState(threshold);
  // Render-time state adjust (React's documented "adjusting state when props
  // change" pattern): a new `items` IDENTITY is a new filter result, so the
  // slice resets to the initial window in the SAME render — no stale-slice
  // frame an effect would paint first, and no effect dep array to drift.
  // With a preserveKey in play, a same-key identity change is a live-feed
  // REFRESH instead: `seen` advances (so the next genuinely-new array is
  // still detected) but the window count survives it.
  const [seen, setSeen] = useState(items);
  const [seenKey, setSeenKey] = useState(preserveKey);
  if (seen !== items || seenKey !== preserveKey) {
    const refresh = preserveKey !== undefined && seenKey === preserveKey;
    setSeen(items);
    setSeenKey(preserveKey);
    if (!refresh) setCount(threshold);
  }

  const visible = items.slice(0, count);
  // hasMore is the ONLY signal consumers may gate the button on: no rows
  // beyond the window, no button (true at zero rows by construction).
  const hasMore = visible.length < items.length;

  const showMore = useCallback(() => {
    // Exactly one threshold per press — never "fill the viewport", never a
    // growing step.
    setCount((c) => loadMoreWindow(items.length, c, threshold));
  }, [items.length, threshold]);

  const reset = useCallback(() => setCount(threshold), [threshold]);

  return { visible, showMore, hasMore, reset };
}
