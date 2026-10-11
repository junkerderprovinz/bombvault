// Where a finding about a disk leads: the storage location that disk holds.

import type { AnomalyView, StorageLocation } from "../../lib/api";
import { locationRoute, sectionDomains } from "../../lib/storageLocations";

export const STORAGE_PATH = "/storage";

export interface FindingPlace {
  path: string;
  /** The location's name, where the finding could be traced to one. */
  name?: string;
}

/**
 * findingPlace is the page a capacity finding points at. The server names a
 * disk by the sections whose backups sit on it and by whether it is remote,
 * not by a storage location, so the finding leads to a location only where
 * exactly one fits and to the list of them otherwise.
 */
export function findingPlace(a: AnomalyView, locations: StorageLocation[]): FindingPlace | undefined {
  if (a.scopeKind !== "volume") return undefined;
  const remote = a.scopeId.startsWith("remote:");
  const onDisk = a.domain.split(",").map((domain) => domain.trim());
  const fits = locations.filter(
    (location) =>
      (location.backend !== "local") === remote && sectionDomains(location).some((domain) => onDisk.includes(domain))
  );
  return fits.length === 1 ? { path: locationRoute(fits[0].id), name: fits[0].name } : { path: STORAGE_PATH };
}
