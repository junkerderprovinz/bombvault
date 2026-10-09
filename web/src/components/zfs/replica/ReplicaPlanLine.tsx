import { useT } from "../../../lib/i18n";
import { relativeTime } from "../../../lib/reltime";
import { targetName, unixOf } from "./replicaModel";
import { useGroup, useReplica, useReplicaServers } from "./replicaStore";

/** ReplicaPlanLine is the replica's part of an item's plan and 3-2-1 line:
 *  where the replica goes and, once one is current, when it was made. Whether
 *  it counts as a second site is the server's call, made with the rest of the
 *  line. */
export function ReplicaPlanLine({ itemId }: { itemId: string }) {
  const { t } = useT();
  const { replica } = useReplica(itemId);
  const { servers } = useReplicaServers();
  const group = useGroup();
  if (!replica || replica.target.kind === "none") return null;
  const target = targetName(replica, servers, group?.members ?? []);
  return (
    <p className="text-xs text-carbon-textMuted">
      {t("zfs.replica.planOn").replace("{target}", () => target)}
      {replica.state === "ok" && (
        <>
          {" · "}
          {t("zfs.replica.planSeen").replace("{when}", () => relativeTime(t, unixOf(replica.lastRun)))}
        </>
      )}
    </p>
  );
}
