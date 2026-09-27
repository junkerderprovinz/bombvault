// RestoreProgress is the inline banner shared by every restore control: the
// started hint, progress bar and cancel button while a restore runs, then a
// success, cancelled or error line. It only displays; the caller owns the
// useBackupWatch cycle and passes the same cancelledRef that useBackupWatch
// received, so a cancel reports "cancelled" instead of success.
//
// The outcome lines stay inline instead of becoming toasts: a restore result
// has to stay visible, an error is often raw backend output worth copying, and
// a success message can carry the restored-to path.

import type { MutableRefObject, ReactNode } from "react";
import type { BackupWatchState } from "../../lib/backupWatch";
import { humanBytes } from "../../lib/forecast";
import type { ProgressState } from "../../lib/progress";
import type { useT } from "../../lib/i18n";
import { RunReasonText } from "../../lib/runReason";
import { ProgressBar } from "../ProgressBar";
import { RestoreCancelButton } from "../RestoreCancelButton";

type T = ReturnType<typeof useT>["t"];

// restoreProgressCaption is the "Restoring… NN%" caption, or undefined (an
// indeterminate bar) until the first percentage arrives, as on the backup bars.
// Saving and importing a dump have no percentage, only the bytes moved so far.
function restoreProgressCaption(t: T, prog: ProgressState | undefined, name: string): string | undefined {
  if (!prog || prog.phase !== "restore") return undefined;
  if ((prog.stage === "dbdumpsave" || prog.stage === "dbimport") && prog.bytes) {
    const key = prog.stage === "dbdumpsave" ? "activityLog.lineSavingDumpItem" : "activityLog.lineImportingItem";
    return t(key).replace("{name}", name).replace("{bytes}", humanBytes(prog.bytes));
  }
  if (prog.percent <= 0) return undefined;
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
  /** Passed to the cancel button where the standing warning does not fit. */
  cancelConfirm?: string;
  /** False where the backend offers no cancel, so no button promises one.
   *  Default true. */
  cancellable?: boolean;
  /** Localized success text. */
  successMessage: ReactNode;
  /** Shows the success text in the warning tone. */
  successWarn?: boolean;
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
  cancelConfirm,
  cancellable = true,
  successMessage,
  successWarn = false,
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
            <ProgressBar percent={prog.percent} active inline label={restoreProgressCaption(t, prog, name)} />
          )}
          {cancellable && (
            <RestoreCancelButton
              cancelKey={cancelKey}
              inPlace={inPlace}
              name={name}
              confirmText={cancelConfirm}
              t={t}
              cancelledRef={cancelledRef}
            />
          )}
        </div>
      )}
      {state.phase === "success" && (
        <p className={`text-xs wrap-break-word ${successWarn ? "text-statusWarn" : "text-statusOk"}`}>{successMessage}</p>
      )}
      {state.phase === "cancelled" && (
        <p className="text-xs text-carbon-textSub wrap-break-word">{t("restore.cancelled")}</p>
      )}
      {state.phase === "error" && (
        <p className="text-xs text-statusFail wrap-break-word">
          <RunReasonText reason={state.message} t={t} />
        </p>
      )}
    </>
  );
}
