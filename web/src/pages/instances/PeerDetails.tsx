import { useState, type ReactNode } from "react";

import { Button } from "../../components/Button";
import { InfoBubble } from "../../components/InfoBubble";
import { AddLocationWindow } from "../../components/storage/AddLocationWindow";
import { Toggle } from "../../components/Toggle";
import { IconPencil } from "../../components/navGlyphs";
import { ReplicaStatePill } from "../../components/zfs/replica/ReplicaStatePill";
import { ZFSReceiveRequests } from "../../components/zfs/replica/ZFSReceiveRequests";
import { isRunning } from "../../components/zfs/replica/replicaModel";
import { checkFleetPeer, updateFleetPeer } from "../../lib/api";
import { unixOf } from "../../components/zfs/replica/replicaModel";
import type { KeeperRole, PullSourceView, ReceivedRepoStatus, RoleRequest, RoleSection } from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { relativeTime } from "../../lib/reltime";
import { flowDomains } from "../../lib/storageFlow";
import { useToast } from "../../lib/toast";
import { usePlacementDefaults } from "../../lib/usePlacementDefaults";
import { useStorageLocations } from "../../lib/useStorageLocations";
import { hueCounter, ToggleRow } from "../settings/shared";
import { Facts } from "./InstanceCard";
import { InstanceWindow, WindowCard } from "./InstanceWindow";
import { depositHelp } from "./KeyDepositWindow";
import { MeshOffers, ProposeMeshDialog } from "./MeshOffers";
import { domainLabelKey, PeerScorecard } from "./PeerScorecard";
import { PullDialog, PullSourceCard } from "./PullSourceCard";
import { ReceivedRepoCard, ReceiverDialog } from "./ReceivedRepoCard";
import { ForgetKeyButton } from "./ForgetKeyButton";
import { RemoveInstanceButton } from "./RemoveInstanceButton";
import { useRoleSwitch } from "./RoleDialogs";
import { RoleRequests } from "./RoleRequests";
import { ROLES, ROLE_STATE_KEY } from "./RoleTile";
import { ROLE_KINDS, tooOldToKeep, unreachableToKeep, withoutScheme, type Instance, type RoleKind, type RoleState } from "./instancesModel";
import type { InstancesState, Modules } from "./useInstances";

function RoleRow({
  role,
  state,
  onToggle,
  onChange,
  action,
  children,
}: {
  role: RoleKind;
  state: RoleState;
  onToggle: () => void;
  /** Opens what the role covers. A role that covers nothing has none. */
  onChange?: () => void;
  /** A button under the row, which gets the row's whole width. */
  action?: ReactNode;
  children: ReactNode;
}) {
  const { t } = useT();
  const { label, tip, glyph } = ROLES[role];
  return (
    <li data-role={role} data-state={state} className="flex flex-wrap items-center gap-x-3 gap-y-2">
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
      <span className="flex items-center gap-2">
        {state !== "off" && onChange && (
          <Button
            label={t("instances.role.change")}
            labelKey="instances.role.change"
            glyph={<IconPencil />}
            glyphOnly
            tone="neutral"
            onClick={onChange}
          />
        )}
        <Toggle checked={state !== "off"} onChange={onToggle} label={t(label)} hideLabel />
      </span>
      {action && <span className="flex basis-full justify-end">{action}</span>}
    </li>
  );
}

type Dialog =
  | { kind: "received"; row: ReceivedRepoStatus | null }
  | { kind: "pull"; row: PullSourceView | null }
  | { kind: "propose" }
  | { kind: "location" };

/** AddReceiverLocation opens the window that adds a storage location, whose
 *  first step offers the receiving servers of the group. */
function AddReceiverLocation({ onClose }: { onClose: () => void }) {
  const { locations } = useStorageLocations();
  const defaults = usePlacementDefaults();
  return <AddLocationWindow domains={flowDomains(locations)} defaults={defaults.rows} onClose={onClose} />;
}

/** PeerDetails is the window of one paired instance: what it does for this
 *  server, what this server does for it, how well it is protected, whether
 *  this server polls it, and the way to remove it. */
