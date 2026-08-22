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
//      that still looks wrong once actually rendered. `equalWidth` gives
//      every segment an identical `flex: 1 1 0%` share of the row instead
//      (plain CSS flex division, not a JS-measured width), so the row's
//      right edge is always the LAST segment's right edge, which is also the
//      row's own container edge — the same edge the Card below already
//      renders flush to. Kept inside the "chip" branch specifically (own
//      prop, not folded into `variant="well"`): the chip look this strip
//      already has — each tab its own individually filled/outlined badge,
//      not a shared padded track — is what round 6 specifically built ("idle
//      badge fill" for unselected tabs); switching to `well` would trade
//      that identity away to solve a width problem `well` doesn't uniquely
//      solve (equal width is just `flex-1` either way — `well`'s OTHER
//      differences, the shared track background and flush borderless
//      segments, aren't what was asked for here). `flex-nowrap` alongside it
//      for the same reason `well` itself is `flex-nowrap` (this file's own
//      item 5 below): equal-width flex children assume one row, wrapping
//      them is a different, broken-looking layout, not a smaller version of
//      the same one. Scoped to exactly one call site (the Settings tab
//      strip) for now, same "try it on one control" pattern `variant="well"`
//      already set — every other "chip" call site keeps its own
//      content-hugging width, unchanged.
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
import { useRef, type CSSProperties, type KeyboardEvent, type ReactNode } from "react";
import { hueVars, rainbowAt } from "../lib/appearance";
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
      // `stretch` (item 5b): same "one row, no wrap" reasoning as "well"
      // above, no shared track background — each segment keeps its own chip
      // fill, just an equal-width one instead of content-hugging.
      className={[
        "flex items-center",
        well
          ? "flex-nowrap gap-[0.2rem] rounded-control bg-carbon-surface2 p-[0.2rem]"
          : stretch
            ? "flex-nowrap gap-1"
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
          // "well" segments carry no radius of their own — TrickWork's
          // `.segmented-button` doesn't set border-radius at all, relying on
          // the track's own 0.2rem padding to inset them from the row's
          // rounded corners (see the wrapper's own comment above). Also the
          // exact `transition: background-color 120ms ease;` from that same
          // rule, in place of "chip"'s Tailwind `transition-colors` (which
          // covers more properties at Tailwind's own default 150ms/timing —
          // fine for a chip's idle-background swap, not what TrickWork's
          // spec actually says for this track).
          well ? "[transition:background-color_120ms_ease]" : "rounded-control transition-colors",
          "disabled:opacity-50 disabled:cursor-not-allowed",
          hue ? "glim-hue glim-hue-icon" : "",
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
          // `stretch` (item 5b): the same `flex-1 justify-center text-center`
          // equal-width/centered treatment as "well", MINUS the fixed
          // --badge-md height swap — a stretch chip keeps its own per-stage
          // padding (its height still comes from padding + line-height, a
          // "chip" trait this variant doesn't touch), so it stays visually a
          // chip, just an evenly-shared-width one.
          well
            ? "flex-1 justify-center text-center h-[var(--badge-md)]"
            : stretch
              ? `flex-1 justify-center text-center ${SIZE[size].padding}`
              : SIZE[size].padding,
        ]
          .filter(Boolean)
          .join(" ");

        return (
          <button
            key={item.id}
            type="button"
            data-sel-id={item.id}
            role={many ? undefined : "tab"}
            aria-selected={many ? undefined : on}
            aria-pressed={many ? on : undefined}
            title={item.title}
            disabled={itemDisabled}
            tabIndex={i === roved ? 0 : -1}
            style={hue ? (hueVars(rainbowAt(i)) as CSSProperties) : undefined}
            className={cls}
            onClick={() => {
              if (!itemDisabled) onChange(item.id);
            }}
          >
            {item.icon}
            <span className="truncate">{item.label}</span>
          </button>
        );
      })}
    </div>
  );
}
