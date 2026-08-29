// ---------------------------------------------------------------------------
// Selector — the one horizontal selector (GlimStone form-engine Phase 2, Task
// 3; design-language.md, "The one horizontal selector": "Tabs, filter bars,
// segmented controls and the corner-style picker are the same thing... Build
// it as ONE component, not a bespoke set of buttons per picker — a second,
// hand-rolled selector drifts from the first one the moment either changes.")
//
// Replaces twelve hand-rolled copies of the same filled/unfilled segment row
// that had already drifted apart in small ways (radius token, padding,
// idle-text tone) across seven files: Settings.tsx's 7-tab strip and its
// drill-type toggle, Containers.tsx's SortControl/FilterControl/ChipFilter(x2),
// VMs.tsx's duplicate SortControl/ChipFilter(x2), Files.tsx's destChip,
// Dashboard.tsx's heatmap domain toggle, CadenceBuilder.tsx's cadence-mode
// pills + weekday pills, and SourceToggle.tsx. All twelve were native
// `<button>` elements, so Enter/Space already worked through ordinary browser
// button semantics; what none of them had was arrow-key/Home/End
// roving-tabindex navigation, which this component adds once, for all twelve
// at once.
//
// Modeled directly on knightloader/web/src/components/Tabs.tsx, a real,
// shipped app's own copy of this exact component (roving tabindex, arrow-key/
// Home/End navigation, RTL-aware direction, select="one"|"many"). Ported with
// deliberate differences from that reference:
//
//   1. `hue` (default true, opt-out). The reference hues every item
//      unconditionally with no opt-out at all. `hue={false}` skips both the
//      `.glim-hue`/`.glim-hue-icon` classes and the `hueVars()` inline style
//      entirely, so a strip carrying it never enters a rainbow subtree no
//      matter what the global rainbow setting is doing elsewhere on the page.
//        REVERSED (jdp, live-review, emphatic standing rule — "Es soll immer
//      alles in die Farb- und Formengine integriert werden!! IMMER!!"):
//      Dashboard.tsx's heatmap domain toggle and Settings.tsx's shape picker
//      both used to pass `hue={false}`, each justified by its own "this one
//      genuinely shouldn't compete for attention" reasoning at the time —
//      exactly the self-authored aesthetic exception jdp has now ruled out
//      categorically. Neither call site opts out any more; both rely on the
//      plain `true` default. The prop itself stays, because an escape hatch
//      exists for a genuine HARD TECHNICAL case — an item count that isn't a
//      stable, enumerable list position at all — never a taste call.
//        There is exactly ONE live consumer, and it is that technical case:
//      Recovery.tsx's StepDisclosure, step 3's two expander chips. A
//      SINGLE-item Selector has only position 0 to hue by, so the rainbow
//      would paint both chips RAINBOW[0] red inside a yellow StepCard and mean
//      nothing by it — there is no list for the colour to encode a position
//      in. This paragraph read "has no live consumer as of this pass", which
//      was true when written (d68d8995, 2026-08-23) and went stale two days
//      later when 362ae3ed added that chip.
//   2. `plain` (default false). None of BombVault's twelve call sites are
//      visually identical at rest: ten are toolbar "chips" that carry a
//      visible `bg-carbon-surface2` pill even when unselected (so they read
//      as clickable controls sitting among plain page content); the Settings
//      top tab strip and the Dashboard heatmap toggle are page-level/card-
//      header tabs that sit flush on the surface behind them until hovered
//      or selected. Both are genuine, pre-existing BombVault treatments —
//      picking only one would visibly change the other, which the task
//      brief explicitly warns against ("the new shared component shouldn't
//      introduce a visually different style"). SourceToggle is the one
//      exception this forced a real (small, positive) change onto: its
//      pre-migration markup put ONE shared `bg-carbon-surface2` behind the
//      whole two-button pill, relying on a wrapping box the "no wrapping
//      bar" rule now removes — so `plain=false` (the chip default) is given
//      to each segment individually here, since nothing else is left to
//      carry that visual weight once the shared wrapper is gone.
//   3. `disabled` (per-item and whole-strip). The reference has no concept
//      of a disabled segment (its own `dim` prop is opacity-only and stays
//      clickable). Files.tsx's destChip needs a real disabled destination
//      (no target path picked yet) and SourceToggle needs the whole strip
//      disabled mid-restore, so this is a genuine BombVault addition rather
//      than something dropped from the reference.
//   4. No `href`/anchor variant, no `badge` slot, no `after` slot, no
//      `activateOnFocus` override. None of BombVault's twelve call sites are
//      real links, show a count, need trailing content inside the strip, or
//      want arrow-key movement without selecting — carrying that surface
//      area over unused would just be dead code with no exerciser, so `auto`
//      (arrow-selects-as-it-moves) is hard-wired to `!many`, matching the
//      reference's own default for select="one" (a JTabbedPane-style tab
//      strip) without exposing a knob nothing here turns.
//   5b. `equalWidth` (default false; "chip"-only when introduced, both
//      variants since round 8 — see item 6). Live-review follow-up:
//      Settings.tsx's 7-tab strip is content-hugging chips (each badge only
//      as wide as its own label — "Allgemein" narrower than "Pfade &
//      Speicher") sitting in a `flex-wrap` row that itself renders at the
//      page's full width (the width-mismatch fix two rounds back removed the
//      Settings Cards' own width cap so they'd match this row's CONTAINER).
//      That fix matched the two elements' bounding boxes but not their
//      VISIBLE content: seven ragged, left-hugging pills leave a stretch of
//      bare gap to the right of the last tab ("System"), while the Card
//      below fills that same width edge-to-edge — a container-width match
//      that still looks wrong once actually rendered.
//
//      CORRECTED (jdp, live-review round 2, explicit): the first pass gave
//      every segment an identical `flex: 1 1 0%` share of the row — that
//      does make all seven equal, but by stretching them to fill whatever
//      width the row's ancestor happens to have, which is NOT what was
//      asked. jdp's own words: "Ich wollte die Tab-Badges nur so breit wie
//      sie breit sein müssen. Also alle so breit wie der
//      Benachrichtigungen-Badge weil das ist der breiteste" — every badge
//      should be exactly as wide as the WIDEST label's own natural content
//      width ("Benachrichtigungen"), no wider, and the row's own rendered
//      width should be the sum of that (now-fixed) per-segment width, not a
//      stretch to the container. That's a genuinely different number (a
//      content measurement, not a CSS percentage), so this now does a real
//      one-time DOM measurement pass instead of a flex trick — see the
//      `itemRefs`/`matchedWidth` state below. `flex: 1` is gone; every
//      segment instead gets an explicit `width: <widest>px` inline style, so
//      the row's own box naturally shrinks to hug that fixed sum (given a
//      caller that doesn't itself force the row to stretch — Settings.tsx's
//      tab-strip call site wraps this in its own `inline-flex` measuring
//      container for exactly that reason; see its own comment).
//
//      The Settings tab strip stays "chip" while taking this prop (it is
//      not a grooved "well" strip): the chip look it already has — each tab
//      its own individually filled/outlined badge, not segments inside a
//      shared padded groove — is what round 6 specifically built ("idle
//      badge fill" for unselected tabs); switching it to `well` would trade
//      that identity away to solve a width problem `well` doesn't uniquely
//      solve. `equalWidth` is orthogonal to `variant` for exactly that
//      reason, and round 8 (item 6) made that literal — it is now the
//      pinning knob for BOTH variants.
//
//      `flex-wrap`, NOT `flex-nowrap` (caught live, not in the harness —
//      the first cut of this correction shipped `flex-nowrap`, copying
//      `well`'s reasoning verbatim without re-checking whether it still
//      applied; round 8 then found the same reasoning was never sound for
//      `well` either and dropped `flex-nowrap` there too). A pinned segment
//      set is a FIXED
//      pixel width (the widest label's own measured content size, floored at
//      MIN_PINNED_WIDTH), a number
//      with NO relationship to whatever width happens to be available — 7
//      German labels at this stage's own padding sum to ~1243px, which
//      genuinely does not fit inside every real viewport (measured live:
//      overflowed a 1400px-wide window's ~1113px available column, spilling
//      "System" straight past the page's right edge and forcing a page-wide
//      horizontal scrollbar — a real, visible regression, not a theoretical
//      one). `flex-wrap` here is simply Selector's own general "wraps,
//      never scrolls" rule (this file's header) applying to a pinned strip
//      the same way it already applies to every plain "chip" call site — a
//      6-tabs-then-1 wrap is a plainer fallback than jdp's ideal, but a
//      wrapped SECOND ROW beats spilling content off the edge of the page.
//   5c. MIN_PINNED_WIDTH (round 3, live-review refinement on top of 5b/5):
//      "Die horizontalen Selektoren bitte breiter und möglichst gleich
//      breit." Every pinned strip (`equalWidth`, either variant — see item
//      6) used to pin ONLY to its own widest label's own natural
//      width — correct per-strip, but with nothing shared ACROSS strips, so
//      the tab strip (174px, driven by "Benachrichtigungen"), the Theme
//      Card's well track (97px, "Dunkel" + icon) and the Shape Card's well
//      track (62px, "Leicht") each rendered a different, narrow number. A
//      shared floor (see MIN_PINNED_WIDTH's own doc, near SIZE below) folds
//      into the SAME widest-segment Math.max used for the truncation fix
//      instead of a separate clamp, so it stays a floor, never a cap — a
//      future locale whose longest label needs more than that floor still
//      gets exactly that larger number. Scoped to every EXISTING pinWidth
//      call site (all three are "lg") — see that constant's own doc for why
//      a per-size table isn't warranted yet.
//   5. `variant` ("chip", default, vs "well"). Live-review follow-up: "turn
//      the shape picker into a horizontal selector styled like the one in
//      TrickWork." TrickWork's own segmentedRow() (ui/src/controlWidgets.ts)
//      is a shared padded track (the row itself carries background + 0.2rem
//      padding + `border-radius: var(--radius-control)`) holding flush,
//      equal-width segments with a crossfade-only active fill (a plain
//      120ms `background-color` transition — no sliding pill/thumb element,
//      no transform/left animation). That track treatment is `variant`,
//      applied ADDITIVELY on top of everything above: a "well" strip still
//      gets this component's roving tabindex, arrow-key/Home/End nav, RTL
//      direction read and disabled-segment handling verbatim — none of
//      which TrickWork's own version has (it hands each segment its own
//      independent Tab stop, no arrow keys, no ARIA tablist/radiogroup role,
//      no RTL awareness, no disabled concept at all). Only the VISUAL
//      treatment is ported; the more complete interaction model here is
//      kept, not regressed to match. `plain` is meaningless under
//      `variant="well"` (the well track answers the same "does an idle
//      segment carry its own background" question `plain` answers for
//      "chip", just differently — transparent-until-hover either way) and
//      is ignored whenever both are given. Started scoped to exactly one
//      call site (Settings.tsx's shape picker) — GlimStone's own "try it on
//      one control, generalize later if it lands well" pattern — and item 6
//      below is that generalization: it now carries every grooved selector
//      in the app, at both scales.
//   6. `variant="track"` — ADDED in round 7, REMOVED again in round 8. Round
//      7 added a third variant for the compact, repeated-per-card selectors
//      (NotifyCard's "on" row, the Integrity Card's drill-kind toggle, every
//      CadenceBuilder mode picker and weekday strip, ItemScheduleOverride).
//      It borrowed "well"'s enclosing groove but deliberately did NOT borrow
//      its idle-transparent segments: every idle segment kept its own
//      `bg-carbon-surface` fill, so the strip read as a groove holding keys
//      rather than as one slab with a single accent pill floating in it.
//        REVERSED (jdp, live-review round 8, explicit): "Die kleinen
//      Selektoren sollen so aussehen wie die grossen! Die nicht
//      ausgewaehlten Optionen sollen kein Badge sein." An individually
//      filled idle segment reads as its own badge, which is exactly what a
//      selector's UNSELECTED options must not do — in a selector only the
//      chosen one is a badge. So the small selector adopts the big one's
//      fill logic verbatim: idle segments `bg-transparent` against the
//      groove, only the active segment filled.
//        Once that landed, "track" and "well" differed in exactly two
//      things, and only one of them was a real concept:
//        (a) The pinning bundle — a measured, MIN_PINNED_WIDTH-floored width
//            on every segment, a fixed `--badge-md` height, centred content,
//            no wrapping. That is ONE idea ("every segment gets the same
//            standardized box"), and this file already had a prop for it:
//            `equalWidth`, driving the very same `pinWidth` measurement
//            pipeline for "chip" (item 5b).
//        (b) The groove token — `surface2` ("well") vs `surface3` ("track").
//            That was never a style choice: it answers "which surface token
//            is distinct from the parent I sit on", and only ONE token is
//            distinct from BOTH parents this control legally sits on (a Card
//            at `surface`, a CadenceBuilder well at `surface2`). Measured on
//            the running container, `surface3` is also the BETTER groove at
//            the Card parent "well" already had: groove-vs-parent 1.94:1
//            dark / 1.53:1 light at surface3 against 1.31:1 / 1.21:1 at the
//            old surface2 — unifying STRENGTHENS the big picker's own
//            enclosure rather than weakening it.
//        Two variants whose only remaining difference is a prop that already
//      exists are two chances to drift apart, and drifting apart is the
//      literal complaint this round is fixing ("die kleinen sollen so
//      aussehen wie die grossen"). So "track" is gone, there are two
//      variants again — "chip" and "well" — and `equalWidth` is promoted
//      from a "chip"-only knob to THE pinning knob for both:
//        - `variant="well"` alone = the SMALL selector: content-hugging
//          segments, height from padding + line-height, wraps, cheap to
//          repeat many times on one page. Every ex-"track" call site.
//        - `variant="well" equalWidth size="lg"` = the BIG selector: every
//          segment pinned to the widest one's own measured width or
//          MIN_PINNED_WIDTH, whichever is larger, at a fixed `--badge-md`
//          height. Settings.tsx's Theme, Shape and Motion pickers.
//      They cannot drift apart now: same branch, same groove token, same
//      idle/active fill, same crossfade transition, same radius. The only
//      difference left is the one the prop's own name states.
//        `w-fit max-w-full` moved onto the "well" row itself (it was
//      "track"-only): the groove hugs its own segments instead of stretching
//      to a block parent's full width. `width: fit-content` is not `auto`,
//      so it ALSO opts the row out of a flex column's default
//      `align-items: stretch` — which is exactly what the three big call
//      sites used to hand-roll as their own `inline-flex self-start
//      max-w-full` wrapper div. Those three wrappers are removed in the same
//      pass: one mechanism inside the component beats the same mechanism
//      re-typed at three call sites, for the same reason this whole item
//      exists.
//        `flex-wrap`, not the old "well"-only `flex-nowrap`: a pinned
//      strip's width is the sum of N x max(widest label, 200px), a number
//      with no guaranteed relationship to the available width — the same
//      overflow "chip"'s own `equalWidth` already wraps for (item 5b, where
//      `flex-nowrap` really did spill a 7-tab strip past the page edge).
//      Wrapping to a second row beats spilling off the edge.
//        Idle-label contrast is what pays for the groove move, and it is the
//      one real cost of this unification — stated here rather than buried:
//      an idle segment's `text-carbon-textSub` now sits on the surface3
//      groove instead of on a `bg-carbon-surface` key (ex-"track") or a
//      surface2 groove (ex-"well"), measuring 4.57:1 dark / 5.12:1 light,
//      down from 8.85:1 / 7.82:1 and 6.76:1 / 6.44:1 respectively. Both
//      still clear WCAG AA for normal text (4.5:1), but the dark figure
//      clears it by 0.08 — this is the floor, not somewhere to spend any
//      further contrast, so a future "let's deepen the groove one more step"
//      needs a text-token change in the same commit. Hover is the other side
//      of that same trade and gets markedly BETTER: `hover:bg-carbon-hover`
//      against the groove goes from a near-invisible 1.06:1 dark / 1.08:1
//      light on surface2 to 1.57:1 / 1.16:1 on surface3.
//        Applied to every call site both variants had: Settings.tsx's Theme,
//      Shape and Motion pickers (`equalWidth size="lg"`), NotifyCard's "on"
//      row, the Integrity Card's drill-kind toggle, and CadenceBuilder's own
//      mode picker AND weekday strip. See design-language.md's "The one
//      horizontal selector" section for the when-to-use-which rule.
//
// No wrapping container (design-language.md: "the container is gone
// entirely, the gap alone carries the separation") — this renders a bare
// `role="tablist"|"group"` flex row with no background, no padding, no
// border of its own. A caller's leading caption ("Sort by:", "Filter:")
// stays a plain `<span>` OUTSIDE this component, exactly as every migrated
// call site already rendered it — NEVER a `<label>` wrapping the row (the
// spec's own documented trap: a `<label>` around several tabs hands its
// click to the first one, and announces that tab's name as the label's own
// name to a screen reader). Selector itself never renders inside a `<label>`
// and grepping every one of its twelve call sites confirms none of them do
// either (see the migration commit).
// ---------------------------------------------------------------------------
import {
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
  type KeyboardEvent,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import { hueVars, rainbowAt } from "../lib/appearance";
import { computeBubblePosition } from "../lib/bubblePosition";
import { useRainbow } from "../lib/useRainbow";
import { useLabelMode } from "../lib/useLabelMode";

export interface SelectorItem {
  /** Stable id — what onChange hands back. */
  id: string;
  label: string;
  /** Optional glyph before the label. Tinted by rainbow via `.glim-hue-icon`
   *  when `hue` is true (design-language's "top, with an icon" rule — the
   *  Settings tab strip is the one call site that actually carries these). */
  icon?: ReactNode;
  disabled?: boolean;
  /** Native tooltip — e.g. Files.tsx's "pick a target folder first" hint on
   *  a disabled destination chip. */
  title?: string;
  /** Hides the visible text label so only `icon` shows (PathModeSwitch's
   *  Local/Remote pair, GlimStone follow-up round point 2 — "replace the
   *  text labels with icon glyphs"). `label` still becomes the button's
   *  `aria-label` so the accessible name survives removing the visible text —
   *  design-language's own rule that a control's name must never depend on
   *  content that stops rendering. */
  iconOnly?: boolean;
  /** Hover/focus-revealed explanation, portal-rendered in the same
   *  `.glim-bubble` chrome InfoBubble.tsx's (i) icon already uses (not a
   *  second, differently-styled tooltip mechanism) — needed once `iconOnly`
   *  strips a segment of its own visible meaning and a bare `label`/`title`
   *  native tooltip isn't enough context on its own. Independent of `title`
   *  (a plain native attribute, still passed straight through unchanged). */
  tip?: string;
}

export type SelectorSize = "sm" | "md" | "lg";

/** "chip" (default) is every existing call site's own treatment — see the
 *  `plain` doc below for the two flavours of it. "well" is the app's ONE
 *  grooved horizontal selector, at either scale: a shared padded track
 *  (`surface3` groove) holding flush, crossfade-only segments that are
 *  TRANSPARENT while idle and filled only when chosen. Small by default
 *  (content-hugging, height from padding — NotifyCard's "on" row, the
 *  drill-kind toggle, every CadenceBuilder mode/weekday picker); add
 *  `equalWidth` for the big page-level scale (every segment pinned to the
 *  widest one's measured width or MIN_PINNED_WIDTH, at a fixed `--badge-md`
 *  height — Settings.tsx's Theme/Shape/Motion pickers). See the file
 *  header's item 5 for what is/isn't ported from TrickWork, and item 6 for
 *  why the round-7 "track" variant folded back into this one. */
export type SelectorVariant = "chip" | "well";

interface SelectorCommon {
  items: SelectorItem[];
  /** Accessible name for the strip, e.g. "Sort", "Settings sections". */
  label: string;
  /** `md` (default) is the dominant chip weight ten of the twelve migrated
   *  call sites share; `sm` is the tighter Dashboard heatmap toggle /
   *  CadenceBuilder weekday-pill weight; `lg` is the page-level Settings tab
   *  strip. Three named stages, one canonical source per instance — see
   *  Badge.tsx's own header comment for why a fourth ad-hoc size never gets
   *  to exist here. */
  size?: SelectorSize;
  /** Rainbow position per item, default true. See the file header for the rule
   *  and for the ONE live opt-out: Recovery.tsx's StepDisclosure, a SINGLE-item
   *  expander chip whose "position in the list" is therefore always 0 and
   *  carries no information — the hard-technical case the escape hatch exists
   *  for, never a taste call. Dashboard.tsx's heatmap toggle was named here
   *  until d68d8995 removed its opt-out; it passes no `hue` at all now. */
  hue?: boolean;
  /** Page-tab treatment (no idle background) instead of the default
   *  toolbar-chip treatment (idle `bg-carbon-surface2` pill). See the file
   *  header for which of the twelve call sites uses which. Ignored under
   *  `variant="well"` — that variant's own groove answers the same "does an
   *  idle segment carry a background" question unconditionally (it does
   *  not; the groove carries the weight). */
  plain?: boolean;
  /** "chip" (default) vs "well" — see the file header's items 5 and 6. */
  variant?: SelectorVariant;
  /** THE pinning knob, for BOTH variants (promoted from "chip"-only in
   *  round 8, file header item 6): pin every segment to the widest one's own
   *  MEASURED content width — floored at MIN_PINNED_WIDTH — instead of each
   *  hugging its own label width. See the file header's item 5b for why this
   *  is a real DOM measurement and not a `flex-1` share (that was the first
   *  cut's own bug, corrected there).
   *    Under `variant="well"` this is what separates the BIG page-level
   *  selector from the small repeated one, and it additionally pins every
   *  segment to the fixed `--badge-md` height with centred content — a
   *  uniform box needs a uniform height, not just a uniform width. Without
   *  it a "well" strip is content-hugging, exactly like the round-7 "track"
   *  variant it replaces.
   *    Default false: every pre-existing "chip" call site, and every small
   *  "well" call site, keeps its own content-hugging width. */
  equalWidth?: boolean;
  /** A fixed CSS width for every segment, replacing `equalWidth`'s live
   *  measurement (#178, [200]).
   *
   *  jdp asked for the Settings tab strip to join the button size system while
   *  staying "wirklich immer gleich breit". Measuring gets the second half
   *  right but not the first: the pinned width is whatever the widest
   *  translation happens to need, which is how this strip once grew to 1424px
   *  in German and wrapped onto two rows. A stage from lib/controls is decided
   *  before the first paint and is bounded, so the strip cannot surprise
   *  anyone in a language nobody tested. */
  segmentWidth?: string;
  /** Bumps the idle "chip" fill one step deeper — `bg-carbon-surface3`
   *  instead of the default `bg-carbon-surface2` (jdp, live-review: "Aus,
   *  Täglich, Wöchentlich, Alle N Tage, Cron sollen auch im nicht
   *  ausgewählten Zustand als Badge erkennbar sein"). Originally
   *  CadenceBuilder's own mode/weekday pills' fix for sitting inside their
   *  own `rounded-card bg-carbon-surface2 p-4` well (the default idle chip
   *  fill is the literal SAME token as that ambient background — an idle
   *  pill was genuinely indistinguishable from the card behind it until
   *  hovered or selected). SUPERSEDED at both of those call sites by
   *  `variant="well"` (file header item 6), which bakes a real enclosing
   *  groove in unconditionally instead of needing a caller to opt in —
   *  `raised` has no live consumer as of that round, same status as the
   *  `hue` prop's own documented zero-consumer state above.
   *  Kept as a lighter escape hatch for a future PLAIN "chip" call site that
   *  wants deeper idle contrast without paying for `variant="well"`'s full
   *  enclosing box. Ignored under `variant="well"`, which already answers
   *  this same question unconditionally and in the opposite direction — its
   *  idle segments are TRANSPARENT against their own groove, so there is no
   *  idle fill left to bump a shade deeper. Default false:
   *  every other "chip" call site
   *  (the Settings tab strip, ten toolbar chips) sits directly on a plain
   *  page/card background, not inside a surface2 well, so their existing
   *  idle fill already contrasts correctly and stays unchanged. */
  raised?: boolean;
  /** Disables every item (e.g. SourceToggle mid-restore). A per-item
   *  `disabled` still applies on top of this. */
  disabled?: boolean;
  className?: string;
}

export type SelectorProps =
  | (SelectorCommon & {
      select?: "one";
      active: string | null;
      onChange: (id: string) => void;
    })
  | (SelectorCommon & {
      select: "many";
      active: ReadonlySet<string>;
      onChange: (id: string) => void;
    });

// Split into gap/padding/text (rather than one combined string, the
// pre-`variant` shape of this table) so `variant="well"` can drop just the
// padding third — a well segment's box comes from the fixed --badge-md
// height below instead, per TrickWork's own segmented-button rule, which
// carries no padding of its own either — while still sharing the same
// gap/text-size values "chip" uses at each stage. `chip`'s own classes are
// the same three utility classes as before, just reassembled from these
// three fields instead of one literal string; reordering plain, independent
// Tailwind utility classes within a `className` doesn't change the
// generated CSS, so this is a pure refactor for that branch.
const SIZE: Record<
  SelectorSize,
  { gap: string; padding: string; text: string }
> = {
  sm: { gap: "gap-1", padding: "px-2 py-0.5", text: "text-xs" },
  md: { gap: "gap-1.5", padding: "px-3 py-1", text: "text-xs" },
  // "lg" is the page-level scale (Settings tabs, Shape/Motion/Labels pickers).
  // `bv-seg` gives it the button height instead of letting padding decide, so
  // the strip neither shrinks in glyph mode nor sits lower than a button.
  lg: { gap: "gap-2", padding: "px-3 bv-seg", text: "text-sm" },
};

// MIN_PINNED_WIDTH — the one standardized floor every `pinWidth` segment (both
// "well" and "chip"'s own `equalWidth`) now measures up to (jdp, live-review
// round 3, refinement on top of item 5b's own fix: "Die horizontalen
// Selektoren bitte breiter und möglichst gleich breit" — wider, and as
// equal-width as possible). Before this, `matchedWidth` pinned every strip to
// ONLY its own widest label's own natural content width — correct per-strip
// (item 5b fixed a real truncation bug doing exactly that), but with no floor
// shared ACROSS strips, so three genuinely different call sites rendered
// three genuinely different widths: measured live at "lg", 1400px viewport —
// the Settings tab strip's own segment (driven by "Benachrichtigungen", the
// single longest label across every one of the 26 shipped locales for that
// strip, already documented above) at 174px, the Theme Card's "well" segment
// (driven by "Dunkel" + its own 20px icon) at 97px, the Shape Card's "well"
// segment (driven by "Leicht", no icon) at 62px — each one narrower than the
// last, reading as three unrelated controls rather than one recurring
// pattern, and none of them "wide" by any absolute measure either.
//   200px is a round number chosen to comfortably clear the app's OWN current
// ceiling (174px) with real breathing room, not just barely clear it — every
// pinned segment in the app today (all three call sites are "lg") ends up
// pinned to this SAME 200px, which happens to make all three literally
// identical right now (200 > every one of 174/97/62), not merely "closer."
// `Math.max` below keeps this a FLOOR, not a cap: a future locale whose
// longest label genuinely needs more than 200px still gets exactly that
// larger measured width (the real bug item 5b fixed — silently truncating a
// genuinely-too-narrow segment — must never come back), it just never goes
// narrower than this. Deliberately a bare, size-independent constant rather
// than one entry per `SelectorSize`: every pinned segment in this app is "lg"
// today (see this constant's own three call sites), so a per-size table would
// be speculative generality with no second stage to size against yet — add
// one the moment a real "sm"/"md" pinned consumer exists, not before.
export const MIN_PINNED_WIDTH = 200;

// ---------------------------------------------------------------------------
// Pure, DOM-free navigation math — exported and unit-tested directly (see
// Selector.test.ts) without jsdom, matching this repo's established
// no-jsdom pattern for pure logic (Badge.test.ts, appearance.test.ts). Only
// the DOM-touching half (real focus movement, the RTL `getComputedStyle()`
// read) stays inside the component below; "what index comes next" is fully
// exercised here with plain numbers and booleans, no renderer required.
// ---------------------------------------------------------------------------

export type SelectorNavKey = "ArrowRight" | "ArrowLeft" | "Home" | "End";
const NAV_KEYS: readonly string[] = ["ArrowRight", "ArrowLeft", "Home", "End"];

/**
 * stepFor is which direction "further along the strip" moves in, for the
 * given key and reading direction. Right means "further along the strip",
 * which in Arabic and Hebrew is to the left — this is the one piece of
 * direction-awareness rule "The one horizontal selector" requires (RTL).
 */
export function stepFor(key: SelectorNavKey, rtl: boolean): -1 | 0 | 1 {
  if (key === "ArrowRight") return rtl ? -1 : 1;
  if (key === "ArrowLeft") return rtl ? 1 : -1;
  return 0; // Home/End jump rather than step
}

/**
 * nextFocusIndex is the roving-tabindex target for a keypress: Home/End jump
 * to the ends, an arrow key steps (wrapping around both ends), and a strip
 * with no current focus (current < 0, e.g. the very first keypress lands here
 * because nothing inside the strip was focused yet) starts at the first item
 * rather than stepping from an undefined position. Returns -1 for an empty
 * strip so the caller can no-op instead of focusing nothing.
 */
export function nextFocusIndex(
  key: SelectorNavKey,
  current: number,
  count: number,
  rtl: boolean,
): number {
  if (count <= 0) return -1;
  if (key === "Home") return 0;
  if (key === "End") return count - 1;
  if (current < 0) return 0;
  const step = stepFor(key, rtl);
  return (current + step + count) % count;
}

/**
 * rovedIndex is which item index holds tabIndex 0 (every other item is -1,
 * reachable only by arrow key once the strip itself has focus — "the strip
 * is ONE stop in the tab order"). Prefers the active, non-disabled item;
 * falls back to the first enabled item so a strip whose active id got
 * disabled out from under it (Files.tsx's "original" chip losing its target
 * path) still has a reachable Tab stop; falls back to 0 if every item is
 * disabled (nothing is actually reachable there, but the strip still needs
 * an index to anchor the (inert) tabIndex=0 to).
 */
export function rovedIndex(disabled: boolean[], activeIndex: number): number {
  if (activeIndex >= 0 && !disabled[activeIndex]) return activeIndex;
  const firstEnabled = disabled.findIndex((d) => !d);
  return firstEnabled >= 0 ? firstEnabled : 0;
}

// ---------------------------------------------------------------------------
// SelectorTab — one rendered segment (GlimStone follow-up round, PathModeSwitch
// icon-only tooltip point). Pulled out of Selector's own `.map()` into a real
// component, rather than inline JSX, specifically so `tip`'s hover/focus
// tooltip state below (useState/useEffect) is legal: each array item gets its
// OWN component instance this way, which is the ordinary "list of stateful
// components" shape — a hook called directly inside a `.map()` callback body
// would violate the rules of hooks the moment the item count ever changed.
// Every other prop is threaded straight through from Selector's own render
// loop unchanged; nothing here duplicates Selector's own selection/hue/sizing
// logic, so an item with no `tip` renders byte-identical output to before.
// ---------------------------------------------------------------------------
interface SelectorTabProps {
  item: SelectorItem;
  many: boolean;
  on: boolean;
  disabled: boolean;
  roved: boolean;
  className: string;
  style?: CSSProperties;
  onSelect: () => void;
  registerRef: (el: HTMLButtonElement | null) => void;
}

function SelectorTab({
  item,
  many,
  on,
  disabled,
  roved,
  className,
  style,
  onSelect,
  registerRef,
}: SelectorTabProps) {
  // #178: how much of a segment is shown follows the "tabs" axis, the same
  // shared setting the Settings tab strip and every other horizontal selector
  // obey, so the app has one answer rather than one per strip.
  const labelMode = useLabelMode("tabs");
  const [tipOpen, setTipOpen] = useState(false);
  const btnRef = useRef<HTMLButtonElement | null>(null);
  const bubbleRef = useRef<HTMLDivElement | null>(null);
  const tooltipId = useId();

  function showTip() {
    if (!item.tip) return;
    setTipOpen(true);
  }
  function hideTip() {
    setTipOpen(false);
  }

  // Positions the bubble (clamped into the viewport, flipped above the
  // trigger when opening below would clip the bottom edge) AFTER it has
  // mounted and laid out — same fix, same shared computeBubblePosition, as
  // InfoBubble.tsx's identical tooltip. This copy had the exact same bug:
  // centred on the trigger with no viewport clamp and always opening
  // downward, so an icon-only tab near a toolbar's edge (or a table's last
  // column) could push its tip half off-screen. See InfoBubble.tsx's header
  // comment for the full root-cause writeup.
  useLayoutEffect(() => {
    if (!tipOpen) return;
    const trigger = btnRef.current;
    const bubble = bubbleRef.current;
    if (!trigger || !bubble) return;
    const r = trigger.getBoundingClientRect();
    const viewport = {
      width: document.documentElement.clientWidth || window.innerWidth,
      height: document.documentElement.clientHeight || window.innerHeight,
    };
    const { left, top } = computeBubblePosition(
      r,
      { width: bubble.offsetWidth, height: bubble.offsetHeight },
      viewport,
    );
    bubble.style.left = `${left}px`;
    bubble.style.top = `${top}px`;
  }, [tipOpen]);

  // Same scroll-closes / Escape-closes contract as InfoBubble.tsx's own
  // tooltip — a floating box anchored to a live rect must not drift out of
  // position under the trigger it's supposed to be pointing at.
  useEffect(() => {
    if (!tipOpen) return;
    const onScroll = () => hideTip();
    const onKey = (e: globalThis.KeyboardEvent) => {
      if (e.key === "Escape") hideTip();
    };
    window.addEventListener("scroll", onScroll, true);
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("scroll", onScroll, true);
      window.removeEventListener("keydown", onKey);
    };
  }, [tipOpen]);

  // A DISABLED segment emits no mouse events and takes no focus, so its tip is
  // unreachable — and for an iconOnly segment the tip is the only thing that
  // names it, at exactly the moment (a short in-flight disable) when the user
  // wants to know why it is dead. The wrapper is not disabled and still sees the
  // pointer. Rendered ONLY when disabled, so the enabled strip's layout, which
  // this component measures for its own sliding indicator, is untouched.
  const wrapDisabled = (node: ReactNode) =>
    disabled && item.tip ? (
      <span
        className="inline-flex"
        onMouseEnter={showTip}
        onMouseLeave={hideTip}
      >
        {node}
      </span>
    ) : (
      node
    );

  return (
    <>
      {wrapDisabled(
        <button
          ref={(el) => {
            btnRef.current = el;
            registerRef(el);
          }}
          type="button"
          data-sel-id={item.id}
          role={many ? undefined : "tab"}
          aria-selected={many ? undefined : on}
          aria-pressed={many ? on : undefined}
          aria-label={item.iconOnly || (labelMode === "glyph" && item.icon) ? item.label : undefined}
          aria-describedby={item.tip && tipOpen ? tooltipId : undefined}
          title={item.title}
          disabled={disabled}
          tabIndex={roved ? 0 : -1}
          style={style}
          className={className}
          onClick={onSelect}
          onMouseEnter={item.tip ? showTip : undefined}
          onMouseLeave={item.tip ? hideTip : undefined}
          onFocus={item.tip ? showTip : undefined}
          onBlur={item.tip ? hideTip : undefined}
        >
          {/* #178: a segment shows what the "tabs" axis asks for. `iconOnly`
              is still honoured as the call site's own hard choice, and a
              segment with NO glyph keeps its text whatever the mode says,
              for the same reason a glyphless Button does: an empty segment
              is unusable, an inconsistent strip merely looks uneven. */}
          {labelMode !== "text" && item.icon}
          {(labelMode === "text" || !item.iconOnly) &&
            (labelMode !== "glyph" || !item.icon) && (
              <span className="truncate">{item.label}</span>
            )}
        </button>,
      )}
      {item.tip &&
        tipOpen &&
        createPortal(
          <div
            ref={bubbleRef}
            role="tooltip"
            id={tooltipId}
            className="glim-bubble glim-fade"
          >
            {item.tip}
          </div>,
          document.body,
        )}
    </>
  );
}

