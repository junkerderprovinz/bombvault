// ---------------------------------------------------------------------------
// Rainbow — the accent, plural (GlimStone form-engine Phase 2, Task 1).
//
// Ported from GlimStone's reference/appearance.ts (see docs/design-language.md,
// "The colour engine" / "Rainbow" in the glimstone repo, and
// knightloader/web/src/lib/appearance.ts for a real shipped app's own copy).
// Colours are handed out to list items by POSITION instead of one accent
// everywhere — a set of eight, with optional "reactive" (rest neutral,
// colour on hover/active) and "rotate" (offset the starting colour) modes.
//
// This file deliberately does NOT re-implement shape or the single accent:
// BombVault has no shape axis yet (see index.css's own "[data-shape] axis...
// that's a later task" comment — the design language treats shape as its own
// separate engine from the colour engine anyway, see "The user-owned axes"),
// and the single accent already has a working, tested home in accent.ts
// (Phase 1). This file adds only the plural half, reusing accent.ts's
// contrastOn()/parseHex() rather than re-deriving sRGB luminance a second
// time in the same app.
//
// Persistence: localStorage, per-browser — the SAME mechanism every other
// appearance/UI-preference axis in this app already uses (accent.ts,
// theme.ts, i18n.ts's language pick, advanced.tsx, toast.tsx's quiet mode,
// even Containers.tsx/VMs.tsx's own sort/filter chips). BombVault has no
// concept of multiple simultaneous viewers who must agree on one colour
// (it's a single-operator Unraid backup tool, not a shared dashboard), so
// there is no reason to introduce a second persistence mechanism — a
// server-round-trip settings field — for what is otherwise exactly the same
// kind of setting as the accent right next to it in Settings.tsx.
// ---------------------------------------------------------------------------

import { contrastOn, parseHex } from "./accent";

/**
 * RAINBOW is the default palette: a full turn of the wheel, tuned to the
 * same warm, slightly dusty register as the accent presets, so switching the
 * mode on changes how much colour there is, not which family it belongs to.
 * The length is fixed — colours are handed out by position, so a palette
 * that could grow would re-colour every existing row the moment one was
 * added. Verbatim from reference/appearance.ts (also identical in
 * knightloader/web/src/lib/appearance.ts).
 */
export const RAINBOW: string[] = [
  "#FF8389", // red 30
  "#FF832B", // orange 40
  "#FCC419", // sunflower — the default accent, so one row always matches it
  "#6FDC8C", // green 30
  "#3DDBD9", // teal 30
  "#1D99F3", // blue
  "#BE95FF", // purple 30
  "#FF7EB6", // magenta 30
];

export interface RainbowState {
  on: boolean;
  /** Rest neutral, colour on hover, keep the colour on the active item. */
  reactive: boolean;
  /** Offset the palette by seed, so a run does not always start on crimson. */
  rotate: boolean;
  seed: number;
  palette: string[];
}

export const RAINBOW_OFF: RainbowState = {
  on: false,
  reactive: false,
  rotate: false,
  seed: 0,
  palette: RAINBOW,
};

// ---------------------------------------------------------------------------
// Live state
//
// Module-level, not component state: an element's rainbow position is a
// property of the document, not of any one component tree — two independent
// consumers (e.g. the Settings tab strip and a container list) must agree on
// which colour position three is, and they never meet in the same component
// tree. Readers subscribe instead of a prop threaded through every
// intermediate component. As of Task 2, rainbowAt()/rainbowColor() are read
// by the Settings tab strip and the container/VM/file-set list rows
// (ContainerRow/VMRow/FileSetRow via hueVars()), each via lib/useRainbow.ts's
// subscription so they re-render live on any change here.
// ---------------------------------------------------------------------------

let state: RainbowState = RAINBOW_OFF;
const listeners = new Set<() => void>();

/** rainbowState is the current snapshot. Stable identity between changes. */
export function rainbowState(): RainbowState {
  return state;
}

export function subscribeRainbow(fn: () => void): () => void {
  listeners.add(fn);
  return () => listeners.delete(fn);
}

