import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError, backupEverythingNow } from "../../lib/api";
import type { Run } from "../../lib/api";
import { useBackupWatch } from "../../lib/backupWatch";
import type { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { useIsDesktop } from "../../lib/useMediaQuery";
import { Button } from "../../components/Button";
import { StickyActionBar } from "../../components/mobile/StickyActionBar";
import { IconBackupNow } from "../../components/Sidebar";

// Phone thumb-zone trigger; the surface's one solid-accent control, and the
// owner of the everything pass's fire-and-watch cycle. Mounted at every
// width: unmounting it at 48rem killed the live watch the moment a phone
// rotated to a landscape at or above the breakpoint (iPhone 15 is 852px,
// Pixel 8 is 892px), and rotating back mounted a fresh hook in the idle
// phase: the bar read ready while the pass was still running on the
// server, with no sheet, no progress and no toast. The desktop cost is
// bounded by the child itself: the bar is not rendered above the
// breakpoint, the confirm dialog exists only while a confirm is pending,
// and what remains mounted is one idle
// progress subscription in a leaf component, not a page re-render per SSE
// frame. Phone behavior is unchanged from when the watch lived on the page
// component: same watch args, confirm-first press, deep-link into the run
// sheet via the page's latch, and terminal toasts (at either width, since a
// pass fired on a phone also reports its outcome after the rotation).
export function PhoneEverythingTrigger({
  t,
  onWatchRun,
  onArmFire,
  onWatchStopped,
}: {
  t: ReturnType<typeof useT>["t"];
  /** Called on every watch poll with the correlated run; the page's
   *  sheet-latch logic decides replace/keep/open (see handleWatchRun). */
  onWatchRun: (run: Run) => void;
  /** Called the moment the user confirms a fire; arms the page's deep-link
   *  latch so the correlation of this pass may steal the sheet. */
  onArmFire: () => void;
  /** Called when the fire-and-watch chain ends (the pending phase clearing,
   *  by terminal outcome or timeout). The page releases the watch's display
   *  ownership at that moment (watchOwnedRun): only a live chain's records
   *  are fresher than the polled list. */
  onWatchStopped: () => void;
}) {
  // Thumb-zone trigger watch (BackupButton's semantics verbatim). The
  // everything pass is async on the server, so the press only starts it
  // ({ok:true,started:true} is never read for the outcome) and the watch
  // resolves from the recorded run; baseline ids seeded from listRuns before
  // firing (never a client clock), then polled until the pass's run turns
  // terminal. `progressKey: ""` is the honest key: the everything parent run
  // publishes no SSE entry of its own (its per-domain children do; see
  // RunDetailSheet's progressKeyFor), so the watch resolves via the run-poll
  // belt exactly like a target whose key never appears.
  const startEverything = useCallback(async () => {
    // The everything pass is single-flight server-side: a second press while
    // one is already running surfaces as HTTP 409. The raw HTTP status text
    // must never reach a toast; map it to the shared translated "already
    // running" copy (Settings.tsx's runNow mapping) and return it as the
    // start failure the hook already displays.
    try {
      return await backupEverythingNow();
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        return { ok: false, error: t("settings.everythingAlreadyRunning") };
      }
      throw err;
    }
  }, [t]);
  const { state, fire, isPending } = useBackupWatch({
    progressKey: "",
    start: startEverything,
    matchRun: (r) => r.domain === "everything",
    onRun: onWatchRun,
  });

  // Terminal outcomes toast per BackupButton's contract ("failed action toasts
  // And shakes its button"); success mirrors its snapshot-id form, falling back
  // to plain Done when the parent run carries no snapshot (the everything pass
  // aggregates its domains, so the parent snapshot is often empty; the
  // container-specific configOnly fallback does not apply here). cancelled and
  // skipped stay silent like BackupButton's cancelled arm: the deep-linked
  // sheet is already showing the run's own record.
  const { push } = useToast();
  const [shake, setShake] = useState(0);
  const seenPhase = useRef(state.phase);
  useEffect(() => {
    if (state.phase === seenPhase.current) return;
    const was = seenPhase.current;
    seenPhase.current = state.phase;
    // Leaving "pending" is the chain's end (terminal outcome, timeout, or a
    // reset). The page hears it before the toast arms: ownership and
    // outcome travel together.
    if (was === "pending" && state.phase !== "pending") onWatchStopped();
    if (state.phase === "success") {
      push(
        state.snapshotId ? `${t("common.done")} · ${state.snapshotId.slice(0, 8)}` : t("common.done"),
        "success"
      );
    } else if (state.phase === "error") {
      push(state.message, "fail");
      setShake((n) => n + 1);
    }
  }, [state, push, t, onWatchStopped]);

  // The question stands between the press and the POST; useConfirm answers it
  // in a sheet below the breakpoint and in the card above it. confirmKey makes
  // the commit button name the outcome with the trigger's own words.
  const isDesktop = useIsDesktop();
  const { confirm, confirmDialog } = useConfirm();
  const confirmThenFire = useCallback(async () => {
    const ok = await confirm(t("home.newBackupConfirm"), {
      confirmKey: "settings.everythingTitle",
    });
    if (!ok) return;
    onArmFire();
    await fire();
  }, [confirm, fire, t, onArmFire]);

  // StickyActionBar as the last direct child of the page column; sticky
  // resolves against main#bv-main, so nothing may wrap it. Never a fab, never
  // position:fixed. While the pass runs the button shows its busy spinner and
  // is disabled; a failed start also shakes (the glim-shake key remount,
  // Containers' Save-bar pattern).
  //
  // The bar is not rendered above the breakpoint, rather than hidden by CSS:
  // the desktop tree carries no phone chrome at all, which is the contract
  // Layout.tsx states for the Sidebar and the bar alike. The component itself
  // stays mounted at every width, because that is what keeps the watch alive
  // across a rotation; a pending confirmation also survives the flip, since
  // useConfirm swaps the sheet for the card without dropping the promise.
  return (
    <>
      {!isDesktop && (
        <StickyActionBar>
          <Button
            key={shake}
            label={t("settings.everythingTitle")}
            labelKey="settings.everythingTitle"
            glyph={<IconBackupNow />}
            tone="accent"
            keepLabel
            disabled={isPending}
            busy={isPending}
            onClick={() => void confirmThenFire()}
            className={`w-full min-h-[2.75rem] justify-center${shake ? " glim-shake" : ""}`}
          />
        </StickyActionBar>
      )}
      {confirmDialog}
    </>
  );
}
