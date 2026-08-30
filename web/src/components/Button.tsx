import type { CSSProperties, ReactNode, Ref } from "react";
import { hueVars, rainbowAt } from "../lib/appearance";
import { hidesLabel, widthStage, type WidthStage } from "../lib/controls";
import { useLabelMode } from "../lib/useLabelMode";
import { useTipBubble } from "../lib/useTipBubble";
import { glyphFor } from "./glyphFor";
import { IconClose } from "./navGlyphs";

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
//      That tooltip is the app's REAL `.glim-bubble` (lib/useTipBubble.tsx),
//      never the native `title=` balloon — jdp's ruling on the contradiction
//      glyph mode created. design-language.md calls a native title on an
//      icon-only control the anti-pattern IconTipButton exists to replace,
//      and lint-rules/icon-badge-needs-tooltip.js fails the build over it;
//      glyph mode would otherwise have committed that anti-pattern 171 times
//      in one setting, where before it was a handful of deliberate spots.
//      The choice was to weaken the rule or to make this component honour it.
//      It honours it — and while the bubble was being wired in, the native
//      balloon left this component entirely, so a button's `title` is now
//      shown the same way in every mode instead of switching mechanism when
//      the text disappears.
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

export type ButtonTone = "accent" | "neutral" | "danger" | "warn";

/**
 * "default" — the ordinary action button described above.
 * "chip"    — the tiny remove control that lives INSIDE a pill (a selected
 *             path, a stop-hook name, an exclusion line, the day filter).
 *
 * Why this needs to be its own variant rather than a `className` override at
 * four call sites: a chip's control is glyph-only by nature, in every mode. It
 * sits inside its chip, so it cannot take a width stage without bursting the
 * pill, and its text would be absurd if shown ("Remove exclusion /var/log/*"
 * printed next to the line it already sits on). It still satisfies the
 * same-width-in-all-modes rule, trivially: it is the same size in all three.
 *
 * What it does NOT get to skip is the accessible name. These four buttons each
 * carried a real one in an `aria-label`, naming the thing they remove, and it
 * survives here as the label: hidden, announced, and shown on hover.
 */
export type ButtonVariant = "default" | "chip";

