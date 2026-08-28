import type { CSSProperties, ReactNode } from "react";
import { hueVars, rainbowAt } from "../lib/appearance";
import { widthStage, type WidthStage } from "../lib/controls";
import { useLabelMode } from "../lib/useLabelMode";
import { glyphFor } from "./glyphFor";

// ---------------------------------------------------------------------------
// Button — the one clickable control (#178).
//
// Naming, jdp's own (2026-08-28): something you can CLICK is a button,
// something you only read is a badge. That is also what HTML and assistive
// technology require, so the split is not cosmetic: a clickable thing has to
// be a real <button> or it cannot be reached by keyboard.
//
// This component owns four things a call site should never repeat:
//
//   1. WHAT IS SHOWN — text, text with glyph, or glyph alone, from the shared
//      "buttons" axis. One setting for the whole app, not a prop per site.
//   2. HOW WIDE IT IS — a stage derived from the LABEL, so the width is the
//      same in all three modes and switching mode never reflows the page.
//      This is jdp's explicit requirement; the width therefore cannot come
//      from what is currently visible, because a lone glyph is narrow.
//   3. THAT IT STAYS READABLE — in glyph mode the label is visually hidden but
//      still in the accessible tree (`sr-only`), and it becomes the tooltip.
//      Nothing here can produce a button whose purpose is unlabelled.
//   4. ITS COLOUR-ENGINE POSITION — `hueIndex` works exactly as it does on
//      Badge, because rainbow mode is a standing, app-wide rule: a control
//      that opts out of it is a bug, not a simplification.
//
// A glyph is optional for now. Only 17 of the app's 163 buttons carry one
// today, and jdp wants all of them to get one; until a given button has its
// glyph, glyph mode falls back to showing that button's text, which is the one
// failure mode worth having (a slightly inconsistent strip beats a row of
// blank squares nobody can identify).
// ---------------------------------------------------------------------------

const STAGE_CLASS: Record<WidthStage, string> = {
  xs: "bv-btn-xs",
  sm: "bv-btn-sm",
  md: "bv-btn-md",
  lg: "bv-btn-lg",
};

export type ButtonTone = "accent" | "neutral" | "danger";

const TONE_CLASS: Record<ButtonTone, string> = {
  accent: "bg-accent text-accentContrast hover:opacity-90",
  neutral: "bg-carbon-surface3 text-carbon-text hover:bg-carbon-hover",
  danger: "bg-statusFail text-white hover:opacity-90",
};

export function Button({
  label,
  labelKey,
  glyph,
  onClick,
  tone = "neutral",
  disabled = false,
  type = "button",
  className = "",
  title,
  hueIndex,
  busy = false,
}: {
  /** The button's words. Always present, in every mode: visible as text, or
   *  hidden but still announced and shown as the tooltip in glyph mode.
   *
   *  It must be STABLE. A label that changes while the button works ("Backing
   *  up…") would resize the control mid-action, which is exactly what the
   *  width stages exist to prevent — pass that through `title` instead. */
  label: string;
  /** The translation key behind `label`. Passing it lets the component pick a
   *  glyph by MEANING (glyphFor), so a call site does not have to choose one,
   *  and so the same verb wears the same symbol everywhere. An explicit
   *  `glyph` always wins over it. */
  labelKey?: string;
  /** Optional icon, overriding whatever `labelKey` would have chosen. Without
   *  either, glyph mode shows this button's text instead of an empty box. */
  glyph?: ReactNode;
  onClick?: () => void;
  tone?: ButtonTone;
  disabled?: boolean;
  type?: "button" | "submit";
  className?: string;
  /** Extra explanation, and the place for anything that CHANGES: the running
   *  state, why the button is unavailable right now, which other job is
   *  holding it. The label is used as the tooltip in glyph mode on its own,
   *  and both are shown when both exist. */
  title?: string;
  /** This button's rainbow position, same meaning as Badge's own prop. */
  hueIndex?: number;
  /** Shows a spinner in place of the glyph. Deliberately separate from
   *  `disabled`: a busy button is usually disabled too, but the two are not
   *  the same thing and a caller may want one without the other. */
  busy?: boolean;
}) {
  const mode = useLabelMode("buttons");
  // An explicit glyph wins; otherwise the key decides, so the same verb wears
  // the same symbol app-wide without 163 call sites each making a choice.
  const resolved = glyph ?? (labelKey ? glyphFor(labelKey) : undefined);
  // No glyph to show means text, whatever the mode says — see the header note.
  const hasGlyph = !!resolved || busy;
  const effective = mode === "glyph" && !hasGlyph ? "text" : mode;
  const showText = effective !== "glyph";
  const showGlyph = effective !== "text" && hasGlyph;

  // The stage comes from the label in the CURRENT language, never from what is
  // rendered: the width has to survive a mode change untouched.
  const stage = STAGE_CLASS[widthStage(label)];

  // In glyph mode the label IS the tooltip, so the control still explains
  // itself on hover. When both exist they are joined rather than one replacing
  // the other, so neither the name nor the state explanation is lost.
  const tip = showText ? title : [label, title].filter(Boolean).join(" — ");

  const hueOn = hueIndex !== undefined;
  const hueStyle = hueOn ? (hueVars(rainbowAt(hueIndex)) as CSSProperties) : undefined;

  return (
    <button
      type={type}
      onClick={onClick}
      disabled={disabled}
      title={tip || undefined}
      style={hueStyle}
      className={`bv-btn ${stage} ${TONE_CLASS[tone]}${hueOn ? " glim-hue" : ""} ${className}`.trim()}
    >
      {showGlyph && (
        <span className="bv-btn-glyph">
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
      {/* Never removed from the DOM, only hidden: a button whose text is gone
          entirely has no accessible name, which is the exact defect this
          engine could otherwise introduce 197 times over. */}
      <span className={showText ? "bv-btn-label" : "sr-only"}>{label}</span>
    </button>
  );
}
