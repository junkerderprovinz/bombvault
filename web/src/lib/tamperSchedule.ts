import type { OffsiteTarget, Settings } from "./api";

const FLAGS = [
  ["containersOffsite", "containersOffsiteImmutable"],
  ["vmsOffsite", "vmsOffsiteImmutable"],
  ["flashOffsite", "flashOffsiteImmutable"],
  ["configOffsite", "configOffsiteImmutable"],
  ["filesOffsite", "filesOffsiteImmutable"],
  ["zfsOffsite", "zfsOffsiteImmutable"],
] as const;

/**
 * tamperPlanned mirrors the scheduler: the tamper test is planned for a
 * section whose off-site settings are append-only, and for every switched-on
 * append-only target, primary or additional.
 */
export function tamperPlanned(settings: Settings, targets: OffsiteTarget[]): boolean {
  return (
    FLAGS.some(([repo, flag]) => settings[repo] !== "" && settings[flag]) || targets.some((t) => t.enabled && t.immutable)
  );
}
