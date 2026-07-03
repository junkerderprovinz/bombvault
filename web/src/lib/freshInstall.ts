import type { Container, VM, DomainStatus } from "./api";
// Fresh = nothing to show yet: no known targets AND no domain has ever backed up successfully.
export function isFreshInstall(containers: Container[], vms: VM[], domains: DomainStatus[]): boolean {
  const noTargets = containers.length === 0 && vms.length === 0;
  const neverBacked = domains.every((d) => d.status === "off" || d.status === "never");
  return noTargets && neverBacked;
}
