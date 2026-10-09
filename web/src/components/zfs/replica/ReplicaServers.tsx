import { useState } from "react";

import { useT } from "../../../lib/i18n";
import { Badge } from "../../Badge";
import { Button } from "../../Button";
import { InfoBubble } from "../../InfoBubble";
import { IconForward } from "../../glyphs";
import { IconZFS } from "../../navGlyphs";
import { AddReplicaServerDialog } from "./AddReplicaServerDialog";
import { ReplicaServerPage } from "./ReplicaServerPage";
import { serverFree } from "./replicaModel";
import { useReplicaServers } from "./replicaStore";

/** ReplicaServers lists the ZFS servers replicas can go to. Without `onOpen`
 *  a server opens in place of the list. */
export function ReplicaServers({ onOpen }: { onOpen?: (id: string) => void }) {
  const { t } = useT();
  const { servers, loaded, reload } = useReplicaServers();
  const [adding, setAdding] = useState(false);
  const [openId, setOpenId] = useState("");

  function open(id: string) {
    if (onOpen) onOpen(id);
    else setOpenId(id);
  }

  return (
    <div className="relative glim-notch-card flex flex-col gap-3 bg-carbon-surface rounded-card p-5">
      <h2 className="flex items-center">
        <Badge tone="heading" size="heading" wrap>
          {t("zfs.replica.servers.title")}
          <InfoBubble tip={t("zfs.replica.servers.hint")} onAccent />
        </Badge>
      </h2>

      {openId && servers.some((s) => s.id === openId) ? (
        <ReplicaServerPage serverId={openId} onBack={() => setOpenId("")} />
      ) : (
        <>
          {loaded && servers.length === 0 && (
            <p className="text-sm text-carbon-textMuted">{t("zfs.replica.servers.empty")}</p>
          )}
          {servers.length > 0 && (
            <ul aria-label={t("zfs.replica.servers.title")} className="flex flex-col gap-1">
              {servers.map((s) => (
                <li key={s.id}>
                  <button
                    type="button"
                    onClick={() => open(s.id)}
                    className="flex w-full items-center gap-3 rounded-control px-2 py-2 text-start hover:bg-carbon-surface2 glim-field-focus pointer-coarse:min-h-11"
                  >
                    <span className="text-carbon-textSub" aria-hidden="true">
                      <IconZFS />
                    </span>
                    <span className="flex-1 min-w-0 flex flex-col">
                      <span className="text-sm text-carbon-text">{s.name}</span>
                      <span className="text-caption text-carbon-textMuted">
                        <bdi dir="ltr">{`${s.user}@${s.host} · ${s.root}`}</bdi>
                        {" · "}
                        {s.usedBy.length > 0
                          ? t("zfs.replica.servers.inUse", s.usedBy.length)
                          : t("zfs.replica.servers.unused")}
                      </span>
                    </span>
                    {s.enabled ? (
                      <span className="glim-num text-xs text-carbon-textSub">{serverFree(t, s)}</span>
                    ) : (
                      <Badge tone="neutral">{t("zfs.replica.servers.off")}</Badge>
                    )}
                    <span className="text-carbon-textMuted" aria-hidden="true">
                      <IconForward />
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          )}
          <div className="flex justify-end">
            <Button
              label={t("zfs.replica.addServer")}
              labelKey="zfs.replica.addServer"
              tone="neutral"
              onClick={() => setAdding(true)}
            />
          </div>
        </>
      )}

      {adding && (
        <AddReplicaServerDialog
          onClose={() => setAdding(false)}
          onSaved={(id) => {
            setAdding(false);
            reload();
            if (id) open(id);
          }}
        />
      )}
    </div>
  );
}
