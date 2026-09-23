// The shape setting: round, soft or square corners via data-shape on <html>,
// persisted in localStorage. index.css keys the radius tokens off the
// attribute; this file picks the value and stamps it, following theme.ts's
// getTheme/setTheme/applyStoredTheme pattern.
import { save as saveDisplayPrefs } from "./displayPrefs";

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
 * applyShape sets the attribute the radius tokens in index.css key off, falling
 * back to "round" for anything outside SHAPES, so a caller can pass an
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
