// What the Instances page shows is spread over six lists on the server, each
// keyed by the member of the pairing group it belongs to. This file folds
// them into one record per instance and says which roles are known.
import type {
  Destination,
  DomainStatus,
  FleetPeer,
  GroupMember,
  GroupReceiver,
  MeshOffer,
  PullSourceView,
  ReceivedRepoStatus,
  ReceiverLogin,
  ZFSReceiveRequest,
  ZFSReplica,
} from "../../lib/api";

export type RoleKind = "receiver" | "fetcher" | "zfs";
export type RoleState = "on" | "waiting" | "off";

/** The order the tiles and rows stand in everywhere. */
export const ROLE_KINDS: readonly RoleKind[] = ["receiver", "fetcher", "zfs"];

/** How often the page asks who is connected. The answer comes from this
 *  instance's own view of the group, so asking costs no network traffic. */
export const FLEET_REFRESH_MS = 20_000;

/** How old a connected member's scorecard may get before the page fetches it
 *  again over the group. */
export const SCORECARD_STALE_S = 15 * 60;

/** scorecardDue says whether the page should fetch a member's scorecard now. */
export function scorecardDue(peer: FleetPeer, nowS: number): boolean {
  return peer.enabled && !peer.needsPairing && peer.kind !== "android" && (peer.direct || peer.relay) && nowS - peer.lastPollAt >= SCORECARD_STALE_S;
}

/** One ZFS item of this server and where its replica stands. */
export interface ItemReplica {
  itemId: string;
  dataset: string;
  replica: ZFSReplica;
}

/** Everything the page loaded. A list a switched-off module owns is empty. */
export interface InstancesData {
  members: GroupMember[];
  peers: FleetPeer[];
  received: ReceivedRepoStatus[];
  pulls: PullSourceView[];
  offers: MeshOffer[];
  requests: ZFSReceiveRequest[];
  logins: ReceiverLogin[];
  /** Null while the destinations or the group's receivers could not be read,
   *  which leaves the receiver role unknown. */
  destinations: Destination[] | null;
  groupReceivers: GroupReceiver[] | null;
  replicas: ItemReplica[];
}

export interface Instance {
  /** The member id, or the fleet row's id for a row from before pairing. */
  key: string;
  memberId: string;
  name: string;
  /** Where the instance takes direct calls, empty when it is not known. */
  address: string;
  app: boolean;
  /** The fleet row, present while the Instances module is on. */
  peer?: FleetPeer;
  connected: boolean;
  needsPairing: boolean;
  received: ReceivedRepoStatus[];
  pulls: PullSourceView[];
  /** The ZFS replicas it asked to send here and the ones allowed. */
  requests: ZFSReceiveRequest[];
  /** Storage it offered that nobody has answered. */
  offers: MeshOffer[];
  /** What it does for this server. A role is left out when no list says. */
  roles: Partial<Record<RoleKind, RoleState>>;
  /** The destination its receiving server became, when the role is on. */
  destination?: Destination;
  /** This server's ZFS items that replicate to it. */
  replicas: ItemReplica[];
  /** How many requests wait for an answer here. */
  asks: number;
}

/** Rows no card can take: set up before pairing, or from a member this
 *  server cannot name. */
export interface Unplaced {
  received: ReceivedRepoStatus[];
  pulls: PullSourceView[];
  offers: MeshOffer[];
}

function restAddress(location: string): string {
  return location.replace(/^rest:/, "").replace(/^(https?:\/\/)[^@/]*@/, "$1").replace(/\/+$/, "");
}

/** destinationAt finds the destination whose repository lies on a member's
 *  receiving server. A group login points at <server>/<user>, so the server's
 *  address is a prefix of the destination's. */
function destinationAt(receiver: GroupReceiver, destinations: Destination[]): Destination | undefined {
  if (!receiver.url) return undefined;
  const base = restAddress(receiver.url);
  return destinations.find((d) => {
    const repo = restAddress(d.repo);
    return repo === base || repo.startsWith(base + "/");
  });
}

function receiverRole(
  memberId: string,
  connected: boolean,
  data: InstancesData,
): { state?: RoleState; destination?: Destination } {
  if (!data.destinations || !data.groupReceivers) return {};
  const offered = data.groupReceivers.find((r) => r.memberId === memberId);
  if (offered?.url) {
    const destination = destinationAt(offered, data.destinations);
    return { state: destination ? "on" : "off", destination };
  }
  // A member this server reaches and that offers no receiving server cannot
  // hold a copy. One it does not reach, or reaches only over the relay, may.
  return connected && !offered ? { state: "off" } : {};
}

