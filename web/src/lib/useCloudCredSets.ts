import { useEffect, useState } from "react";
import type { CloudCredSet, CloudCredSetInfo, OkEnvelope, SaveWarning } from "./api";
import { getCloudCredSets, setCloudCredSets } from "./api";
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

// credSetDraft blanks the secrets of a stored set. The backend keeps a stored
// secret when the posted one is blank (matched by id), so resending untouched
// sets this way preserves their keys.
export function credSetDraft(s: CloudCredSetInfo): CloudCredSet {
  return { id: s.id, name: s.name, s3KeyId: s.s3KeyId, s3Secret: "", s3Region: s.s3Region, restUser: s.restUser, restPassword: "", s3StorageClass: s.s3StorageClass };
}

/** saveCredSet stores one set among the stored ones and tells the readers.
 *  The list is saved as a whole, so every other set goes along as a draft. */
export async function saveCredSet(
  stored: CloudCredSetInfo[],
  set: CloudCredSet
): Promise<OkEnvelope & { warnings?: SaveWarning[] }> {
  const r = await setCloudCredSets([...stored.filter((s) => s.id !== set.id).map(credSetDraft), set]);
  if (r.ok) credSetsChanged();
  return r;
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
