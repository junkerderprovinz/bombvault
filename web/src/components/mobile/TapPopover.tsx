import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
  type MouseEvent as ReactMouseEvent,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import { computeBubblePosition } from "../../lib/bubblePosition";

// ---------------------------------------------------------------------------
// TapPopover — the anchored tap-popover primitive.
//
// Hover has no touch equivalent, so every affordance that only opens on
// hover/focus is dead on a phone. This component is the below-breakpoint
// presentation those affordances mount into: a trigger button that reports
// the standard aria-expanded + aria-haspopup="dialog" pair (the
// FilterPopover.tsx / ColorPickerPopover.tsx precedent, lifted verbatim) and
// a portalled role="dialog" panel anchored to the trigger.
//
// Why hand-rolled, and what is lifted rather than re-invented (the house
// anti-pattern is a mechanism re-derived from scratch — see BottomSheet's
// header for the doctrine):
//   - Positioning is THE ONE positioning maths, lib/bubblePosition.ts's
//     computeBubblePosition (clamp-then-flip over BUBBLE_VIEWPORT_MARGIN 8),
//     in useTipBubble.tsx's measure-then-place useLayoutEffect shape: the
//     panel is measured only once it is in the DOM (its real height depends
//     on how the content wraps), and the corrected position lands before the
//     first paint. No second flip/clamp implementation may exist.
//     computeBubblePosition returns the CLAMPED CENTER X, so the panel pairs
//     `w-max` with `-translate-x-1/2` — the exact documented pairing the
//     .glim-bubble/.glim-time-popover engine classes use; dropping either
//     puts the panel's left EDGE at the trigger's center (half off-screen).
//   - Escape is a document-level keydown listener, never a React onKeyDown —
//     the pattern documented broken at useConfirm.tsx:25-33 (focus drifting
//     to <body> silently kills inline Escape).
//   - Focus is light (a popover is NOT a modal): on open, focus moves
//     into the panel itself (tabIndex -1); on ANY dismissal it restores to
//     the trigger. Capture-then-restore in one effect with the cleanup doing
//     the restore is BottomSheet.tsx's open-effect shape minus the trap.
//     There is deliberately NO Tab containment cycling: a trap would make
//     Escape/outside-tap semantics modal for no reason, and the panel's own
//     content stays in the page's normal tab order (asserted in the dom
//     tests — a Tab keydown is never preventDefault()ed).
//   - The dismissal layer is a TRANSPARENT full-viewport div UNDER the panel
//     (z-40 below the panel's z-50), not a scrim: a popover is not a modal,
//     so there is no dimming. It is a REAL event-consuming layer: outside
//     taps land on it and die there, never reaching the
//     content beneath. It is deliberately NOT `inert` for the reason
//     BottomSheet measured for its scrim: inert elements are skipped in
//     hit-testing, so the tap would pass straight through to whatever is
//     behind it — the exact leak this layer exists to prevent. The
//     target === currentTarget guard mirrors BottomSheet's scrim.
//   - Re-tap-to-toggle comes free from the layering: while open, the
//     backdrop covers the trigger too, so a second tap on the trigger's
//     spot hits the backdrop and closes. The trigger's own onClick is the
//     same toggle for the direct-dispatch case (jsdom, keyboard) — both
//     paths close, which is what the contract requires.
//   - Entrance is the existing glim-fade engine class — an opacity-only
//     keyframe already self-gated by prefers-reduced-motion in index.css
//     (and already the .glim-bubble/.glim-picker-popover entrance), so no
//     new motion code and no motion-variant juggling.
//
// Controlled/uncontrolled, like a native input: pass neither `open` nor
// `onOpenChange` and the state is internal (FilterPopover); pass both and
// the owner decides (ColorPickerPopover, whose drag/dismissal effects are
// gated on its own `open` state and must stay live on mobile).
//
// This component owns NO user-visible strings: the panel's accessible name
// arrives via `label` and the trigger's via `triggerLabel`, both already
// translated by the caller (the locale tables are single-owner — no new keys
// may be minted here).
// ---------------------------------------------------------------------------

export interface TapPopoverProps {
  /** The panel dialog's accessible name, already translated by the caller. */
  label: string;
  /** Panel content — the consumer's own surface, unmodified. */
  children: ReactNode;
  /** The trigger button's content (glyph, label text, indicator dot). Leave
   *  undefined for a trigger that is its own visual (the colour swatch). */
  trigger?: ReactNode;
  /** Classes for the trigger button. The component bakes in the 44px touch
   *  floor (min-h-11 min-w-11) and centering; the consumer's classes layer
   *  on top (glim-btn + surface tokens for FilterPopover, swatch sizing for
   *  ColorPickerPopover). */
  triggerClassName?: string;
  /** Inline styles for the trigger button (the colour swatch's backgroundColor). */
  triggerStyle?: CSSProperties;
  /** The trigger's own accessible name + native title, for a trigger whose
   *  content is not text (the colour swatch). Omitted when the trigger
   *  content carries the name itself (FilterPopover's labelled button). */
  triggerLabel?: string;
  /** Disables the trigger (the colour swatch's disabled contract). */
  disabled?: boolean;
  /** Controlled open state. Undefined = uncontrolled (internal state). */
  open?: boolean;
  /** Controlled change callback — every open/close path reports through it. */
  onOpenChange?: (open: boolean) => void;
  /** Extra classes for the panel: surface-internal layout (padding, column
   *  gaps, min/max width) is the consumer's; this component owns the chrome
   *  (fixed anchoring, elevation, surface colour, radius, entrance). */
  panelClassName?: string;
}

