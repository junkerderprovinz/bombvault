import { useEffect, useRef, useState } from "react";

import { useT } from "../../../lib/i18n";
import { useToast } from "../../../lib/toast";
import { Badge } from "../../Badge";
import { Button } from "../../Button";
import { IconFleet, IconZFS } from "../../navGlyphs";
import { ReplicaSnapshotsSheet } from "./ReplicaSnapshotsSheet";
import { ReplicaStatePill } from "./ReplicaStatePill";
import { isRunning, peerHolds, targetName } from "./replicaModel";
import { useGroup, useReplica, useReplicaServers } from "./replicaStore";

interface Restoring {
  dataset: string;
  keyNeeded: boolean;
  target: string;
  seen: boolean;
  ok?: boolean;
}

/** ReplicaPlaceRow is the replica among an item's storage locations: where it
 *  lives, how many snapshots it keeps there, and the way into them. */
export function ReplicaPlaceRow({ itemId, name }: { itemId: string; name: string }) {
  const { t } = useT();
  const { push } = useToast();
  const { replica, progressActive, restore } = useReplica(itemId);
  const { servers } = useReplicaServers();
  const group = useGroup();
  const [open, setOpen] = useState(false);
  const restoring = useRef<Restoring | null>(null);

  // The bring back's last frame says whether it worked, and once its progress
  // is gone the new dataset is there to look at. Its first frame can arrive
  // before the request is answered.
  useEffect(() => {
    const r = restoring.current;
    if (!r) return;
    if (restore) {
      r.seen = true;
      r.ok = restore.finished ? restore.percent >= 100 : undefined;
      return;
    }
    if (!r.seen) return;
    restoring.current = null;
    if (r.ok === false) {
      push(t("zfs.replica.restoreFailed").replaceAll("{name}", () => name).replace("{target}", () => r.target), "fail");
    } else if (r.ok) {
      const key = r.keyNeeded ? "zfs.replica.restoredLocked" : "zfs.replica.restored";
      push(t(key).replace("{fresh}", () => r.dataset).replace("{name}", () => name), "success");
    }
  }, [restore, name, push, t]);

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
        {(running || replica.state !== "ok" || peerHolds(replica)) && (
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
          onRestoring={(dataset, keyNeeded) => {
            restoring.current = { dataset, keyNeeded, target: shownName, seen: restore !== undefined };
          }}
        />
      )}
    </div>
  );
}
