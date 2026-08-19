import type { ToastSeverity } from "../lib/toastEngine";

// ---------------------------------------------------------------------------
// Toast / ToastViewport — the GlimStone toast system's presentational half
// (form-engine Task 9, design-language.md "Toasts").
//
// Pure, hookless function components on purpose — same shape as Toggle.tsx/
// Badge.tsx/RevealInput.tsx/ConfirmDialog.tsx (props in, an element tree
// out) — so stacking, dismiss wiring and the severity->role mapping stay
// unit-testable by calling these directly (no jsdom/renderer; see
// Toggle.test.ts's header comment). The STATEFUL half — the toast queue,
// real setTimeout handles, and the createPortal(..., document.body) call
// that lets `position: fixed` escape any transformed ancestor (the same
// containing-block bug useConfirm.tsx/InfoBubble.tsx already fixed for their
// own portals) — lives in the companion hook, lib/toast.tsx.
//
// NON-BLOCKING BY DESIGN — the opposite requirement from Task 7's confirm
// dialog: a toast must NEVER intercept a click meant for the page behind or
// around it. The viewport wrapper is `pointer-events: none`; only each
// individual card re-enables `pointer-events: auto` (via .glim-toast in
// index.css), so the empty space in the corner — which is most of it — lets
// every click straight through to whatever is actually underneath.
// ---------------------------------------------------------------------------

export interface ToastCardProps {
  id: string;
  message: string;
  severity: ToastSeverity;
  /** Accessible name for the dismiss (X) button — generic across every
   *  toast, not per-message copy (mirrors ConfirmDialog's closeLabel). */
  dismissLabel: string;
  onDismiss: (id: string) => void;
  onMouseEnter: (id: string) => void;
  onMouseLeave: (id: string) => void;
  onFocus: (id: string) => void;
  onBlur: (id: string) => void;
}

// Neutral glyphs, colour-coded ONLY via the state hue itself (rule 4: "four
// state hues... never introduce a fifth"). Deliberately NOT a coloured left
// bar/rail on the card — rule 5 rules that out explicitly ("no vertical
// marks at all... a rail also breaks under the square corner setting").
const SEVERITY_ICON_CLASS: Record<ToastSeverity, string> = {
  success: "text-statusOkSolid",
  warn: "text-statusWarnSolid",
  fail: "text-statusFailSolid",
};

// role="alert" carries an IMPLICIT aria-live="assertive" (interrupts the
// screen reader immediately — matches design-language.md's "failures...
// always surface"); role="status" carries an implicit aria-live="polite"
// (announced once the user's screen reader is idle, never interrupting a
// routine completion). Both roles are live regions on their own the moment
// the browser sees them; no separately-managed, always-present
// `<div aria-live>` wrapper is needed, because each toast is a brand-new
// element inserted once (not existing content mutated in place) — exactly
// the case both roles are designed to announce on insertion.
function roleFor(severity: ToastSeverity): "alert" | "status" {
  return severity === "success" ? "status" : "alert";
}

function ToastGlyph({ severity }: { severity: ToastSeverity }) {
  const className = `mt-0.5 shrink-0 ${SEVERITY_ICON_CLASS[severity]}`;
  if (severity === "fail") {
    return (
      <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true" className={className}>
        <circle cx="8" cy="8" r="6.4" stroke="currentColor" strokeWidth="1.4" />
        <path d="M8 4.8v3.8M8 10.9h.01" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      </svg>
    );
  }
  if (severity === "warn") {
    return (
      <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true" className={className}>
        <path d="M8 2.2 14.5 13.6H1.5L8 2.2Z" stroke="currentColor" strokeWidth="1.4" strokeLinejoin="round" />
        <path d="M8 6.6v3M8 11.9h.01" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      </svg>
    );
  }
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true" className={className}>
      <circle cx="8" cy="8" r="6.4" stroke="currentColor" strokeWidth="1.4" />
      <path d="M5.3 8.2 7.2 10.1 10.7 5.9" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function ToastCard({
  id,
  message,
  severity,
  dismissLabel,
  onDismiss,
  onMouseEnter,
  onMouseLeave,
  onFocus,
  onBlur,
}: ToastCardProps) {
  return (
    <div
      role={roleFor(severity)}
      // onFocus/onBlur are React's delegated focusin/focusout — they fire
      // when the dismiss button INSIDE this div gains/loses focus too, so
      // Tab-ing onto it pauses the countdown exactly like a mouse hover
      // does (design-language.md: "Hover OR focus pauses"). No extra
      // plumbing needed on the button itself.
      onMouseEnter={() => onMouseEnter(id)}
      onMouseLeave={() => onMouseLeave(id)}
      onFocus={() => onFocus(id)}
      onBlur={() => onBlur(id)}
      onKeyDown={(e) => {
        // Escape dismisses the focused toast — same precedent as
        // ConfirmDialog's Escape handling, offered here as a second,
        // keyboard-native way to close a toast beyond Tab-then-Enter on the
        // X button.
        if (e.key === "Escape") {
          e.stopPropagation();
          onDismiss(id);
        }
      }}
      className="glim-toast pointer-events-auto flex w-80 max-w-[calc(100vw-2rem)] items-start gap-2.5 rounded-card bg-carbon-surface px-3.5 py-3 text-carbon-text"
    >
      <ToastGlyph severity={severity} />
      <p className="min-w-0 flex-1 text-sm leading-snug wrap-break-word">{message}</p>
      <button
        type="button"
        onClick={() => onDismiss(id)}
        aria-label={dismissLabel}
        className="-m-1 shrink-0 rounded-control p-1 text-carbon-textMuted opacity-80 hover:bg-carbon-hover hover:text-carbon-text hover:opacity-100 focus-visible:opacity-100 focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-(--focus-ring)"
      >
        <svg width="14" height="14" viewBox="0 0 16 16" fill="none" aria-hidden="true">
          <path d="M2.5 2.5l11 11M13.5 2.5l-11 11" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </svg>
      </button>
    </div>
  );
}

export interface ToastViewportEntry {
  id: string;
  message: string;
  severity: ToastSeverity;
}

export interface ToastViewportProps {
  toasts: ToastViewportEntry[];
  dismissLabel: string;
  onDismiss: (id: string) => void;
  onPause: (id: string) => void;
  onResume: (id: string) => void;
}

// Fixed bottom-end corner (inset-inline-end via Tailwind's logical `end-*`,
// per design-language.md's RTL section: the corner itself must follow
// writing direction, same as the reveal eye's `end-2`). New toasts are
// appended to the END of the array (toastEngine.addToast) and rendered in
// plain DOM order, so the newest toast lands closest to the anchored bottom
// edge (where it visually "enters" from) and older ones get pushed upward —
// no flex-reverse needed.
export function ToastViewport({ toasts, dismissLabel, onDismiss, onPause, onResume }: ToastViewportProps) {
  return (
    <div className="pointer-events-none fixed bottom-4 end-4 z-[70] flex flex-col gap-2">
      {toasts.map((t) => (
        <ToastCard
          key={t.id}
          id={t.id}
          message={t.message}
          severity={t.severity}
          dismissLabel={dismissLabel}
          onDismiss={onDismiss}
          onMouseEnter={onPause}
          onMouseLeave={onResume}
          onFocus={onPause}
          onBlur={onResume}
        />
      ))}
    </div>
  );
}
