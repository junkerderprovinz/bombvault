import type { CSSProperties, ReactNode, Ref } from "react";
import { hueVars } from "../lib/appearance";
import { hidesLabel, labelWidth, widthStage, type WidthStage } from "../lib/controls";
import { mergeRefs } from "../lib/mergeRefs";
import { useLabelMode } from "../lib/useLabelMode";
import { useTipBubble } from "../lib/useTipBubble";
import { glyphFor } from "./glyphFor";
import { IconClose } from "./navGlyphs";

// Button is the app's one clickable control; something you only read is a
// Badge. The app-wide "buttons" label mode decides whether it shows text, text
// with a glyph, or the glyph alone. A hidden label stays in the accessibility
// tree and becomes the tooltip, shown in the app's .glim-bubble because
// lint-rules/icon-badge-needs-tooltip.js forbids a native title on icon-only
// controls.

const STAGE_CLASS: Record<WidthStage, string> = {
  xs: "glim-btn-xs",
  sm: "glim-btn-sm",
  md: "glim-btn-md",
  lg: "glim-btn-lg",
};

/**
 * A button's surface. A control that needs a new one gets a tone here: a
 * background set in `className` next to `tone` wins or loses by stylesheet
 * order, so the class list reads right and the button paints wrong.
 */
export type ButtonTone = "accent" | "neutral" | "subtle" | "danger" | "warn";

/**
 * "chip" is the remove control inside a pill (a selected path, a stop-hook
 * name, an exclusion line, the day filter). It shows its glyph in every mode,
 * because it has to fit inside the pill and its text would repeat what the pill
 * already says. The label remains its accessible name and tooltip.
 *
 * "icon" is a small row action such as copy, reset or delete. It follows the
 * label mode like any other button, but becomes a square tile in glyph mode,
 * where an ordinary button hugs its glyph and a row of them comes out as
 * lozenges.
 */
export type ButtonVariant = "default" | "chip" | "icon";

// danger and warn put carbon-background ink on the solid status tokens: in both
// themes the solid fail/warn values sit at the opposite lightness to the
// background, so one ink reads on both.
const TONE_CLASS: Record<ButtonTone, string> = {
  accent: "bg-accent text-accentContrast hover:opacity-90",
  neutral: "bg-carbon-surface3 text-carbon-text hover:bg-carbon-hoverRaised",
  subtle: "bg-carbon-surface2 text-carbon-text hover:bg-carbon-surface3",
  danger: "bg-statusFailSolid text-carbon-background hover:opacity-90",
  warn: "bg-statusWarnSolid text-carbon-background hover:opacity-90",
};

