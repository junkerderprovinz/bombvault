import { useEffect, useState } from "react";

import { deleteZFSReplicaServer, listZFSDatasets, patchZFSReplicaServer, testZFSReplicaServer } from "../../../lib/api";
import { useT } from "../../../lib/i18n";
import { useConfirm } from "../../../lib/useConfirm";
import { useTestVerdict } from "../../../lib/useTestVerdict";
import { useToast } from "../../../lib/toast";
import { zfsCodeSentence } from "../../../lib/zfsCodes";
import { ToggleRow } from "../../../pages/settings/shared";
import { Badge } from "../../Badge";
import { Button } from "../../Button";
import { InfoBubble } from "../../InfoBubble";
import { TestButton, VerdictLine } from "../../TestButton";
import { IconZFS } from "../../navGlyphs";
import { AddReplicaServerDialog } from "./AddReplicaServerDialog";
import { ReplicaStatePill } from "./ReplicaStatePill";
import { examplePath, isRunning, rootMember, serverFree, targetName, testFailure } from "./replicaModel";
import { useGroup, useReplica, useReplicaServers } from "./replicaStore";

function EntryRow({ itemId, dataset }: { itemId: string; dataset: string }) {
  const { replica, progressActive } = useReplica(itemId);
  const { servers } = useReplicaServers();
  const group = useGroup();
  // The server names the folder, which keeps the name of the first run and
  // has the characters ZFS refuses replaced.
  const path = replica ? rootMember(replica, dataset)?.targetPath : undefined;
  return (
    <li className="flex items-center gap-3 flex-wrap">
      <span className="text-carbon-textSub" aria-hidden="true">
        <IconZFS />
      </span>
      <div className="flex-1 min-w-0 flex flex-col">
        <span dir="ltr" className="text-sm text-carbon-text text-start wrap-anywhere">
          {dataset}
        </span>
        {path && (
          <span dir="ltr" className="font-mono text-caption text-carbon-textMuted text-start wrap-anywhere">
            {path}
          </span>
        )}
      </div>
      {replica && (
        <ReplicaStatePill
          replica={replica}
          running={isRunning(replica.state, progressActive)}
          peerName={targetName(replica, servers, group?.members ?? [])}
        />
      )}
    </li>
  );
}

/** ReplicaServerPage is one ZFS server: its connection and root, whether
 *  BombVault sends to it, and the items that replicate there. It never shows
 *  restic settings, because no backup lands on it. */
