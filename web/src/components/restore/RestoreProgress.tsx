// RestoreProgress is the inline banner shared by every restore control: the
// started hint, progress bar and cancel button while a restore runs, then a
// success, cancelled or error line. It only displays; the caller owns the
// useBackupWatch cycle and passes the same cancelledRef that useBackupWatch
// received, so a cancel reports "cancelled" instead of success.
//
// The outcome lines stay inline instead of becoming toasts: a restore result
// has to stay visible, an error is often raw backend output worth copying, and
// a success message can carry the restored-to path.

import type { MutableRefObject } from "react";
import type { BackupWatchState } from "../../lib/backupWatch";
import type { ProgressState } from "../../lib/progress";
import type { useT } from "../../lib/i18n";
import { ProgressBar } from "../ProgressBar";
import { RestoreCancelButton } from "../RestoreCancelButton";

type T = ReturnType<typeof useT>["t"];

// restoreProgressCaption is the "Restoring… NN%" caption, or undefined (an
// indeterminate bar) until the first percentage arrives, as on the backup bars.
function restoreProgressCaption(t: T, prog: ProgressState | undefined): string | undefined {
  if (!prog || prog.phase !== "restore" || prog.percent <= 0) return undefined;
  return t("restore.progress").replace("{pct}", String(Math.round(prog.percent)));
}

interface RestoreProgressProps {
  /** State of the caller's useBackupWatch; picks the outcome line. */
  state: BackupWatchState;
  isPending: boolean;
  /** This target's live progress entry, if any. */
  prog: ProgressState | undefined;
  /** The progress key the backend registered this restore under. */
  cancelKey: string;
  /** True for a destructive in-place restore (hard cancel warning); false for a
   *  restore to a folder (light warning). */
  inPlace: boolean;
  /** Name shown in the in-place cancel warning. */
  name: string;
  /** The ref the caller's useBackupWatch received, passed on to
   *  RestoreCancelButton. */
  cancelledRef?: MutableRefObject<boolean>;
  /** Localized success text. */
  successMessage: string;
  /** Shows the restore.started and restore.bgHint lines. Default true. */
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
              <p className="text-caption text-carbon-textMuted">{t("restore.bgHint")}</p>
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
        <p className="text-xs text-statusOk wrap-break-word">{successMessage}</p>
      )}
      {state.phase === "cancelled" && (
        <p className="text-xs text-carbon-textSub wrap-break-word">{t("restore.cancelled")}</p>
      )}
      {state.phase === "error" && (
        <p className="text-xs text-statusFail wrap-break-word">{state.message}</p>
      )}
    </>
  );
}
