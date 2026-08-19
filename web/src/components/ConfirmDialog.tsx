// ---------------------------------------------------------------------------
// ConfirmDialog — the GlimStone styled confirmation dialog (form-engine
// Task 7), replacing every window.confirm() call site in the app.
//
// Extracted from the SAME modal chrome already used by WhatsNewDialog.tsx and
// ErrorDetailPanel.tsx: rounded-card bg-carbon-surface, an anchored header
// (title + close X), a scrolling middle (the actual per-call-site message —
// same copy every call site already passed to window.confirm(), this task
// swaps the MECHANISM only, never the copy), an anchored footer (Cancel +
// Confirm). Escape/backdrop-click/the header X/the footer Cancel button all
// resolve to the same onCancel — four independent ways out, matching
// WhatsNewDialog's own close-affordance count.
//
// Pure, hookless function component on purpose — same shape as Toggle.tsx/
// Badge.tsx/RevealInput.tsx (props in, an element tree out) — so it stays
// unit-testable by calling it directly with props, no renderer/jsdom needed
// (see Toggle.test.ts's header comment: this repo's test suite is
// `environment: "node"` with zero DOM-rendering infrastructure). That is
// exactly why Escape is wired as a React `onKeyDown` on the dialog's own root
// (bubles up from the auto-focused Cancel button) instead of a
// document-level `useEffect` listener the way WhatsNewDialog/ErrorDetailPanel
// do it: a `useEffect` here would need a dispatcher that only exists while
// React is actually rendering, which calling this function directly in a
// test does not provide. The STATEFUL half (the pending-request queue, the
// promise plumbing) lives in the companion hook, lib/useConfirm.tsx —
// mirrors useReveal.ts/RevealInput.tsx's exact split for the same reason.
// ---------------------------------------------------------------------------

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
  /** Fault-red for a genuinely irreversible action (the default — every
   *  migrated call site but one is exactly this), warn-amber for
   *  RestoreCancelButton's "light" (non-destructive, restore-to-folder)
   *  branch. Design-language rule: "the destructive control is always the
   *  fault colour." */
  tone?: ConfirmTone;
  onConfirm: () => void;
  onCancel: () => void;
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
  tone = "fail",
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  return (
    <div
      className="bv-modal-backdrop fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      onClick={(e) => {
        if (e.target === e.currentTarget) onCancel();
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="confirmdialog-title"
        onKeyDown={(e) => {
          if (e.key === "Escape") onCancel();
        }}
        className="bv-modal-card flex max-h-[85vh] w-full max-w-md flex-col rounded-card bg-carbon-surface shadow-2xl"
      >
        {/* Header */}
        <div className="flex items-start justify-between gap-4 border-b border-carbon-border px-5 py-4">
          <h2 id="confirmdialog-title" className="text-lg font-semibold text-carbon-text">
            {title}
          </h2>
          <button
            type="button"
            onClick={onCancel}
            aria-label={cancelLabel}
            className="shrink-0 rounded-control p-1 text-carbon-textMuted hover:bg-carbon-hover hover:text-carbon-text"
          >
            <svg width="20" height="20" viewBox="0 0 20 20" fill="none" aria-hidden="true">
              <path d="M5 5l10 10M15 5L5 15" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
            </svg>
          </button>
        </div>

        {/* Body (scrolls) — the real per-call-site question/explanation. */}
        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
          <p className="text-sm leading-relaxed text-carbon-textSub wrap-break-word">{message}</p>
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
