import { useState } from "react";
import { deleteBackupsVM } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { RestoreAction } from "../restore/RestoreAction";
import { RecentRunsList } from "../RecentRunsList";
import { SizeBreakdown } from "../SizeBreakdown";
import { IconRestore } from "../Sidebar";
import { Button } from "../Button";
import { useProgress, anyActive } from "../../lib/progress";
import { useConfirm } from "../../lib/useConfirm";
import { useToast } from "../../lib/toast";
import { Timeline, type TimelinePick } from "../timeline/Timeline";
import { useOpenAnomalies } from "../../lib/useAnomalies";

type T = ReturnType<typeof useT>["t"];

function VMSnapshotActions({
  pick,
  vmName,
  vmDisplayName,
  preselected,
  t,
}: {
  pick: TimelinePick;
  /** Raw libvirt name: drives the progress key and the restore action. */
  vmName: string;
  vmDisplayName?: string;
  /** A finding's restore link asked for this backup, so its restore starts open. */
  preselected: boolean;
  t: T;
}) {
  const running = anyActive(useProgress());
  const [showRestore, setShowRestore] = useState(preselected);
  return (
    <>
      {pick.mark.tags.length > 0 && (
        <span className="text-carbon-textMuted text-xs hidden sm:block">{pick.mark.tags.join(", ")}</span>
      )}
      <Button
        label={t("restore.open")}
        labelKey="restore.open"
        glyph={<IconRestore />}
        tone={pick.lead ? "accent" : "neutral"}
        onClick={() => setShowRestore((p) => !p)}
        className="shrink-0"
      />
      {showRestore && (
        <div className="basis-full ps-24">
          <RestoreAction
            domain="vm"
            name={vmName}
            displayName={vmDisplayName}
            snapshotId={pick.snapshotId}
            source={pick.source}
            otherActive={running}
            successMessage={t("restore.completeVM")}
            onMissing={pick.onMissing}
            t={t}
          />
        </div>
      )}
    </>
  );
}

export function VMRestorePanel({
  name,
  displayName,
  t,
  open,
  preselect = "",
  preselectAt = 0,
}: {
  /** Raw libvirt name. Every call in this panel uses it, never displayName. */
  name: string;
  /** Display name shown in the restore cancel-confirm text; falls back to
   *  name. */
  displayName?: string;
  t: T;
  /** Owned by VMRow's `openSections`, as components/RestorePanel.tsx takes
   *  `open` from ContainerRow, so both cards share one disclosure. */
  open: boolean;
  /** The snapshot a finding's restore link asked for. */
  preselect?: string;
  /** When that snapshot was taken, in Unix seconds. */
  preselectAt?: number;
}) {
  const [reloadTick, setReloadTick] = useState(0);
  const [deletingAll, setDeletingAll] = useState(false);
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [shakeDeleteAll, setShakeDeleteAll] = useState(0);
  const { flagged } = useOpenAnomalies();

  // "Delete all" empties the local place, which is what the question it asks
  // says; a copy at a target goes through its own row in the timeline. It
  // fails as a toast, not inline. Bumping reloadTick remounts the timeline
  // below under a fresh key, so it reads the place again instead of keeping
  // the rows the delete just emptied.
  async function handleDeleteAll() {
    // TODO: name the stake in the confirmation ("N snapshots, X GB").
    if (!(await confirm(t("snapshots.deleteAllConfirm"), { confirmKey: "snapshots.deleteAll" }))) return;
    setDeletingAll(true);
    deleteBackupsVM(name, "local")
      .then((res) => {
        if (!res.ok) {
          push(res.error ?? t("common.deleteBackupsFailed"), "fail");
          setShakeDeleteAll((n) => n + 1);
        }
      })
      .catch(() => {
        push(t("common.deleteBackupsFailed"), "fail");
        setShakeDeleteAll((n) => n + 1);
      })
      .finally(() => {
        setDeletingAll(false);
        setReloadTick((n) => n + 1);
      });
  }

  // Closed renders nothing at all: the trigger lives in VMRow, as it does for
  // components/RestorePanel.tsx.
  if (!open) return null;

  return (
    <>
      <div className="rounded-card bg-carbon-background px-3 py-1">
        <RecentRunsList name={name} domain="vm" t={t} />
        <SizeBreakdown domain="vms" item={name} t={t} />
        <Timeline
          key={reloadTick}
          domain="vms"
          itemKey={name}
          itemName={displayName ?? name}
          open={open}
          flagged={flagged}
          request={preselect ? { snapshot: preselect, at: preselectAt } : undefined}
          header={(rows) =>
            rows.some((r) => r.places.some((m) => m.place === "local")) && (
              <Button
                key={shakeDeleteAll}
                label={t("snapshots.deleteAll")}
                labelKey="snapshots.deleteAll"
                tone="neutral"
                onClick={() => void handleDeleteAll()}
                disabled={deletingAll}
                busy={deletingAll}
                title={deletingAll ? t("snapshots.deletingAll") : undefined}
                className={`self-end my-1${shakeDeleteAll ? " glim-shake" : ""}`}
              />
            )
          }
          renderActions={(pick) => (
            <VMSnapshotActions
              pick={pick}
              vmName={name}
              vmDisplayName={displayName}
              preselected={pick.row.key === preselect}
              t={t}
            />
          )}
        />
      </div>
      {confirmDialog}
    </>
  );
}
