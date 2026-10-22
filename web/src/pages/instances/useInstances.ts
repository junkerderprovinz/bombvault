import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { useReceiveRequests } from "../../components/zfs/replica/replicaStore";
import {
  getGroup,
  getHealth,
  getReceiverServer,
  getStatus,
  getZFSReplica,
  listDeposits,
  listDestinations,
  listFleetPeers,
  listGroupReceivers,
  listMeshOffers,
  listPullSources,
  listReceivedRepos,
  listRoles,
  listZFSDatasets,
  pollFleetPeer,
} from "../../lib/api";
import type {
  Deposits,
  Destination,
  DomainStatus,
  FleetPeer,
  AwayMember,
  GroupMember,
  GroupReceiver,
  MemberRoles,
  MeshOffer,
  PullSourceView,
  ReceivedRepoStatus,
  SelfRoles,
} from "../../lib/api";
import { useT } from "../../lib/i18n";
import { FLEET_REFRESH_MS, scorecardDue, type InstancesData, type ItemReplica } from "./instancesModel";

/** Which of the modules behind the page are switched on. */
export interface Modules {
  fleet: boolean;
  receiver: boolean;
  pull: boolean;
  zfs: boolean;
}

export interface Self {
  name: string;
  address: string;
  version: string;
  domains: DomainStatus[];
}

export interface InstancesState {
  loading: boolean;
  /** One line per list that could not be read. */
  errors: string[];
  self: Self | null;
  /** For whom this server is Receiver, Fetcher and ZFS server. */
  selfRoles: SelfRoles;
  /** This server runs a receiving server. */
  receiverServer: boolean;
  /** Both sides of the key deposits: what this server gave its keepers and
   *  what it keeps for others. */
  deposits: Deposits;
  data: InstancesData;
  /** Reads who is in the group and who is away. */
  reloadGroup: () => Promise<void>;
  reloadPeers: () => Promise<void>;
  reloadOffers: () => Promise<void>;
  reloadReceived: () => Promise<void>;
  reloadPulls: () => Promise<void>;
  reloadRoles: () => Promise<void>;
  /** Reads the deposits and, since they are roles too, the roles. */
  reloadDeposits: () => Promise<void>;
  reloadReplicas: () => Promise<void>;
  reloadServer: () => Promise<void>;
}

const NO_ROLES: SelfRoles = { receiver: [], fetcher: [], zfs: [], keeper: [] };
const NO_DEPOSITS: Deposits = { codeSince: "", sent: [], kept: [], releases: [] };

function message(err: unknown, fallback: string): string {
  return err instanceof Error ? err.message : fallback;
}

/** useInstances reads what the modules that are on know about the group and
 *  keeps it fresh while the page is open. Asking who is connected and where
 *  the roles stand costs no network traffic. A member's scorecard is fetched
 *  over the group only once it has gone stale, since that call may cross a
 *  relay. */
