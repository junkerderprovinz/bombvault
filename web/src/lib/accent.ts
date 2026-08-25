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
  const ink = contrastOn(color);
  root.setProperty("--accent-contrast", ink);
  // The OTHER end of contrastOn()'s binary choice — always exactly the one of
  // #161616/#FFFFFF that --accent-contrast is not. Same reasoning as
  // appearance.ts's --item-hue-ink-inv (the rainbow-position counterpart): a
  // treatment that has to shade the accent fill AWAY from its own ink can't
  // hard-code the direction, because it flips with the accent's luminance —
  // a light custom accent needs to go lighter, a dark one darker. Consumed by
  // `.glim-badge-prefix` (index.css), Badge's split heading badge.
  root.setProperty("--accent-contrast-inv", ink === "#FFFFFF" ? "#161616" : "#FFFFFF");
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

// ---------------------------------------------------------------------------
// Accent presets — GlimStone follow-up pass, live-review round 6 (jdp: "Die
// Voreinstellungsfelder der Akzentfarbe sollen auch bearbeitbar sein und
// auch ein Reset-Badge bekommen. Bitte mehr Voreinstellungsfarbfelder" — the
// preset swatches should become individually editable too, get a reset, and
// there should be more of them).
//
// DEFAULT_ACCENT_PRESETS is now 8 long, not 5 — reusing the exact same 8 hex
// values as lib/appearance.ts's own RAINBOW array (just reordered: the
// original 5 keep their original order, the 3 new ones — Orange/Teal/
// Magenta — are appended in RAINBOW's own wheel order). Reusing RAINBOW's
// hexes rather than inventing new ones means no new colour math is needed,
// every one of the 8 already has a proven contrastOn() ink pairing (see
// appearance.ts's own "KNOWN LIMITATION" comment for the two that coincide
// with fixed status hues in dark theme — same accepted trade-off, now also
// reachable from here), and the Accent Card's presets and the Rainbow
// Card's palette become LITERALLY the same 8 swatches, offered through two
// different mental models (pick one vs. hand eight out by position) — the
// "visual/systemic consistency between the two colour-set UIs on the same
// page" this round asked for.
//
// This is a DELIBERATE, BombVault-local divergence from GlimStone's shared
// reference/appearance.ts, whose own ACCENTS comment is explicit that the
// preset list is "the same five across every adopting app... so someone who
// set 'Blue' in one app finds the same blue in the next" — fixed, not
// editable, exactly five. Diverging here is accepted per jdp's own
// "prototype in BombVault first" pattern (the same one the card-notch-badge
// work already followed), not a signal to backport editability or the
// widened count into the shared reference — every other GlimStone adopter
// keeps the original 5-fixed contract unless and until that's a separate,
// deliberate decision.
//
// Persistence: localStorage, mirroring appearance.ts's own
// getRainbow()/setRainbow() shape — same all-or-nothing validation
// (isValidAccentPresets, below), same "corrupt/missing storage silently
// falls back to the built-in default" contract. These are the same *kind*
// of appearance preference as the rainbow palette, just feeding the single
// accent instead of eight positions at once, so there is no reason for the
// persistence shape to differ.
// ---------------------------------------------------------------------------

export const DEFAULT_ACCENT_PRESETS: string[] = [
  "#FCC419", // Sunflower — also DEFAULT_ACCENT
  "#1D99F3", // Blue
  "#6FDC8C", // Green
  "#FF8389", // Red
  "#BE95FF", // Purple
  "#FF832B", // Orange — new, = RAINBOW[1]
  "#3DDBD9", // Teal — new, = RAINBOW[4]
  "#FF7EB6", // Magenta — new, = RAINBOW[7]
];

const PRESETS_STORAGE_KEY = "bv-accent-presets";

/** isValidAccentPresets mirrors appearance.ts's own isValidPalette(): same
 * fixed length + every entry independently parseable, checked all-or-
 * nothing so a corrupt/tampered localStorage value can never partially
 * apply (one bad entry among eight good ones falls back to the full
 * built-in default, same rule the rainbow palette already enforces). */
export function isValidAccentPresets(p: string[]): boolean {
  return p.length === DEFAULT_ACCENT_PRESETS.length && p.every((c) => parseHex(c) !== undefined);
}

function usablePresets(p: string[] | undefined): string[] {
  if (!p || !isValidAccentPresets(p)) return DEFAULT_ACCENT_PRESETS;
  return p;
}

/** getAccentPresets is the persisted preset set, defaulting to the built-in
 * 8 when nothing is stored (or storage is disabled/corrupt) — same contract
 * as getRainbow(). */
export function getAccentPresets(): string[] {
  try {
    const raw = localStorage.getItem(PRESETS_STORAGE_KEY);
    if (!raw) return DEFAULT_ACCENT_PRESETS;
    return usablePresets(JSON.parse(raw) as string[]);
  } catch {
    return DEFAULT_ACCENT_PRESETS;
  }
}

/** setAccentPresets persists the VALIDATED result — never the raw input —
 * and returns it, the same "never let a locally-clamped value diverge from
 * what's on disk" reasoning as setRainbow()'s own comment: a caller should
 * sync its component state from this return value, not a second
 * localStorage round-trip. */
export function setAccentPresets(next: string[]): string[] {
  const usable = usablePresets(next);
  try {
    localStorage.setItem(PRESETS_STORAGE_KEY, JSON.stringify(usable));
  } catch {
    // A browser with storage disabled simply pays one flash per load; the
    // in-memory value returned below still makes this call correct.
  }
  return usable;
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
