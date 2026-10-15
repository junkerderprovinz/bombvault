import type { Run } from "./api";
import type { TranslationKey } from "./i18n";

type Translate = (key: TranslationKey) => string;

// The sentence for each reason a container on ZFS was stopped for its whole
// backup. The server writes "code: path or dataset".
const REASON_KEY: Record<string, TranslationKey> = {
  "not-on-zfs": "snapshot.reason.notOnZFS",
  "split-share": "snapshot.reason.splitShare",
  "share-not-found": "snapshot.reason.shareNotFound",
  "nested-mount": "snapshot.reason.nestedMount",
  "ssh-missing": "snapshot.reason.sshMissing",
  "zfs-unreachable": "snapshot.reason.hostUnreachable",
  "view-unavailable": "snapshot.reason.viewUnavailable",
  "dataset-unknown": "snapshot.reason.datasetUnknown",
  "snapdir-disabled": "snapshot.reason.snapdirDisabled",
  "key-not-loaded": "snapshot.reason.keyNotLoaded",
  "repo-in-container": "snapshot.reason.repoInContainer",
};

/** The window a snapshot backup held the container down, in whole seconds
 *  rounded up, so a window under a second does not read as none. */
export function snapshotWindowText(t: Translate, ms: number): string {
  return t("run.snapshotWindow").replace("{seconds}", String(Math.max(1, Math.ceil(ms / 1000))));
}

/** Why a container with data on ZFS was stopped for its whole backup. */
export function snapshotFallbackText(t: Translate, raw: string): string {
  const sep = raw.indexOf(": ");
  const code = sep < 0 ? raw : raw.slice(0, sep);
  const detail = sep < 0 ? "" : raw.slice(sep + 2);
  const key = REASON_KEY[code];
  const reason = key
    ? t(key).replace("{path}", detail)
    : t("snapshot.reason.other").replace("{path}", detail).replace("{code}", code);
  return t("run.snapshotFallback").replace("{reason}", reason);
}

/** Whether a run says anything about how it read its data. */
export function hasSnapshotNote(run: Run): boolean {
  return run.snapshotWindowMs != null || !!run.snapshotFallback;
}
