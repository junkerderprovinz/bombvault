// Theme resolution logic — shared by the server layout and tests.
// NOTE: do NOT add "use client" here. This module is called from the server
// component app/layout.tsx to read the theme cookie and set the initial
// data-theme attribute, avoiding any flash of wrong theme on load.

export const THEME_COOKIE = "bv_theme";
export const THEMES = ["dark", "light"] as const;
export type Theme = (typeof THEMES)[number];
export const DEFAULT_THEME: Theme = "dark";

/**
 * Resolve which theme to use from a cookie value.
 * If the stored value is not a valid theme name, fall back to DEFAULT_THEME.
 * First-visit behaviour (cookie absent) is handled by the client:
 * it reads prefers-color-scheme and writes the cookie.
 */
export function resolveTheme(cookie: string | null | undefined): Theme {
  if (cookie && (THEMES as readonly string[]).includes(cookie)) {
    return cookie as Theme;
  }
  return DEFAULT_THEME;
}

/** Persist the chosen theme for a year (client only). */
export function writeThemeCookie(theme: Theme): void {
  if (typeof document === "undefined") return;
  const maxAge = 60 * 60 * 24 * 365;
  document.cookie = `${THEME_COOKIE}=${theme}; path=/; max-age=${maxAge}; samesite=lax`;
}

