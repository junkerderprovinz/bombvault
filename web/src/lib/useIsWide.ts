import { useSyncExternalStore } from "react";

/** Tailwind's lg breakpoint. Below it the settings rail shows its glyphs
 *  alone, since the content column needs the room more than the names do. */
export const WIDE_QUERY = "(min-width: 64rem)";

let wideMql: MediaQueryList | null = null;

function currentWideMql(): MediaQueryList | null {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") return null;
  if (!wideMql) wideMql = window.matchMedia(WIDE_QUERY);
  return wideMql;
}

function subscribeWide(onChange: () => void): () => void {
  const mql = currentWideMql();
  if (!mql) return () => {};
  if (typeof mql.addEventListener === "function") {
    mql.addEventListener("change", onChange);
    return () => mql.removeEventListener("change", onChange);
  }
  mql.addListener(onChange);
  return () => mql.removeListener(onChange);
}

/** True at or above Tailwind's lg breakpoint (64rem), and wherever no media
 *  query can be asked, like getSnapshot's desktop default. */
export function useIsWide(): boolean {
  return useSyncExternalStore(subscribeWide, () => currentWideMql()?.matches ?? true, () => true);
}
