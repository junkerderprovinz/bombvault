import type { GroupState, ZFSReplica, ZFSReplicaServer } from "../../../lib/api";

export function replica(overrides: Partial<ZFSReplica> = {}): ZFSReplica {
  return {
    target: { kind: "server", id: "nas" },
    afterBackup: true,
    cadence: "",
    keep: { preset: "own", own: [0, 7, 3, 0, 0] },
    state: "ok",
    code: "",
    lastRun: new Date(Date.now() - 2 * 3600 * 1000).toISOString(),
    lastBytes: 118 * 1024 * 1024,
    lastSeconds: 42,
    snapshots: [
      { name: "bombvault-replica-20261006014100", created: "2026-10-06T01:41:00Z" },
      { name: "bombvault-replica-20261005014000", created: "2026-10-05T01:40:00Z" },
    ],
    members: [
      {
        dataset: "cache/appdata",
        volume: false,
        targetPath: "backup/bombvault-replica/tower/cache/appdata",
        state: "ok",
        code: "",
        lastBytes: 31 * 1024 * 1024,
      },
      {
        dataset: "cache/appdata/vm-disk",
        volume: true,
        targetPath: "backup/bombvault-replica/tower/cache/appdata/vm-disk",
        state: "ok",
        code: "",
        lastBytes: 0,
      },
    ],
    grants: [],
    ...overrides,
  };
}

export function server(overrides: Partial<ZFSReplicaServer> = {}): ZFSReplicaServer {
  return {
    id: "nas",
    name: "Backup-NAS",
    host: "192.168.1.30",
    user: "root",
    port: 22,
    pool: "backup",
    root: "backup/bombvault-replica",
    enabled: true,
    freeBytes: 3.1 * 1024 ** 4,
    sizeBytes: 8 * 1024 ** 4,
    usedBy: [],
    ...overrides,
  };
}

export function group(): GroupState {
  return {
    ok: true,
    active: true,
    instanceId: "self",
    name: "tower",
    passwordSet: true,
    members: [{ id: "peer-1", name: "tower-2", version: "9.9.1", direct: true, relay: false, address: "" }],
    relay: { mode: "off", url: "", projectUrl: "", connected: false, serve: false, serveClients: 0 },
    joinedAgo: 0,
    memberSeen: true,
    selfAddress: "",
    selfAddressManual: false,
  };
}