// `danger`/`warn` use the SOLID status tokens over `carbon-background`, the
// pairing ConfirmDialog worked out for itself and documented at length: both
// themes' solid fail/warn values sit at the opposite lightness to that theme's
// own background, so one ink is legible on both without a dedicated contrast
// token. Adopting it here rather than inventing a second recipe means the
// destructive-confirm button looks the same after moving into this component.
const TONE_CLASS: Record<ButtonTone, string> = {
  accent: "bg-accent text-accentContrast hover:opacity-90",
  neutral: "bg-carbon-surface3 text-carbon-text hover:bg-carbon-hover",
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
  ref,
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
  /** "chip" for the small remove control inside a pill; see ButtonVariant. */
  variant?: ButtonVariant;
  disabled?: boolean;
  type?: "button" | "submit";
  className?: string;
  /** Extra explanation, and the place for anything that CHANGES: the running
   *  state, why the button is unavailable right now, which other job is
   *  holding it. The label is used as the tooltip in glyph mode on its own,
   *  and both are shown when both exist.
   *
   *  Shown in the app's own hover/focus bubble, NOT as a native `title`
   *  attribute — the name is kept because that is what 57 call sites already
   *  call this, and because "the extra line of explanation" is exactly what a
   *  title was for. Reaching a DISABLED button's explanation still works: see
   *  useTipBubble's `wrap`, which is why the native balloon could go without
   *  losing the "why is this dead" hint 54 of those call sites pass. */
  title?: string;
  /** This button's rainbow position, same meaning as Badge's own prop. */
  hueIndex?: number;
  /** Forwarded to the underlying <button>. Dialogs need it: they move focus
   *  to their close control on open, which is what makes the Escape key and
   *  the focus trap behave. */
  ref?: Ref<HTMLButtonElement>;
  /** Shows a spinner in place of the glyph. Deliberately separate from
   *  `disabled`: a busy button is usually disabled too, but the two are not
   *  the same thing and a caller may want one without the other. */
  busy?: boolean;
  /** Dialogs open with focus already on one of their buttons; that is what
   *  makes Escape and the focus trap behave, so it has to be expressible
   *  here rather than forcing a call site back to a raw <button>. */
  autoFocus?: boolean;
}) {
  const mode = useLabelMode("buttons");
  // An explicit glyph wins; otherwise the key decides, so the same verb wears
  // the same symbol app-wide without 163 call sites each making a choice.
  const chip = variant === "chip";
  // A chip always closes, so it has a glyph even when no call site passes one.
  const resolved = glyph ?? (labelKey ? glyphFor(labelKey) : undefined) ?? (chip ? <IconClose /> : undefined);
  // No glyph to show means text, whatever the mode says — see the header note.
  const hasGlyph = !!resolved || busy;
  // A glyphless button falls back to its text in BOTH hiding modes: an empty
  // box is unusable, and a reactive empty box is an empty box you have to hunt
  // for with the pointer first.
  const effective = chip ? "glyph" : hidesLabel(mode) && !hasGlyph ? "text" : mode;
  const reactive = effective === "reactive";
  // "Shown" means painted at rest. Reactive is not: its words arrive on hover,
  // which is a CSS state, so as far as this render is concerned it is a hiding
  // mode like glyph.
  const showText = effective !== "glyph" && !reactive;
  const showGlyph = effective !== "text" && hasGlyph;

  // The stage comes from the label in the CURRENT language, never from what is
  // rendered: the width has to survive a mode change untouched. A chip takes no
  // stage at all: it is sized by its glyph so it fits inside its pill.
  const stage = chip ? "bv-btn-chip" : STAGE_CLASS[widthStage(label)];

  // In glyph mode the label IS the tooltip, so the control still explains
  // itself on hover. When both exist they are joined rather than one replacing
  // the other, so neither the name nor the state explanation is lost — unless
  // they are the same words, which collapse rather than printing twice (the
  // Settings tab strip does exactly that on purpose, one component over).
  //
  // Reactive counts as "text is shown" here even though it is hidden at rest:
  // hovering is exactly what reveals the words, so a bubble carrying the same
  // words would arrive at the same moment, say the same thing, and cover the
  // animation doing it.
  const tip =
    (showText || reactive
      ? title
      : [...new Set([label, title].filter(Boolean))].join(" — ")) || undefined;
  const tooltip = useTipBubble(tip, disabled);

  const hueOn = hueIndex !== undefined;
  const hueStyle = hueOn ? (hueVars(rainbowAt(hueIndex)) as CSSProperties) : undefined;

  return (
    <>
      {/* `wrap` only does anything for a DISABLED button carrying a tip, which
          is 54 of the 57 that pass one: a disabled <button> emits no mouse
          events and takes no focus, so without a box around it the "why is
          this unavailable" line would be unreachable at the exact moment it is
          wanted. The native balloon used to show there for free; that is the
          one thing it did better, and this is the price of replacing it.
          No enabled button gains a wrapper, and no `w-full`/`flex-1` call site
          is ever disabled, so no layout changes. */}
      {tooltip.wrap(
        <button
          ref={mergeRefs(ref, tooltip.ref)}
          type={type}
          onClick={onClick}
          disabled={disabled}
          autoFocus={autoFocus}
          aria-describedby={tooltip.describedBy}
          {...tooltip.handlers}
          style={hueStyle}
          className={`bv-btn ${stage} ${chip ? "" : TONE_CLASS[tone]}${hueOn ? " glim-hue" : ""}${reactive ? " bv-reactive" : ""} ${className}`.trim()}
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
          {/* Never removed from the DOM, only hidden: a button whose text is
              gone entirely has no accessible name, which is the exact defect
              this engine could otherwise introduce 197 times over.
              Reactive gets a THIRD treatment: visible to everyone, but with a
              collapsed box that opens on hover — so unlike `sr-only` it is
              really there, and unlike `bv-btn-label` it takes no room until
              asked. */}
          <span className={showText ? "bv-btn-label" : reactive ? "bv-label-reactive" : "sr-only"}>
            {label}
          </span>
        </button>,
      )}
      {tooltip.bubble}
    </>
  );
}

/**
 * Feeds one element to both the caller's `ref` (dialogs move focus to a button
 * on open, which is what makes Escape and the focus trap behave) and the
 * tooltip's own, which needs the same node's rect to place the bubble against.
 *
 * Written out rather than reached for from a library because this is the only
 * place in the app that needs it, and because the object-ref case has to be
 * handled too: `ConfirmDialog` passes a `useRef`, not a callback.
 */
function mergeRefs(
  outer: Ref<HTMLButtonElement> | undefined,
  inner: (el: HTMLElement | null) => void,
): (el: HTMLButtonElement | null) => void {
  return (el) => {
    inner(el);
    if (typeof outer === "function") outer(el);
    else if (outer) outer.current = el;
  };
}
