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
// apply to it"). Colour choice:
//   - IS the full, solid `--accent` fill + `--accent-contrast` ink — the
//     EXACT SAME pairing this app already uses for a solid CTA elsewhere
//     (Sidebar.tsx's `navActive`, the "Speichern" button). REVISED from this
//     component's original rule-3-driven reasoning (rule 3 reserves solid
//     accent for ACTIVITY — "the active nav item, the single primary action,
//     progress fills... a page has at most one solid accent button" — and a
//     first pass read a Card heading as NOT activity, so it shipped a soft
//     `--accent-soft` wash instead, then an even-more-opaque `color-mix()`
//     of that same wash once the wash's own transparency became visible at
//     the notch's card-edge seam). jdp reviewed BOTH of those live and, on
//     the second round, said the notch still read as "abgedunkelt"
//     (darkened/dimmed) regardless of opacity — because a 14%-accent-into-
//     surface wash is inherently a pale, muted colour, full stop, whether
//     it's rendered with alpha or composited to a flat opaque equivalent of
//     the exact same pale colour. What was actually wanted is the real,
//     saturated accent colour, unmixed with anything — so this tone now
//     renders `bg-accent text-accentContrast` (see TONE_CLASSES below), and
//     `--accent-soft-solid` (the interim opaque-wash token from the prior
//     round) is gone from index.css entirely — nothing references it anymore.
//   - NOT any of the four state hues (ok/fail/warn/neutral): rule 4's hues
//     are load-bearing semantic signals elsewhere on the same pages these
//     headings live on (a container's running/settled/fault state, a
//     schedule's warning). Painting a heading "neutral" would borrow the
//     literal "waiting" state hue for a label that isn't waiting on
//     anything; painting it any of the other three invents a false status.
//     (This is specifically about rule 4's STATUS reading of neutral, on a
//     structural element that has no status. It doesn't conflict with
//     tone="neutral" being this same file's generic no-real-status default
//     for actual chips/badges elsewhere — see the TONE_CLASSES comment below
//     on the Task 7 "→ neutral" sites, which lands in that other,
//     pre-existing role instead.)
//   - REVISED AGAIN (jdp, second live-review round, emphatic): "Die ganzen
//     Toggle, Abschnittsbadges sind nicht in der Farbengine!! Im
//     Regenbogen-Modus hat alles die gleiche Farbe!" — every heading on a
//     rainbow-mode page WAS rendering the identical flat accent fill,
//     confirmed live against the real deployed container (Settings' seven
//     general-tab Card notches — Domänen/Sprache/Design/Akzentfarbe/Ecken/
//     Regenbogen-Modus/Leise Benachrichtigungen — all measured
//     rgb(252,196,25), the flat accent, with rainbow mode ON). The
//     paragraph below (kept for its still-correct "a heading isn't a
//     control" / rule-9 reasoning) argued headings should NEVER take a
//     rainbow position at all; jdp's live reaction overrides that specific
//     conclusion, not the reasoning about rule 3/rule 9 — a heading can
//     hold a rainbow POSITION (like any other list member) while still
//     being furniture rather than a control.
//
//     `hueIndex` (optional, below) is the opt-in this reversal needed:
//     omitted, a heading renders exactly as before (flat `bg-accent`,
//     unaffected by rainbow mode — correct for a genuine singleton heading,
//     the one-of-a-kind case design-language.md's rainbow section itself
//     carves out). Passed, the heading joins the rainbow the same way a
//     Selector segment or a ContainerRow does: `.glim-hue` +
//     `hueVars(rainbowAt(hueIndex))`, position assigned by the caller's own
//     LIST INDEX among the headings visible on that page/tab at once — a
//     page's several Card titles are exactly the "genuine equal-member set"
//     rule 2's "one hero, everything else quiet" was never meant to flatten
//     into a single shared colour; rule 2 governs the page's ONE <h1>, not
//     how many DIFFERENT hues its supporting section headings may carry.
//     Deliberately restricted to `tone === "heading"` only (see the Badge()
//     body below): every other tone is one of rule 4's four state hues, a
//     load-bearing status signal a stray `hueIndex` must never overwrite.
//     EXTENDED (offsite-tab card-split follow-up): `tone === "active"` now
//     qualifies too — see the `hueOn` computation in Badge()'s own body for
//     the full reasoning (it was never one of rule 4's four state hues to
//     begin with, the same reason this section's own history above gives
//     for why "active" replaced "info").
//   - Every heading gets a SOLID fill (not a translucent wash) either way —
//     flat accent when no `hueIndex` is given, its own rainbow position's
//     colour when one is. This does mean rule 3's "at most one solid accent
//     thing per page" no longer literally holds once a page has several
//     Cards — an accepted, deliberate consequence of this live-review
//     round's explicit ask, not an oversight: a heading badge is furniture
//     (rule 11 already exempts it from rule 9's reactive-colour treatment
//     for the same "not a control" reason), not a second competing call to
//     action, so several solid-accent HEADINGS on one page don't dilute the
//     page's one real primary-action button the way several solid-accent
//     BUTTONS would.
//   - Text is `text-accentContrast` (the computed black/white ink for
//     legibility ON TOP of a solid accent fill — see index.css's
//     `--accent-contrast` comment), not the previous quiet
//     `text-carbon-textSub`: that muted ink was chosen specifically for a
//     translucent wash where the card surface still dominated the contrast
//     ratio, a premise that no longer holds now the fill is fully opaque
//     accent.
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
// RESOLVED (GlimStone follow-up pass, folded into v8.0.0): the ~7 modal
// dialog titles (Files.tsx, Fleet.tsx x2, Receiver.tsx, WhatsNewDialog.tsx,
// ConfirmDialog.tsx, ErrorDetailPanel.tsx) are now `tone="heading"
// size="heading"` Badges too. Rule 15 ("a window is a window... same
// surface, same radius, same elevation, title as a badge, button row at the
// bottom") is explicit about this: "title as a badge" is one clause in a
// list of concrete window-chrome requirements sitting alongside surface,
// radius and elevation — it is a prescription, not a placeholder for "figure
// this out later." It doesn't say WHICH badge, so this reuses rule 11's
// already-reasoned `tone="heading"`/`size="heading"` stage rather than
// inventing a second heading treatment the spec never asked for — a dialog
// title and a section heading are both "the name of the thing the reader is
// looking at," just at different scopes.
//
// Badge growing an `id` prop turned out unnecessary: the `<h2 id="…">`
// wrapper stays exactly where it was in the three sites that wire
// `aria-labelledby` to it (ConfirmDialog, WhatsNewDialog, ErrorDetailPanel)
// — only the h2's INNER content changed from bare text to `<Badge>`. An
// `aria-labelledby` reference resolves to the target element's computed
// text content, which still includes the Badge's rendered text regardless
// of the span nested inside it, so the accessible name is byte-identical to
// before. The other four dialogs (Files.tsx, Fleet.tsx x2, Receiver.tsx)
// name themselves via `aria-label` on the `role="dialog"` div directly, not
// via this heading at all, so there was nothing to preserve there either.
// Verified live in both themes with a screen reader against all 7 — see the
// PR/commit description for the accessibility-check writeup.
//
// Also deliberately NOT converted, and the only outermost <h2> in the app
// that stays bare text: an ALERT/CALLOUT heading whose panel is itself a
// filled status surface — today exactly one site, Dashboard.tsx's
// RecoveryNag (`bg-statusWarnBg` + `recovery.nagTitle`). Rule 11's "filled
// section badge" silently assumes a neutral card surface underneath for the
// fill to register against; a status callout has already spent that budget
// on its own background. Measured live at that site, at the time, against
// tone="heading"'s THEN-current accent-soft wash fill: accent-soft 1.06:1
// light / 1.39:1 dark, and warn-strong 1.00:1 light (index.css gives
// --status-warn-bg and --status-warn-bg-strong the same value in light mode)
// / 1.11:1 dark (numbers now superseded — a later live-review round replaced
// tone="heading"'s wash with a solid `bg-accent` fill, see this file's own
// tone="heading" section above — not re-measured against that fill, since
// RecoveryNag's heading stays plain either way and this site's own
// conclusion doesn't turn on the exact figure). So the badge would read as
// plain text with extra padding, while also
// discarding the text-statusWarn colour that currently carries the alert's
// meaning at 8.62:1. Left plain on purpose — this is a token gap, not an
// oversight, and the call site says so too. Every OTHER outermost heading
// in the app is a badge; if a future pass adds a real "badge on a status
// surface" tone pair, this is the one site waiting for it.
//
// RESOLVED, exempted (GlimStone follow-up pass, folded into v8.0.0): the 11
// page titles (the `<h1 text-2xl font-semibold>` at the top of each page,
// plus Recovery.tsx's `text-lg` one) were filed by the ORIGINAL Task 5 pass
// as "belongs to the same future rule-15 pass as the dialogs" — that framing
// doesn't survive actually reading rule 15's text. Rule 15 opens "A WINDOW
// is a window" and every clause under it (surface, radius, elevation, a
// button row at the bottom, an anchored title with only the middle region
// scrolling) describes MODAL chrome specifically — a floating, elevated
// card with its own button row. A page's root <h1> isn't inside a window by
// this app's own vocabulary (rule 1: `.glim-card` is the one elevation, and
// a page's own top-level content sits on the page ground, not raised); rule
// 15 has nothing to say about it either way.
//
// What DOES govern an <h1> is rule 2 ("one hero per page — exactly one
// element carries weight, everything else is supporting detail") plus the
// type scale's own explicit carve-out: "a page's one hero figure (rule 2) is
// sized on its own merits case by case, not from a repeated 'display' step —
// forcing every hero into one fixed size would fight the same rule that says
// there's exactly one hero and everything else is quiet." Each of these 11
// `<h1>`s IS that page's one hero (the single largest, heaviest text on the
// page, everything else — including every rule-11/rule-15 badge on the same
// page — deliberately quieter than it). Converting it to the SAME
// `tone="heading" size="heading"` Badge every section heading and dialog
// title now uses would make the page's one hero visually indistinguishable
// from the section badges nested underneath it, which is exactly the
// "everything else is quiet, exactly one thing carries weight" hierarchy
// rule 2 exists to protect — collapsing that hierarchy would be a rule-2
// regression dressed up as rule-15 compliance.
//
// So: NOT converted, and this is a considered exemption, not a re-deferral.
// All 11 stay exactly as they render today (`text-2xl font-semibold` /
// Recovery.tsx's `text-lg font-semibold` — already "sized on its own
// merits," per the type scale's own words, and already the loudest thing on
// each page). If a future page ever grows a genuinely bigger hero element
// (a stat figure, a chart headline) that outweighs its own <h1>, that page's
// <h1> would need re-examination on its own merits — but no page in this
// app has that shape today, so there's no live case to design against.
// ---------------------------------------------------------------------------

