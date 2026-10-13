// SettingsRail is the navigation between the Settings pages. It is drawn like
// the sidebar's rows, in the sidebar's colours, so the two read as one
// navigation: the pages are links, the current one is filled with its hue, and
// a long press lifts a tile to move it.
//
// Where the window leaves room for it, the rail is a column beside the page
// that keeps to the height of the scroller it sits in, its tiles sharing that
// height so every page is in view without scrolling the rail. Where a column
// would squeeze the page, it is a strip above the page instead, one row of
// tiles that scrolls sideways and fades out at an edge with more behind it.
import {
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
  type KeyboardEvent,
  type PointerEvent,
  type ReactNode,
  type RefObject,
} from "react";
import { flushSync } from "react-dom";
import { Link } from "react-router-dom";
import { hueVars } from "../lib/appearance";
import { labelWidth } from "../lib/controls";
import { useReorder } from "../lib/dragLift";
import { motionStopped } from "../lib/motion";
import { useLabelMode } from "../lib/useLabelMode";
import { useTipBubble } from "../lib/useTipBubble";

export interface RailItem {
  id: string;
  label: string;
  icon: ReactNode;
  to: string;
}

export type RailForm = "column" | "strip";

// The room a column takes beside the page, in rem: its own width (w-51, or
// w-13 with glyphs alone) and the gap after it, less the md:-ms-5 that pulls it
// towards the sidebar. Below 40rem the cards of the page start to wrap their
// rows, so a column only stands where the page keeps that much.
const COLUMN_REM = { names: 12.75 + 2.5 - 1.25, glyphs: 3.25 + 2.5 - 1.25 };
const PAGE_REM = 40;

/**
 * useRailForm picks the column where `el`, the box holding the rail and the
 * page, leaves the page 40rem beside it, and the strip elsewhere. A frame that
 * is not laid out keeps the column.
 */
export function useRailForm(el: HTMLElement | null): RailForm {
  const glyphs = useLabelMode("tabs") === "glyph";
  const [form, setForm] = useState<RailForm>("column");
  useLayoutEffect(() => {
    if (!el) return;
    const measure = (sync: boolean) => {
      if (el.clientWidth === 0) return;
      const rem = parseFloat(getComputedStyle(document.documentElement).fontSize) || 16;
      const need = ((glyphs ? COLUMN_REM.glyphs : COLUMN_REM.names) + PAGE_REM) * rem;
      const next: RailForm = el.clientWidth >= need ? "column" : "strip";
      // Synchronously on a resize, so no frame paints the old form at the new width.
      if (sync) flushSync(() => setForm(next));
      else setForm(next);
    };
    measure(false);
    if (typeof ResizeObserver === "undefined") return;
    const watch = new ResizeObserver(() => measure(true));
    watch.observe(el);
    return () => watch.disconnect();
  }, [el, glyphs]);
  return form;
}

interface SettingsRailProps {
  items: RailItem[];
  active: string;
  /** Accessible name of the navigation. */
  label: string;
  form: RailForm;
  onReorder: (ids: string[]) => void;
}

// How far a tile is kept from a strip's edge when it is brought into view: the
// fade's width and a little of the next tile, so the strip shows it goes on.
const STRIP_FADE_PX = 32;
const STRIP_REVEAL_PX = STRIP_FADE_PX + 16;

