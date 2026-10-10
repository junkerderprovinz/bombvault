// ZFSServer is the page of one ZFS server without BombVault, opened from its
// card in the instances grid.
import { Navigate, useNavigate, useParams } from "react-router-dom";

import { PageTitle } from "../components/PageTitle";
import { ReplicaServerPage } from "../components/zfs/replica/ReplicaServerPage";
import { useReplicaServers } from "../components/zfs/replica/replicaStore";
import { useT } from "../lib/i18n";
import { PAGE_SHELL } from "../lib/pageShell";
import { Card } from "./settings/shared";

export function ZFSServer() {
  const { t } = useT();
  const navigate = useNavigate();
  const { id = "" } = useParams();
  const { servers, loaded } = useReplicaServers();

  // An address that names no server, such as a removed one, leads back to the grid.
  if (loaded && !servers.some((s) => s.id === id)) return <Navigate to="/instances" replace />;

  return (
    <div className={PAGE_SHELL}>
      <PageTitle>{t("zfs.replica.servers.title")}</PageTitle>
      {loaded && (
        <Card title={t("zfs.replica.servers.title")} hint={t("zfs.replica.servers.hint")} hueIndex={0}>
          <ReplicaServerPage serverId={id} onBack={() => navigate("/instances")} />
        </Card>
      )}
    </div>
  );
}
