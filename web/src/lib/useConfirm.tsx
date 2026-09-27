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
// A click cannot reach the page behind an open question, but a timer can: a
// question asked while another is open waits its turn instead of taking the
// first one's place, so every caller gets its own answer.
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
  id: number;
  message: string;
  resolve: (value: boolean) => void;
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
  const queue = useRef<PendingConfirm[]>([]);
  const nextId = useRef(0);
  const dialogRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLElement | null>(null);

  const confirm = useCallback((message: string, options?: ConfirmOptions) => {
    // Read before setPending: the re-render moves focus into the dialog. A
    // question that waits its turn finds focus in the one on screen, whose
    // buttons are gone by the time focus goes back.
    if (queue.current.length === 0) {
      const active = document.activeElement;
      triggerRef.current = active instanceof HTMLElement && active !== document.body ? active : null;
    }
    return new Promise<boolean>((resolve) => {
      const question = { ...options, id: nextId.current++, message, resolve };
      queue.current.push(question);
      if (queue.current.length > 1) return;
      setTyped("");
      setPending(question);
    });
  }, []);

  const settle = useCallback((result: boolean) => {
    const [answered, ...waiting] = queue.current;
    queue.current = waiting;
    answered?.resolve(result);
    setTyped("");
    setPending(waiting[0] ?? null);
    if (waiting.length > 0) return;
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
  // sheet gets no dialogRef because BottomSheet owns Escape and the Tab trap,
  // and useDialogKeys leaves a window it has no card for alone. Each question
  // mounts afresh, so the next one starts on Cancel rather than on the button
  // that answered the last.
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
  const confirmDialog =
    pending && surface
    ? createPortal(
        isDesktop ? (
          <ConfirmDialog key={pending.id} ref={dialogRef} {...surface} />
        ) : (
          <ConfirmSheet key={pending.id} {...surface} />
        ),
        document.body
      )
    : null;

  return { confirm, confirmDialog, dismiss };
}

/** useDialogKeys gives an open dialog Escape from anywhere and a Tab trap over
 *  its own controls, so focus never reaches the page it covers. The card must
 *  carry aria-modal="true": a window opened on top of it, such as a question
 *  asked from inside it, is the last such card in the document and takes the
 *  keys. */
export function useDialogKeys(open: boolean, dialogRef: RefObject<HTMLDivElement | null>, onCancel: () => void): void {
  useEffect(() => {
    if (!open) return;
    function onKeyDown(e: KeyboardEvent) {
      // The window on top may have answered this Escape and gone before this
      // listener runs, which would leave this one looking like the top.
      if (e.defaultPrevented) return;
      const card = dialogRef.current;
      const windows = document.querySelectorAll('[aria-modal="true"]');
      if (!card || windows[windows.length - 1] !== card) return;
      if (e.key === "Escape") {
        e.preventDefault();
        onCancel();
        return;
      }
      if (e.key !== "Tab") return;
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
