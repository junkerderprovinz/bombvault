import { useEffect, useId, useRef, useState, type ReactNode, type RefObject } from "react";
import { createPortal } from "react-dom";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { InfoBubble } from "../InfoBubble";
import { IconClose } from "../navGlyphs";
import { useT } from "../../lib/i18n";
import { useBackCloses } from "../../lib/useBackCloses";

// BottomSheet; the mobile bottom-sheet primitive.
//
// lib/useConfirm.tsx's stateful mechanism re-expressed as a bottom-anchored
// sheet, not a third modal implementation: the portal, document-level Escape,
// FOCUSABLE_SELECTOR Tab trap and focus restore below are lifted from
// useConfirm.tsx with only the deltas a viewport change forces; a trap
// re-derived from scratch is the documented house anti-pattern. Deltas:
// max-h-[85dvh] not 85vh (dvh tracks mobile browser chrome; 100vh is banned
// from the mobile shell); var(--safe-area-bottom) padding for the
// home-indicator gap; a motion-safe slide-up entrance, not glim-modal-card's
// 10px pop; no drag-to-dismiss and no grabber, because the sheet closes by
// scrim click, Escape, the header close button and Back, and
// background content is unreachable through the Tab trap.
//
// React 19's boolean `inert` on the scrim was probed and rejected: inert
// elements are skipped in hit-testing, so a scrim click lands behind the
// sheet. The scrim is `aria-hidden` instead, and panel and scrim are
// siblings (a DOM child of an aria-hidden element is hidden from
// assistive technology).

export interface BottomSheetProps {
  /** Whether the sheet is open. The caller owns the state; every close path calls onClose. */
  open: boolean;
  /** Called by every close path: scrim click, Escape, header close button,
   *  Back. Returning false keeps the sheet open, and Back with it. */
  onClose: () => void | boolean;
  /** The header's close button, on by default. A sheet whose footer already
   *  answers turns it off: two ways to cancel read as a choice between two
   *  answers, the same reason ConfirmDialog carries no corner X. Initial
   *  focus then goes to the first control in the panel, which the footer
   *  stack puts ahead of the commit. */
  headerClose?: boolean;
  /** The sheet's heading, already translated by the caller. */
  title: ReactNode;
  /** What the sheet is for, as the (i) in its heading. */
  hint?: string;
  /** The sheet body. */
  children: ReactNode;
  /** Full-height variant (the run detail sheet): h-dvh, edge to edge,
   *  instead of the floating max-h-[85dvh] card; a boolean so the primitive
   *  owns its viewport contract. */
  fullHeight?: boolean;
  /** Optional action row pinned after the scroll body; flex-none, chrome
   *  language (sidebar surface over the body's, separation by tint alone,
   *  no line), safe-area padded on the full-height sheet, never scrolled
   *  away; content padding is
   *  the consumer's. */
  footer?: ReactNode;
  /** Optional id the panel's aria-describedby points at; the consumer owns
   *  element and id, the primitive only wires the reference. */
  describedBy?: string;
}

// The panel's focusables, DOM/tab order; verbatim useConfirm.tsx, generic so the body can grow controls.
const FOCUSABLE_SELECTOR =
  'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

function focusableElements(root: HTMLElement): HTMLElement[] {
  return Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR));
}

// The open panels, newest last. Two sheets can be up at once (the More sheet
// with the run sheet over it), and each listens on the document, so without a
// top-of-stack check both would pull focus into their own panel on every Tab
// and the keyboard would stop moving.
const openPanels: RefObject<HTMLDivElement | null>[] = [];

/** underRestingPanel reports whether a point lies where the panel comes to
 *  rest once its slide has ended. */
function underRestingPanel(panel: HTMLElement | null, x: number, y: number): boolean {
  if (!panel) return false;
  const box = panel.getBoundingClientRect();
  const shift = translateY(getComputedStyle(panel).transform);
  return x >= box.left && x <= box.right && y >= box.top - shift && y <= box.bottom - shift;
}

/** translateY reads the vertical offset out of a computed transform. */
function translateY(transform: string): number {
  const matrix = /^matrix(3d)?\((.+)\)$/.exec(transform);
  if (!matrix) return 0;
  const values = matrix[2].split(",").map(Number);
  return (matrix[1] ? values[13] : values[5]) ?? 0;
}

/** slideMs is how long the panel's slide runs, 0 when motion is off. */
function slideMs(panel: HTMLElement | null): number {
  const seconds = panel ? parseFloat(getComputedStyle(panel).transitionDuration) : 0;
  return Number.isFinite(seconds) ? seconds * 1000 : 0;
}

