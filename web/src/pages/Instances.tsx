// Instances shows the pairing group: a picture of who does what for whom,
// then a grid of cards with this server first, every paired instance and
// every ZFS server reached over SSH. The picture and the cards stand on the
// page itself, since a card around them would frame each of them twice. A
// card's tiles switch what the instance does for this server, and its Details
// window holds everything else.
import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";

import { Badge } from "../components/Badge";
import { Button } from "../components/Button";
import { InfoBubble } from "../components/InfoBubble";
import { PageAction, PageActions } from "../components/PageActions";
import { PageTitle } from "../components/PageTitle";
import { IconEye, IconInfo, IconKey, IconLink } from "../components/glyphs";
import { IconReceiver } from "../components/navGlyphs";
import { useReplicaServers } from "../components/zfs/replica/replicaStore";
import { getSettings } from "../lib/api";
import type { DomainStatus, RoleSection, SelfRoles, Settings } from "../lib/api";
import { useT } from "../lib/i18n";
import { PAGE_SHELL_RESPONSIVE } from "../lib/pageShell";
import { relativeTime } from "../lib/reltime";
import { useLoudAnomalies } from "../lib/useAnomalies";
import { Facts, Figure, InstanceCard, PhoneMark } from "./instances/InstanceCard";
import { NetPicture, useNetHover } from "./instances/NetPicture";
import { KeyDepositWindow } from "./instances/KeyDepositWindow";
import { PeerDetails } from "./instances/PeerDetails";
import { ReceiverServerWindow } from "./instances/ReceiverServerWindow";
import { RemoveInstanceButton } from "./instances/RemoveInstanceButton";
import { useRoleSwitch } from "./instances/RoleDialogs";
import { depositStateLabel, RoleTiles } from "./instances/RoleTile";
import { UnplacedCard } from "./instances/UnplacedCard";
import { ZFSServerCards } from "./instances/ZFSServerCards";
import { buildInstances, protectionFigures, ROLE_KINDS, withoutScheme, type Instance, type RoleKind, type RoleState } from "./instances/instancesModel";
import { groupNodes } from "./instances/netModel";
import { useInstances, type InstancesState, type Modules, type Self } from "./instances/useInstances";

/** The figures under an instance's name. Only this server knows its open
 *  anomalies; a scorecard from another instance does not carry them. */
function ProtectionFigures({ domains, anomalies }: { domains: DomainStatus[]; anomalies?: number }) {
  const { t } = useT();
  const { protectedCount, total, lastBackup } = protectionFigures(domains);
  return (
    <>
      {total > 0 && <Figure value={`${protectedCount} / ${total}`} label={t("fleet.protection.green")} />}
      {anomalies !== undefined && <Figure value={anomalies} label={t("nav.anomalies")} />}
      {lastBackup > 0 && <Figure value={relativeTime(t, lastBackup)} label={t("containers.lastBackup")} />}
    </>
  );
}

function SelfCard({
  self,
  served,
  receiverServer,
  hot,
  onOpenServer,
  onOpenDeposits,
}: {
  self: Self;
  served: SelfRoles;
  receiverServer: boolean;
  hot: boolean;
  onOpenServer: () => void;
  /** Opens this server's key deposits. Missing while there are none. */
  onOpenDeposits?: () => void;
}) {
  const { t } = useT();
  const navigate = useNavigate();
  const anomalies = useLoudAnomalies().count;
  const holders = (k: RoleKind) => served[k] ?? [];
  const states = Object.fromEntries(ROLE_KINDS.map((k) => [k, holders(k).length > 0 ? "on" : "off"])) as Record<RoleKind, RoleState>;
  const details = Object.fromEntries(
    ROLE_KINDS.map((k) => [
      k,
      holders(k).length > 0
        ? t("instances.forNames").replace("{names}", () => holders(k).map((holder) => holder.name || holder.memberId).join(", "))
        : t("instances.forNoOne"),
    ]),
  ) as Record<RoleKind, string>;

  return (
    <InstanceCard
      cardKey="self"
      name={self.name}
      eyebrow={t("instances.thisInstance")}
      facts={
        <Facts items={[self.address && <bdi dir="ltr">{withoutScheme(self.address)}</bdi>, self.version]} />
      }
      figures={<ProtectionFigures domains={self.domains} anomalies={anomalies} />}
      actions={
        <>
          <Button
            label={t("instances.open")}
            labelKey="instances.open"
            glyph={<IconEye />}
            tone="neutral"
            onClick={() => navigate("/dashboard")}
          />
          {receiverServer && (
            <Button
              label={t("receiver.server.title")}
              labelKey="receiver.server.title"
              glyph={<IconReceiver />}
              tone="neutral"
              onClick={onOpenServer}
            />
          )}
          {onOpenDeposits && (
            <Button
              label={t("keeper.window.title")}
              labelKey="keeper.window.title"
              glyph={<IconKey />}
              tone="neutral"
              onClick={onOpenDeposits}
            />
          )}
        </>
      }
      index={0}
      hot={hot}
    >
      <RoleTiles heading={t("instances.serverDoesForOthers")} states={states} details={details} />
    </InstanceCard>
  );
}

