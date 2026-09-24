// Selector is the app's one horizontal selector. Tabs, filter bars, segmented
// controls and pickers all render through it, so they cannot drift apart
// (design-language.md, "The one horizontal selector").
//
// The strip is a single tab stop. Arrow keys, Home and End move between
// segments with a roving tabindex, and the arrows follow the reading direction
// under dir="rtl". With select="one" moving also selects, as in a tab strip;
// with select="many" the segments are toggle buttons.
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

/** "chip" gives every segment a pill of its own. "well" sets flush segments in
 *  a shared groove, and only the chosen one is filled. A plain "well" is the
 *  small, content-hugging strip for cards; with `equalWidth` and size="lg" it
 *  is the big page-level picker. */
export type SelectorVariant = "chip" | "well";

interface SelectorCommon {
  items: SelectorItem[];
  /** Accessible name for the strip, e.g. "Sort", "Settings sections". */
  label: string;
  /** `md` (default) is the toolbar chip, `sm` the dense strip (heatmap toggle,
   *  weekday pills), `lg` the page-level scale (Settings tabs and pickers). */
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
  /** Page-tab look with no idle background, instead of the toolbar chip's idle
   *  `bg-carbon-surface2` pill. Ignored under `variant="well"`. */
  plain?: boolean;
  variant?: SelectorVariant;
  /** Pin every segment to the widest one's measured width, at least
   *  MIN_PINNED_WIDTH, instead of letting each hug its label. Under "well" it
   *  also fixes the height at `--badge-md`, which makes the big page-level
   *  picker. */
  equalWidth?: boolean;
  /** A fixed CSS width for every segment, used instead of `equalWidth`'s
   *  measurement. A measured width follows the widest translation and can
   *  outgrow the page; a stage from lib/controls is known before the first
   *  paint and bounded. */
  segmentWidth?: string;
  /** Deepens the idle chip fill to `bg-carbon-surface3`, for a "chip" strip on
   *  a surface2 background, where the default idle fill would disappear.
   *  Ignored under "well". */
  raised?: boolean;
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
 * The palette has eight colours and the tree ten selectors, so `drillKind`
 * shares the first label row's start and `mcpClient` shares `notifyOn`'s. Each
 * pair sits in different tabs, `notifyOn` in Notifications and the client
 * picker in System, so neither pair is ever on screen together.
 */
export const HUE_OFFSET = {
  tabs: 0,
  /** The three label-mode rows take +0..2 by axis, so the block reads as one
   *  group. */
  labels: 1,
  shape: 4,
  motion: 5,
  theme: 6,
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
 * than the controls around it. Text-only strips keep the compact padding.
 */
function segmentPadding(size: SelectorSize, hasGlyph: boolean, buttonHeight: boolean): string {
  return hasGlyph || buttonHeight ? SIZE[size].glyphPadding : SIZE[size].padding;
}

// MIN_PINNED_WIDTH is the narrowest a pinned segment gets. The shared floor
// keeps the pinned strips on one page at one width instead of each following
// its own widest label; a label that needs more still gets it.
export const MIN_PINNED_WIDTH = 200;

// The groove's padding and the gap between its segments, both 0.2rem.
const GROOVE_STEP = 3.2;

/**
 * rowFill returns the width a pinned segment takes in a row of `available`
 * pixels. While the whole strip fits on one line the segment keeps its pinned
 * width and the groove hugs it. Once it does not fit, the row is divided into
 * equal columns instead, so the groove stops drawing track no segment stands
 * on. The column count is the widest one that divides the strip: four segments
 * in a row that holds three go two and two, and three in the same row go one
 * per row rather than leaving a column empty underneath.
 */
export function rowFill(pinned: number, count: number, available: number): number {
  if (available <= 0) return pinned;
  const fits = Math.floor((available + GROOVE_STEP) / (pinned + GROOVE_STEP));
  if (fits >= count) return pinned;
  let columns = 1;
  for (let c = Math.max(1, fits); c > 1; c--) {
    if (count % c === 0) {
      columns = c;
      break;
    }
  }
  return (available - (columns - 1) * GROOVE_STEP) / columns;
}

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
  onSelect: () => void;
  registerRef: (el: HTMLButtonElement | null) => void;
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
  onSelect,
  registerRef,
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
          ref={(el) => {
            tooltip.ref(el);
            registerRef(el);
          }}
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
              ? ({ ...style, "--reactive-chars": labelWidth(item.label) } as CSSProperties)
              : style
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
            <span className="glim-label-reactive">{item.label}</span>
          ) : (
            (labelMode === "text" || !item.iconOnly) &&
            (!hidesLabel(labelMode) || !item.icon) && (
              <span className="truncate">{item.label}</span>
            )
          )}
        </button>,
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
    plain = false,
    variant = "chip",
    equalWidth = false,
    segmentWidth,
    raised = false,
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

  // Pinning needs the widest segment's natural width, which only a DOM
  // measurement gives. A flex share (`flex-1`) does not work: a shrink-to-fit
  // parent adds up its zero basis and truncates the widest label.
  //
  // itemsKey stands in for `items` in the effect deps. Callers rebuild the array
  // on every render, and re-measuring each time would flash the strip back to
  // ragged widths.
  const itemsKey = items.map((it) => it.label).join("");
  const [matchedWidth, setMatchedWidth] = useState<number | null>(null);
  const itemRefs = useRef<Array<HTMLButtonElement | null>>([]);

  // Unpin when the labels or the size change. A segment with an explicit width
  // reports that width back, so it has to render at its natural width before it
  // can be measured again. Both passes are layout effects and finish before the
  // browser paints.
  useLayoutEffect(() => {
    if (pinWidth) setMatchedWidth(null);
  }, [pinWidth, itemsKey, size]);

  useLayoutEffect(() => {
    if (!pinWidth || matchedWidth !== null) return;
    // Refs past the current item count are left over from a longer render.
    const nodes = itemRefs.current
      .slice(0, items.length)
      .filter((n): n is HTMLButtonElement => n !== null);
    if (nodes.length === 0) return;
    const widest = Math.max(
      ...nodes.map((n) => n.getBoundingClientRect().width),
      MIN_PINNED_WIDTH,
    );
    setMatchedWidth(widest);
  }, [pinWidth, matchedWidth, itemsKey, size, items.length]);


  const many = props.select === "many";
  const chosen = props.select === "many" ? props.active : null;
  const only = props.select === "many" ? null : props.active;
  const { onChange } = props;
  const auto = !many;

  const strip = useRef<HTMLDivElement>(null);

  // rowFill needs the width the groove is allowed to take, which is the
  // parent's content box. Measuring the groove itself would feed the segment
  // widths it just set back into the next measurement, since the groove is
  // `w-fit`. A box that measures nothing (a hidden card, a run under jsdom)
  // leaves the pinned width alone.
  const [rowWidth, setRowWidth] = useState<number | null>(null);
  useLayoutEffect(() => {
    const row = well && pinWidth ? strip.current?.parentElement : null;
    if (!row) return;
    const measure = () => {
      const style = getComputedStyle(row);
      const inner =
        row.clientWidth -
        parseFloat(style.paddingInlineStart) -
        parseFloat(style.paddingInlineEnd);
      setRowWidth(inner - GROOVE_STEP * 2);
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(row);
    return () => observer.disconnect();
  }, [well, pinWidth]);

  const pinnedWidth =
    matchedWidth !== null && well && rowWidth !== null
      ? rowFill(matchedWidth, items.length, rowWidth)
      : matchedWidth;

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
      // the thinnest ring that still reads as a groove. `w-fit` hugs the
      // segments; `self-start` would too, but it top-aligns the strip in rows
      // that centre a label beside it. Both variants wrap, since a pinned strip
      // is N times the widest label wide.
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
          // Well segments are rounded too, so the selected pill follows the
          // shape setting along with the groove.
          well
            ? "rounded-control [transition:background-color_120ms_ease]"
            : "rounded-control transition-colors",
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
              : plain
                ? "text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
                : raised
                  ? "bg-carbon-surface3 text-carbon-textSub hover:bg-carbon-hoverRaised hover:text-carbon-text"
                  : "bg-carbon-surface2 text-carbon-textSub hover:bg-carbon-surface3 hover:text-carbon-text",
          // No gap while a reactive segment is closed: the collapsed label is
          // still a flex item, so the gap would push the glyph off centre. The
          // label brings its own margin when it opens.
          reactiveStrip ? "gap-0" : SIZE[size].gap,
          SIZE[size].text,
          // Pinned segments are flex-none, so a row that is too narrow wraps
          // instead of squeezing them. An unpinned iconOnly segment is the 32px
          // square of Badge's size="icon"; Selector does not render through
          // Badge, so the two sizes have to be kept in step by hand.
          well && equalWidth
            ? `flex-none justify-center text-center h-[var(--badge-md)] ${SIZE[size].padding}`
            : equalWidth
              ? `flex-none justify-center text-center ${segmentPadding(size, !!item.icon, buttonHeight)}`
              : item.iconOnly
                ? "justify-center h-8 w-8 p-0"
                : segmentPadding(size, !!item.icon, buttonHeight),
        ]
          .filter(Boolean)
          .join(" ");

        const hueStyle = hue
          ? (hueVars(i + hueOffset) as CSSProperties)
          : undefined;
        const widthStyle: CSSProperties | undefined =
          segmentWidth
            ? { width: segmentWidth }
            : pinWidth && pinnedWidth !== null
            ? { width: `${pinnedWidth}px` }
            : undefined;
        const itemStyle =
          hueStyle || widthStyle ? { ...hueStyle, ...widthStyle } : undefined;

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
