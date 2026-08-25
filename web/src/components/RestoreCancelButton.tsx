// ---------------------------------------------------------------------------
// RestoreCancelButton — cancel an in-flight restore, with a type-aware confirm.
//
// The confirmation text depends on the restore's ACTUAL destination:
//   - in-place (original locations): the hard warning — the target is left
//     partially restored and must be restored again to be usable.
//   - to a chosen folder (non-destructive): the light warning — the partial
//     output folder is simply left as-is.
//
// On confirm it POSTs POST /api/restore/cancel with the restore's exact progress
// key ("container:<name>" / "vm:<name>" / "stack:<project>"). Cancelling maps to a
// recorded "cancelled" run (not a failure); the fire-and-watch surfaces the
// neutral cancelled banner.
// ---------------------------------------------------------------------------

import { useState, type MutableRefObject } from "react";
import { cancelRestore } from "../lib/api";
import type { useT } from "../lib/i18n";
import { useConfirm } from "../lib/useConfirm";

type T = ReturnType<typeof useT>["t"];

export function RestoreCancelButton({
  cancelKey,
  inPlace,
  name,
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
  t: T;
  /** Paired watch's cancelled flag: set true on a successful cancel so a no-run
   *  restore finishes "cancelled", not a green "Restored". */
  cancelledRef?: MutableRefObject<boolean>;
}) {
  const [cancelling, setCancelling] = useState(false);
  const { confirm, confirmDialog } = useConfirm();

  async function handle() {
    const msg = inPlace
      ? t("restore.cancelConfirmInPlace").replace(/\{name\}/g, name)
      : t("restore.cancelConfirmSafe");
    // inPlace keeps the hard (fault-red) tone — the target is left partially
    // restored and must be restored again; the light warning (restore-to-a-
    // folder, non-destructive) downgrades to warn (amber) so the dialog's
    // own visual weight still tracks the same hard/light distinction the
    // message copy already carries, unchanged, from before this mechanism
    // swap (GlimStone form-engine Task 7).
    if (!(await confirm(msg, { tone: inPlace ? "fail" : "warn" }))) return;
    setCancelling(true);
    try {
      await cancelRestore(cancelKey);
      // The cancel was accepted server-side: mark it so the watch's no-run
      // fallback reports "cancelled" instead of a phantom success.
      if (cancelledRef) cancelledRef.current = true;
    } catch {
      // A failed cancel POST leaves the restore running; the button stays
      // available to retry. The watch remains the source of truth for the outcome.
    } finally {
      setCancelling(false);
    }
  }

  return (
    <>
      {/* NO bespoke red hover (whole-app sweep). This carried
          `hover:bg-statusFailBg hover:text-statusFail` — a colourless button
          at rest that flashed red under the cursor. That is the SAME
          treatment already removed from RestorePanel's delete badge,
          Config's snapshot rows, Files' per-snapshot delete and Flash's and
          VMs' own row actions (each of those call sites names it verbatim as
          the defect: "a plain text button whose only colour was a bespoke
          hover:bg-statusFailBg hover:text-statusFail red flash"). This was
          the last surviving copy — grepped across web/src to confirm.
            Nothing is lost by dropping it: the destructive weight of
          cancelling a restore is carried by the confirm dialog handle()
          already opens, which itself still uses the hard/light tone split
          (fail for an in-place restore, warn for a restore-to-a-folder) —
          that dialog tone is a STATUS surface and stays exactly as it was.
          Only this trigger's own bespoke hover colour goes. */}
      <button
        type="button"
        onClick={() => void handle()}
        disabled={cancelling}
        className="self-start inline-flex items-center rounded-control px-2.5 py-1 text-xs font-medium text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
      >
        {cancelling ? t("restore.cancelling") : t("restore.cancel")}
      </button>
      {confirmDialog}
    </>
  );
}