export function Button({
  label,
  labelKey,
  glyph,
  onClick,
  tone = "neutral",
  variant = "default",
  disabled = false,
  type = "button",
  className = "",
  title,
  hueIndex,
  busy = false,
  autoFocus = false,
  ariaExpanded,
  ariaControls,
  stage: stageOverride,
  keepLabel = false,
  ref,
}: {
  /** The button's words, present in every mode: visible, or hidden but
   *  announced and used as the tooltip. Keep it stable, since a label that
   *  changes while the button works would resize it mid-action; changing state
   *  belongs in `title`. */
  label: string;
  /** The translation key behind `label`, from which glyphFor picks a glyph by
   *  meaning, so the same verb gets the same symbol everywhere. `null` when the
   *  label is data such as a directory name. Required so that no call site
   *  leaves it out by accident. When the label switches between two wordings,
   *  switch the key with it. */
  labelKey: string | null;
  /** Overrides the glyph `labelKey` would choose. Without either, glyph mode
   *  shows the text. */
  glyph?: ReactNode;
  onClick?: () => void;
  tone?: ButtonTone;
  variant?: ButtonVariant;
  disabled?: boolean;
  type?: "button" | "submit";
  className?: string;
  /** Extra explanation, and the place for anything that changes: the running
   *  state, or why the button is unavailable. Shown in the app's hover/focus
   *  bubble rather than as a native `title`, and joined to the label in glyph
   *  mode. */
  title?: string;
  /** This button's own rainbow position, as on Badge. Rarely needed: a Card
   *  with a `hueIndex` rebinds `--accent` for its whole subtree, so the buttons
   *  inside it already paint in its colour. Pass it only for a colour that
   *  differs from the card's, such as a list row that owns its position. */
  hueIndex?: number;
  /** Forwarded to the <button>. Dialogs use it to focus a control on open. */
  ref?: Ref<HTMLButtonElement>;
  /** Shows a spinner in place of the glyph. Independent of `disabled`. */
  busy?: boolean;
  autoFocus?: boolean;
  /** For a disclosure: whether the panel it opens is showing. */
  ariaExpanded?: boolean;
  /** The id of the panel a disclosure opens. */
  ariaControls?: string;
  /** Forces a width stage instead of deriving one from the label, for two
   *  buttons that must match but live in different components. Pass
   *  `groupStage([labelA, labelB])` to both so they agree in every language.
   *  Ignored in glyph and reactive mode, which take no stage. */
  stage?: WidthStage;
  /** Shows the text in every mode. Only for a label that is content rather
   *  than an action, such as a directory row in the folder browser, where a
   *  column of identical folder glyphs would be unreadable. */
  keepLabel?: boolean;
}) {
  const mode = useLabelMode("buttons");
  const chip = variant === "chip";
  const iconOnly = variant === "icon";
  // An explicit glyph wins over the key's. A chip always removes something, so
  // it falls back to the close glyph.
  const resolved = glyph ?? (labelKey ? glyphFor(labelKey) : undefined) ?? (chip ? <IconClose /> : undefined);
  const hasGlyph = !!resolved || busy;
  // A button without a glyph shows its text in the hiding modes, because a row
  // of blank squares is worse than an uneven strip. keepLabel keeps the text
  // without dropping the glyph.
  const effective = chip
    ? "glyph"
    : keepLabel
      ? hasGlyph
        ? "textGlyph"
        : "text"
      : hidesLabel(mode) && !hasGlyph
        ? "text"
        : mode;
  const reactive = effective === "reactive";
  // Reactive words appear on hover, a CSS state, so at rest it is a hiding mode.
  const showText = effective !== "glyph" && !reactive;
  const showGlyph = effective !== "text" && hasGlyph;

  // The stage comes from the label in the current language, not from what is
  // rendered, so switching between text and text-with-glyph never reflows.
  // Glyph and reactive buttons take no stage: they hug the glyph, and a
  // reactive one grows as its words arrive.
  const stage = chip
    ? "glim-btn-chip"
    : effective === "glyph"
      ? iconOnly
        ? "glim-btn-icon"
        : ""
      : reactive
        ? ""
        : STAGE_CLASS[stageOverride ?? widthStage(label)];

  // In glyph mode the label is the tooltip, joined with `title` unless the two
  // are the same words. Reactive counts as showing text here, because hovering
  // reveals the words and a bubble would cover them.
  const tip =
    (showText || reactive
      ? title
      : [...new Set([label, title].filter(Boolean))].join(" — ")) || undefined;
  // A disabled reactive button gets no hover, so its label never reveals and it
  // keeps the bubble to say why it is unavailable.
  const tooltip = useTipBubble(reactive && !disabled ? undefined : tip, disabled);

  const hueOn = hueIndex !== undefined;
  const hueStyle = {
    ...(hueOn ? (hueVars(hueIndex) as CSSProperties) : {}),
    // Caps the reveal width; see `.glim-label-reactive` in index.css.
    ...(reactive ? ({ "--reactive-chars": labelWidth(label) } as CSSProperties) : {}),
    // An explicit stage promises that two buttons match, so it sets a real
    // width. A derived stage is only a floor that a long label may overhang.
    ...(stageOverride && stage ? ({ width: `var(--btn-w-${stageOverride})` } as CSSProperties) : {}),
  };

  return (
    <>
      {/* `wrap` only acts on a disabled button with a tip: a disabled <button>
          gets no mouse events or focus, so a box around it has to receive the
          hover instead. */}
      {tooltip.wrap(
        <button
          ref={mergeRefs(ref, tooltip.ref)}
          type={type}
          onClick={onClick}
          disabled={disabled}
          autoFocus={autoFocus}
          aria-expanded={ariaExpanded}
          aria-controls={ariaControls}
          aria-describedby={tooltip.describedBy}
          {...tooltip.handlers}
          style={Object.keys(hueStyle).length ? hueStyle : undefined}
          className={`glim-btn ${stage} ${chip ? "" : TONE_CLASS[tone]}${hueOn ? " glim-hue" : ""}${reactive ? " glim-reactive" : ""} ${className}`.trim()}
        >
          {showGlyph && (
            <span className="glim-btn-glyph">
              {busy ? (
                <span
                  className="h-3.5 w-3.5 rounded-full border-2 border-t-transparent animate-spin inline-block"
                  style={{ borderColor: "currentColor", borderTopColor: "transparent" }}
                />
              ) : (
                resolved
              )}
            </span>
          )}
          {/* Hidden rather than removed, so the button keeps its accessible
              name. Reactive mode collapses the box and opens it on hover. */}
          <span className={showText ? "glim-btn-label" : reactive ? "glim-label-reactive" : "sr-only"}>
            {label}
          </span>
        </button>,
      )}
      {tooltip.bubble}
    </>
  );
}
