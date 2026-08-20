// ---------------------------------------------------------------------------
// Accent color — persisted in localStorage, applied as CSS custom properties
// --accent / --accent-contrast / --accent-soft.
//
// --accent-contrast is COMPUTED from the chosen accent's sRGB luminance
// (never a fixed hex) so a dark custom accent gets light ink and a light
// custom accent gets dark ink — ported from GlimStone's reference/
// appearance.ts (contrastOn/luminance). --accent-soft is set alongside it
// as a fixed 14%-alpha tint of the same colour, matching reference's own
// applyAccent().
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
  const root = document.documentElement.style;
  root.setProperty("--accent", color);
  root.setProperty("--accent-contrast", contrastOn(color));
  root.setProperty("--accent-soft", softTint(color));
}

/** Called at boot in main.tsx before first render (flash prevention). */
export function applyStoredAccent(): void {
  applyAccent(getAccent());
}

// ---------------------------------------------------------------------------
// Pure colour math — no DOM, so it's unit-tested directly (see
// accent.test.ts) without needing jsdom in this project's node-environment
// test runner.
// ---------------------------------------------------------------------------

/** contrastOn is black or white, whichever is readable on top of the given
 * colour. Falls back to the default accent's own contrast for an
 * unparseable hex rather than throwing. */
export function contrastOn(hex: string): string {
  const parsed = parseHex(hex);
  if (!parsed) return DEFAULT_ACCENT_CONTRAST;
  // Carbon's own ink (#161616), not a warm near-black: on a yellow accent a
  // brown-tinted black reads as a smudge.
  //
  // A single hardcoded luminance cutoff (this used to be `> 0.55`) is the
  // wrong shape for this decision: the real WCAG crossover between "black
  // ink wins" and "white ink wins" isn't a fixed midpoint once one of the
  // two candidates is #161616 rather than true #000000, and 5 of the 8
  // rainbow hues (design-language.md's "The colour engine") landed on the
  // wrong side of 0.55 as a result — e.g. #FF8389 (rainbow red) got white
  // ink at ~2.37:1 when #161616 gives it ~7.63:1 (both measured live, see
  // accent.test.ts). Comparing the two candidates' ACTUAL contrast ratios
  // against the background and picking the higher one is self-correcting
  // for any background colour, with no cutoff to mistune.
  const bg = luminance(parsed.r, parsed.g, parsed.b);
  const darkInk = luminance(0x16, 0x16, 0x16);
  const whiteInk = 1; // luminance(255, 255, 255) === 1 exactly
  return contrastRatio(bg, darkInk) >= contrastRatio(bg, whiteInk) ? "#161616" : "#FFFFFF";
}

/**
 * contrastRatio is the standard WCAG contrast ratio between two relative
 * luminances: (lighter + 0.05) / (darker + 0.05). Order of the two
 * arguments doesn't matter — the lighter/darker pick happens inside.
 */
function contrastRatio(l1: number, l2: number): number {
  const lighter = Math.max(l1, l2);
  const darker = Math.min(l1, l2);
  return (lighter + 0.05) / (darker + 0.05);
}

/** softTint is a fixed 14%-alpha rgba of the given colour, used for tinted
 * backgrounds/washes behind accent-coloured UI. Falls back to the default
 * accent's own tint for an unparseable hex. */
export function softTint(hex: string): string {
  const parsed = parseHex(hex) ?? parseHex(DEFAULT_ACCENT)!;
  return `rgba(${parsed.r}, ${parsed.g}, ${parsed.b}, 0.14)`;
}

/** Exported so lib/appearance.ts (the rainbow engine, GlimStone form-engine
 * Phase 2 Task 1) can reuse the same hex validation + parsing rather than
 * re-deriving it a second time in the same app — rainbow's palette entries
 * and this module's single accent are the same kind of value. */
export function parseHex(hex: string): { r: number; g: number; b: number } | undefined {
  if (!/^#[0-9a-fA-F]{6}$/.test(hex)) return undefined;
  const n = parseInt(hex.slice(1), 16);
  return { r: (n >> 16) & 255, g: (n >> 8) & 255, b: n & 255 };
}

/**
 * luminance is the perceptual brightness used to decide black or white on
 * top. The sRGB channels are linearised first, because the raw values
 * overstate how bright blue is and understate green, which is exactly the
 * case that produces unreadable buttons.
 */
function luminance(r: number, g: number, b: number): number {
  const lin = (c: number) => {
    const v = c / 255;
    return v <= 0.04045 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4);
  };
  return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
}