export function SettingsRail({ items, active, label, form, onReorder }: SettingsRailProps) {
  const strip = form === "strip";
  const mode = useLabelMode("tabs");
  const glyphs = mode === "glyph";
  const reactive = mode === "reactive";
  const showIcon = mode !== "text";

  const list = useRef<HTMLDivElement>(null);
  const height = useScrollerHeight();
  const drag = useReorder({
    ids: items.map((i) => i.id),
    container: list,
    attr: "data-rail-id",
    axis: strip ? "x" : "y",
    arm: "hold",
    enabled: true,
    onReorder,
  });
  const byId = new Map(items.map((i) => [i.id, i] as const));
  const ordered = drag.order.map((id) => byId.get(id)).filter((i): i is RailItem => !!i);
  // The rail is one tab stop, on the open page; the arrow keys walk the rest.
  const stop = ordered.some((i) => i.id === active) ? active : ordered[0]?.id;

  useStripEdges(list, strip);

  // A deep link or a resize lands on the open page's tile at once; a change
  // of page slides to it.
  const shown = useRef(false);
  useLayoutEffect(() => {
    const el = list.current;
    if (!strip || !el) {
      shown.current = false;
      return;
    }
    const tile = el.querySelector<HTMLElement>(`[data-rail-id="${active}"]`);
    const first = !shown.current;
    shown.current = true;
    if (!tile) return;
    reveal(el, tile, !first && !motionStopped());
    // The names widen the tiles once the web font is in.
    if (first) void document.fonts?.ready.then(() => reveal(el, tile, false));
  }, [strip, active]);

  function onKeyDown(e: KeyboardEvent<HTMLDivElement>) {
    const tiles = Array.from(list.current?.querySelectorAll<HTMLElement>("[data-rail-id]") ?? []);
    const here = tiles.indexOf(document.activeElement as HTMLElement);
    const rtl = !!list.current && getComputedStyle(list.current).direction === "rtl";
    // The arrows follow the rail's axis, and in a strip the reading direction.
    const forward = strip ? (rtl ? "ArrowLeft" : "ArrowRight") : "ArrowDown";
    const back = strip ? (rtl ? "ArrowRight" : "ArrowLeft") : "ArrowUp";
    // Alt with an arrow moves the tile itself, the keyboard's way to reorder.
    if (e.altKey && (e.key === forward || e.key === back)) {
      const ids = ordered.map((i) => i.id);
      const to = here + (e.key === forward ? 1 : -1);
      if (here < 0 || to < 0 || to >= ids.length) return;
      e.preventDefault();
      [ids[here], ids[to]] = [ids[to], ids[here]];
      onReorder(ids);
      return;
    }
    const next =
      e.key === forward
        ? (here + 1) % tiles.length
        : e.key === back
          ? (here - 1 + tiles.length) % tiles.length
          : e.key === "Home"
            ? 0
            : e.key === "End"
              ? tiles.length - 1
              : -1;
    if (next < 0 || tiles.length === 0) return;
    e.preventDefault();
    if (strip && list.current) {
      tiles[next].focus({ preventScroll: true });
      reveal(list.current, tiles[next], !motionStopped());
    } else {
      tiles[next].focus();
    }
  }

  return (
    <nav
      aria-label={label}
      data-new="settings-tiles"
      data-rail-form={form}
      // A column is sticky at the scroller's top, so it stays put while the
      // page scrolls past it. On desktop it moves out of the page padding to
      // 4px from the sidebar, the room the focus ring and a lifted tile's
      // shadow need, since main clips there.
      className={
        strip
          ? "flex w-full min-w-0 flex-col"
          : `sticky top-0 flex shrink-0 flex-col self-start md:-ms-5 ${glyphs ? "w-13" : "w-51"}`
      }
      style={!strip && height ? { height } : undefined}
    >
      <div
        ref={list}
        onKeyDown={onKeyDown}
        // The list is the tiles' offsetParent, the layout a drag measures in,
        // and keeps 4px inside its clipping edge for the focus ring and the
        // lifted tile's scale.
        className={
          strip
            ? "glim-strip relative -m-1 flex min-w-0 flex-row gap-1 overflow-x-auto overflow-y-hidden overscroll-x-contain p-1 [scrollbar-width:none]"
            : "relative -mx-1 flex min-h-0 flex-1 flex-col gap-1 overflow-y-auto px-1"
        }
      >
        {ordered.map((item, i) => (
          <RailTile
            key={item.id}
            item={item}
            hue={i}
            on={item.id === active}
            tabStop={item.id === stop}
            strip={strip}
            glyphs={glyphs}
            reactive={reactive}
            showIcon={showIcon}
            look={drag.look(item.id)}
            onPress={(e) => drag.press(e, item.id)}
            endsDrag={drag.endsDrag}
          />
        ))}
      </div>
    </nav>
  );
}

/**
 * reveal scrolls a strip the least that brings `tile` clear of its fades. It
 * reads the layout rather than the painted boxes, which the page's entrance
 * still scales while a deep link lands. The strip is the tile's offsetParent,
 * and its scrollLeft counts in the same frame under either reading direction.
 */
function reveal(list: HTMLElement, tile: HTMLElement, smooth: boolean): void {
  const view = list.clientWidth;
  const from = list.scrollLeft;
  const left = tile.offsetLeft;
  const right = left + tile.offsetWidth;
  const room = Math.min(STRIP_REVEAL_PX, Math.max(0, (view - tile.offsetWidth) / 2));
  const by =
    left < from + room ? left - from - room : right > from + view - room ? right - from - view + room : 0;
  // jsdom has no scrollBy.
  if (by !== 0) list.scrollBy?.({ left: by, behavior: smooth ? "smooth" : "instant" });
}

/** useStripEdges marks the sides of a strip with tiles scrolled out past
 *  them, which index.css fades out. */
