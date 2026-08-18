// ---------------------------------------------------------------------------
// Badge — the one shared status chip/pill (GlimStone form-engine Task 5).
//
// Replaces five previously-separate copies of the same "colored chip" markup:
// Dashboard.tsx's StatusChip, components/SpikePanel.tsx's own byte-different
// duplicate StatusChip (which also hard-coded untranslated English
// "OK"/"FAIL"/"INFO" — fixed at ITS call site via the spike.ok/spike.fail/
// spike.info i18n keys, not inside Badge, since status TEXT is domain content
// Badge has no business owning), Receiver.tsx's Badge, Fleet.tsx's byte-
// identical duplicate of it, and Containers.tsx/VMs.tsx's StateChip pair.
//
// Badge only knows how to paint a fixed-size, fixed-shape, fixed-color chip.
// Mapping a domain status/state string to a `tone` stays call-site logic (a
// few lines of local switch/lookup) — the same separation Toggle.tsx draws
// between "how a switch renders" and "what a caller's label says".
//
// Sizing is three named stages (small/medium/large per the design language's
// "however many an app genuinely needs — small/medium/large is usually
// enough"), each pinning height + horizontal padding + font-size together as
// one canonical source, read verbatim from the stage table below rather than
// left for a call site to repeat as its own literals — that repetition is
// exactly how the five duplicates this file replaces drifted apart in the
// first place (px-2/px-1.5/no-size-class/text-[10px] all coexisting for what
// was supposed to be one visual weight).
//   - small  = 18px tall, 11px text  (--text-caption) — the OffsiteTargets-
//     Section storage-class/immutable weight, previously the "no size class"
//     bug (padding with no font-size utility of its own, height at the mercy
//     of whatever the parent's ambient font-size happened to be).
//   - medium = 20px tall, 12px text (--text-dense) — the dominant weight
//     used by every StatusChip/Badge/StateChip predecessor.
//   - large  = 24px tall, 12px text (--text-dense) — the ErrorDetailPanel
//     count-badge + "Resolve"-button weight (see `as="button"` below).
//
// `as="button"` renders a real <button> instead of a <span> but resolves to
// the EXACT SAME box at a given stage: same height, same padding, same font,
// same radius. A native button brings its own default padding/border/font
// that a <span> never had, so this is spelled out explicitly rather than
// left to whatever the element normally defaults to — `appearance-none`
// strips native button chrome, `box-border` fixes box-sizing so the
// declared height isn't inflated by border/padding math, and `min-h-0`
// blocks a stretch-alignment flex/grid parent from growing the box taller
// than the stage's own height. This is what actually fixes the
// ErrorDetailPanel span-vs-button height mismatch: both render through this
// one component at the same stage, so nothing downstream can drift them
// apart independently again.
//
// `wrap` — a stage's height is a fixed pixel floor for the common one-line
// case, but some call sites (the Dashboard protection-card badges: local
// verify shield / off-site subset / off-site DR, all icon+label+relative-
// time strings inside a narrow grid column) genuinely wrap to two lines in
// normal use, in the default locale, not just a translated-string edge case.
// A fixed `h-*` + `leading-none` painted the tinted background at exactly
// one line tall and let the wrapped second line spill outside it — the
// background box didn't grow with the content. `wrap` swaps the stage's
// `h-*` for `min-h-*` at the same px floor (so a single-line badge is
// unchanged), drops `leading-none` for readable multi-line spacing, and adds
// real vertical padding, so the box grows to contain however many lines the
// content needs instead of clipping at the stage's one-line floor.
// ---------------------------------------------------------------------------

import type { ReactNode } from "react";

export type BadgeTone = "ok" | "fail" | "warn" | "info" | "neutral";
export type BadgeSize = "small" | "medium" | "large";
// Four shapes per the design language's Badges section: pill (fully round,
// standalone chips/count badges), rounded (small fixed radius, compact
// inline badges — the default, matching every predecessor's rounded-control),
// square (0, mirrors the shape engine's square setting), circle (pill radius
// again but width locked to height, for icon-only/single-glyph badges — same
// radius as pill, distinct semantic use). Deliberately NOT the percentage-
// capped `min(var(--radius-pill), 50%)` formula: a CSS percentage
// border-radius resolves per-axis into an ellipse, not a stadium, so the
// plain length-based `rounded-pill` token (which already auto-scales
// correctly against this component's own fixed per-stage heights) is used
// as-is, with zero cap needed.
export type BadgeShape = "pill" | "rounded" | "square" | "circle";

