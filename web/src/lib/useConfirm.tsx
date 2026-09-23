import { useCallback, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { ConfirmDialog } from "../components/ConfirmDialog";
import { ConfirmSheet } from "../components/mobile/ConfirmSheet";
import { useT, type TranslationKey } from "./i18n";
import { useIsDesktop } from "./useMediaQuery";

// useConfirm replaces window.confirm(): ConfirmDialog at desktop widths,
// ConfirmSheet below the breakpoint. It keeps the one-string-in, boolean-out
// shape, only async:
//
//   const { confirm, confirmDialog } = useConfirm();
//   if (!(await confirm(t("x.deleteConfirm"), { confirmKey: "x.delete" }))) return;
//   return (<>... {confirmDialog}</>);
//
// The hook owns everything that needs `document` or hooks, so ConfirmDialog
// stays a plain function component: the portal, a document-level Escape
// listener (an onKeyDown on the dialog stops firing once focus falls to
// <body>), a Tab trap, and returning focus to the trigger on every close path.
//
// One pending request per instance is enough: the dialog is modal, so a second
// confirm() cannot be triggered while one is open.
export interface ConfirmOptions {
  /** The key of the button that asked, so the answer repeats its words and
   *  glyph ("Delete") rather than a bare "Confirm". */
  confirmKey?: TranslationKey;
  cancelLabel?: string;
}

interface PendingConfirm extends ConfirmOptions {
  message: string;
}

const FOCUSABLE_SELECTOR =
  'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

function focusableElements(root: HTMLElement): HTMLElement[] {
  return Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR));
}

export function useConfirm() {
  const { t } = useT();
  // The presentation half swaps at the one width breakpoint; ConfirmDialog
  // (the desktop card) at/above 48rem, ConfirmSheet (the bottom sheet, safe
  // cancel stacked above the confirm in the thumb-default bottom slot) below
  // it.
  // Nothing else about the contract moves: same confirm() promise, same
  // settle paths, same translated strings, zero per-call-site changes.
  const isDesktop = useIsDesktop();
  const [pending, setPending] = useState<PendingConfirm | null>(null);
  const resolveRef = useRef<((value: boolean) => void) | null>(null);
  const dialogRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLElement | null>(null);

  const confirm = useCallback((message: string, options?: ConfirmOptions) => {
    // Read before setPending: the re-render moves focus into the dialog.
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

  // An ancestor with a CSS transform (e.g. .glim-page-enter) would confine a
  // position: fixed backdrop to its own box, so the dialog goes to <body>.
  //
  // The sheet branch does not attach dialogRef: the ref drives
  // this hook's Tab trap, and BottomSheet already runs its own (same
  // FOCUSABLE_SELECTOR lift) over the panel; leaving the ref null makes the
  // trap below a no-op instead of fighting the sheet's. Escape fires from
  // both listeners on one keypress in the sheet branch; settle() nulls its
  // resolver on the first call, so the double dispatch is benign (the second
  // is a guarded no-op); asserted once-and-only-once in
  // ConfirmSheet.dom.test.tsx.
  const confirmDialog = pending
    ? createPortal(
        isDesktop ? (
          <ConfirmDialog
            ref={dialogRef}
            title={t("confirmDialog.title")}
            message={pending.message}
            confirmLabel={t(pending.confirmKey ?? "common.confirm")}
            confirmLabelKey={pending.confirmKey ?? "common.confirm"}
            cancelLabel={pending.cancelLabel ?? t("common.cancel")}
            onConfirm={() => settle(true)}
            onCancel={() => settle(false)}
          />
        ) : (
          <ConfirmSheet
            title={t("confirmDialog.title")}
            message={pending.message}
            confirmLabel={t(pending.confirmKey ?? "common.confirm")}
            confirmLabelKey={pending.confirmKey ?? "common.confirm"}
            cancelLabel={pending.cancelLabel ?? t("common.cancel")}
            onConfirm={() => settle(true)}
            onCancel={() => settle(false)}
          />
        ),
        document.body
      )
    : null;

  return { confirm, confirmDialog };
}
