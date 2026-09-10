// ---------------------------------------------------------------------------
// BackupCancelButton — stop a backup that is running (#200).
//
// The counterpart of RestoreCancelButton, and deliberately a much quieter
// control, because the two actions are not comparable. Cancelling a RESTORE
// leaves a container gone and its appdata half-written, which is why that
// button opens a red confirmation naming what will be broken. Cancelling a
// BACKUP breaks nothing: restic writes its snapshot as the last act of a run,
// so an aborted backup leaves unreferenced data and no snapshot, and the next
// prune collects it. Nothing on the host is touched at all.
//
// So this asks once, in the light tone, and only because a long backup that
// somebody stops by accident is an hour of somebody's evening rather than a
// disaster. It does NOT borrow the restore's fault-red dialog.
//
// It POSTs the backup's exact progress key ("files:<id>", "container:<name>",
// "vm:<name>", "flash", "config"). A key that is no longer running answers
// cancelled:false and nothing happens, so the button in a browser tab that has
// not caught up cannot produce an error.
// ---------------------------------------------------------------------------

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
    if (!(await confirm(msg, { tone: "warn" }))) return;
    setCancelling(true);
    try {
      await cancelBackup(cancelKey);
      onCancelled?.();
    } catch {
      // A failed POST leaves the backup running, which is the safe outcome:
      // the button stays available and the run row remains the source of
      // truth for what actually happened.
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
