// ---------------------------------------------------------------------------
// Control label engine (#178) — how much of a control's identity is shown:
// its text, its glyph, or both.
//
// Three INDEPENDENT axes, because the same answer is rarely right for all
// three: the action buttons scattered through the pages, the sidebar's
// navigation rail, and the tab strips inside Settings. jdp asked for one
// selector each rather than a single global switch, since a sidebar reduced to
// glyphs is a layout decision (the rail gets narrower) while a button reduced
// to glyphs is only a density preference.
//
// Stored in localStorage and applied as attributes on <html>, exactly like
// motion.ts / shape.ts / accent.ts. No server round-trip: this is a per-viewer
// appearance preference, in a single-operator tool, and it has to be readable
// before first paint to avoid a flash of the wrong layout.
// ---------------------------------------------------------------------------

/**
 * "text"      — label only, no glyph.
 * "textGlyph" — glyph next to the label (today's look, hence the default).
 * "glyph"     — glyph only; the label survives as the accessible name and the
 *               tooltip, never as nothing (see ControlLabel in Button.tsx).
 * "reactive"  — glyph only at rest, and the words come back on hover or focus
 *               (jdp: "reaktiver Text und Symbole").
 *
 * Why "reactive" costs no layout at all, which is the whole reason it can
 * exist: every control already reserves its LABEL's width in glyph mode, so
 * the box is wide enough for the words before they arrive. Revealing them
 * moves nothing on the page — the text appears inside a box that was always
 * that size. Without the width-stage engine this mode would reflow the page
 * under the pointer, which is exactly the thing the stages exist to prevent.
 */
export type LabelMode = "text" | "textGlyph" | "glyph" | "reactive";

export const LABEL_MODES: LabelMode[] = ["text", "textGlyph", "glyph", "reactive"];

/**
 * Whether a mode hides the label from view. Both hiding modes keep the
 * accessible name and the hover bubble; they differ only in whether the words
 * themselves come back.
 *
 * A predicate rather than `mode === "glyph"` scattered around, because that
 * comparison lived at eleven call sites and every one of them would have had
 * to learn about the fourth mode independently — the kind of place a new enum
 * value gets half-adopted and nobody notices until a strip renders wrong.
 */
export function hidesLabel(mode: LabelMode): boolean {
  return mode === "glyph" || mode === "reactive";
}

/**
 * The three axes. Kept as a list rather than three copies of the same code so
 * a fourth axis is one entry, and so the settings card can iterate instead of
 * repeating itself three times.
 */
export type ControlAxis = "buttons" | "sidebar" | "tabs";

export const CONTROL_AXES: ControlAxis[] = ["buttons", "sidebar", "tabs"];

const STORAGE_KEY: Record<ControlAxis, string> = {
  buttons: "bv-labels-buttons",
  sidebar: "bv-labels-sidebar",
  tabs: "bv-labels-tabs",
};

const ATTRIBUTE: Record<ControlAxis, string> = {
  buttons: "data-labels-buttons",
  sidebar: "data-labels-sidebar",
  tabs: "data-labels-tabs",
};

/**
 * DEFAULT is "textGlyph" for every axis: that is what the app looks like today
 * (buttons with a label, sidebar rows with icon plus text, tabs with text),
 * so nobody's interface changes merely because the setting now exists. Same
 * reasoning motion.ts gives for defaulting to "full".
 */
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
    // returning null; the default is a perfectly good answer there.
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

// ---------------------------------------------------------------------------
// Width stages (#178, [200]).
//
// jdp's requirement: a button keeps the SAME width in all three modes, so
// switching mode never reflows the page. The width therefore cannot come from
// what is currently rendered (a lone glyph is narrow), it has to come from the
// LABEL — which is present in every mode, even when it is only the accessible
// name.
//
// Why stages rather than each button measuring its own text: measuring happens
// in the browser, after layout, which is both untestable and a source of
// jitter. A stage is a pure function of the label, known before the first
// paint, and it gives the tidy, aligned look jdp asked for ("sonst haben wir
// in den drei modi total viele verschieden breite buttons").
//
// Why the CURRENT language decides the stage, measured rather than assumed:
// across the 42 locales the same label grows by up to 3.4x ("Clear" becomes
// "Kijelölés törlése" in Hungarian, "Show" becomes "Megjelenítés"). Pinning
// one global stage per button would mean every English and Chinese interface
// pays for the longest translation, permanently. Deriving the stage from the
// active language keeps each language tidy on its own terms; the width changes
// when the LANGUAGE changes, which is a reload-level event, not while anyone
// is looking at a mode selector.
// ---------------------------------------------------------------------------

export type WidthStage = "xs" | "sm" | "md" | "lg";

export const WIDTH_STAGES: WidthStage[] = ["xs", "sm", "md", "lg"];

/**
 * Upper bounds in "visual units", where a CJK/fullwidth character counts as
 * two. Derived from the real distribution of the app's 80 button labels across
 * all 42 locales: 17 fall under 14 units, 28 land between 14 and 22, 12
 * between 22 and 34, and the rest above.
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

/** The stage a label belongs to. Pure, so it is testable without a DOM and
 *  gives the same answer during SSR-less first paint as it does later. */
export function widthStage(label: string): WidthStage {
  const w = labelWidth(label);
  for (const [stage, max] of STAGE_MAX) {
    if (w <= max) return stage;
  }
  return "lg";
}

/**
 * The stage a set of labels shares: the one the LONGEST of them needs.
 *
 * For BUTTONS that belong together visually but are rendered by different
 * components, so neither can see the other's label. jdp, on the Container
 * card: "die buttons jetzt sichern und Export sollen gleich breit sein" —
 * "Jetzt sichern" lands on `sm` and "Export (Plain-tar)" on `md`, and they sit
 * side by side.
 *
 * Both components compute this from the SAME two labels rather than one
 * passing a width to the other, so they agree in all 42 languages without a
 * prop threaded through the card between them, and they keep agreeing when one
 * of the two words is retranslated.
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
 * Padding for the things a stage table cannot see.
 *
 * `widthStage` measures TEXT, but a rendered button also carries a glyph, the
 * gap beside it and its own horizontal padding — about 52px, or eight units at
 * the ~7px per unit the stage bounds are calibrated to. For the ordinary case
 * that gap does not matter, because a stage is a floor and a slightly wide
 * label simply overhangs it (a known, accepted property — jdp has looked at it
 * and left it alone).
 *
 * It matters here, because a group's stage is applied as an EXACT width. Left
 * unpadded, "Diesen Ordner verwenden" lands on `md` (184px) and renders 218px,
 * so the pair it was supposed to match would be the one thing it does not do.
 *
 * Eight blanks rather than a number, so it flows through the same
 * `labelWidth`/`widthStage` pair as everything else instead of duplicating
 * their arithmetic somewhere it can drift.
 */
const GROUP_CHROME = "        ";
