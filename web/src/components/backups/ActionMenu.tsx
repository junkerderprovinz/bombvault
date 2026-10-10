import { useEffect, useLayoutEffect, useRef, useState, type KeyboardEvent, type ReactNode } from "react";
import { createPortal } from "react-dom";

export interface MenuAction {
  id: string;
  label: string;
  glyph: ReactNode;
  onSelect: () => void;
}

/** Where a menu opens: at a point of the window, or above a control. */
export type MenuPlace = { x: number; y: number } | { anchor: HTMLElement };

const EDGE = 8;

function placeOf(place: MenuPlace, panel: HTMLElement): { left: number; top: number } {
  const width = panel.offsetWidth;
  const height = panel.offsetHeight;
  const maxLeft = window.innerWidth - width - EDGE;
  const maxTop = window.innerHeight - height - EDGE;
  if ("anchor" in place) {
    const rect = place.anchor.getBoundingClientRect();
    const rtl = getComputedStyle(place.anchor).direction === "rtl";
    const above = rect.top - height - EDGE;
    return {
      left: Math.max(EDGE, Math.min(rtl ? rect.left : rect.right - width, maxLeft)),
      top: above >= EDGE ? above : Math.min(rect.bottom + EDGE, maxTop),
    };
  }
  return { left: Math.max(EDGE, Math.min(place.x, maxLeft)), top: Math.max(EDGE, Math.min(place.y, maxTop)) };
}

/**
 * ActionMenu is a short list of things to do, opened by a right-click at the
 * pointer or by a button above itself. It closes on a choice, on Escape and
 * on a press anywhere else.
 */
export function ActionMenu({
  label,
  heading,
  actions,
  place,
  onClose,
}: {
  /** What the menu is about, for a screen reader. */
  label: string;
  /** A line above the actions, such as how many rows they act on. */
  heading?: string;
  actions: MenuAction[];
  place: MenuPlace;
  onClose: () => void;
}) {
  const panel = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState<{ left: number; top: number } | null>(null);
  const closeRef = useRef(onClose);
  closeRef.current = onClose;
  const placeRef = useRef(place);
  placeRef.current = place;

  useLayoutEffect(() => {
    if (panel.current) setPos(placeOf(place, panel.current));
  }, [place]);

  // The first action takes the focus once the menu stands where it belongs.
  // Before that it is hidden and cannot.
  const placed = pos !== null;
  useLayoutEffect(() => {
    if (placed) panel.current?.querySelector<HTMLElement>('[role="menuitem"]')?.focus();
  }, [placed]);

  useEffect(() => {
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const close = () => closeRef.current();
    // A press on the button that opened the menu is that button's to answer.
    const onPress = (e: PointerEvent) => {
      const on = (el: Element | null | undefined) => e.target instanceof Node && !!el?.contains(e.target);
      const opener = "anchor" in placeRef.current ? placeRef.current.anchor : null;
      if (!on(panel.current) && !on(opener)) close();
    };
    // A menu at the pointer belongs to what was under it and goes when that
    // scrolls away. One above a control follows the control.
    const onScroll = () => {
      if (!("anchor" in placeRef.current)) close();
      else if (panel.current) setPos(placeOf(placeRef.current, panel.current));
    };
    document.addEventListener("pointerdown", onPress, true);
    document.addEventListener("scroll", onScroll, true);
    window.addEventListener("resize", close);
    window.addEventListener("blur", close);
    return () => {
      document.removeEventListener("pointerdown", onPress, true);
      document.removeEventListener("scroll", onScroll, true);
      window.removeEventListener("resize", close);
      window.removeEventListener("blur", close);
      // The focus went with the menu unless a press elsewhere took it.
      if (opener && document.contains(opener) && document.activeElement === document.body) opener.focus();
    };
  }, []);

  function onKeyDown(e: KeyboardEvent) {
    const items = Array.from(panel.current?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? []);
    const at = items.indexOf(document.activeElement as HTMLElement);
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      const step = e.key === "ArrowDown" ? 1 : -1;
      items[(at + step + items.length) % items.length]?.focus();
    } else if (e.key === "Home" || e.key === "End") {
      e.preventDefault();
      items[e.key === "Home" ? 0 : items.length - 1]?.focus();
    } else if (e.key === "Escape" || e.key === "Tab") {
      e.preventDefault();
      onClose();
    }
  }

  return createPortal(
    <div
      ref={panel}
      role="menu"
      aria-label={label}
      onKeyDown={onKeyDown}
      onContextMenu={(e) => e.preventDefault()}
      style={{ left: pos?.left ?? 0, top: pos?.top ?? 0, visibility: pos ? "visible" : "hidden" }}
      className="fixed z-50 flex w-max min-w-[13rem] max-w-[calc(100vw-1rem)] flex-col rounded-control bg-carbon-surface p-1.5 shadow-2xl ring-1 ring-carbon-border"
    >
      {heading && (
        <p className="px-3 pb-1 pt-2 text-xs font-medium uppercase tracking-wider text-carbon-textMuted">{heading}</p>
      )}
      {actions.map((action) => (
        <button
          key={action.id}
          type="button"
          role="menuitem"
          onClick={() => {
            onClose();
            action.onSelect();
          }}
          className="flex items-center gap-2.5 rounded-control px-3 py-2 text-start text-sm text-carbon-text hover:bg-carbon-hover focus-visible:bg-carbon-hover focus-visible:outline-none pointer-coarse:min-h-11"
        >
          <span className="flex flex-none text-accentText">{action.glyph}</span>
          {action.label}
        </button>
      ))}
    </div>,
    document.body
  );
}
