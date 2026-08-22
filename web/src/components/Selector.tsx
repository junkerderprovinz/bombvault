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
//      unconditionally with no opt-out at all. Dashboard.tsx's heatmap
//      domain toggle needs exactly that opt-out — Task 2's own audit
//      decided that toggle stays deliberately un-rainbowed (5 fixed,
//      already-label-identified domains sitting right beside the heatmap's
//      own fixed red/green state hues; a 5-way rainbow strip there competes
//      with rule 4 for no tracking benefit). `hue={false}` skips both the
//      `.glim-hue`/`.glim-hue-icon` classes and the `hueVars()` inline style
//      entirely, so that one strip never enters a rainbow subtree no matter
//      what the global rainbow setting is doing elsewhere on the page.
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
//   5b. `equalWidth` (default false, "chip" only). Live-review follow-up:
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
//      Kept inside the "chip" branch specifically (own prop, not folded into
//      `variant="well"`): the chip look this strip already has — each tab
//      its own individually filled/outlined badge, not a shared padded
//      track — is what round 6 specifically built ("idle badge fill" for
//      unselected tabs); switching to `well` would trade that identity away
//      to solve a width problem `well` doesn't uniquely solve.
//
//      `flex-wrap`, NOT `flex-nowrap` (caught live, not in the harness —
//      the first cut of this correction shipped `flex-nowrap`, copying
//      `well`'s reasoning verbatim without re-checking whether it still
//      applied). `well`'s segments are `flex: 1` shares of a track that IS
//      the available width by construction — they can never overflow it, so
//      forcing one row there is free. `stretch` segments are now a FIXED
//      pixel width (the widest label's own measured content size), a number
//      with NO relationship to whatever width happens to be available — 7
//      German labels at this stage's own padding sum to ~1243px, which
//      genuinely does not fit inside every real viewport (measured live:
//      overflowed a 1400px-wide window's ~1113px available column, spilling
//      "System" straight past the page's right edge and forcing a page-wide
//      horizontal scrollbar — a real, visible regression, not a theoretical
//      one). `flex-wrap` here is simply Selector's own general "wraps,
//      never scrolls" rule (this file's header) applying to `stretch` the
//      same way it already applies to every plain "chip" call site — a
//      6-tabs-then-1 wrap is a plainer fallback than jdp's ideal, but a
//      wrapped SECOND ROW beats spilling content off the edge of the page.
//      Scoped to exactly one call site (the Settings tab strip) for now,
//      same "try it on one control" pattern `variant="well"` already set —
//      every other "chip" call site keeps its own content-hugging width,
//      unchanged.
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
//      is ignored whenever both are given. Scoped to exactly one call site
//      for now (Settings.tsx's shape picker) — GlimStone's own "try it on
//      one control, generalize later if it lands well" pattern — so the
//      other eleven migrated call sites keep rendering byte-identical
//      "chip" output; nothing about this addition changes their classes.
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
import { useEffect, useId, useLayoutEffect, useRef, useState, type CSSProperties, type KeyboardEvent, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { hueVars, rainbowAt } from "../lib/appearance";
import { computeBubblePosition } from "../lib/bubblePosition";
import { useRainbow } from "../lib/useRainbow";

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
 *  `plain` doc below for the two flavours of it. "well" is TrickWork's
 *  shared padded track with flush, crossfade-only segments; see the file
 *  header's item 5 for the full rationale and exactly what is/isn't ported
 *  from TrickWork's version. */
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
  /** Rainbow position per item, default true. See the file header — the ONE
   *  documented exception is Dashboard.tsx's heatmap toggle. */
  hue?: boolean;
  /** Page-tab treatment (no idle background) instead of the default
   *  toolbar-chip treatment (idle `bg-carbon-surface2` pill). See the file
   *  header for which of the twelve call sites uses which. Ignored under
   *  `variant="well"` — see that prop's own doc. */
  plain?: boolean;
  /** "chip" (default) vs "well" — see the file header's item 5. */
  variant?: SelectorVariant;
  /** Stretch every "chip" segment to an equal `flex-1` share of the row
   *  instead of each hugging its own label width — see the file header's
   *  item 5b. Ignored under `variant="well"` (already always equal-width).
   *  Default false: every pre-existing "chip" call site keeps its own
   *  content-hugging width. */
  equalWidth?: boolean;
  /** Disables every item (e.g. SourceToggle mid-restore). A per-item
   *  `disabled` still applies on top of this. */
  disabled?: boolean;
  className?: string;
}

