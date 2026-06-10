// ---------------------------------------------------------------------------
// Accent color — persisted in localStorage, applied as CSS variable --accent
// ---------------------------------------------------------------------------

export const DEFAULT_ACCENT = "#FCC419";
export const DEFAULT_ACCENT_CONTRAST = "#161616";

const STORAGE_KEY = "bv-accent";

export function getAccent(): string {
  return localStorage.getItem(STORAGE_KEY) ?? DEFAULT_ACCENT;
}

export function setAccent(hex: string): void {
  localStorage.setItem(STORAGE_KEY, hex);
  applyAccent(hex);
}

export function applyAccent(hex?: string): void {
  const color = hex ?? getAccent();
  document.documentElement.style.setProperty("--accent", color);
}

/** Called at boot in main.tsx before first render (flash prevention). */
export function applyStoredAccent(): void {
  applyAccent(getAccent());
}
