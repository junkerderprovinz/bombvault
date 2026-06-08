import type { ContainerInfo } from "dockerode";
import { appdataPathForName } from "../../lib/appdata";

// ---------------------------------------------------------------------------
// ContainerRow — the display model rendered by the containers page
// ---------------------------------------------------------------------------

export interface ContainerRow {
  /** Dockerode container ID (short or full). */
  id: string;
  /** Container name without the leading "/". */
  name: string;
  /** Image tag string (first entry in Names). */
  image: string;
  /** Container state string ("running", "exited", …). */
  state: string;
  /** Resolved appdata paths (bind-mounts under appdataDir, or the conventional path). */
  appdataPaths: string[];
  /**
   * ISO timestamp of the last successful backup, or null when none has been
   * recorded. The page maps null to the i18n key "containers.never".
   */
  lastBackup: string | null;
}

/**
 * Pure view-model function: merge a dockerode ContainerInfo list with the
 * last-backup-by-name map into typed ContainerRows ready for rendering.
 *
 * @param list             - ContainerInfo[] from dockerode listContainers({ all: true })
 * @param appdataDir       - Host appdata root, e.g. "/mnt/user/appdata"
 * @param lastBackupByName - Map<containerName, ISO-string-or-null> from the DB.
 *                           Containers absent from the map are treated as "never".
 */
export function toContainerRows(
  list: ContainerInfo[],
  appdataDir: string,
  lastBackupByName: Map<string, string | null>,
): ContainerRow[] {
  return list.map((info) => {
    // dockerode Names is an array of strings with a leading "/" (e.g. ["/plex"]).
    // Use the first entry; strip the leading slash for display.
    const rawName = info.Names[0] ?? "";
    const name = rawName.startsWith("/") ? rawName.slice(1) : rawName;

    // Derive the conventional appdata path from the name (ContainerInfo does not
    // include Mounts — we use the naming convention, which matches resolveAppdataPaths
    // fallback). The full inspect-based resolution is done by the action when
    // creating a backup_target.
    const appdataPaths = [appdataPathForName(name, appdataDir)];

    const lastBackup = lastBackupByName.get(name) ?? null;

    return {
      id: info.Id,
      name,
      image: info.Image,
      state: info.State,
      appdataPaths,
      lastBackup,
    };
  });
}