export type SelectorProps =
  | (SelectorCommon & { select?: "one"; active: string | null; onChange: (id: string) => void })
  | (SelectorCommon & { select: "many"; active: ReadonlySet<string>; onChange: (id: string) => void });

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
const SIZE: Record<SelectorSize, { gap: string; padding: string; text: string }> = {
  sm: { gap: "gap-1", padding: "px-2 py-0.5", text: "text-xs" },
  md: { gap: "gap-1.5", padding: "px-3 py-1", text: "text-xs" },
  lg: { gap: "gap-2", padding: "px-3 py-1.5", text: "text-sm" },
};

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
export function nextFocusIndex(key: SelectorNavKey, current: number, count: number, rtl: boolean): number {
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

function SelectorTab({ item, many, on, disabled, roved, className, style, onSelect, registerRef }: SelectorTabProps) {
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
      viewport
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

  return (
    <>
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
        aria-label={item.iconOnly ? item.label : undefined}
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
        {item.icon}
        {!item.iconOnly && <span className="truncate">{item.label}</span>}
      </button>
      {item.tip &&
        tipOpen &&
        createPortal(
          <div ref={bubbleRef} role="tooltip" id={tooltipId} className="glim-bubble glim-fade">
            {item.tip}
          </div>,
          document.body
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
    disabled = false,
    className = "",
  } = props;
  const well = variant === "well";
  // Only meaningful on "chip" — "well" is already always equal-width via its
  // own `flex-1` (see that branch below), so this never double-applies.
  const stretch = equalWidth && !well;

  // Content-width measurement for `stretch` (item 5b, corrected — see the
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
    if (stretch) setMatchedWidth(null);
  }, [stretch, itemsKey, size]);

  // Pass 2: once segments have rendered at their own natural width (either
  // the very first render, or immediately after pass 1 cleared a stale
  // match), measure the widest one and pin every segment to that SAME fixed
  // pixel width. Skipped once matchedWidth is already set — there is nothing
  // to re-measure until the label set changes again and pass 1 clears it.
  useLayoutEffect(() => {
    if (!stretch || matchedWidth !== null) return;
    // Sliced to the CURRENT item count before filtering: a shrunk item list
    // (not a concern for Settings.tsx's fixed 7-tab strip today, but this is
    // a shared component) would otherwise leave stale trailing refs from a
    // longer previous render mixed into the measurement.
    const nodes = itemRefs.current
      .slice(0, items.length)
      .filter((n): n is HTMLButtonElement => n !== null);
    if (nodes.length === 0) return;
    const widest = Math.max(...nodes.map((n) => n.getBoundingClientRect().width));
    setMatchedWidth(widest);
  }, [stretch, matchedWidth, itemsKey, size, items.length]);

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
    return Array.from(strip.current?.querySelectorAll<HTMLElement>("[data-sel-id]:not(:disabled)") ?? []);
  }

  function onKeyDown(e: KeyboardEvent<HTMLDivElement>) {
    if (!NAV_KEYS.includes(e.key)) return;
    const nodes = segNodes();
    if (nodes.length === 0) return;

    // Read direction off the strip itself rather than assuming it, so the
    // same component behaves correctly under dir="rtl" (Arabic, Hebrew —
    // both shipped locales, lib/i18n.ts's isRtl) without a prop.
    const rtl = strip.current ? getComputedStyle(strip.current).direction === "rtl" : false;
    const here = nodes.indexOf(document.activeElement as HTMLElement);
    const next = nextFocusIndex(e.key as SelectorNavKey, here, nodes.length, rtl);
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
      // "well": the one deliberate exception (file header item 5) — this IS
      // TrickWork's shared track surface (background + 0.2rem padding +
      // `rounded-control`, so it still reshapes with the shape engine like
      // every other radius in the app), holding equal-width flush segments.
      // No flex-wrap here: TrickWork's segments are `flex: 1` (see below),
      // which assumes one row — wrapping equal-width flex children onto a
      // second line is a different, broken-looking layout, not a smaller
      // version of the same one.
      // `stretch` (item 5b, corrected — see the file header for the live bug
      // this fixed): `flex-wrap`, NOT `flex-nowrap` like "well" above — a
      // fixed-pixel-width segment set has no guaranteed relationship to the
      // available row width the way `well`'s `flex: 1` shares do, so it can
      // genuinely need to wrap. No shared track background either way — each
      // segment keeps its own chip fill, just pinned to the widest segment's
      // own measured content width (via inline `style.width` below) instead
      // of hugging its own individually. This wrapper itself is NOT
      // stretched to fill anything — it's a plain flex row with no
      // `flex-1`/`w-full` of its own, so its rendered width is simply
      // whichever is smaller: the sum of its now-fixed-width children (if
      // they all fit on one line), or the available row width (once they
      // don't and a second line is needed).
      className={[
        "flex items-center",
        well
          ? "flex-nowrap gap-[0.2rem] rounded-control bg-carbon-surface2 p-[0.2rem]"
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
          // addition doesn't newly animate anything.
          well ? "rounded-control [transition:background-color_120ms_ease]" : "rounded-control transition-colors",
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
            : well
              ? "bg-transparent text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
              : plain
                ? "text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
                : "bg-carbon-surface2 text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text",
          SIZE[size].gap,
          SIZE[size].text,
          // "chip" keeps its own per-stage padding (the box comes from
          // padding + line-height, as it always has). "well" segments are
          // `flex-1` (equal-width, sharing the track evenly — TrickWork's
          // own layout, which only makes sense un-wrapped, see above) with
          // their content centered (TrickWork's `justify-content`/
          // `text-align: center` — a "chip"'s intrinsic, shrink-to-fit width
          // never needed this) and a fixed height read off --badge-md
          // (index.css) instead of padding, so every segment in the row is
          // the same height regardless of label length or an icon's own
          // intrinsic size.
          // `stretch` (item 5b, corrected): centred content within a FIXED
          // width (set via inline `style.width` below, not `flex-1` — see
          // this file's header for why a flex share was the wrong fix).
          // `flex-none` pins the segment to exactly that width rather than
          // letting the default `flex-shrink: 1` compress it back down if
          // the row's own content ever exceeds its container (matched-width
          // segments should shrink together with the row wrapping, per
          // `flex-nowrap` above, not individually). Keeps its own per-stage
          // padding otherwise — height still comes from padding +
          // line-height, a "chip" trait this variant doesn't touch, so it
          // stays visually a chip, just a fixed-width one.
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
          // the one real field height that's actually next to them. Checked
          // ahead of `stretch`/`well` (both unused by today's one iconOnly
          // consumer, PathModeSwitch) so a future combination degrades to
          // this fixed square rather than silently falling through to a
          // per-stage padding class that was never sized for a bare glyph.
          well
            ? "flex-1 justify-center text-center h-[var(--badge-md)]"
            : stretch
              ? `flex-none justify-center text-center ${SIZE[size].padding}`
              : item.iconOnly
                ? "justify-center h-8 w-8 p-0"
                : SIZE[size].padding,
        ]
          .filter(Boolean)
          .join(" ");

        // Merge the rainbow custom-property style (if any) with the matched
        // fixed width (if `stretch` has completed its measurement pass) —
        // both are optional and independent, so this stays undefined
        // whenever neither applies rather than always allocating an object.
        const hueStyle = hue ? (hueVars(rainbowAt(i)) as CSSProperties) : undefined;
        const widthStyle: CSSProperties | undefined =
          stretch && matchedWidth !== null ? { width: `${matchedWidth}px` } : undefined;
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
