import { useCallback, useSyncExternalStore } from "react";
import { getPlacementOptions, PLACEMENT_DOMAINS, type PlacementDomain, type PlacementOptions } from "./api";
import { subscribePlacement } from "./placementEvents";
import { subscribeRepos } from "./useNamedRepos";
import { subscribeOffsiteTargets } from "./useOffsiteTargets";

export interface PlacementOptionsState {
  options: PlacementOptions | null;
  error: string | null;
}

interface Entry {
  state: PlacementOptionsState;
  listeners: Set<() => void>;
  seq: number;
  stop: () => void;
}

// One entry per domain while a card of it is mounted: a page of forty cards asks
// once, and the next page starts from the server again.
const entries = new Map<PlacementDomain, Entry>();

const PENDING = Object.fromEntries(
  PLACEMENT_DOMAINS.map((d) => [d, { options: null, error: null }])
) as Record<PlacementDomain, PlacementOptionsState>;

function publish(entry: Entry, next: Partial<PlacementOptionsState>): void {
  entry.state = { ...entry.state, ...next };
  entry.listeners.forEach((fn) => fn());
}

function load(domain: PlacementDomain): void {
  const entry = entries.get(domain);
  if (!entry) return;
  const seq = ++entry.seq;
  const current = () => entries.get(domain) === entry && entry.seq === seq;
  getPlacementOptions(domain)
    .then((r) => {
      if (current()) publish(entry, r.ok && r.options ? { options: r.options, error: null } : { error: r.error ?? "" });
    })
    .catch((err: unknown) => {
      if (current()) publish(entry, { error: err instanceof Error ? err.message : "" });
    });
}

function subscribe(domain: PlacementDomain, onChange: () => void): () => void {
  let entry = entries.get(domain);
  if (!entry) {
    const reload = () => load(domain);
    const offs = [subscribeOffsiteTargets(reload), subscribeRepos(reload), subscribePlacement(reload)];
    entry = { state: PENDING[domain], listeners: new Set(), seq: 0, stop: () => offs.forEach((off) => off()) };
    entries.set(domain, entry);
    load(domain);
  }
  entry.listeners.add(onChange);
  return () => {
    const current = entries.get(domain);
    if (!current) return;
    current.listeners.delete(onChange);
    if (current.listeners.size === 0) {
      current.stop();
      entries.delete(domain);
    }
  };
}

/** usePlacementOptions is what a domain's bar offers, loaded once for all its
 *  cards and again after every write it depends on. */
export function usePlacementOptions(domain: PlacementDomain): PlacementOptionsState {
  const sub = useCallback((onChange: () => void) => subscribe(domain, onChange), [domain]);
  return useSyncExternalStore(sub, () => entries.get(domain)?.state ?? PENDING[domain]);
}
