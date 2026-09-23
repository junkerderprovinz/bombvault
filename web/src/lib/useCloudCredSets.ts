import { useEffect, useState } from "react";
import type { CloudCredSetInfo } from "./api";
import { getCloudCredSets } from "./api";

const CRED_SETS_CHANGED = "bv:cred-sets-changed";

/**
 * Tells every mounted useCloudCredSets reader to refetch. Call it after a
 * successful write only: each subscriber issues its own GET.
 */
export function credSetsChanged(): void {
  window.dispatchEvent(new Event(CRED_SETS_CHANGED));
}

/**
 * The stored credential sets, kept current across components. The list is
 * edited in Settings' CloudCredSetsCard and read by each OffsiteTargetsSection
 * picker on the same page, so a per-component copy would hide a newly created
 * set until a reload.
 *
 * Readers refetch on a broadcast instead of receiving the new list because the
 * POST response blanks the secrets. A failed fetch keeps the previous list, since
 * an empty picker would drop the target's current selection.
 */
export function useCloudCredSets(): CloudCredSetInfo[] {
  const [sets, setSets] = useState<CloudCredSetInfo[]>([]);

  useEffect(() => {
    let active = true;
    const load = () => {
      getCloudCredSets()
        .then((r) => {
          if (active && r.ok) setSets(r.sets ?? []);
        })
        .catch(() => undefined);
    };
    load();
    window.addEventListener(CRED_SETS_CHANGED, load);
    return () => {
      active = false;
      window.removeEventListener(CRED_SETS_CHANGED, load);
    };
  }, []);

  return sets;
}
