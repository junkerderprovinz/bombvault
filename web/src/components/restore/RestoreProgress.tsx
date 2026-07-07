// ---------------------------------------------------------------------------
// RestoreProgress — the shared inline restore banner primitive.
//
// Every restore control (containers, VMs, recovery, recreate, file/to-folder)
// rendered the SAME "restore in flight → started hint + progress bar + cancel"
// block followed by success / cancelled / error banners. This extracts that one
// block so the six call sites stop hand-copying it.
//
// It is display-only: the caller owns the useBackupWatch cycle and passes in the
// derived `state` / `isPending` / `prog`. Crucially the caller MUST forward the
// SAME `cancelledRef` instance its useBackupWatch received — RestoreCancelButton
// sets `cancelledRef.current = true` on a successful cancel and the watch's
// no-run fallback reads that exact ref to report a neutral "cancelled" instead
// of a phantom green success.
// ---------------------------------------------------------------------------

import type { MutableRefObject } from "react";
import type { BackupWatchState } from "../../lib/backupWatch";
import type { ProgressState } from "../../lib/progress";
import type { useT } from "../../lib/i18n";
import { ProgressBar } from "../ProgressBar";
import { RestoreCancelButton } from "../RestoreCancelButton";

type T = ReturnType<typeof useT>["t"];

// restoreProgressCaption builds the "Restoring… NN%" caption for the inline
// ProgressBar from a progress-store entry. Indeterminate (undefined) until the
// first real percent arrives, mirroring the backup bars.
function restoreProgressCaption(t: T, prog: ProgressState | undefined): string | undefined {
  if (!prog || prog.phase !== "restore" || prog.percent <= 0) return undefined;
  return t("restore.progress").replace("{pct}", String(Math.round(prog.percent)));
}

interface RestoreProgressProps {
  /** The paired useBackupWatch state — drives the success/cancelled/error banner. */
  state: BackupWatchState;
  /** True while the fire-and-watch is in flight (state.phase === "pending"). */
  isPending: boolean;
  /** This target's live SSE progress entry (progressKey-indexed), if any. */
  prog: ProgressState | undefined;
  /** The exact progress key the backend registered this restore under. */
  cancelKey: string;
  /** True for a destructive in-place restore (hard cancel warning); false for a
   *  non-destructive restore-to-a-folder (light warning). */
  inPlace: boolean;
  /** Human name substituted into the in-place cancel warning. */
  name: string;
  /** The SAME cancelled-flag ref the paired useBackupWatch received — forwarded
   *  to RestoreCancelButton so a no-run cancel resolves neutral, not green. */
  cancelledRef?: MutableRefObject<boolean>;
  /** Sticky success-banner text (already localized by the caller). */
  successMessage: string;
  /** Gates the two restore.started / restore.bgHint lines. Default true;
   *  Recovery passes false (its stepper omits them). */
  showStartedHint?: boolean;
  t: T;
}

export function RestoreProgress({
  state,
  isPending,
  prog,
  cancelKey,
  inPlace,
  name,
  cancelledRef,
  successMessage,
  showStartedHint = true,
  t,
}: RestoreProgressProps) {
  return (
    <>
      {isPending && (
        <div className="flex flex-col gap-1">
          {showStartedHint && (
            <>
              <p className="text-xs text-carbon-textSub">{t("restore.started")}</p>
              <p className="text-[11px] text-carbon-textMuted">{t("restore.bgHint")}</p>
            </>
          )}
          {prog?.phase === "restore" && prog.active && (
            <ProgressBar percent={prog.percent} active inline label={restoreProgressCaption(t, prog)} />
          )}
          <RestoreCancelButton
            cancelKey={cancelKey}
            inPlace={inPlace}
            name={name}
            t={t}
            cancelledRef={cancelledRef}
          />
        </div>
      )}
      {state.phase === "success" && (
        <p className="text-xs text-[#6fdc8c] break-words">{successMessage}</p>
      )}
      {state.phase === "cancelled" && (
        <p className="text-xs text-carbon-textSub break-words">{t("restore.cancelled")}</p>
      )}
      {state.phase === "error" && (
        <p className="text-xs text-[#ff8389] break-words">{state.message}</p>
      )}
    </>
  );
}
