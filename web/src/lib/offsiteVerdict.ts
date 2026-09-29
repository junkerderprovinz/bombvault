import type { OkEnvelope } from "./api";
import type { useT } from "./i18n";
import type { Verdict } from "./useTestVerdict";

/**
 * offsiteVerdict reads the answer of an off-site connection test. A reachable
 * destination without a repository passes: the first replication creates the
 * repository, so the line above the buttons only says so.
 */
export function offsiteVerdict(
  r: OkEnvelope & { reachable?: boolean; initialized?: boolean },
  t: ReturnType<typeof useT>["t"],
): Verdict {
  if (r.ok && r.reachable) return { ok: true, note: r.initialized ? undefined : t("offsite.testNoRepoYet") };
  return { ok: false, reason: r.error ?? t("offsite.testFailed") };
}
