// Selector is the app's one horizontal selector. Tabs, filter bars, segmented
// controls and pickers all render through it, so they cannot drift apart
// (design-language.md, "The one horizontal selector").
//
// The strip is a single tab stop. Arrow keys, Home and End move between
// segments with a roving tabindex, and the arrows follow the reading direction
// under dir="rtl". With select="one" moving also selects, as in a tab strip;
// with select="many" the segments are toggle buttons.
//
// The strip spans its box and its segments share the width, so a card of
// stacked selectors ends in one edge. A strip that shares a toolbar row with a
// search field or buttons passes `inline` and hugs its segments instead.
//
// A caption such as "Sort by:" belongs in a plain <span> outside the component,
// not a <label> around the row: a label around several tabs forwards its clicks
// to the first one and gives screen readers that tab's name.
import {
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
  type KeyboardEvent,
  type ReactNode,
} from "react";
import { hueVars } from "../lib/appearance";
import { useLabelMode } from "../lib/useLabelMode";
import type { ControlAxis } from "../lib/controls";
import { hidesLabel, labelWidth } from "../lib/controls";
import { useTipBubble } from "../lib/useTipBubble";
import { segmentLayout, type SegmentWidths } from "../lib/segmentLayout";

export interface SelectorItem {
  /** Stable id, handed back by onChange. */
  id: string;
  label: string;
  /** Glyph before the label, tinted with the segment's hue while `hue` is on. */
  icon?: ReactNode;
  disabled?: boolean;
  /** Why the segment is in its state, such as "pick a target folder first" on
   *  a disabled chip. It goes into the tooltip rather than a native title,
   *  which keyboard users never see. */
  title?: string;
  /** Shows only `icon` in every label mode except text. `label` stays the
   *  button's accessible name. */
  iconOnly?: boolean;
  /** Tooltip on hover and focus, for a label too terse to stand alone
   *  ("Local" → "Local path on this host"). A segment whose label is hidden
   *  falls back to the label. */
  tip?: string;
}

export type SelectorSize = "sm" | "md" | "lg";

/** "well" sets flush segments in a shared groove, and only the chosen one is
 *  filled. Without `equalWidth` it is the small scale for strips that repeat
 *  per card; with `equalWidth` and size="lg" it is the big page-level picker.
 *  "chip" gives every segment a pill of its own, for a row of disclosure
 *  toggles that open sections rather than choose a value. */
export type SelectorVariant = "chip" | "well";

interface SelectorCommon {
  items: SelectorItem[];
  /** Accessible name for the strip, e.g. "Sort", "Settings sections". */
  label: string;
  /** `md` (default) is the card and toolbar scale, `sm` the dense strip
   *  (heatmap domains, weekday pills), `lg` the page-level scale (Settings
   *  tabs and pickers). */
  size?: SelectorSize;
  /** Give segments the button height (`--btn-h`) instead of a padding-derived
   *  one, for a strip beside a real Button. Opt-in because a strip cannot see
   *  its neighbours, and the dense strips should stay short. */
  buttonHeight?: boolean;
  /** Colour each segment by its position in the rainbow palette. Default true.
   *  Turn it off where the position means nothing, such as a single-item strip
   *  (Recovery's StepDisclosure), not for looks. */
  hue?: boolean;
  /** Where this strip starts in the rainbow palette. Default 0. Strips stacked
   *  on one page would otherwise repeat each column's colour straight down, so
   *  a caller rendering a group gives each strip its own offset. The settings
   *  tree takes its offsets from HUE_OFFSET. */
  hueOffset?: number;
  variant?: SelectorVariant;
  /** Pin every segment to the widest one's measured width, at least
   *  MIN_PINNED_WIDTH where the row has room for it, instead of letting each
   *  hug its label. Under "well" it also fixes the height at `--badge-md`,
   *  which makes the big page-level picker. */
  equalWidth?: boolean;
  /** Hug the segments instead of spanning the box, for a strip that shares a
   *  toolbar row with a search field or buttons, and for page tabs. */
  inline?: boolean;
  /** Disables every item (e.g. SourceToggle mid-restore). A per-item
   *  `disabled` still applies on top of this. */
  disabled?: boolean;
  className?: string;
}

