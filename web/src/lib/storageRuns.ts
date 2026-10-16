// The runs a storage location starts that are neither a backup nor a copy on
// schedule: moving a section's copy to another destination, and deleting the
// backups at a storage location before it is removed.

import type { useT } from "./i18n";
import { MOVE_PREFIX, offsiteRunProgress, type ProgressMap, type ProgressState } from "./progress";

type T = ReturnType<typeof useT>["t"];

/** The progress key a section's move reports under. */
export function moveKey(targetId: string): string {
  return MOVE_PREFIX + targetId;
}

const ERASE = "erase:";

/** The progress key the deletion at a storage location reports under, by the
 *  id of the object the location is read from. */
export function eraseKey(objectId: string): string {
  return ERASE + objectId;
}

/** liveRun is the run under `key` while it runs. One that ended lingers for a
 *  moment so its bar can finish, and counts as over. */
export function liveRun(progress: ProgressMap, key: string): ProgressState | null {
  const state = progress[key];
  return state?.active && !state.finished ? state : null;
}

/** Whether a section is moving or a storage location is being emptied. The busy
 *  hints leave both out, so the readers of the locations ask here to read
 *  again once such a run is gone. */
export function storageRunning(progress: ProgressMap): boolean {
  return Object.keys(progress).some((key) => key.startsWith(ERASE) || key.startsWith(MOVE_PREFIX));
}

/** What a row says while its section moves: that it does, and which backup
 *  the copy has reached once restic counts them. */
export function moveText(t: T, state: ProgressState): string {
  const overall = offsiteRunProgress(state);
  if (!overall) return t("storage.move.running");
  const step = t("overview.head.copyStep").replace("{index}", String(overall.index)).replace("{total}", String(overall.total));
  return `${t("storage.move.running")} · ${step}`;
}
