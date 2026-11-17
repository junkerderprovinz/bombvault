// Whether an instance has off-site copies to set up, or keeps every backup on
// the premises by choice. The server counts the choice by the same rule.

import type { OffsiteTarget, Settings } from "./api";

const SECTIONS = ["containers", "vms", "flash", "files", "zfs", "config"] as const;

/** copiesOffsite says whether any section copies off-site: to its switched-on
 *  targets, or to its off-site field while it has no target at all. */
export function copiesOffsite(settings: Settings, targets: OffsiteTarget[]): boolean {
  return SECTIONS.some((domain) => {
    const own = targets.filter((x) => x.domain === domain);
    return own.length > 0 ? own.some((x) => x.enabled) : !!settings[`${domain}Offsite`];
  });
}

/** offsiteKeptOff says the user keeps every backup on the premises and no
 *  section copies off-site, so there is nothing off-site to set. */
export function offsiteKeptOff(settings: Settings, targets: OffsiteTarget[]): boolean {
  return settings.noOffsiteByChoice && !copiesOffsite(settings, targets);
}