function PeerCard({
  instance,
  peers,
  index,
  modules,
  sections,
  state,
  hot,
  onDetails,
}: {
  instance: Instance;
  peers: Instance[];
  index: number;
  modules: Modules;
  sections: RoleSection[];
  state: InstancesState;
  hot: boolean;
  onDetails: () => void;
}) {
  const { t } = useT();
  const { peer, name, address } = instance;
  const paired = !instance.needsPairing;
  const polled = peer?.enabled ?? true;
  const roles = useRoleSwitch({ instance, peers, sections, state });
  const tiles: Partial<Record<RoleKind, RoleState>> = { ...roles.states };
  if (!modules.zfs && !instance.roles?.byMember.zfs) delete tiles.zfs;
  // A deposit that is not current says so in place of the tile's own word.
  const deposit = instance.roles?.byMember.keeper?.state;
  const details = deposit && deposit !== "held" ? { keeper: depositStateLabel(t, deposit) } : undefined;

  const badges = (
    <>
      {instance.asks > 0 && (
        <Badge tone="warn" size="large">
          {t("instances.requests", instance.asks)}
        </Badge>
      )}
      {!paired ? (
        <Badge tone="warn" size="large">
          {t("pairing.pairAgain")}
          <InfoBubble tip={t("fleet.pairAgainTip")} onAccent />
        </Badge>
      ) : !polled ? (
        <Badge tone="neutral" size="large">
          {t("fleet.monitoringOff")}
        </Badge>
      ) : (
        <Badge tone={instance.connected ? "ok" : "fail"} size="large">
          {instance.connected ? t("instances.connected") : t("instances.notConnected")}
        </Badge>
      )}
    </>
  );

  return (
    <InstanceCard
      cardKey={instance.key}
      name={name}
      eyebrow={instance.app ? t("fleet.kindAndroid") : undefined}
      mark={instance.app ? <PhoneMark /> : undefined}
      badges={badges}
      facts={
        <Facts
          items={[
            address && <bdi dir="ltr">{withoutScheme(address)}</bdi>,
            // api.Version already starts with "v".
            peer?.lastPollVersion,
            peer && peer.lastPollAt > 0 && t("fleet.lastPolled").replace("{time}", relativeTime(t, peer.lastPollAt)),
            peer?.lastPollOk === false && peer.lastPollError && (
              <span className="text-statusFail">{peer.lastPollError}</span>
            ),
          ]}
        />
      }
      // A phone backs nothing up, so its card has no figures.
      figures={peer && !instance.app && <ProtectionFigures domains={peer.lastPollDomains} />}
      actions={
        <>
          {address && paired && !instance.app && (
            <Button
              label={t("instances.open")}
              labelKey="instances.open"
              glyph={<IconEye />}
              tone="neutral"
              onClick={() => window.open(address, "_blank", "noopener")}
            />
          )}
          {paired && !instance.app && (
            <Button label={t("fleet.details")} labelKey="fleet.details" glyph={<IconInfo />} tone="neutral" onClick={onDetails} />
          )}
          {modules.fleet && peer && <RemoveInstanceButton peerId={peer.id} onRemoved={() => void state.reloadPeers()} />}
        </>
      }
      index={index}
      hot={hot}
    >
      {paired && !instance.app && (
        <RoleTiles
          heading={t("instances.doesForServer").replaceAll("{peer}", () => name)}
          states={tiles}
          details={details}
          onToggle={roles.toggle}
        />
      )}
      {roles.dialogs}
    </InstanceCard>
  );
}

/** The sections this server backs up, in the order the app lists them. */
function backedUpSections(settings: Settings): RoleSection[] {
  const on: [RoleSection, boolean][] = [
    ["containers", settings.containersEnabled],
    ["vms", settings.vmsEnabled],
    ["files", settings.filesEnabled],
    ["flash", settings.flashEnabled],
    ["zfs", settings.zfsEnabled],
    ["config", settings.configEnabled],
  ];
  return on.filter(([, enabled]) => enabled).map(([section]) => section);
}