/**
 * HUE_OFFSET is where each selector in the settings tree starts in the palette.
 * The selectors are spread over several files and would all repeat the same
 * colours at the default offset; pages/settings/hueOffsets.test.ts requires
 * every hued selector in that tree to take its offset from here.
 *
 * There are more selectors than the palette has colours, so some share a
 * start: `drillKind` with the first label row, and `theme`, `notifyOn` and
 * `mcpClient` the last colour. `theme` sits in General, `notifyOn` in
 * Notifications and the client picker in a dialog from System, so none of
 * them is on screen with another. General spends all eight positions, so a
 * selector added there has no free start and the table needs rethinking
 * rather than another entry.
 */
export const HUE_OFFSET = {
  tabs: 0,
  /** One row per control axis, each a colour further along, so the block reads
   *  as one group. */
  labels: 1,
  shape: 5,
  motion: 6,
  theme: 7,
  notifyOn: 7,
  drillKind: 1,
  mcpClient: 7,
} as const;

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

// Kept as separate fields because segments mix them: a reactive strip drops the
// gap, and segmentPadding swaps the padding for the button-height box.
const SIZE: Record<
  SelectorSize,
  { gap: string; padding: string; glyphPadding: string; text: string }
> = {
  sm: { gap: "gap-1", padding: "px-2 py-0.5", glyphPadding: "px-2 glim-seg-btn", text: "text-xs" },
  md: { gap: "gap-1.5", padding: "px-3 py-1", glyphPadding: "px-3 glim-seg-btn", text: "text-xs" },
  // `glim-seg` sets the page-level height instead of the padding, so the strip
  // neither shrinks in glyph mode nor sits lower than a button.
  lg: { gap: "gap-2", padding: "px-3 glim-seg", glyphPadding: "px-3 glim-seg", text: "text-sm" },
};

/**
 * segmentPadding gives a segment with a glyph, or any segment of a strip beside
 * a button, the button height, since a box sized by its text comes out shorter
 * than the controls around it. Text-only strips keep the compact padding, and
 * grow to the button height only under a coarse pointer, where a 20px pill is
 * too small to hit.
 */
function segmentPadding(size: SelectorSize, hasGlyph: boolean, buttonHeight: boolean): string {
  return hasGlyph || buttonHeight ? SIZE[size].glyphPadding : `${SIZE[size].padding} pointer-coarse:min-h-(--btn-h)`;
}

// MIN_PINNED_WIDTH is the narrowest a pinned segment gets. The shared floor
// keeps the pinned strips on one page at one width instead of each following
// its own widest label; a label that needs more still gets it, up to
// MAX_PINNED_WIDTH, so one long translation does not widen every segment of
// its strip. Past that the label wraps, or the row is laid out by content.
export const MIN_PINNED_WIDTH = 200;
export const MAX_PINNED_WIDTH = 352;

/**
 * pinnedWidth is the width every pinned segment gets: the widest label within
 * the cap, and the shared floor where the row holds that many. On a phone the
 * floor gives way to an even share of the row, so two short options sit side
 * by side instead of one above the other. The share is rounded down, since an
 * exact one comes back a hair short of fitting.
 */
export function pinnedWidth(widest: number, count: number, room: number, gap: number): number {
  const share = Math.floor((room - gap * (count - 1)) / count);
  return Math.min(Math.max(widest, Math.min(MIN_PINNED_WIDTH, share)), MAX_PINNED_WIDTH);
}

/**
 * segmentWidths measures each segment with its label on one line and at its
 * narrowest. The segments are held to their flex share, so each is set to the
 * size being read and given its own style back straight after. The width
 * comes from the computed style, since a window's scale-in animation shrinks
 * the bounding box.
 */