function useStripEdges(list: RefObject<HTMLElement | null>, on: boolean): void {
  useLayoutEffect(() => {
    const el = list.current;
    if (!on || !el) return;
    const mark = () => {
      const max = el.scrollWidth - el.clientWidth;
      const from = Math.abs(el.scrollLeft);
      const before = from > 1;
      const after = from < max - 1;
      const rtl = getComputedStyle(el).direction === "rtl";
      el.toggleAttribute("data-more-left", rtl ? after : before);
      el.toggleAttribute("data-more-right", rtl ? before : after);
    };
    mark();
    el.addEventListener("scroll", mark, { passive: true });
    const watch = typeof ResizeObserver === "undefined" ? null : new ResizeObserver(mark);
    watch?.observe(el);
    for (const tile of el.children) watch?.observe(tile);
    return () => {
      el.removeEventListener("scroll", mark);
      watch?.disconnect();
      el.removeAttribute("data-more-left");
      el.removeAttribute("data-more-right");
    };
  }, [list, on]);
}

interface RailTileProps {
  item: RailItem;
  hue: number;
  on: boolean;
  tabStop: boolean;
  strip: boolean;
  glyphs: boolean;
  reactive: boolean;
  showIcon: boolean;
  look: string;
  onPress: (e: PointerEvent<HTMLElement>) => void;
  endsDrag: (e: { detail: number }) => boolean;
}

function RailTile({ item, hue, on, tabStop, strip, glyphs, reactive, showIcon, look, onPress, endsDrag }: RailTileProps) {
  const tooltip = useTipBubble(glyphs ? item.label : undefined);
  // In a column, with glyphs only or names on hover, the glyph stands centred
  // and a name opens as a caption under it, the shape of the phone's bottom
  // bar tab. A strip has the room beside the glyph, as every tab strip does.
  const stacked = !strip && (glyphs || reactive);
  const shape = strip
    ? `shrink-0 flex-row justify-center whitespace-nowrap ${glyphs ? "w-(--nav-row-h)" : reactive ? "px-3" : "gap-2 px-4"}`
    : stacked
      ? `w-full grow basis-0 flex-col justify-center gap-0.5 px-2 ${reactive ? "min-h-12 py-1" : "min-h-10 py-1.5"}`
      : "w-full grow basis-0 min-h-10 flex-row gap-3 px-3";
  const name = glyphs
    ? "sr-only"
    : reactive
      ? strip
        ? "glim-label-reactive"
        : "glim-rail-caption text-xs"
      : strip
        ? undefined
        : // Two lines at 20px fill the 40px floor without cutting a
          // descender; the clip reaches further for the marks Arabic and
          // Devanagari set outside the line.
          "line-clamp-2 overflow-clip [overflow-clip-margin:3px] break-words text-balance leading-5";
  const reveals = strip && reactive && !glyphs;
  return (
    <>
      <Link
        to={item.to}
        ref={tooltip.ref}
        data-rail-id={item.id}
        aria-current={on ? "page" : undefined}
        tabIndex={tabStop ? 0 : -1}
        aria-describedby={tooltip.describedBy}
        {...tooltip.handlers}
        // A native link drag would cancel the pointer and end the reorder.
        draggable={false}
        onPointerDown={onPress}
        onClick={(e) => {
          if (endsDrag(e)) {
            e.preventDefault();
            e.stopPropagation();
          }
        }}
        className={[
          "glim-nav-row glim-hue glim-hue-icon group flex min-w-0 items-center",
          "overflow-hidden rounded-pill text-sm font-medium transition-colors select-none [-webkit-touch-callout:none]",
          shape,
          reveals ? "glim-reactive" : "",
          on
            ? "glim-active bg-accent text-accentContrast"
            : "bg-carbon-sidebar text-(--sidebar-text) hover:bg-carbon-hover hover:text-carbon-text",
          look,
        ]
          .filter(Boolean)
          .join(" ")}
        // A column's tiles share its height, so the row token's fixed height
        // gives way to the flex share. A strip's keep the token.
        style={
          {
            ...hueVars(hue),
            ...(strip ? {} : { height: "auto" }),
            ...(reveals ? { "--reactive-chars": labelWidth(item.label) } : {}),
          } as CSSProperties
        }
      >
        {showIcon && item.icon}
        <span className={name}>{item.label}</span>
      </Link>
      {tooltip.bubble}
    </>
  );
}

/** useScrollerHeight follows the inner height of the page scroller (main#bv-main),
 *  the room a sticky rail has. Undefined until it is measured. */
function useScrollerHeight(): number | undefined {
  const [height, setHeight] = useState<number>();
  useLayoutEffect(() => {
    const main = document.getElementById("bv-main");
    if (!main) return;
    const measure = () => {
      const own = getComputedStyle(main);
      setHeight(main.clientHeight - parseFloat(own.paddingTop) - parseFloat(own.paddingBottom));
    };
    measure();
    const watch = new ResizeObserver(measure);
    watch.observe(main);
    return () => watch.disconnect();
  }, []);
  return height;
}
