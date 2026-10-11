import { useRef } from "react";
import { restore } from "../../lib/api";
import { useBackupWatch } from "../../lib/backupWatch";
import type { useT } from "../../lib/i18n";
import { anyActive, busyPhraseKey, useProgress } from "../../lib/progress";
import { useConfirm } from "../../lib/useConfirm";
import { restoreBlockReason, useRestoreCheck } from "../../lib/useRestoreCheck";
import { Button } from "../Button";
import { RestoreCheckPanel } from "./RestoreCheckPanel";
import { RestoreProgress } from "./RestoreProgress";

type T = ReturnType<typeof useT>["t"];

/**
 * RecreateButton recreates a container that is not installed from its saved
 * definition, since a config-only backup has no snapshot to restore. The
 * server turns a restore of "latest" into a recreate.
 */
export function RecreateButton({ name, source, t }: { name: string; source: string; t: T }) {
  const cancelledRef = useRef(false);
  const withoutRuntimeRef = useRef(false);
  const { state, fire, isPending } = useBackupWatch({
    progressKey: `container:${name}`,
    kind: "restore",
    start: () => restore(name, "latest", true, source, false, withoutRuntimeRef.current),
    matchRun: (r) => r.domain === "container" && r.target === name,
    cancelledRef,
  });
  const progressMap = useProgress();
  const prog = progressMap[`container:${name}`];
  const running = anyActive(progressMap);
  const blockedByOther = running.active && !isPending;
  const { confirm, confirmDialog } = useConfirm();
  const check = useRestoreCheck({ kind: "container", name, snapshotId: "latest", source });
  const idle = isPending || state.phase === "success";
  async function handle() {
    if (!(await confirm(t("snapshots.recreateConfirm")))) return;
    withoutRuntimeRef.current = false;
    void fire();
  }
  function recreateWithoutRuntime() {
    withoutRuntimeRef.current = true;
    void fire();
  }
  return (
    <div className="flex flex-col gap-1 py-2">
      {!idle && <RestoreCheckPanel check={check} t={t} />}
      <Button
        label={t("snapshots.recreate")}
        labelKey="snapshots.recreate"
        tone="accent"
        onClick={() => void handle()}
        disabled={isPending || blockedByOther || state.phase === "success" || !check.ready}
        busy={isPending}
        title={isPending ? t("common.restoring") : undefined}
        hint={idle ? undefined : restoreBlockReason(check, t)}
        className="self-start"
      />
      {blockedByOther && (
        <span className="text-caption text-carbon-textMuted">{t(busyPhraseKey(running.phase))}</span>
      )}
      <RestoreProgress
        state={state}
        isPending={isPending}
        prog={prog}
        cancelKey={`container:${name}`}
        inPlace
        name={name}
        cancelledRef={cancelledRef}
        successMessage={t("restore.recreateComplete")}
        onRestoreWithoutRuntime={recreateWithoutRuntime}
        t={t}
      />
      {confirmDialog}
    </div>
  );
}
