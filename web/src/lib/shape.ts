import { save as saveDisplayPrefs } from "./displayPrefs";
// ---------------------------------------------------------------------------
// Shape — round / soft / square via data-shape on <html> + localStorage.
//
// GlimStone form-engine — the shape engine (design-language.md: "Shape sets
// data-shape on <html>: round (16/10px) · soft (8/5px) · square (0). One
// token drives every radius, no exception list"). This is the piece BOTH
// prior GlimStone integration phases in this app deliberately deferred:
// index.css already carries the --radius-card/--radius-control/--radius-pill
// tokens AND (as of this same change) the [data-shape="soft"]/
// [data-shape="square"] selectors that key off them — what was missing was
// this file, the JS half that actually reads/writes/persists which of the
// three is chosen and stamps the attribute those CSS rules match against.
//
// Ported from glimstone/reference/appearance.ts's own Shape/SHAPES/
// applyShape (identical validate-or-fall-back-to-"round" contract), with the
// persistence half (getShape/setShape/applyStoredShape) following
// theme.ts's own getTheme/setTheme/applyStoredTheme shape exactly — shape is
// the same kind of setting theme.ts already models: a small fixed enum
// painted onto <html> via one attribute, not a colour like accent.ts/
// appearance.ts (rainbow). Same reasoning as those two for staying
// localStorage-only rather than a server round-trip: BombVault is a
// single-operator tool, so there's no second viewer who needs to agree on
// which corner radius is "current".
// ---------------------------------------------------------------------------

export type Shape = "round" | "soft" | "square";

export const SHAPES: Shape[] = ["round", "soft", "square"];

const STORAGE_KEY = "bv-shape";
const DEFAULT: Shape = "round";

function isShape(v: unknown): v is Shape {
  return typeof v === "string" && (SHAPES as string[]).includes(v);
}

/** The stored preference, defaulting to "round" when unset or corrupt. */
export function getShape(): Shape {
  const stored = localStorage.getItem(STORAGE_KEY);
  return isShape(stored) ? stored : DEFAULT;
}

/**
 * applyShape sets the attribute the radius tokens in index.css key off,
 * validating against SHAPES and falling back to "round" for anything else —
 * matches glimstone/reference/appearance.ts's own applyShape() exactly, so a
 * caller can hand this an unvalidated value (e.g. straight out of
 * localStorage or an imported settings file) without checking it first.
 */
export function applyShape(shape: Shape | string | undefined): void {
  const s = isShape(shape) ? shape : DEFAULT;
  document.documentElement.setAttribute("data-shape", s);
}

/** setShape persists the choice and applies it immediately (no separate
 * "Save" step, matching accent.ts's setAccent()/theme.ts's setTheme()). */
export function setShape(shape: Shape): void {
  localStorage.setItem(STORAGE_KEY, shape);
  saveDisplayPrefs();
  applyShape(shape);
}

/** Called at boot in main.tsx before first render (flash prevention), same
 * spot accent.ts's applyStoredAccent(), theme.ts's applyStoredTheme() and
 * appearance.ts's applyStoredRainbow() already run from. */
export function applyStoredShape(): void {
  applyShape(getShape());
}

/**
 * armShapeTransitions — GlimStone motion-engine, animation 1 (shape-morph).
 * Adds `.bv-shape-transitions` to <html>, the class index.css's own
 * "Round 2, item 1" rule scopes its `transition: border-radius` onto — see
 * that rule's own comment for the full contract. Never removed once added:
 * every render after main.tsx calls this is by definition a LIVE change, the
 * exact case this transition exists for.
 *
 * Called from main.tsx via a DOUBLE requestAnimationFrame after the initial
 * render call, not a plain setTimeout(0) or a single rAF: applyStoredShape()
 * already stamps the correct data-shape attribute on <html> before React's
 * very first render, so the first frame ANY element ever paints already
 * shows the right radius — there is no "wrong then corrected" flash to
 * avoid. This still waits two frames rather than arming immediately for a
 * different reason: it is the difference between "this class is added
 * before or after the browser has committed that first, already-correct
 * paint" that determines whether a hypothetical same-tick re-render (React
 * 18 Strict Mode's dev-only double-invoke, a fast-refresh remount, or simply
 * a slow first paint on a loaded machine) could still land INSIDE the same
 * paint the class arrives in — one rAF fires before the NEXT paint, so a
 * second one guarantees at least one full paint has already happened with
 * the class absent before it is ever added. Cheap (two no-op callbacks,
 * once, at boot) insurance against a flash that the reasoning above already
 * argues cannot happen, kept anyway because "cannot happen" is exactly the
 * kind of claim a live browser is free to disagree with.
 */
export function armShapeTransitions(): void {
  document.documentElement.classList.add("bv-shape-transitions");
}
