import {
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
  type PointerEvent as ReactPointerEvent,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import { computeBubblePosition } from "./bubblePosition";

// useTipBubble is the app's one hover and focus tooltip. InfoBubble,
// IconTipButton, SelectorTab and Button are its triggers: each owns its element
// and decides when it has something to say, the positioning lives here.
//
// The native title attribute is no substitute: it never shows on keyboard
// focus, cannot be styled, and lint-rules/icon-badge-needs-tooltip.js rejects it.

export interface TipBubble {
  /** Ref callback for the trigger element the bubble is placed against. A
   *  callback so a trigger with a ref of its own (SelectorTab registers each
   *  segment with its strip) can feed both from one attribute. */
  ref: (el: HTMLElement | null) => void;
  /** Spread on the trigger. Keyboard focus opens the bubble as well as hover;
   *  see `showOnFocus`. */
  handlers: {
    onMouseEnter: () => void;
    onMouseLeave: () => void;
    onFocus: () => void;
    onBlur: () => void;
  };
  /** The bubble's id while open, for aria-describedby. */
  describedBy: string | undefined;
  /** The bubble, portalled to <body> so no `overflow: hidden` can clip it, or
   *  null when closed. Render it next to the trigger. */
  bubble: ReactNode;
  /** Wraps a disabled trigger in a span that still gets pointer events. A
   *  disabled <button> fires none, yet that is when the user most wants to
   *  know why. Applied only when disabled and there is a tip, so enabled
   *  layouts are untouched. `style` goes on the span, which takes the
   *  trigger's place among its parent's flex items. */
  wrap: (node: ReactNode, style?: CSSProperties) => ReactNode;
  /** Open and close by hand, for a call site with its own hover wrapper. */
  show: () => void;
  hide: () => void;
}

/**
 * @param tip  What the bubble says. When empty, nothing opens or renders and
 *             `wrap` returns its node unchanged, so callers need not branch.
 * @param disabled  Whether the trigger is currently disabled; see `wrap`.
 * @param readOnTap  Whether a tap on a touch screen is a request to read the
 *             tip, as on an (i), so the bubble stays until the next tap
 *             elsewhere. Any other trigger is tapped to act, and its bubble
 *             only flashes.
 */
export function useTipBubble(tip?: string, disabled = false, readOnTap = false): TipBubble {
  const [open, setOpen] = useState(false);
  const [tapped, setTapped] = useState(false);
  const [readByTap, setReadByTap] = useState(false);
  const triggerRef = useRef<HTMLElement | null>(null);
  const wrapRef = useRef<HTMLSpanElement | null>(null);
  const bubbleRef = useRef<HTMLDivElement | null>(null);
  const tooltipId = useId();

  // Flipping `disabled` replaces the trigger element: `wrap` boxes a disabled
  // one, so React mounts a new <button> in its place. The old one takes its
  // focus and hover with it without firing blur or mouseleave, so an open
  // bubble would stay up for good (#243). Closing during render keeps it from
  // being painted even once; a pointer still over the control reopens it on
  // its next move, through the wrapper's mouseenter.
  const [seenDisabled, setSeenDisabled] = useState(disabled);
  if (seenDisabled !== disabled) {
    setSeenDisabled(disabled);
    setOpen(false);
  }

  const shown = !!tip && open;

  function show() {
    if (!tip) return;
    setOpen(true);
    setTapped(false);
  }
  function hide() {
    setOpen(false);
    setReadByTap(false);
  }
  // A tap reaches the trigger as a mouseenter with no mouseleave to follow,
  // so without this the bubble would stay up after the tap did its work.
  function showOnEnter() {
    show();
    if (touchWasLast && !readOnTap && !readByTap) setTapped(true);
  }
  useEffect(() => {
    if (!shown || !tapped) return;
    const timer = window.setTimeout(hide, TAP_TIP_MS);
    return () => window.clearTimeout(timer);
  }, [shown, tapped]);
  // In Chrome a tap on a disabled control does not reliably arrive as a
  // mouseenter, and the mouse events that follow it can land beside the
  // control. The tap can do nothing but ask why, so the wrapper opens the
  // bubble on the touch itself and keeps it until a tap elsewhere.
  function readOnTouch(e: ReactPointerEvent) {
    if (e.pointerType === "mouse" || !tip) return;
    setOpen(true);
    setTapped(false);
    setReadByTap(true);
  }
  function leave() {
    if (!readByTap) hide();
  }
  useEffect(() => {
    if (!shown || !readByTap) return;
    const away = (e: PointerEvent) => {
      if (!wrapRef.current?.contains(e.target as Node)) hide();
    };
    document.addEventListener("pointerdown", away, true);
    return () => document.removeEventListener("pointerdown", away, true);
  }, [shown, readByTap]);
  // Focus opens the bubble only when the keyboard was used last. Opening on
  // focus is for Tab users. A mouse user gets focus as a side effect, from a
  // click or from a dialog handing it back to its opener (useConfirm), and a
  // bubble opened then stays where the pointer no longer is.
  useEffect(trackInputModality, []);
  function showOnFocus() {
    if (!pointerWasLast) show();
  }

  // The bubble's size is known only once it is in the DOM, and it has to be
  // placed before paint so it does not visibly jump. computeBubblePosition
  // clamps it into the viewport and flips it above the trigger near the bottom.
  useLayoutEffect(() => {
    if (!shown) return;
    const trigger = triggerRef.current;
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
  }, [shown]);

  // Close on scroll so the bubble never drifts away from its trigger.
  useEffect(() => {
    if (!shown) return;
    const onScroll = () => hide();
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") hide();
    };
    window.addEventListener("scroll", onScroll, true);
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("scroll", onScroll, true);
      window.removeEventListener("keydown", onKey);
    };
  }, [shown]);

  return {
    ref: (el: HTMLElement | null) => {
      triggerRef.current = el;
    },
    handlers: {
      onMouseEnter: showOnEnter,
      onMouseLeave: leave,
      onFocus: showOnFocus,
      onBlur: hide,
    },
    describedBy: shown ? tooltipId : undefined,
    bubble: shown
      ? createPortal(
          <div
            ref={bubbleRef}
            role="tooltip"
            id={tooltipId}
            className="glim-bubble glim-fade"
          >
            {tip}
          </div>,
          document.body,
        )
      : null,
    wrap: (node: ReactNode, style?: CSSProperties) =>
      disabled && tip ? (
        <span
          ref={wrapRef}
          className="inline-flex"
          style={style}
          onMouseEnter={show}
          onMouseLeave={leave}
          onPointerDown={readOnTouch}
        >
          {node}
        </span>
      ) : (
        node
      ),
    show,
    hide,
  };
}

// Whether the user's last input was a pointer rather than a key. Kept for the
// whole page, not per trigger, because the focus in question usually lands on
// one element after the user acted on another: Cancel in a dialog, and then
// the focus handed back to the button that opened it.
//
// Not `:focus-visible`: jsdom evaluates it in a document listener that runs
// after React's focus handler, so inside the handler it reads false for
// keyboard focus too, and no test could tell the two cases apart.
let pointerWasLast = false;
let touchWasLast = false;
let tracking = false;

/** How long the bubble of a tapped trigger stays. */
const TAP_TIP_MS = 1500;

function trackInputModality() {
  if (tracking || typeof document === "undefined") return;
  tracking = true;
  const notePointer = (e: PointerEvent) => (touchWasLast = e.pointerType === "touch");
  document.addEventListener("pointerover", notePointer, true);
  document.addEventListener("pointerdown", (e) => {
    pointerWasLast = true;
    notePointer(e);
  }, true);
  document.addEventListener("keydown", () => (pointerWasLast = false), true);
}
