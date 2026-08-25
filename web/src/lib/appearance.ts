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
// This file deliberately does NOT implement shape or the single accent: the
// design language treats shape as its own separate engine from the colour
// engine anyway (see "The user-owned axes"), and it now has its own working,
// tested home in shape.ts, the same way the single accent already had one in
// accent.ts (Phase 1) before this file existed. This file adds only the
// plural half of the colour engine, reusing accent.ts's contrastOn()/
// parseHex() rather than re-deriving sRGB luminance a second time in the
// same app.
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

/**
 * KNOWN LIMITATION, documented on purpose (Phase 2 whole-branch review item
 * 3, not fixed here — same "accepted, not a bug" style as index.css's own
 * --status-warn-text KNOWN LIMITATION comment, for the sibling collision
 * that one documents): positions 0 and 3 above are byte-identical, in DARK
 * theme, to two of index.css's fixed state hues — #FF8389 (RAINBOW[0]) is
 * dark theme's --status-fail-text/--status-fail-solid; #6FDC8C (RAINBOW[3])
 * is dark theme's --status-ok-text/--status-ok-solid. A rainbow-hued row can
 * therefore coincidentally render in exactly the same colour as an unrelated
 * status chip elsewhere on the page — a healthy container washed in the same
 * red a "failed" status chip uses, say. Light theme is NOT affected the same
 * way: its own status hues (--status-fail-text #da1e28/--status-fail-solid
 * #da1e28, --status-ok-text #198038/--status-ok-solid #24a148) are Carbon
 * light-theme counterparts, not this fixed palette, so none of them match
 * RAINBOW[0]/[3] there.
 *
 * NOT a contrast/accessibility bug: every status chip still carries its own
 * text, so colour is never the only signal (SC 1.4.1 is not in play) — this
 * is a coherence gap, not a WCAG failure.
 *
 * NOT fixed by moving either position: RAINBOW is meant to be byte-identical
 * across every app sharing the GlimStone design language
 * (glimstone/reference/appearance.ts's own RAINBOW — verified identical,
 * same eight hexes in the same order). "Someone who set a colour in one app
 * finds the same colour in the next" is the whole point of a SHARED palette;
 * a BombVault-local shift here would quietly break that guarantee for this
 * one app while every other adopter's palette stayed put. Accepted per the
 * shared-palette contract, left here for whichever future task next touches
 * either the palette or the state-hue tokens.
 */

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
// intermediate component. As of Task 3 the live read path is
// hueVars(rainbowAt(index)), used by every hue-enabled Selector segment
// (components/Selector.tsx, its own default — twelve call sites across seven
// files, Settings.tsx's tab strip and drill-type toggle among them) and by
// the container/VM/file-set row cards (ContainerRow/VMRow/FileSetRow), each
// under a lib/useRainbow.ts subscription so they re-render on any change
// here.
// rainbowColor() has no call site yet: it is part of the ported contract,
// kept for a consumer that wants "undefined when the mode is off" rather than
// a colour it would have to gate itself.
// ---------------------------------------------------------------------------

let state: RainbowState = RAINBOW_OFF;
const listeners = new Set<() => void>();

// ---------------------------------------------------------------------------
// Colour-wipe (GlimStone motion-engine, animation 4) — module state private
// to applyRainbow() below, mirroring shape.ts's own armShapeTransitions()
// gate: `wipeMounted` only ever flips true→once, right after the FIRST
// applyRainbow() call completes (main.tsx's boot-time applyStoredRainbow()),
// so that call itself never triggers a wipe — there is nothing on screen yet
// for a colour to visibly "change" away from. `wipeLastAttr` remembers the
// resolved `data-rainbow` value (null/"on"/"reactive") across calls so a
// later call can tell whether this is a genuine flip or a no-op re-apply
// (e.g. setRainbow() persisting a patch that didn't actually touch on/
// reactive, or a re-render reapplying the identical state).
// ---------------------------------------------------------------------------
let wipeMounted = false;
let wipeLastAttr: string | null = null;
let wipeTimer: ReturnType<typeof setTimeout> | undefined;

/**
 * beginColourWipe adds `.bv-colour-wipe` to <html> — the class index.css's
 * own "Round 2, item 4" rule scopes its coordinated colour transition onto —
 * and clears it again after a fixed delay. A flat constant (not read back
 * out of --motion-wipe-dur) on purpose: that token can be as low as 0ms
 * ("off"), and removing the class too early would do nothing visible either
 * way (nothing is still transitioning by the time it comes off), so there is
 * no correctness reason to parse a CSS value back into JS just to compute a
 * number this constant already safely over-approximates for every
 * off/subtle/full state at once. 500ms comfortably exceeds --motion-wipe-dur's
 * own top end (320ms, "full") with margin for a slow paint. Re-entrant: a
 * second flip while the first wipe's timer is still pending clears and
 * restarts it, so a rapid on→off→on never removes the class out from under a
 * still-settling transition.
 */
function beginColourWipe(): void {
  if (wipeTimer !== undefined) clearTimeout(wipeTimer);
  const root = document.documentElement;
  root.classList.add("bv-colour-wipe");
  wipeTimer = setTimeout(() => {
    root.classList.remove("bv-colour-wipe");
    wipeTimer = undefined;
  }, 500);
}

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
  const nextAttr = merged.on ? (merged.reactive ? "reactive" : "on") : null;

  // Colour-wipe (GlimStone motion-engine, animation 4) — only for a REAL
  // flip (nextAttr !== the value this function set last time) that happens
  // AFTER the boot-time call has already completed once (wipeMounted); see
  // beginColourWipe()'s own header comment above for the class this adds
  // and why a flat timeout, not a token read-back, clears it again. Placed
  // before the attribute write below so the class is already present on
  // <html> the instant data-rainbow itself changes — a hued element's
  // --accent/--accent-soft redefinition (the [data-rainbow] .glim-hue rule)
  // and this transition need to land in the SAME style recalculation for
  // the wipe to actually catch the colour change instead of missing it by
  // one frame.
  if (wipeMounted && nextAttr !== wipeLastAttr) beginColourWipe();
  wipeLastAttr = nextAttr;
  wipeMounted = true;

  if (nextAttr === null) root.removeAttribute("data-rainbow");
  else root.setAttribute("data-rainbow", nextAttr);

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
    // The OTHER end of contrastOn()'s own binary choice — always exactly the
    // one of #161616/#FFFFFF that --item-hue-ink is not. Needed by any
    // treatment that has to shade a hued fill AWAY from its own ink instead
    // of toward it (Badge's split heading badge, see `.glim-badge-prefix` in
    // index.css and Badge.tsx's `prefix` prop). Deriving it here, from the
    // same contrastOn() call the ink itself comes from, is what keeps the
    // pair guaranteed-opposite for EVERY hue including a user's own custom
    // palette entry — a call site that tried to guess "the inverse is
    // probably white" would be wrong for every dark hue.
    "--item-hue-ink-inv": contrastOn(hex) === "#FFFFFF" ? "#161616" : "#FFFFFF",
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
