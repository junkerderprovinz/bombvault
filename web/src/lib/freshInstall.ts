import type { DomainStatus } from "./api";
// Fresh = a BombVault that has never backed up anything yet: at least one domain
// is configured and EVERY domain is either off or has never had a successful
// backup. The docker-container count is deliberately NOT a signal: listContainers
// always includes BombVault's own container, so it is never 0 on a healthy host —
// basing "fresh" on it made the disaster-recovery nudge dead (it only fired on a
// Docker outage). The backup signal fires correctly for a brand-new install AND a
// rebuilt one recovering from existing backups.
export function isFreshInstall(domains: DomainStatus[]): boolean {
  return domains.length > 0 && domains.every((d) => d.status === "off" || d.status === "never");
}
