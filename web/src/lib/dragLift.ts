// Reordering by dragging, as GlimStone's design language describes it: the
// carried item lifts and follows the pointer, the others slide aside while it
// passes, and on release it slides into its gap. Escape or a cancelled pointer
// slides everything back and writes nothing. index.css draws the three states
// (LIFT, SHIFT, SETTLE) from the --drag-* tokens; the code here sets only each
// item's `translate` and switches the classes. The same gesture as
// KnightLoader's, so it moves the same in every app.
import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type PointerEvent as ReactPointerEvent,
  type RefObject,
} from "react";
import { flushSync } from "react-dom";

export const LIFT = "glim-drag-lift";
const SHIFT = "glim-drag-shift";
const SETTLE = "glim-drag-settle";

/** How far a press travels before it becomes a drag: Windows' SM_CXDRAG,
 *  measured per axis. */
const THRESHOLD_PX = 5;

function pastThreshold(dx: number, dy: number): boolean {
  return Math.abs(dx) >= THRESHOLD_PX || Math.abs(dy) >= THRESHOLD_PX;
}

/** A finger arms a drag by holding this long, and moving further than
 *  HOLD_SLOP_PX first makes it a scroll or a tap instead. */
const HOLD_MS = 300;
const HOLD_SLOP_PX = 8;

// The token's scale suits a tile or a phone row. A table row a thousand pixels
// wide would grow by thirty and push its first column off the card, so an item
// wider than this grows by the same pixels a row this wide does.
const LIFT_REF_PX = 360;

interface Point {
  x: number;
  y: number;
}

/** liftScale is the scale a lifted node takes, read from --drag-lift-scale. */
function liftScale(node: HTMLElement): number {
  const token = parseFloat(getComputedStyle(node).getPropertyValue("--drag-lift-scale")) || 1;
  return 1 + (token - 1) * Math.min(1, LIFT_REF_PX / Math.max(node.offsetWidth, 1));
}

/** settleMs is how long the landing transition on a node lasts. */
function settleMs(node: HTMLElement): number {
  return Math.max(0, ...getComputedStyle(node).transitionDuration.split(",").map(parseFloat)) * 1000;
}

/** translateOf reads the translate a node is painted with, a slide in flight included. */
function translateOf(node: HTMLElement): Point {
  const t = getComputedStyle(node).translate;
  const [x = 0, y = 0] = t === "none" ? [] : t.split(" ").map(parseFloat);
  return { x, y };
}

function clamp(v: number, lo: number, hi: number): number {
  return Math.min(Math.max(v, lo), hi);
}

function sameOrder(a: readonly string[], b: readonly string[]): boolean {
  return a.length === b.length && a.every((id, at) => id === b[at]);
}

/** An item's box in the container's layout, which no transform moves. */
interface Box extends Point {
  w: number;
  h: number;
}

function boxOf(node: HTMLElement): Box {
  return { x: node.offsetLeft, y: node.offsetTop, w: node.offsetWidth, h: node.offsetHeight };
}

/** The item a press lifted, measured when it armed. */
interface Lifted {
  id: string;
  /** Where the pointer went down, in client pixels. */
  down: Point;
  from: Box;
  /**
   * The range its corner may take: the area all items cover, inset by what the
   * lift's scale adds, so it cannot overflow a container that scrolls.
   */
  min: Point;
  max: Point;
  /** Whether it follows the pointer on both axes, which only a wrapped row needs. */
  free: boolean;
  rtl: boolean;
  scale: number;
}

/** A press that has not armed yet. */
interface Press {
  id: string;
  down: Point;
  pointerId: number;
  /** Whether it arms by holding still; otherwise by moving past the threshold. */
  hold: boolean;
}

export interface ReorderOptions {
  /** The resting order. */
  ids: readonly string[];
  /** The items' offsetParent, the layout a drag measures in. */
  container: RefObject<HTMLElement | null>;
  /** The attribute carrying an item's id on the element that lifts. */
  attr: string;
  /** `y` for a column; `x` for a row, which follows on both axes once it wraps. */
  axis: "x" | "y";
  /**
   * How a mouse or pen arms the drag: `hold` where the item is also a click
   * target, `move` on a grip that does nothing else. A finger always holds.
   */
  arm: "hold" | "move";
  enabled: boolean;
  /** Called with the full new order after a drop that changed it. */
  onReorder?: (ids: string[]) => void;
}

