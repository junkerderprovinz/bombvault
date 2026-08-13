import { useEffect, useState } from "react";
import type { OffsiteTarget } from "./api";
import { listOffsiteTargets } from "./api";

/** The five domains that can carry an off-site destination. */
export type OffsiteDomain = "containers" | "vms" | "flash" | "config" | "files";

/**
 * A domain's ENABLED off-site targets, in the same order the backend resolves
 * them (sortOrder, then createdAt) — so index 0 is the PRIMARY target that a
 * bare "offsite" source addresses, and every other entry needs "offsite:<id>".
 *
 * Returns [] while loading, on error, and for a domain configured only through
 * the legacy Settings columns (no target rows yet). Every caller treats "fewer
 * than two targets" as "no choice to offer" and falls back to the plain
 * local/off-site behaviour, so a failed fetch degrades to today's UI.
 */
export function useOffsiteTargets(domain?: OffsiteDomain): OffsiteTarget[] {
  const [targets, setTargets] = useState<OffsiteTarget[]>([]);

  useEffect(() => {
    if (!domain) {
      setTargets([]);
      return;
    }
    let active = true;
    listOffsiteTargets(domain)
      .then((r) => {
        if (!active) return;
        setTargets(r.ok ? (r.targets ?? []).filter((t) => t.enabled) : []);
      })
      .catch(() => {
        if (active) setTargets([]);
      });
    return () => {
      active = false;
    };
  }, [domain]);

  return targets;
}

/** A target's display name: its label, else the repo location it points at. */
export function offsiteTargetLabel(target: OffsiteTarget): string {
  return target.name.trim() || target.repo;
}

/**
 * The source string addressing a target within its domain. Index 0 (the
 * primary) keeps the bare "offsite" form so the default selection is byte
 * identical to before the picker existed.
 */
export function offsiteTargetSource(target: OffsiteTarget, index: number): string {
  return index === 0 ? "offsite" : `offsite:${target.id}`;
}
