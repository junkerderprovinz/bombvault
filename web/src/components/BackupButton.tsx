import { useCallback, useEffect, useRef, useState } from "react";
import { backupNow } from "../lib/api";
import { useBackupWatch } from "../lib/backupWatch";
import { useConfirm } from "../lib/useConfirm";
import { busyPhraseKey } from "../lib/progress";
import type { useT } from "../lib/i18n";
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
  running?: { active: boolean; phase?: string };
}

// Per-browser acknowledgement of the stop warning. Storage keys keep the bv-
// prefix, since renaming one resets every user's stored state.
const STOP_ACK_KEY = "bv-container-stop-ack";

export function BackupButton({ name, t, onBackedUp, running }: BackupButtonProps) {
  // The server runs the backup detached and answers at once; the outcome comes
  // from the progress stream and the recorded run. Awaiting the backup itself
  // would break when the container is the proxy this UI runs through.
  const { state, fire, isPending } = useBackupWatch({
    progressKey: `container:${name}`,
    start: () => backupNow(name),
    matchRun: (r) => r.domain === "container" && r.target === name,
    onDone: onBackedUp,
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

  // A container backup stops the container for the whole run, minutes on a
  // first full backup. Only the first backup in this browser warns: a warning on
  // every press gets clicked away unread, and it is about how backups work, not
  // about one container.
  const confirmStopThenFire = useCallback(async () => {
    let acked = false;
    try {
      acked = localStorage.getItem(STOP_ACK_KEY) === "1";
    } catch {
      // Without storage (private window, blocked site data) it asks every time.
    }
    if (!acked) {
      const ok = await confirm(t("containers.stopWarning"), { confirmKey: "containers.backupNow" });
      if (!ok) return;
      try {
        localStorage.setItem(STOP_ACK_KEY, "1");
      } catch {
        /* asks again next time */
      }
    }
    await fire();
  }, [confirm, fire, t]);

  // The label stays fixed and busy states go into the tooltip; a changing
  // label would resize the button mid-action.
  const stateTip = isPending
    ? t("common.backingUp")
    : blockedByOther
      ? t(busyPhraseKey(running?.phase))
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
