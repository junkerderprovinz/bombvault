import { useEffect, useState } from "react";
import type { CloudCredSetInfo } from "./api";
import { getCloudCredSets } from "./api";

/** Event name for "the stored credential sets changed". */
const CRED_SETS_CHANGED = "bv:cred-sets-changed";

/**
 * Announce that the additional named credential sets (#141 stage 2) were
 * created, edited or removed, so every mounted reader refetches.
 *
 * Call this after a WRITE that succeeded, not after a plain read — the
 * subscribers each issue their own GET, so a spurious broadcast costs one
 * request per mounted picker.
 */
export function credSetsChanged(): void {
  window.dispatchEvent(new Event(CRED_SETS_CHANGED));
}

/**
 * The stored credential sets, kept current across components (#173).
 *
 * Every reader of this list subscribes here rather than holding its own
 * fetched-once copy. That is the whole point: the list is EDITED in one place
 * (Settings' CloudCredSetsCard) and CONSUMED in another (each domain's
 * OffsiteTargetsSection credential picker), and those two are mounted on the
 * SAME page at the same time — the Off-site tab renders one section per domain
 * plus the credential card. With a per-component copy, creating a set updated
 * only the editor's own state, so the picker for a newly added off-site target
 * still offered the old list and the just-created set could not be selected
 * until a full page reload — exactly what issue #173 reported ("Had to refresh
 * the browser, then it worked").
 *
 * Refetching on a broadcast (rather than passing the new list around) keeps the
 * server as the one source of truth: the POST is a replace-all whose response
 * blanks the secrets, so a reader must re-GET to see the canonical rows anyway.
 *
 * A failed fetch leaves the previous list in place rather than blanking it — an
 * empty picker would silently drop the target's current selection.
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
