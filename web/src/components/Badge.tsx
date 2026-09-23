// Badge is the shared status chip. It only paints a chip of a fixed size,
// shape and colour; mapping a domain status to a `tone` stays at the call
// site.
//
// Each size pins height, padding and font size together, so call sites never
// repeat them as literals. Every square icon badge uses `icon`: badges of
// different roles sit side by side in one card, so a per-role size shows up
// as a mismatch. Its 32px is the height of the app's text inputs
// (`text-sm px-3 py-1.5`), so a badge next to a field lines up with it.
//
// `as="button"` and `as="a"` resolve to the same box as the default <span>.
// `appearance-none` strips native button chrome, `box-border` keeps padding
// from inflating the height, and `min-h-0` stops a stretching flex or grid
// parent from growing the box. The anchor keeps native link behaviour (copy
// link, middle-click, the link role) that an onClick on a span loses.
//
// `wrap` turns the fixed `h-*` into a `min-h-*` floor with vertical padding,
// for badges that wrap to two lines in narrow columns. Without a definite
// height the box stretches to a taller flex sibling, so a short wrap badge
// next to a tall one needs `className="self-start"`.
//
// A `tone="heading"` + `size="heading"` badge is a notch: it straddles the top
// edge of its card. `inFlow` keeps the notch's look but drops its positioning,
// so a caller can place several heading badges as one group (see
// StepCard.tsx).

import type { CSSProperties, ReactNode } from "react";
import { hueVars } from "../lib/appearance";
import { IconTipButton } from "./IconTipButton";

export type BadgeTone = "ok" | "fail" | "warn" | "active" | "neutral" | "heading" | "muted";
export type BadgeSize = "small" | "medium" | "large" | "heading" | "icon";
// `square` resolves to the same radius as `rounded` and exists so an icon
// tile reads as one at the call site. `circle` is a pill locked to a 1:1
// aspect. The pill radius is a length, not `50%`, because a percentage radius
// resolves per axis into an ellipse.
export type BadgeShape = "pill" | "rounded" | "square" | "circle";

// Horizontal offset for a heading notch whose static position is wrong (see
// badgeClassName). Tailwind only generates classes it finds as literals in
// source, so the steps are a closed table rather than `start-${n}`.
export type NotchInset = 4 | 5 | 6;
const INSET_START_CLASSES: Record<NotchInset, string> = {
  4: "start-4",
  5: "start-5",
  6: "start-6",
};

// warn uses --status-warn-bg-strong, the token meant for small chips; the
// plain one is the softer fill for full-width warning panels. The two differ
// only in dark mode.
//
// active is a soft wash rather than a solid fill because a run list can show
// several running rows at once. Its text uses text-accentText rather than
// text-accent: the flat accent gold measures 1.50:1 on the accent-soft
// background in light theme, well under the 4.5:1 text minimum.
const TONE_CLASSES: Record<BadgeTone, string> = {
  ok: "bg-statusOkBg text-statusOk",
  fail: "bg-statusFailBg text-statusFail",
  warn: "bg-statusWarnBgStrong text-statusWarn",
  active: "bg-accentSoft text-accentText",
  neutral: "bg-carbon-surface2 text-carbon-textSub",
  // Solid accent with computed ink, the same pair as the primary button. A
  // translucent wash reads as dimmed at the card edge.
  heading: "bg-accent text-accentContrast",
  // No background at all: a quiet caption, such as the version link in the
  // Settings footer, that keeps the badge's box and hover.
  muted: "text-carbon-textMuted",
};

const RADIUS_CLASSES: Record<BadgeShape, string> = {
  pill: "rounded-pill",
  square: "rounded-control",
  rounded: "rounded-control",
  circle: "rounded-pill",
};

// Padding is kept apart from height so the icon-only branch can replace it
// instead of emitting two conflicting px-* utilities, whose order a component
// cannot control. minHeight is the same floor as height, used by `wrap`.
const SIZE_TOKENS: Record<BadgeSize, { height: string; minHeight: string; text: string; padding: string }> = {
  small: { height: "h-[18px]", minHeight: "min-h-[18px]", text: "text-caption", padding: "px-1.5" },
  medium: { height: "h-5", minHeight: "min-h-5", text: "text-dense", padding: "px-2" },
  large: { height: "h-6", minHeight: "min-h-6", text: "text-dense", padding: "px-2.5" },
  // Its own height so a section title never looks like a status chip.
  heading: { height: "h-[22px]", minHeight: "min-h-[22px]", text: "text-dense uppercase tracking-widest", padding: "px-3" },
  // text and padding are unused: an icon-only badge has no text and gets px-0.
  icon: { height: "h-8", minHeight: "min-h-8", text: "text-dense", padding: "px-2" },
};

interface BadgeStyleOptions {
  tone?: BadgeTone;
  size?: BadgeSize;
  shape?: BadgeShape;
  wrap?: boolean;
  className?: string;
  /** Zero padding and a 1:1 aspect ratio. Badge sets it from `tip`, so a
   *  caller cannot set it inconsistently. */
  iconOnly?: boolean;
  inFlow?: boolean;
  insetStart?: NotchInset;
}

