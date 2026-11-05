// A section that never copied off site from this database can find backups of
// its own from before it. BombVault then holds the section's copies until the
// person takes those backups as this server's own, because a database set up
// again has lost the rules that kept entries away from a storage location.

import type { PlacementDomain, Run, StorageLocation } from "./api";
import { isPlacementDomain } from "./placement";
import { locationRoute } from "./storageLocations";

/** waitingDomains are the sections whose copies wait for the decision, in the
 *  order the storage locations name them. */
export function waitingDomains(locations: StorageLocation[]): PlacementDomain[] {
  const out: PlacementDomain[] = [];
  for (const location of locations) {
    for (const section of location.sections) {
      if (section.awaitsDecision && isPlacementDomain(section.domain) && !out.includes(section.domain)) out.push(section.domain);
    }
  }
  return out;
}

/** decisionRoute is the page of the first storage location the section's
 *  waiting copies go to, where the decision is offered. */
export function decisionRoute(locations: StorageLocation[], domain: PlacementDomain): string {
  const location = locations.find((l) => l.sections.some((s) => s.domain === domain && s.awaitsDecision));
  return location ? locationRoute(location.id) : rulesRoute(domain);
}

/** The id of a section's line under "Storage locations per section". */
export function rulesAnchor(domain: PlacementDomain): string {
  return `copy-rules-${domain}`;
}

/** rulesRoute leads to the section's rules on the storage page. */
export function rulesRoute(domain: PlacementDomain): string {
  return `/storage#${rulesAnchor(domain)}`;
}

const RUN_DOMAIN: Record<string, PlacementDomain> = { container: "containers", vm: "vms", files: "files" };

/** runSection is the section a run's entry belongs to, where it has one. */
export function runSection(run: Run): PlacementDomain | undefined {
  return RUN_DOMAIN[run.domain];
}
