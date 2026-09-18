// ---------------------------------------------------------------------------
// USE MEDIA QUERY — the ONE breakpoint hook (and the ONE home of the desktop
// breakpoint literal), plus the ONE pointer-capability hook.
//
// DESKTOP_QUERY below is the single JS definition of "desktop" in the app,
// pinned to Tailwind's md breakpoint (`--breakpoint-md: 48rem`,
// node_modules/tailwindcss/theme.css) by src/lib/useMediaQuery.test.ts's
// source assert. It must never be copied: Layout's chrome switch (the
// Sidebar <-> bottom-bar + More-sheet decision) is the only consumer for now,
// and the CSS side keeps using `md:`/`max-md:` variants. A second JS
// literal somewhere else would let the JS chrome flip at one width while the
// CSS variants flip at another, flickering both chrome systems in the 1px
// window where they disagree — the fragile-pair discipline lib/theme.ts
// documents for its index.html duplicate, applied to the breakpoint.
//
// POINTER_COARSE_QUERY is a DIFFERENT axis and deliberately independent of
// the width one: it answers "does the primary input lack a fine pointer"
// (touch), not "is the window wide". Width decides which chrome mounts
// (Layout's Sidebar vs bottom bar); pointer capability decides interaction
// patterns — collapsing the two would hand a landscape phone (>=48rem wide,
// still coarse-pointer) a hover-designed desktop tree whose hover affordances
// and 24px targets a finger cannot use, and would hand a hybrid touchpad
// laptop a touch tree it did not ask for. This file stays the one home of
// BOTH literals so the "no second copy" discipline covers each; the guard
// test pins DESKTOP_QUERY as the only WIDTH literal and the exact
// coarse-pointer string alongside it.
//
// useSyncExternalStore, not useEffect+useState: the subscribe/getSnapshot
// contract is React's sanctioned shape for a browser API as a store and
// cannot tear between render and effect (a media query answering one value
// during render and another after the effect is exactly the torn snapshot a
// listener-in-effect hook can produce). The subscribe/unsubscribe shape
// mirrors onSystemThemeChange in theme.ts — addEventListener with the
// deprecated addListener/removeListener fallback for Safari < 14.
// ---------------------------------------------------------------------------
import { useSyncExternalStore } from "react";

/** Tailwind's md breakpoint, as a media query. The ONLY place this literal
 *  may live — see this file's header comment and the guard test, which fails
 *  with the why if the literal is altered or duplicated. */
export const DESKTOP_QUERY = "(min-width: 48rem)";

// One MediaQueryList for the lifetime of the page, created lazily so a
// windowless runtime (node-env test files import source modules freely) never
// touches `window` at module scope. null means "no matchMedia available".
let desktopMql: MediaQueryList | null = null;

function currentMql(): MediaQueryList | null {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") return null;
  if (!desktopMql) desktopMql = window.matchMedia(DESKTOP_QUERY);
  return desktopMql;
}

function subscribe(onChange: () => void): () => void {
  const mql = currentMql();
  if (!mql) return () => {};
  // Same shape as theme.ts's onSystemThemeChange — modern API first, Safari
  // < 14 fallback.
  if (typeof mql.addEventListener === "function") {
    mql.addEventListener("change", onChange);
    return () => mql.removeEventListener("change", onChange);
  }
  mql.addListener(onChange);
  return () => mql.removeListener(onChange);
}

function getSnapshot(): boolean {
  const mql = currentMql();
  // Desktop default when matchMedia is unavailable — consistent with the
  // desktop-default answer the jsdom stub (src/lib/testSetup/matchMedia.ts)
  // gives, so test suites keep asserting the desktop layout they were
  // written against.
  return mql ? mql.matches : true;
}

/** True at/above Tailwind's md breakpoint (48rem). Windowless/server snapshot:
 *  true (desktop default). */
export function useIsDesktop(): boolean {
  return useSyncExternalStore(subscribe, getSnapshot, () => true);
}

// ---------------------------------------------------------------------------
// Pointer-capability axis — an additive twin of the width hook above, same
// lazy-MediaQueryList / useSyncExternalStore / Safari<14 shape, over a second
// module-level list so the two queries can never share state.
// ---------------------------------------------------------------------------

/** The pointer-capability media query: true when the PRIMARY pointing device
 *  has no fine pointer (finger, stylus-as-touch). The only other input-axis
 *  literal in the app — see the header comment for why it must stay
 *  independent of DESKTOP_QUERY. */
export const POINTER_COARSE_QUERY = "(pointer: coarse)";

// One MediaQueryList for the lifetime of the page, created lazily so a
// windowless runtime never touches `window` at module scope (same rule as
// desktopMql above).
let pointerCoarseMql: MediaQueryList | null = null;

function currentCoarseMql(): MediaQueryList | null {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") return null;
  if (!pointerCoarseMql) pointerCoarseMql = window.matchMedia(POINTER_COARSE_QUERY);
  return pointerCoarseMql;
}

function subscribeCoarse(onChange: () => void): () => void {
  const mql = currentCoarseMql();
  if (!mql) return () => {};
  // Same shape as theme.ts's onSystemThemeChange — modern API first, Safari
  // < 14 fallback.
  if (typeof mql.addEventListener === "function") {
    mql.addEventListener("change", onChange);
    return () => mql.removeEventListener("change", onChange);
  }
  mql.addListener(onChange);
  return () => mql.removeListener(onChange);
}

function getCoarseSnapshot(): boolean {
  const mql = currentCoarseMql();
  // POINTER default when matchMedia is unavailable — deliberately the
  // OPPOSITE of getSnapshot above: jsdom (and the windowless server snapshot)
  // answer no coarse-pointer query, and every existing dom test was written
  // against the pointer tree, so the falsy answer keeps those suites on the
  // interaction mode they assert. A real browser always answers the query
  // itself.
  return mql ? mql.matches : false;
}

/** True when the primary pointer is coarse (touch input). Windowless/jsdom
 *  snapshot: false (pointer mode). Drives touch-specific interaction
 *  patterns — NEVER derive it from useIsDesktop (width is the chrome axis
 *  only). */
export function useIsCoarsePointer(): boolean {
  return useSyncExternalStore(subscribeCoarse, getCoarseSnapshot, () => false);
}
