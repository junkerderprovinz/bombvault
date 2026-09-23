// Stops a running backup. Unlike RestoreCancelButton it confirms in the normal
// tone: restic writes the snapshot last, so an aborted backup leaves only
// unreferenced data for the next prune and nothing on the host changes.
//
// It posts the backup's progress key ("files:<name>", "container:<name>",
// "vm:<name>", "flash", "config"). A key whose backup has finished answers
// cancelled:false, so a stale tab cannot cause an error.

import { useState } from "react";
import { cancelBackup } from "../lib/api";
import type { useT } from "../lib/i18n";
import { useConfirm } from "../lib/useConfirm";
import { Button } from "./Button";

type T = ReturnType<typeof useT>["t"];

export function BackupCancelButton({
  cancelKey,
  name,
  t,
  onCancelled,
}: {
  /** The exact progress key the backend registered this backup under. */
  cancelKey: string;
  /** Human name substituted into the confirmation ({name}). */
  name: string;
  t: T;
  /** Called once the server accepted the cancellation, so the card can stop
   *  presenting the run as in progress before the next poll arrives. */
  onCancelled?: () => void;
}) {
  const [cancelling, setCancelling] = useState(false);
  const { confirm, confirmDialog } = useConfirm();

  async function handle() {
    const msg = t("backup.cancelConfirm").replace(/\{name\}/g, name);
    if (!(await confirm(msg))) return;
    setCancelling(true);
    try {
      await cancelBackup(cancelKey);
      onCancelled?.();
    } catch {
      // The backup keeps running and the button stays usable; the run row
      // shows what happened.
    } finally {
      setCancelling(false);
    }
  }

  return (
    <>
      <Button
        label={t("backup.cancel")}
        labelKey="backup.cancel"
        tone="neutral"
        onClick={() => void handle()}
        disabled={cancelling}
        busy={cancelling}
        title={cancelling ? t("restore.cancelling") : undefined}
      />
      {confirmDialog}
    </>
  );
}
