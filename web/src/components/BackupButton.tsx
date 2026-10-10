import { useCallback, useEffect, useRef, useState } from "react";
import { backupNow, type Run } from "../lib/api";
import { useBackupWatch } from "../lib/backupWatch";
import { useConfirm } from "../lib/useConfirm";
import { busyPhraseKey, type ProgressStage, type ProgressState } from "../lib/progress";
import { humanBytes } from "../lib/forecast";
import type { TranslationKey, useT } from "../lib/i18n";
import { Button } from "./Button";
import { groupStage } from "../lib/controls";
import { IconBackupNow } from "./Sidebar";
import { useToast } from "../lib/toast";

type T = ReturnType<typeof useT>["t"];

interface BackupButtonProps {
  name: string;
  t: T;
  /** Called after a successful backup so the caller can refresh (e.g. last-backup time). */
  onBackedUp?: () => void;
  /** Another operation is running (anyActive). Disables the button with a
   *  hint, except while this button's own backup is the one running. */
  running?: { active: boolean; phase?: string; stage?: ProgressStage };
  /** This container's own live progress, for the step the busy hint names. */
  progress?: ProgressState;
  /** The correlated run, reported by useBackupWatch the moment the
   *  baseline-id match identifies it (and on each later poll, with the
   *  refreshed record). The mobile container detail deep-links the
   *  RunDetailSheet into the live run at that moment; desktop callers omit
   *  it (BackupButton.dom.test.tsx pins that). */
  onRunCorrelated?: (run: Run) => void;
}

// Per-browser acknowledgement of the stop warning. Storage keys keep the bv-
// prefix, since renaming one resets every user's stored state.
export const STOP_ACK_KEY = "bv-container-stop-ack";

/**
 * stopAcknowledged asks whether a container may be stopped for its backup,
 * which takes minutes on a first full one. It asks once per browser: a warning
 * on every press gets clicked away unread, and it is about how backups work,
 * not about one container.
 */
export async function stopAcknowledged(
  confirm: (message: string, options: { confirmKey: TranslationKey }) => Promise<boolean>,
  t: T
): Promise<boolean> {
  try {
    if (localStorage.getItem(STOP_ACK_KEY) === "1") return true;
  } catch {
    // Without storage it asks every time.
  }
  if (!(await confirm(t("containers.stopWarning"), { confirmKey: "containers.backupNow" }))) return false;
  try {
    localStorage.setItem(STOP_ACK_KEY, "1");
  } catch {
    // It asks again next time.
  }
  return true;
}

export function BackupButton({ name, t, onBackedUp, running, progress, onRunCorrelated }: BackupButtonProps) {
  // The server runs the backup detached and answers at once; the outcome comes
  // from the progress stream and the recorded run. Awaiting the backup itself
  // would break when the container is the proxy this UI runs through.
  const { state, fire, isPending } = useBackupWatch({
    progressKey: `container:${name}`,
    start: () => backupNow(name),
    matchRun: (r) => r.domain === "container" && r.target === name,
    onDone: onBackedUp,
    onRun: onRunCorrelated,
  });
  const blockedByOther = !!running?.active && !isPending;
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  // A failure toasts and shakes the button. The count doubles as the button's
  // key, so each failure remounts it and replays the animation.
  const [shake, setShake] = useState(0);
  // The last phase already toasted. useBackupWatch always starts at "idle",
  // so mounting toasts nothing.
  const seenPhase = useRef(state.phase);

  useEffect(() => {
    if (state.phase === seenPhase.current) return;
    seenPhase.current = state.phase;
    if (state.phase === "success") {
      // No snapshot id: a stateless container without data folders, so only
      // its definition was saved.
      push(
        state.snapshotId ? `${t("common.done")} · ${state.snapshotId.slice(0, 8)}` : t("backup.configOnly"),
        "success"
      );
    } else if (state.phase === "error") {
      push(state.message, "fail");
      setShake((n) => n + 1);
    } else if (state.phase === "skipped") {
      // The container is gone, so the backup was skipped rather than failed.
      push(`↷ ${t("containers.notInstalledTitle")}`, "warn");
    }
  }, [state, push, t]);

  const confirmStopThenFire = useCallback(async () => {
    if (await stopAcknowledged(confirm, t)) await fire();
  }, [confirm, fire, t]);

  // The label stays fixed and busy states go into the tooltip; a changing
  // label would resize the button mid-action. A database dump reports bytes
  // and no percentage, so its count is the only sign it is moving.
  const stateTip = isPending
    ? progress?.stage === "dbdump"
      ? t("dbdump.progress").replace("{bytes}", humanBytes(progress.bytes ?? 0))
      : t("common.backingUp")
    : blockedByOther
      ? t(busyPhraseKey(running?.phase, running?.stage))
      : undefined;

  return (
    <>
      {confirmDialog}
      <Button
        key={shake}
        label={t("containers.backupNow")}
        labelKey="containers.backupNow"
        glyph={<IconBackupNow />}
        tone="accent"
        // Same width as the Export button beside it: both derive it from the
        // same two labels, so they match in every language.
        stage={groupStage([t("containers.backupNow"), t("export.button")])}
        onClick={() => void confirmStopThenFire()}
        disabled={isPending || blockedByOther}
        busy={isPending}
        title={stateTip}
        className={shake ? "glim-shake" : ""}
      />
    </>
  );
}