import type { CSSProperties, ReactNode } from "react";
import { hueVars, rainbowAt } from "../lib/appearance";
import { IconTipButton } from "./IconTipButton";

export type BadgeTone = "ok" | "fail" | "warn" | "active" | "neutral" | "heading";
export type BadgeSize = "small" | "medium" | "large" | "heading" | "icon" | "field";
// Four shapes per the design language's Badges section: pill (fully round,
// standalone chips/count badges), rounded (small fixed radius, compact
// inline badges — the default, matching every predecessor's rounded-control),
// square (a square-ASPECT icon badge — see `iconOnly`/`tip` — that still
// tracks the shape engine's own --radius-control token, exactly like
// `rounded` below; kept as its own named value purely so a call site reads
// as "this is a square icon tile" rather than "this is a rounded text chip",
// not because the two resolve to different CSS), circle (pill radius again
// but width locked to height, for icon-only/single-glyph badges — same
// radius as pill, distinct semantic use). Deliberately NOT the percentage-
// capped `min(var(--radius-pill), 50%)` formula: a CSS percentage
// border-radius resolves per-axis into an ellipse, not a stadium, so the
// plain length-based `rounded-pill` token (which already auto-scales
// correctly against this component's own fixed per-stage heights) is used
// as-is, with zero cap needed.
//
// FIXED (GlimStone follow-up round, jdp's live review of the off-site tab's
// four square icon badges — "nicht in der Formengine... die sind falsch
// eingefärbt"): `square` used to hard-code `rounded-none` (literal 0px
// corners, ALWAYS, in every shape-engine mode) — see RADIUS_CLASSES below.
// That was a real shape-engine miss, the SAME mistake pattern as if a
// Selector or any other control ignored `--radius-control`: with the app's
// shape engine set to "soft" or "round", these four badges stayed hard
// square instead of picking up the user's own chosen corner roundness,
// because the doc comment above (kept, now corrected) conflated "this
// component's own shape option named square" with "the global shape
// engine's square PRESET" and hard-coded the latter's value (0) instead of
// reading the engine's live token. Fixed by pointing `square` at the exact
// same `rounded-control` class `rounded` already uses — a square-aspect
// badge now renders 0 corners in "square" mode, genuinely rounded corners in
// "soft"/"round" mode, verified live with getComputedStyle against all three
// shape-engine settings.
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
// active replaces the old "info" tone (GlimStone form-engine Phase 2 Task 7,
// design-language.md rule 4: "four state hues... never a fifth" — blue
// "info" was a de-facto fifth hue, not a real state). Every call site that
// used tone="info" for genuine, currently-happening activity (Dashboard's
// statusTone: a run whose status is literally "running") now uses this
// instead — --accent-soft wash + accent-derived text, the SAME soft/tinted
// register every other tone here already uses, deliberately not a solid
// accent fill: Dashboard's run-history list can legitimately show several
// "running" rows at once (independent domains backing up concurrently), and
// rule 3 caps SOLID accent at one thing per page — a soft chip carries no
// such cap, it reads at the same visual weight as an ok/fail/warn chip
// sitting next to it. Sites that meant something else entirely (pure
// informational prose, a link/action badge) did NOT become "active" — those
// use --status-neutral-*/tone="neutral" instead, in its pre-existing role as
// this file's generic "no real status, muted default" tone (the same role
// BadgeProps' own `tone = "neutral"` default already plays, per that prop's
// doc comment below) — NOT rule 4's literal "skipped/waiting" state
// semantics, which is specifically what the file header's tone="heading"
// reasoning above declines to borrow for a structural element that isn't a
// status at all. Same word, two pre-existing and non-conflicting jobs; see
// index.css's TASK 7 comment for exactly which "→ neutral" sites landed here
// and why.
//
// text-accentText, not the flat text-accent: a spec-compliance review
// measured the flat accent gold at only 1.50:1 against this exact
// accent-soft-tinted background in light theme (WCAG needs 4.5:1 for text;
// dark theme measured fine). This is the identical failure mode
// --field-focus-ring already solved once for this same accent hue (flat
// accent gold has no contrast on a light surface, so light theme needs a
// separate, darker value) — text-accentText (--accent-text, see index.css)
// applies that same fix here rather than inventing a new mechanism.
const TONE_CLASSES: Record<BadgeTone, string> = {
  ok: "bg-statusOkBg text-statusOk",
  fail: "bg-statusFailBg text-statusFail",
  warn: "bg-statusWarnBgStrong text-statusWarn",
  active: "bg-accentSoft text-accentText",
  neutral: "bg-carbon-surface2 text-carbon-textSub",
  // See the file header's long-form reasoning (REVISED, live-review round —
  // "the notch reads as darkened/dimmed, not the real accent colour"): the
  // full, solid `--accent` fill + `--accent-contrast` ink, the same pairing
  // `navActive`/the "Speichern" button use, not a translucent or
  // opaque-composited wash of it. Used unconditionally for every
  // tone="heading" badge, at any size — badgeClassName's own isHeadingNotch
  // branch below no longer swaps in a different fill for the notch
  // specifically; this single entry now covers both cases identically.
  heading: "bg-accent text-accentContrast",
};

