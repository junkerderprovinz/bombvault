import type { ZFSReplica } from "../../../lib/api";
import { useT } from "../../../lib/i18n";
import { relativeTime } from "../../../lib/reltime";
import { zfsCodeSentence } from "../../../lib/zfsCodes";
import { Badge } from "../../Badge";
import { InfoBubble } from "../../InfoBubble";
import { unixOf, waitingCode } from "./replicaModel";
import { useReplicaServers } from "./replicaStore";

/** ReplicaStatePill says where an item's replica stands, in the words the
 *  card, the server page and the storage row share. A paired instance that
 *  has not allowed the replica outranks the state of the runs. */
export function ReplicaStatePill({
  replica,
  running,
  peerName,
}: {
  replica: ZFSReplica;
  running: boolean;
  /** The receiving instance of a peer target. */
  peerName: string;
}) {
  const { t } = useT();
  const { servers } = useReplicaServers();
  if (replica.target.kind === "peer") {
    switch (replica.peerState) {
      case "asked":
        return <Badge tone="neutral">{t("zfs.replica.peer.asked").replaceAll("{peer}", () => peerName)}</Badge>;
      case "refused":
        return <Badge tone="warn">{t("zfs.replica.peer.refused").replaceAll("{peer}", () => peerName)}</Badge>;
      case "revoked":
        return <Badge tone="warn">{t("zfs.replica.peer.revoked").replaceAll("{peer}", () => peerName)}</Badge>;
      case "off":
        return <Badge tone="warn">{t("zfs.replica.peer.off").replaceAll("{peer}", () => peerName)}</Badge>;
    }
  }
  if (running) return <Badge tone="active">{t("zfs.replica.state.running")}</Badge>;
  switch (replica.state) {
    case "ok":
      return (
        <Badge tone="ok">
          {t("zfs.replica.state.ok").replace("{when}", () => relativeTime(t, unixOf(replica.lastRun)))}
        </Badge>
      );
    case "failed":
      return <Badge tone="fail">{t("zfs.replica.state.failed")}</Badge>;
    case "waiting":
      return (
        <span className="inline-flex items-center gap-1">
          <Badge tone="neutral">{t("zfs.replica.state.waiting")}</Badge>
          <InfoBubble tip={zfsCodeSentence(t, waitingCode(replica, servers))} />
        </span>
      );
    default:
      return <Badge tone="neutral">{t("zfs.replica.state.never")}</Badge>;
  }
}
