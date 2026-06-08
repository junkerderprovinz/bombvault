import path from "node:path";

// Narrow inspect type — only the fields needed for appdata resolution.
export interface MountEntry {
  Type: string;
  Source: string;
  Destination: string;
}

export interface ContainerInspectSubset {
  Name: string;
  Mounts: MountEntry[];
}

/**
 * Build the conventional Unraid appdata path for a container name.
 * Strips a leading `/` from the dockerode Name field, then posix-joins
 * with appdataDir.
 */
export function appdataPathForName(name: string, appdataDir: string): string {
  const stripped = name.startsWith("/") ? name.slice(1) : name;
  return path.posix.join(appdataDir, stripped);
}

/**
 * Derive the set of appdata paths for a container from its inspect data.
 *
 * Strategy:
 *  1. Collect bind-mount Sources whose path starts with appdataDir + "/".
 *  2. De-duplicate.
 *  3. If none qualify, fall back to the name convention via appdataPathForName.
 */
export function resolveAppdataPaths(
  inspect: ContainerInspectSubset,
  appdataDir: string,
): string[] {
  const prefix = appdataDir.endsWith("/") ? appdataDir : appdataDir + "/";

  const seen = new Set<string>();
  for (const mount of inspect.Mounts) {
    if (mount.Type === "bind" && mount.Source.startsWith(prefix)) {
      seen.add(mount.Source);
    }
  }

  if (seen.size > 0) {
    return Array.from(seen);
  }

  return [appdataPathForName(inspect.Name, appdataDir)];
}
