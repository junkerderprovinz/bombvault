import type { ReactNode } from "react";
import { widthStage, type WidthStage } from "../lib/controls";
import { useLabelMode } from "../lib/useLabelMode";

// ---------------------------------------------------------------------------
// Button — the one clickable control (#178).
//
// Naming, jdp's own (2026-08-28): something you can CLICK is a button,
// something you only read is a badge. That is also what HTML and assistive
// technology require, so the split is not cosmetic: a clickable thing has to
// be a real <button> or it cannot be reached by keyboard.
//
// This component owns three things a call site should never repeat:
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
  glyph,
  onClick,
  tone = "neutral",
  disabled = false,
  type = "button",
  className = "",
  title,
}: {
  /** The button's words. Always present, in every mode: visible as text, or
   *  hidden but still announced and shown as the tooltip in glyph mode. */
  label: string;
  /** Optional icon. Without one, glyph mode shows this button's text instead
   *  of an empty box. */
  glyph?: ReactNode;
  onClick?: () => void;
  tone?: ButtonTone;
  disabled?: boolean;
  type?: "button" | "submit";
  className?: string;
  /** Extra explanation. The label is used as the tooltip in glyph mode on its
   *  own, so this is only for buttons that genuinely need more than their
   *  name; passing it does not suppress that. */
  title?: string;
}) {
  const mode = useLabelMode("buttons");
  // No glyph to show means text, whatever the mode says — see the header note.
  const effective = mode === "glyph" && !glyph ? "text" : mode;
  const showText = effective !== "glyph";
  const showGlyph = effective !== "text" && !!glyph;

  // The stage comes from the label in the CURRENT language, never from what is
  // rendered: the width has to survive a mode change untouched.
  const stage = STAGE_CLASS[widthStage(label)];

  // In glyph mode the label IS the tooltip, so the control still explains
  // itself on hover. A caller-supplied title wins, and is appended rather than
  // replaced when both exist, so neither explanation is lost.
  const tip = showText ? title : [label, title].filter(Boolean).join(" — ");

  return (
    <button
      type={type}
      onClick={onClick}
      disabled={disabled}
      title={tip}
      className={`bv-btn ${stage} ${TONE_CLASS[tone]} ${className}`.trim()}
    >
      {showGlyph && <span className="bv-btn-glyph">{glyph}</span>}
      {/* Never removed from the DOM, only hidden: a button whose text is gone
          entirely has no accessible name, which is the exact defect this
          engine could otherwise introduce 197 times over. */}
      <span className={showText ? "bv-btn-label" : "sr-only"}>{label}</span>
    </button>
  );
}
