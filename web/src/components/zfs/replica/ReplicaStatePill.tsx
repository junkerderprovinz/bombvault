import type { ZFSReplica } from "../../../lib/api";
import { useT } from "../../../lib/i18n";
import { relativeTime } from "../../../lib/reltime";
import { Badge } from "../../Badge";
import { unixOf } from "./replicaModel";

/** ReplicaStatePill says where an item's replica stands, in the words the
 *  card, the server page and the storage row share. */
export function ReplicaStatePill({
  replica,
  running,
  peerName,
}: {
  replica: ZFSReplica;
  running: boolean;
  /** The pulling instance a waiting replica waits for. */
  peerName: string;
}) {
  const { t } = useT();
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
      return <Badge tone="neutral">{t("zfs.replica.state.waiting").replace("{peer}", () => peerName)}</Badge>;
    default:
      return <Badge tone="neutral">{t("zfs.replica.state.never")}</Badge>;
  }
}
