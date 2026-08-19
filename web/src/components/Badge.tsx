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
// `as="a"` (Task 5, rule 13 — "everything clickable is a badge, including
// links") renders a real <a href> instead of onClick-on-a-span: a plain-text
// link still needs native anchor behaviour (right-click "copy link address",
// middle-click/ctrl-click to open in a new tab, the status-bar URL preview,
// screen-reader "link" role vs. "button" role) that a synthetic onClick
// handler degrades. `href`/`target`/`rel` are only meaningful with `as="a"`
// and pass straight through. Resolves to the identical box as span/button at
// the same stage — see `badgeClassName` below, which all three branches
// share so nothing can drift them apart independently.
//
// react-router's <Link> is a fourth clickable element considered for this
// task (it needs router context Badge has no reason to depend on, so
// teaching Badge to render one directly would be real added machinery) —
// but both of this app's two actual <Link>-as-plain-text sites turned out
// NOT to want the Badge treatment on inspection: Dashboard.tsx's offsite-
// fault row link stays plain text (converting only the one clickable ROW
// STATE in an otherwise-plain-text list would make row height/typography
// jump depending on which domain is faulted — see that call site's own
// comment), and its "go to Recovery" nudge link instead adopted the app's
// existing primary-action button idiom directly (rule 3's one-solid-accent-
// action allowance, matching every other primary button already in the
// app — also documented at that call site). `badgeClassName` stays a
// private, non-exported helper for now: exporting a public escape hatch
// with zero real call sites is speculative API surface, not a needed one —
// if the same "must render as a router <Link>, but look like a Badge" need
// turns up at a genuine future site, exporting this one-line function is a
// one-line change.
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
//
// One consequence worth knowing: a fixed `h-*` is a DEFINITE cross size, which
// is what actually makes a stretch-aligned flex/grid parent leave the box at
// the stage's own height (`min-h-0` above is belt-and-braces). `wrap` trades
// that definite height for `min-h-*`, so a wrap badge sharing a flex line with
// a taller sibling WILL stretch to that line's height. Harmless where it is
// used today (a Dashboard wrap badge is the only thing in its grid cell, and
// OffsiteTargetsSection's two chips wrap onto separate flex lines before either
// grows), but a new call site that wants a short wrap badge to stay short next
// to a tall one needs `className="self-start"`.
//
// `tone="heading"` / `size="heading"` (Task 5, rule 11 — "every heading is a
// filled section badge, never bare text... always coloured: it is a heading,
// not a control, so rule 9 [the three-way reactive colour mode] does not
// apply to it"). Colour choice, reasoned from the rest of the spec rather
// than copied from an example (no adopting app — including this plan's own
// KnightLoader reference implementation — has actually built rule 11 yet, so
// there was no existing call site to match):
//   - NOT solid `--accent`: rule 3 reserves the accent for ACTIVITY ("the
//     active nav item, the single primary action, progress fills... a page
//     has at most one solid accent button") — a page with a dozen Settings
//     Cards would need a dozen solid-accent headings, which turns "activity"
//     into "everything", and the one real primary action on the page would
//     stop reading as special.
//   - NOT any of the four state hues (ok/fail/warn/neutral): rule 4's hues
//     are load-bearing semantic signals elsewhere on the same pages these
//     headings live on (a container's running/settled/fault state, a
//     schedule's warning). Painting a heading "neutral" would borrow the
//     literal "waiting" state hue for a label that isn't waiting on
//     anything; painting it any of the other three invents a false status.
//   - IS `--accent-soft`: the spec's own vocabulary already has a distinct
//     register for "identity, not activity" — rule 5's "what is selected is
//     filled with the accent; what owns a rainbow position is washed with
//     it" and the colour engine's `.glim-tint` (a 7% wash, not a solid fill,
//     for a settled row that still needs to read as coloured without
//     re-triggering the running/activity read). A heading badge borrows that
//     same wash vocabulary: `--accent-soft` ties every heading to the app's
//     own chosen brand colour — "coloured", genuinely, and consistent with
//     whichever accent preset the user picked — without ever reading as
//     "this is currently active" the way a solid fill would. Every heading
//     gets the SAME flat accent-soft wash (not a rainbow position): a badge
//     is not a list row with an identity to distinguish from its neighbours,
//     and cycling 21+ headings through 8 rainbow hues would fight rule 2
//     ("one hero, everything else quiet") far worse than a uniform wash does.
//   - Text stays `text-carbon-textSub` (not full-strength `text-carbon-text`
//     and not `--accent-contrast`, which assumes a SOLID accent background
//     dark/light enough to need a computed-contrast partner — accent-soft is
//     11-14% opacity, so the card surface underneath still dominates the
//     actual contrast ratio): this keeps a heading badge reading at the same
//     quiet register the eyebrow treatment always used, rather than
//     brightening 20+ headings per page to full text strength.
//   - Sizing is a dedicated stage, not a reuse of `large` (the existing
//     tallest stage, already the visual weight of a real status/count chip —
//     ErrorDetailPanel's count badge, a "Resolve" button): giving headings
//     that SAME stage would make a passive section label compete pixel-for-
//     pixel with a genuine activity/status indicator for attention, exactly
//     backwards from what a heading should do. `heading`'s own stage keeps
//     the source uppercase+letter-spaced treatment the app's headings always
//     had (so this reads as an evolution of the existing convention, not a
//     wholesale redesign) with roomier padding sized for a title's worth of
//     text rather than a two-character status word.
//
// Deliberately NOT converted to this stage: a SECOND, nested heading level
// (an <h3>/<span> sub-label already living inside a Card/panel this stage
// already gave its own badge — e.g. Settings.tsx's "Export"/"Import"
// sub-headings inside the Settings I/O Card, OffsiteWizard's per-step
// labels inside its one wizard card). Badging every nesting level would
// stack multiple equally-loud badges inside one another and read as a wall
// of chips, not a hierarchy — rule 5 ("hierarchy from type and colour step")
// still applies one level down even though rule 11 exempts the outer
// heading from rule 9. Those sub-labels keep the plain eyebrow-style
// treatment on purpose; see each call site.
//
// (An earlier pass of this task filed Dashboard.tsx's SummaryCell labels
// here too — that was wrong. Each SummaryCell is its own standalone
// bg-carbon-surface rounded-card box, not nested inside anything already
// badged; it was actually a missed OUTERMOST heading and is now converted
// at its own call site, same as every other Card heading on that page.)
//
// Also deliberately NOT converted: the ~7 modal dialog titles (Files.tsx,
// Fleet.tsx x2, Receiver.tsx, WhatsNewDialog.tsx, ConfirmDialog.tsx,
// ErrorDetailPanel.tsx — all `text-lg font-semibold`). A dialog's <h2> is a
// different element class from a page's section heading: it names the
// MODAL WINDOW itself (rule 15 territory, "title as a badge" for a window
// chrome), not a content section inside a page (rule 11). Several of these
// also wire their `id` to the dialog's own `aria-labelledby` for the
// accessible name (ConfirmDialog, WhatsNewDialog, ErrorDetailPanel) — Badge
// has no `id` prop today, so converting them would mean either dropping
// that wiring or growing Badge's public API in the same pass, neither of
// which belongs in a section-heading fix. Left for a dedicated rule-15 pass.
//
// Also deliberately NOT converted, and the only outermost <h2> in the app
// that stays bare text: an ALERT/CALLOUT heading whose panel is itself a
// filled status surface — today exactly one site, Dashboard.tsx's
// RecoveryNag (`bg-statusWarnBg` + `recovery.nagTitle`). Rule 11's "filled
// section badge" silently assumes a neutral card surface underneath for the
// fill to register against; a status callout has already spent that budget
// on its own background. Measured live at that site, badge-fill vs. the
// panel it would sit on: accent-soft 1.06:1 light / 1.39:1 dark, and
// warn-strong 1.00:1 light (index.css gives --status-warn-bg and
// --status-warn-bg-strong the same value in light mode) / 1.11:1 dark. So
// the badge would read as plain text with extra padding, while also
// discarding the text-statusWarn colour that currently carries the alert's
// meaning at 8.62:1. Left plain on purpose — this is a token gap, not an
// oversight, and the call site says so too. Every OTHER outermost heading
// in the app is a badge; if a future pass adds a real "badge on a status
// surface" tone pair, this is the one site waiting for it.
//
// Page titles (the `<h1 text-2xl font-semibold>` at the top of each page,
// plus Recovery.tsx's `text-lg` one) are out of rule 11's scope for the same
// reason the dialog titles above are: an <h1> names the WHOLE VIEW, it is
// not a section inside one. All 11 are consistently left plain today; they
// belong to the same future rule-15 "title as a badge" pass as the dialogs.
// ---------------------------------------------------------------------------

