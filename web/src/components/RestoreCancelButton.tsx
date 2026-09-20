// Cancels a running restore. The confirmation depends on where the restore
// writes: in place, the target is left partly restored and has to be restored
// again; into a chosen folder, the partial output stays there.
//
// It posts the restore's progress key ("container:<name>", "vm:<name>",
// "stack:<project>"). A cancelled restore is recorded as cancelled, not as a
// failure.

import { useState, type MutableRefObject } from "react";
import { cancelRestore } from "../lib/api";
import type { useT } from "../lib/i18n";
import { useConfirm } from "../lib/useConfirm";
import { Button } from "./Button";

type T = ReturnType<typeof useT>["t"];

export function RestoreCancelButton({
  cancelKey,
  inPlace,
  name,
  confirmText,
  t,
  cancelledRef,
}: {
  /** The exact progress key the backend registered this restore under. */
  cancelKey: string;
  /** True for a destructive in-place restore (hard warning); false for a
   *  restore-to-a-folder (light warning). */
  inPlace: boolean;
  /** Human name substituted into the in-place warning ({name}). */
  name: string;
  /** Replaces the warning where cancelling leaves something else behind than a
   *  half-written folder. */
  confirmText?: string;
  t: T;
  /** Paired watch's cancelled flag: set true on a successful cancel so a no-run
   *  restore finishes "cancelled", not a green "Restored". */
  cancelledRef?: MutableRefObject<boolean>;
}) {
  const [cancelling, setCancelling] = useState(false);
  const { confirm, confirmDialog } = useConfirm();

  async function handle() {
    const msg =
      confirmText ??
      (inPlace
        ? t("restore.cancelConfirmInPlace").replace(/\{name\}/g, name)
        : t("restore.cancelConfirmSafe"));
    // The warning is in the message; the dialog has no colour of its own.
    if (!(await confirm(msg))) return;
    setCancelling(true);
    try {
      await cancelRestore(cancelKey);
      if (cancelledRef) cancelledRef.current = true;
    } catch {
      // The restore keeps running and the button stays usable; the watch
      // reports the outcome.
    } finally {
      setCancelling(false);
    }
  }

  return (
    <>
      <Button
        label={t("restore.cancel")}
        labelKey="restore.cancel"
        tone="neutral"
        onClick={() => void handle()}
        disabled={cancelling}
        busy={cancelling}
        title={cancelling ? t("restore.cancelling") : undefined}
        className="self-start"
      />
      {confirmDialog}
    </>
  );
}