const RADIUS_CLASSES: Record<BadgeShape, string> = {
  pill: "rounded-pill",
  // square: shape-engine token, not a literal 0 — see BadgeShape's own doc
  // comment above for the live bug this fixes (a square-aspect badge used to
  // stay hard-square in every shape-engine mode; now it reads the SAME
  // --radius-control token `rounded` does, so "soft"/"round" genuinely
  // rounds its corners and "square" still resolves to 0 via that token's own
  // [data-shape="square"] value).
  square: "rounded-control",
  rounded: "rounded-control",
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
  // GlimStone follow-up pass, live-review round 3 point 3: the rainbow
  // palette editor's reset control needed to sit at the exact same 28px
  // footprint as its own PaletteSwatch neighbours (Settings.tsx, h-7 w-7) —
  // none of the three existing status-chip stages (18/20/24px) hit that, and
  // "heading" (22px) is a different visual register entirely (a section
  // title, not a row-level control). This is the first live call site for
  // shape="circle" (previously type-only, see BadgeShape's own comment) — an
  // icon-only glyph badge, sized to match a same-row swatch rather than a
  // text stage. text/padding are unused whenever shape="circle" (that branch
  // always overrides to px-0, and there is no visible text), but are filled
  // in anyway for interface completeness / a future non-circle "icon" call.
  icon: { height: "h-7", minHeight: "min-h-7", text: "text-dense", padding: "px-1" },
  // GlimStone follow-up round (jdp, live review of the off-site tab: "Können
  // wir die Buttons in quadratische Badges mit Glyphen umwandeln?" — Test
  // connection/Replicate now/Setup/Add-target's four `tone="active"` text
  // badges become square icon-only glyph badges). NOT a reuse of the existing
  // `icon` stage above (28px) — that value is pinned to a DIFFERENT sibling's
  // own footprint (the rainbow palette editor's PaletteSwatch, h-7 w-7) and
  // this session's own documented cautionary lesson (Selector.tsx's own
  // `iconOnly` doc, the `--badge-md` mismatch history) is explicit that
  // reusing a token from a DIFFERENT context without re-measuring is exactly
  // how a visible height mismatch slips in. Measured live instead, via
  // getComputedStyle against THIS row's own real control — the off-site
  // repo-url `<input>` (`px-3 py-2 text-sm`, Settings.tsx) — which renders at
  // 36px, not the 32px FolderBrowser/Registries' OWN `px-3 py-1.5` fields
  // measure to (a different padding value, a different real number; see
  // Selector.tsx's own `iconOnly` doc for that other, already-correct 32px
  // case — the two are neighbours, not the same stage). h-9 is Tailwind's
  // plain default spacing scale, step 9 = 2.25rem = 36px, not a new bracket
  // invented for this call site, the same "plain scale step, not an
  // arbitrary value" discipline FolderBrowser's own h-8 used for its number.
  field: { height: "h-9", minHeight: "min-h-9", text: "text-dense", padding: "px-3" },
};

