import type { CSSProperties, ReactNode } from "react";
import { useTipBubble } from "../lib/useTipBubble";

// ---------------------------------------------------------------------------
// IconTipButton — a plain icon-only <button> whose accessible name AND its
// only hover/focus explanation is a real `.glim-bubble` tooltip, not the OS's
// own native `title=` balloon.
//
// Root cause (jdp, live-review: "beim Ordnersymbol ist die Hover-Infobubble
// nicht im GlimStone"): FolderBrowser's own "Durchsuchen" icon button carried
// only `title=`/`aria-label=` — the browser's own plain OS balloon, visibly
// disagreeing with PathModeSwitch's Local/Remote icon pair sitting right next
// to it, which already got the real `.glim-bubble` treatment in an earlier
// round. `tip` is REQUIRED here, not optional: design-language.md's own
// tooltip section is explicit that "an icon-only button (no visible text at
// all) needs one unconditionally — there's no other way to know what it does."
//
// This file used to carry its own copy of the open-state / measure-then-clamp
// / close-on-scroll-or-Escape machinery — the THIRD of four identical copies
// in this app. That machinery now lives in lib/useTipBubble.tsx and this file
// is what remained once it left: a button element, a `tip` that is also its
// accessible name, and the two ARIA state props below. See the hook's own
// header for why the four copies had to become one.
//
// What is left here that Button.tsx does NOT do — i.e. why this component
// still exists as its own thing now that Button has grown the same bubble:
// this is the control with NO label to hide. A bare glyph in a caller-sized
// box (`h-8 w-8`, `size-7`), taking no width stage, no tone fill and no place
// in the label-mode engine, because there is no text for a mode to show or
// hide. Button is a labelled action that CAN be shown as a glyph; this is a
// glyph that never had words in the first place.
// ---------------------------------------------------------------------------
export function IconTipButton({
  tip,
  onClick,
  disabled,
  className,
  style,
  children,
  type = "button",
  ariaPressed,
  ariaExpanded,
}: {
  /** Hover/focus-revealed explanation AND the button's accessible name
   *  (`aria-label`) — an icon-only trigger has no other visible text a name
   *  could come from. */
  tip: string;
  onClick?: () => void;
  disabled?: boolean;
  className?: string;
  /** `aria-pressed` for an icon-only trigger that is a TOGGLE rather than a
   *  one-shot action — it stays visibly "on" between clicks, so assistive tech
   *  needs the pressed state as well as the name. Added for Dashboard's
   *  customize pencil, which was one of the icon-only controls still
   *  explaining itself with a native `title=` balloon — exactly the
   *  anti-pattern this file's header says it exists to replace.
   *    (That change's own note called the pencil "the LAST" such control. It
   *  was not: nine more survived it, in dashboardLayout's four card controls
   *  and the Containers/VMs reorder arrows, and they only came to light when
   *  bombvault/icon-badge-needs-tooltip started failing the build on them.
   *  They are converted now, and the rule is what keeps the count at zero.)
   *  Omitted — the default — renders no attribute at all, so every existing
   *  one-shot action button is unchanged. */
  ariaPressed?: boolean;
  /** `aria-expanded` for an icon-only trigger that OPENS something — a
   *  disclosure chevron whose panel appears below it. Same contract as
   *  `ariaPressed` above (omitted renders no attribute), added for
   *  Containers' stack-restore disclosure toggle, which carried
   *  `aria-expanded` on the plain `<button>` it replaced and must not lose
   *  it on the way to a real tooltip. */
  ariaExpanded?: boolean;
  /** Inline style, added for Badge.tsx's own `tip` branch (GlimStone
   *  follow-up round — the off-site tab's four action buttons converting
   *  from text badges to icon-only ones): a hue-enabled Badge needs its
   *  `--item-hue*` custom properties set inline (the same mechanism its own
   *  plain `<button>`/`<a>` branches already use via `style={hueStyle}`),
   *  and this plain-`<button>` wrapper had no way to carry that until now —
   *  every other consumer of this component today has no hue, so this is
   *  additive and optional, not a behaviour change for them. */
  style?: CSSProperties;
  children: ReactNode;
  type?: "button" | "submit";
}) {
  const tooltip = useTipBubble(tip, disabled);

  return (
    <>
      {tooltip.wrap(
        <button
          ref={tooltip.ref}
          type={type}
          onClick={onClick}
          disabled={disabled}
          aria-label={tip}
          aria-pressed={ariaPressed}
          aria-expanded={ariaExpanded}
          aria-describedby={tooltip.describedBy}
          {...tooltip.handlers}
          className={className}
          style={style}
        >
          {children}
        </button>,
      )}
      {tooltip.bubble}
    </>
  );
}