export function useInstances(modules: Modules | null): InstancesState {
  const { t } = useT();
  const { requests } = useReceiveRequests();
  const [loading, setLoading] = useState(true);
  const [self, setSelf] = useState<Self | null>(null);
  const [members, setMembers] = useState<GroupMember[]>([]);
  const [away, setAway] = useState<AwayMember[]>([]);
  const [settling, setSettling] = useState(false);
  const [peers, setPeers] = useState<FleetPeer[]>([]);
  const [received, setReceived] = useState<ReceivedRepoStatus[]>([]);
  const [pulls, setPulls] = useState<PullSourceView[]>([]);
  const [offers, setOffers] = useState<MeshOffer[]>([]);
  const [roles, setRoles] = useState<MemberRoles[]>([]);
  const [selfRoles, setSelfRoles] = useState<SelfRoles>(NO_ROLES);
  const [receiverServer, setReceiverServer] = useState(false);
  const [deposits, setDeposits] = useState<Deposits>(NO_DEPOSITS);
  const [destinations, setDestinations] = useState<Destination[] | null>(null);
  const [groupReceivers, setGroupReceivers] = useState<GroupReceiver[] | null>(null);
  const [replicas, setReplicas] = useState<ItemReplica[]>([]);
  const [failed, setFailed] = useState<Record<string, string>>({});
  const polling = useRef(new Set<string>());

  const ready = modules !== null;
  const fleet = modules?.fleet ?? false;
  const receiver = modules?.receiver ?? false;
  const pull = modules?.pull ?? false;
  const zfs = modules?.zfs ?? false;

  const note = useCallback((list: string, error: string | null) => {
    setFailed((prev) => {
      if ((prev[list] ?? null) === error) return prev;
      const next = { ...prev };
      if (error === null) delete next[list];
      else next[list] = error;
      return next;
    });
  }, []);

  const loadSelf = useCallback(
    (): Promise<void> =>
      Promise.all([getGroup(), getHealth(), getStatus()])
        .then(([group, health, status]) => {
          setSelf({
            name: group.name,
            address: group.selfAddress,
            version: health.version ?? "",
            domains: status.domains ?? [],
          });
          setMembers(group.members);
          setAway(group.away);
          setSettling(group.settling);
        })
        .catch(() => undefined),
    [],
  );

  const reloadPeers = useCallback(
    (): Promise<void> =>
      listFleetPeers()
        .then((res) => {
          if (res.ok) setPeers(res.peers ?? []);
          note("peers", res.ok ? null : (res.error ?? t("fleet.loadError")));
        })
        .catch((err) => note("peers", message(err, t("fleet.loadError")))),
    [note, t],
  );

  const reloadOffers = useCallback(
    (): Promise<void> =>
      listMeshOffers()
        .then((res) => {
          if (res.ok) setOffers(res.offers ?? []);
        })
        .catch(() => undefined),
    [],
  );

  const reloadReceived = useCallback(
    (): Promise<void> =>
      listReceivedRepos()
        .then((res) => {
          if (res.ok) setReceived(res.repos ?? []);
          note("received", res.ok ? null : (res.error ?? t("receiver.loadError")));
        })
        .catch((err) => note("received", message(err, t("receiver.loadError")))),
    [note, t],
  );

  const reloadPulls = useCallback(
    (): Promise<void> =>
      listPullSources()
        .then((res) => {
          if (res.ok) setPulls(res.sources ?? []);
          note("pulls", res.ok ? null : (res.error ?? t("pull.saveError")));
        })
        .catch((err) => note("pulls", message(err, t("pull.saveError")))),
    [note, t],
  );

  const reloadServer = useCallback(
    (): Promise<void> =>
      getReceiverServer()
        .then((res) => {
          if (res.ok) setReceiverServer(Boolean(res.server));
        })
        .catch(() => undefined),
    [],
  );

  const reloadRoles = useCallback(
    (): Promise<void> =>
      listRoles()
        .then((res) => {
          if (res.ok) {
            setRoles(res.members ?? []);
            setSelfRoles(res.self ?? NO_ROLES);
          }
          note("roles", res.ok ? null : (res.error ?? t("instances.role.loadError")));
        })
        .catch((err) => note("roles", message(err, t("instances.role.loadError")))),
    [note, t],
  );

  const reloadDeposits = useCallback(
    (): Promise<void> =>
      Promise.all([
        listDeposits()
          .then((res) => {
            if (res.ok && res.deposits) setDeposits(res.deposits);
          })
          .catch(() => undefined),
        reloadRoles(),
      ]).then(() => undefined),
    [reloadRoles],
  );

  const reloadReplicas = useCallback(
    (): Promise<void> =>
      listZFSDatasets()
        .then((res) =>
          Promise.all(
            (res.datasets ?? []).map((d) =>
              getZFSReplica(d.id)
                .then((replica): ItemReplica => ({ itemId: d.id, dataset: d.dataset, replica }))
                .catch(() => null),
            ),
          ),
        )
        .then((found) => setReplicas(found.filter((r) => r !== null)))
        .catch(() => undefined),
    [],
  );

  useEffect(() => {
    if (!ready) return;
    const loadDestinations = Promise.all([listDestinations(), listGroupReceivers()])
      .then(([dest, recv]) => {
        setDestinations(dest.ok ? (dest.destinations ?? []) : null);
        setGroupReceivers(recv.ok ? (recv.receivers ?? []) : null);
      })
      .catch(() => undefined);
    void Promise.all([
      loadSelf(),
      loadDestinations,
      reloadDeposits(),
      zfs ? reloadReplicas() : undefined,
      fleet ? reloadPeers() : undefined,
      fleet ? reloadOffers() : undefined,
      receiver ? reloadReceived() : undefined,
      receiver ? reloadServer() : undefined,
      pull ? reloadPulls() : undefined,
    ]).finally(() => setLoading(false));

    const refresh = () => {
      if (document.visibilityState !== "visible") return;
      void loadSelf();
      void reloadDeposits();
      if (fleet) void reloadPeers();
    };
    const id = setInterval(refresh, FLEET_REFRESH_MS);
    document.addEventListener("visibilitychange", refresh);
    return () => {
      clearInterval(id);
      document.removeEventListener("visibilitychange", refresh);
    };
  }, [ready, fleet, receiver, pull, zfs, loadSelf, reloadPeers, reloadOffers, reloadReceived, reloadServer, reloadPulls, reloadDeposits, reloadReplicas]);

  // An answer to a ZFS request changes the roles the server reports.
  const answers = requests.map((r) => `${r.id}:${r.state}`).join(" ");
  const seen = useRef(answers);
  useEffect(() => {
    if (seen.current === answers) return;
    seen.current = answers;
    void reloadRoles();
  }, [answers, reloadRoles]);

  // A failure lands in the member's row, which the reload then shows.
  useEffect(() => {
    if (!fleet) return;
    const nowS = Date.now() / 1000;
    for (const p of peers) {
      if (!scorecardDue(p, nowS) || polling.current.has(p.id)) continue;
      polling.current.add(p.id);
      void pollFleetPeer(p.id)
        .catch(() => undefined)
        .finally(() => {
          polling.current.delete(p.id);
          void reloadPeers();
        });
    }
  }, [fleet, peers, reloadPeers]);

  const data = useMemo<InstancesData>(
    () => ({
      members,
      away,
      settling,
      peers: fleet ? peers : [],
      received: receiver ? received : [],
      pulls: pull ? pulls : [],
      offers: fleet ? offers : [],
      roles,
      destinations,
      groupReceivers,
      replicas: zfs ? replicas : [],
    }),
    [members, away, settling, peers, received, pulls, offers, roles, destinations, groupReceivers, replicas, fleet, receiver, pull, zfs],
  );

  const errors = useMemo(() => Object.values(failed), [failed]);
  return {
    loading,
    errors,
    self,
    selfRoles,
    receiverServer: receiver && receiverServer,
    deposits,
    data,
    reloadGroup: loadSelf,
    reloadPeers,
    reloadOffers,
    reloadReceived,
    reloadPulls,
    reloadRoles,
    reloadDeposits,
    reloadReplicas,
    reloadServer,
  };
}
