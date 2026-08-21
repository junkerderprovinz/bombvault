// ---------------------------------------------------------------------------
// ConfirmDialog — the GlimStone styled confirmation dialog (form-engine
// Task 7), replacing every window.confirm() call site in the app.
//
// Extracted from the SAME modal chrome already used by WhatsNewDialog.tsx and
// ErrorDetailPanel.tsx: rounded-card bg-carbon-surface, an anchored header
// (title + close X), a scrolling middle (the actual per-call-site message —
// same copy every call site already passed to window.confirm(), this task
// swaps the MECHANISM only, never the copy), an anchored footer (Cancel +
// Confirm). Backdrop-click/the header X/the footer Cancel button all resolve
// to the same onCancel. Escape and the Tab focus-trap are NOT handled here —
// see lib/useConfirm.tsx's header comment for why (a real modal needs both to
// work even when focus has left the dialog's own DOM subtree, which a React
// `onKeyDown` on this root can never observe).
//
// Pure, hookless function component on purpose — same shape as Toggle.tsx/
// Badge.tsx/RevealInput.tsx (props in, an element tree out) — so it stays
// unit-testable by calling it directly with props, no renderer/jsdom needed
// (see Toggle.test.ts's header comment: this repo's test suite is
// `environment: "node"` with zero DOM-rendering infrastructure — `document`
// itself is undefined there, which is also why the createPortal(...,
// document.body) call that makes this dialog genuinely modal lives in
// useConfirm.tsx instead of here: this component never touches `document`).
// The STATEFUL half (the pending-request queue, the promise plumbing, the
// portal, Escape, the focus trap, and returning focus to the trigger on
// close) lives in the companion hook, lib/useConfirm.tsx — mirrors
// useReveal.ts/RevealInput.tsx's split, extended to cover real modality.
//
// `ref` is a plain prop here (React 19 — function components accept `ref`
// without forwardRef): useConfirm.tsx attaches it to the dialog card so it
// can find the card's focusable elements for the Tab trap and query
// `document.activeElement` against it. Passing it through a plain function
// call in a test (as this file's own tests do) is inert — React only treats
// `ref` specially on a JSX-created host element, not on the props object a
// hand-written test builds, and no test here exercises it.
// ---------------------------------------------------------------------------
import type { Ref } from "react";
import { Badge } from "./Badge";

export type ConfirmTone = "fail" | "warn";

export interface ConfirmDialogProps {
  /** Generic, reusable dialog title (e.g. t("confirmDialog.title") = "Confirm")
   *  — NOT per-call-site copy; every migrated confirm() site already carried
   *  exactly one string, which is `message` below. */
  title: string;
  /** The exact string every call site used to pass to window.confirm() —
   *  unchanged copy, per Task 7's scope (mechanism swap only). */
  message: string;
  confirmLabel: string;
  cancelLabel: string;
  /** Accessible name for the header close (X) button — DELIBERATELY separate
   *  from cancelLabel: they are two distinct controls that both cancel, and
   *  sharing one label gave them the same accessible name (a screen-reader
   *  user would hear two identically-named controls; it also broke
   *  Playwright's own strict-mode selector matching in review). */
  closeLabel: string;
  /** Fault-red for a genuinely irreversible action (the default — every
   *  migrated call site but one is exactly this), warn-amber for
   *  RestoreCancelButton's "light" (non-destructive, restore-to-folder)
   *  branch. Design-language rule: "the destructive control is always the
   *  fault colour." */
  tone?: ConfirmTone;
  onConfirm: () => void;
  onCancel: () => void;
  /** The dialog card's root DOM node, for useConfirm.tsx's focus trap. */
  ref?: Ref<HTMLDivElement>;
}

