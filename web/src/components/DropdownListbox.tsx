import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
  type KeyboardEvent as ReactKeyboardEvent,
  type ReactNode,
  type RefObject,
} from "react";
import { createPortal } from "react-dom";
import { computeBubblePosition } from "../lib/bubblePosition";
import { usePortalHue } from "../lib/portalHue";
import { enableWheelStep } from "../lib/selectScroll";

// DropdownListbox is the shared role="listbox" panel for pickers that open a
// list from a button. It is portalled to document.body because a card with
// overflow: hidden (ContainerRow clips its progress bar that way) would cut it
// off whatever its z-index. computeBubblePosition returns the panel's centre,
// so translateX(-50%) centres it on the trigger with no RTL branch.

/** Gap between trigger and panel, and the panel's minimum distance from the
 *  viewport edge. */
const DROPDOWN_GAP = 4;

export interface DropdownListboxProps {
  /** Whether the panel is shown. Each picker renders its own trigger. */
  open: boolean;
  /** Called when the panel dismisses itself (outside click, Escape, scroll,
   *  resize). Choosing an option does not call it: a single-select list closes
   *  itself at the call site, a multi-select one stays open. */
  onClose: () => void;
  /** The trigger element. It anchors the panel and is exempt from the
   *  outside-click dismissal, so clicking an open trigger does not close and
   *  immediately reopen it. */
  triggerRef: RefObject<HTMLElement | null>;
  /** Accessible name for the listbox, usually the trigger's label. */
  label: string;
  /** Sets `aria-multiselectable` for a checkbox list. */
  multiselectable?: boolean;
  /** What one wheel notch on the trigger does, as on a closed native
   *  `<select>`: 1 moves down the list, -1 up. Clamp with `stepIndex` rather
   *  than wrapping. Omit it for a multi-select, which has no current value.
   *  It is a real listener rather than `onWheel`, which React registers as
   *  passive, so `preventDefault` could not stop the page scrolling. */
  wheelStep?: (delta: 1 | -1) => void;
  /** The `role="option"` buttons. */
  children: ReactNode;
}

