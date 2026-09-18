import { save as saveDisplayPrefs } from "./displayPrefs";
// ---------------------------------------------------------------------------
// Theme — dark / light / system via data-theme on <html> + localStorage
//
// "system" follows the OS live via prefers-color-scheme (design-language.md:
// "Default is system, not a hard-coded light or dark — an app that opens
// dark on a light-mode OS made a choice nobody asked it to make"). A user
// who explicitly picks dark or light overrides the OS until they clear it.
//
// index.html's inline <head> script duplicates the resolution logic below
// (STORAGE_KEY included) to paint data-theme synchronously before first
// paint, since it can't import this module. Keep the two in sync.
// ---------------------------------------------------------------------------

export type Theme = "dark" | "light" | "system";
export type ResolvedTheme = "dark" | "light";

const STORAGE_KEY = "bv-theme";
const DEFAULT: Theme = "system";

function getHtml(): HTMLElement {
  return document.documentElement;
}

function systemPrefersDark(): boolean {
  return window.matchMedia("(prefers-color-scheme: dark)").matches;
}

function resolve(theme: Theme): ResolvedTheme {
  return theme === "system" ? (systemPrefersDark() ? "dark" : "light") : theme;
}

/** The stored preference: an explicit "dark"/"light", or "system" if unset. */
export function getTheme(): Theme {
  const stored = localStorage.getItem(STORAGE_KEY);
  if (stored === "dark" || stored === "light" || stored === "system") return stored;
  return DEFAULT;
}

/** What's actually painted right now — "system" resolved against the OS. */
export function getResolvedTheme(): ResolvedTheme {
  return resolve(getTheme());
}

// The browser-chrome color (mobile address bar / task switcher) per resolved
// theme — mirrors --carbon-bg's dark/light values. PAIRED with index.html's
// static <meta name="theme-color" content="#161616"> fallback the same way
// this file's STORAGE_KEY/resolution logic is paired with index.html's inline
// FOUC script (see the header above: "Keep the two in sync"): the static value
// is what shows before JS runs, paint() takes over on every theme application,
// and the two sets of values must change together.
const THEME_COLOR: Record<ResolvedTheme, string> = {
  dark: "#161616",
  light: "#f4f4f4",
};

function paint(theme: Theme): void {
  const resolved = resolve(theme);
  getHtml().setAttribute("data-theme", resolved);
  // Mirror into the live meta theme-color tag. paint() is the single
  // application choke point — setTheme, applyStoredTheme and the system-flip
  // listener all funnel through it — so this one write covers every path.
  // Null-guarded with ?. : jsdom test environments have no index.html head,
  // so the meta tag may not exist.
  document
    .querySelector('meta[name="theme-color"]')
    ?.setAttribute("content", THEME_COLOR[resolved]);
}

export function setTheme(theme: Theme): void {
  localStorage.setItem(STORAGE_KEY, theme);
  saveDisplayPrefs();
  paint(theme);
}

/** Explicit dark<->light toggle — lands on a real choice, never "system",
 * matching the existing two-state sidebar control.
 *
 * KNOWN GAP, left deliberately: once a user's first toggle click moves them
 * off "system" (the default), there is currently no UI path back to it —
 * only clearing localStorage does. Giving the sidebar control a real third
 * state needs a new icon, a new i18n key across all locales (this project's
 * own established per-key-across-every-locale discipline), and a decision on
 * cycle order — real UI design work, not a mechanical follow-on to this
 * token/theme-foundation task. Flagged as explicit deferred scope, not an
 * oversight. */
export function toggleTheme(): ResolvedTheme {
  const next: ResolvedTheme = getResolvedTheme() === "dark" ? "light" : "dark";
  setTheme(next);
  return next;
}

/** Subscribes `onChange` to OS-level dark/light flips (fires on every flip,
 * regardless of the stored preference — callers that only care while on
 * "system" check `getTheme()` inside their own callback, as both call sites
 * below do). Returns an unsubscribe function. Isolates the Safari < 14
 * `addListener`/`removeListener` fallback in one place instead of it being
 * copied at every call site. */
export function onSystemThemeChange(onChange: () => void): () => void {
  const mql = window.matchMedia("(prefers-color-scheme: dark)");
  if (typeof mql.addEventListener === "function") {
    mql.addEventListener("change", onChange);
    return () => mql.removeEventListener("change", onChange);
  }
  // Safari < 14 fallback — deprecated but still the only API there.
  mql.addListener(onChange);
  return () => mql.removeListener(onChange);
}

let liveListenerAttached = false;

/** Called at boot in main.tsx before first render. Also wires a live
 * listener so an OS-level theme change repaints immediately for anyone
 * still on "system" (attached once; safe to call repeatedly — the
 * subscription is intentionally never torn down for the lifetime of the
 * page, unlike Sidebar.tsx's own use of onSystemThemeChange). */
export function applyStoredTheme(): void {
  paint(getTheme());

  if (liveListenerAttached) return;
  liveListenerAttached = true;

  onSystemThemeChange(() => {
    if (getTheme() === "system") paint("system");
  });
}
