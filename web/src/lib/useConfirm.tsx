import { useCallback, useId, useRef, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";
import type { ButtonTone } from "../components/Button";
import { ConfirmDialog } from "../components/ConfirmDialog";
import { ConfirmSheet } from "../components/mobile/ConfirmSheet";
import { SettingRow } from "../components/storage/rows";
import { useDialogKeys } from "../components/useConfirm";
import { useT, type TranslationKey } from "./i18n";
import { useBackCloses } from "./useBackCloses";
import { useIsDesktop } from "./useMediaQuery";

// useConfirm replaces window.confirm(): ConfirmDialog at desktop widths,
// ConfirmSheet below the breakpoint. It keeps the one-string-in, boolean-out
// shape, only async:
//
//   const { confirm, confirmDialog } = useConfirm();
//   if (!(await confirm(t("x.deleteConfirm"), { confirmKey: "x.delete" }))) return;
//   return (<>... {confirmDialog}</>);
//
// GlimStone's hook in components/useConfirm.tsx draws the desktop card alone
// and takes its title once per page. The app calls this one, which adds the
// phone sheet, a title, tones and a typed answer per question, and Back
// closing the dialog; Escape and the Tab trap come from GlimStone's
// useDialogKeys.
//
// One pending request per instance is enough: the dialog is modal, so a second
// confirm() cannot be triggered while one is open.
export interface ConfirmOptions {
  /** A title that names the question, in place of the generic "Confirm". */
  title?: string;
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
  /** Why the action cannot go ahead; disables Confirm. */
  confirmBlocked?: string;
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
  const typedId = useId();
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

  // For a question whose words arrive after the window opened: the answer
  // still settles the first confirm() call.
  const revise = useCallback((message: string, options?: ConfirmOptions) => {
    setPending((open) => (open ? { message, ...options } : open));
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
  // The sheet answers Back itself.
  useBackCloses(pending !== null && isDesktop, dismiss);

  // Locked while a required text is still unmatched; undefined for every
  // caller that never asked for one, so the ordinary dialog stays unaffected.
  const locked = pending?.requireText !== undefined && typed !== pending.requireText;
  const extra =
    pending?.requireText === undefined ? (
      pending?.extra
    ) : (
      <>
        {pending.extra}
        <div className="mt-3">
          <SettingRow label={pending.requirePrompt ?? ""} htmlFor={typedId}>
            <input
              id={typedId}
              type="text"
              value={typed}
              onChange={(e) => setTyped(e.target.value)}
              autoComplete="off"
              spellCheck={false}
              className="w-72 max-w-full rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus"
            />
          </SettingRow>
        </div>
      </>
    );

  // An ancestor with a CSS transform (e.g. .glim-page-enter) would confine a
  // position: fixed backdrop to its own box, so the dialog goes to <body>. The
  // sheet gets no dialogRef because BottomSheet traps Tab itself; Escape
  // reaches both listeners, and settle() ignores the second call.
  const surface = pending && {
    title: pending.title ?? t("confirmDialog.title"),
    message: pending.message,
    confirmLabel: pending.confirmLabel ?? t(pending.confirmKey ?? "common.confirm"),
    confirmLabelKey: pending.confirmLabelKey ?? pending.confirmKey ?? "common.confirm",
    cancelLabel: pending.cancelLabel ?? t("common.cancel"),
    extra,
    confirmDisabled: locked,
    confirmBlocked: pending.confirmBlocked,
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

  return { confirm, confirmDialog, dismiss, revise };
}
