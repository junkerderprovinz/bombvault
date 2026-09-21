// What the pages need to say about a database dump: the engine's own name, the
// sentence for a container's data coverage, the advice for a failed dump and
// the pairing between a dump and the volume snapshot taken with it.

import type { Container, DbDataCoverage, DbEngine } from "./api";
import type { TranslationKey } from "./i18n";

/** The engines as their makers write them, so no table translates a product name. */
export const ENGINE_NAMES = {
  postgres: "PostgreSQL",
  mysql: "MySQL",
  mariadb: "MariaDB",
} as const;

const DB_DUMP_PREFIX = "dbdump:";

/** Whether a snapshot tag identifies a database dump rather than a file backup. */
export function isDbDumpIdentity(tag: string): boolean {
  return tag.startsWith(DB_DUMP_PREFIX);
}

/** The container name in a dump identity tag, "" for any other tag. */
export function dbDumpNameOf(tag: string): string {
  return isDbDumpIdentity(tag) ? tag.slice(DB_DUMP_PREFIX.length) : "";
}

/**
 * Whether a dump was taken in the same backup as a snapshot. An off-site copy
 * carries a new id and keeps the source id in `original`, so the pairing shows
 * on a remote source too.
 */
export function pairsWith(
  dump: { pairedSnapshotId?: string },
  snap: { id: string; original?: string }
): boolean {
  if (!dump.pairedSnapshotId) return false;
  return dump.pairedSnapshotId === (snap.original || snap.id);
}

/**
 * A cancelled dump is recorded as a failed run whose reason is the
 * cancellation, so the reason decides how it is worded, not the status.
 */
export function dumpWasCancelled(reason: string | null | undefined): boolean {
  return reason?.trim() === "cancelled by the user";
}

/**
 * Whether an import's note reports errors from the import tool. Every
 * successful import carries a note, and most of them only say where the
 * previous data folder was kept.
 */
export function importHadErrors(note: string | null | undefined): boolean {
  return (note ?? "").startsWith("database imported with errors");
}

/** The refusal ids POST /dbdumps/{id}/import answers with, one sentence each. */
const IMPORT_REFUSED: Record<string, TranslationKey> = {
  busy: "dbdump.importRefused.busy",
  notRunning: "dbdump.importRefused.notRunning",
  engineMismatch: "dbdump.importRefused.engineMismatch",
  noDataMount: "dbdump.importRefused.noDataMount",
  version: "dbdump.importRefused.version",
  damaged: "dbdump.importRefused.damaged",
  notADump: "dbdump.importRefused.notADump",
};

/**
 * The sentence for a refused import, or null for a failure of another kind.
 * PostgreSQL reads a dump on the same or a newer major version, MySQL and
 * MariaDB only on the same one, so a version refusal says which applies.
 */
export function importRefusedKey(code: string | undefined, engine: DbEngine): TranslationKey | null {
  if (code === "version" && engine !== "postgres") return "dbdump.importRefused.versionSameMajor";
  return IMPORT_REFUSED[code ?? ""] ?? null;
}

/**
 * The warning an image update carries for a database, or null for any other
 * container. A dump taken before the update helps on the new version only
 * where the engine reads an older dump, and MySQL and MariaDB do not across a
 * major version.
 */
export function updateWarnKey(
  c: Pick<Container, "dbTier" | "dbEngine" | "dbDumpEngine" | "dbSuggestedEngine">
): TranslationKey | null {
  if (c.dbTier === "") return null;
  const engine = c.dbEngine || c.dbDumpEngine || c.dbSuggestedEngine;
  return engine === "postgres" ? "dbdump.updateWarn" : "dbdump.updateWarnSameMajor";
}

/**
 * The databases the one-time notice introduces: those recognised by their image
 * whose dump runs. A dump switched off for every container, or for one on its
 * card or by its label, is not news to whoever did it.
 */
export function introDatabases(containers: Container[]): Container[] {
  if (containers.some((c) => c.dbDumpsGlobalOff)) return [];
  return containers.filter((c) => c.dbTier === "curated" && !c.dbDumpOff && !c.dbDumpLabelOff);
}

const COVERAGE_KEYS: Record<string, TranslationKey> = {
  stopped: "dbdump.coverageStopped",
  live: "dbdump.coverageLive",
  none: "dbdump.coverageNone",
  unknown: "dbdump.coverageUnknown",
};

/** The line describing what this container's files backup is worth, or null
 *  when the backend reported no coverage at all. */
export function coverageKey(coverage: DbDataCoverage): TranslationKey | null {
  return COVERAGE_KEYS[coverage] ?? null;
}

/**
 * The advice for a failed dump, keyed by the reason the run recorded. Several
 * failures share one remedy where the same thing has to be done about them.
 * Keep in step with RUN_REASON_PREFIXES: dbdump.test.ts fails when a dump
 * failure added there has no entry here.
 */
const REMEDIES: Record<string, TranslationKey> = {
  "database dump failed: the database refused the login": "dbdump.fixAuth",
  "database dump failed: the database user lacks a privilege the dump needs": "dbdump.fixPrivileges",
  "database dump failed: the database server did not accept a connection": "dbdump.fixUnreachable",
  "database dump failed: the container is paused or restarting": "dbdump.fixNotRunning",
  "database dump failed: no dump tool found in the container": "dbdump.fixNoClient",
  "database dump failed: a password file could not be read": "dbdump.fixSecret",
  "database dump failed: no usable credentials in the container settings": "dbdump.fixNoCredentials",
  "database dump failed: the database's system tables need an upgrade": "dbdump.fixNeedsUpgrade",
  "database dump failed: time limit reached": "dbdump.fixTimeout",
  "database dump failed: the backup's own time limit was reached": "dbdump.fixBackupCap",
  "database dump failed: no progress": "dbdump.fixStalled",
  "database dump failed: the dump helper gave no result": "dbdump.fixHelper",
  "database dump failed: the dump was empty": "dbdump.fixDumpData",
  "database dump failed: the dump ended before its completion marker": "dbdump.fixDumpData",
  "database dump failed: the dump tool reported an error": "dbdump.fixDumpData",
  "database dump failed: Docker refused the command": "dbdump.fixDocker",
  "database dump failed: the repository did not accept it": "dbdump.fixRepository",
  "database dump failed: the stored size does not match what was dumped": "dbdump.fixRepository",
  "database dump failed: a damaged dump snapshot could not be removed": "dbdump.fixLeftover",
};

/**
 * remedyKey maps a recorded reason to the advice shown next to it. A reason
 * that carries the tool's own message behind ": " still matches. Cancellations,
 * the success notes, imports and errors from restic or Docker have no advice of
 * ours, and their callers then render no bubble.
 */
export function remedyKey(reason: string | null | undefined): TranslationKey | null {
  const text = reason?.trim() ?? "";
  if (!text) return null;
  const exact = REMEDIES[text];
  if (exact) return exact;
  for (const [failure, key] of Object.entries(REMEDIES)) {
    if (text.startsWith(failure + ": ")) return key;
  }
  return null;
}