export interface Reorder {
  /** The order to draw: the live one while an item is carried or landing. */
  order: string[];
  /** The item in the hand, or null. */
  held: string | null;
  /** The pointerdown handler, on the item or on its grip. */
  press: (e: ReactPointerEvent<HTMLElement>, id: string) => void;
  /** The drag class for an item: LIFT, SETTLE, SHIFT or none. */
  look: (id: string) => string;
  /**
   * For the item's click handler: whether this click is the release of a
   * drag and has to be swallowed. Only a pointer's click counts (a key or a
   * screen reader activates with a detail of 0), and no timer clears the
   * flag, since one would race a touch tap's later click.
   */
  endsDrag: (e: { detail: number }) => boolean;
}

/** useReorder is the drag gesture for a list of ids; see the file comment. */
export function useReorder({ ids, container, attr, axis, arm, enabled, onReorder }: ReorderOptions): Reorder {
  // liveOrder is the order laid out while an item is held or landing, and null
  // otherwise. Positions go straight to each item's `translate` rather than
  // through state, so the held item keeps up with the pointer.
  const [held, setHeld] = useState<string | null>(null);
  const [settling, setSettling] = useState<string | null>(null);
  const [liveOrder, setLiveOrder] = useState<string[] | null>(null);
  const holdTimer = useRef<number | null>(null);
  const pressed = useRef<Press | null>(null);
  const pointer = useRef<Point>({ x: 0, y: 0 });
  const lifted = useRef<Lifted | null>(null);
  // Where each item was painted before liveOrder moved it, for the slide.
  const slideFrom = useRef<Map<string, Point> | null>(null);
  const swallow = useRef(false);

  // Ref mirrors for the document listeners, which bind once and must read
  // current values.
  const liveRef = useRef<string[] | null>(null);
  liveRef.current = liveOrder;
  const idsRef = useRef<readonly string[]>(ids);
  idsRef.current = ids;
  const onReorderRef = useRef(onReorder);
  onReorderRef.current = onReorder;

  function nodes(): HTMLElement[] {
    return Array.from(container.current?.querySelectorAll<HTMLElement>(`[${attr}]`) ?? []);
  }

  function nodeOf(id: string): HTMLElement | undefined {
    return nodes().find((node) => node.getAttribute(attr) === id);
  }

  function cancelPress() {
    if (holdTimer.current !== null) {
      window.clearTimeout(holdTimer.current);
      holdTimer.current = null;
    }
    pressed.current = null;
  }

  function press(e: ReactPointerEvent<HTMLElement>, id: string) {
    // A second finger during a drag does not start another.
    if (!enabled || e.button !== 0 || lifted.current) return;
    cancelPress();
    const down = { x: e.clientX, y: e.clientY };
    const hold = arm === "hold" || e.pointerType === "touch";
    pressed.current = { id, down, pointerId: e.pointerId, hold };
    pointer.current = down;
    swallow.current = false;
    // No setPointerCapture: in Chromium, moving past a captured button's bounds
    // fired a spurious pointercancel, and the document listeners track the
    // gesture anyway.
    if (hold) {
      holdTimer.current = window.setTimeout(() => {
        holdTimer.current = null;
        lift(id, down);
      }, HOLD_MS);
    }
  }

  function lift(id: string, down: Point) {
    const node = nodeOf(id);
    const el = container.current;
    if (!node || !el) return;
    const boxes = nodes().map(boxOf);
    const from = boxOf(node);
    const scale = liftScale(node);
    const grow = (scale - 1) / 2;
    const left = Math.min(...boxes.map((b) => b.x));
    const top = Math.min(...boxes.map((b) => b.y));
    const right = Math.max(...boxes.map((b) => b.x + b.w));
    const bottom = Math.max(...boxes.map((b) => b.y + b.h));
    lifted.current = {
      id,
      down,
      from,
      min: { x: left + from.w * grow, y: top + from.h * grow },
      max: { x: right - from.w * (1 + grow), y: bottom - from.h * (1 + grow) },
      free: axis === "x" && boxes.some((b) => b.y >= from.y + from.h || b.y + b.h <= from.y),
      rtl: getComputedStyle(el).direction === "rtl",
      scale,
    };
    setSettling(null);
    setHeld(id);
    setLiveOrder([...idsRef.current]);
  }

  /**
   * Where the held item's corner goes for the current pointer, in the
   * container's layout. Only the drawn item is kept inside the container:
   * aimed from there, the lift's inset would leave the first and last slots
   * out of reach.
   */
  function heldAt(h: Lifted, drawn: boolean): Point {
    const x = h.from.x + pointer.current.x - h.down.x;
    const y = h.from.y + pointer.current.y - h.down.y;
    const keep = (v: number, lo: number, hi: number) => (drawn ? clamp(v, lo, hi) : v);
    return {
      x: axis === "y" ? h.from.x : keep(x, h.min.x, h.max.x),
      y: axis === "y" || h.free ? keep(y, h.min.y, h.max.y) : h.from.y,
    };
  }

  function placeHeld(h: Lifted) {
    const node = nodeOf(h.id);
    if (!node) return;
    // Before any layout is read, so the scale changes in the same style pass
    // as the lift class that transitions it.
    node.style.scale = String(h.scale);
    const at = heldAt(h, true);
    node.style.translate = `${at.x - node.offsetLeft}px ${at.y - node.offsetTop}px`;
  }

  /**
   * aim returns the order the held item's position asks for. An item counts
   * as before it once the held item's centre passes the item's centre; the
   * passed item then moves away by the held item's own size, so two items of
   * different sizes cannot trade places back and forth. Measured boxes, never
   * one assumed size, since the items need not match.
   */
  function aim(h: Lifted): string[] {
    const at = heldAt(h, false);
    const cx = at.x + h.from.w / 2;
    const cy = at.y + h.from.h / 2;
    const rest: string[] = [];
    let ahead = 0;
    for (const node of nodes()) {
      const id = node.getAttribute(attr);
      if (!id || id === h.id) continue;
      const b = boxOf(node);
      const mid = b.x + b.w / 2;
      // A wrapped row reads by line first, then along the line.
      const before =
        axis === "y" ? b.y + b.h / 2 < cy : cy > b.y + b.h || (cy >= b.y && (h.rtl ? mid > cx : mid < cx));
      if (before) ahead++;
      rest.push(id);
    }
    rest.splice(ahead, 0, h.id);
    return rest;
  }

  /** Where each item is painted, a slide in flight included. */
  function painted(): Map<string, Point> {
    const out = new Map<string, Point>();
    for (const node of nodes()) {
      const id = node.getAttribute(attr);
      const t = translateOf(node);
      if (id) out.set(id, { x: node.offsetLeft + t.x, y: node.offsetTop + t.y });
    }
    return out;
  }

  function release(h: Lifted) {
    lifted.current = null;
    // A click that follows the release would activate the held item.
    swallow.current = true;
    const node = nodeOf(h.id);
    if (node) node.style.scale = "";
    setHeld(null);
    setSettling(h.id);
  }

  // After every render, since a new liveOrder moves the items in the DOM: the
  // held item is placed under the pointer again, and every item the order
  // moved starts from where it was painted and slides to its new slot.
  useLayoutEffect(() => {
    const h = lifted.current;
    // First, so the forced layout below commits this offset with the others.
    if (h) placeHeld(h);
    const from = slideFrom.current;
    if (!from) return;
    slideFrom.current = null;
    const moved: HTMLElement[] = [];
    for (const node of nodes()) {
      const id = node.getAttribute(attr);
      const was = id ? from.get(id) : undefined;
      if (!was || id === h?.id) continue;
      const dx = was.x - node.offsetLeft;
      const dy = was.y - node.offsetTop;
      if (dx === 0 && dy === 0) continue;
      node.style.transition = "none";
      node.style.translate = `${dx}px ${dy}px`;
      moved.push(node);
    }
    if (moved.length === 0) return;
    // Reading layout commits the jump back, so the transition starts from there.
    void container.current?.offsetWidth;
    for (const node of moved) {
      node.style.transition = "";
      node.style.translate = "0px 0px";
    }
  });

  // The dropped item slides into its slot, and once the landing has played the
  // offsets and liveOrder go, leaving the order to `ids`.
  useLayoutEffect(() => {
    if (!settling) return;
    const node = nodeOf(settling);
    let ms = 0;
    if (node) {
      node.style.translate = "0px 0px";
      ms = settleMs(node);
    }
    const timer = window.setTimeout(() => {
      for (const n of nodes()) n.style.translate = "";
      setSettling(null);
      setLiveOrder(null);
    }, ms);
    return () => window.clearTimeout(timer);
    // nodeOf and nodes read the container's DOM, not render state.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [settling]);

  // On the document, so a pointer that leaves the pressed item is still tracked.
  useEffect(() => {
    if (!enabled) return;
    const el = container.current;

    function onMove(e: PointerEvent) {
      const p = pressed.current;
      if (!p || e.pointerId !== p.pointerId) return;
      pointer.current = { x: e.clientX, y: e.clientY };
      const h = lifted.current;
      if (!h) {
        const dx = e.clientX - p.down.x;
        const dy = e.clientY - p.down.y;
        // Moving before a hold arms makes it a click or a scroll.
        if (p.hold) {
          if (Math.abs(dx) > HOLD_SLOP_PX || Math.abs(dy) > HOLD_SLOP_PX) cancelPress();
        } else if (pastThreshold(dx, dy)) {
          lift(p.id, p.down);
        }
        return;
      }
      placeHeld(h);
      const next = aim(h);
      const shown = liveRef.current;
      if (shown && !sameOrder(next, shown)) {
        slideFrom.current = painted();
        setLiveOrder(next);
      }
    }

    function onUp(e: PointerEvent) {
      if (e.pointerId !== pressed.current?.pointerId) return;
      cancelPress();
      const h = lifted.current;
      if (!h) return;
      // A drop where it started is not a change and writes nothing.
      const order = liveRef.current;
      if (order && !sameOrder(order, idsRef.current)) onReorderRef.current?.(order);
      release(h);
    }

    // Escape, or the system taking the touch: everything slides back and
    // nothing is written.
    function putBack(h: Lifted) {
      cancelPress();
      const order = liveRef.current;
      if (order && !sameOrder(order, idsRef.current)) {
        // The others go home while the item is still lifted, so it lands from
        // under the pointer rather than jumping to its old slot first.
        slideFrom.current = painted();
        flushSync(() => setLiveOrder([...idsRef.current]));
      }
      release(h);
    }

    function onCancel(e: PointerEvent) {
      if (e.pointerId !== pressed.current?.pointerId) return;
      const h = lifted.current;
      if (h) putBack(h);
      else cancelPress();
    }

    function onEscape(e: KeyboardEvent) {
      const h = lifted.current;
      if (e.key === "Escape" && h) putBack(h);
    }

    // Once armed the gesture is the drag's: a long press does not open the
    // context menu, and a moving finger drags instead of scrolling. Bound
    // natively, since React's touchmove listener is passive and cannot cancel.
    function onMenu(e: Event) {
      if (lifted.current) e.preventDefault();
    }
    function onTouchMove(e: TouchEvent) {
      if (lifted.current) e.preventDefault();
    }

    document.addEventListener("pointermove", onMove);
    document.addEventListener("pointerup", onUp);
    document.addEventListener("pointercancel", onCancel);
    document.addEventListener("keydown", onEscape);
    el?.addEventListener("contextmenu", onMenu);
    el?.addEventListener("touchmove", onTouchMove, { passive: false });
    return () => {
      cancelPress();
      document.removeEventListener("pointermove", onMove);
      document.removeEventListener("pointerup", onUp);
      document.removeEventListener("pointercancel", onCancel);
      document.removeEventListener("keydown", onEscape);
      el?.removeEventListener("contextmenu", onMenu);
      el?.removeEventListener("touchmove", onTouchMove);
    };
    // The listeners read refs, so only these values need a re-bind.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled, axis, attr, arm]);

  // Items that arrive mid-drag are drawn after the live order rather than
  // left out until it ends.
  const order = liveOrder
    ? [...liveOrder.filter((id) => ids.includes(id)), ...ids.filter((id) => !liveOrder.includes(id))]
    : [...ids];

  function look(id: string): string {
    if (id === held) return LIFT;
    if (id === settling) return SETTLE;
    return held !== null || settling !== null ? SHIFT : "";
  }

  function endsDrag(e: { detail: number }): boolean {
    const ends = swallow.current && e.detail > 0;
    swallow.current = false;
    return ends;
  }

  return { order, held, press, look, endsDrag };
}
