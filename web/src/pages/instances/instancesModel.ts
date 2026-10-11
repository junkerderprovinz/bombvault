// What the Instances page shows is spread over several lists on the server,
// each keyed by the member of the pairing group it belongs to. This file
// folds them into one record per instance.
import type {
  Destination,
  DomainStatus,
  FleetPeer,
  GroupMember,
  GroupReceiver,
  MemberRoles,
  MeshOffer,
  PullSourceView,
  ReceivedRepoStatus,
  RoleSide,
  ZFSReplica,
} from "../../lib/api";

export type RoleKind = "receiver" | "fetcher" | "zfs" | "keeper";
export type RoleState = "on" | "waiting" | "off";

/** The order the tiles and rows stand in everywhere. */
export const ROLE_KINDS: readonly RoleKind[] = ["receiver", "fetcher", "zfs", "keeper"];

/** The roles that carry backups from one instance to another. A key keeper
 *  holds one sealed file and takes no backups. */
export const FLOW_ROLES = ["receiver", "fetcher", "zfs"] as const satisfies readonly RoleKind[];

/** How often the page asks who is connected and where the roles stand. The
 *  answers come from this instance's own view of the group, so asking costs
 *  no network traffic. */
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
  /** Every member's roles in both directions, as the server keeps them. */
  roles: MemberRoles[];
  /** Null while the destinations or the group's receivers could not be read.
   *  They only name the storage location a Receiver became. */
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
  /** Reached on this network, without the relay. */
  direct: boolean;
  needsPairing: boolean;
  received: ReceivedRepoStatus[];
  pulls: PullSourceView[];
  /** Storage it offered that nobody has answered. */
  offers: MeshOffer[];
  /** Its roles in both directions. Missing for a phone, a row from before
   *  pairing and a member the group does not reach and no request names. */
  roles?: MemberRoles;
  /** The destination its receiving server became, when that role is on. */
  destination?: Destination;
  /** This server's ZFS items that replicate to it. */
  replicas: ItemReplica[];
  /** How many requests and offers wait for an answer here. */
  asks: number;
}

/** Rows no card can take: set up before pairing, or from a member this
 *  server cannot name. */
export interface Unplaced {
  received: ReceivedRepoStatus[];
  pulls: PullSourceView[];
  offers: MeshOffer[];
}

/** roleState reads one role of one direction as a tile shows it. A request
 *  the other side declined or took back counts as off, and so does a key
 *  deposit the keeper deleted. A deposit whose fresh copy has not reached the
 *  keeper waits like a request. */
export function roleState(side: RoleSide | undefined, role: RoleKind): RoleState {
  const state: string | undefined = side?.[role]?.state;
  // A deposit is with the keeper in every state but the one that says it was
  // deleted there, and only "held" says it is current.
  if (role === "keeper" && state !== undefined) return state === "held" ? "on" : state === "declined" ? "off" : "waiting";
  if (state === "asked") return "waiting";
  return state === "allowed" ? "on" : "off";
}

/** roleStates is what an instance does for this server, role by role. */
export function roleStates(instance: Instance): Record<RoleKind, RoleState> {
  const side = instance.roles?.byMember;
  return {
    receiver: roleState(side, "receiver"),
    fetcher: roleState(side, "fetcher"),
    zfs: roleState(side, "zfs"),
    keeper: roleState(side, "keeper"),
  };
}

/** tooOldToKeep says an instance in reach runs a version that keeps no key
 *  deposit. An instance out of reach has not said either way. */
export function tooOldToKeep(instance: Instance): boolean {
  return instance.roles?.keep === "too-old";
}

/** unreachableToKeep says an instance could not be asked whether it keeps a
 *  key deposit, so nothing is known about its version. */
export function unreachableToKeep(instance: Instance): boolean {
  return instance.roles?.keep === "unreachable";
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
      direct: false,
      needsPairing: false,
      received: [],
      pulls: [],
      offers: [],
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
    inst.direct = p.direct;
    inst.address = p.url;
  }
  for (const m of data.members) {
    const inst = member(m.id, m.name || m.id);
    inst.connected = inst.connected || m.direct || m.relay;
    inst.direct = inst.direct || m.direct;
    inst.app = inst.app || m.kind === "android";
    if (m.address) inst.address = m.address;
  }
  for (const r of data.roles) member(r.memberId, r.name || r.memberId).roles = r;

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
    inst.asks = (inst.roles?.asks ?? 0) + inst.offers.length;
    const offered = data.groupReceivers?.find((r) => r.memberId === inst.memberId);
    if (offered && data.destinations && roleState(inst.roles?.byMember, "receiver") === "on") {
      inst.destination = destinationAt(offered, data.destinations);
    }
  }
  return { instances, unplaced };
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
