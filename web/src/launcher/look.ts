// The launcher looks like the first server in its list until someone sets a
// look here, as KnightLoader's app follows its default instance. Motion, disco
// and the language stay the phone's own: they are about how this person uses
// this phone, and a server's choice for its browser says nothing about that.
import { applyStoredAccent } from "../lib/accent";
import { applyStoredRainbow } from "../lib/appearance";
import { applyStoredDisco } from "../lib/disco";
import { applyStoredShape } from "../lib/shape";
import { applyStoredTheme } from "../lib/theme";

/** The display preferences a followed server decides. */
export const FOLLOWED_KEYS = ["bv-theme", "bv-accent", "bv-accent-presets", "bv-rainbow", "bv-shape"] as const;

/** Set while the look is this phone's own. */
const LOCAL_KEY = "bv-app-look-local";

/** The phone's own look, put away while a server's is followed. */
const SHELF_KEY = "bv-app-look-shelf";

/** Fired after a followed look was written, so open pages read it again. */
export const FOLLOWED_EVENT = "bv-app-look-followed";

export function following(): boolean {
  try {
    return localStorage.getItem(LOCAL_KEY) === null;
  } catch {
    return true;
  }
}

/**
 * setFollowing goes back to following, putting the phone's own look away, or
 * stops following and gives that look back. The first time there is nothing
 * to give back, so the followed look stays on screen as the phone's own.
 */
export function setFollowing(on: boolean): void {
  try {
    if (on) {
      const shelf: Record<string, string> = {};
      for (const key of FOLLOWED_KEYS) {
        const value = localStorage.getItem(key);
        if (value !== null) shelf[key] = value;
      }
      localStorage.setItem(SHELF_KEY, JSON.stringify(shelf));
      localStorage.removeItem(LOCAL_KEY);
      return;
    }
    localStorage.setItem(LOCAL_KEY, "1");
    const shelf = localStorage.getItem(SHELF_KEY);
    if (shelf === null) return;
    localStorage.removeItem(SHELF_KEY);
    write(JSON.parse(shelf) as Record<string, unknown>);
  } catch {
    // Without storage the look is the default either way.
  }
}

/**
 * adopt writes the followed server's look, read from the body its display
 * preferences route answers with, and repaints when anything changed. A body
 * that is not BombVault's leaves the look as it is.
 */
export function adopt(body: string): void {
  let prefs: Record<string, unknown>;
  try {
    const parsed = JSON.parse(body) as { prefs?: Record<string, unknown> };
    if (!parsed.prefs || typeof parsed.prefs !== "object") return;
    prefs = parsed.prefs;
  } catch {
    return;
  }
  write(prefs);
}

/** Puts the followed keys of [prefs] in place and repaints when one changed. */
function write(prefs: Record<string, unknown>): void {
  let changed = false;
  for (const key of FOLLOWED_KEYS) {
    const value = prefs[key];
    if (typeof value !== "string") continue;
    try {
      if (localStorage.getItem(key) !== value) {
        localStorage.setItem(key, value);
        changed = true;
      }
    } catch {
      return;
    }
  }
  if (!changed) return;
  applyStoredTheme();
  applyStoredAccent();
  applyStoredRainbow({ animate: false });
  applyStoredShape();
  // The disco walk reads the rainbow, so it decides again whether it runs.
  applyStoredDisco();
  window.dispatchEvent(new Event(FOLLOWED_EVENT));
}

/** Keeps the look on screen as the phone's own, after a change made while following. */
export function keepAsOwn(): void {
  try {
    localStorage.setItem(LOCAL_KEY, "1");
    localStorage.removeItem(SHELF_KEY);
  } catch {
    // See setFollowing.
  }
}
