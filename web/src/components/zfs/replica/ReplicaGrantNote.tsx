import { useState } from "react";

import { decideZFSReplicaGrant } from "../../../lib/api";
import type { ZFSReplicaGrant } from "../../../lib/api";
import { useT } from "../../../lib/i18n";
import { relativeTime } from "../../../lib/reltime";
import { useConfirm } from "../../../lib/useConfirm";
import { useToast } from "../../../lib/toast";
import { zfsCodeSentence } from "../../../lib/zfsCodes";
import { Button } from "../../Button";
import { InfoBubble } from "../../InfoBubble";
import { IconCheck } from "../../Sidebar";
import { useGroup, useReplica } from "./replicaStore";
import { unixOf } from "./replicaModel";

type Decision = "allow" | "refuse" | "revoke";

function usePeerName(): (id: string) => string {
  const group = useGroup();
  return (id) => group?.members.find((m) => m.id === id)?.name || id;
}

function useDecide(itemId: string, onDecided: () => void) {
  const { t } = useT();
  const { push } = useToast();
  const [busy, setBusy] = useState("");

  async function decide(peer: string, decision: Decision): Promise<boolean> {
    setBusy(peer);
    try {
      const res = await decideZFSReplicaGrant(itemId, peer, decision);
      if (!res.ok) {
        push(res.code ? zfsCodeSentence(t, res.code) : (res.error ?? t("settings.error")), "fail");
        return false;
      }
      onDecided();
      return true;
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      return false;
    } finally {
      setBusy("");
    }
  }

  return { decide, busy };
}

/** ReplicaGrantNote shows each paired instance that asked to pull this item.
 *  Nothing reaches the key file before someone here allows it. */
export function ReplicaGrantNote({ itemId, name }: { itemId: string; name: string }) {
  const { t } = useT();
  const { push } = useToast();
  const { replica, reload } = useReplica(itemId);
  const peerName = usePeerName();
  const { decide, busy } = useDecide(itemId, reload);

  const asked = (replica?.grants ?? []).filter((g) => g.state === "asked");
  if (asked.length === 0) return null;

  async function answer(grant: ZFSReplicaGrant, allow: boolean) {
    const peer = peerName(grant.peer);
    if (!(await decide(grant.peer, allow ? "allow" : "refuse"))) return;
    push(
      allow
        ? t("zfs.replica.grant.accepted").replace("{peer}", () => peer).replace("{name}", () => name)
        : t("zfs.replica.grant.declined").replace("{peer}", () => peer),
      "success",
    );
  }

  return (
    <div className="flex flex-col gap-2">
      {asked.map((grant) => {
        const peer = peerName(grant.peer);
        return (
          <div
            key={grant.peer}
            className="flex items-center gap-3 flex-wrap rounded-card bg-carbon-surface2 p-3"
          >
            <div className="flex-1 min-w-0 flex flex-col gap-0.5">
              <span className="flex items-center gap-1.5 text-sm font-semibold text-carbon-text">
                {t("zfs.replica.grant.asked").replace("{peer}", () => peer).replace("{name}", () => name)}
                <InfoBubble tip={t("zfs.replica.grant.askedHint")} />
              </span>
              <span className="text-xs text-carbon-textMuted">
                {t("zfs.replica.grant.key")
                  .replace("{fp}", () => grant.fingerprint)
                  .replace("{when}", () => relativeTime(t, unixOf(grant.askedAt)))}
              </span>
            </div>
            <div className="flex items-center gap-2 max-md:w-full max-md:justify-end">
              <Button
                label={t("zfs.replica.grant.decline")}
                labelKey="zfs.replica.grant.decline"
                tone="neutral"
                onClick={() => void answer(grant, false)}
                disabled={busy !== ""}
              />
              <Button
                label={t("zfs.replica.grant.accept")}
                labelKey="zfs.replica.grant.accept"
                glyph={<IconCheck />}
                tone="accent"
                onClick={() => void answer(grant, true)}
                disabled={busy !== ""}
                busy={busy === grant.peer}
              />
            </div>
          </div>
        );
      })}
    </div>
  );
}

/** ReplicaGrantRows lists the instances allowed to pull this item, each with
 *  the way to take the permission back. */
export function ReplicaGrantRows({ itemId, grants }: { itemId: string; grants: ZFSReplicaGrant[] }) {
  const { t } = useT();
  const { push } = useToast();
  const { reload } = useReplica(itemId);
  const { confirm, confirmDialog } = useConfirm();
  const peerName = usePeerName();
  const { decide, busy } = useDecide(itemId, reload);

  const allowed = grants.filter((g) => g.state === "allowed");
  if (allowed.length === 0) return null;

  async function revoke(grant: ZFSReplicaGrant) {
    const peer = peerName(grant.peer);
    const question = `${t("zfs.replica.grant.revokeQuestion")} ${t("zfs.replica.grant.revokeText")}`.replaceAll(
      "{peer}",
      peer,
    );
    if (!(await confirm(question, { confirmKey: "zfs.replica.grant.revokeConfirm" }))) return;
    if (await decide(grant.peer, "revoke")) push(t("zfs.replica.grant.revoked"), "success");
  }

  return (
    <div className="flex flex-col gap-2">
      {allowed.map((grant) => (
        <div key={grant.peer} className="flex items-center gap-3 flex-wrap">
          <div className="flex-1 min-w-0 flex flex-col gap-0.5">
            <span className="flex items-center gap-1.5 text-sm text-carbon-text">
              {t("zfs.replica.grant.allowed").replace("{peer}", () => peerName(grant.peer))}
              <InfoBubble tip={t("zfs.replica.grant.allowedHint")} />
            </span>
            <span className="text-xs text-carbon-textMuted">
              {t("zfs.replica.grant.allowedAt").replace("{when}", () => relativeTime(t, unixOf(grant.decidedAt)))}
            </span>
          </div>
          <Button
            label={t("zfs.replica.grant.revoke")}
            labelKey="zfs.replica.grant.revoke"
            tone="neutral"
            onClick={() => void revoke(grant)}
            disabled={busy !== ""}
            busy={busy === grant.peer}
          />
        </div>
      ))}
      {confirmDialog}
    </div>
  );
}