/**
 * applyRainbow stores the new state, mirrors it onto the document root and
 * wakes the readers. The custom properties are set even when the mode is off
 * so a stylesheet can reference `--rb-3` without having to know; the
 * `data-rainbow` attribute is what actually turns the look on.
 *
 * SECURITY: `merged.palette = usablePalette(...)` below is the one place a
 * candidate palette is allowed to reach `document.documentElement.style` —
 * every entry is validated as `^#[0-9a-fA-F]{6}$` and the whole palette is
 * rejected ALL-OR-NOTHING (design-language.md: "seven good colours plus one
 * injected value isn't an 87%-safe palette, it's an invisible line"). There
 * is no path from a caller's raw string array to a CSS custom property that
 * skips this check.
 */
export function applyRainbow(next: Partial<RainbowState> | undefined): void {
  const merged: RainbowState = { ...RAINBOW_OFF, ...next };
  merged.palette = usablePalette(merged.palette);
  merged.seed = Number.isFinite(merged.seed) ? Math.abs(Math.trunc(merged.seed)) % RAINBOW.length : 0;
  state = merged;

  const root = document.documentElement;
  for (let i = 0; i < RAINBOW.length; i++) {
    root.style.setProperty(`--rb-${i}`, rainbowAt(i));
  }
  if (!merged.on) root.removeAttribute("data-rainbow");
  else root.setAttribute("data-rainbow", merged.reactive ? "reactive" : "on");

  for (const fn of listeners) fn();
}

/**
 * rainbowColorAt is the pure position/rotation math, split out from the
 * stateful rainbowAt() below the same way accent.ts splits contrastOn()/
 * softTint() from the DOM-touching applyAccent() — so the actual mapping
 * (position → colour, rotation/seed offset, wraparound for out-of-range i)
 * is unit-testable directly (see appearance.test.ts) without needing
 * DOM/localStorage, matching this branch's established no-jsdom pattern.
 *
 * Position, never a hash: design-language.md's first documented trap is that
 * hashing an id/name "sounds better" (a row's colour survives as rows above
 * it finish) until three rows land in the same bucket and two neighbours
 * share a colour — position is exactly what this mode exists to prevent.
 * Every caller of `i` MUST pass a stable list index, never a derived hash of
 * the item's id/name.
 */
export function rainbowColorAt(i: number, palette: string[], rotate: boolean, seed: number): string {
  const p = palette.length > 0 ? palette : RAINBOW;
  const off = rotate ? seed : 0;
  const n = ((Math.trunc(i) % p.length) + p.length) % p.length;
  const color = p[(n + off) % p.length];
  if (color === undefined) {
    // Unreachable in practice: p is never empty (falls back to RAINBOW
    // above), but the index is computed via modulo, which TS can't verify.
    throw new Error("rainbowColorAt: palette is empty");
  }
  return color;
}

/**
 * rainbowAt is the colour at a position for the CURRENT live state, rotation
 * applied. It answers even when the mode is off, because the Settings page
 * has to show the palette it is editing regardless of the master switch.
 */
export function rainbowAt(i: number): string {
  return rainbowColorAt(i, state.palette, state.rotate, state.seed);
}

/**
 * rainbowColor is what a component asks for: the colour this item should
 * use, or undefined when the mode is off and the single accent applies.
 * Returning undefined rather than the accent keeps the accent in CSS, where
 * a theme change still reaches it.
 */
export function rainbowColor(i: number): string | undefined {
  return state.on ? rainbowAt(i) : undefined;
}

/**
 * hueVars are the inline custom properties an element carrying a palette
 * position sets on itself. The matching `.glim-hue` rules in index.css
 * decide whether the hue is shown at rest or held back until hover, so a
 * component only has to say which colour it owns, never which mode is
 * active.
 *
 * The class and these properties always travel together: `.glim-hue` with no
 * `--item-hue` under it would resolve the accent to nothing — Task 2's job
 * is handing both out from one call per row.
 */