function segmentWidths(segs: HTMLElement[]): SegmentWidths[] {
  const own = segs.map((s) => s.style.cssText);
  const at = (width: string) => {
    for (const s of segs) Object.assign(s.style, { flex: "none", width, minWidth: "0", maxWidth: "none" });
    return segs.map((s) => parseFloat(getComputedStyle(s).width));
  };
  const oneLine = at("max-content");
  const narrowest = at("min-content");
  segs.forEach((s, i) => (s.style.cssText = own[i]));
  return segs.map((_, i) => ({ oneLine: oneLine[i], narrowest: narrowest[i] }));
}

interface RowLayout {
  pinned: number;
  room: number;
  gap: number;
  /** The track's own padding, which a hugging track adds to its segments. */
  inset: number;
  perRow: number;
  byContent: boolean;
}

const sameLayout = (a: RowLayout | null, b: RowLayout) =>
  !!a &&
  a.perRow === b.perRow &&
  a.byContent === b.byContent &&
  Math.abs(a.pinned - b.pinned) < 0.5 &&
  Math.abs(a.room - b.room) < 0.5 &&
  a.gap === b.gap &&
  a.inset === b.inset;

// The navigation math is pure so Selector.test.ts can cover it without a DOM.

export type SelectorNavKey = "ArrowRight" | "ArrowLeft" | "Home" | "End";
const NAV_KEYS: readonly string[] = ["ArrowRight", "ArrowLeft", "Home", "End"];

/**
 * stepFor returns the index step for an arrow key. ArrowRight moves one segment
 * to the right on screen, which under RTL is one index back.
 */
export function stepFor(key: SelectorNavKey, rtl: boolean): -1 | 0 | 1 {
  if (key === "ArrowRight") return rtl ? -1 : 1;
  if (key === "ArrowLeft") return rtl ? 1 : -1;
  return 0; // Home/End jump rather than step
}

/**
 * nextFocusIndex returns the roving-tabindex target for a key press. Home and
 * End jump to the ends, arrows step and wrap around. With nothing focused yet
 * (current < 0) it starts at the first item; an empty strip returns -1.
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
 * rovedIndex returns the item that holds tabIndex 0, the strip's one tab stop.
 * That is the active item unless it is disabled, then the first enabled one, so
 * a strip whose active item was disabled under it (Files' "original" chip
 * losing its target path) stays reachable. With every item disabled it is 0.
 */
export function rovedIndex(disabled: boolean[], activeIndex: number): number {
  if (activeIndex >= 0 && !disabled[activeIndex]) return activeIndex;
  const firstEnabled = disabled.findIndex((d) => !d);
  return firstEnabled >= 0 ? firstEnabled : 0;
}

// SelectorTab is one segment, a component of its own so that each segment can
// hold its own tooltip hook.
interface SelectorTabProps {
  item: SelectorItem;
  many: boolean;
  on: boolean;
  disabled: boolean;
  roved: boolean;
  className: string;
  style?: CSSProperties;
  /** How the segment sits in the row. It goes on whatever element is the
   *  row's flex item, the button or the span a disabled tip wraps it in. */
  flex: CSSProperties;
  /** Lets the label break onto a second line, for a pinned segment, where
   *  segmentLayout decides when a label may wrap. Elsewhere it truncates. */
  wraps: boolean;
  onSelect: () => void;
  /** The label-mode axis the strip follows; see labelAxis in Selector. */
  axis: ControlAxis;
}

