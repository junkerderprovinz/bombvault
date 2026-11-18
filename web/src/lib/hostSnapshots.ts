// The server's own snapshots: ZFS, btrfs and VM snapshots, which are not
// backups. The page groups them by what they were taken of, works out what a
// delete frees and turns the backend's codes into sentences.
import type { HostSnapKind, HostSnapPlan, HostSnapPool, HostSnapRef, HostSnapSource, HostSnapshot, HostSnapshotList } from "./api";
import type { TranslationKey } from "./i18n";

export const KIND_ORDER: HostSnapKind[] = ["zfs", "btrfs", "vm"];

export const KIND_LABEL: Record<HostSnapKind, TranslationKey> = {
  zfs: "hostSnap.kind.zfs",
  btrfs: "hostSnap.kind.btrfs",
  vm: "hostSnap.kind.vm",
};

/** refKey names a snapshot uniquely across kinds, for selection and React keys. */
export function refKey(r: HostSnapRef): string {
  return [r.kind, r.fs ?? "", r.source, r.name].join("\u0000");
}

/** sourceKey names what snapshots are taken of. */
export function sourceKey(kind: HostSnapKind, fs: string | undefined, source: string): string {
  return [kind, fs ?? "", source].join("\u0000");
}

export interface SnapGroup {
  key: string;
  kind: HostSnapKind;
  fs?: string;
  source: string;
  snapshots: HostSnapshot[];
}

/** groupSnapshots puts the snapshots of one source together, in kind order and
 *  in the order the server sent them, which is newest first. */
export function groupSnapshots(snaps: HostSnapshot[]): SnapGroup[] {
  const groups = new Map<string, SnapGroup>();
  for (const s of snaps) {
    const key = sourceKey(s.kind, s.fs, s.source);
    let g = groups.get(key);
    if (!g) {
      g = { key, kind: s.kind, fs: s.fs, source: s.source, snapshots: [] };
      groups.set(key, g);
    }
    g.snapshots.push(s);
  }
  return [...groups.values()].sort((a, b) => KIND_ORDER.indexOf(a.kind) - KIND_ORDER.indexOf(b.kind));
}

/** sourceLabel is how the page names a source: the dataset, the VM, or a
 *  btrfs subvolume below its filesystem. "?" is a source that is gone. */
export function sourceLabel(
  kind: HostSnapKind,
  fs: string | undefined,
  source: string,
  t: (key: TranslationKey) => string
): string {
  if (kind !== "btrfs") return source;
  if (source === "?") return `${fs ?? ""}: ${t("hostSnap.sourceGone")}`;
  if (source === "") return `${fs ?? ""} (${t("hostSnap.topLevel")})`;
  return `${fs ?? ""}/${source}`;
}

export interface FreedEstimate {
  bytes: number;
  /** False when a part of the selection reports no size, so bytes is a floor. */
  complete: boolean;
}

/** freedEstimate adds what btrfs quotas report to the figure ZFS worked out
 *  for its part of the selection. */
export function freedEstimate(selected: HostSnapshot[], zfsBytes: number): FreedEstimate {
  let bytes = Math.max(0, zfsBytes);
  let complete = true;
  for (const s of selected) {
    if (s.kind === "zfs") continue;
    if (s.kind === "btrfs" && s.usedBytes >= 0) {
      bytes += s.usedBytes;
      continue;
    }
    complete = false;
  }
  return { bytes, complete };
}

/** sleepingPools are the pools whose disks slept, so their snapshots were not read. */
export function sleepingPools(pools: HostSnapPool[]): HostSnapPool[] {
  return pools.filter((p) => p.state === "asleep" && !p.read);
}

/** wokenPools are the pools a read of sleeping pools spun up. */
export function wokenPools(pools: HostSnapPool[]): HostSnapPool[] {
  return pools.filter((p) => p.woke);
}

/** stillAsleep are the pools a read of sleeping pools found and left asleep. */
export function stillAsleep(pools: HostSnapPool[]): HostSnapPool[] {
  return pools.filter((p) => p.state === "asleep" && p.read && !p.woke);
}

const SSH_CODES = new Set(["ssh-missing", "ssh-auth", "ssh-hostkey", "ssh-unreachable"]);

export interface KindStates {
  /** Kinds this server does not have. */
  missing: HostSnapKind[];
  /** Kinds that could not be read, with their code. */
  failed: { kind: HostSnapKind; code: string; detail?: string }[];
  /** The host connection itself is not set up or does not answer. */
  noHost: boolean;
}