export function hueVars(hex: string | undefined): Record<string, string> {
  const parsed = hex ? parseHex(hex) : undefined;
  if (!parsed || !hex) return {};
  const { r, g, b } = parsed;
  return {
    "--item-hue": hex,
    "--item-hue-ink": contrastOn(hex),
    "--item-hue-soft": `rgba(${r}, ${g}, ${b}, 0.14)`,
    // The wash covers a whole row, so it sits far below the soft tint: at
    // 14% eight rows of eight hues stop being a list and start being a
    // colour chart.
    "--item-hue-wash": `rgba(${r}, ${g}, ${b}, 0.07)`,
    // The focus ring follows the position too. A gold ring around a teal tab
    // is the one place the single accent leaks back into the plural mode,
    // and it is the most visible one, because it only ever appears on the
    // element the keyboard is standing on.
    "--item-hue-ring": `rgba(${r}, ${g}, ${b}, 0.55)`,
  };
}

/**
 * isValidPalette is the standalone, callable form of the all-or-nothing
 * palette check applyRainbow()/usablePalette() enforce internally — exposed
 * so a caller (the Settings UI, or its tests) can validate a candidate
 * palette explicitly before ever handing it to applyRainbow/setRainbow,
 * matching this module's contract that nothing unvalidated reaches
 * document.documentElement.style.
 */
export function isValidPalette(p: string[]): boolean {
  return p.length === RAINBOW.length && p.every((c) => parseHex(c) !== undefined);
}

/** A palette is taken only in full — one bad entry among eight good ones
 * falls back to the full built-in default rather than a partially-applied
 * mix, per design-language.md's explicit all-or-nothing rule. */
function usablePalette(p: string[] | undefined): string[] {
  if (!p || !isValidPalette(p)) return RAINBOW;
  return p;
}

// ---------------------------------------------------------------------------
// Persistence — localStorage, mirroring accent.ts's own getAccent/setAccent/
// applyStoredAccent shape exactly, so the two sibling appearance settings
// read the same at a glance.
// ---------------------------------------------------------------------------

const STORAGE_KEY = "bv-rainbow";

interface StoredRainbow {
  on?: boolean;
  reactive?: boolean;
  rotate?: boolean;
  seed?: number;
  palette?: string[];
}

/** getRainbow is the persisted preference, defaulting to fully off with the
 * built-in palette when nothing is stored (or storage is disabled/corrupt). */
export function getRainbow(): RainbowState {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return RAINBOW_OFF;
    const parsed = JSON.parse(raw) as StoredRainbow;
    return {
      on: !!parsed.on,
      reactive: !!parsed.reactive,
      rotate: !!parsed.rotate,
      seed: typeof parsed.seed === "number" ? parsed.seed : 0,
      palette: usablePalette(parsed.palette),
    };
  } catch {
    return RAINBOW_OFF;
  }
}

/** setRainbow merges `patch` onto the current persisted state, applies it to
 * the document immediately (no separate "Save" step, matching accent.ts's
 * setAccent()), persists the VALIDATED result — never the raw merged input —
 * and returns the new state so a caller can sync its own component state
 * from the return value instead of a second localStorage round-trip.
 *
 * Persisting `state` (applyRainbow's own sanitized output) rather than
 * `merged` matters: applyRainbow() clamps an out-of-range seed and rejects a
 * bad palette all-or-nothing, but that validation happened on a LOCAL copy —
 * without this ordering, an invalid `merged` would still hit localStorage
 * first, and because getRainbow() re-validates on every read, the DOM would
 * keep showing the clamped value while the stored JSON quietly stayed
 * poisoned forever (each subsequent setRainbow() re-merges from that same
 * still-invalid stored value, never converging). See appearance.dom.test.tsx. */
export function setRainbow(patch: Partial<RainbowState>): RainbowState {
  const merged: RainbowState = { ...getRainbow(), ...patch };
  applyRainbow(merged);
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  } catch {
    // A browser with storage disabled simply pays one flash per load; the
    // in-memory apply above still makes this load correct.
  }
  return state;
}

/** Called at boot in main.tsx before first render (flash prevention), same
 * spot accent.ts's applyStoredAccent() and theme.ts's applyStoredTheme()
 * already run from. */
export function applyStoredRainbow(): void {
  applyRainbow(getRainbow());
}