function SelectorTab({
  item,
  many,
  on,
  disabled,
  roved,
  className,
  style,
  flex,
  wraps,
  onSelect,
  axis,
}: SelectorTabProps) {
  const labelMode = useLabelMode(axis);

  const nameHidden = !!item.iconOnly || (hidesLabel(labelMode) && !!item.icon);
  // Reactive mode brings the label back on hover. Not for `iconOnly`: that
  // segment sits in a strip with no room for words (PathModeSwitch in a path
  // row), and a label mode must not override the call site.
  const reactive = labelMode === "reactive" && !!item.icon && !item.iconOnly;

  // A `tip` is written as the fuller version of the label, so it stands in for
  // the label instead of joining it; without one a hidden label is used.
  // `title`, the reason for the segment's state, is appended. Identical parts
  // collapse because the Settings tab strip passes `title: label` for its
  // truncated labels.
  const explains = item.tip ?? (nameHidden ? item.label : undefined);
  const tip = [...new Set([explains, item.title].filter(Boolean))].join(" — ") || undefined;
  // No bubble in reactive mode, where hovering brings the words back. A
  // disabled segment keeps it: it takes no hover, so its tip is the only thing
  // that says why.
  const tooltip = useTipBubble(reactive && !disabled ? undefined : tip, disabled);

  return (
    <>
      {tooltip.wrap(
        <button
          ref={tooltip.ref}
          type="button"
          data-sel-id={item.id}
          role={many ? undefined : "tab"}
          aria-selected={many ? undefined : on}
          aria-pressed={many ? on : undefined}
          aria-label={nameHidden ? item.label : undefined}
          aria-describedby={tooltip.describedBy}
          disabled={disabled}
          tabIndex={roved ? 0 : -1}
          style={
            reactive
              ? ({ ...style, ...flex, "--reactive-chars": labelWidth(item.label) } as CSSProperties)
              : { ...style, ...flex }
          }
          className={`${className}${reactive ? " glim-reactive" : ""}`}
          onClick={onSelect}
          {...tooltip.handlers}
        >
          {/* A segment without a glyph keeps its text in every mode: an empty
              segment is unusable, an uneven strip only looks odd. */}
          {labelMode !== "text" && item.icon}
          {reactive ? (
            /* No `truncate` here: it would fight the max-width reveal. */
            <span data-sel-label className="glim-label-reactive">
              {item.label}
            </span>
          ) : (
            (labelMode === "text" || !item.iconOnly) &&
            (!hidesLabel(labelMode) || !item.icon) && (
              <span data-sel-label className={wraps ? undefined : "truncate"}>
                {item.label}
              </span>
            )
          )}
        </button>,
        flex,
      )}
      {tooltip.bubble}
    </>
  );
}

