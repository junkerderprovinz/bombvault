import { useEffect, useState } from "react";
import type { CloudCredSetInfo } from "./api";
import { getCloudCredSets } from "./api";
import type { TranslationKey } from "./i18n";
import { repoDisplayName } from "./directRepo";

const CRED_SETS_CHANGED = "bv:cred-sets-changed";

/**
 * Tells every mounted useCloudCredSets reader to refetch. Call it after a
 * successful write only: each subscriber issues its own GET.
 */
export function credSetsChanged(): void {
  window.dispatchEvent(new Event(CRED_SETS_CHANGED));
}

/** credSetLabel is how a set is named on screen. A set kept for a direct
 *  repository carries that repository's name, and the rest of its label is
 *  said in the reader's language. */
export function credSetLabel(t: (key: TranslationKey) => string, set: CloudCredSetInfo): string {
  return set.keptFor ? t("cloud.credSets.kept").replace("{name}", () => repoDisplayName(t, set.name, set.directOf)) : set.name;
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
