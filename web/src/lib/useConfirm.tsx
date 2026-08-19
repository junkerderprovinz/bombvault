import { useCallback, useRef, useState } from "react";
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
// button is the only way to reach either call and it's disabled while busy.
export interface ConfirmOptions {
  confirmLabel?: string;
  cancelLabel?: string;
  tone?: ConfirmTone;
}

interface PendingConfirm extends ConfirmOptions {
  message: string;
}

export function useConfirm() {
  const { t } = useT();
  const [pending, setPending] = useState<PendingConfirm | null>(null);
  const resolveRef = useRef<((value: boolean) => void) | null>(null);

  const confirm = useCallback((message: string, options?: ConfirmOptions) => {
    return new Promise<boolean>((resolve) => {
      resolveRef.current = resolve;
      setPending({ message, ...options });
    });
  }, []);

  const settle = useCallback((result: boolean) => {
    resolveRef.current?.(result);
    resolveRef.current = null;
    setPending(null);
  }, []);

  const confirmDialog = pending ? (
    <ConfirmDialog
      title={t("confirmDialog.title")}
      message={pending.message}
      confirmLabel={pending.confirmLabel ?? t("common.confirm")}
      cancelLabel={pending.cancelLabel ?? t("common.cancel")}
      tone={pending.tone ?? "fail"}
      onConfirm={() => settle(true)}
      onCancel={() => settle(false)}
    />
  ) : null;

  return { confirm, confirmDialog };
}