export function TapPopover({
  label,
  children,
  trigger = null,
  triggerClassName = "",
  triggerStyle,
  triggerLabel,
  disabled = false,
  open: openProp,
  onOpenChange,
  panelClassName = "",
}: TapPopoverProps) {
  const [openState, setOpenState] = useState(false);
  const isControlled = openProp !== undefined;
  const open = isControlled ? openProp : openState;

  const setOpen = useCallback(
    (next: boolean) => {
      if (!isControlled) setOpenState(next);
      onOpenChange?.(next);
    },
    [isControlled, onOpenChange],
  );

  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const panelRef = useRef<HTMLDivElement | null>(null);

  // Measure-then-place, useTipBubble.tsx's shape: the panel's real size only
  // exists once it is mounted, and the useLayoutEffect (not useEffect) puts
  // the corrected position down BEFORE the first paint — no top-left flash.
  // computeBubblePosition is THE positioning maths; nothing inline here.
  useLayoutEffect(() => {
    if (!open) return;
    const triggerEl = triggerRef.current;
    const panel = panelRef.current;
    if (!triggerEl || !panel) return;
    const rect = triggerEl.getBoundingClientRect();
    const viewport = {
      width: document.documentElement.clientWidth || window.innerWidth,
      height: document.documentElement.clientHeight || window.innerHeight,
    };
    const { left, top } = computeBubblePosition(
      rect,
      { width: panel.offsetWidth, height: panel.offsetHeight },
      viewport,
    );
    panel.style.left = `${left}px`;
    panel.style.top = `${top}px`;
  }, [open]);

  // Light focus, BottomSheet's open-effect shape minus the trap: move
  // focus into the panel on open; the effect's cleanup — which runs on every
  // open→closed transition, i.e. on EVERY dismissal path (backdrop, Escape,
  // re-tap) — restores focus to the trigger, guarding the same way for a
  // trigger that unmounted while the popover was open. No Tab containment
  // cycling anywhere: see the header note.
  useEffect(() => {
    if (!open) return;
    panelRef.current?.focus();
    const triggerEl = triggerRef.current;
    return () => {
      if (triggerEl && document.contains(triggerEl)) triggerEl.focus();
    };
  }, [open]);

  // Escape, document-level so it works no matter where focus currently is
  // (useConfirm.tsx:25-33 documents why an inline onKeyDown is the bug).
  useEffect(() => {
    if (!open) return;
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") {
        e.preventDefault();
        setOpen(false);
      }
    }
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [open, setOpen]);

  // The trigger stays MOUNTED on both sides of open (focus-restore needs the
  // element alive), and one toggle handler serves the closed→open and
  // direct-dispatch open→closed cases; the backdrop owns the real-world
  // open→closed case (it covers the trigger — header note).
  const triggerButton = (
    <button
      ref={triggerRef}
      type="button"
      disabled={disabled}
      aria-expanded={open}
      aria-haspopup="dialog"
      aria-label={triggerLabel}
      title={triggerLabel}
      style={triggerStyle}
      onClick={() => setOpen(!open)}
      // min-h-11/min-w-11 is the 44px touch floor — this component only ever
      // renders below the breakpoint, so the floor is unconditional here.
      className={`inline-flex min-h-11 min-w-11 items-center justify-center ${triggerClassName}`}
    >
      {trigger}
    </button>
  );

  if (!open) return triggerButton;

  const onBackdropClick = (e: ReactMouseEvent<HTMLDivElement>) => {
    if (e.target === e.currentTarget) setOpen(false);
  };

  return (
    <>
      {triggerButton}
      {createPortal(
        <>
          {/* The consuming dismissal layer: transparent (NO scrim tint — a
              popover is not a modal), full-viewport, UNDER the panel, and a
              real hit target so outside taps die here. Deliberately
              not `inert` — see the header note. */}
          <div aria-hidden="true" className="fixed inset-0 z-40" onClick={onBackdropClick} />
          {/* The panel: portalled to <body> (escapes ancestor CSS transforms,
              the InfoBubble/useConfirm portal fix), centred on
              computeBubblePosition's clamped-center X via the w-max +
              -translate-x-1/2 pairing, height-capped with its own scroll so a
              panel taller than the phone keeps its content reachable. */}
          <div
            ref={panelRef}
            role="dialog"
            aria-label={label}
            tabIndex={-1}
            className={`glim-fade fixed z-50 w-max max-h-[calc(100dvh-1rem)] -translate-x-1/2 overflow-y-auto rounded-card bg-carbon-surface shadow-xl ${panelClassName}`}
          >
            {children}
          </div>
        </>,
        document.body,
      )}
    </>
  );
}
