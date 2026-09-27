import { useIsDesktop } from "../lib/useMediaQuery";
import { ConfirmDialog, type ConfirmDialogProps } from "./ConfirmDialog";
import { ConfirmSheet } from "./mobile/ConfirmSheet";

// ConfirmPrompt is the confirmation for a card that keeps its own pending state
// rather than awaiting useConfirm, usually because the question has a title of
// its own. Like useConfirm it shows the dialog card at and above 48rem and the
// bottom sheet below, where the card's two buttons side by side do not fit.
export type ConfirmPromptProps = Omit<ConfirmDialogProps, "ref" | "confirmGlyph">;

export function ConfirmPrompt(props: ConfirmPromptProps) {
  return useIsDesktop() ? <ConfirmDialog {...props} /> : <ConfirmSheet {...props} />;
}