import type { ReactNode } from "react";

export type BadgeTone = "ok" | "fail" | "warn" | "info" | "neutral" | "heading";
export type BadgeSize = "small" | "medium" | "large" | "heading";
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
  // See the file header's long-form reasoning: accent-soft wash (identity,
  // matching rule 5's "washed" vocabulary), not solid accent (rule 3,
  // activity-only) and not one of the four state hues (rule 4, semantic
  // elsewhere on the same page).
  heading: "bg-accentSoft text-carbon-textSub",
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
  // Deliberately its own height (22px — between medium and large), not a
  // reuse of either: same height as a real status chip would make a passive
  // heading compete with genuine activity/status indicators for attention
  // (see file header); a fixed size distinct from all three status-chip
  // stages keeps a heading visually identifiable as "a heading" on sight,
  // separate from whether it happens to render at a similar footprint.
  // uppercase+tracking-widest preserves the source eyebrow treatment's
  // typographic character; roomier px-3 padding fits a title's worth of
  // text rather than a two-character status word.
  heading: { height: "h-[22px]", minHeight: "min-h-[22px]", text: "text-dense uppercase tracking-widest", padding: "px-3" },
};

interface BadgeStyleOptions {
  tone?: BadgeTone;
  size?: BadgeSize;
  shape?: BadgeShape;
  wrap?: boolean;
  className?: string;
}

