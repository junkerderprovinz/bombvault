import type {
  FleetPeer,
  GroupMember,
  MemberRoles,
  MeshOffer,
  PullSourceView,
  ReceivedRepoStatus,
  RoleRequest,
  RoleSide,
  SelfRoles,
  ZFSRole,
} from "../../lib/api";

export const NOW_S = Math.floor(Date.now() / 1000);

export function fleetPeer(over: Partial<FleetPeer> = {}): FleetPeer {
  return {
    id: "p1",
    memberId: "m1",
    name: "attic",
    url: "",
    enabled: true,
    needsPairing: false,
    direct: true,
    relay: false,
    lastPollAt: NOW_S - 120,
    lastPollOk: true,
    lastPollError: "",
    lastPollInstanceName: "",
    lastPollVersion: "v9.9.1",
    lastPollDomains: [],
    createdAt: 0,
    sortOrder: 0,
    ...over,
  };
}

export function groupMember(over: Partial<GroupMember> = {}): GroupMember {
  return { id: "m1", name: "attic", version: "v9.9.1", direct: true, relay: false, address: "https://192.168.1.21:3443", ...over };
}

export function receivedRepo(over: Partial<ReceivedRepoStatus> = {}): ReceivedRepoStatus {
  return {
    id: "r1",
    name: "Attic off-site",
    repo: "/mnt/user/restic/attic",
    deadManHours: 26,
    checkCadence: "",
    readDataPercent: 0,
    lastCheckAt: 0,
    lastCheckOk: null,
    lastCheckError: "",
    lastCheckReadData: false,
    enabled: true,
    createdAt: 0,
    sortOrder: 0,
    memberId: "m1",
    needsPairing: false,
    lastReceived: "",
    snapshotCount: 0,
    reachable: true,
    ...over,
  };
}

export function pullSource(over: Partial<PullSourceView> = {}): PullSourceView {
  return {
    id: "s1",
    name: "Attic containers",
    repo: "rest:http://192.168.1.21:8000/attic",
    credsRef: "",
    domain: "containers",
    cadence: "",
    limitDownload: 0,
    limitUpload: 0,
    lastPullAt: 0,
    lastPullOk: null,
    lastPullError: "",
    snapshotsPulled: 0,
    enabled: true,
    createdAt: 0,
    sortOrder: 0,
    memberId: "m1",
    needsPairing: false,
    ...over,
  };
}

export function meshOffer(over: Partial<MeshOffer> = {}): MeshOffer {
  return {
    id: "o1",
    from: "attic",
    suggestedDomain: "vms",
    repo: "rest:http://192.168.1.21:8000/vms",
    restUser: "bv",
    status: "pending",
    receivedAt: NOW_S - 60,
    ...over,
  };
}

export function roleRequest(over: Partial<RoleRequest> = {}): RoleRequest {
  return {
    id: "rr1",
    state: "asked",
    sections: ["containers", "flash"],
    store: "rest",
    askedAt: new Date((NOW_S - 600) * 1000).toISOString(),
    decidedAt: "",
    decidedBy: "",
    ...over,
  };
}

export function zfsRole(state: ZFSRole["state"], datasets: string[] = ["tank/appdata"]): ZFSRole {
  return { state, items: datasets.map((dataset, i) => ({ id: `z${i + 1}`, dataset, state })) };
}

export function roleSide(over: Partial<RoleSide> = {}): RoleSide {
  return { receiver: null, fetcher: null, zfs: null, ...over };
}

export function memberRoles(over: Partial<MemberRoles> = {}): MemberRoles {
  return {
    memberId: "m1",
    name: "attic",
    reachable: true,
    answers: true,
    byMember: roleSide(),
    forMember: roleSide(),
    asks: 0,
    ...over,
  };
}

export const NO_SELF_ROLES: SelfRoles = { receiver: [], fetcher: [], zfs: [] };
