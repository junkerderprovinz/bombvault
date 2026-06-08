// ---------------------------------------------------------------------------
// Theme — dark / light via data-theme on <html> + localStorage
// ---------------------------------------------------------------------------

type Theme = "dark" | "light";

const STORAGE_KEY = "bv-theme";
const DEFAULT: Theme = "dark";

function getHtml(): HTMLElement {
  return document.documentElement;
}

export function getTheme(): Theme {
  const stored = localStorage.getItem(STORAGE_KEY);
  if (stored === "dark" || stored === "light") return stored;
  return DEFAULT;
}

export function setTheme(theme: Theme): void {
  localStorage.setItem(STORAGE_KEY, theme);
  getHtml().setAttribute("data-theme", theme);
}

export function toggleTheme(): Theme {
  const next: Theme = getTheme() === "dark" ? "light" : "dark";
  setTheme(next);
  return next;
}

/** Called at boot in main.tsx before first render. */
export function applyStoredTheme(): void {
  getHtml().setAttribute("data-theme", getTheme());
}
