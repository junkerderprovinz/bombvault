import { save as saveDisplayPrefs } from "./displayPrefs";

// The control label engine: how much of a control's identity is shown, its
// text, its glyph or both. One axis per chrome surface, because the right
// answer differs: a rail reduced to glyphs changes the layout (it gets
// narrower) while a button reduced to glyphs is only a density preference.
//
// Like motion.ts, shape.ts and accent.ts, the choice is read from localStorage
// and applied as attributes on <html> before first paint, so the layout never
// flashes in the wrong mode.

/**
 * "text": label only.
 * "textGlyph": glyph next to the label, the default.
 * "glyph": glyph only; the label stays as the accessible name and the tooltip
 *   (see ControlLabel in Button.tsx).
 * "reactive": glyph only at rest, and the words come back on hover or focus.
 *
 * Reactive costs no layout because every control already reserves its
 * label's width in glyph mode (see the width stages below), so the words
 * appear inside a box that was always that size.
 */
export type LabelMode = "text" | "textGlyph" | "glyph" | "reactive";

export const LABEL_MODES: LabelMode[] = ["text", "textGlyph", "glyph", "reactive"];

/**
 * Whether a mode hides the label from view. Both hiding modes keep the
 * accessible name and the hover bubble; they differ only in whether the words
 * themselves come back. Call sites ask this rather than comparing against
 * "glyph", so a new hiding mode reaches all of them.
 */
export function hidesLabel(mode: LabelMode): boolean {
  return mode === "glyph" || mode === "reactive";
}

/** The axes as a list, so the settings card can iterate over them. */
export type ControlAxis = "buttons" | "sidebar" | "tabs" | "bottombar";

export const CONTROL_AXES: ControlAxis[] = ["buttons", "sidebar", "tabs", "bottombar"];

const STORAGE_KEY: Record<ControlAxis, string> = {
  buttons: "bv-labels-buttons",
  sidebar: "bv-labels-sidebar",
  tabs: "bv-labels-tabs",
  bottombar: "bv-labels-bottombar",
};

const ATTRIBUTE: Record<ControlAxis, string> = {
  buttons: "data-labels-buttons",
  sidebar: "data-labels-sidebar",
  tabs: "data-labels-tabs",
  bottombar: "data-labels-bottombar",
};

/** DEFAULT is "textGlyph" on every axis: buttons with a label, sidebar rows
 *  with icon and text, tabs with text. */
const DEFAULT: LabelMode = "textGlyph";

function isLabelMode(v: unknown): v is LabelMode {
  return typeof v === "string" && (LABEL_MODES as string[]).includes(v);
}

/** The stored preference for one axis, defaulting when unset or corrupt. */
export function getLabelMode(axis: ControlAxis): LabelMode {
  let stored: string | null = null;
  try {
    stored = localStorage.getItem(STORAGE_KEY[axis]);
  } catch {
    // Private windows and blocked site data throw on access rather than
    // returning null; the default will do there.
  }
  return isLabelMode(stored) ? stored : DEFAULT;
}

/**
 * Sets the attribute index.css keys its label rules off, validating first, so
 * a caller can pass an unvalidated value (straight out of localStorage) the
 * same way applyMotionIntensity accepts one.
 */
export function applyLabelMode(axis: ControlAxis, mode: LabelMode | string | undefined): void {
  document.documentElement.setAttribute(ATTRIBUTE[axis], isLabelMode(mode) ? mode : DEFAULT);
}

/** Persists the choice and applies it immediately (no separate save step). */
export function setLabelMode(axis: ControlAxis, mode: LabelMode): void {
  try {
    localStorage.setItem(STORAGE_KEY[axis], mode);
    saveDisplayPrefs();
  } catch {
    // Not being able to remember the choice is not a reason to refuse it for
    // this session.
  }
  applyLabelMode(axis, mode);
}

/** Called at boot in main.tsx before first render, so the layout never flashes
 *  in one mode and settles into another. */
export function applyStoredLabelModes(): void {
  for (const axis of CONTROL_AXES) applyLabelMode(axis, getLabelMode(axis));
}

// Width stages. A button keeps the same width in every label mode, so
// switching modes never reflows the page. The width therefore comes from the
// label, which exists in every mode (at least as the accessible name), rather
// than from what is rendered. A stage is a pure function of the label, known
// before first paint and testable, where measuring the rendered text would
// happen after layout and jitter; it also lines buttons up.
//
// The stage follows the current language. Across the 42 locales a label grows
// by up to 3.4x ("Clear" is "Kijelölés törlése" in Hungarian), and one global
// stage per button would make every language pay for the longest translation.
// The width changes only when the language does.

export type WidthStage = "xs" | "sm" | "md" | "lg";

export const WIDTH_STAGES: WidthStage[] = ["xs", "sm", "md", "lg"];

/**
 * Upper bounds in visual units, where a CJK or fullwidth character counts as
 * two, taken from how the app's button labels are distributed across all 42
 * locales.
 */
const STAGE_MAX: [WidthStage, number][] = [
  ["xs", 10],
  ["sm", 16],
  ["md", 26],
  ["lg", Infinity],
];

/** Visual width of a label: CJK and other fullwidth characters count double,
 *  since they occupy roughly two Latin character cells. */
export function labelWidth(label: string): number {
  let total = 0;
  for (const ch of label) {
    const code = ch.codePointAt(0) ?? 0;
    const fullwidth =
      (code >= 0x1100 && code <= 0x115f) ||
      (code >= 0x2e80 && code <= 0xa4cf) ||
      (code >= 0xac00 && code <= 0xd7a3) ||
      (code >= 0xf900 && code <= 0xfaff) ||
      (code >= 0xfe30 && code <= 0xfe6f) ||
      (code >= 0xff00 && code <= 0xff60) ||
      (code >= 0xffe0 && code <= 0xffe6);
    total += fullwidth ? 2 : 1;
  }
  return total;
}

/** The stage a label belongs to. */
export function widthStage(label: string): WidthStage {
  const w = labelWidth(label);
  for (const [stage, max] of STAGE_MAX) {
    if (w <= max) return stage;
  }
  return "lg";
}

/**
 * groupStage is the stage the longest of `labels` needs, for buttons that sit
 * side by side but are rendered by different components, such as "Jetzt
 * sichern" (sm) and "Export (Plain-tar)" (md) on the container card. Each
 * component computes it from the same labels, so they agree in every language
 * without a width passed between them.
 */
export function groupStage(labels: string[]): WidthStage {
  let widest: WidthStage = "xs";
  for (const label of labels) {
    const stage = widthStage(label + GROUP_CHROME);
    if (WIDTH_STAGES.indexOf(stage) > WIDTH_STAGES.indexOf(widest)) widest = stage;
  }
  return widest;
}

/**
 * The glyph, gap and padding a rendered button adds to its text: about 52px,
 * or eight units at the ~7px per unit the stages are calibrated to. A single
 * button treats its stage as a floor and may overhang it, but a group's stage
 * is applied as an exact width, and without this "Diesen Ordner verwenden"
 * lands on md (184px) while rendering 218px wide. Eight blanks rather than a
 * number, so the padding goes through labelWidth like any other text.
 */
const GROUP_CHROME = "        ";