export function Selector(props: SelectorProps) {
  const {
    items,
    label,
    size = "md",
    hue = true,
    plain = false,
    variant = "chip",
    equalWidth = false,
  segmentWidth,
    raised = false,
    disabled = false,
    className = "",
  } = props;
  const well = variant === "well";

  // `pinWidth` — whether segments get pinned to a real MEASURED width
  // instead of hugging their own labels. Used to read `stretch` alone (the
  // "chip" `equalWidth` case only); "well" relied on `flex-1` doing the
  // equal-width job on its own, which worked as long as an ANCESTOR forced
  // this whole strip to stretch across a wide container — plenty of leftover
  // flex-grow space meant even the widest label always had generous room.
  //   BROKEN (jdp, live-review, task 1 verification screenshot — "Dunkel"
  // rendering as "Du..." in the Theme Card, "Leicht" as "Lei..." in the Shape
  // Card): once task 1's own fix (Settings.tsx's own `inline-flex self-start`
  // wrapper) stopped an ANCESTOR from stretching this strip, the browser had
  // to size the "well" track itself via shrink-to-fit — and a flex item
  // whose `flex-basis` resolves to a definite value (`flex-1` is shorthand
  // for `flex: 1 1 0%`, i.e. `flex-basis: 0%`) contributes that ZERO basis,
  // not its own content width, to a shrink-to-fit ancestor's intrinsic-size
  // calculation (CSS Flexbox spec section 9.9) — combined with this file's
  // own unconditional `min-w-0` on every segment (needed for the "chip"
  // wrap/shrink cases), nothing stopped the widest segment from being sized
  // narrower than its own label needs. Confirmed live via
  // getBoundingClientRect: the "Dunkel" segment's own label span reported a
  // `scrollWidth` bigger than its `clientWidth` — genuine overflow silently
  // cropped by the `truncate` class, not a font-rendering artifact.
  //   The fix is the SAME one item 5b already proved for "chip"'s own
  // `equalWidth`: a real DOM measurement of the widest segment's natural
  // content width, then an explicit pixel `width` on every segment — no
  // flex-basis trick involved, so this particular intrinsic-sizing gap never
  // applies.
  //   ROUND 8 (file header item 6): `pinWidth` is now simply `equalWidth`,
  // for BOTH variants. It briefly read `stretch || well` — "well" pinned
  // unconditionally, because at the time every "well" strip WAS a big
  // page-level picker. Folding the round-7 "track" variant back into "well"
  // makes that untrue: a small, repeated "well" strip is content-hugging, so
  // the caller has to say which scale it wants, and `equalWidth` is the prop
  // that already meant exactly that for "chip" — one prop, one measurement
  // pipeline, three call sites that pass it. No `stretch` alias any more
  // either: the "chip"-only guard existed to stop `equalWidth` engaging on a
  // "track" segment, and there is no "track" to protect.
  const pinWidth = equalWidth;

  // Content-width measurement for `pinWidth` (item 5b, corrected — see the
  // file header): every segment gets pinned to the WIDEST segment's own
  // natural content width, which only a real DOM measurement can answer (a
  // pure-CSS flex/grid trick can equalize widths, but "equal to N's own
  // intrinsic size" specifically needs to know what N's intrinsic size IS).
  //
  // itemsKey is a primitive string, not the `items` array reference itself:
  // Settings.tsx's tab-strip call site rebuilds that array with `.map()` on
  // every render of its (very large) parent component, so a NEW array
  // identity says nothing about whether the LABELS actually changed — using
  // it directly as a dependency would re-run the reset-and-remeasure pair
  // below on every unrelated keystroke elsewhere on the page, flashing the
  // strip back to its natural (momentarily unequal) widths each time. Two
  // JS string primitives with the same characters compare equal (`===`) even
  // when freshly concatenated on every call, so this stays referentially
  // stable across renders unless a label genuinely changed (e.g. a locale
  // switch).
  const itemsKey = items.map((it) => it.label).join("");
  const [matchedWidth, setMatchedWidth] = useState<number | null>(null);
  const itemRefs = useRef<Array<HTMLButtonElement | null>>([]);

  // Pass 1: release any previously matched width whenever the label set (or
  // the size stage, which changes padding/font and therefore every label's
  // natural width) changes. Necessary because a segment currently holding an
  // explicit `width` renders with NO overflow, so measuring it directly
  // (getBoundingClientRect/scrollWidth) would just report that stale applied
  // width back, not the new content's actual natural size — e.g. switching
  // from German (longer labels) to English (shorter ones) must not leave
  // every tab permanently wearing German's width forever. Both effects are
  // useLayoutEffect, so the null-render and the re-measured render both
  // commit before the browser's next paint — no visible flash back to
  // ragged widths.
  useLayoutEffect(() => {
    if (pinWidth) setMatchedWidth(null);
  }, [pinWidth, itemsKey, size]);

  // Pass 2: once segments have rendered at their own natural width (either
  // the very first render, or immediately after pass 1 cleared a stale
  // match), measure the widest one and pin every segment to that SAME fixed
  // pixel width. Skipped once matchedWidth is already set — there is nothing
  // to re-measure until the label set changes again and pass 1 clears it.
  useLayoutEffect(() => {
    if (!pinWidth || matchedWidth !== null) return;
    // Sliced to the CURRENT item count before filtering: a shrunk item list
    // (not a concern for Settings.tsx's fixed 7-tab strip today, but this is
    // a shared component) would otherwise leave stale trailing refs from a
    // longer previous render mixed into the measurement.
    const nodes = itemRefs.current
      .slice(0, items.length)
      .filter((n): n is HTMLButtonElement => n !== null);
    if (nodes.length === 0) return;
    // MIN_PINNED_WIDTH folded into the SAME Math.max as the widest-segment
    // measurement, not a separate clamp afterwards — one call, one floor,
    // still a pure floor (see that constant's own doc): a widest label that
    // already measures past 200px keeps its own larger number untouched.
    const widest = Math.max(
      ...nodes.map((n) => n.getBoundingClientRect().width),
      MIN_PINNED_WIDTH,
    );
    setMatchedWidth(widest);
  }, [pinWidth, matchedWidth, itemsKey, size, items.length]);

  // Subscribed, not read — see lib/useRainbow.ts's own header for why this
  // has to happen during render rather than an effect. Called unconditionally
  // (rules of hooks) even when hue=false: a hue=false strip (Dashboard's
  // heatmap toggle) still re-renders correctly when something ELSE on the
  // page reacts to a rainbow change, it just never reads the value into its
  // own style.
  useRainbow();

  const many = props.select === "many";
  const chosen = props.select === "many" ? props.active : null;
  const only = props.select === "many" ? null : props.active;
  const { onChange } = props;
  const auto = !many;

  const strip = useRef<HTMLDivElement>(null);
  const isOn = (id: string) => (chosen ? chosen.has(id) : only === id);
  const isItemDisabled = (item: SelectorItem) => disabled || !!item.disabled;

  const disabledFlags = items.map(isItemDisabled);
  const activeIdx = items.findIndex((it) => isOn(it.id));
  const roved = rovedIndex(disabledFlags, activeIdx);

  function segNodes(): HTMLElement[] {
    return Array.from(
      strip.current?.querySelectorAll<HTMLElement>(
        "[data-sel-id]:not(:disabled)",
      ) ?? [],
    );
  }

  function onKeyDown(e: KeyboardEvent<HTMLDivElement>) {
    if (!NAV_KEYS.includes(e.key)) return;
    const nodes = segNodes();
    if (nodes.length === 0) return;

    // Read direction off the strip itself rather than assuming it, so the
    // same component behaves correctly under dir="rtl" (Arabic, Hebrew —
    // both shipped locales, lib/i18n.ts's isRtl) without a prop.
    const rtl = strip.current
      ? getComputedStyle(strip.current).direction === "rtl"
      : false;
    const here = nodes.indexOf(document.activeElement as HTMLElement);
    const next = nextFocusIndex(
      e.key as SelectorNavKey,
      here,
      nodes.length,
      rtl,
    );
    if (next < 0) return;

    e.preventDefault();
    const node = nodes[next];
    node?.focus();
    const id = node?.getAttribute("data-sel-id");
    if (auto && id) onChange(id);
  }

  return (
    <div
      ref={strip}
      role={many ? "group" : "tablist"}
      aria-label={label}
      aria-orientation="horizontal"
      onKeyDown={onKeyDown}
      // "chip" (default): no background, no padding, no border on this
      // wrapper — the filled segment already says which one is chosen; a
      // box around the row is one elevation too many (rule 1) and says
      // nothing a plain gap between bare badges doesn't already say. Wraps
      // (flex-wrap), never scrolls.
      // "well" (file header items 5 and 6): the one deliberate exception —
      // this IS TrickWork's shared track surface (background + 0.2rem padding
      // + `rounded-control`, so it still reshapes with the shape engine like
      // every other radius in the app), holding flush segments that are
      // transparent until chosen. ONE set of classes for both scales; only
      // the segments below differ, and only via `equalWidth`.
      //   `bg-carbon-surface3`, NOT the surface2 this variant shipped with
      // through round 7 (file header item 6 (b)): the groove has to be
      // distinct from BOTH parents it legally sits on — a Card
      // (`bg-carbon-surface`) and a CadenceBuilder well
      // (`bg-carbon-surface2`) — and surface3 is the only surface token that
      // is. A surface2 groove inside a surface2 well measured as literally
      // the same colour as its own parent on the running container, which is
      // how the small variant shipped invisible in round 7's first cut.
      // Measured at the Card parent the big pickers sit on, surface3 is also
      // the stronger enclosure of the two (1.94:1 dark / 1.53:1 light,
      // against surface2's 1.31:1 / 1.21:1), so unifying on it costs the big
      // scale nothing.
      //   `[0.2rem]` gap/padding is the established value for this exact
      // role (the visible groove ring between the track edge and its
      // segments); round 7's own re-derived `[0.15rem]` left a 2.4px ring,
      // too thin to read as an enclosure. Scale separation comes from the
      // segments, never from shaving the groove.
      //   `w-fit max-w-full`: the groove hugs its own segments instead of
      // stretching to a block parent's full width (CadenceBuilder's mode
      // strip is a direct child of a `flex flex-col` fieldset and rendered as
      // a full-width bar the moment the groove became visible). `width:
      // fit-content` is not `auto`, so it ALSO opts the strip out of a flex
      // column's default `align-items: stretch` — without an explicit
      // `self-start`, which would have top-aligned it inside the row-shaped
      // parents (NotifyCard's "on" row, the weekday row, the drill-kind row)
      // that centre a label beside it. This is what let Settings.tsx drop its
      // three hand-rolled `inline-flex self-start max-w-full` wrapper divs.
      //   `flex-wrap`, NOT the `flex-nowrap` "well" carried before round 8:
      // an `equalWidth` strip's own width is the sum of N x max(widest label,
      // MIN_PINNED_WIDTH), a number with no guaranteed relationship to the
      // available width — the same overflow "chip"'s own `equalWidth` already
      // wraps for (item 5b, where `flex-nowrap` genuinely spilled a 7-tab
      // strip past the page's right edge). A content-hugging "well" can
      // equally need a second line on a narrow card.
      className={[
        "flex items-center",
        well
          ? "w-fit max-w-full flex-wrap gap-[0.2rem] rounded-control bg-carbon-surface3 p-[0.2rem]"
          : "flex-wrap gap-1",
        className,
      ]
        .filter(Boolean)
        .join(" ")}
    >
      {items.map((item, i) => {
        const on = isOn(item.id);
        const itemDisabled = disabledFlags[i];
        const cls = [
          "inline-flex min-w-0 max-w-full items-center font-medium",
          // CORRECTED (jdp, live-review — "the shape picker's own well
          // track/segments don't reshape"): this used to read "well segments
          // carry no radius of their own", copying TrickWork's
          // `.segmented-button` verbatim (it sets no border-radius at all,
          // relying only on the track's own 0.2rem padding to inset flush
          // segments from the row's rounded corners). Verified live
          // (getComputedStyle on a real well segment across all three
          // [data-shape] values): the TRACK's own `rounded-control` above DID
          // reshape correctly (10px/5px/0px), but every segment stayed at a
          // hard 0px regardless — meaning the one piece of this control
          // someone actually watches while clicking Rund/Leicht/Eckig (the
          // filled, selected pill) never visibly changed, which reads as
          // "nothing is happening" even though the track quietly did its
          // job. TrickWork's own reference is a fixed-shape app with no
          // per-user shape engine, so "no radius of its own" was never load-
          // bearing there the way it is here — jdp's explicit ask ("the same
          // way every other shape-reactive element on the page does") wins
          // over the ported spec. `rounded-control` now applies to a "well"
          // segment too, same token/class the track itself and every "chip"
          // segment already reshape through — no new mechanism, just the one
          // this file already proved works. Kept alongside the exact
          // `transition: background-color 120ms ease;` from TrickWork's own
          // rule (not "chip"'s broader `transition-colors`, which times more
          // properties at Tailwind's own default 150ms — fine for a chip's
          // idle-background swap, not what this track's spec asks for);
          // border-radius was never part of either transition list, so this
          // addition doesn't newly animate anything. Both "well" SCALES get
          // this same crossfade-only transition — the variant exists so a
          // strip reads as one coherent physical control, and that reading
          // is worth exactly as much at the small scale as the large one.
          well
            ? "rounded-control [transition:background-color_120ms_ease]"
            : "rounded-control transition-colors",
          "disabled:opacity-50 disabled:cursor-not-allowed",
          // `.glim-hue-icon` (index.css) additionally tints the glyph itself
          // with the item's own hue while idle — right for a tab-strip icon
          // that sits beside its own always-visible text label (the Settings
          // tab strip, this class's one real consumer so far), wrong for an
          // `iconOnly` segment: there the glyph IS the badge's entire visible
          // content, and design-language.md's own icon-only-badge rule is
          // explicit that only the BADGE (background/fill) ever carries
          // colour, the glyph itself always stays plain contrast ink
          // ("die icons sollen nicht eingefärbt werden, nur die badges also
          // der hintergrund" — a real adopting app's own rejection of exactly
          // this, quoted verbatim in that doc). `.glim-hue` still applies
          // unconditionally either way — the ACTIVE segment's own
          // `bg-accent text-accentContrast` below still needs it to resolve
          // this item's position colour for its background/contrast-ink text,
          // only the separate glyph-tint rider is what iconOnly opts out of.
          hue ? (item.iconOnly ? "glim-hue" : "glim-hue glim-hue-icon") : "",
          on
            ? "glim-active bg-accent text-accentContrast"
            : // "well", BOTH scales (jdp, live-review round 8: "Die kleinen
              // Selektoren sollen so aussehen wie die grossen! Die nicht
              // ausgewaehlten Optionen sollen kein Badge sein"). An idle
              // segment carries NO fill of its own — the groove behind it is
              // the whole enclosure, and only the chosen segment is a badge.
              // Round 7's small variant filled every idle segment at
              // `bg-carbon-surface` instead ("a groove holding keys"), which is
              // exactly the per-segment badge this reverses; see file header
              // item 6 for the full writeup.
              //   The cost, stated where the class lives: this label now sits
              // on the surface3 groove rather than on a surface fill, so
              // `text-carbon-textSub` measures 4.57:1 (dark) / 5.12:1 (light)
              // instead of 8.85:1 / 7.82:1. Both clear WCAG AA for normal text;
              // dark clears it by 0.08, so treat that as the floor.
              //   `hover:bg-carbon-hover` is the other half of that trade and
              // gets better on the deeper groove: 1.57:1 dark / 1.16:1 light
              // against surface3, up from a near-invisible 1.06:1 / 1.08:1 when
              // the groove was surface2.
              //   `plain`/`raised` are both meaningless here (see each prop's
              // own doc) so neither is read in this branch.
              well
              ? "bg-transparent text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
              : plain
                ? "text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
                : raised
                  ? "bg-carbon-surface3 text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
                  : "bg-carbon-surface2 text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text",
          SIZE[size].gap,
          SIZE[size].text,
          // The one place the two SCALES of this control differ, and the
          // only thing `equalWidth` changes (file header item 6).
          //   NOT pinned (default, both variants): per-stage padding, height
          // from padding + line-height. That is "chip"'s own long-standing
          // box, and it is also what makes a small "well" strip cheap enough
          // to repeat on every schedule card across two tabs — the role the
          // round-7 "track" variant used to hold.
          //   Pinned (`equalWidth`): every segment gets an explicit pixel
          // `width` (set via inline `style.width` below, from the `pinWidth`
          // measurement pass — NOT a flex-basis trick; see that constant's
          // own doc for the live truncation bug `flex-1` caused here) plus
          // centred content. `flex-none` so the explicit width actually wins
          // instead of competing with a flex-grow share, and so the default
          // `flex-shrink: 1` can't compress a matched segment back down —
          // pinned segments give way together by wrapping the row, never
          // individually.
          //   Pinned AND "well" additionally takes the FIXED --badge-md
          // height (index.css) in place of padding-derived height: a
          // standardized box needs a standardized height, not just a
          // standardized width, so every segment matches regardless of label
          // length or an icon's own intrinsic size. It keeps `SIZE[size]
          // .padding` alongside that so the widest label isn't touching the
          // pinned box's edge. Pinned "chip" (the Settings tab strip) does
          // NOT take that height — it stays visually a chip, just a
          // fixed-width one.
          //
          // `iconOnly` (GlimStone follow-up round, live-review screenshot —
          // "the Local/Remote badges read as wider/pill-shaped, the separate
          // Browse button below them reads as square, they need to match"):
          // a per-stage `SIZE[size].padding` (`px-2 py-0.5` at "sm", asymmetric
          // on purpose for a TEXT chip's pill shape) renders a single 16px
          // icon as a ~32×20 rectangle, not a square, and independently of
          // whatever height FolderBrowser's own Browse button happened to be
          // built to. Fixed `h-8 w-8` (Tailwind's own plain default spacing
          // scale, step 8 = 2rem = 32px — not a new bracket/arbitrary value
          // invented for this one call site) makes every iconOnly segment a
          // true square, and 32px is the SAME height the adjacent path
          // `<input>` (FolderBrowser's own `text-sm px-3 py-1.5` field, and
          // PathModeSwitch's own remote-mode URL field) actually renders at —
          // verified live via getComputedStyle, not assumed: neither
          // Badge.tsx's own three status-chip stages (18/20/24px) nor this
          // file's own `--badge-md` (2.35rem, a value ported verbatim from a
          // DIFFERENT app's — TrickWork's — segmented-row spec for the
          // Settings shape-picker's own well track, never independently
          // checked against BombVault's own field heights) happens to equal
          // that number, so reusing either here would still leave a visible
          // height mismatch. FolderBrowser's own Browse button below is sized
          // the identical `h-8 w-8` for exactly this reason — one fixed
          // square footprint, shared by both controls in this row, matching
          // the one real field height that's actually next to them.
          //   32px is now the app-wide constant for EVERY square icon-only
          // control, not just this row's: Badge.tsx's `size="icon"` stage
          // resolves to the same h-8, and its "ONE SIZE FOR SQUARE ICON
          // BADGES" block is the authority. This literal stays a literal only
          // because Selector is its own component and never renders through
          // Badge; if the app-wide number ever moves, it moves here too, in
          // lockstep. Do NOT re-derive it from whatever field happens to sit
          // beside a given Selector — the per-neighbour reasoning that made
          // this comment's own measurement necessary is exactly the split jdp
          // rejected. Ordered AFTER the two pinned branches (neither is used
          // by today's one iconOnly consumer, PathModeSwitch) but ahead of
          // the plain per-stage padding, so an UNPINNED iconOnly segment —
          // including an unpinned "well" one, which round 8 newly makes
          // possible — still gets the fixed square rather than a padding
          // class that was never sized for a bare glyph.
          well && equalWidth
            ? `flex-none justify-center text-center h-[var(--badge-md)] ${SIZE[size].padding}`
            : equalWidth
              ? `flex-none justify-center text-center ${SIZE[size].padding}`
              : item.iconOnly
                ? "justify-center h-8 w-8 p-0"
                : SIZE[size].padding,
        ]
          .filter(Boolean)
          .join(" ");

        // Merge the rainbow custom-property style (if any) with the matched
        // fixed width (if `pinWidth` has completed its measurement pass) —
        // both are optional and independent, so this stays undefined
        // whenever neither applies rather than always allocating an object.
        const hueStyle = hue
          ? (hueVars(rainbowAt(i)) as CSSProperties)
          : undefined;
        const widthStyle: CSSProperties | undefined =
          segmentWidth
            ? { width: segmentWidth }
            : pinWidth && matchedWidth !== null
            ? { width: `${matchedWidth}px` }
            : undefined;
        const itemStyle =
          hueStyle || widthStyle ? { ...hueStyle, ...widthStyle } : undefined;

        return (
          <SelectorTab
            key={item.id}
            item={item}
            many={many}
            on={on}
            disabled={itemDisabled}
            roved={i === roved}
            className={cls}
            style={itemStyle}
            onSelect={() => {
              if (!itemDisabled) onChange(item.id);
            }}
            registerRef={(el) => {
              itemRefs.current[i] = el;
            }}
          />
        );
      })}
    </div>
  );
}
