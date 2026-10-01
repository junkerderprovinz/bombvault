// SettingsRail is the column of Settings pages beside the content. It is drawn
// like the sidebar's rows, in the sidebar's colours, so the two read as one
// navigation: the pages are links, the current one is filled with its hue, and
// a long press lifts a tile to move it.
//
// The rail keeps to the height of the scroller it sits in and its tiles share
// that height, so every page is in view without scrolling the rail. Below the
// lg breakpoint, or when the Tabs labels are set to glyphs, it narrows to the
// glyphs and names each tile in its bubble.
import {
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
  type KeyboardEvent,
  type PointerEvent,
  type ReactNode,
} from "react";
import { Link } from "react-router-dom";
import { hueVars } from "../lib/appearance";
import { useReorder } from "../lib/dragLift";
import { useLabelMode } from "../lib/useLabelMode";
import { useIsWide } from "../lib/useIsWide";
import { useTipBubble } from "../lib/useTipBubble";
import { HUE_OFFSET } from "./Selector";

export interface RailItem {
  id: string;
  label: string;
  icon: ReactNode;
  to: string;
}

interface SettingsRailProps {
  items: RailItem[];
  active: string;
  /** Accessible name of the navigation. */
  label: string;
  onReorder: (ids: string[]) => void;
}

export function SettingsRail({ items, active, label, onReorder }: SettingsRailProps) {
  const wide = useIsWide();
  const mode = useLabelMode("tabs");
  const glyphs = !wide || mode === "glyph";
  const reactive = wide && mode === "reactive";
  const showIcon = glyphs || mode !== "text";

  const list = useRef<HTMLDivElement>(null);
  const height = useScrollerHeight();
  const drag = useReorder({
    ids: items.map((i) => i.id),
    container: list,
    attr: "data-rail-id",
    axis: "y",
    arm: "hold",
    enabled: true,
    onReorder,
  });
  const byId = new Map(items.map((i) => [i.id, i] as const));
  const ordered = drag.order.map((id) => byId.get(id)).filter((i): i is RailItem => !!i);

  function onKeyDown(e: KeyboardEvent<HTMLDivElement>) {
    const tiles = Array.from(list.current?.querySelectorAll<HTMLElement>("[data-rail-id]") ?? []);
    const here = tiles.indexOf(document.activeElement as HTMLElement);
    const next =
      e.key === "ArrowDown"
        ? (here + 1) % tiles.length
        : e.key === "ArrowUp"
          ? (here - 1 + tiles.length) % tiles.length
          : e.key === "Home"
            ? 0
            : e.key === "End"
              ? tiles.length - 1
              : -1;
    if (next < 0 || tiles.length === 0) return;
    e.preventDefault();
    tiles[next].focus();
  }

  return (
    <nav
      aria-label={label}
      // Sticky at the scroller's top, so the rail stays put while the page
      // scrolls past it. On desktop it moves out of the page padding to 4px
      // from the sidebar, the room the focus ring and a lifted tile's shadow
      // need, since main clips there.
      className={`sticky top-0 flex shrink-0 flex-col self-start md:-ms-5 ${glyphs ? "w-13" : "w-51"}`}
      style={height ? { height } : undefined}
    >
      <div
        ref={list}
        onKeyDown={onKeyDown}
        // The list is the tiles' offsetParent, the layout a drag measures in,
        // and keeps 4px inside its clipping edge for the lifted tile's scale.
        className="relative -mx-1 flex min-h-0 flex-1 flex-col gap-1 overflow-y-auto px-1"
      >
        {ordered.map((item, i) => (
          <RailTile
            key={item.id}
            item={item}
            hue={i + HUE_OFFSET.tabs}
            on={item.id === active}
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

interface RailTileProps {
  item: RailItem;
  hue: number;
  on: boolean;
  glyphs: boolean;
  reactive: boolean;
  showIcon: boolean;
  look: string;
  onPress: (e: PointerEvent<HTMLElement>) => void;
  endsDrag: (e: { detail: number }) => boolean;
}

function RailTile({ item, hue, on, glyphs, reactive, showIcon, look, onPress, endsDrag }: RailTileProps) {
  const tooltip = useTipBubble(glyphs ? item.label : undefined);
  // With glyphs only, or names on hover, the glyph stands centred and a name
  // opens as a caption under it, the shape of the phone's bottom bar tab.
  const stacked = glyphs || reactive;
  return (
    <>
      <Link
        to={item.to}
        ref={tooltip.ref}
        data-rail-id={item.id}
        aria-current={on ? "page" : undefined}
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
          "glim-nav-row glim-hue glim-hue-icon group flex w-full min-w-0 shrink-0 grow basis-0 items-center",
          "overflow-hidden rounded-pill text-[15px] font-medium transition-colors select-none [-webkit-touch-callout:none]",
          stacked
            ? `flex-col justify-center gap-0.5 px-2 ${reactive ? "min-h-12 py-1" : "min-h-10 py-1.5"}`
            : "min-h-10 flex-row gap-3 px-3",
          on
            ? "glim-active bg-accent text-accentContrast"
            : "bg-carbon-sidebar text-(--sidebar-text) hover:bg-carbon-hover hover:text-carbon-text",
          look,
        ]
          .filter(Boolean)
          .join(" ")}
        // The tiles share the rail's height, so the row token's fixed height
        // gives way to the flex share.
        style={{ ...hueVars(hue), height: "auto" } as CSSProperties}
      >
        {showIcon && item.icon}
        <span
          className={
            glyphs
              ? "sr-only"
              : reactive
                ? "glim-rail-caption text-xs"
                : // Two lines at 20px fill the 40px floor without cutting a
                  // descender; the clip reaches further for the marks Arabic
                  // and Devanagari set outside the line.
                  "line-clamp-2 overflow-clip [overflow-clip-margin:3px] break-words text-balance leading-5"
          }
        >
          {item.label}
        </span>
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

