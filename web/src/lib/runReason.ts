// The run reasons BombVault writes itself, in the reader's language ([377]).
//
// Measured on jdp's box before this existed: the activity log read
// "Unraid flash-Backup fehlgeschlagen: interrupted (BombVault restarted
// mid-run)" and "MinIO-Backup übersprungen: container no longer exists on the
// host". German label, German verb, English reason. In an interface that ships
// 42 languages, the half that explains WHY was the half nobody outside English
// could read.
//
// Only OUR sentences are here. Most of what lands in runs.error comes from
// restic, rclone or Docker, and translating those would mean translating three
// other projects and going stale the first time one of them rewords an error.
// These three we wrote, so these three we can answer for.
//
// Matching on the exact English string rather than a stored code is deliberate.
// A code would be cleaner in the abstract and worse in every concrete place the
// column is actually read: `docker logs`, a support paste, a sqlite3 shell. It
// also handles rows written by older versions for free, which a code cannot do
// without a migration that rewrites history to make today's UI tidier.
//
// The pairing is pinned from both sides. Go has these as named constants
// (internal/store/runs.go) with a test that they still match the list below.

import type { TranslationKey, useT } from "./i18n";

type T = ReturnType<typeof useT>["t"];

/**
 * The exact strings BombVault writes into runs.error, mapped to their key.
 *
 * Keep in step with internal/store/runs.go's Reason* constants. Adding one here
 * without adding it there (or the reverse) fails runReason.test.ts.
 */
export const RUN_REASONS: Record<string, TranslationKey> = {
  "interrupted (BombVault restarted mid-run)": "runReason.interrupted",
  "aborted: BombVault was shut down": "runReason.shutdown",
  "container no longer exists on the host": "runReason.containerGone",
};

/**
 * Render a run's reason in the reader's language, or hand back exactly what was
 * stored.
 *
 * The fallback is the whole point: a restic error, an rclone error and a reason
 * from a version older than this table all pass through untouched, which is
 * right for every one of them. Nothing here may swallow or reword a message it
 * does not recognise.
 */
export function runReason(raw: string | null | undefined, t: T): string {
  if (!raw) return "";
  const key = RUN_REASONS[raw.trim()];
  return key ? t(key) : raw;
}

/**
 * Whether a reason is one of ours, i.e. whether runReason will translate it.
 *
 * Callers use this for direction: our translated sentences follow the page (and
 * must not be forced to `dir="ltr"`), while an untranslated restic error is
 * Latin-script technical text that has to stay left-to-right even on an Arabic
 * or Hebrew page.
 */
export function isOwnReason(raw: string | null | undefined): boolean {
  return !!raw && raw.trim() in RUN_REASONS;
}
