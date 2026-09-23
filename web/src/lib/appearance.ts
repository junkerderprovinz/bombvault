// Rainbow: the accent, plural. Ported from reference/appearance.ts in the
// glimstone repo (docs/design-language.md there, "The colour engine"). List
// items get colours by position from a set of eight instead of one accent
// everywhere, with an optional "reactive" mode (neutral at rest, colour on
// hover and on the active item) and "rotate" (offset the starting colour). The
// single accent lives in accent.ts and shape in shape.ts; this file reuses
// accent.ts's contrastOn and parseHex.
//
// Like the other appearance settings it is read from localStorage before first
// paint, and displayPrefs.ts keeps that in step with the server.

import { contrastOn, parseHex } from "./accent";
import { save as saveDisplayPrefs } from "./displayPrefs";

/**
 * RAINBOW is the default palette: a full turn of the wheel, tuned to the
 * same warm, slightly dusty register as the accent presets, so switching the
 * mode on changes how much colour there is, not which family it belongs to.
 * The length is fixed: colours are handed out by position, so a palette that
 * could grow would re-colour every existing row the moment one was added. It
 * matches GlimStone's reference palette entry for entry.
 *
 * Entries 0 and 3 equal the dark theme's fail and ok status hues in index.css,
 * so a hued row can share a colour with an unrelated status chip. Every chip
 * carries text, so colour is never the only signal, and the palette stays
 * shared across the GlimStone apps rather than shifting in this one: someone
 * who picked a colour in one app should find it again in the next.
 */
