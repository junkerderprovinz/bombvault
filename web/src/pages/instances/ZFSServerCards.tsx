import { useState } from "react";
import { useNavigate } from "react-router-dom";

import { Badge } from "../../components/Badge";
import { Button } from "../../components/Button";
import { IconEye } from "../../components/glyphs";
import { IconPencil, IconZFS } from "../../components/navGlyphs";
import { AddReplicaServerDialog } from "../../components/zfs/replica/AddReplicaServerDialog";
import { unixOf } from "../../components/zfs/replica/replicaModel";
import { useReplicaServers } from "../../components/zfs/replica/replicaStore";
import type { ZFSReplicaServer } from "../../lib/api";
import { humanBytes } from "../../lib/forecast";
import { useT } from "../../lib/i18n";
import { relativeTime } from "../../lib/reltime";
import { Facts, Figure, GlyphMark, InstanceCard } from "./InstanceCard";
import type { ItemReplica } from "./instancesModel";

/** ZFSServerCards are the ZFS servers without BombVault, reached over SSH.
 *  They take replicas like a paired instance does, so they stand in the same
 *  grid, each opening its own page. */
export function ZFSServerCards({ replicas, firstIndex }: { replicas: ItemReplica[]; firstIndex: number }) {
  const { t } = useT();
  const navigate = useNavigate();
  const { servers, reload } = useReplicaServers();
  const [editing, setEditing] = useState<ZFSReplicaServer | null>(null);

  return (
    <>
      {servers.map((s, i) => {
        const lastRun = replicas
          .filter((r) => s.usedBy.includes(r.itemId) && r.replica.state === "ok")
          .reduce((latest, r) => Math.max(latest, unixOf(r.replica.lastRun)), 0);
        return (
          <InstanceCard
            key={s.id}
            cardKey={`zfs:${s.id}`}
            name={s.name}
            eyebrow={t("instances.overSsh")}
            mark={
              <GlyphMark>
                <IconZFS />
              </GlyphMark>
            }
            badges={!s.enabled && <Badge tone="neutral" size="large">{t("zfs.replica.servers.off")}</Badge>}
            facts={
              <Facts
                items={[
                  <bdi dir="ltr">{`${s.user}@${s.host}`}</bdi>,
                  <bdi dir="ltr" className="font-mono">
                    {s.root}
                  </bdi>,
                ]}
              />
            }
            figures={
              <>
                <Figure value={humanBytes(s.freeBytes)} label={t("instances.fig.free")} />
                <Figure value={s.usedBy.length} label={t("instances.fig.replicas")} />
                <Figure
                  value={lastRun > 0 ? relativeTime(t, lastRun) : t("receiver.never")}
                  label={t("instances.fig.lastReplica")}
                />
              </>
            }
            actions={
              <>
                <Button
                  label={t("instances.open")}
                  labelKey="instances.open"
                  glyph={<IconEye />}
                  tone="neutral"
                  onClick={() => navigate(`/instances/zfs/${encodeURIComponent(s.id)}`)}
                />
                <Button
                  label={t("zfs.replica.server.edit")}
                  labelKey="zfs.replica.server.edit"
                  glyph={<IconPencil />}
                  tone="neutral"
                  onClick={() => setEditing(s)}
                />
              </>
            }
            index={firstIndex + i}
          />
        );
      })}
      {editing && (
        <AddReplicaServerDialog
          server={editing}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setEditing(null);
            reload();
          }}
        />
      )}
    </>
  );
}