const CONFIRM_BUTTON_TONE: Record<ConfirmTone, string> = {
  // text-carbon-background reads correctly against BOTH themes' fail/warn
  // solid values without a dedicated "-contrast" token: dark theme's
  // --status-fail-solid (#ff8389) and --status-warn-solid (#f1c21b) are both
  // light, so the DARK carbon-background (#161616 in dark theme) sits on top
  // with good contrast; light theme's versions (#da1e28 / #b28600) are both
  // dark, so the LIGHT carbon-background (#f4f4f4 in light theme) does the
  // same job. This is the same "opposite ground" trick Toggle.tsx's thumb
  // already relies on (bg-carbon-background sitting on the accent track).
  fail: "bg-statusFailSolid text-carbon-background",
  warn: "bg-statusWarnSolid text-carbon-background",
};

export function ConfirmDialog({
  title,
  message,
  confirmLabel,
  cancelLabel,
  closeLabel,
  tone = "fail",
  onConfirm,
  onCancel,
  ref,
}: ConfirmDialogProps) {
  return (
    <div
      className="bv-modal-backdrop fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      onClick={(e) => {
        if (e.target === e.currentTarget) onCancel();
      }}
    >
      <div
        ref={ref}
        role="dialog"
        aria-modal="true"
        aria-labelledby="confirmdialog-title"
        aria-describedby="confirmdialog-message"
        className="bv-modal-card relative flex max-h-[85vh] w-full max-w-md flex-col rounded-card bg-carbon-surface shadow-2xl"
      >
        {/* Header */}
        <div className="flex items-start justify-between gap-4 border-b border-carbon-border px-5 py-4">
          {/* Task 5 follow-up (rule 15 — "a window is a window... title as a
              badge"): a dialog's <h2> names the WINDOW CHROME itself, so it
              gets the same tone="heading" Badge treatment as a page's section
              headings (rule 11), just applied to a different element class.
              The <h2>+id stays exactly where it was — aria-labelledby reads
              the referenced element's computed text content, which still
              includes the Badge's text regardless of the span nested inside,
              so the accessible name is unaffected by this markup change.
              GlimStone follow-up pass ("half-overlap card notch"): `relative`
              added on the OUTER dialog div above (not this Header, not the
              <h2> below) — this outer box has no overflow/scroll of its own
              (only the Body further down scrolls), so the heading Badge's
              new `position: absolute` -11px poke straddles the WHOLE
              modal's own top edge cleanly, unclipped; see Badge.tsx's
              badgeClassName comment. */}
          <h2 id="confirmdialog-title" className="flex items-center">
            <Badge tone="heading" size="heading" wrap>{title}</Badge>
          </h2>
          <button
            type="button"
            onClick={onCancel}
            aria-label={closeLabel}
            className="shrink-0 rounded-control p-1 text-carbon-textMuted hover:bg-carbon-hover hover:text-carbon-text"
          >
            <svg width="20" height="20" viewBox="0 0 20 20" fill="none" aria-hidden="true">
              <path d="M5 5l10 10M15 5L5 15" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
            </svg>
          </button>
        </div>

        {/* Body (scrolls) — the real per-call-site question/explanation. Also
            the dialog's accessible description (aria-describedby above), so
            the stake-bearing confirmation text is announced, not just shown. */}
        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
          <p id="confirmdialog-message" className="text-sm leading-relaxed text-carbon-textSub wrap-break-word">
            {message}
          </p>
        </div>

        {/* Footer */}
        <div className="flex items-center justify-end gap-3 border-t border-carbon-border px-5 py-4">
          <button
            type="button"
            autoFocus
            onClick={onCancel}
            className="rounded-control bg-carbon-surface3 px-4 py-2 text-sm font-medium text-carbon-text hover:bg-carbon-hover"
          >
            {cancelLabel}
          </button>
          <button
            type="button"
            onClick={onConfirm}
            className={`rounded-control px-4 py-2 text-sm font-medium transition-opacity hover:opacity-90 ${CONFIRM_BUTTON_TONE[tone]}`}
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}
