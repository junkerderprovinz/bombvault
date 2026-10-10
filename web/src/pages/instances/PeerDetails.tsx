import { useState, type ReactNode } from "react";

import { Badge } from "../../components/Badge";
import { Button } from "../../components/Button";
import { InfoBubble } from "../../components/InfoBubble";
import { ReplicaStatePill } from "../../components/zfs/replica/ReplicaStatePill";
import { ZFSReceiveRequests } from "../../components/zfs/replica/ZFSReceiveRequests";
import { isRunning } from "../../components/zfs/replica/replicaModel";
import { checkFleetPeer, updateFleetPeer } from "../../lib/api";
import type { PullSourceView, ReceivedRepoStatus } from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { relativeTime } from "../../lib/reltime";
import { useToast } from "../../lib/toast";
import { hueCounter, ToggleRow } from "../settings/shared";
import { Facts } from "./InstanceCard";
import { InstanceWindow, WindowCard } from "./InstanceWindow";
import { MeshOffers, ProposeMeshDialog } from "./MeshOffers";
import { PeerScorecard } from "./PeerScorecard";
import { PullDialog, PullSourceCard } from "./PullSourceCard";
import { ReceivedRepoCard, ReceiverDialog } from "./ReceivedRepoCard";
import { RemoveInstanceButton } from "./RemoveInstanceButton";
import { ROLES, ROLE_STATE_KEY } from "./RoleTile";
import { ROLE_KINDS, withoutScheme, type Instance, type RoleKind, type RoleState } from "./instancesModel";
import type { InstancesState, Modules } from "./useInstances";

const STATE_TONE: Record<RoleState, "active" | "warn" | "neutral"> = { on: "active", waiting: "warn", off: "neutral" };

function RoleRow({ role, state, children }: { role: RoleKind; state: RoleState; children?: ReactNode }) {
  const { t } = useT();
  const { label, tip, glyph } = ROLES[role];
  return (
    <li data-role={role} className="flex flex-wrap items-center gap-x-3 gap-y-2">
      <span className="grid h-9 w-9 shrink-0 place-items-center rounded-control bg-carbon-surface3 text-accentText">
        {glyph}
      </span>
      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="flex items-center gap-1.5 text-sm font-semibold text-carbon-text">
          {t(label)}
          <InfoBubble tip={t(tip)} />
        </span>
        {children}
      </span>
      <Badge tone={STATE_TONE[state]}>{t(ROLE_STATE_KEY[state])}</Badge>
    </li>
  );
}

type Dialog =
  | { kind: "received"; row: ReceivedRepoStatus | null }
  | { kind: "pull"; row: PullSourceView | null }
  | { kind: "propose" };

/** PeerDetails is the window of one paired instance: what it does for this
 *  server, what this server does for it, how well it is protected, whether
 *  this server polls it, and the way to remove it. */
