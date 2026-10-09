import { useEffect, useRef, useState } from "react";

import { useT } from "../../../lib/i18n";
import { useToast } from "../../../lib/toast";
import { Badge } from "../../Badge";
import { Button } from "../../Button";
import { IconFleet, IconZFS } from "../../navGlyphs";
import { ReplicaSnapshotsSheet } from "./ReplicaSnapshotsSheet";
import { ReplicaStatePill } from "./ReplicaStatePill";
import { isRunning, targetName } from "./replicaModel";
import { useGroup, useReplica, useReplicaServers } from "./replicaStore";

/** ReplicaPlaceRow is the replica among an item's storage locations: where it
 *  lives, how many snapshots it keeps there, and the way into them. */
export function ReplicaPlaceRow({ itemId, name }: { itemId: string; name: string }) {
  const { t } = useT();
  const { push } = useToast();
  const { replica, progressActive } = useReplica(itemId);
  const { servers } = useReplicaServers();
  const group = useGroup();
  const [open, setOpen] = useState(false);
  const restoring = useRef<{ dataset: string; seen: boolean } | null>(null);

  // The restore runs on the replica's progress key; its end is the moment
  // the new dataset is there to look at.
  useEffect(() => {
    const r = restoring.current;
    if (!r) return;
    if (progressActive) {
      r.seen = true;
      return;
    }
    if (!r.seen) return;
    restoring.current = null;
    push(t("zfs.replica.restored").replace("{fresh}", () => r.dataset).replace("{name}", () => name), "success");
  }, [progressActive, name, push, t]);

  if (!replica || replica.target.kind === "none") return null;
  const shownName = targetName(replica, servers, group?.members ?? []);
  const running = isRunning(replica.state, progressActive);
  const count = replica.snapshots.length;

  return (
    <div className="flex items-center gap-3 flex-wrap rounded-card bg-carbon-surface2 px-3 py-2">
      <span className="text-carbon-textSub" aria-hidden="true">
        {replica.target.kind === "peer" ? <IconFleet /> : <IconZFS />}
      </span>
      <div className="flex-1 min-w-0 flex flex-col">
        <span className="text-sm text-carbon-text">{shownName}</span>
        <span className="text-caption text-carbon-textMuted">{t("zfs.replica.place")}</span>
      </div>
      <div className="flex items-center gap-2 flex-wrap">
        {(running || replica.state !== "ok") && (
          <ReplicaStatePill replica={replica} running={running} peerName={shownName} />
        )}
        {count > 0 && (
          <>
            <Badge tone="muted">{t("zfs.replica.snapshots", count)}</Badge>
            <Button
              label={t("zfs.replica.view")}
              labelKey="zfs.replica.view"
              tone="neutral"
              onClick={() => setOpen(true)}
            />
          </>
        )}
      </div>
      {open && (
        <ReplicaSnapshotsSheet
          itemId={itemId}
          name={name}
          targetName={shownName}
          replica={replica}
          onClose={() => setOpen(false)}
          onRestoring={(dataset) => {
            restoring.current = { dataset, seen: false };
          }}
        />
      )}
    </div>
  );
}
