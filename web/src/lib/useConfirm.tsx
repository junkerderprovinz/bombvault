import { useCallback, useEffect, useRef, useState, type ReactNode, type RefObject } from "react";
import { createPortal } from "react-dom";
import type { ButtonTone } from "../components/Button";
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
  /** A composed label, for an answer that carries a name or a count and so has
   *  no key of its own. It wins over confirmKey. */
  confirmLabel?: string;
  /** Translation key behind confirmLabel, so the confirm button shows its glyph. */
  confirmLabelKey?: string;
  cancelLabel?: string;
  /** Lines or switches the answer needs, shown under the question. */
  extra?: ReactNode;
  /** The confirm button stays locked until exactly this has been typed. */
  requireText?: string;
  /** Label of the field requireText is typed into. */
  requirePrompt?: string;
  /** Surface of the confirm button; a delete that leaves Cancel the one accent passes "neutral". */
  confirmTone?: ButtonTone;
  /** Surface of Cancel; a question whose answer is the one accent passes "neutral". */
  cancelTone?: ButtonTone;
}

interface PendingConfirm extends ConfirmOptions {
  message: string;
}

const FOCUSABLE_SELECTOR =
  'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

export function focusableElements(root: HTMLElement): HTMLElement[] {
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
  const [typed, setTyped] = useState("");
  const resolveRef = useRef<((value: boolean) => void) | null>(null);
  const dialogRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLElement | null>(null);

  const confirm = useCallback((message: string, options?: ConfirmOptions) => {
    // Read before setPending: the re-render moves focus into the dialog.
    const active = document.activeElement;
    triggerRef.current = active instanceof HTMLElement && active !== document.body ? active : null;
    setTyped("");
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

  // For a caller whose question stopped making sense while the dialog was
  // open, and for Escape.
  const dismiss = useCallback(() => settle(false), [settle]);
  useDialogKeys(pending !== null, dialogRef, dismiss);

  // Locked while a required text is still unmatched; undefined for every
  // caller that never asked for one, so the ordinary dialog stays unaffected.
  const locked = pending?.requireText !== undefined && typed !== pending.requireText;
  const extra =
    pending?.requireText === undefined ? (
      pending?.extra
    ) : (
      <>
        {pending.extra}
        <label className="mt-3 flex flex-col gap-1 text-xs text-carbon-textSub">
          {pending.requirePrompt}
          <input
            type="text"
            value={typed}
            onChange={(e) => setTyped(e.target.value)}
            autoComplete="off"
            spellCheck={false}
            className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus"
          />
        </label>
      </>
    );

  // An ancestor with a CSS transform (e.g. .glim-page-enter) would confine a
  // position: fixed backdrop to its own box, so the dialog goes to <body>. The
  // sheet gets no dialogRef because BottomSheet traps Tab itself; Escape
  // reaches both listeners, and settle() ignores the second call.
  const surface = pending && {
    title: t("confirmDialog.title"),
    message: pending.message,
    confirmLabel: pending.confirmLabel ?? t(pending.confirmKey ?? "common.confirm"),
    confirmLabelKey: pending.confirmLabelKey ?? pending.confirmKey ?? "common.confirm",
    cancelLabel: pending.cancelLabel ?? t("common.cancel"),
    extra,
    confirmDisabled: locked,
    confirmTone: pending.confirmTone,
    cancelTone: pending.cancelTone,
    onConfirm: () => settle(true),
    onCancel: () => settle(false),
  };
  const confirmDialog = surface
    ? createPortal(
        isDesktop ? <ConfirmDialog ref={dialogRef} {...surface} /> : <ConfirmSheet {...surface} />,
        document.body
      )
    : null;

  return { confirm, confirmDialog, dismiss };
}

/** useDialogKeys gives an open dialog Escape from anywhere and a Tab trap over
 *  its own controls, so focus never reaches the page it covers. */
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
      } else if (!insideCard || active === last) {
        e.preventDefault();
        first.focus();
      }
    }
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [open, dialogRef, onCancel]);
}
