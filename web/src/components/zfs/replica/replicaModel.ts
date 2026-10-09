import type {
  GroupMember,
  ZFSReplica,
  ZFSReplicaKeep,
  ZFSReplicaKeepCounts,
  ZFSReplicaKeepPreset,
  ZFSReplicaPool,
  ZFSReplicaServer,
  ZFSReplicaState,
} from "../../../lib/api";
import { humanBytes } from "../../../lib/forecast";
import type { TranslationKey } from "../../../lib/i18n";

type Translate = (key: TranslationKey, n?: number) => string;

/** The keep rule a new replica starts with: about a month of dailies and
 *  weeklies. */
export const DEFAULT_KEEP: ZFSReplicaKeep = { preset: "own", own: [0, 7, 3, 0, 0] };

export const KEEP_PRESETS: Record<Exclude<ZFSReplicaKeepPreset, "own">, ZFSReplicaKeepCounts> = {
  short: [0, 7, 4, 3, 0],
  balanced: [0, 7, 4, 6, 1],
  long: [0, 14, 8, 12, 3],
};

export const KEEP_PRESET_KEY = {
  short: "zfs.replica.keep.short",
  balanced: "zfs.replica.keep.balanced",
  long: "zfs.replica.keep.long",
  own: "zfs.replica.keep.own",
} as const satisfies Record<ZFSReplicaKeepPreset, TranslationKey>;

/** The labels of the five numbers, in the order the counts hold them. */
export const KEEP_FIELD_KEYS: readonly TranslationKey[] = [
  "zfs.replica.keep.latest",
  "zfs.replica.keep.daily",
  "zfs.replica.keep.weekly",
  "zfs.replica.keep.monthly",
  "zfs.replica.keep.yearly",
];

const KEEP_SUMMARY_KEYS: readonly TranslationKey[] = [
  "zfs.replica.keepSummary.latest",
  "zfs.replica.keepSummary.daily",
  "zfs.replica.keepSummary.weekly",
  "zfs.replica.keepSummary.monthly",
  "zfs.replica.keepSummary.yearly",
];

export function keepCounts(keep: ZFSReplicaKeep): ZFSReplicaKeepCounts {
  return keep.preset === "own" ? keep.own : KEEP_PRESETS[keep.preset];
}

/** keepSummary words a rule as "7 daily, 3 weekly", leaving out the zeros. */
export function keepSummary(t: Translate, keep: ZFSReplicaKeep): string {
  return keepCounts(keep)
    .map((n, i) => (n > 0 ? t(KEEP_SUMMARY_KEYS[i], n) : ""))
    .filter(Boolean)
    .join(", ");
}

/** withOwnCount sets one of the five numbers, clamped to a whole count. */
export function withOwnCount(counts: ZFSReplicaKeepCounts, index: number, raw: string): ZFSReplicaKeepCounts {
  const value = Math.max(0, Math.floor(Number(raw)) || 0);
  return counts.map((n, i) => (i === index ? value : n)) as ZFSReplicaKeepCounts;
}

export function replicaProgressKey(itemId: string): string {
  return `zfs-replica:${itemId}`;
}

/** targetRoot is the root above a copy's server folder. A copy lives at
 *  <root>/<server>/<source dataset>, so the path minus the dataset and the
 *  server folder is the root. */
export function targetRoot(targetPath: string, dataset: string): string {
  const serverFolder = targetPath.endsWith("/" + dataset) ? targetPath.slice(0, -dataset.length - 1) : targetPath;
  const cut = serverFolder.lastIndexOf("/");
  return cut < 0 ? serverFolder : serverFolder.slice(0, cut);
}

/** The commands the snapshot sheet hands out for the target server. */
export function cloneCommand(targetPath: string, dataset: string, snapshot: string): string {
  const leaf = dataset.slice(dataset.lastIndexOf("/") + 1);
  return `zfs clone ${targetPath}@${snapshot} ${targetRoot(targetPath, dataset)}/clone-${leaf}`;
}

export function takeoverCommand(targetPath: string): string {
  return `zfs inherit -r readonly ${targetPath} && zfs inherit -r canmount ${targetPath} && zfs mount -a`;
}

/** The delegation a user other than root needs on the receiving pool. */
export function allowLine(user: string): string {
  return `zfs allow ${user} receive,create,mount,rollback,destroy,userprop <pool>`;
}

/** An example path below a root, for the hint beside the root field. */
export function examplePath(root: string, serverName: string, dataset = "tank/appdata"): string {
  return `${root}/${serverName}/${dataset}`;
}

/** The root a pool offers before anyone types one. */
export function defaultRoot(pool: string): string {
  return pool ? `${pool}/bombvault-replica` : "";
}

/** A pool as a picker shows it, with its free space once it is known. */
export function poolLabel(t: Translate, pool: ZFSReplicaPool): string {
  return pool.sizeBytes
    ? t("zfs.replica.add.pool").replace("{pool}", () => pool.name).replace("{free}", () => humanBytes(pool.freeBytes))
    : pool.name;
}

/** The name a target goes by on screen: the server's name, or the paired
 *  instance's, falling back to its id while the group is not loaded. */
export function targetName(
  replica: ZFSReplica,
  servers: readonly ZFSReplicaServer[],
  members: readonly GroupMember[],
): string {
  const { kind, id } = replica.target;
  if (kind === "server") return servers.find((s) => s.id === id)?.name ?? id;
  if (kind === "peer") return members.find((m) => m.id === id)?.name || id;
  return "";
}

/** The member a sheet and the clone command speak about: the item's root. */
export function rootMember(replica: ZFSReplica, dataset: string) {
  return replica.members.find((m) => m.dataset === dataset) ?? replica.members[0];
}

/** peerHolds is true while a paired instance has not allowed the replica, or
 *  has taken the permission back, so nothing can be sent there. */
export function peerHolds(replica: ZFSReplica): boolean {
  return replica.target.kind === "peer" && replica.peerState !== "allowed";
}

export function isRunning(state: ZFSReplicaState, progressActive: boolean): boolean {
  return state === "running" || progressActive;
}

export function unixOf(rfc3339: string): number {
  return rfc3339 ? Math.floor(Date.parse(rfc3339) / 1000) : 0;
}

export function serverFree(t: Translate, server: ZFSReplicaServer): string {
  return t("zfs.replica.servers.free")
    .replace("{free}", () => humanBytes(server.freeBytes))
    .replace("{size}", () => humanBytes(server.sizeBytes));
}