export function DropdownListbox({
  open,
  onClose,
  triggerRef,
  label,
  multiselectable,
  wheelStep,
  children,
}: DropdownListboxProps) {
  const panelRef = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState<{ left: number; top: number; width: number } | null>(null);
  const focusedOnOpen = useRef(false);
  const focusInsideRef = useRef(false);
  useEffect(() => {
    if (!open) focusedOnOpen.current = false;
  }, [open]);
  // Read through a ref so an inline onClose at the call site does not re-attach
  // every listener on each render; the multi-select re-renders on every toggle.
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;

  // Read through a ref for the same reason: the value change this handler
  // causes re-renders the call site.
  const wheelStepRef = useRef(wheelStep);
  wheelStepRef.current = wheelStep;
  useEffect(() => {
    const trigger = triggerRef.current;
    if (!trigger || !wheelStepRef.current) return;
    return enableWheelStep(trigger, (delta) => wheelStepRef.current?.(delta));
    // `open` is a dependency only so the listener follows the current trigger
    // element; the wheel also works while the list is closed.
  }, [triggerRef, open]);

  // Forget the position on close, so the next open measures again instead of
  // flashing at the previous coordinates.
  useLayoutEffect(() => {
    if (!open) setPos(null);
  }, [open]);

  // The portalled panel is outside the trigger's hued ancestor and would
  // otherwise fall back to the global accent.
  const hue = usePortalHue(open, triggerRef);

  // A layout effect, so the position lands before paint rather than as a jump
  // from the off-screen parking spot.
  useLayoutEffect(() => {
    if (!open) return;
    const trigger = triggerRef.current;
    const panel = panelRef.current;
    if (!trigger || !panel) return;
    const rect = trigger.getBoundingClientRect();
    // Size the node before reading its height, because labels wrap against the
    // width; React writes the same values on its next render. The trigger's
    // width is only a minimum, since each option spends 24px on padding, and
    // max-content keeps a fixed box with only `left` set from shrinking to the
    // space on its right.
    panel.style.minWidth = `${rect.width}px`;
    panel.style.width = "max-content";
    const viewport = {
      width: document.documentElement.clientWidth || window.innerWidth,
      height: document.documentElement.clientHeight || window.innerHeight,
    };
    const measured = panel.offsetWidth;
    const { left, top } = computeBubblePosition(
      rect,
      { width: measured, height: panel.offsetHeight },
      viewport,
      DROPDOWN_GAP
    );
    // `children` is a dependency so a list that changes length is measured
    // again. Its identity changes on every render, so keeping the previous
    // object when nothing moved stops this from looping.
    setPos((prev) =>
      prev && prev.left === left && prev.top === top && prev.width === rect.width
        ? prev
        : { left, top, width: rect.width }
    );
  }, [open, triggerRef, children]);

  // Dismiss on outside mousedown, Escape, scroll and resize, since a fixed
  // panel that has lost its trigger looks broken. The panel is exempt from the
  // outside check, or a mousedown would unmount an option before its click
  // arrived. Scroll is captured on window because the page scroller is `main`,
  // not the document; a scroll inside the panel is its own list moving and
  // leaves it open.
  useEffect(() => {
    if (!open) return;
    function onPointerDown(e: MouseEvent) {
      const target = e.target as Node;
      if (panelRef.current?.contains(target)) return;
      if (triggerRef.current?.contains(target)) return;
      onCloseRef.current();
    }
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") onCloseRef.current();
    }
    function onScroll(e: Event) {
      const target = e.target;
      if (target instanceof Node && panelRef.current?.contains(target)) return;
      onCloseRef.current();
    }
    function onResize() {
      onCloseRef.current();
    }
    document.addEventListener("mousedown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    window.addEventListener("scroll", onScroll, true);
    window.addEventListener("resize", onResize);
    return () => {
      document.removeEventListener("mousedown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
      window.removeEventListener("scroll", onScroll, true);
      window.removeEventListener("resize", onResize);
    };
  }, [open, triggerRef]);

  // The options are the caller's children, so they are found by role.
  function options(): HTMLElement[] {
    const panel = panelRef.current;
    if (!panel) return [];
    return Array.from(panel.querySelectorAll<HTMLElement>('[role="option"]'));
  }

  // Move focus to the selected option on open, so a keyboard user can reach the
  // list. It waits for `pos`: focusing an option still parked at -9999px makes
  // the browser scroll to it, which trips the scroll listener and closes the
  // panel.
  useLayoutEffect(() => {
    if (!open || !pos || focusedOnOpen.current) return;
    const opts = options();
    if (opts.length === 0) return;
    const active = opts.find((o) => o.getAttribute("aria-selected") === "true") ?? opts[0];
    focusedOnOpen.current = true;
    active.scrollIntoView?.({ block: "nearest" });
    active.focus();
  }, [open, pos]);

  // Return focus to the trigger on close, but only if it was inside the panel;
  // a user who clicked elsewhere has chosen where focus goes. Focus is tracked
  // as it moves because by cleanup time the portal's nodes are gone and
  // activeElement is <body>.
  useLayoutEffect(() => {
    const trigger = triggerRef.current;
    return () => {
      if (focusInsideRef.current) trigger?.focus();
      focusInsideRef.current = false;
    };
  }, [open, triggerRef]);

  // Arrow keys, Home and End are handled here so every listbox behaves the
  // same. Enter and Space are left to the option buttons.
  function onPanelKeyDown(e: ReactKeyboardEvent<HTMLDivElement>) {
    if (e.key !== "ArrowDown" && e.key !== "ArrowUp" && e.key !== "Home" && e.key !== "End") return;
    const opts = options();
    if (opts.length === 0) return;
    e.preventDefault();
    const at = opts.indexOf(document.activeElement as HTMLElement);
    let next: number;
    if (e.key === "Home") next = 0;
    else if (e.key === "End") next = opts.length - 1;
    else {
      const dir = e.key === "ArrowDown" ? 1 : -1;
      // Arrows wrap, like TimePicker's columns.
      next = at < 0 ? (dir === 1 ? 0 : opts.length - 1) : (at + dir + opts.length) % opts.length;
    }
    opts[next].scrollIntoView?.({ block: "nearest" });
    opts[next].focus();
  }

  if (!open) return null;

  return createPortal(
    <div
      ref={panelRef}
      role="listbox"
      onKeyDown={onPanelKeyDown}
      onFocus={() => {
        focusInsideRef.current = true;
      }}
      onBlur={(e) => {
        if (!e.currentTarget.contains(e.relatedTarget as Node | null)) focusInsideRef.current = false;
      }}
      aria-multiselectable={multiselectable ? "true" : undefined}
      aria-label={label}
      className={`fixed z-50 max-h-60 overflow-y-auto rounded-card bg-carbon-surface shadow-xl glim-fade${
        hue.className ? ` ${hue.className}` : ""
      }`}
      style={{
        // Parked off-screen until measured, since clamping needs its real height.
        left: pos?.left ?? -9999,
        top: pos?.top ?? -9999,
        minWidth: pos?.width,
        width: "max-content",
        maxWidth: "min(92vw, 28rem)",
        transform: "translateX(-50%)",
        scrollbarColor: "var(--carbon-border) transparent",
        // Last, so the trigger's hue reaches the options.
        ...(hue.style ?? {}),
      } as CSSProperties}
    >
      {children}
    </div>,
    document.body
  );
}
