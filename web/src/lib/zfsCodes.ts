/**
 * The ZFS backend answers with a reason code and the numbers or names that
 * belong to it; the sentence is built here. The maps are written out rather
 * than assembled from a template so that tsc and the orphan test can see every
 * key, and so zfsCodes.test.ts can hold them against the Go lists.
 */
import type { TranslationKey } from "./i18n";
import { tLtr } from "./ltrFragments";

/** The fields a caller may hand in, gathered from the view, the check result
 *  and the member being rendered. */
export interface ZFSCodeVars {
  target?: string;
  uriTarget?: string;
  max?: number;
  names?: string[];
  hostMountpoint?: string;
  leftoverCount?: number;
}

type Translate = (key: TranslationKey) => string;

export const ZFS_CODE_KEY = {
  ok: "zfs.code.ok",
  "ssh-missing": "zfs.code.ssh-missing",
  "host-placeholder": "zfs.code.host-placeholder",
  "host-fallback": "zfs.code.host-fallback",
  "ssh-unreachable": "zfs.code.ssh-unreachable",
  "ssh-auth": "zfs.code.ssh-auth",
  "zfs-not-found": "zfs.code.zfs-not-found",
  "zfs-permission": "zfs.code.zfs-permission",
  "uri-mismatch": "zfs.code.uri-mismatch",
  "zfs-error": "zfs.code.zfs-error",
  "propagation-missing": "zfs.code.propagation-missing",
  "invalid-name": "zfs.code.invalid-name",
  "name-too-long": "zfs.code.name-too-long",
  "invalid-exclude": "zfs.code.invalid-exclude",
  "not-found": "zfs.code.not-found",
  "not-filesystem": "zfs.code.not-filesystem",
  "overlaps-item": "zfs.code.overlaps-item",
  "docker-storage": "zfs.code.docker-storage",
  "nothing-readable": "zfs.code.nothing-readable",
  "snapshot-failed": "zfs.code.snapshot-failed",
  "containers-busy": "zfs.code.containers-busy",
  "consistency-stop-failed": "zfs.code.consistency-stop-failed",
  "pre-snapshot-failed": "zfs.code.pre-snapshot-failed",
  "container-unknown": "zfs.code.container-unknown",
  "container-is-self": "zfs.code.container-is-self",
  "hook-container-missing": "zfs.code.hook-container-missing",
  "leftover-snapshots": "zfs.code.leftover-snapshots",
  zvol: "zfs.code.zvol",
  "canmount-off": "zfs.code.canmount-off",
  "legacy-mount": "zfs.code.legacy-mount",
  "no-mountpoint": "zfs.code.no-mountpoint",
  "not-mounted": "zfs.code.not-mounted",
  "key-not-loaded": "zfs.code.key-not-loaded",
  "snapdir-disabled": "zfs.code.snapdir-disabled",
  "not-visible": "zfs.code.not-visible",
  "shfs-only": "zfs.code.shfs-only",
  "snapshot-not-visible": "zfs.code.snapshot-not-visible",
  "snapshot-loop": "zfs.code.snapshot-loop",
  "backup-failed": "zfs.code.backup-failed",
  "not-reached": "zfs.code.not-reached",
  gone: "zfs.code.gone",
  "read-only-mount": "zfs.code.read-only-mount",
  "destination-not-mounted": "zfs.code.destination-not-mounted",
  "not-enough-space": "zfs.code.not-enough-space",
  "safety-snapshot-failed": "zfs.code.safety-snapshot-failed",
  "safety-name-too-long": "zfs.code.safety-name-too-long",
} as const satisfies Record<string, TranslationKey>;

export type ZFSReasonCode = keyof typeof ZFS_CODE_KEY;

/** What to do about it, where there is something to do. snapshot-loop shares
 *  the propagation fix because the remedy is the same mapping. */
