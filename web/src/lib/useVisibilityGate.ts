// ---------------------------------------------------------------------------
// Use visibility gate; the one Page-Visibility hook (the visibility pause gate), plus
// the plain read non-React consumers use.
//
// The contract is that SSE consumers and poll timers pause while the
// page is hidden and reconcile from server state when it returns. The fix that makes it work:
// the mechanism: the gate lives on the consumer side, around the frozen
// lib/progress.ts singleton; a hidden page unmounts its progress consumers,
// the singleton's ref-count drops to zero and closeSource() closes the shared
// EventSource (dropping its cached state too), and the remount on return
// reconnects into the backend's snapshot replay. Nothing here ever reads a
// client clock or a timestamp to infer what happened while hidden; the
// return-trip truth comes from refetched server records (listRuns), never
// from extrapolation.
//
// One home, like lib/useMediaQuery.ts's two axis literals: every consumer
// answers "is the page visible" through this module. A second module reading
// document.visibilityState directly would fork the visibility axis the same
// way a second breakpoint literal forks the width axis; the gate's whole
// value is that "hidden" means exactly one thing to every consumer that
// pauses on it.
//
// useSyncExternalStore, not a listener-in-effect hook: same reasoning as
// useMediaQuery.ts's header; visibilitychange is a browser API acting as a
// store, and the subscribe/getSnapshot contract cannot tear between render
// and effect (a gate answering "visible" during render and "hidden" after
// the effect would mount/unmount a consumer for nothing).
//
// Windowless default visible, in every fallback position: node-env tests
// import source modules freely (useMediaQuery's documented reason), and the
// server snapshot must agree; "visible" is the safe default in both
// directions. The opposite default would pause every consumer forever in any
// document-less context; this default degrades to exactly the pre-gate
// behavior (consumers stay live), never to a frozen UI.
// ---------------------------------------------------------------------------
import { useSyncExternalStore } from "react";

/** Plain visibility read for non-React consumers; backupWatch's poll chain
 *  (lib/backupWatch.ts) schedules its hops from inside a setTimeout closure
 *  far from any render, so it needs a function, not a hook. Same single
 *  source of truth as the hook below; windowless default: visible. */
export function isPageVisible(): boolean {
  if (typeof document === "undefined") return true;
  return document.visibilityState !== "hidden";
}

function subscribeVisibility(onChange: () => void): () => void {
  if (typeof document === "undefined" || typeof document.addEventListener !== "function") {
    return () => {};
  }
  document.addEventListener("visibilitychange", onChange);
  return () => document.removeEventListener("visibilitychange", onChange);
}

function getVisibilitySnapshot(): boolean {
  return isPageVisible();
}

/** True while the page is visible (the pause gate). Windowless/server
 *  snapshot: true (visible default; see the header). Consumers render their
 *  live machinery conditionally on this: `{visible && <progress consumer/>}`.
 *  Mounting is the subscription (lib/progress.ts's ref-count), so unmounting
 *  on hidden is the unsubscribe. */
export function useVisibilityGate(): boolean {
  return useSyncExternalStore(subscribeVisibility, getVisibilitySnapshot, () => true);
}
