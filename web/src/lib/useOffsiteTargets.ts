import { useEffect, useState } from "react";
import type { OffsiteTarget, OffsiteDomain } from "./api";
import { listOffsiteTargets } from "./api";

/** Re-exported so the many callers that import it from here keep working; the
 *  definition itself lives in api.ts, which is the one place it can be edited
 *  without leaving a stale copy behind. */
export type { OffsiteDomain } from "./api";

/** Event name for "a domain's off-site target rows changed". */
const OFFSITE_TARGETS_CHANGED = "bv:offsite-targets-changed";

/**
 * Announce that off-site target rows were created, edited or removed, so every
 * mounted reader refetches. Broadcast after a successful WRITE only.
 *
 * Deliberately not per-domain: the write paths that matter (the per-domain
 * editor, and accepting a fleet mesh offer, which mints a target for whichever
 * domain the peer offered) are rare user actions, and a domain filter here
 * would only trade one refetch for a class of missed invalidations.
 */
export function offsiteTargetsChanged(): void {
  window.dispatchEvent(new Event(OFFSITE_TARGETS_CHANGED));
}

/**
 * Subscribe to that broadcast; returns the unsubscribe function.
 *
 * For readers that cannot use the hook below because they keep their own
 * derived view of the list (OffsiteTargetsSection shows only the ADDITIONAL
 * targets, sortOrder > 0, and tracks its own load/error state). They still
 * refetch through the same one event, so the event name has a single owner.
 */
export function subscribeOffsiteTargets(onChange: () => void): () => void {
  window.addEventListener(OFFSITE_TARGETS_CHANGED, onChange);
  return () => window.removeEventListener(OFFSITE_TARGETS_CHANGED, onChange);
}

/**
 * A domain's ENABLED off-site targets, in the same order the backend resolves
 * them (sortOrder, then createdAt) — so index 0 is the PRIMARY target that a
 * bare "offsite" source addresses, and every other entry needs "offsite:<id>".
 *
 * Returns [] while loading, on error, and for a domain configured only through
 * the legacy Settings columns (no target rows yet). Every caller treats "fewer
 * than two targets" as "no choice to offer" and falls back to the plain
 * local/off-site behaviour, so a failed fetch degrades to today's UI.
 *
 * Re-reads on the broadcast above for the same reason useCloudCredSets does
 * (#173): a reader and the editor that changes this list are mounted at once.
 * Settings' own TestConnectionButton is the live case — it labels itself "Test
 * primary" only once a domain holds more than one target, and that count came
 * from a fetched-once copy, so adding a second target in the section directly
 * below it left the button claiming to test the only destination until reload.
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
 * The source string addressing a target within its domain. Index 0 (the
 * primary) keeps the bare "offsite" form so the default selection is byte
 * identical to before the picker existed.
 */
export function offsiteTargetSource(target: OffsiteTarget, index: number): string {
  return index === 0 ? "offsite" : `offsite:${target.id}`;
}
