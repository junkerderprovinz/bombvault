// ---------------------------------------------------------------------------
// RestoreAction — the shared in-place restore control.
//
// Containers, VMs, and Recovery each hand-rolled the SAME in-place restore
// mechanics: a useBackupWatch(kind:'restore') fire-and-watch cycle, an optional
// confirm gate, an optional "leave stopped" toggle, the accent restore button
// (spinner while pending + a busy hint when another op blocks it), and the
// <RestoreProgress> banner underneath. This owns that one control so the three
// call sites stop diverging.
//
// It is deliberately delete-agnostic and list-agnostic: the caller owns the row
// chrome (snapshot id / time / tags), the snapshot list + Source toggle, and the
// delete button (delete uses the PLURAL-domain deleteSnapshot — never crossed
// here; this watch's matchRun stays SINGULAR "container"/"vm" or it never
// resolves). RestoreAction only drives the one restore.
//
// cancelledRef is load-bearing: this component owns the single ref instance,
// hands it to useBackupWatch (whose no-run fallback reads it to report a neutral
// "cancelled" instead of a phantom green "success"), and forwards the SAME
// instance to RestoreProgress → RestoreCancelButton (which sets it true on a
// successful cancel). They must all share one ref.
// ---------------------------------------------------------------------------

import { useRef, useState } from "react";
import { restore, restoreVM } from "../../lib/api";
import type { useT } from "../../lib/i18n";
import { useBackupWatch } from "../../lib/backupWatch";
import { useProgress, busyPhraseKey } from "../../lib/progress";
import { RestoreProgress } from "./RestoreProgress";
import type { RepoSource } from "../SourceToggle";

type T = ReturnType<typeof useT>["t"];

interface RestoreActionProps {
  /** Restore domain — drives the progressKey `${domain}:${name}`, the matchRun
   *  domain string (SINGULAR: a plural typo makes the watch never resolve), and
   *  the choice of restore() vs restoreVM(). */
  domain: "container" | "vm";
  /** Target container/VM name. */
  name: string;
  /** Snapshot to restore — a snapshot id or the literal "latest". */
  snapshotId: string;
  /** Repo to restore from; undefined => the backend-default repo (Recovery). */
  source?: RepoSource;
  /** "Something else is running" signal (anyActive) — busy-guards this restore.
   *  Recovery wraps its plain boolean as { active }. */
  otherActive: { active: boolean; phase?: string };
  /** Sticky success-banner text (already localized by the caller). */
  successMessage: string;
  /** Gate the restore behind an explicit confirm checkbox. Default true;
   *  Recovery passes false (its own stepper gates the whole flow). */
  requireConfirm?: boolean;
  /** Offer the "recreate but leave stopped" checkbox. Default true;
   *  Recovery passes false. */
  showLeaveStopped?: boolean;
  /** Force leaveStopped on regardless of the checkbox — Recovery restores every
   *  target left-stopped, then you start them from the tabs. Default false. */
  forceLeaveStopped?: boolean;
  /** Show the "another op is running" phrase beside a blocked button. Default
   *  true; Recovery passes false. */
  showBusyHint?: boolean;
  /** Forwarded to RestoreProgress — gates the started / bgHint lines. Default
   *  true; Recovery passes false. */
  showStartedHint?: boolean;
  /** Button label when idle. Default t("snapshots.restore"). */
  label?: string;
  t: T;
}

export function RestoreAction({
  domain,
  name,
  snapshotId,
  source,
  otherActive,
  successMessage,
  requireConfirm = true,
  showLeaveStopped = true,
  forceLeaveStopped = false,
  showBusyHint = true,
  showStartedHint = true,
  label,
  t,
}: RestoreActionProps) {
  const [confirmed, setConfirmed] = useState(false);
  // leaveStopped overrides the captured run-state so an in-place restore
  // recreates the target without starting it (rebuild a stack member by member).
  const [leaveStopped, setLeaveStopped] = useState(false);

  const progressKey = `${domain}:${name}`;
  // The SAME ref instance flows to useBackupWatch AND (via RestoreProgress) to
  // RestoreCancelButton — see the header note. Never split it.
  const cancelledRef = useRef(false);
  const { state, fire, isPending } = useBackupWatch({
    progressKey,
    kind: "restore",
    matchRun: (r) => r.domain === domain && r.target === name,
    cancelledRef,
    start: () =>
      domain === "container"
        ? restore(name, snapshotId, true, source, forceLeaveStopped || leaveStopped)
        : restoreVM(name, snapshotId, true, source, forceLeaveStopped || leaveStopped),
  });
  const prog = useProgress()[progressKey];
  // Busy-guard: block a new restore while any OTHER backup/restore/replication
  // runs (this item's own in-flight op is covered by isPending, never blocked).
  const blockedByOther = otherActive.active && !isPending;
  const done = state.phase === "success";

  function handleRestore() {
    if (requireConfirm && !confirmed) return;
    void fire();
  }

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-3 flex-wrap">
        {requireConfirm && (
          <label className="flex items-center gap-1.5 text-xs text-carbon-textSub cursor-pointer shrink-0">
            <input
              type="checkbox"
              checked={confirmed}
              onChange={(e) => setConfirmed(e.target.checked)}
              disabled={isPending || done}
              className="rounded border-carbon-border bg-carbon-surface2 focus:ring-offset-0"
              style={{ accentColor: "var(--accent)" }}
            />
            {t("restore.confirm")}
          </label>
        )}
        <button
          onClick={handleRestore}
          disabled={(requireConfirm && !confirmed) || isPending || blockedByOther || done}
          className="inline-flex items-center gap-1.5 rounded-lg bg-accent px-2.5 py-1 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-40 disabled:cursor-not-allowed shrink-0"
        >
          {isPending ? (
            <>
              <span
                className="h-2.5 w-2.5 rounded-full border-2 border-t-transparent animate-spin inline-block"
                style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
              />
              {t("common.restoring")}
            </>
          ) : (
            label ?? t("snapshots.restore")
          )}
        </button>
        {showBusyHint && blockedByOther && (
          <span className="text-[11px] text-carbon-textMuted shrink-0">{t(busyPhraseKey(otherActive.phase))}</span>
        )}
      </div>
      {/* Leave stopped: recreate/restore but don't start (rebuild a stack in order). */}
      {showLeaveStopped && (
        <label className="flex items-center gap-1.5 text-[11px] text-carbon-textSub cursor-pointer">
          <input
            type="checkbox"
            checked={leaveStopped}
            onChange={(e) => setLeaveStopped(e.target.checked)}
            disabled={isPending || done}
            className="rounded border-carbon-border bg-carbon-surface2 focus:ring-offset-0"
            style={{ accentColor: "var(--accent)" }}
          />
          {t("restore.leaveStopped")}
        </label>
      )}
      <RestoreProgress
        state={state}
        isPending={isPending}
        prog={prog}
        cancelKey={progressKey}
        inPlace={true}
        name={name}
        cancelledRef={cancelledRef}
        successMessage={successMessage}
        showStartedHint={showStartedHint}
        t={t}
      />
    </div>
  );
}