function badgeClassName({
  tone = "neutral",
  size = "medium",
  shape = "rounded",
  wrap,
  className,
  iconOnly,
  inFlow,
  insetStart,
}: BadgeStyleOptions = {}): string {
  const { height, minHeight, text, padding } = SIZE_TOKENS[size];
  const isIconOnly = shape === "circle" || iconOnly === true;
  // Never both height and minHeight: same property, same specificity.
  const sizing = wrap
    ? `${minHeight} py-0.5 leading-tight wrap-break-word`
    : `${height} min-h-0 leading-none`;

  // The notch sits on its card's top edge. top-0 plus -translate-y-1/2
  // centres it on that edge at whatever height it renders, so a wrapped notch
  // does not cover the card's first line.
  //
  // With no left or start set, the badge falls back to its static position:
  // the start edge of the <h2> around it, which follows the card's padding
  // and is correct under RTL. That breaks in two card shapes, where the
  // caller passes `insetStart` instead:
  //   1. The `relative` ancestor is not the padded content box, usually
  //      because the content box is overflow-hidden and would clip the notch.
  //   2. The card centres its content, so the <h2>, empty once the badge is
  //      absolute, collapses to zero width in the middle of the card.
  //
  // z-10 only has to clear the card's own content. The radius is a fixed pill
  // whatever the shape engine says: a notch is card chrome, not a control.
  const isHeadingNotch = tone === "heading" && size === "heading";
  const notchChrome = isHeadingNotch ? "shadow-[var(--elevation)]" : "";
  // An in-flow notch is positioned by its caller, so insetStart is ignored.
  const notchPositioning =
    isHeadingNotch && !inFlow
      ? [
          "absolute top-0 -translate-y-1/2 z-10",
          insetStart !== undefined ? INSET_START_CLASSES[insetStart] : "",
        ]
          .filter(Boolean)
          .join(" ")
      : "";

  // An icon-only active badge takes the solid accent: a bare glyph has no
  // text that needs the hued ink, and the translucent wash reads as dimmed.
  // Solid fills from the rainbow palette need the computed ink for contrast.
  const toneClasses =
    isIconOnly && tone === "active" ? "bg-accent text-accentContrast" : TONE_CLASSES[tone];

  return [
    "inline-flex box-border items-center justify-center gap-1 font-medium",
    sizing,
    text,
    isIconOnly ? "px-0 aspect-square" : padding,
    isHeadingNotch ? "rounded-pill" : RADIUS_CLASSES[shape],
    toneClasses,
    notchChrome,
    notchPositioning,
    className,
  ]
    .filter(Boolean)
    .join(" ");
}

export interface BadgeProps {
  children: ReactNode;
  tone?: BadgeTone;
  /** Square icon-only badges always use `"icon"`. */
  size?: BadgeSize;
  shape?: BadgeShape;
  as?: "span" | "button" | "a";
  onClick?: () => void;
  disabled?: boolean;
  title?: string;
  /** Accessible name for a badge whose only content is an aria-hidden glyph. */
  ariaLabel?: string;
  /** Only used with `as="a"`. */
  href?: string;
  target?: string;
  rel?: string;
  /** Let the box grow for content that wraps, instead of clipping at one line. */
  wrap?: boolean;
  className?: string;
  /** Rainbow position, by the caller's index among the badges shown together.
   *  Only `heading` and `active` use it; the status tones keep their meaning.
   *  Omit it for a badge that is the only one of its kind on the page. */
  hueIndex?: number;
  /** With `as="button"`: renders through IconTipButton, which gives an
   *  icon-only badge a real tooltip and its accessible name, and marks the
   *  badge icon-only for sizing. Takes the place of `title` and `ariaLabel`. */
  tip?: string;
  /** Heading notch only. The Tailwind spacing step of the card's content
   *  padding (`5` for `p-5`), for the two card shapes where the static
   *  position is wrong (see badgeClassName). Omit it everywhere else. */
  insetStart?: NotchInset;
  /** Heading notch only. Keeps the notch's look but leaves positioning to the
   *  caller, for several badges that straddle one card edge together. */
  inFlow?: boolean;
}

export function Badge({
  children,
  tone = "neutral",
  size = "medium",
  shape = "rounded",
  as = "span",
  onClick,
  disabled,
  title,
  ariaLabel,
  href,
  target,
  rel,
  wrap,
  className,
  hueIndex,
  tip,
  insetStart,
  inFlow,
}: BadgeProps) {
  // Badge stays hookless: Badge.test.ts calls it as a plain function, where a
  // hook has no dispatcher.
  const hueOn = hueIndex !== undefined && (tone === "heading" || tone === "active");
  // index.css's card-wide reactive hover keys off `.glim-notch-hue`, so only
  // a notch may carry it.
  const isNotchHue = hueOn && size === "heading";
  const shared = badgeClassName({ tone, size, shape, wrap, className, iconOnly: tip !== undefined, inFlow, insetStart });
  const merged = hueOn ? `glim-hue ${isNotchHue ? "glim-notch-hue " : ""}${shared}` : shared;
  const hueStyle = hueOn ? (hueVars(hueIndex) as CSSProperties) : undefined;

  if (as === "button") {
    const buttonClassName = `appearance-none transition-opacity hover:opacity-80 disabled:opacity-50 disabled:hover:opacity-50 ${merged}`;
    if (tip !== undefined) {
      return (
        <IconTipButton tip={tip} onClick={onClick} disabled={disabled} style={hueStyle} className={buttonClassName}>
          {children}
        </IconTipButton>
      );
    }
    return (
      <button
        type="button"
        onClick={onClick}
        disabled={disabled}
        title={title}
        aria-label={ariaLabel}
        style={hueStyle}
        className={buttonClassName}
      >
        {children}
      </button>
    );
  }

  if (as === "a") {
    return (
      <a href={href} target={target} rel={rel} title={title} aria-label={ariaLabel} style={hueStyle} className={`transition-opacity hover:opacity-80 ${merged}`}>
        {children}
      </a>
    );
  }

  return (
    <span title={title} aria-label={ariaLabel} style={hueStyle} className={merged}>
      {children}
    </span>
  );
}