interface BadgeStyleOptions {
  tone?: BadgeTone;
  size?: BadgeSize;
  shape?: BadgeShape;
  wrap?: boolean;
  className?: string;
  /** Zero horizontal padding + locked 1:1 aspect ratio, the same treatment
   *  `shape="circle"` has always applied below (see `isIconOnly`) — generalised
   *  here (GlimStone follow-up round, the off-site tab's four icon-only
   *  conversions) so a `shape="square"` glyph badge gets the identical
   *  padding/aspect fix a `shape="circle"` one already had, rather than
   *  leaving `square` to render as a wide rectangle sized for TEXT it no
   *  longer has. Not exposed on the public `BadgeProps` — Badge()'s own body
   *  derives it FROM `tip` being set (an icon-only glyph is exactly what a
   *  `tip`-carrying Badge always is; see that prop's own doc), so this stays
   *  a private plumbing detail of badgeClassName rather than a second public
   *  toggle a caller could set inconsistently with `tip`. */
  iconOnly?: boolean;
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
  iconOnly,
}: BadgeStyleOptions = {}): string {
  const { height, minHeight, text, padding } = SIZE_TOKENS[size];
  // circle is icon/glyph-only: zero horizontal padding + a locked 1:1 aspect
  // ratio against the stage's own height turns it into a true circle rather
  // than an oval widened by the stage's normal text padding. `iconOnly`
  // (GlimStone follow-up round — see BadgeStyleOptions' own doc) extends the
  // identical fix to a `shape="square"` glyph badge: same zero-padding/
  // locked-aspect need, a different final silhouette (the shape's own
  // RADIUS_CLASSES below still decides square vs. circle vs. pill/rounded —
  // this only ever governs padding/aspect, never the corner radius).
  const isIconOnly = shape === "circle" || iconOnly === true;
  // wrap swaps the fixed one-line `h-*`+`leading-none`+`min-h-0` sizing for a
  // `min-h-*` floor + real vertical padding + normal line-height, so a
  // second wrapped line grows the box instead of overflowing it — see the
  // file header. Never emit both `height` and `minHeight` (same CSS
  // property, same specificity — exactly the two-conflicting-utilities
  // hazard the padding/circle split above already guards against).
  const sizing = wrap
    ? `${minHeight} py-0.5 leading-tight wrap-break-word`
    : `${height} min-h-0 leading-none`;

  // tone="heading" + size="heading" (GlimStone follow-up pass, live-review
  // round — "half-overlap card notch"): every real call site pairs these two
  // props together (verified — no call site uses one without the other), so
  // gating on BOTH is the precise, self-documenting condition for "this is
  // the notch treatment," not just "any heading-sized or heading-toned
  // badge" in isolation.
  //
  // jdp disliked the old inline filled-badge-inside-the-content-flow look
  // (this same accent-soft chip, just sitting in normal document flow) and
  // asked for CannonadeCommand's fieldset/legend-style tab instead: the
  // badge now straddles ITS OWN CARD's top edge, half poking above the
  // card's visual boundary, half overlapping onto it — a notch, not a
  // heading floating inside the content area. BombVault's Cards are plain
  // divs (no real <fieldset>/<legend>), so this reproduces that look
  // synthetically with `position: absolute` on the badge + `position:
  // relative` on the card (added at each call site — see those call sites'
  // own comments for why a shared Card component doesn't exist here to
  // carry that in one place).
  //
  // top: -11px is exactly HALF of this stage's own real height — 22px
  // (`h-[22px]` in SIZE_TOKENS.heading above) — not CC's own 26px/13px pair,
  // which belongs to a differently-sized badge on a different app. Assumes a
  // single text line: `wrap` (used defensively at every heading call site,
  // see the file header) lets a badge grow past 22px for a genuinely long
  // string, and a badge that actually wraps to two lines pokes out more than
  // exactly half — accepted as a known, rare-in-practice tradeoff (checked
  // against this app's longest heading strings across all 26 locales: short
  // section-title phrases that fit on one line at this stage's roomy px-3
  // padding, except the one narrow sm:grid-cols-3 cell that already
  // documents wrapping as expected in the DEFAULT locale too) rather than a
  // JS-measured dynamic offset for what a fixed CSS value already covers in
  // the overwhelming common case.
  //
  // Deliberately no explicit left/right/start/end offset: with both left and
  // right left `auto`, an absolutely positioned box falls back to its CSS
  // "static position" — where it would have rendered had position stayed
  // static. Every call site wraps Badge in a `flex items-center` <h2> (or,
  // per this project's RTL-positioning convention — see FilterPopover.tsx
  // and Settings.tsx's dropdown menu — the logical `start`/`end` pair is how
  // this app expresses direction-aware offsets elsewhere), so the static
  // position is the flex container's own start edge: it automatically
  // inherits whichever padding the real call site's card already uses (p-5
  // here, p-4 there, zero on a few bare group-label headings, or wherever a
  // heading sits deeper in a decorated row — StepCard.tsx's numbered-circle
  // row, for one) with zero per-call-site horizontal class needed, AND it is
  // automatically RTL-correct (a flex row's start edge is the row's RIGHT
  // edge under dir="rtl") for the same zero-extra-classes reason — the
  // static-position fallback is direction-aware by definition, so this
  // extends the RTL sweep's own logical-property convention without
  // needing to repeat it as literal start-N classes at 20+ call sites.
  //
  // z-10: only needs to draw above this SAME card's own in-flow content
  // immediately below it (default z-index:auto siblings don't establish
  // their own stacking order); nothing here reaches for a shared app-wide
  // z-index scale (dialogs/backdrops sit at z-50, popovers at z-20/z-50 —
  // this badge never needs to out-rank those, it always renders inside its
  // own card's local stacking, never competing with a DIFFERENT card or a
  // dialog above it).
  //
  // rounded-pill (not this stage's usual shape-engine-driven radius, which
  // is what plain `RADIUS_CLASSES[shape]` below would otherwise resolve to
  // for the default shape="rounded" every heading call site actually
  // passes): a fixed pill regardless of the app's round/soft/square
  // shape-engine setting, matching CC's own reference implementation. A
  // card-edge notch reads as its own fixed piece of window chrome — like a
  // physical tab cut into the edge — not a data control the shape engine is
  // meant to govern; the shape engine's whole point is reshaping CONTROLS
  // (buttons, fields, swatches), and a heading was never one of those even
  // before this pass (rule 11 exempts headings from rule 9's reactive-colour
  // treatment for the identical reason — a heading isn't a control).
  //
  // shadow: var(--elevation) only, not this app's usual elevation+hairline
  // pairing (`.rounded-card`'s own box-shadow: var(--elevation),
  // var(--hairline) — see index.css): CC's own reference snippet specifies
  // a single box-shadow (the elevation lift that reads as "this sits above
  // the surface, not flush with it"), and --hairline is this app's
  // border-emulating inset highlight for a surface's OWN edge — a different
  // concern from "this floats above a surface," and one this same
  // tone="heading" treatment never reached for even before this pass (a
  // heading badge is furniture riding on the card, not a card boundary of
  // its own).
  const isHeadingNotch = tone === "heading" && size === "heading";
  const notchPositioning = isHeadingNotch ? "absolute -top-[11px] z-10 shadow-[var(--elevation)]" : "";

  // No per-notch colour override anymore (see the file header's tone=heading
  // section for the two earlier rounds this went through — a translucent
  // wash, then an opaque color-mix() of that same wash — and why jdp's
  // latest live review replaced both with the plain, full-strength
  // TONE_CLASSES.heading fill below). A heading-toned badge renders the
  // identical solid accent fill whether or not it's also the notch size.
  //
  // isIconOnly && tone==="active" is the one exception to reusing
  // TONE_CLASSES verbatim.
  //
  // ROUND 1 (GlimStone follow-up round, jdp's live review of the converted
  // off-site buttons, emphatic and specific: "Die Buttons ... haben farbige
  // Schrift" — the coloured TEXT itself, not the tinted background wash, was
  // the complaint). A text badge's `text-accentText` ink is only legible
  // BECAUSE it carries the hue — there is no text left to read once the
  // content is a bare glyph, so that reasoning no longer applies, and
  // design-language's own established "icons carry no colour of their own,
  // only the badge does" rule (already the reason IconAdd/IconTrash render
  // `currentColor` inside a plain `text-carbon-textSub` button, Settings.tsx's
  // Registries card) took over instead: `text-carbon-textSub`, the same
  // neutral ink every other icon-only badge in the app already reads its
  // glyph colour from — riding on the UNCHANGED hued `bg-accentSoft` wash
  // underneath it.
  //
  // ROUND 2, THIS FIX (jdp's next live-review round, on those same four
  // badges: "die sind falsch eingefärbt, so halb abgedunkelt, das soll nicht
  // so sein"): `bg-accentSoft` IS a 14%-alpha wash (`--accent-soft: rgba(...,
  // 0.14)`/`--item-hue-soft` — see index.css/appearance.ts) — the EXACT same
  // "half-darkened" failure mode this file's own tone="heading" section
  // documents fixing once already (a translucent accent-into-surface wash
  // reads as pale/dimmed no matter its alpha, full stop). Round 1 only ever
  // touched the TEXT colour and explicitly left "the wash stays" — but the
  // wash was the actual bug jdp is now naming directly. Fixed the same way:
  // drop the wash, use the full solid `bg-accent` fill instead — which,
  // exactly like tone="heading", now needs a computed-contrast ink rather
  // than a flat neutral one (`text-carbon-textSub` was only ever safe against
  // a PALE 14%-wash background; measured live against the RAINBOW palette's
  // own solid hues once the fill went opaque, several dropped to ~1.0–1.8:1
  // in dark theme and ~2.5–4.9:1 in light theme — a real contrast failure,
  // the identical premise-no-longer-holds reasoning tone="heading" already
  // gives for why IT moved off `text-carbon-textSub` onto `text-accentContrast`
  // when ITS fill went from wash to solid). `text-accentContrast` is still a
  // NEUTRAL ink in the sense the design-language rule actually cares about —
  // computed black/white (`contrastOn()`/`--item-hue-ink`), carrying no hue of
  // its own — it only stops being the specific token `text-carbon-textSub`,
  // which was tuned for a pale wash background this badge no longer has.
  // Every other tone/isIconOnly combination (a text chip, or an icon-only
  // badge of any OTHER tone — none exist live today, but nothing here assumes
  // otherwise) keeps the plain TONE_CLASSES lookup untouched.
  const toneClasses =
    isIconOnly && tone === "active" ? "bg-accent text-accentContrast" : TONE_CLASSES[tone];

  return [
    "inline-flex box-border items-center justify-center gap-1 font-medium",
    sizing,
    text,
    isIconOnly ? "px-0 aspect-square" : padding,
    isHeadingNotch ? "rounded-pill" : RADIUS_CLASSES[shape],
    toneClasses,
    notchPositioning,
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
  /** Accessible name for an icon-only badge (its visible content is a
   *  decorative aria-hidden glyph, so the element has no text of its own to
   *  compute a name from). Pass alongside `title` for an icon-only control —
   *  `title` alone gives a hover tooltip and would work as an accname
   *  fallback, but an explicit `aria-label` is the direct, unambiguous
   *  signal for assistive tech rather than relying on that fallback chain. */
  ariaLabel?: string;
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
  /** Rainbow position, by the caller's own LIST INDEX (never a hash) among
   *  the heading badges visible at once on the same page/tab — jdp's
   *  live-review override of this file's original "every heading gets the
   *  same flat fill" stance, see the file header's tone="heading" section
   *  for the full history. Meaningful on `tone === "heading"` AND
   *  `tone === "active"` (offsite-tab card-split follow-up — see the
   *  `hueOn` computation below for why "active" qualifies: it is the one
   *  other tone that is accent-derived rather than one of rule 4's four
   *  state hues). ok/fail/warn/neutral are load-bearing status signals
   *  `hueIndex` must never overwrite, so it is silently ignored for those.
   *  Omit for a genuine singleton (the only hue-eligible badge on its
   *  page/tab) — that one keeps the flat, un-rainbowed accent, per
   *  design-language's own "the only one of its kind on the page keeps the
   *  single accent" exclusion. */
  hueIndex?: number;
  /** `as="button"` only. Real hover/focus `.glim-bubble` tooltip AND the
   *  button's accessible name, rendered through IconTipButton instead of a
   *  plain `<button>` — GlimStone follow-up round, the off-site tab's four
   *  action buttons converting from short-text badges to icon-only glyph
   *  badges (jdp: "Können wir die Buttons in quadratische Badges mit Glyphen
   *  umwandeln?" — the text is gone, so an icon-only trigger needs the same
   *  real tooltip every other icon-only control in this app already gets,
   *  not the `title`/`ariaLabel` pair above, which is only a native OS
   *  balloon + a silent accessible name with no visible hover affordance —
   *  IconTipButton.tsx's own header comment is explicit that a stray native
   *  `title=` on an icon-only trigger is exactly the anti-pattern that file
   *  exists to replace). Presence of `tip` is ALSO what marks this Badge as
   *  icon-only for sizing purposes — see badgeClassName's own `iconOnly`
   *  option: a `tip`-carrying Badge has no visible text by definition (an
   *  icon-only trigger has nothing else a name/tooltip could come from), so
   *  there is no real case where a caller would want `tip` set AND the
   *  normal text-chip padding at the same time. When set, supersedes
   *  `title`/`ariaLabel` for accessible-naming purposes (IconTipButton sets
   *  its own `aria-label`); `title`/`ariaLabel` stay meaningful on every
   *  OTHER `as` value and every button call site that doesn't pass `tip`. */
  tip?: string;
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
}: BadgeProps) {
  // Deliberately NO useRainbow() subscription here, unlike Selector's own
  // identical-looking hue support: Badge (like Toggle) is a pure, hookless
  // function component by established convention (see Badge.test.ts's own
  // header comment — "invoked directly as a plain function... no jsdom/
  // testing-library needed"), and a first attempt at this DID add the hook,
  // which broke that entire suite (and Settings.tsx's own ToggleRow tests)
  // with "Cannot read properties of null (reading 'useSyncExternalStore')" —
  // calling a component as a plain function outside React's reconciler gives
  // hooks no dispatcher to run against. Reverted; hueVars()/rainbowAt() below
  // are plain functions reading module state, not hooks, so they still
  // resolve correctly at render time with zero subscription. The live-repaint
  // job this would otherwise cover is instead already handled by the ONE
  // caller that actually edits rainbow settings: Settings.tsx's own
  // SettingsPage() holds the Rainbow Card's state in a plain useState() in
  // THAT SAME component (see its own `rainbow`/`setRainbowLocal`), so
  // flipping the mode there already re-renders every Card/ToggleRow hueIndex
  // call site on the page — no separate subscription needed for the one real
  // place this value can change while the page showing it is mounted.
  // EXTENDED (offsite-tab card-split follow-up, jdp: "Die Buttons Verbindung
  // testen, Jetzt replizieren, Einrichten, Ziel hinzufügen in die Farbengine
  // aufnehmen" — wire those four per-domain action buttons into the colour
  // engine too): `tone === "active"` now qualifies alongside `"heading"`.
  // This does NOT weaken the "never overwrite a load-bearing status signal"
  // rule the comment above `hueIndex` explains — "active" was never one of
  // rule 4's four state hues in the first place (see this file's own
  // TONE_CLASSES/file-header history: it replaced the old "info" tone
  // specifically BECAUSE blue "info" was a de-facto FIFTH hue, not a real
  // state — "active" already means "accent-derived, no real status", the
  // exact shape a rainbow position is allowed to colour). ok/fail/warn/
  // neutral are untouched by this gate and stay hue-immune. A real per-
  // domain call site (Settings.tsx's TestConnectionButton/ReplicateNowButton/
  // offsite-wizard-toggle, OffsiteTargetsSection's "Ziel hinzufügen") passes
  // a genuine hueIndex on tone="active" buttons and was verified live with
  // getComputedStyle: each domain's own background-color matches that
  // domain's own RAINBOW[i], not a flat accent.
  //   KNOWN LIMITATION carried over from index.css's own --accent-text
  // comment, not newly introduced here: LIGHT theme's --accent-text is a
  // STATIC #7a5c00 (not `var(--accent)`, unlike dark theme), and the
  // [data-rainbow] .glim-hue rebind block never redeclares --accent-text —
  // only --accent/--accent-soft/--color-accent* — so a hued tone="active"
  // badge's BACKGROUND (bg-accentSoft -> --item-hue-soft) shifts per position
  // in both themes, but its TEXT stays the flat, gold-calibrated
  // --accent-text colour in light theme specifically. Real, but the same
  // already-accepted class of gap index.css's own "warn/active read as the
  // same amber" comment documents for this exact token — not a new hole this
  // change opens, and background-colour alone is enough for the position to
  // read as genuinely different per domain (verified live).
  const hueOn = hueIndex !== undefined && (tone === "heading" || tone === "active");
  // A hue-enabled heading badge is always the notch treatment (badgeClassName's
  // own isHeadingNotch above gates the SAME tone==="heading" && size==="heading"
  // pair — every real call site already pairs them, see that comment) — but
  // this is computed again here, explicitly, rather than trusted as an
  // established invariant: `.glim-notch-hue` below is a load-bearing selector
  // hook (index.css's card-wide reactive-hover rule keys off it specifically,
  // not the general `.glim-hue` every rainbow-hued element carries), and a
  // future call site that ever passed `hueIndex` without `size="heading"`
  // must NOT silently pick up that card-wide reveal too.
  const isNotchHue = hueOn && size === "heading";
  // `tip !== undefined` marks this Badge icon-only for sizing purposes too —
  // see badgeClassName's own `iconOnly` doc and BadgeProps' own `tip` doc for
  // why the two are the same condition rather than two props a caller could
  // set inconsistently.
  const shared = badgeClassName({ tone, size, shape, wrap, className, iconOnly: tip !== undefined });
  const merged = hueOn ? `glim-hue ${isNotchHue ? "glim-notch-hue " : ""}${shared}` : shared;
  const hueStyle = hueOn ? (hueVars(rainbowAt(hueIndex)) as CSSProperties) : undefined;

  if (as === "button") {
    const buttonClassName = `appearance-none transition-opacity hover:opacity-80 disabled:opacity-50 disabled:hover:opacity-50 ${merged}`;
    // `tip` renders through IconTipButton instead of a plain <button> — see
    // BadgeProps' own `tip` doc for why (a real hover/focus tooltip an
    // icon-only trigger needs, not the title/aria-label pair below, which
    // stays the plain-button path's own accessible-naming mechanism for
    // every call site that doesn't pass `tip`).
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