export function PeerDetails({
  instance,
  modules,
  state,
  onClose,
}: {
  instance: Instance;
  modules: Modules;
  state: InstancesState;
  onClose: () => void;
}) {
  const { t } = useT();
  const { push } = useToast();
  const [dialog, setDialog] = useState<Dialog | null>(null);
  const { peer, name, memberId } = instance;
  const paired = memberId !== "" && !instance.needsPairing;
  const nextHue = hueCounter();
  const named = (key: TranslationKey) => t(key).replaceAll("{peer}", () => name);

  // A check outlasts any request, so the instance only confirms it started;
  // the verdict shows in its next scorecard.
  async function check(domain: string) {
    if (!peer) return;
    try {
      const res = await checkFleetPeer(peer.id, domain);
      if (res.ok) push(t("fleet.checkStarted"), "success");
      else push(res.error ?? t("fleet.saveError"), "fail");
    } catch (err) {
      push(err instanceof Error ? err.message : t("fleet.saveError"), "fail");
    }
  }

  async function setPolled(enabled: boolean) {
    if (!peer) return;
    try {
      const res = await updateFleetPeer(peer.id, { enabled });
      if (!res.ok) push(res.error ?? t("fleet.saveError"), "fail");
      void state.reloadPeers();
    } catch (err) {
      push(err instanceof Error ? err.message : t("fleet.saveError"), "fail");
    }
  }

  const roles = ROLE_KINDS.filter((k) => instance.roles[k] !== undefined);
  const canOffer = modules.fleet && peer !== undefined && paired;

  return (
    <InstanceWindow
      title={name}
      facts={
        <Facts
          items={[
            instance.address && <bdi dir="ltr">{withoutScheme(instance.address)}</bdi>,
            peer?.lastPollVersion,
            peer && peer.lastPollAt > 0 && t("fleet.lastPolled").replace("{time}", relativeTime(t, peer.lastPollAt)),
          ]}
        />
      }
      onClose={onClose}
      footer={
        <>
          {modules.fleet && peer && (
            <RemoveInstanceButton
              peerId={peer.id}
              onRemoved={() => {
                void state.reloadPeers();
                onClose();
              }}
            />
          )}
          <Button label={t("common.done")} labelKey="common.done" tone="accent" onClick={onClose} />
        </>
      }
    >
      {(roles.length > 0 || instance.offers.length > 0) && (
        <WindowCard title={named("instances.doesForServer")} hint={t("instances.role.diff")} hueIndex={nextHue()}>
          <MeshOffers offers={instance.offers} onChanged={() => void state.reloadOffers()} />
          <ul className="flex flex-col gap-4">
            {roles.map((k) => (
              <RoleRow key={k} role={k} state={instance.roles[k]!}>
                {k === "receiver" && instance.destination && (
                  <span className="text-xs text-carbon-textMuted">
                    {t("instances.role.location").replace("{name}", () => instance.destination!.name)}
                  </span>
                )}
                {k === "zfs" && instance.replicas.length === 0 && (
                  <span className="text-xs text-carbon-textMuted">{t("instances.role.noReplica")}</span>
                )}
                {k === "zfs" &&
                  instance.replicas.map((r) => (
                    <span key={r.itemId} className="flex flex-wrap items-center gap-2 text-xs text-carbon-textMuted">
                      <bdi dir="ltr" className="font-mono">
                        {r.dataset}
                      </bdi>
                      <ReplicaStatePill replica={r.replica} running={isRunning(r.replica.state, false)} peerName={name} />
                    </span>
                  ))}
              </RoleRow>
            ))}
          </ul>
        </WindowCard>
      )}

      {paired && (
        <WindowCard title={named("instances.serverDoesFor")} hueIndex={nextHue()}>
          {modules.receiver && <ZFSReceiveRequests peer={memberId} />}
          {instance.received.map((r, i) => (
            <ReceivedRepoCard
              key={r.id}
              repo={r}
              t={t}
              index={i}
              onRefresh={() => void state.reloadReceived()}
              onEdit={() => setDialog({ kind: "received", row: r })}
            />
          ))}
          {instance.pulls.map((s, i) => (
            <PullSourceCard
              key={s.id}
              source={s}
              t={t}
              index={instance.received.length + i}
              onRefresh={() => void state.reloadPulls()}
              onEdit={() => setDialog({ kind: "pull", row: s })}
            />
          ))}
          <div className="flex flex-wrap items-center justify-end gap-2">
            {canOffer && (
              <Button
                label={t("fleet.mesh.proposeButton")}
                labelKey="fleet.mesh.proposeButton"
                tone="neutral"
                onClick={() => setDialog({ kind: "propose" })}
              />
            )}
            {modules.receiver && (
              <Button
                label={t("receiver.addRepo")}
                labelKey="receiver.addRepo"
                tone="neutral"
                onClick={() => setDialog({ kind: "received", row: null })}
              />
            )}
            {modules.pull && (
              <Button
                label={t("pull.addSource")}
                labelKey="pull.addSource"
                tone="neutral"
                onClick={() => setDialog({ kind: "pull", row: null })}
              />
            )}
          </div>
        </WindowCard>
      )}

      {modules.fleet && peer && paired && (
        <>
          <WindowCard title={t("instances.protection")} hueIndex={nextHue()}>
            <PeerScorecard domains={peer.lastPollDomains} onCheck={(domain) => void check(domain)} />
            {peer.lastPollOk === false && peer.lastPollError && (
              <p className="text-xs text-statusFail wrap-break-word">{peer.lastPollError}</p>
            )}
          </WindowCard>
          <WindowCard title={t("instances.monitoring")} hueIndex={nextHue()}>
            <ToggleRow
              label={t("fleet.enabledLabel")}
              hint={t("instances.pollHint")}
              checked={peer.enabled}
              onChange={(v) => void setPolled(v)}
            />
          </WindowCard>
        </>
      )}

      {dialog?.kind === "received" && (
        <ReceiverDialog
          initial={dialog.row}
          member={memberId}
          t={t}
          onClose={() => setDialog(null)}
          onSaved={() => {
            setDialog(null);
            void state.reloadReceived();
          }}
        />
      )}
      {dialog?.kind === "pull" && (
        <PullDialog
          initial={dialog.row}
          member={memberId}
          t={t}
          onClose={() => setDialog(null)}
          onSaved={() => {
            setDialog(null);
            void state.reloadPulls();
          }}
        />
      )}
      {dialog?.kind === "propose" && peer && <ProposeMeshDialog peer={peer} t={t} onClose={() => setDialog(null)} />}
    </InstanceWindow>
  );
}