export function BottomSheet({ open, onClose, headerClose = true, title, hint, children, fullHeight, footer, describedBy }: BottomSheetProps) {
  const { t } = useT();
  const titleId = useId();
  // panelRef roots the Tab trap; closeRef gets initial focus; triggerRef is restored on close (useConfirm.tsx's trio).
  const panelRef = useRef<HTMLDivElement>(null);
  const closeRef = useRef<HTMLButtonElement>(null);
  const triggerRef = useRef<HTMLElement | null>(null);
  // Flipped after a double rAF so the parked frame paints first and the
  // transform transition runs instead of coalescing both states into one paint.
  const [entered, setEntered] = useState(false);
  // Set once the slide has ended. Until then the panel takes no tap, and a
  // tap on the scrim where the sheet comes to rest does not close it: it was
  // aimed at the rising sheet and landed where the sheet no longer was.
  const [resting, setResting] = useState(false);
  useBackCloses(open, onClose);

  // Focus capture + initial focus-in + restore, in one effect. A controlled
  // component has no confirm()-style call to capture the trigger from, so the
  // open transition is the entry point. No `autoFocus` on any child: React
  // applies it during the commit, before this effect runs, so the captured
  // "trigger" would be the sheet's own first control. The focus-in is
  // ConfirmDialog parity: focus starts inside the aria-modal surface.
  useEffect(() => {
    if (!open) return;
    const active = document.activeElement;
    triggerRef.current = active instanceof HTMLElement && active !== document.body ? active : null;
    const panel = panelRef.current;
    (closeRef.current ?? (panel ? focusableElements(panel)[0] : undefined))?.focus();
    const trigger = triggerRef.current;
    return () => {
      triggerRef.current = null;
      // Focus is only ours to give back while it still sits in this sheet or
      // has fallen to the body. A width flip mid-confirmation swaps the sheet
      // for the desktop card, which focuses its own Cancel during the commit;
      // restoring the trigger there would pull focus out of the modal that
      // just took over.
      const active = document.activeElement;
      const taken =
        active instanceof HTMLElement &&
        active !== document.body &&
        panel !== null &&
        !panel.contains(active);
      if (taken) return;
      if (trigger && document.contains(trigger)) trigger.focus();
    };
  }, [open]);

  // Escape (document-level, so it works wherever focus is; never an inline
  // onKeyDown, documented broken at useConfirm.tsx) plus the Tab/Shift+Tab
  // trap, lifted from useConfirm.tsx with card->panel and settle(false)->onClose().
  useEffect(() => {
    if (!open) return;
    openPanels.push(panelRef);
    function onKeyDown(e: KeyboardEvent) {
      if (openPanels[openPanels.length - 1] !== panelRef) return;
      // A dialog opened from inside the sheet stands after it in the document
      // and has to get the keys: trapped here, Tab could never reach it.
      const dialogs = document.querySelectorAll('[role="dialog"], [role="alertdialog"]');
      if (dialogs[dialogs.length - 1] !== panelRef.current) return;
      if (e.key === "Escape") {
        e.preventDefault();
        onClose();
        return;
      }
      if (e.key !== "Tab") return;
      const panel = panelRef.current;
      if (!panel) return;
      const focusables = focusableElements(panel);
      if (focusables.length === 0) return;
      const first = focusables[0];
      const last = focusables[focusables.length - 1];
      const active = document.activeElement;
      const insidePanel = active instanceof Node && panel.contains(active);
      if (e.shiftKey) {
        if (!insidePanel || active === first) {
          e.preventDefault();
          last.focus();
        }
      } else {
        if (!insidePanel || active === last) {
          e.preventDefault();
          first.focus();
        }
      }
    }
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("keydown", onKeyDown);
      const at = openPanels.lastIndexOf(panelRef);
      if (at !== -1) openPanels.splice(at, 1);
    };
  }, [open, onClose]);

  // Slide-up entrance under motion-safe: only (header note has the why).
  // --motion-page-dur lets the motion levels govern the slide; at
  // data-motion="off" the global override zeroes it: the sheet appears.
  useEffect(() => {
    if (!open) {
      setEntered(false);
      setResting(false);
      return;
    }
    const slide = slideMs(panelRef.current);
    if (slide === 0) setResting(true);
    let raf2 = 0;
    let rest = 0;
    const raf1 = requestAnimationFrame(() => {
      raf2 = requestAnimationFrame(() => {
        setEntered(true);
        if (slide > 0) rest = window.setTimeout(() => setResting(true), slide);
      });
    });
    return () => {
      cancelAnimationFrame(raf1);
      cancelAnimationFrame(raf2);
      window.clearTimeout(rest);
    };
  }, [open]);

  if (!open) return null;
  return createPortal(
    <>
      {/* The scrim: decoration, hidden from assistive technology;
          .glim-modal-backdrop paints var(--glim-scrim) (index.css), the one
          token every modal shares; an inline bg-black/60 beside it duplicates
          the value and the modalBackdrop guard rejects it. The click closes
          only on the scrim itself (target === currentTarget; ConfirmDialog's
          guard). Not `inert`: see the header note. */}
      <div
        aria-hidden="true"
        className="glim-modal-backdrop fixed inset-0 z-50"
        onClick={(e) => {
          if (e.target !== e.currentTarget) return;
          if (!resting && underRestingPanel(panelRef.current, e.clientX, e.clientY)) return;
          onClose();
        }}
      />
      {/* Bottom-anchored panel; a scrim sibling, not its DOM child (an
          aria-hidden element's child is hidden from assistive technology
          too). A capped sheet floats with the BottomNav card's insets and
          rounding, so its closed position has to clear that gap as well;
          the full-height sheet is a screen and stays edge to edge. */}
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        // A plain title names the sheet by itself, without the tip of the (i)
        // that sits in the heading.
        aria-label={typeof title === "string" ? title : undefined}
        aria-labelledby={typeof title === "string" ? undefined : titleId}
        aria-describedby={describedBy}
        className={`fixed z-50 flex ${
          fullHeight
            ? "inset-x-0 bottom-0 h-dvh rounded-t-card"
            : "bottom-[calc(var(--safe-area-bottom)+0.5rem)] left-[max(1rem,var(--safe-area-left))] right-[max(1rem,var(--safe-area-right))] max-h-[85dvh] overflow-hidden rounded-card"
        } flex-col bg-carbon-surface shadow-2xl motion-safe:transition-transform motion-safe:duration-[var(--motion-page-dur)] ${
          entered ? "translate-y-0" : fullHeight ? "translate-y-full" : "translate-y-[calc(100%+var(--safe-area-bottom)+0.5rem)]"
        } ${resting ? "" : "pointer-events-none"}`}
      >
        {/* Side padding is inset-clamped: each physical side takes
            max(1rem, its own safe-area inset), so content clears landscape
            display cutouts. Physical pl/pr + physical inset vars, not logical
            padding-inline, so the mapping stays correct under direction:rtl
            (env() insets never flip with writing direction). */}
        <div className="flex items-center justify-between gap-4 py-3 pl-[max(1rem,var(--safe-area-left))] pr-[max(1rem,var(--safe-area-right))]">
          {/* Title in flow, not a self-positioning notch: Badge's default
              heading badge is `absolute top-0 -translate-y-1/2`, wrong on a
              fullHeight sheet (a device probe measured the badge half
              outside the viewport); `inFlow` (Badge.tsx) hands positioning
              to the caller and the notch default stays reserved for cards.
              The <h2> carries the aria-labelledby id; min-w-0 so a long
              title wraps; useId because sheets nest. */}
          <h2 id={titleId} className="flex min-w-0 items-center">
            <Badge tone="heading" size="heading" wrap inFlow className="min-w-0">
              {title}
              {hint && <InfoBubble tip={hint} onAccent />}
            </Badge>
          </h2>
          {/* Engine Button: shared tone table, tooltips and press motion
              (.glim-btn:active already scales by --motion-press-scale).
              Distinct accessible name (#178: duplicate names broke
              Playwright strict matching). The key stage (glim-btn-key) is
              the close box's height: .glim-btn owns its height outside any
              utility layer, so a h-* utility in the className loses the
              cascade, and the key stage is the height that wins
              (glim-btn-icon squares the box at it). */}
          {headerClose && (
            <Button
              ref={closeRef}
              label={t("common.close")}
              labelKey="common.close"
              glyph={<IconClose />}
              tone="neutral"
              variant="icon"
              onClick={onClose}
              className="glim-btn-key shrink-0 rounded-control"
            />
          )}
        </div>
        {/* Body (scrolls); its own scroll contains all interaction and
            background content is unreachable through the Tab trap, so no
            body-scroll-lock. On the full-height sheet the bottom padding
            clears the device safe area, but only when no footer follows:
            the footer is the last surface on screen and pads itself, so a
            body inset beside one paid the home-indicator gap twice. A
            floating sheet already sits above the safe area and only keeps
            its rows off the rounded edge. Ternary between complete literal
            classes only: the
            Tailwind JIT scans source text, so a token assembled from
            fragments would silently stop existing. */}
        <div
          className={`min-h-0 flex-1 overflow-y-auto overscroll-contain pl-[max(1rem,var(--safe-area-left))] pr-[max(1rem,var(--safe-area-right))] ${
            footer !== undefined ? "" : fullHeight ? "pb-[var(--safe-area-bottom)]" : "pb-3"
          }`}
        >
          {children}
        </div>
        {/* Footer (optional); pinned after the scroll body so actions never
            scroll away; chrome language (bottom bar / sticky action bar
            tokens), no line: the sidebar surface over the body's surface is
            the separation, the app's tint-only rule. On the full-height
            sheet it is the last surface on screen, so the home-indicator
            inset is here. */}
        {footer !== undefined && (
          <div
            data-dialog-actions
            className={`flex-none bg-carbon-sidebar pl-[max(1rem,var(--safe-area-left))] pr-[max(1rem,var(--safe-area-right))] ${
              fullHeight ? "pb-[var(--safe-area-bottom)]" : ""
            }`}
          >
            {footer}
          </div>
        )}
      </div>
    </>,
    document.body,
  );
}