// warn uses --status-warn-bg-STRONG, not the plain --status-warn-bg: the
// token file (index.css) labels -strong verbatim "emphasised warn chip
// (Files)" — it exists FOR small high-contrast chips like this one, while
// plain --status-warn-bg is the softer tone used by full-width warning
// panels/callouts (Settings.tsx, OffsiteWizard.tsx) that hold paragraph
// text, a different UI role that can afford to be quieter. Receiver.tsx and
// Fleet.tsx's old local Badge both used -strong for warn; using the plain
// tone here would have silently weakened their warn chips (invisible in
// light mode today, where the two tokens happen to share one value, but a
// real regression in dark mode).
const TONE_CLASSES: Record<BadgeTone, string> = {
  ok: "bg-statusOkBg text-statusOk",
  fail: "bg-statusFailBg text-statusFail",
  warn: "bg-statusWarnBgStrong text-statusWarn",
  info: "bg-statusInfoBg text-statusInfo",
  neutral: "bg-carbon-surface2 text-carbon-textSub",
};

const RADIUS_CLASSES: Record<BadgeShape, string> = {
  pill: "rounded-pill",
  rounded: "rounded-control",
  square: "rounded-none",
  circle: "rounded-pill",
};

// height/text are the two dimensions that must be pixel-identical between a
// span and a button at the same stage; padding is kept separate so the
// circle shape can zero it out below without ever needing two conflicting
// px-* utilities to coexist in one className (Tailwind's cascade order
// between two same-specificity utility classes isn't something a component
// file can pin down on its own). minHeight is the same px floor as height,
// spelled as `min-h-*` instead of `h-*` for `wrap` mode (see the file
// header) — never emitted alongside `height` in the same className for the
// same cascade-order reason padding is kept separate from the circle override.
const SIZE_TOKENS: Record<BadgeSize, { height: string; minHeight: string; text: string; padding: string }> = {
  small: { height: "h-[18px]", minHeight: "min-h-[18px]", text: "text-caption", padding: "px-1.5" },
  medium: { height: "h-5", minHeight: "min-h-5", text: "text-dense", padding: "px-2" },
  large: { height: "h-6", minHeight: "min-h-6", text: "text-dense", padding: "px-2.5" },
};

export interface BadgeProps {
  children: ReactNode;
  /** Status color. Defaults to the neutral/muted chip every predecessor fell
   *  back to when no other tone applied. */
  tone?: BadgeTone;
  /** Named size stage — see the file header for the exact px values and
   *  which predecessor weight each one replaces. */
  size?: BadgeSize;
  shape?: BadgeShape;
  /** Render as a clickable <button> instead of a plain <span>, resolving to
   *  the identical box (height/padding/font/radius) as a <span> at the same
   *  stage — see the file header. */
  as?: "span" | "button";
  onClick?: () => void;
  disabled?: boolean;
  title?: string;
  /** Grow-to-fit instead of clipping: the stage's height becomes a floor
   *  (`min-h-*`) rather than a fixed `h-*`, so content that wraps to more
   *  than one line grows the box instead of overflowing it. For a badge
   *  whose content is known to sometimes wrap (e.g. inside a narrow column
   *  with `max-w-full`) — see the file header. */
  wrap?: boolean;
  /** Extra classes (e.g. `tabular-nums` for a count badge, `max-w-full` for
   *  a narrow-column badge that needs to wrap rather than overflow — pair
   *  with `wrap` so the box grows instead of clipping). */
  className?: string;
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
  wrap,
  className,
}: BadgeProps) {
  const { height, minHeight, text, padding } = SIZE_TOKENS[size];
  // circle is icon/glyph-only: zero horizontal padding + a locked 1:1 aspect
  // ratio against the stage's own height turns it into a true circle rather
  // than an oval widened by the stage's normal text padding.
  const isCircle = shape === "circle";
  // wrap swaps the fixed one-line `h-*`+`leading-none`+`min-h-0` sizing for a
  // `min-h-*` floor + real vertical padding + normal line-height, so a
  // second wrapped line grows the box instead of overflowing it — see the
  // file header. Never emit both `height` and `minHeight` (same CSS
  // property, same specificity — exactly the two-conflicting-utilities
  // hazard the padding/circle split above already guards against).
  const sizing = wrap
    ? `${minHeight} py-0.5 leading-tight wrap-break-word`
    : `${height} min-h-0 leading-none`;
  const shared = [
    "inline-flex box-border items-center justify-center gap-1 font-medium",
    sizing,
    text,
    isCircle ? "px-0 aspect-square" : padding,
    RADIUS_CLASSES[shape],
    TONE_CLASSES[tone],
    className,
  ]
    .filter(Boolean)
    .join(" ");

  if (as === "button") {
    return (
      <button
        type="button"
        onClick={onClick}
        disabled={disabled}
        title={title}
        className={`appearance-none transition-opacity hover:opacity-80 disabled:opacity-50 disabled:hover:opacity-50 ${shared}`}
      >
        {children}
      </button>
    );
  }

  return (
    <span title={title} className={shared}>
      {children}
    </span>
  );
}
