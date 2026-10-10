import { useEffect, useState } from "react";
import { adoptIntoDestination, listDestinations, type Destination, type OkEnvelope } from "./api";
import { offsiteTargetsChanged } from "./useOffsiteTargets";

const DESTINATIONS_CHANGED = "bv:destinations-changed";

/** Tells every mounted reader that destinations changed. */
export function destinationsChanged(): void {
  window.dispatchEvent(new Event(DESTINATIONS_CHANGED));
}

/** Hangs a target typed in by hand on a destination and tells the readers of
 *  both lists. The answer says whether the server took it. */
export async function adoptTarget(destinationId: string, targetId: string): Promise<OkEnvelope> {
  const res = await adoptIntoDestination(destinationId, targetId).catch(
    (e: unknown): OkEnvelope => ({ ok: false, error: e instanceof Error ? e.message : undefined })
  );
  if (res.ok) {
    destinationsChanged();
    offsiteTargetsChanged();
  }
  return res;
}

/**
 * The destinations, refetched on destinationsChanged. `error` is set when the
 * list could not be read, so a screen can tell an empty list from a failed one.
 */
export function useDestinations(): { destinations: Destination[]; loaded: boolean; error: string | null } {
  const [state, setState] = useState<{ destinations: Destination[]; loaded: boolean; error: string | null }>({
    destinations: [],
    loaded: false,
    error: null,
  });
  useEffect(() => {
    let alive = true;
    const load = () => {
      listDestinations()
        .then((r) => {
          if (!alive) return;
          setState(r.ok ? { destinations: r.destinations ?? [], loaded: true, error: null } : (s) => ({ ...s, error: r.error ?? "" }));
        })
        .catch((e: unknown) => alive && setState((s) => ({ ...s, error: e instanceof Error ? e.message : "" })));
    };
    load();
    window.addEventListener(DESTINATIONS_CHANGED, load);
    return () => {
      alive = false;
      window.removeEventListener(DESTINATIONS_CHANGED, load);
    };
  }, []);
  return state;
}