export function Selector(props: SelectorProps) {
  const {
    items,
    label,
    size = "md",
    buttonHeight = false,
    hue = true,
    hueOffset = 0,
    variant = "well",
    equalWidth = false,
    inline = false,
    disabled = false,
    className = "",
  } = props;
  const well = variant === "well";

  // `lg` strips are the page-level tabs and pickers and follow the Tabs label
  // mode; smaller strips sit in form rows beside buttons and follow the Buttons
  // mode. It is the same split `.glim-seg` and `.glim-seg-btn` draw in the
  // stylesheet.
  const labelAxis: ControlAxis = size === "lg" ? "tabs" : "buttons";
  // Pinning is decided for the whole row, so the strip reads the mode as well.
  const labelModeForStrip = useLabelMode(labelAxis);

  // A strip whose labels are all off screen does not pin: the pinned width is
  // measured from text and floored at MIN_PINNED_WIDTH, which around a lone
  // glyph is only empty pill. The `lg` tabs are exempt, so the Settings tab row
  // stays a row of equal tabs in glyph mode.
  const labelsOffScreen = hidesLabel(labelModeForStrip) && items.every((i) => !!i.icon);
  const reactiveStrip = labelModeForStrip === "reactive" && items.every((i) => !!i.icon && !i.iconOnly);
  const pinWidth = equalWidth && !(labelsOffScreen && size !== "lg");

  const many = props.select === "many";
  const chosen = props.select === "many" ? props.active : null;
  const only = props.select === "many" ? null : props.active;
  const { onChange } = props;
  const auto = !many;

  const strip = useRef<HTMLDivElement>(null);

  // How pinned segments share the row, from segmentLayout. Pinning needs each
  // segment's natural width, which only the DOM knows; a flex share alone
  // (`flex-1`) lets a shrink-to-fit parent add up zero bases and truncate the
  // widest label. The room is the track's own content box while it spans, and
  // its parent's while it hugs, since a hugging track is only as wide as the
  // segments it holds. Measured again when the box or a segment resizes: a
  // late web font changes what a label needs.
  //
  // itemsKey stands in for `items` in the deps. Callers rebuild the array on
  // every render, and measuring each time would be wasted work.
  //
  // A segment that turns disabled with a tip is mounted afresh inside the
  // tip's wrapper, so the segments are looked up on every measure, and the
  // watch moves to the new ones through disabledKey. A detached segment has
  // no computed width, and measuring it would drop the pinned width.
  const itemsKey = items.map((it) => it.label).join("\n");
  const disabledKey = items.map((it) => (disabled || it.disabled ? "1" : "0")).join("");
  const [layout, setLayout] = useState<RowLayout | null>(null);
  useLayoutEffect(() => {
    if (!pinWidth) {
      setLayout(null);
      return;
    }
    const el = strip.current!;
    const box = inline ? el.parentElement! : el;
    const segments = () => Array.from(el.querySelectorAll<HTMLElement>("[data-sel-id]"));
    if (segments().length === 0) return;
    const measure = () => {
      const segs = segments();
      const own = getComputedStyle(el);
      const inset = parseFloat(own.paddingLeft) + parseFloat(own.paddingRight);
      const outer = inline ? getComputedStyle(box) : own;
      const room =
        box.clientWidth -
        parseFloat(outer.paddingLeft) -
        parseFloat(outer.paddingRight) -
        (inline ? inset : 0);
      // A hidden box measures nothing; it is laid out once it shows.
      if (room <= 0) return;
      const gap = parseFloat(own.columnGap) || 0;
      const widths = segmentWidths(segs);
      const widest = Math.max(...widths.map((w) => w.oneLine));
      let pinned = pinnedWidth(widest, segs.length, room, gap);
      let rows = segmentLayout(room, pinned, widths, gap);
      // A hugging strip that wraps takes the floor its rows have room for, so
      // a tab is as wide in a strip of eight as in a strip of four.
      if (inline && !rows.byContent && rows.perRow < segs.length) {
        pinned = pinnedWidth(widest, rows.perRow, room, gap);
        rows = segmentLayout(room, pinned, widths, gap);
      }
      const next: RowLayout = { pinned, room, gap, inset, ...rows };
      setLayout((prev) => (sameLayout(prev, next) ? prev : next));
    };
    measure();
    const watch = new ResizeObserver(measure);
    watch.observe(box);
    for (const seg of segments()) watch.observe(seg);
    return () => watch.disconnect();
  }, [pinWidth, inline, itemsKey, disabledKey, size, labelModeForStrip]);

  // A pinned segment keeps the pinned width as its floor and grows into its
  // share of the row, so every row ends at the track's edge once it wraps. The
  // basis leaves one gap of slack against sub-pixel rounding, and the growth
  // takes it back. The floor gives way to the room, so a box narrower than one
  // segment squeezes it instead of pushing it out of the card. A segment that
  // hugs its label grows too, and so does a pinned one whose row is laid out
  // by content.
  //
  // In a hugging strip the pinned segments keep their width, and the track is
  // held to one row's worth of them, a pixel over against rounding, so it wraps
  // into even rows that end where the segments do.
  const byContent = pinWidth && !!layout?.byContent;
  const pinnedRows = pinWidth && layout && !layout.byContent ? layout : null;
  const hugged = inline && pinnedRows;
  const segmentFlex: CSSProperties = hugged
    ? { flex: `0 0 ${Math.min(hugged.pinned, hugged.room)}px` }
    : pinnedRows
      ? {
          minWidth: `${Math.min(pinnedRows.pinned, pinnedRows.room)}px`,
          flex: `1 0 calc((100% - ${pinnedRows.perRow} * ${pinnedRows.gap}px) / ${pinnedRows.perRow})`,
        }
      : { flex: "1 0 auto" };
  const trackStyle: CSSProperties | undefined = byContent
    ? { width: "100%", flexWrap: "nowrap" }
    : hugged
      ? {
          maxWidth: `${hugged.perRow * hugged.pinned + (hugged.perRow - 1) * hugged.gap + hugged.inset + 1}px`,
        }
      : undefined;

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

    // Read the direction off the strip, so dir="rtl" works without a prop.
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
      // The "well" groove is surface3 to stand out from both parents it sits
      // on, a Card (surface) and a CadenceBuilder well (surface2), and 0.2rem is
      // the thinnest ring that still reads as a groove. A hugging track is
      // `w-fit`; `self-start` would hug too, but it top-aligns the strip in rows
      // that centre a label beside it. Tracks wrap rather than scroll, except
      // one laid out by content, whose fit already allows for a pixel of
      // rounding that would otherwise push its last segment onto a row alone.
      className={[
        "flex flex-wrap items-center",
        inline ? "w-fit max-w-full" : "w-full",
        well
          ? "gap-[0.2rem] rounded-pill bg-carbon-surface3 p-[0.2rem]"
          : "gap-1",
        className,
      ]
        .filter(Boolean)
        .join(" ")}
      style={trackStyle}
    >
      {items.map((item, i) => {
        const on = isOn(item.id);
        const itemDisabled = disabledFlags[i];
        const cls = [
          "inline-flex min-w-0 max-w-full items-center justify-center font-medium",
          // Well segments are rounded too, so the selected pill follows the
          // shape setting along with the groove.
          well
            ? "rounded-pill [transition:background-color_120ms_ease]"
            : "rounded-pill transition-colors",
          "disabled:opacity-50 disabled:cursor-not-allowed",
          // An iconOnly segment is all glyph, and on an icon-only badge only
          // the fill takes the colour (design-language.md), so it skips the
          // glyph tint of glim-hue-icon.
          hue ? (item.iconOnly ? "glim-hue" : "glim-hue glim-hue-icon") : "",
          on
            ? "glim-active bg-accent text-accentContrast"
            : // Idle well segments are transparent, which puts
              // text-carbon-textSub on surface3 at 4.57:1 in dark mode, just
              // above WCAG AA. A deeper groove needs a lighter text token.
              well
              ? "bg-transparent text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
              : "bg-carbon-surface2 text-carbon-textSub hover:bg-carbon-surface3 hover:text-carbon-text",
          // No gap while a reactive segment is closed: the collapsed label is
          // still a flex item, so the gap would push the glyph off centre. The
          // label brings its own margin when it opens.
          reactiveStrip ? "gap-0" : SIZE[size].gap,
          SIZE[size].text,
          // An unpinned iconOnly segment is the 32px square of Badge's
          // size="icon"; Selector does not render through Badge, so the two
          // sizes have to be kept in step by hand.
          well && equalWidth
            ? `text-center h-[var(--badge-md)] ${SIZE[size].padding}`
            : equalWidth
              ? `text-center ${segmentPadding(size, !!item.icon, buttonHeight)}`
              : item.iconOnly
                ? "h-8 w-8 p-0"
                : segmentPadding(size, !!item.icon, buttonHeight),
        ]
          .filter(Boolean)
          .join(" ");

        return (
          <SelectorTab
            key={item.id}
            item={item}
            axis={labelAxis}
            many={many}
            on={on}
            disabled={itemDisabled}
            roved={i === roved}
            className={cls}
            style={hue ? (hueVars(i + hueOffset) as CSSProperties) : undefined}
            flex={segmentFlex}
            wraps={pinWidth}
            onSelect={() => {
              if (!itemDisabled) onChange(item.id);
            }}
          />
        );
      })}
    </div>
  );
}