function zfsRole(replicas: ItemReplica[]): RoleState {
  if (replicas.some((r) => r.replica.peerState === "asked")) return "waiting";
  return replicas.some((r) => r.replica.peerState === "allowed") ? "on" : "off";
}

export function buildInstances(data: InstancesData): { instances: Instance[]; unplaced: Unplaced } {
  const byMember = new Map<string, Instance>();
  const legacy: Instance[] = [];

  function blank(key: string, memberId: string, name: string): Instance {
    return {
      key,
      memberId,
      name,
      address: "",
      app: false,
      connected: false,
      needsPairing: false,
      received: [],
      pulls: [],
      requests: [],
      offers: [],
      roles: {},
      replicas: [],
      asks: 0,
    };
  }
  function member(id: string, name: string): Instance {
    let found = byMember.get(id);
    if (!found) {
      found = blank(id, id, name);
      byMember.set(id, found);
    }
    return found;
  }

  for (const p of data.peers) {
    const name = p.lastPollInstanceName || p.name;
    if (!p.memberId) {
      legacy.push({ ...blank(p.id, "", name), peer: p, address: p.url, needsPairing: true, app: p.kind === "android" });
      continue;
    }
    const inst = member(p.memberId, name);
    inst.peer = p;
    inst.needsPairing = p.needsPairing;
    inst.app = p.kind === "android";
    inst.connected = p.direct || p.relay;
    inst.address = p.url;
  }
  for (const m of data.members) {
    const inst = member(m.id, m.name || m.id);
    inst.connected = inst.connected || m.direct || m.relay;
    inst.app = inst.app || m.kind === "android";
    if (m.address) inst.address = m.address;
  }
  for (const l of data.logins) member(l.memberId, l.name);
  for (const r of data.requests) {
    if (r.state !== "asked" && r.state !== "allowed") continue;
    member(r.peer, r.peerName || r.sourceServer || r.peer).requests.push(r);
  }

  const unplaced: Unplaced = { received: [], pulls: [], offers: [] };
  for (const r of data.received) {
    const inst = r.needsPairing ? undefined : byMember.get(r.memberId);
    if (inst) inst.received.push(r);
    else unplaced.received.push(r);
  }
  for (const s of data.pulls) {
    const inst = s.needsPairing ? undefined : byMember.get(s.memberId);
    if (inst) inst.pulls.push(s);
    else unplaced.pulls.push(s);
  }

  const instances = [...byMember.values(), ...legacy];
  // An offer names its sender only by the name it gives itself, so it goes to
  // a card when exactly one instance carries that name.
  for (const o of data.offers) {
    if (o.status !== "pending") continue;
    const named = o.from ? instances.filter((i) => i.name === o.from && !i.needsPairing) : [];
    if (named.length === 1) named[0].offers.push(o);
    else unplaced.offers.push(o);
  }

  for (const inst of byMember.values()) {
    inst.replicas = data.replicas.filter((r) => r.replica.target.kind === "peer" && r.replica.target.id === inst.memberId);
    inst.asks = inst.requests.filter((r) => r.state === "asked").length + inst.offers.length;
    if (inst.app || inst.needsPairing) continue;
    const receiver = receiverRole(inst.memberId, inst.connected, data);
    if (receiver.state) inst.roles.receiver = receiver.state;
    inst.destination = receiver.destination;
    inst.roles.zfs = zfsRole(inst.replicas);
  }
  return { instances, unplaced };
}

/** servedNames lists, per role, the instances this server plays it for. */
export function servedNames(instances: Instance[], logins: ReceiverLogin[]): Record<RoleKind, string[]> {
  const withLogin = new Set(logins.map((l) => l.memberId));
  const names = (keep: (i: Instance) => boolean) => instances.filter(keep).map((i) => i.name);
  return {
    receiver: names((i) => i.received.length > 0 || withLogin.has(i.memberId)),
    fetcher: names((i) => i.pulls.length > 0),
    zfs: names((i) => i.requests.some((r) => r.state === "allowed")),
  };
}

export interface ProtectionFigures {
  /** Sections whose protection is green, and sections that report any. */
  protectedCount: number;
  total: number;
  /** Unix seconds of the newest successful backup of any section, 0 for none. */
  lastBackup: number;
}

export function protectionFigures(domains: DomainStatus[]): ProtectionFigures {
  const shown = domains.filter((d) => d.enabled && d.protection !== "");
  return {
    protectedCount: shown.filter((d) => d.protection === "green").length,
    total: shown.length,
    lastBackup: shown.reduce((latest, d) => Math.max(latest, d.lastSuccess), 0),
  };
}

/** The address as a card shows it, the way it would be typed. */
export function withoutScheme(url: string): string {
  return url.replace(/^https?:\/\//, "");
}
