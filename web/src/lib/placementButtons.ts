import { destinationForDomain, type OkEnvelope, type PlacementDomain, type PlacementOptions } from "./api";
import type { useT } from "./i18n";
import { lockHint, type PlacementStep } from "./placement";
import { destinationsChanged, useDestinations } from "./useDestinations";
import { offsiteTargetsChanged } from "./useOffsiteTargets";

type T = ReturnType<typeof useT>["t"];

/** The sentence a button that cannot move says instead. */
export function blockedText(t: T, step: Extract<PlacementStep, { kind: "blocked" }>, home: string): string {
  switch (step.reason) {
    case "home-fixed":
      return lockHint(t, "home-fixed", home);
    case "remote":
      return lockHint(t, "own-credentials", home);
    case "last":
      return t("placement.lastButton");
  }
}

/** The destinations that have no target in the domain yet, for a button each. */
export function useNewDestinations(domain: PlacementDomain, options: PlacementOptions | null) {
  const { destinations } = useDestinations();
  const known = new Set(options?.targets.map((x) => x.id) ?? []);
  return destinations.filter((d) => !d.domains.includes(domain) && !known.has(d.id));
}

/**
 * tickDestination creates the domain's target under a destination and hands
 * back its id, or the reason it could not. The target starts unticked for
 * every item, so only the caller's own step lights it.
 */
export async function tickDestination(
  id: string,
  domain: PlacementDomain
): Promise<{ targetId: string } | { error: OkEnvelope }> {
  let r: Awaited<ReturnType<typeof destinationForDomain>>;
  try {
    r = await destinationForDomain(id, domain);
  } catch (err) {
    return { error: { ok: false, error: err instanceof Error ? err.message : undefined } };
  }
  if (!r.ok || !r.target) return { error: r };
  offsiteTargetsChanged();
  destinationsChanged();
  return { targetId: r.target.id };
}
