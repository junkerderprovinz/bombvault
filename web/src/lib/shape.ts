// The shape setting: round, soft or square corners via data-shape on <html>,
// persisted in localStorage. index.css keys the radius tokens off the
// attribute; this file picks the value and stamps it, following theme.ts's
// getTheme/setTheme/applyStoredTheme pattern.
import { save as saveDisplayPrefs } from "./displayPrefs";

export type Shape = "round" | "soft" | "square" | "leaf";

/** SHAPES is what the picker offers. The hidden "leaf" is left out, but
 *  isShape accepts it as a stored value. */
export const SHAPES: Shape[] = ["round", "soft", "square"];

/** Every shape, including the one no picker lists. Validation reads this. */
const ALL_SHAPES: Shape[] = [...SHAPES, "leaf"];

/** How many clicks on an already chosen "square" open the leaf. */
export const LEAF_CLICKS = 5;

/**
 * leafTap counts clicks on "square" while it is already chosen and returns
 * "leaf" on the fifth, otherwise undefined. Any other click resets the count.
 * The caller keeps the count and the found flag in the settings screen's state
 * rather than in storage, so the option is offered only while it is chosen or
 * for as long as that screen stays open.
 */
export function leafTap(state: { taps: number }, clicked: string, current: Shape): Shape | undefined {
  if (clicked !== "square" || current !== "square") {
    state.taps = 0;
    return undefined;
  }
  state.taps += 1;
  if (state.taps < LEAF_CLICKS) return undefined;
  state.taps = 0;
  return "leaf";
}

const STORAGE_KEY = "bv-shape";

/** A stored choice keeps its shape; only a browser with nothing stored starts
 *  on "soft". */
const DEFAULT: Shape = "soft";

// Checks against ALL_SHAPES so a stored "leaf" survives the next reload.
function isShape(v: unknown): v is Shape {
  return typeof v === "string" && (ALL_SHAPES as string[]).includes(v);
}

/** The stored preference, defaulting to "soft" when unset or corrupt. */
export function getShape(): Shape {
  const stored = localStorage.getItem(STORAGE_KEY);
  return isShape(stored) ? stored : DEFAULT;
}

/**
 * applyShape sets the attribute the radius tokens in index.css key off, falling
 * back to "soft" for anything that is not a shape, so a caller can pass an
 * unvalidated value straight from localStorage or an imported settings file.
 */
export function applyShape(shape: Shape | string | undefined): void {
  const s = isShape(shape) ? shape : DEFAULT;
  document.documentElement.setAttribute("data-shape", s);
}

/** setShape persists the choice and applies it at once, with no separate save
 * step. */
export function setShape(shape: Shape): void {
  localStorage.setItem(STORAGE_KEY, shape);
  saveDisplayPrefs();
  applyShape(shape);
}

/** Called in main.tsx before the first render, so the first paint already has
 * the right corners. */
export function applyStoredShape(): void {
  applyShape(getShape());
}

/**
 * armShapeTransitions adds `.glim-shape-transitions` to <html>, which index.css
 * uses to animate border-radius. The class is never removed, since every shape
 * change after boot is a live one.
 *
 * main.tsx calls it two animation frames after the first render. The first
 * paint already shows the right radius, but waiting until one full paint has
 * happened without the class keeps a slow first paint or a double-invoked
 * render from animating the initial shape.
 */
export function armShapeTransitions(): void {
  document.documentElement.classList.add("glim-shape-transitions");
}
