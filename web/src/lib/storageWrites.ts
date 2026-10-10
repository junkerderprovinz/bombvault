// Saving a storage location's settings. A location is read from one of four
// kinds of object, and each kind is written through its own route, so what a
// location can change depends on what it is.

import {
  getSettings,
  putSettings,
  updateDestination,
  updateOffsiteTarget,
  updateRepo,
  type Compression,
  type Destination,
  type FollowedSetting,
  type OffsiteTarget,
  type OkEnvelope,
  type RetentionKeep,
  type SaveWarning,
  type StorageLocation,
  type StorageLocationSection,
} from "./api";
import { credentialKind } from "./storageLocations";

/** The records behind a location that its routes want sent back whole. */
export interface LocationRecords {
  destination?: Destination;
  targets: OffsiteTarget[];
}

export interface LocationPatch {
  name?: string;
  enabled?: boolean;
  compression?: Compression;
  offPremises?: boolean;
  storageClass?: string;
  immutable?: boolean;
  retention?: RetentionKeep;
  limitUpload?: number;
  limitDownload?: number;
  credsRef?: string;
}

export type LocationSetting = keyof LocationPatch;

export type SaveAnswer = OkEnvelope & { warnings?: SaveWarning[] };

const DESTINATION: LocationSetting[] = ["name", "enabled", "compression", "offPremises", "immutable", "retention", "limitUpload", "limitDownload"];
const TARGET: LocationSetting[] = ["name", "enabled", "compression", "immutable", "retention", "limitUpload", "limitDownload"];
const REPO: LocationSetting[] = ["name", "enabled", "compression", "offPremises", "immutable"];

function idOf(location: StorageLocation): string {
  return location.id.slice(location.id.indexOf(":") + 1);
}

function ownTarget(location: StorageLocation, records: LocationRecords): OffsiteTarget | undefined {
  return records.targets.find((target) => target.id === idOf(location));
}

/** The S3 storage class a location writes with, "" for the provider's own.
 *  Undefined where the location's object carries none. */
export function storageClassOf(location: StorageLocation, records: LocationRecords): string | undefined {
  if (location.object === "destination") return records.destination?.storageClass;
  return ownTarget(location, records)?.storageClass;
}

/** settingsOwned reports whether the location is the copy a domain's off-site
 *  settings describe. Those settings rewrite its target on every save, so
 *  nothing is changed on the target itself. */
export function settingsOwned(location: StorageLocation): boolean {
  return location.object === "target" && location.sections.some((section) => section.primary);
}

/** writable lists the settings the server lets this location change. */
export function writable(location: StorageLocation, records: LocationRecords): Set<LocationSetting> {
  const s3 = location.backend === "s3";
  const withCredentials = credentialKind(location.backend) !== null;
  switch (location.object) {
    case "destination":
      if (!records.destination) return new Set();
      return new Set(s3 ? [...DESTINATION, "storageClass"] : DESTINATION);
    case "target":
      if (settingsOwned(location) || !ownTarget(location, records)) return new Set();
      return new Set([...TARGET, ...(s3 ? (["storageClass"] as const) : []), ...(withCredentials ? (["credsRef"] as const) : [])]);
    case "repo":
      return new Set(withCredentials ? [...REPO, "credsRef"] : REPO);
    case "path":
      // The folder the domains write to ages by the shared local rule.
      return new Set(["retention"]);
  }
}

function keepColumns(keep: RetentionKeep) {
  return {
    retentionKeepLast: keep.keepLast,
    retentionKeepDaily: keep.keepDaily,
    retentionKeepWeekly: keep.keepWeekly,
    retentionKeepMonthly: keep.keepMonthly,
    retentionKeepYearly: keep.keepYearly,
  };
}

/** saveLocation writes the patch through the route of the object the location
 *  is read from. */
export async function saveLocation(
  location: StorageLocation,
  records: LocationRecords,
  patch: LocationPatch
): Promise<SaveAnswer> {
  const id = idOf(location);
  switch (location.object) {
    case "destination": {
      if (!records.destination) return { ok: false };
      const { name, storageClass, immutable } = records.destination;
      return updateDestination(id, { name, storageClass, immutable, ...patch });
    }
    case "target": {
      const target = ownTarget(location, records);
      if (!target) return { ok: false };
      const { retention, ...rest } = patch;
      return updateOffsiteTarget(id, { ...target, ...rest, ...(retention ? keepColumns(retention) : {}) });
    }
    case "repo":
      return updateRepo(id, patch);
    case "path": {
      if (!patch.retention) return { ok: false };
      const read = await getSettings();
      if (!read.ok) return read;
      return putSettings({ ...read.settings, ...keepColumns(patch.retention) });
    }
  }
}

/** followLocation gives a section the location's value for `setting` back. */
export async function followLocation(
  location: StorageLocation,
  records: LocationRecords,
  section: StorageLocationSection,
  setting: FollowedSetting
): Promise<SaveAnswer> {
  if (location.object === "destination") {
    const target = records.targets.find((t) => t.id === section.targetId);
    if (!target) return { ok: false };
    return updateOffsiteTarget(target.id, target, undefined, [setting]);
  }
  const read = await getSettings();
  if (!read.ok) return read;
  const ownRetention = { ...read.settings.ownRetention };
  delete ownRetention[section.domain];
  return putSettings({ ...read.settings, ownRetention });
}
