// The run reasons BombVault writes itself, in the reader's language.
//
// Only our own sentences are translated. Most of what lands in runs.error comes
// from restic, rclone or Docker, and translating those would go stale the first
// time one of them rewords an error.
//
// The match is on the exact English string rather than a stored code, because
// the string is what `docker logs`, a support paste or a sqlite3 shell show, and
// it also covers rows written by older versions without a migration.

import type { TranslationKey, useT } from "./i18n";

type T = ReturnType<typeof useT>["t"];

/**
 * The exact strings BombVault writes into runs.error, mapped to their key.
 * Keep in step with the Reason* constants in internal/store/runs.go;
 * runreason_internal_test.go fails when one of them is missing here.
 */
export const RUN_REASONS: Record<string, TranslationKey> = {
  "interrupted (BombVault restarted mid-run)": "runReason.interrupted",
  "aborted: BombVault was shut down": "runReason.shutdown",
  "container no longer exists on the host": "runReason.containerGone",
  "cancelled by the user": "runReason.cancelled",
};

/**
 * Renders a run's reason in the reader's language, or hands back exactly what
 * was stored. A restic or rclone error, or a reason from a version older than
 * this table, passes through untouched.
 */
export function runReason(raw: string | null | undefined, t: T): string {
  if (!raw) return "";
  const key = RUN_REASONS[raw.trim()];
  return key ? t(key) : raw;
}

/**
 * Whether a reason is one of ours, i.e. whether runReason will translate it.
 * Callers use this for direction: a translated sentence follows the page, while
 * an untranslated restic error is technical text that stays left-to-right even
 * on an Arabic or Hebrew page.
 */
export function isOwnReason(raw: string | null | undefined): boolean {
  return !!raw && raw.trim() in RUN_REASONS;
}
