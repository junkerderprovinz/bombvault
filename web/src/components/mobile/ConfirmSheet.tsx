import { useId } from "react";
import { Button } from "../Button";
import { IconCancel } from "../glyphs";
import { BottomSheet } from "./BottomSheet";

// ---------------------------------------------------------------------------
// ConfirmSheet is useConfirm's presentation below the 48rem breakpoint. The
// desktop card asks for a precise click on two side-by-side buttons inside a
// max-w-md box, which ignores everything a thumb-reachable surface owns:
// safe-area insets, the touch floor, chrome that stays put. The stateful half
// (lib/useConfirm.tsx) keeps its contract and only the surface changes.
//
// Hookless and portal-less, the same doctrine as ConfirmDialog: the portal,
// Escape, the Tab trap and the focus restore are BottomSheet's, so this file
// is props in and an element tree out. useConfirm's own Escape listener fires
// as well, which is benign because settle() nulls its resolver on the first
// call (asserted in ConfirmSheet.dom.test.tsx).
//
// The stack follows the desktop card's rule on a vertical axis: the forward
// action comes last, under the thumb's resting arc, with cancel above it, and
// both sit in BottomSheet's footer slot so a long message cannot scroll them
// away. Both stand on the key-control stage (--btn-h-key), the tallest
// sanctioned box, and both wear the accent: the question states the stakes,
// and a red button on every delete teaches people to read past it.
//
// The header carries no close button, because the footer already answers, so
// focus starts on Cancel. The message is the panel's aria-describedby target,
// so a screen reader announces the question and not just the title; its id
// comes from useId, since sheets can coexist.
// ---------------------------------------------------------------------------
export interface ConfirmSheetProps {
  /** Same generic window title ConfirmDialog takes (t("confirmDialog.title")). */
  title: string;
  /** The exact per-call-site copy passed to confirm(); unchanged, mechanism swap only. */
  message: string;
  confirmLabel: string;
  /** Translation key behind `confirmLabel`, so the commit button picks the
   *  trigger's glyph, exactly as ConfirmDialog does. */
  confirmLabelKey?: string;
  cancelLabel: string;
  onConfirm: () => void;
  onCancel: () => void;
}

export function ConfirmSheet({
  title,
  message,
  confirmLabel,
  confirmLabelKey,
  cancelLabel,
  onConfirm,
  onCancel,
}: ConfirmSheetProps) {
  const messageId = useId();
  return (
    <BottomSheet
      open
      onClose={onCancel}
      headerClose={false}
      title={title}
      describedBy={messageId}
      // The consumer padding is vertical only; the footer already owns the
      // inset-clamped sides.
      footer={
        <div className="flex flex-col gap-3 py-3">
          <Button
            label={cancelLabel}
            labelKey="common.cancel"
            glyph={<IconCancel />}
            tone="accent"
            onClick={onCancel}
            className="glim-btn-key w-full"
          />
          <Button
            label={confirmLabel}
            // The meaning changes with the action confirmed (delete, prune,
            // overwrite), so the key comes from the caller. The fallback is
            // the one useConfirm falls back to for the label, so word and
            // glyph stay the same answer.
            labelKey={confirmLabelKey ?? "common.confirm"}
            tone="accent"
            onClick={onConfirm}
            className="glim-btn-key w-full"
          />
        </div>
      }
    >
      {/* The message; ConfirmDialog's exact text treatment; the sheet body
          owns no content padding, so the consumer supplies it (the primitive's
          documented contract). Side padding stays off the content: the
          primitive's body is already inset-clamped, so a px here would double
          it; the block owns only its vertical breathing room. */}
      <div className="py-4">
        <p id={messageId} className="text-sm leading-relaxed text-carbon-textSub wrap-break-word">{message}</p>
      </div>
    </BottomSheet>
  );
}