/**
 * The exact class list a Badge stage resolves to. Not exported — Badge's own
 * span/button/a branches below are its only callers today, so this stays a
 * private implementation-sharing helper rather than public API with no real
 * consumer (see the file header's note on why this app's two react-router
 * `<Link>` sites didn't end up needing it after all).
 */
function badgeClassName({
  tone = "neutral",
  size = "medium",
  shape = "rounded",
  wrap,
  className,
}: BadgeStyleOptions = {}): string {
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
  return [
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
}

export interface BadgeProps {
  children: ReactNode;
  /** Status color. Defaults to the neutral/muted chip every predecessor fell
   *  back to when no other tone applied. */
  tone?: BadgeTone;
  /** Named size stage — see the file header for the exact px values and
   *  which predecessor weight each one replaces. */
  size?: BadgeSize;
  shape?: BadgeShape;
  /** Render as a clickable <button> or a real <a href> instead of a plain
   *  <span>, resolving to the identical box (height/padding/font/radius) at
   *  the same stage either way — see the file header. */
  as?: "span" | "button" | "a";
  onClick?: () => void;
  disabled?: boolean;
  title?: string;
  /** `as="a"` only: passed straight through to the underlying <a>. */
  href?: string;
  target?: string;
  rel?: string;
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
  href,
  target,
  rel,
  wrap,
  className,
}: BadgeProps) {
  const shared = badgeClassName({ tone, size, shape, wrap, className });

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

  if (as === "a") {
    return (
      <a href={href} target={target} rel={rel} title={title} className={`transition-opacity hover:opacity-80 ${shared}`}>
        {children}
      </a>
    );
  }

  return (
    <span title={title} className={shared}>
      {children}
    </span>
  );
}
