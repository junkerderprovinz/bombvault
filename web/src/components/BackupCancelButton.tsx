// Stops a running backup. Unlike RestoreCancelButton it confirms in the normal
// tone: restic writes the snapshot last, so an aborted backup leaves only
// unreferenced data for the next prune and nothing on the host changes.
//
// It posts the backup's progress key ("files:<name>", "container:<name>",
// "vm:<name>", "flash", "config"). A key whose backup has finished answers
// cancelled:false, so a stale tab cannot cause an error.
//
// A container backup that has written its restore point and only starts its
// containers again cannot be cancelled, and the confirmation would promise a
// run without a snapshot. The progress stream marks that phase and the button
// hides. A dialog that is already open stays, and the server's answer then says
// the cancel came too late.

import { useState } from "react";
import { cancelBackup } from "../lib/api";
import type { useT } from "../lib/i18n";
import { useToast } from "../lib/toast";
import { useProgress } from "../lib/progress";
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
  const { push } = useToast();
  const committed = useProgress()[cancelKey]?.committed === true;

  async function handle() {
    const msg = t("backup.cancelConfirm").replace(/\{name\}/g, name);
    if (!(await confirm(msg))) return;
    setCancelling(true);
    try {
      const res = await cancelBackup(cancelKey);
      if (res.cancelled) {
        onCancelled?.();
      } else {
        // The button stays on screen until the next poll, so without a word
        // the click would look ignored.
        const key = res.reason === "committed" ? "backup.cancelTooLate" : "backup.cancelNotRunning";
        push(t(key).replace(/\{name\}/g, name), "warn");
      }
    } catch {
      // The backup keeps running and the button stays usable; the run row
      // shows what happened.
    } finally {
      setCancelling(false);
    }
  }

  return (
    <>
      {!committed && (
        <Button
          label={t("backup.cancel")}
          labelKey="backup.cancel"
          tone="neutral"
          onClick={() => void handle()}
          disabled={cancelling}
          busy={cancelling}
          title={cancelling ? t("restore.cancelling") : undefined}
        />
      )}
      {confirmDialog}
    </>
  );
}