export function kindStates(kinds: HostSnapshotList["kinds"]): KindStates {
  const out: KindStates = { missing: [], failed: [], noHost: false };
  for (const kind of KIND_ORDER) {
    const st = kinds?.[kind];
    if (!st || st.code === "ok") continue;
    if (SSH_CODES.has(st.code)) {
      out.noHost = true;
      continue;
    }
    if (st.code === "missing" || (kind === "vm" && st.code === "vm-unreachable")) out.missing.push(kind);
    else out.failed.push({ kind, code: st.code, detail: st.detail });
  }
  return out;
}

const OWNER_KEY: Record<string, TranslationKey> = {
  plan: "hostSnap.owner.plan",
  backup: "hostSnap.owner.backup",
  replica: "hostSnap.owner.replica",
  safety: "hostSnap.owner.safety",
};

export function ownerKey(s: HostSnapshot): TranslationKey | null {
  if (s.managedBy === "unraid") return "hostSnap.owner.unraid";
  return OWNER_KEY[s.owner] ?? null;
}

const CODE_KEY: Record<string, TranslationKey> = {
  busy: "hostSnap.code.busy",
  held: "hostSnap.code.held",
  clones: "hostSnap.code.clones",
  "not-found": "hostSnap.code.notFound",
  exists: "hostSnap.code.exists",
  managed: "hostSnap.code.managed",
  "not-supported": "hostSnap.code.notSupported",
  "invalid-name": "hostSnap.code.invalidName",
  "invalid-plan": "hostSnap.code.invalidPlan",
  "zfs-permission": "hostSnap.code.permission",
  "ssh-missing": "hostSnap.code.noHost",
  "ssh-auth": "hostSnap.code.noHost",
  "ssh-hostkey": "hostSnap.code.noHost",
  "ssh-unreachable": "hostSnap.code.noHost",
  "skipped-asleep": "hostSnap.code.skippedAsleep",
  "kept-some": "hostSnap.code.keptSome",
  "needs-qcow2": "hostSnap.code.needsQcow2",
};

/** codeKey is the sentence for a backend code, a generic failure for one it
 *  does not know. */
export function codeKey(code: string | undefined): TranslationKey {
  return (code && CODE_KEY[code]) || "hostSnap.code.failed";
}

/** planOf finds the plan that took a snapshot. Names carry the first eight
 *  characters of the plan's id. */
export function planOf(s: HostSnapshot, plans: HostSnapPlan[]): HostSnapPlan | undefined {
  if (!s.planId) return undefined;
  return plans.find((p) => p.id.toLowerCase().startsWith(s.planId ?? ""));
}

/** defaultName is the name the create window offers, the UTC instant like
 *  the names BombVault gives its own snapshots. */
export function defaultName(now: Date): string {
  const p = (n: number) => String(n).padStart(2, "0");
  return (
    "manual-" +
    `${now.getUTCFullYear()}${p(now.getUTCMonth() + 1)}${p(now.getUTCDate())}` +
    `${p(now.getUTCHours())}${p(now.getUTCMinutes())}${p(now.getUTCSeconds())}`
  );
}

/** validName mirrors the server's check, so the window can say no before a
 *  round trip. */
export function validName(name: string): boolean {
  return /^[A-Za-z0-9_.:][A-Za-z0-9_.:-]{0,199}$/.test(name) && name !== "." && name !== "..";
}

/** needsQcow2 names the VMs whose raw disks cannot hold a snapshot, which a
 *  window lists apart instead of offering them. */
export function needsQcow2(sources: HostSnapSource[]): string[] {
  return sources.filter((s) => s.kind === "vm" && s.reason === "needs-qcow2").map((s) => s.source);
}

export interface BareGroup {
  kind: HostSnapKind;
  sources: HostSnapSource[];
}

/** bareSources are the datasets, subvolumes and VMs without a snapshot yet,
 *  per kind, so the page can offer the first one. */
export function bareSources(sources: HostSnapSource[], snaps: HostSnapshot[]): BareGroup[] {
  const taken = new Set(snaps.map((s) => sourceKey(s.kind, s.fs, s.source)));
  return KIND_ORDER.map((kind) => ({
    kind,
    sources: sources.filter((s) => s.kind === kind && !taken.has(sourceKey(s.kind, s.fs, s.source)) && (s.canCreate || s.reason)),
  })).filter((g) => g.sources.length > 0);
}
