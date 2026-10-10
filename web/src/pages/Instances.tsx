// Instances shows the pairing group as a grid of cards: this server first,
// then every paired instance and every ZFS server reached over SSH. A card
// says what the instance does for this server, and its Details window holds
// everything else about it.
import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";

import { Badge } from "../components/Badge";
import { Button } from "../components/Button";
import { InfoBubble } from "../components/InfoBubble";
import { PageTitle } from "../components/PageTitle";
import { IconEye, IconInfo, IconLink } from "../components/glyphs";
import { IconReceiver } from "../components/navGlyphs";
import { getSettings } from "../lib/api";
import type { DomainStatus, Settings } from "../lib/api";
import { useT } from "../lib/i18n";
import { PAGE_SHELL_RESPONSIVE } from "../lib/pageShell";
import { relativeTime } from "../lib/reltime";
import { useLoudAnomalies } from "../lib/useAnomalies";
import { Facts, Figure, InstanceCard } from "./instances/InstanceCard";
import { PeerDetails } from "./instances/PeerDetails";
import { ReceiverServerWindow } from "./instances/ReceiverServerWindow";
import { RemoveInstanceButton } from "./instances/RemoveInstanceButton";
import { RoleTiles } from "./instances/RoleTile";
import { UnplacedCard } from "./instances/UnplacedCard";
import { ZFSServerCards } from "./instances/ZFSServerCards";
import {
  buildInstances,
  protectionFigures,
  ROLE_KINDS,
  servedNames,
  withoutScheme,
  type Instance,
  type RoleKind,
  type RoleState,
} from "./instances/instancesModel";
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
  receiver,
  onOpenServer,
}: {
  self: Self;
  served: Record<RoleKind, string[]>;
  receiver: boolean;
  onOpenServer: () => void;
}) {
  const { t } = useT();
  const navigate = useNavigate();
  const anomalies = useLoudAnomalies().count;
  const states = Object.fromEntries(ROLE_KINDS.map((k) => [k, served[k].length > 0 ? "on" : "off"])) as Record<
    RoleKind,
    RoleState
  >;
  const details = Object.fromEntries(
    ROLE_KINDS.map((k) => [
      k,
      served[k].length > 0
        ? t("instances.forNames").replace("{names}", () => served[k].join(", "))
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
          {receiver && (
            <Button
              label={t("receiver.server.title")}
              labelKey="receiver.server.title"
              glyph={<IconReceiver />}
              tone="neutral"
              onClick={onOpenServer}
            />
          )}
        </>
      }
      index={0}
    >
      <RoleTiles heading={t("instances.serverDoesForOthers")} states={states} details={details} />
    </InstanceCard>
  );
}

function PeerCard({
  instance,
  index,
  fleet,
  state,
  onDetails,
}: {
  instance: Instance;
  index: number;
  fleet: boolean;
  state: InstancesState;
  onDetails: () => void;
}) {
  const { t } = useT();
  const { peer, name, address } = instance;
  const paired = !instance.needsPairing;
  const polled = peer?.enabled ?? true;

  const badges = (
    <>
      {instance.asks > 0 && (
        <Badge as="button" tone="active" size="large" onClick={onDetails}>
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
          {fleet && peer && <RemoveInstanceButton peerId={peer.id} onRemoved={() => void state.reloadPeers()} />}
        </>
      }
      index={index}
    >
      {paired && !instance.app && (
        <RoleTiles heading={t("instances.doesForServer").replaceAll("{peer}", () => name)} states={instance.roles} />
      )}
    </InstanceCard>
  );
}

export function Instances() {
  const { t } = useT();
  const navigate = useNavigate();
  const [settings, setSettings] = useState<Settings | null>(null);
  const [detailsOf, setDetailsOf] = useState<string | null>(null);
  const [serverOpen, setServerOpen] = useState(false);

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
  const state = useInstances(modules);
  const { instances, unplaced } = useMemo(() => buildInstances(state.data), [state.data]);
  const served = useMemo(() => servedNames(instances, state.data.logins), [instances, state.data.logins]);
  const receiver = modules?.receiver ?? false;
  // With all three modules off the page is only the way into pairing, and
  // the home of the ZFS servers if there are any.
  const grouped = modules !== null && (modules.fleet || modules.receiver || modules.pull);
  const shown = grouped ? instances : [];

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

  const openPairing = () => navigate("/settings/pairing");
  const opened = shown.find((i) => i.key === detailsOf);
  const hasUnplaced = unplaced.received.length + unplaced.pulls.length + unplaced.offers.length > 0;

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
        <div className="rounded-card bg-carbon-surface p-3 md:p-6 glim-content-fade">
          <div className="grid grid-cols-[repeat(auto-fill,minmax(min(100%,24rem),1fr))] items-stretch gap-4">
            {grouped && state.self && (
              <SelfCard self={state.self} served={served} receiver={receiver} onOpenServer={() => setServerOpen(true)} />
            )}
            {shown.map((inst, i) => (
              <PeerCard
                key={inst.key}
                instance={inst}
                index={i + 1}
                fleet={modules.fleet}
                state={state}
                onDetails={() => setDetailsOf(inst.key)}
              />
            ))}
            {shown.length === 0 && (
              <div
                data-instance="none"
                className="flex flex-col items-center justify-center gap-3 rounded-card border-[1.5px] border-dashed border-carbon-border px-5 py-7 text-center"
              >
                <b className="font-semibold text-carbon-text">{t("instances.emptyTitle")}</b>
                <p className="text-sm text-carbon-textSub">{t("instances.emptyLead")}</p>
                <Button label={t("instances.pair")} labelKey="instances.pair" glyph={<IconLink />} tone="accent" onClick={openPairing} />
              </div>
            )}
            {modules.zfs && <ZFSServerCards replicas={state.data.replicas} firstIndex={shown.length + 1} />}
          </div>
        </div>
      )}

      {grouped && !state.loading && hasUnplaced && <UnplacedCard unplaced={unplaced} state={state} hueIndex={0} />}

      {modules && !state.loading && shown.length > 0 && (
        <div className="flex justify-end">
          <Button label={t("instances.pair")} labelKey="instances.pair" glyph={<IconLink />} tone="accent" onClick={openPairing} />
        </div>
      )}

      {opened && modules && (
        <PeerDetails instance={opened} modules={modules} state={state} onClose={() => setDetailsOf(null)} />
      )}
      {serverOpen && (
        <ReceiverServerWindow
          t={t}
          onClose={() => {
            setServerOpen(false);
            void state.reloadLogins();
          }}
        />
      )}
    </div>
  );
}
