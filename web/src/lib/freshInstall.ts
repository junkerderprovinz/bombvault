import type { BackupItem } from "./api";

/** isFreshInstall reports a BombVault that holds no backup of anything yet. */
export function isFreshInstall(items: BackupItem[]): boolean {
  return items.every((item) => item.lastBackup === 0);
}
