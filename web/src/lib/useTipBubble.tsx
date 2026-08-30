import {
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import { computeBubblePosition } from "./bubblePosition";

// ---------------------------------------------------------------------------
// useTipBubble — the app's ONE hover/focus tooltip, as a hook (#178).
//
// design-language.md, "The tooltip and info bubble": "A plain icon-only
// button's hover tooltip and a control's '(i)' explanatory bubble are the same
// mechanism wearing two different trigger elements, not two separate
// implementations that happen to look similar." That was the rule; the code
// said otherwise. The identical open-state / measure-then-clamp / close-on-
// scroll-or-Escape block existed FOUR times over — InfoBubble.tsx's "(i)"
// glyph, Selector.tsx's SelectorTab, IconTipButton.tsx, and it was about to be
// written a fifth time for Button.tsx (jdp's decision, glyph-mode round: a
// button that hides its text gets the real bubble, not the native balloon).
// Four copies is where "the same mechanism" stops being true: the viewport
// clamp had already been fixed twice, in two files, from the same bug report.
//
// So the mechanism moved here and the four call sites became four TRIGGERS,
// which is what they always were:
//
//   InfoBubble      a 15px "(i)" <span>
//   IconTipButton   a bare icon-only <button>
//   SelectorTab     one segment of a Selector strip
//   Button          any action button whose text the label engine has hidden
//
// What the trigger owns is its own element, its own classes, and WHEN it has
// something to say. What it no longer owns is a copy of the positioning maths.
//
// The native `title=` attribute is deliberately not an option here. It never
// appears on keyboard focus, it cannot be styled, and lint-rules/
// icon-badge-needs-tooltip.js rejects it on sight — it is the anti-pattern
// this hook is the alternative to.
// ---------------------------------------------------------------------------

export interface TipBubble {
  /** Ref callback for the trigger element — the rect the bubble is placed
   *  against. Callback rather than an object ref so a trigger that already
   *  keeps its own (SelectorTab registers every segment with its parent strip)
   *  can feed both from one attribute. */
  ref: (el: HTMLElement | null) => void;
  /** Spread on the trigger. Focus as well as hover, always: a tooltip only a
   *  mouse can reach is the exact defect the native balloon has. */
  handlers: {
    onMouseEnter: () => void;
    onMouseLeave: () => void;
    onFocus: () => void;
    onBlur: () => void;
  };
  /** `aria-describedby` while open, `undefined` otherwise, so the bubble is
   *  announced as the trigger's description rather than being invisible to
   *  assistive tech. */
  describedBy: string | undefined;
  /** The portalled bubble, or null when there is nothing to show. Render it
   *  as a sibling of the trigger; it lands on <body> either way, so no card's
   *  `overflow: hidden` can clip it. */
  bubble: ReactNode;
  /** Wraps a DISABLED trigger in a box that can still see the pointer.
   *
   *  A disabled <button> emits no mouse events and takes no focus, so the
   *  handlers above never fire — and that is exactly the moment the user most
   *  wants to know why the control is dead. Applied ONLY when disabled and
   *  only when there is a tip, so an enabled layout is untouched rather than
   *  gaining a box everywhere for a corner case. */
  wrap: (node: ReactNode) => ReactNode;
  /** Open/close by hand, for a trigger that needs its own extra reason to
   *  (a hover wrapper of the call site's own making). */
  show: () => void;
  hide: () => void;
}

/**
 * @param tip  What the bubble says. Falsy (no tip, empty string) means the
 *             trigger has nothing to explain: nothing opens, nothing renders,
 *             and `wrap` is a no-op — so a caller can pass a value that is
 *             only sometimes present without branching around this hook.
 * @param disabled  Whether the trigger is currently disabled; see `wrap`.
 */
export function useTipBubble(tip?: string, disabled = false): TipBubble {
  const [open, setOpen] = useState(false);
  const triggerRef = useRef<HTMLElement | null>(null);
  const bubbleRef = useRef<HTMLDivElement | null>(null);
  const tooltipId = useId();

  const shown = !!tip && open;

  function show() {
    if (!tip) return;
    setOpen(true);
  }
  function hide() {
    setOpen(false);
  }

  // Positions the bubble (clamped into the viewport, flipped above the trigger
  // when opening below would clip the bottom edge) AFTER it has mounted and
  // laid out — offsetWidth/offsetHeight only resolve once the element is in
  // the DOM, and the real wrapped height depends on the tip's own length, not
  // on the CSS max-width.
  //
  // useLayoutEffect, not useEffect: this has to run before the browser paints,
  // so the corrected position is what is actually painted rather than a
  // visible jump on the next frame.
  //
  // The bug this exists for (jdp, live): the "Wiederherstellungskit" bubble in
  // Settings overflowed the window and could not be read. Every copy of this
  // code centred the bubble on its trigger and always opened downward with no
  // viewport awareness at all, so a trigger near an edge pushed its bubble
  // half off-screen. `computeBubblePosition` (lib/bubblePosition.ts, ported
  // from GlimStone's reference/tooltip.ts) is the fix, and living here means
  // it cannot be fixed in one trigger and left broken in the next three.
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

  // A floating box anchored to a live rect must not drift out of position
  // under the trigger it is pointing at, and Escape has to dismiss it without
  // requiring the pointer to move.
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
      onMouseEnter: show,
      onMouseLeave: hide,
      onFocus: show,
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
    wrap: (node: ReactNode) =>
      disabled && tip ? (
        // `inline-flex`, matching what these controls already sit in, so the
        // wrapper is the same shape as the thing it wraps.
        <span className="inline-flex" onMouseEnter={show} onMouseLeave={hide}>
          {node}
        </span>
      ) : (
        node
      ),
    show,
    hide,
  };
}
