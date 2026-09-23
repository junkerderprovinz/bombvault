// RestoreAction is the in-place restore control shared by Containers, VMs and
// Recovery: the confirm gate, the leave-stopped toggle, the restore trigger and
// the progress banner under it. The caller owns the row, the snapshot list and
// the delete button.
//
// cancelledRef goes to useBackupWatch, whose no-run fallback reads it to report
// "cancelled" instead of success, and through RestoreProgress to
// RestoreCancelButton, which sets it on a successful cancel. Both must see the
// same ref.

import { useRef, useState, type ReactNode } from "react";
import { restore, restoreVM } from "../../lib/api";
import type { useT } from "../../lib/i18n";
import { useBackupWatch } from "../../lib/backupWatch";
import { useProgress, busyPhraseKey } from "../../lib/progress";
import { SNAPSHOT_MISSING } from "../../lib/timeline";
import { RestoreProgress } from "./RestoreProgress";
import type { RepoSource } from "../SourceToggle";
import { Button } from "../Button";
import { IconRestore } from "../Sidebar";
import { useConfirm } from "../../lib/useConfirm";
import { Toggle } from "../Toggle";

type T = ReturnType<typeof useT>["t"];

interface RestoreActionProps {
  /** Picks restore() or restoreVM() and keys the progress entry and run match.
   *  Singular, like the run domains; a plural never matches a run. */
  domain: "container" | "vm";
  /** Identifier for the restore call and the run match. For VMs this is the
   *  raw libvirt name (VM.libvirtName), not the display name. */
  name: string;
  /** Name shown in the in-place cancel warning. Defaults to name. */
  displayName?: string;
  /** A snapshot id or "latest". */
  snapshotId: string;
  /** Repo to restore from; undefined uses the backend default. */
  source?: RepoSource;
  /** Whether another operation is running, which blocks this restore. */
  otherActive: { active: boolean; phase?: string };
  /** Localized success text. */
  successMessage: string;
  /** Gates the restore behind a confirm toggle. Default true. Row actions
   *  have no room for the toggle and pass confirmMessage instead. */
  requireConfirm?: boolean;
  /** Localized question asked in a modal before the restore starts. */
  confirmMessage?: string;
  /** Offers the "leave stopped" toggle. Default true. */
  showLeaveStopped?: boolean;
  /** Recreates the target stopped regardless of the toggle, so Recovery can
   *  start targets in order afterwards. Default false. */
  forceLeaveStopped?: boolean;
  /** Names the operation that blocks the trigger. Default true. */
  showBusyHint?: boolean;
  /** Passed to RestoreProgress. Default true. */
  showStartedHint?: boolean;
  /** Tooltip and accessible name of the icon badge trigger. Defaults to
   *  t("snapshots.restore"). */
  label?: string;
  /** Renders the trigger as a square icon badge at the row's far edge instead
   *  of a text button, for per-item list rows. The badge takes its hue from
   *  the row's .glim-hue, so it gets no hueIndex. */
  iconBadge?: boolean;
  /** Content placed before the trigger in the same row, so a list row can put
   *  its name and time on the badge's line. */
  leading?: ReactNode;
  /** Called when the place answered snapshot-missing, so the timeline can offer the next one. */
  onMissing?: () => void;
  t: T;
}

export function RestoreAction({
  domain,
  name,
  displayName,
  snapshotId,
  source,
  otherActive,
  successMessage,
  requireConfirm = true,
  confirmMessage,
  showLeaveStopped = true,
  forceLeaveStopped = false,
  showBusyHint = true,
  showStartedHint = true,
  label,
  iconBadge = false,
  leading,
  onMissing,
  t,
}: RestoreActionProps) {
  const [confirmed, setConfirmed] = useState(false);
  const { confirm, confirmDialog } = useConfirm();
  // leaveStopped overrides the captured run-state so an in-place restore
  // recreates the target without starting it (rebuild a stack member by member).
  const [leaveStopped, setLeaveStopped] = useState(false);

  const progressKey = `${domain}:${name}`;
  const cancelledRef = useRef(false);
  const { state, fire, isPending } = useBackupWatch({
    progressKey,
    kind: "restore",
    matchRun: (r) => r.domain === domain && r.target === name,
    cancelledRef,
    start: async () => {
      const res = await (domain === "container"
        ? restore(name, snapshotId, true, source, forceLeaveStopped || leaveStopped)
        : restoreVM(name, snapshotId, true, source, forceLeaveStopped || leaveStopped));
      if (res.code === SNAPSHOT_MISSING) onMissing?.();
      return res;
    },
  });
  const prog = useProgress()[progressKey];
  // otherActive also counts this target's own restore, which isPending covers.
  const blockedByOther = otherActive.active && !isPending;
  const done = state.phase === "success";

  async function handleRestore() {
    if (requireConfirm && !confirmed) return;
    if (confirmMessage && !(await confirm(confirmMessage))) return;
    void fire();
  }

  // Both trigger shapes share this and handleRestore, so a row action and a
  // form submit cannot disagree on whether a restore may run.
  const triggerDisabled = (requireConfirm && !confirmed) || isPending || blockedByOther || done;
  // The icon badge is the row's last child, pushed to the far edge by ms-auto,
  // so the busy phrase goes before it.
  const busyHint =
    showBusyHint && blockedByOther ? (
      <span className="text-caption text-carbon-textMuted shrink-0">{t(busyPhraseKey(otherActive.phase))}</span>
    ) : null;

  const trigger = iconBadge ? (
    <Button
      label={label ?? t("snapshots.restore")}
      labelKey="snapshots.restore"
      glyph={<IconRestore />}
      tone="accent"
      onClick={() => void handleRestore()}
      disabled={triggerDisabled}
      busy={isPending}
      className="ms-auto shrink-0"
    />
  ) : (
    <Button
      label={label ?? t("snapshots.restore")}
      labelKey="snapshots.restore"
      tone="accent"
      onClick={() => void handleRestore()}
      disabled={triggerDisabled}
      busy={isPending}
      title={isPending ? t("common.restoring") : undefined}
      className="shrink-0"
    />
  );

  return (
    <div className="flex flex-col gap-2">
      {confirmDialog}
      <div className="flex items-center gap-3 flex-wrap">
        {leading}
        {requireConfirm && (
          <Toggle
            checked={confirmed}
            onChange={setConfirmed}
            disabled={isPending || done}
            label={t("common.confirm")}
            className="shrink-0"
          />
        )}
        {iconBadge ? (
          <>
            {busyHint}
            {trigger}
          </>
        ) : (
          <>
            {trigger}
            {busyHint}
          </>
        )}
      </div>
      {showLeaveStopped && (
        <Toggle
          checked={leaveStopped}
          onChange={setLeaveStopped}
          disabled={isPending || done}
          label={t("restore.leaveStopped")}
        />
      )}
      <RestoreProgress
        state={state}
        isPending={isPending}
        prog={prog}
        cancelKey={progressKey}
        inPlace={true}
        name={displayName ?? name}
        cancelledRef={cancelledRef}
        successMessage={successMessage}
        showStartedHint={showStartedHint}
        t={t}
      />
    </div>
  );
}