export function ReplicaServerPage({ serverId, onBack }: { serverId: string; onBack: () => void }) {
  const { t } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const { servers, reload } = useReplicaServers();
  const group = useGroup();
  const [datasets, setDatasets] = useState<ReadonlyMap<string, string>>(new Map());
  const [editing, setEditing] = useState(false);
  const [enabledBusy, setEnabledBusy] = useState(false);
  const server = servers.find((s) => s.id === serverId);
  const check = useTestVerdict([server?.host, server?.user, server?.port], t("common.networkError"));

  useEffect(() => {
    listZFSDatasets()
      .then((res) => setDatasets(new Map((res.datasets ?? []).map((d) => [d.id, d.dataset]))))
      .catch(() => undefined);
  }, []);

  if (!server) return null;
  const users = server.usedBy.map((id) => ({ id, dataset: datasets.get(id) ?? id }));
  const ownName = group?.name || "<server>";

  async function setEnabled(next: boolean) {
    setEnabledBusy(true);
    try {
      const res = await patchZFSReplicaServer(serverId, { enabled: next });
      if (res.ok) reload();
      else push(res.code ? zfsCodeSentence(t, res.code) : (res.error ?? t("settings.error")), "fail");
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    } finally {
      setEnabledBusy(false);
    }
  }

  function test() {
    void check.run(async () => {
      const r = await testZFSReplicaServer(serverId);
      return r.ok ? { ok: true } : { ok: false, reason: testFailure(t, r) };
    });
  }

  const remove = async () => {
    const question = `${t("zfs.replica.server.removeQuestion").replace("{name}", () => server.name)} ${t(
      "zfs.replica.server.removeText",
    )}`;
    if (!(await confirm(question, { confirmKey: "zfs.replica.server.removeConfirm" }))) return;
    try {
      const res = await deleteZFSReplicaServer(serverId, server.usedBy.length > 0);
      if (!res.ok) {
        push(res.code ? zfsCodeSentence(t, res.code) : (res.error ?? t("common.removeFailed")), "fail");
        return;
      }
      push(t("zfs.replica.server.removed"), "success");
      reload();
      onBack();
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.removeFailed"), "fail");
    }
  };

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-start gap-3 flex-wrap">
        <span className="mt-0.5 text-carbon-textSub" aria-hidden="true">
          <IconZFS />
        </span>
        <div className="flex-1 min-w-0 flex flex-col">
          <span className="font-semibold text-carbon-text">{server.name}</span>
          <span className="text-xs text-carbon-textMuted">
            {t("zfs.replica.servers.title")}
            {" · "}
            {users.length > 0
              ? t("zfs.replica.server.usedBy").replace("{names}", () => users.map((u) => u.dataset).join(", "))
              : t("zfs.replica.servers.unused")}
          </span>
        </div>
        {server.enabled ? (
          <span className="glim-num text-xs text-carbon-textSub">{serverFree(t, server)}</span>
        ) : (
          <Badge tone="neutral">{t("zfs.replica.servers.off")}</Badge>
        )}
      </div>

      <div className="flex flex-col gap-3 rounded-card bg-carbon-surface2 p-3">
        <span className="text-xs font-semibold text-carbon-textSub">{t("zfs.replica.server.connection")}</span>
        <div className="flex items-center justify-between gap-3 flex-wrap text-sm">
          <span className="text-carbon-textSub">{t("zfs.replica.server.address")}</span>
          <span dir="ltr" className="font-mono text-xs text-carbon-text">{`${server.user}@${server.host}:${server.port}`}</span>
        </div>
        <div className="flex flex-col gap-0.5 text-sm">
          <div className="flex items-center justify-between gap-3 flex-wrap">
            <span className="flex items-center gap-1.5 text-carbon-textSub">
              {t("zfs.replica.server.root")}
              <InfoBubble tip={t("zfs.replica.server.rootHint")} />
            </span>
            <span dir="ltr" className="font-mono text-xs text-carbon-text">{server.root}</span>
          </div>
          <span className="text-caption text-carbon-textMuted">
            <bdi dir="ltr">
              {t("zfs.replica.server.example").replace("{path}", () => examplePath(server.root, ownName))}
            </bdi>
          </span>
        </div>
        <ToggleRow
          label={t("zfs.replica.server.enabled")}
          hint={t("zfs.replica.server.enabledHint")}
          checked={server.enabled}
          onChange={(next) => void setEnabled(next)}
          disabled={enabledBusy}
        />
        <VerdictLine verdict={check.verdict} />
        <div className="flex justify-end">
          <TestButton
            label={t("zfs.replica.server.test")}
            labelKey="zfs.replica.server.test"
            test={check}
            onClick={test}
          />
        </div>
      </div>

      <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3">
        <span className="text-xs font-semibold text-carbon-textSub">{t("zfs.replica.server.entries")}</span>
        {users.length === 0 ? (
          <p className="text-xs text-carbon-textMuted">{t("zfs.replica.server.entriesEmpty")}</p>
        ) : (
          <ul aria-label={t("zfs.replica.server.entries")} className="flex flex-col gap-2">
            {users.map((u) => (
              <EntryRow key={u.id} itemId={u.id} dataset={u.dataset} />
            ))}
          </ul>
        )}
      </div>

      <div className="flex flex-wrap items-center justify-end gap-2">
        <Button label={t("common.back")} labelKey="common.back" tone="neutral" onClick={onBack} />
        <Button
          label={t("zfs.replica.server.remove")}
          labelKey="zfs.replica.server.remove"
          tone="neutral"
          onClick={() => void remove()}
        />
        <Button
          label={t("zfs.replica.server.edit")}
          labelKey="zfs.replica.server.edit"
          tone="neutral"
          onClick={() => setEditing(true)}
        />
      </div>

      {editing && (
        <AddReplicaServerDialog
          server={server}
          onClose={() => setEditing(false)}
          onSaved={() => {
            setEditing(false);
            reload();
          }}
        />
      )}
      {confirmDialog}
    </div>
  );
}
