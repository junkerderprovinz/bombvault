// Theme: dark, light or system via data-theme on <html>, persisted in
// localStorage. "system" is the default and follows the OS live through
// prefers-color-scheme; picking dark or light overrides the OS until cleared.
//
// OLED is data-oled on <html>, which index.css applies only while the dark
// theme shows, so it can stay on through a switch to light. It belongs to the
// screen rather than the person, so it is kept on this device and never with
// the look the server keeps: the same server opens on an LCD laptop too.
//
// index.html's inline <head> script repeats the resolution below (the storage
// keys included) to set both attributes before first paint, since it cannot
// import this module. Keep the two in sync.
import { appDevice, tellAppOled } from "./appDevice";
import { save as saveDisplayPrefs } from "./displayPrefs";

export type Theme = "dark" | "light" | "system";
export type ResolvedTheme = "dark" | "light";

const STORAGE_KEY = "bv-theme";
const OLED_KEY = "bv-oled";
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

// The browser-chrome color (mobile address bar, task switcher, the app's
// system bars) per resolved theme; mirrors --carbon-bg. index.html's static
// <meta name="theme-color"> and its inline script carry the same values, and
// the three change together.
const THEME_COLOR: Record<ResolvedTheme, string> = {
  dark: "#161616",
  light: "#f4f4f4",
};
const OLED_COLOR = "#000000";

/** Whether this screen wants true black under the dark theme. Inside the
 *  Android app the phone's choice counts, so every server shows it alike. */
export function getOled(): boolean {
  const device = appDevice();
  if (device) return device.oled;
  try {
    return localStorage.getItem(OLED_KEY) === "1";
  } catch {
    return false;
  }
}

export function setOled(on: boolean): void {
  try {
    if (on) localStorage.setItem(OLED_KEY, "1");
    else localStorage.removeItem(OLED_KEY);
  } catch {
    // The switch still holds until the page closes.
  }
  tellAppOled(on);
  paint(getTheme());
}

/** Whether the dark theme is painted now, chosen or from the device. */
export function darkShowing(): boolean {
  return document.documentElement.getAttribute("data-theme") === "dark";
}

/** Calls `onChange` whenever the painted theme changes, so a settings page can
 *  say whether OLED is in effect. Returns the function that stops watching. */
export function watchDarkShowing(onChange: () => void): () => void {
  const observer = new MutationObserver(onChange);
  observer.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });
  return () => observer.disconnect();
}

function paint(theme: Theme): void {
  const resolved = resolve(theme);
  const oled = getOled();
  const root = document.documentElement;
  root.setAttribute("data-theme", resolved);
  if (oled) root.setAttribute("data-oled", "");
  else root.removeAttribute("data-oled");
  // paint() is where setTheme, applyStoredTheme and the system-flip listener
  // all meet, so this one write keeps the browser's chrome right on every
  // path. jsdom tests have no index.html head, hence the ?.
  document
    .querySelector('meta[name="theme-color"]')
    ?.setAttribute("content", resolved === "dark" && oled ? OLED_COLOR : THEME_COLOR[resolved]);
}

export function setTheme(theme: Theme): void {
  localStorage.setItem(STORAGE_KEY, theme);
  saveDisplayPrefs();
  paint(theme);
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
