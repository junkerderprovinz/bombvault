import { Button } from "../Button";
import { IconCancel } from "../glyphs";
import { BottomSheet } from "./BottomSheet";

// The tone union the desktop ConfirmDialog dropped (GlimStone 1.12.0: no
// status colour on the commit button). The mobile sheet keeps it ON PURPOSE —
// a destructive confirm surfaces as the danger Button here — so the type is
// owned and exported HERE now; useConfirm imports it from this file.
export type ConfirmTone = "fail" | "warn";

// ---------------------------------------------------------------------------
// ConfirmSheet — the MOBILE presentation half of useConfirm.
//
// useConfirm's ONE promise API (`confirm(message) -> Promise<boolean>`)
// previously had exactly one face: ConfirmDialog, the centered desktop card.
// Below the 48rem breakpoint that card is the wrong shape twice over — it
// asks for a precise click on two side-by-side buttons, and its max-w-md
// card ignores everything a thumb-reachable surface has to own (safe-area
// insets, the 44px touch floor, chrome that stays put). So the stateful half
// (lib/useConfirm.tsx) keeps its exact contract and swaps the PRESENTATION
// half on useIsDesktop: desktop gets ConfirmDialog byte-identically (not one
// class token may move), narrow viewports get this sheet. Same promise, same
// settle paths, same translated strings — only the surface changes.
//
// Hookless and portal-less by construction, same doctrine as ConfirmDialog
// itself: everything modal lives in the stateful half or in BottomSheet (the
// sheet primitive), so this file is props in, an element tree out. The
// sheet's containment machinery (portal, Escape, Tab trap, focus capture and
// restore) is BottomSheet's; useConfirm's own Escape listener fires too, and
// that is benign — settle() nulls its resolver on first call, so the double
// dispatch resolves the promise exactly once (asserted in
// ConfirmSheet.dom.test.tsx).
//
// The button stack inverts the desktop card's order, on purpose:
//   - The DESTRUCTIVE action sits on TOP, furthest from the thumb's resting
//     arc, so a careless tap swipe must travel across the safe action first.
//     It carries the tone-mapped Button (danger for fail, warn for warn —
//     the same mapping ConfirmDialog's footer uses), full-width per the
//     stacked-action pattern.
//   - The SAFE action (cancel) sits LAST — the bottom-most, thumb-default
//     position — per the stacked-action pattern: "safe action = neutral,
//     rendered as the primary-position (thumb-default) control". BottomSheet's
//     open effect lands initial focus on the header close button (the safe
//     outcome), and nothing here re-focuses the destructive control: the
//     destructive control is never default-focused, which holds without an
//     autoFocus anywhere.
//
// Known gap, deliberate: the desktop card is aria-describedby its message;
// BottomSheet has no described-by slot yet, so the sheet announces title +
// visible message only. Adding the slot is a primitive change owned by the
// sheet primitive, not this consumer.
// ---------------------------------------------------------------------------
export interface ConfirmSheetProps {
  /** Same generic window title ConfirmDialog takes (t("confirmDialog.title")). */
  title: string;
  /** The exact per-call-site copy passed to confirm() — unchanged, mechanism swap only. */
  message: string;
  confirmLabel: string;
  cancelLabel: string;
  /** Fault-red for irreversible actions (the default), warn-amber for the
   *  "light" branch — the ConfirmTone union, passed straight through to both
   *  the panel surface (BottomSheet's tone) and the confirm Button. */
  tone?: ConfirmTone;
  onConfirm: () => void;
  onCancel: () => void;
}

export function ConfirmSheet({
  title,
  message,
  confirmLabel,
  cancelLabel,
  tone = "fail",
  onConfirm,
  onCancel,
}: ConfirmSheetProps) {
  return (
    <BottomSheet open onClose={onCancel} title={title} tone={tone}>
      {/* The message — ConfirmDialog's exact text treatment; the sheet body
          owns no content padding, so the consumer supplies it (the primitive's
          documented contract). */}
      <div className="px-4 py-4">
        <p className="text-sm leading-relaxed text-carbon-textSub wrap-break-word">{message}</p>
      </div>
      {/* Stacked actions in BottomSheet's footer slot: chrome pinned while the
          message scrolls, safe-area bottom inset owned by the primitive. */}
      <div className="flex flex-col gap-3 px-4 py-3">
        {/* bv-convention-exception: no-status-color-on-control -- the same
            sanctioned exception as ConfirmDialog's confirm button, cited by
            the design language itself: "the destructive control is always
            the fault colour". The guard exists to stop bespoke red on
            arbitrary controls; the ONE place status colour IS the meaning is
            the destructive confirmation, and this is its mobile face —
            tone is the closed ConfirmTone union, so no third shade can
            drift in. */}
        <Button
          label={confirmLabel}
          // The confirm button's meaning changes with the action it confirms
          // (delete, prune, overwrite), so no fixed translation key can pick
          // its glyph — ConfirmDialog passes the identical null. The
          // destructive control is never default-focused (no autoFocus; see
          // the header note).
          labelKey={null}
          tone={tone === "fail" ? "danger" : "warn"}
          onClick={onConfirm}
          className="w-full"
        />
        <Button
          label={cancelLabel}
          labelKey="common.cancel"
          glyph={<IconCancel />}
          tone="neutral"
          onClick={onCancel}
          className="w-full"
        />
      </div>
    </BottomSheet>
  );
}