export function PeerDetails({
  instance,
  peers,
  modules,
  sections,
  state,
  onClose,
}: {
  instance: Instance;
  /** Every instance of the grid, for the names a dialog shows. */
  peers: Instance[];
  modules: Modules;
  /** The sections this server backs up, which a role can cover. */
  sections: RoleSection[];
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
  const roles = useRoleSwitch({ instance, peers, sections, state });

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

  /** What a Receiver or Fetcher row says under the role's name. */
  function requestLine(request: RoleRequest | null | undefined, role: RoleKind): ReactNode {
    const covers = request?.sections.map((s) => t(domainLabelKey(s))).join(", ");
    switch (request?.state) {
      case "asked":
        return <Facts items={[named("zfs.replica.peer.asked"), covers]} />;
      case "allowed":
        return (
          <Facts
            items={[
              covers,
              role === "receiver" && instance.destination && t("instances.role.location").replace("{name}", () => instance.destination!.name),
              role === "receiver" && instance.noDestination && t("instances.role.noDestination"),
              role === "receiver" && instance.shareReceiver && named("instances.role.shareLocation"),
              request.decidedBy === "automatic" && named("instances.role.automatic"),
            ]}
          />
        );
      case "refused":
        return named("instances.role.declined");
      case "revoked":
        return named("instances.role.revoked");
      default:
        return t(ROLE_STATE_KEY.off);
    }
  }

  /** What the key keeper's row says: where the deposit stands there, and
   *  what to do when it is not current. */
  function keeperLine(keeper: KeeperRole | null | undefined): ReactNode {
    if (!keeper) {
      if (tooOldToKeep(instance)) return named("keeper.tooOld");
      return unreachableToKeep(instance) ? named("keeper.unreachable") : t(ROLE_STATE_KEY.off);
    }
    const help = depositHelp(keeper.state);
    return (
      <Facts
        items={[
          keeper.state === "held" && named("keeper.holds"),
          t("keeper.sent").replace("{when}", () => relativeTime(t, unixOf(keeper.at))),
          help && named(help),
        ]}
      />
    );
  }

  const shownRoles = ROLE_KINDS.filter((k) => k !== "zfs" || modules.zfs || instance.roles?.byMember.zfs);
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
          {paired && !instance.app && <ForgetKeyButton memberId={instance.memberId} />}
          {modules.fleet && peer && (
            <RemoveInstanceButton
              instance={instance}
              onRemoved={() => {
                void state.reloadPeers();
                void state.reloadGroup();
                void state.reloadRoles();
                onClose();
              }}
            />
          )}
          <Button label={t("common.done")} labelKey="common.done" tone="accent" onClick={onClose} />
        </>
      }
    >
      {paired && (
        <WindowCard title={named("instances.doesForServer")} hint={t("instances.role.diff")} hueIndex={nextHue()}>
          <MeshOffers offers={instance.offers} onChanged={() => void state.reloadOffers()} />
          <ul className="flex flex-col gap-4">
            {shownRoles.map((k) => (
              <RoleRow
                key={k}
                role={k}
                state={roles.states[k]}
                onToggle={() => roles.toggle(k)}
                onChange={k === "keeper" ? undefined : () => roles.change(k)}
                action={
                  k === "receiver" &&
                  instance.roles?.byMember.receiver?.state === "allowed" &&
                  (instance.noDestination || instance.shareReceiver) && (
                    <Button
                      label={t("storage.add.title")}
                      labelKey="storage.add.title"
                      tone="neutral"
                      onClick={() => setDialog({ kind: "location" })}
                    />
                  )
                }
              >
                {(k === "receiver" || k === "fetcher") && (
                  <span className="text-xs text-carbon-textMuted wrap-anywhere">{requestLine(instance.roles?.byMember[k], k)}</span>
                )}
                {k === "keeper" && (
                  <span className="text-xs text-carbon-textMuted wrap-anywhere">{keeperLine(instance.roles?.byMember.keeper)}</span>
                )}
                {k === "zfs" && instance.replicas.length === 0 && (
                  <span className="text-xs text-carbon-textMuted">
                    {t(roles.states.zfs === "off" ? ROLE_STATE_KEY.off : "instances.role.noReplica")}
                  </span>
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
          <RoleRequests instance={instance} part="asked" state={state} onFetcherAllowed={() => setDialog({ kind: "pull", row: null })} />
          {modules.receiver && <ZFSReceiveRequests peer={memberId} only="asked" />}
          <RoleRequests instance={instance} part="allowed" state={state} onFetcherAllowed={() => undefined} />
          {modules.receiver && <ZFSReceiveRequests peer={memberId} only="allowed" />}
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

      {roles.dialogs}
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
      {dialog?.kind === "location" && <AddReceiverLocation onClose={() => setDialog(null)} />}
    </InstanceWindow>
  );
}