export const RAINBOW: string[] = [
  "#FF8389", // red 30
  "#FF832B", // orange 40
  "#FCC419", // sunflower, the default accent, so one row always matches it
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

// Module state rather than component state: the palette lives on the document
// root, where every hued element reads it through hueVars(), and it changes
// from outside any component tree (Settings, disco, the server's stored look).
let state: RainbowState = RAINBOW_OFF;

// Colour-wipe state for applyRainbow. The first apply is the boot call, with
// nothing on screen yet to change away from, so wipeMounted keeps it quiet.
// wipeLastAttr is the data-rainbow value set last, so a re-apply that changes
// nothing does not wipe.
let wipeMounted = false;
let wipeLastAttr: string | null = null;
let wipeTimer: ReturnType<typeof setTimeout> | undefined;

/** `animate: false` applies the value without the colour wipe. For a change
 *  nobody made: the stored look arriving from the server after paint. */
export type ApplyOpts = { animate?: boolean };

/**
 * beginColourWipe adds `.glim-colour-wipe` to <html>, which index.css turns
 * into a coordinated colour transition, and removes it after 500ms. That is a
 * constant rather than a read of --motion-wipe-dur because the token tops out
 * at 320ms and removing the class a little late costs nothing. A second flip
 * restarts the timer, so a quick on, off, on never pulls the class from under
 * a transition that is still settling.
 */
function beginColourWipe(): void {
  if (wipeTimer !== undefined) clearTimeout(wipeTimer);
  const root = document.documentElement;
  root.classList.add("glim-colour-wipe");
  wipeTimer = setTimeout(() => {
    root.classList.remove("glim-colour-wipe");
    wipeTimer = undefined;
  }, 500);
}

/** rainbowState is the current snapshot. */
export function rainbowState(): RainbowState {
  return state;
}

/**
 * applyRainbow stores the new state and mirrors it onto the document root.
 * The `--rb-*` properties are set even when the mode is off, because
 * hueVars() points every hued element at them; the `data-rainbow` attribute is
 * what turns the look on.
 *
 * usablePalette is the only way a palette reaches
 * document.documentElement.style: every entry has to be a six-digit hex, and
 * one bad entry rejects the whole palette, because seven good colours and one
 * injected value are not a mostly safe palette.
 */
export function applyRainbow(next: Partial<RainbowState> | undefined, opts?: ApplyOpts): void {
  const merged: RainbowState = { ...RAINBOW_OFF, ...next };
  merged.palette = usablePalette(merged.palette);
  merged.seed = Number.isFinite(merged.seed) ? Math.abs(Math.trunc(merged.seed)) % RAINBOW.length : 0;
  state = merged;

  const root = document.documentElement;
  for (let i = 0; i < RAINBOW.length; i++) {
    root.style.setProperty(`--rb-${i}`, rainbowAt(i));
    root.style.setProperty(`--rb-ink-${i}`, contrastOn(rainbowAt(i)));
  }
  const nextAttr = merged.on ? (merged.reactive ? "reactive" : "on") : null;

  // Wipe only on a real flip after the boot apply, and add the class before
  // the attribute changes so the transition and the new colours land in the
  // same style recalculation. `animate: false` covers the other apply nobody
  // made, the server's stored look arriving after paint, which would
  // otherwise walk every hued element from the accent to its hue and back.
  const animate = opts?.animate !== false;
  if (animate && wipeMounted && nextAttr !== wipeLastAttr) beginColourWipe();
  wipeLastAttr = nextAttr;
  wipeMounted = true;

  if (nextAttr === null) root.removeAttribute("data-rainbow");
  else root.setAttribute("data-rainbow", nextAttr);
}

/**
 * rainbowColorAt is the pure position and rotation math behind rainbowAt, so
 * it can be tested without a DOM. `i` has to be a stable list index, never a
 * hash of the item's id or name: hashing puts neighbours in the same bucket,
 * and two adjacent rows sharing a colour is what this mode exists to prevent.
 */
export function rainbowColorAt(i: number, palette: string[], rotate: boolean, seed: number): string {
  const p = palette.length > 0 ? palette : RAINBOW;
  const off = rotate ? seed : 0;
  const n = ((Math.trunc(i) % p.length) + p.length) % p.length;
  const color = p[(n + off) % p.length];
  if (color === undefined) {
    // Unreachable for the non-negative integer seeds applyRainbow stores;
    // TypeScript cannot see that.
    throw new Error("rainbowColorAt: palette is empty");
  }
  return color;
}

/**
 * rainbowAt is the colour at a position for the current state, rotation
 * applied, which applyRainbow writes to the root. A component uses hueVars()
 * instead: nothing re-renders when the palette moves, so a colour read during
 * render would stay where it was.
 */
export function rainbowAt(i: number): string {
  return rainbowColorAt(i, state.palette, state.rotate, state.seed);
}

/**
 * hueVars are the inline custom properties an element carrying palette
 * position `i` sets on itself. The matching `.glim-hue` rules in index.css
 * decide whether the hue is shown at rest or held back until hover, so a
 * component only has to say which colour it owns, never which mode is
 * active. The class and these properties travel together: `.glim-hue`
 * without `--item-hue` resolves the accent to nothing.
 *
 * They point at the root's `--rb-*` properties rather than holding the
 * colour, so a palette or rotation change lands on the root alone, and disco
 * can walk the colours there without re-rendering a single component.
 */
export function hueVars(i: number): Record<string, string> {
  const n = ((Math.trunc(i) % RAINBOW.length) + RAINBOW.length) % RAINBOW.length;
  const hue = `var(--rb-${n})`;
  return {
    "--item-hue": hue,
    "--item-hue-ink": `var(--rb-ink-${n})`,
    "--item-hue-soft": `color-mix(in srgb, ${hue} 14%, transparent)`,
    // The wash covers a whole row, so it sits far below the soft tint: at
    // 14% eight rows of eight hues stop being a list and start being a
    // colour chart.
    "--item-hue-wash": `color-mix(in srgb, ${hue} 7%, transparent)`,
    // The focus ring follows the position too. A gold ring around a teal tab
    // is the one place the single accent leaks back into the plural mode,
    // and it is the most visible one, because it only ever appears on the
    // element the keyboard is standing on.
    "--item-hue-ring": `color-mix(in srgb, ${hue} 55%, transparent)`,
  };
}

/**
 * isValidPalette is the all-or-nothing check applyRainbow applies, exported
 * so the Settings UI can validate a candidate before handing it over.
 */
export function isValidPalette(p: string[]): boolean {
  return p.length === RAINBOW.length && p.every((c) => parseHex(c) !== undefined);
}

/** A palette is taken whole or not at all: one bad entry falls back to the
 *  built-in default rather than a partly applied mix. */
function usablePalette(p: string[] | undefined): string[] {
  if (!p || !isValidPalette(p)) return RAINBOW;
  return p;
}

const STORAGE_KEY = "bv-rainbow";

interface StoredRainbow {
  on?: boolean;
  reactive?: boolean;
  rotate?: boolean;
  seed?: number;
  palette?: string[];
}

/** getRainbow is the persisted preference, defaulting to fully off with the
 *  built-in palette when nothing is stored or storage is disabled or corrupt. */
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

/**
 * setRainbow merges `patch` onto the persisted state, applies it right away
 * and returns the new state. It stores `state`, the value applyRainbow
 * validated, not `merged`: getRainbow validates on every read, so an invalid
 * raw value in storage would never show, but it would never go away either,
 * since each later call merges onto it again.
 */
export function setRainbow(patch: Partial<RainbowState>): RainbowState {
  const merged: RainbowState = { ...getRainbow(), ...patch };
  applyRainbow(merged);
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
    saveDisplayPrefs();
  } catch {
    // With storage disabled the next load flashes once; this one is already
    // applied.
  }
  return state;
}

/**
 * applyStoredRainbow runs at boot in main.tsx before first render, and again
 * when the server hands this browser a different stored look. That second
 * call passes `{ animate: false }`, because adopting a look is not a mode
 * flip.
 */
export function applyStoredRainbow(opts?: ApplyOpts): void {
  applyRainbow(getRainbow(), opts);
}
