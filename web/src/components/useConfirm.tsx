import { useCallback, useEffect, useRef, useState, type ReactNode, type RefObject } from "react";
import { createPortal } from "react-dom";
import { ConfirmDialog } from "./ConfirmDialog";

// useConfirm replaces window.confirm() with ConfirmDialog, keeping the
// one-question-in, boolean-out shape, only async:
//
//   const { confirm, confirmDialog } = useConfirm({ title, cancel });
//   if (!(await confirm("Delete the snapshot?", { confirmLabel: "Delete" }))) return;
//   return <>... {confirmDialog}</>;
//
// The hook owns everything that needs `document`, so ConfirmDialog stays a plain
// component: the portal, a document-level Escape listener (an onKeyDown on the
// dialog stops firing once focus falls to <body>), a Tab trap, and returning
// focus to the control that asked on every way out.
//
// One pending question per instance is enough: the dialog is modal, so a second
// confirm() cannot be triggered while one is open.

export interface ConfirmText {
  /** The dialog's title, the same for every question, such as "Confirm". */
  title: string;
  cancel: string;
  /** The confirm button's words where a question passes none of its own. */
  confirm: string;
}

export interface ConfirmOptions {
  /** The words of the action being confirmed, so the answer repeats "Delete"
   *  rather than a bare "Confirm". */
  confirmLabel?: string;
  /** Translation key behind confirmLabel, so the confirm button shows its glyph. */
  confirmLabelKey?: string;
  confirmGlyph?: ReactNode;
  /** A switch or two the answer needs, shown under the question. */
  extra?: ReactNode;
}

interface Pending extends ConfirmOptions {
  message: string;
}

const FOCUSABLE =
  'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

export function useConfirm(text: ConfirmText) {
  const [pending, setPending] = useState<Pending | null>(null);
  const resolveRef = useRef<((value: boolean) => void) | null>(null);
  const dialogRef = useRef<HTMLDivElement>(null);
  const askerRef = useRef<HTMLElement | null>(null);

  const confirm = useCallback((message: string, options?: ConfirmOptions) => {
    // Read before the dialog renders, which moves focus into it.
    const active = document.activeElement;
    askerRef.current = active instanceof HTMLElement && active !== document.body ? active : null;
    return new Promise<boolean>((resolve) => {
      resolveRef.current = resolve;
      setPending({ message, ...options });
    });
  }, []);

  const settle = useCallback((result: boolean) => {
    resolveRef.current?.(result);
    resolveRef.current = null;
    setPending(null);
    const asker = askerRef.current;
    askerRef.current = null;
    if (asker && document.contains(asker)) asker.focus();
  }, []);

  /** For a caller whose question stopped making sense while it was open. */
  const dismiss = useCallback(() => settle(false), [settle]);
  useDialogKeys(pending !== null, dialogRef, dismiss);

  // An ancestor with a transform would confine a fixed backdrop to its own box,
  // so the dialog goes to <body>.
  const confirmDialog = pending
    ? createPortal(
        <ConfirmDialog
          ref={dialogRef}
          title={text.title}
          message={pending.message}
          confirmLabel={pending.confirmLabel ?? text.confirm}
          confirmLabelKey={pending.confirmLabelKey}
          confirmGlyph={pending.confirmGlyph}
          cancelLabel={text.cancel}
          extra={pending.extra}
          onConfirm={() => settle(true)}
          onCancel={() => settle(false)}
        />,
        document.body,
      )
    : null;

  return { confirm, confirmDialog, dismiss };
}

/** Escape from anywhere and a Tab trap over the dialog's own controls, so focus
 *  never reaches the page it covers. Exported for any other modal window. */
export function useDialogKeys(open: boolean, dialogRef: RefObject<HTMLDivElement | null>, onCancel: () => void): void {
  useEffect(() => {
    if (!open) return;
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") {
        e.preventDefault();
        onCancel();
        return;
      }
      if (e.key !== "Tab") return;
      const card = dialogRef.current;
      if (!card) return;
      const focusables = Array.from(card.querySelectorAll<HTMLElement>(FOCUSABLE));
      if (focusables.length === 0) return;
      const first = focusables[0];
      const last = focusables[focusables.length - 1];
      const active = document.activeElement;
      const inside = active instanceof Node && card.contains(active);
      if (e.shiftKey) {
        if (!inside || active === first) {
          e.preventDefault();
          last.focus();
        }
      } else if (!inside || active === last) {
        e.preventDefault();
        first.focus();
      }
    }
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [open, dialogRef, onCancel]);
}
