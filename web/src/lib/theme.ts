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

function paint(theme: Theme): void {
  getHtml().setAttribute("data-theme", resolve(theme));
}

export function setTheme(theme: Theme): void {
  localStorage.setItem(STORAGE_KEY, theme);
  paint(theme);
}

/** Explicit dark<->light toggle — lands on a real choice, never "system",
 * matching the existing two-state sidebar control. */
export function toggleTheme(): ResolvedTheme {
  const next: ResolvedTheme = getResolvedTheme() === "dark" ? "light" : "dark";
  setTheme(next);
  return next;
}

let liveListenerAttached = false;

/** Called at boot in main.tsx before first render. Also wires a live
 * listener so an OS-level theme change repaints immediately for anyone
 * still on "system" (attached once; safe to call repeatedly). */
export function applyStoredTheme(): void {
  paint(getTheme());

  if (liveListenerAttached) return;
  liveListenerAttached = true;

  const mql = window.matchMedia("(prefers-color-scheme: dark)");
  const onChange = () => {
    if (getTheme() === "system") paint("system");
  };
  if (typeof mql.addEventListener === "function") {
    mql.addEventListener("change", onChange);
  } else {
    // Safari < 14 fallback — deprecated but still the only API there.
    mql.addListener(onChange);
  }
}
