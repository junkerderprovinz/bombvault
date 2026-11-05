import type { OkEnvelope } from "./api";
import type { TranslationKey, useT } from "./i18n";
import type { Verdict } from "./useTestVerdict";

/** The code of a test that found the repository it reached before gone. */
export const REPO_GONE = "repo-gone";

/** What a test or a copy can be refused for that the user has to put right. */
export const REFUSAL_KEYS: Record<string, TranslationKey> = {
  "repo-wrong-key": "placementCode.repoWrongKey",
  "bucket-missing": "placementCode.bucketMissing",
};

/**
 * offsiteVerdict reads the answer of an off-site connection test. A reachable
 * destination without a repository passes: the first replication creates the
 * repository, so the line above the buttons only says so. One that held a
 * repository before fails, because no copy creates a new one there on its own.
 */
export function offsiteVerdict(
  r: OkEnvelope & { reachable?: boolean; initialized?: boolean },
  t: ReturnType<typeof useT>["t"],
): Verdict {
  if (r.ok && r.reachable) return { ok: true, note: r.initialized ? undefined : t("offsite.testNoRepoYet") };
  if (r.code === REPO_GONE) return { ok: false, reason: t("placementCode.repoGone") };
  const refused = r.code ? REFUSAL_KEYS[r.code] : undefined;
  if (refused) return { ok: false, reason: t(refused) };
  return { ok: false, reason: r.error ?? t("offsite.testFailed") };
}
