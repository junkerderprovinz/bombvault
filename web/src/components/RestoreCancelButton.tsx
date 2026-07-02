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

import { useState } from "react";
import { cancelRestore } from "../lib/api";
import type { useT } from "../lib/i18n";

type T = ReturnType<typeof useT>["t"];

export function RestoreCancelButton({
  cancelKey,
  inPlace,
  name,
  t,
}: {
  /** The exact progress key the backend registered this restore under. */
  cancelKey: string;
  /** True for a destructive in-place restore (hard warning); false for a
   *  restore-to-a-folder (light warning). */
  inPlace: boolean;
  /** Human name substituted into the in-place warning ({name}). */
  name: string;
  t: T;
}) {
  const [cancelling, setCancelling] = useState(false);

  async function handle() {
    const msg = inPlace
      ? t("restore.cancelConfirmInPlace").replace(/\{name\}/g, name)
      : t("restore.cancelConfirmSafe");
    if (!window.confirm(msg)) return;
    setCancelling(true);
    try {
      await cancelRestore(cancelKey);
    } catch {
      // A failed cancel POST leaves the restore running; the button stays
      // available to retry. The watch remains the source of truth for the outcome.
    } finally {
      setCancelling(false);
    }
  }

  return (
    <button
      type="button"
      onClick={() => void handle()}
      disabled={cancelling}
      className="self-start inline-flex items-center rounded-lg border border-carbon-border px-2.5 py-1 text-xs font-medium text-carbon-textSub hover:bg-[#3a1c1c] hover:text-[#ff8389] transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
    >
      {cancelling ? t("restore.cancelling") : t("restore.cancel")}
    </button>
  );
}
