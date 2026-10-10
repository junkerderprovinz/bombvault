import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError, backupEverythingNow } from "../../lib/api";
import type { Run } from "../../lib/api";
import { useBackupWatch } from "../../lib/backupWatch";
import type { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { useIsDesktop } from "../../lib/useMediaQuery";
import { PageAction } from "../../components/PageActions";
import { IconBackupNow, IconSync } from "../../components/navGlyphs";

/**
 * EverythingAction is the overview's main action, "Back up everything now",
 * and the owner of that pass's watch: it starts the pass, follows its run and
 * reports how it ended. While any backup runs it turns into the way to that
 * run instead.
 *
 * It stays mounted whatever the page shows, because the watch dies with it: a
 * pass started before a phone was turned to landscape would otherwise finish
 * without a word.
 */
export function EverythingAction({
  t,
  running,
  hidden = false,
  onWatchRun,
  onArmFire,
  onWatchStopped,
  onOpenRun,
}: {
  t: ReturnType<typeof useT>["t"];
  /** A backup or an off-site copy is under way, whoever started it. */
  running: boolean;
  /** Shows no button while the page is being arranged. The watch goes on. */
  hidden?: boolean;
  /** Called on every watch poll with the pass's run. */
  onWatchRun: (run: Run) => void;
  /** Called the moment the person starts a pass, before its run is known. */
  onArmFire: () => void;
  /** Called when the watch ends, by an outcome or by its timeout. */
  onWatchStopped: () => void;
  /** Opens the run that is under way. */
  onOpenRun: () => void;
}) {
  // The server runs one pass at a time and answers a second start with 409.
  // That status must not reach a toast as raw HTTP text.
  const startEverything = useCallback(async () => {
    try {
      return await backupEverythingNow();
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        return { ok: false, error: t("settings.everythingAlreadyRunning") };
      }
      throw err;
    }
  }, [t]);
  // The pass's parent run publishes no progress entry of its own, only its
  // children do, so the watch has no key and resolves from the recorded run.
  const { state, fire, isPending } = useBackupWatch({
    progressKey: "",
    start: startEverything,
    matchRun: (r) => r.domain === "everything",
    onRun: onWatchRun,
  });

  const { push } = useToast();
  const [shake, setShake] = useState(0);
  const seenPhase = useRef(state.phase);
  useEffect(() => {
    if (state.phase === seenPhase.current) return;
    const was = seenPhase.current;
    seenPhase.current = state.phase;
    // The page hears the end of the watch before the toast, so the run sheet
    // and the toast change together.
    if (was === "pending" && state.phase !== "pending") onWatchStopped();
    if (state.phase === "success") {
      push(state.snapshotId ? `${t("common.done")} · ${state.snapshotId.slice(0, 8)}` : t("common.done"), "success");
    } else if (state.phase === "error") {
      push(state.message, "fail");
      setShake((n) => n + 1);
    }
  }, [state, push, t, onWatchStopped]);

  const isDesktop = useIsDesktop();
  const { confirm, confirmDialog } = useConfirm();
  const start = useCallback(async () => {
    // On a phone the corner is one tap from the thumb.
    if (!isDesktop && !(await confirm(t("home.newBackupConfirm"), { confirmKey: "settings.everythingTitle" }))) return;
    onArmFire();
    await fire();
  }, [isDesktop, confirm, fire, t, onArmFire]);

  return (
    <>
      {hidden ? null : isPending || running ? (
        <PageAction label={t("overview.head.backingUp")} glyph={<IconSync />} onClick={onOpenRun} busy />
      ) : (
        // The key remounts the button after a failed start, which replays
        // its shake.
        <span key={shake} className={`flex${shake ? " glim-shake" : ""}`}>
          <PageAction primary label={t("backups.backUpEverything")} glyph={<IconBackupNow />} onClick={() => void start()} />
        </span>
      )}
      {confirmDialog}
    </>
  );
}