export function Instances() {
  const { t } = useT();
  const navigate = useNavigate();
  const [settings, setSettings] = useState<Settings | null>(null);
  const [detailsOf, setDetailsOf] = useState<string | null>(null);
  const [serverOpen, setServerOpen] = useState(false);
  const [depositsOpen, setDepositsOpen] = useState(false);
  const { hot, handlers } = useNetHover();

  useEffect(() => {
    getSettings()
      .then((res) => {
        if (res.ok && res.settings) setSettings(res.settings);
      })
      .catch(() => undefined);
  }, []);

  const modules = useMemo<Modules | null>(
    () =>
      settings && {
        fleet: settings.fleetEnabled,
        receiver: settings.receiverEnabled,
        pull: settings.pullEnabled,
        zfs: settings.zfsEnabled,
      },
    [settings],
  );
  const sections = useMemo(() => (settings ? backedUpSections(settings) : []), [settings]);
  const state = useInstances(modules);
  const { servers: zfsServers } = useReplicaServers();
  const { instances, unplaced } = useMemo(() => buildInstances(state.data), [state.data]);
  const receiver = modules?.receiver ?? false;
  // With all three modules off the page is only the way into pairing, and
  // the home of the ZFS servers if there are any.
  const grouped = modules !== null && (modules.fleet || modules.receiver || modules.pull);
  const shown = grouped ? instances : [];
  const servers = shown.filter((i) => !i.app);
  const phones = shown.filter((i) => i.app);
  const zfs = modules?.zfs ?? false;
  const zfsCards = zfs ? zfsServers.length : 0;
  const nodes = useMemo(
    () => (grouped ? groupNodes(instances, zfs ? zfsServers : [], t) : []),
    [grouped, instances, zfsServers, zfs, t],
  );

  // Bookmarks and support answers carry these hashes. Pairing lives in
  // Settings, #receiver means the receiving server, and the other two mean
  // the grid.
  useEffect(() => {
    const hash = window.location.hash;
    if (hash === "#pairing") {
      navigate("/settings/pairing", { replace: true });
      return;
    }
    if (hash !== "#receiver" && hash !== "#pull" && hash !== "#fleet") return;
    if (hash === "#receiver") {
      if (!receiver) return;
      setServerOpen(true);
    }
    window.history.replaceState(null, "", window.location.pathname);
  }, [navigate, receiver]);

  const opened = shown.find((i) => i.key === detailsOf);
  const { deposits } = state;
  const hasDeposits = deposits.codeSince !== "" || deposits.sent.length + deposits.kept.length + deposits.releases.length > 0;
  const hasUnplaced = unplaced.received.length + unplaced.pulls.length + unplaced.offers.length > 0;

  const peerCard = (inst: Instance, index: number) =>
    modules && (
      <PeerCard
        key={inst.key}
        instance={inst}
        peers={shown}
        index={index}
        modules={modules}
        sections={sections}
        state={state}
        hot={hot === inst.key}
        onDetails={() => setDetailsOf(inst.key)}
      />
    );

  return (
    <div className={PAGE_SHELL_RESPONSIVE}>
      <PageTitle>{t("instances.title")}</PageTitle>

      {state.loading && modules && <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>}
      {state.errors.map((error) => (
        <p key={error} className="text-sm text-statusFail wrap-break-word">
          {error}
        </p>
      ))}

      {modules && !state.loading && (
        <div className="grid grid-cols-[repeat(auto-fill,minmax(min(100%,24rem),1fr))] items-stretch gap-4 glim-content-fade" {...handlers}>
          {grouped && state.self && nodes.length > 0 && (
            <div className="col-span-full rounded-card bg-carbon-surface px-3 py-3 md:px-5 md:py-4 [--net-ground:var(--carbon-surface)]">
              <NetPicture selfName={state.self.name} nodes={nodes} hot={hot} />
            </div>
          )}
          {grouped && state.self && (
            <SelfCard
              self={state.self}
              served={state.selfRoles}
              receiverServer={state.receiverServer}
              hot={hot === "self"}
              onOpenServer={() => setServerOpen(true)}
              onOpenDeposits={hasDeposits ? () => setDepositsOpen(true) : undefined}
            />
          )}
          {servers.map((inst, i) => peerCard(inst, i + 1))}
          {shown.length === 0 && (
            <div
              data-instance="none"
              className="flex flex-col items-center justify-center gap-3 rounded-card border-[1.5px] border-dashed border-carbon-border px-5 py-7 text-center"
            >
              <b className="font-semibold text-carbon-text">{t("instances.emptyTitle")}</b>
              <p className="text-sm text-carbon-textSub">{t("instances.emptyLead")}</p>
            </div>
          )}
          {modules.zfs && <ZFSServerCards replicas={state.data.replicas} firstIndex={servers.length + 1} hot={hot} />}
          {phones.map((inst, i) => peerCard(inst, servers.length + zfsCards + i + 1))}
        </div>
      )}

      {/* The hue goes on from the last card of the grid. */}
      {grouped && !state.loading && hasUnplaced && (
        <UnplacedCard unplaced={unplaced} state={state} hueIndex={1 + servers.length + zfsCards + phones.length} />
      )}

      {opened && modules && (
        <PeerDetails
          instance={opened}
          peers={shown}
          modules={modules}
          sections={sections}
          state={state}
          onClose={() => setDetailsOf(null)}
        />
      )}
      {serverOpen && (
        <ReceiverServerWindow
          t={t}
          onClose={() => {
            setServerOpen(false);
            void state.reloadServer();
          }}
        />
      )}
      {depositsOpen && <KeyDepositWindow state={state} onClose={() => setDepositsOpen(false)} />}
      {modules && !state.loading && (
        <PageActions>
          <span data-new="instances-pair" className="flex">
            <PageAction primary label={t("instances.pair")} glyph={<IconLink />} onClick={() => navigate("/settings/pairing")} />
          </span>
        </PageActions>
      )}
    </div>
  );
}
