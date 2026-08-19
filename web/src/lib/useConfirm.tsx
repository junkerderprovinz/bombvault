import { useCallback, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { ConfirmDialog, type ConfirmTone } from "../components/ConfirmDialog";
import { useT } from "./i18n";

// ---------------------------------------------------------------------------
// useConfirm — the stateful half of ConfirmDialog (GlimStone form-engine
// Task 7), the direct replacement for window.confirm() across the app.
//
// Mirrors useReveal.ts/RevealInput.tsx's exact split: the pending-request
// queue and the promise plumbing live HERE so ConfirmDialog itself stays a
// pure, hookless function component, callable directly as a plain function in
// ConfirmDialog.test.ts (same shape as Toggle.tsx/Badge.tsx/RevealInput.tsx —
// see ConfirmDialog.tsx's own header comment for why).
//
// This hook ALSO owns everything that makes the dialog a genuinely modal
// dialog rather than an inline styled box, because every one of those needs
// either `document` (undefined in ConfirmDialog.test.ts's node-environment
// unit tests) or a real hook (useEffect/useRef), neither of which
// ConfirmDialog.tsx may use:
//   - createPortal(..., document.body) — renders past any ancestor with a CSS
//     transform (a transform creates a new containing block, so a
//     `position: fixed` backdrop nested under one no longer covers the real
//     viewport). Exactly InfoBubble.tsx's fix for the identical problem.
//   - Escape — a document-level keydown listener, not a React `onKeyDown` on
//     the dialog root: the old onKeyDown only fired while focus was still
//     somewhere inside the dialog, so clicking the (unfocusable) message text
//     or tabbing out moved focus to <body> and silently killed Escape for
//     good. Matches WhatsNewDialog.tsx/ErrorDetailPanel.tsx/FilterPopover.tsx/
//     InfoBubble.tsx/RestorePanel.tsx/Sidebar.tsx's own document-listener
//     pattern.
//   - A Tab/Shift+Tab focus trap over the dialog card's own focusable
//     elements (header close-X, Cancel, Confirm), so focus can never land on
//     the page behind a dialog that is now ACTUALLY covering it (a portal +
//     backdrop with nothing stopping Tab would just be a bigger version of
//     the same escape-the-modal bug).
//   - Returning focus to whatever triggered the confirm() call once it
//     settles, on all four close paths (Escape / Cancel / Confirm /
//     backdrop-click) — they all funnel through `settle` below.
//
// Call sites keep window.confirm()'s exact control-flow shape — same
// one-string-in, boolean-out contract, just async and non-blocking instead of
// a native, unstylable, tab-freezing dialog:
//
//   if (!window.confirm(t("x.deleteConfirm"))) return;
//   ...
// becomes
//   const { confirm, confirmDialog } = useConfirm();
//   ...
//   if (!(await confirm(t("x.deleteConfirm")))) return;
//   ...
//   return (<>... {confirmDialog}</>);
//
// One useConfirm() instance per component is enough even when a component
// has several confirm() call sites (VMSnapshotRow, IntegrityCard, ...): only
// one confirmation can ever be genuinely pending for a given user at a time,
// so the single pending-request slot is never a real constraint — it just
// means a second confirm() call before the first settles would replace the
// pending dialog, which never happens in practice since the triggering
// button is the only way to reach either call and it's disabled while busy
// (and, now that the dialog is genuinely modal, physically unreachable while
// one is already open).
export interface ConfirmOptions {
  confirmLabel?: string;
  cancelLabel?: string;
  tone?: ConfirmTone;
}

interface PendingConfirm extends ConfirmOptions {
  message: string;
}

// The dialog card's own focusable controls, in DOM/tab order: header
// close-X, Cancel, Confirm. Kept generic (not hardcoded to those three)
// so it still holds if the card ever grows another focusable element.
const FOCUSABLE_SELECTOR =
  'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

function focusableElements(root: HTMLElement): HTMLElement[] {
  return Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR));
}

export function useConfirm() {
  const { t } = useT();
  const [pending, setPending] = useState<PendingConfirm | null>(null);
  const resolveRef = useRef<((value: boolean) => void) | null>(null);
  // The dialog card's DOM node (for the Tab trap) and whatever had focus the
  // moment confirm() was called (to restore it once the dialog closes).
  const dialogRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLElement | null>(null);

  const confirm = useCallback((message: string, options?: ConfirmOptions) => {
    // Captured BEFORE setPending fires the re-render that auto-focuses the
    // dialog's own Cancel button — after that, document.activeElement would
    // already be the dialog, not the button that opened it.
    const active = document.activeElement;
    triggerRef.current = active instanceof HTMLElement && active !== document.body ? active : null;
    return new Promise<boolean>((resolve) => {
      resolveRef.current = resolve;
      setPending({ message, ...options });
    });
  }, []);

  const settle = useCallback((result: boolean) => {
    resolveRef.current?.(result);
    resolveRef.current = null;
    setPending(null);
    const trigger = triggerRef.current;
    triggerRef.current = null;
    if (trigger && document.contains(trigger)) trigger.focus();
  }, []);

  // Escape (document-level, so it works no matter where focus currently is)
  // + the Tab/Shift+Tab focus trap. Both only need to exist while a
  // confirmation is actually showing.
  useEffect(() => {
    if (!pending) return;
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") {
        e.preventDefault();
        settle(false);
        return;
      }
      if (e.key !== "Tab") return;
      const card = dialogRef.current;
      if (!card) return;
      const focusables = focusableElements(card);
      if (focusables.length === 0) return;
      const first = focusables[0];
      const last = focusables[focusables.length - 1];
      const active = document.activeElement;
      const insideCard = active instanceof Node && card.contains(active);
      if (e.shiftKey) {
        if (!insideCard || active === first) {
          e.preventDefault();
          last.focus();
        }
      } else {
        if (!insideCard || active === last) {
          e.preventDefault();
          first.focus();
        }
      }
    }
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [pending, settle]);

  // Portal-rendered to <body> (InfoBubble.tsx's fix for the same problem):
  // any ancestor of the call site with a CSS transform (e.g. .bv-page-enter)
  // creates a new containing block, so a `position: fixed` backdrop nested
  // under it only covers that ancestor's box, not the real viewport.
  const confirmDialog = pending
    ? createPortal(
        <ConfirmDialog
          ref={dialogRef}
          title={t("confirmDialog.title")}
          message={pending.message}
          confirmLabel={pending.confirmLabel ?? t("common.confirm")}
          cancelLabel={pending.cancelLabel ?? t("common.cancel")}
          closeLabel={t("common.close")}
          tone={pending.tone ?? "fail"}
          onConfirm={() => settle(true)}
          onCancel={() => settle(false)}
        />,
        document.body
      )
    : null;

  return { confirm, confirmDialog };
}
