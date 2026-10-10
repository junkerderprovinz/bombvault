// What the storage pages read out of a storage location: who uses it, how full
// it is, and which of its settings a section holds for itself.

import type {
  FollowedSetting,
  OffsiteDomain,
  RetentionKeep,
  StorageLocation,
  StorageLocationCapacity,
  StorageLocationSection,
} from "./api";
import { FORECAST_CAP_WEEKS, FORECAST_WARN_WEEKS, humanBytes } from "./forecast";
import type { TranslationKey, useT } from "./i18n";
import type { KeepCounts } from "./keepPlan";
import { formatList } from "./placement";

type T = ReturnType<typeof useT>["t"];

export const DOMAIN_LABEL: Record<OffsiteDomain, TranslationKey> = {
  containers: "nav.containers",
  vms: "nav.vms",
  flash: "nav.flash",
  files: "nav.files",
  zfs: "nav.zfs",
  config: "nav.config",
};

export function locationRoute(id: string): string {
  return `/storage/${encodeURIComponent(id)}`;
}

/** The domains that keep backups at the location, each once, in the order the
 *  server lists their sections. */
export function sectionDomains(location: StorageLocation): OffsiteDomain[] {
  return [...new Set(location.sections.map((section) => section.domain))];
}

/** Which sections keep backups at a location, as one line. */
export function usedByText(t: T, lang: string, location: StorageLocation): string {
  const domains = sectionDomains(location).map((domain) => t(DOMAIN_LABEL[domain]));
  return domains.length > 0 ? t("dest.usedBy").replace("{domains}", () => formatList(lang, domains)) : t("dest.unused");
}

export function keepCounts(keep: RetentionKeep): KeepCounts {
  return [keep.keepLast, keep.keepDaily, keep.keepWeekly, keep.keepMonthly, keep.keepYearly];
}

export function retentionKeep(counts: KeepCounts): RetentionKeep {
  const [keepLast, keepDaily, keepWeekly, keepMonthly, keepYearly] = counts;
  return { keepLast, keepDaily, keepWeekly, keepMonthly, keepYearly };
}

/** A target's direct repository carries the target's settings, so the home it
 *  is says nothing its copy does not. */
function mirrorsCopy(section: StorageLocationSection): boolean {
  return section.use === "home" && !!section.targetId;
}

/** The sections that hold `setting` themselves instead of taking the
 *  location's value. */
export function deviating(location: StorageLocation, setting: FollowedSetting): StorageLocationSection[] {
  return location.sections.filter((section) => section.own.includes(setting) && !mirrorsCopy(section));
}

/** canFollow reports whether the server gives a section its location's value
 *  back. A target made from a destination can follow it, except the copy a
 *  domain's off-site settings describe, which those settings rewrite on every
 *  save. A domain with a local rule of its own goes back to the shared one. */
export function canFollow(location: StorageLocation, section: StorageLocationSection): boolean {
  if (location.object === "destination") return section.use === "copy" && !!section.targetId && !section.primary;
  return (location.object === "path" || location.object === "repo") && section.use === "home";
}

export type Fill =
  | { state: "measured"; share: number; weeks?: number; warn: boolean }
  /** The backend reports no room at all. */
  | { state: "unsupported" }
  /** Nobody has asked the remote yet. */
  | { state: "unmeasured" };

/** fillOf reads how full a location is. `weeks` is the forecast in whole
 *  weeks, capped one past FORECAST_CAP_WEEKS for "more than a year". */
export function fillOf(capacity: StorageLocationCapacity): Fill {
  const { usedBytes, totalBytes, weeksToFull } = capacity;
  if (usedBytes === undefined || !totalBytes) {
    return { state: capacity.unsupported ? "unsupported" : "unmeasured" };
  }
  const share = Math.min(1, Math.max(0, usedBytes / totalBytes));
  if (weeksToFull === undefined) return { state: "measured", share, warn: false };
  const weeks = weeksToFull > FORECAST_CAP_WEEKS ? FORECAST_CAP_WEEKS + 1 : Math.max(1, Math.round(weeksToFull));
  return { state: "measured", share, weeks, warn: weeksToFull < FORECAST_WARN_WEEKS };
}

/** How much a location holds: what is used of its volume where that was
 *  measured, else what its repositories held at their last sample. */
export function amountText(t: T, capacity: StorageLocationCapacity): string {
  const { usedBytes, totalBytes, storedBytes } = capacity;
  if (usedBytes !== undefined && totalBytes) {
    return t("storage.usedOf").replace("{used}", humanBytes(usedBytes)).replace("{total}", humanBytes(totalBytes));
  }
  return storedBytes === undefined ? "" : humanBytes(storedBytes);
}

/** The rclone remote an address like rclone:name:folder goes through. */
export function rcloneRemote(where: string): string | null {
  const match = /^rclone:([^:]+):/.exec(where.trim());
  return match ? match[1] : null;
}

/** Which kind of stored credentials a backend signs in with. Every other
 *  backend keeps its login in the rclone.conf or uses an SSH key. */
export function credentialKind(backend: string): "s3" | "rest" | null {
  return backend === "s3" || backend === "rest" ? backend : null;
}
