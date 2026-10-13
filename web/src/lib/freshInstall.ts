import type { BackupItem } from "./api";

/** isFreshInstall reports a BombVault that holds no backup of anything yet. */
export function isFreshInstall(items: BackupItem[]): boolean {
  return items.every((item) => item.lastBackup === 0);
}

const IN_USE = "bombvault.inUse";

/** seenInUse reports that the entries held a backup when this browser last
 *  read them, so the next start can open the overview before they are read. */
export function seenInUse(): boolean {
  try {
    return localStorage.getItem(IN_USE) === "1";
  } catch {
    return false;
  }
}

export function rememberInUse(inUse: boolean): void {
  try {
    if (inUse) localStorage.setItem(IN_USE, "1");
    else localStorage.removeItem(IN_USE);
  } catch {
    // Without storage every start waits for the entries.
  }
}
