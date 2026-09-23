import { save as saveDisplayPrefs } from "./displayPrefs";

// The accent colour lives in localStorage and is applied as --accent,
// --accent-contrast (the ink that reads best on the accent) and --accent-soft
// (a 14% alpha tint).

export const DEFAULT_ACCENT = "#FCC419";
export const DEFAULT_ACCENT_CONTRAST = "#161616";

const STORAGE_KEY = "bv-accent";

export function getAccent(): string {
  return localStorage.getItem(STORAGE_KEY) ?? DEFAULT_ACCENT;
}

export function setAccent(hex: string): void {
  localStorage.setItem(STORAGE_KEY, hex);
  saveDisplayPrefs();
  applyAccent(hex);
}

export function applyAccent(hex?: string): void {
  const color = hex ?? getAccent();
  const root = document.documentElement.style;
  root.setProperty("--accent", color);
  const ink = contrastOn(color);
  root.setProperty("--accent-contrast", ink);
  root.setProperty("--accent-soft", softTint(color));
}

/** applyStoredAccent runs in main.tsx before the first render, so the page
 * never flashes the default accent. */
export function applyStoredAccent(): void {
  applyAccent(getAccent());
}

/** contrastOn returns the dark or the white ink, whichever contrasts more with
 * the given colour, or the default ink for an unparseable hex. */
export function contrastOn(hex: string): string {
  const parsed = parseHex(hex);
  if (!parsed) return DEFAULT_ACCENT_CONTRAST;
  // The dark ink is Carbon's #161616; a warm near-black reads as a smudge on
  // yellow. Since it is not pure black, no fixed luminance cutoff picks the
  // right ink for every colour, so the two contrast ratios are compared.
  const bg = luminance(parsed.r, parsed.g, parsed.b);
  const darkInk = luminance(0x16, 0x16, 0x16);
  const whiteInk = 1; // luminance(255, 255, 255) === 1 exactly
  return contrastRatio(bg, darkInk) >= contrastRatio(bg, whiteInk) ? "#161616" : "#FFFFFF";
}

/** contrastRatio is the WCAG contrast ratio of two relative luminances, given
 * in either order. */
function contrastRatio(l1: number, l2: number): number {
  const lighter = Math.max(l1, l2);
  const darker = Math.min(l1, l2);
  return (lighter + 0.05) / (darker + 0.05);
}

/** softTint is the colour at 14% alpha, for washes behind accent-coloured UI.
 * An unparseable hex gives the default accent's tint. */
export function softTint(hex: string): string {
  const parsed = parseHex(hex) ?? parseHex(DEFAULT_ACCENT)!;
  return `rgba(${parsed.r}, ${parsed.g}, ${parsed.b}, 0.14)`;
}

/** parseHex parses a "#RRGGBB" colour and returns undefined for anything
 * else. */
export function parseHex(hex: string): { r: number; g: number; b: number } | undefined {
  if (!/^#[0-9a-fA-F]{6}$/.test(hex)) return undefined;
  const n = parseInt(hex.slice(1), 16);
  return { r: (n >> 16) & 255, g: (n >> 8) & 255, b: n & 255 };
}

// The accent presets are the rainbow's eight colours, so each has a checked ink
// pairing and the accent and rainbow cards offer the same swatches. The first
// five are GlimStone's shared presets in their shared order. Users can edit
// them; they are stored and validated like the rainbow palette.
export const DEFAULT_ACCENT_PRESETS: string[] = [
  "#FCC419", // Sunflower, the default accent
  "#1D99F3", // Blue
  "#6FDC8C", // Green
  "#FF8389", // Red
  "#BE95FF", // Purple
  "#FF832B", // Orange
  "#3DDBD9", // Teal
  "#FF7EB6", // Magenta
];

const PRESETS_STORAGE_KEY = "bv-accent-presets";

/** isValidAccentPresets reports whether p is a full set of valid colours. One
 * bad entry rejects the whole set, so a corrupt stored value never applies in
 * part. */
export function isValidAccentPresets(p: string[]): boolean {
  return p.length === DEFAULT_ACCENT_PRESETS.length && p.every((c) => parseHex(c) !== undefined);
}

function usablePresets(p: string[] | undefined): string[] {
  if (!p || !isValidAccentPresets(p)) return DEFAULT_ACCENT_PRESETS;
  return p;
}

/** getAccentPresets returns the stored presets, or the defaults when nothing
 * valid is stored. */
export function getAccentPresets(): string[] {
  try {
    const raw = localStorage.getItem(PRESETS_STORAGE_KEY);
    if (!raw) return DEFAULT_ACCENT_PRESETS;
    return usablePresets(JSON.parse(raw) as string[]);
  } catch {
    return DEFAULT_ACCENT_PRESETS;
  }
}

/** setAccentPresets stores the validated presets and returns them. Callers
 * take their state from the return value, which matches what was stored. */
export function setAccentPresets(next: string[]): string[] {
  const usable = usablePresets(next);
  try {
    localStorage.setItem(PRESETS_STORAGE_KEY, JSON.stringify(usable));
    saveDisplayPrefs();
  } catch {
    // Without storage the presets last until the next load.
  }
  return usable;
}

/**
 * luminance is the WCAG relative luminance. The sRGB channels are linearised
 * first, because the raw values overstate blue and understate green.
 */
function luminance(r: number, g: number, b: number): number {
  const lin = (c: number) => {
    const v = c / 255;
    return v <= 0.04045 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4);
  };
  return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
}
