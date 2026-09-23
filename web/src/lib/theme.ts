// Theme: dark, light or system via data-theme on <html>, persisted in
// localStorage. "system" is the default and follows the OS live through
// prefers-color-scheme; picking dark or light overrides the OS until cleared.
//
// index.html's inline <head> script repeats the resolution below (STORAGE_KEY
// included) to set data-theme before first paint, since it cannot import this
// module. Keep the two in sync.
import { save as saveDisplayPrefs } from "./displayPrefs";

export type Theme = "dark" | "light" | "system";
export type ResolvedTheme = "dark" | "light";

const STORAGE_KEY = "bv-theme";
const DEFAULT: Theme = "system";

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

/** What is painted right now, with "system" resolved against the OS. */
export function getResolvedTheme(): ResolvedTheme {
  return resolve(getTheme());
}

function paint(theme: Theme): void {
  document.documentElement.setAttribute("data-theme", resolve(theme));
}

export function setTheme(theme: Theme): void {
  localStorage.setItem(STORAGE_KEY, theme);
  saveDisplayPrefs();
  paint(theme);
}

/** Flips between dark and light, always landing on an explicit choice rather
 * than "system". After the first toggle there is no way back to "system" short
 * of clearing localStorage; a third state would need its own icon and i18n
 * key. */
export function toggleTheme(): ResolvedTheme {
  const next: ResolvedTheme = getResolvedTheme() === "dark" ? "light" : "dark";
  setTheme(next);
  return next;
}

/** Subscribes `onChange` to OS dark/light flips whatever the stored preference;
 * callers that care only while on "system" check getTheme() themselves.
 * Returns the unsubscribe. */
export function onSystemThemeChange(onChange: () => void): () => void {
  const mql = window.matchMedia("(prefers-color-scheme: dark)");
  if (typeof mql.addEventListener === "function") {
    mql.addEventListener("change", onChange);
    return () => mql.removeEventListener("change", onChange);
  }
  // Safari before 14 has only the deprecated listener API.
  mql.addListener(onChange);
  return () => mql.removeListener(onChange);
}

let liveListenerAttached = false;

/** Called in main.tsx before the first render. Also attaches, once, a listener
 * that repaints on an OS theme change while on "system"; it lives as long as
 * the page. */
export function applyStoredTheme(): void {
  paint(getTheme());

  if (liveListenerAttached) return;
  liveListenerAttached = true;

  onSystemThemeChange(() => {
    if (getTheme() === "system") paint("system");
  });
}
