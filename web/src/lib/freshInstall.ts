import type { DomainStatus } from "./api";

// isFreshInstall reports a BombVault that has never backed anything up: at least
// one domain is configured and each is either off or has never succeeded. The
// container count is no signal, because listContainers always includes
// BombVault's own container.
export function isFreshInstall(domains: DomainStatus[]): boolean {
  return domains.length > 0 && domains.every((d) => d.status === "off" || d.status === "never");
}