export const ZFS_FIX_KEY: Partial<Record<ZFSReasonCode, TranslationKey>> = {
  "host-placeholder": "zfs.fix.host-placeholder",
  "ssh-auth": "zfs.fix.ssh-auth",
  "ssh-unreachable": "zfs.fix.ssh-unreachable",
  "zfs-not-found": "zfs.fix.zfs-not-found",
  "zfs-permission": "zfs.fix.zfs-permission",
  "uri-mismatch": "zfs.fix.uri-mismatch",
  "propagation-missing": "zfs.fix.propagation-missing",
  "overlaps-item": "zfs.fix.overlaps-item",
  "docker-storage": "zfs.fix.docker-storage",
  "name-too-long": "zfs.fix.name-too-long",
  "containers-busy": "zfs.fix.containers-busy",
  "consistency-stop-failed": "zfs.fix.consistency-stop-failed",
  "pre-snapshot-failed": "zfs.fix.pre-snapshot-failed",
  "leftover-snapshots": "zfs.fix.leftover-snapshots",
  zvol: "zfs.fix.zvol",
  "canmount-off": "zfs.fix.canmount-off",
  "legacy-mount": "zfs.fix.legacy-mount",
  "not-mounted": "zfs.fix.not-mounted",
  "key-not-loaded": "zfs.fix.key-not-loaded",
  "snapdir-disabled": "zfs.fix.snapdir-disabled",
  "not-visible": "zfs.fix.not-visible",
  "shfs-only": "zfs.fix.shfs-only",
  "snapshot-loop": "zfs.fix.propagation-missing",
  "read-only-mount": "zfs.fix.read-only-mount",
  "destination-not-mounted": "zfs.fix.destination-not-mounted",
};

/** Member states that are not problems, plus the mark a first sighting adds. */
export const ZFS_MEMBER_KEY = {
  "backed-up": "zfs.member.backed-up",
  empty: "zfs.member.empty",
  excluded: "zfs.member.excluded",
  new: "zfs.member.new",
} as const satisfies Record<string, TranslationKey>;

export type ZFSMemberOutcome = keyof typeof ZFS_MEMBER_KEY;

/** Which field fills which placeholder, per code. The names differ where the
 *  sentence reads better with a short word than with the field name. */
export const ZFS_CODE_VARS: Partial<Record<ZFSReasonCode, Record<string, keyof ZFSCodeVars>>> = {
  "ssh-unreachable": { target: "target" },
  "uri-mismatch": { target: "target", uriTarget: "uriTarget" },
  "name-too-long": { max: "max", names: "names" },
  "invalid-exclude": { names: "names" },
  "overlaps-item": { names: "names" },
  "consistency-stop-failed": { names: "names" },
  "container-unknown": { names: "names" },
  "leftover-snapshots": { n: "leftoverCount" },
  "not-visible": { path: "hostMountpoint" },
};

function isReasonCode(code: string): code is ZFSReasonCode {
  return code in ZFS_CODE_KEY;
}

/** The translation key of a member outcome, or null for a reason code. */
export function zfsMemberKey(outcome: string): TranslationKey | null {
  return outcome in ZFS_MEMBER_KEY ? ZFS_MEMBER_KEY[outcome as ZFSMemberOutcome] : null;
}

/** The fix for a code, or null when there is nothing the reader can do. */
export function zfsFixKey(code: string): TranslationKey | null {
  return isReasonCode(code) ? (ZFS_FIX_KEY[code] ?? null) : null;
}

/** The sentence for one code, with its numbers and names put in. An unknown
 *  code is named rather than swallowed, so a backend that grew a code still
 *  says something true. */
export function zfsCodeSentence(t: Translate, code: string, vars: ZFSCodeVars = {}): string {
  if (!isReasonCode(code)) return t("zfs.code.unknown").replace("{code}", code);
  // A run can name a dataset that has left the tree, and its mountpoint went
  // with it.
  if (code === "not-visible" && !vars.hostMountpoint) return tLtr(t, "zfs.notVisibleNoPath");
  let text = tLtr(t, ZFS_CODE_KEY[code]);
  for (const [placeholder, field] of Object.entries(ZFS_CODE_VARS[code] ?? {})) {
    const value = vars[field];
    const filled = Array.isArray(value) ? value.join(", ") : String(value ?? "");
    text = text.replace(`{${placeholder}}`, filled);
  }
  return text;
}
