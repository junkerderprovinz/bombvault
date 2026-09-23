import { useEffect, useState } from "react";
import type { OffsiteTarget, OffsiteDomain } from "./api";
import { listOffsiteTargets } from "./api";

export type { OffsiteDomain } from "./api";

const OFFSITE_TARGETS_CHANGED = "bv:offsite-targets-changed";

/**
 * Tells every mounted reader that off-site target rows changed. Call it after
 * a successful write. The event is not per-domain: writes are rare, and a
 * domain filter would risk missed invalidations to save one refetch.
 */
export function offsiteTargetsChanged(): void {
  window.dispatchEvent(new Event(OFFSITE_TARGETS_CHANGED));
}

/**
 * Subscribes to offsiteTargetsChanged and returns the unsubscribe function.
 * For readers that keep their own view of the list and cannot use the hook,
 * such as OffsiteTargetsSection.
 */
export function subscribeOffsiteTargets(onChange: () => void): () => void {
  window.addEventListener(OFFSITE_TARGETS_CHANGED, onChange);
  return () => window.removeEventListener(OFFSITE_TARGETS_CHANGED, onChange);
}

/**
 * A domain's enabled off-site targets in backend order (sortOrder, then
 * createdAt): index 0 is the primary that a bare "offsite" source addresses,
 * every other entry needs "offsite:<id>".
 *
 * Returns [] while loading, on error, and for a domain configured only through
 * the legacy Settings columns. Callers treat fewer than two targets as no
 * choice to offer, so a failed fetch falls back to the plain local/off-site UI.
 *
 * Refetches on offsiteTargetsChanged because the editor and readers such as
 * TestConnectionButton ("Test primary") are mounted on the same page.
 */
export function useOffsiteTargets(domain?: OffsiteDomain): OffsiteTarget[] {
  const [targets, setTargets] = useState<OffsiteTarget[]>([]);

  useEffect(() => {
    if (!domain) {
      setTargets([]);
      return;
    }
    let active = true;
    const load = () => {
      listOffsiteTargets(domain)
        .then((r) => {
          if (!active) return;
          setTargets(r.ok ? (r.targets ?? []).filter((t) => t.enabled) : []);
        })
        .catch(() => {
          if (active) setTargets([]);
        });
    };
    load();
    window.addEventListener(OFFSITE_TARGETS_CHANGED, load);
    return () => {
      active = false;
      window.removeEventListener(OFFSITE_TARGETS_CHANGED, load);
    };
  }, [domain]);

  return targets;
}

/** A target's display name: its label, else the repo location it points at. */
export function offsiteTargetLabel(target: OffsiteTarget): string {
  return target.name.trim() || target.repo;
}

/**
 * The source string addressing a target within its domain. The primary keeps
 * the bare "offsite" form so the default selection matches what a
 * single-target domain sends.
 */
export function offsiteTargetSource(target: OffsiteTarget, index: number): string {
  return index === 0 ? "offsite" : `offsite:${target.id}`;
}
