// The run reasons BombVault writes itself, in the reader's language.
//
// Only our own sentences are translated. Most of what lands in runs.error comes
// from restic, rclone or Docker, and translating those would go stale the first
// time one of them rewords an error.
//
// The match is on the exact English string rather than a stored code, because
// the string is what `docker logs`, a support paste or a sqlite3 shell show, and
// it also covers rows written by older versions without a migration.

import { createElement, Fragment, type ReactElement } from "react";
import { dumpLeftRunning, ORPHAN_NOTE } from "./dbdump";
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
 * The reasons a database dump or import writes, which may be followed by
 * ": <detail>" holding the tool's own message. The head is translated, the
 * detail is shown as it was stored. Keep in step with the Reason* and Note*
 * constants in internal/store/runs.go; runreason_internal_test.go fails when
 * one of them is missing here, and pins that none of them begins another.
 */
export const RUN_REASON_PREFIXES: Record<string, TranslationKey> = {
  "database dump failed: the database refused the login": "runReason.dbdumpAuth",
  "database dump failed: the database user lacks a privilege the dump needs": "runReason.dbdumpPrivileges",
  "database dump failed: the database server did not accept a connection": "runReason.dbdumpUnreachable",
  "database dump failed: no dump tool found in the container": "runReason.dbdumpNoClient",
  "database dump failed: a password file could not be read": "runReason.dbdumpSecret",
  "database dump failed: no usable credentials in the container settings": "runReason.dbdumpNoCredentials",
  "database dump failed: the container is paused or restarting": "runReason.dbdumpNotRunning",
  "database dump failed: the database's system tables need an upgrade": "runReason.dbdumpNeedsUpgrade",
  "database dump failed: time limit reached": "runReason.dbdumpTimeout",
  "database dump failed: the backup's own time limit was reached": "runReason.dbdumpBackupCap",
  "database dump failed: no progress": "runReason.dbdumpStalled",
  "database dump failed: the dump was empty": "runReason.dbdumpEmpty",
  "database dump failed: the dump ended before its completion marker": "runReason.dbdumpIncomplete",
  "database dump failed: the dump tool reported an error": "runReason.dbdumpTool",
  "database dump failed: Docker refused the command": "runReason.dbdumpDocker",
  "database dump failed: the repository did not accept it": "runReason.dbdumpRepository",
  "database dump failed: the dump helper gave no result": "runReason.dbdumpHelper",
  "database dump failed: the stored size does not match what was dumped": "runReason.dbdumpMismatch",
  "database dump failed: a damaged dump snapshot could not be removed": "runReason.dbdumpLeftover",
  "database dump covers one database only": "runReason.dbdumpOneDatabase",
  "database dump skipped: its run could not be recorded": "runReason.dbdumpNotRecorded",
  "database import failed before it started, the old data is back in place": "runReason.dbimportPrepare",
  "database import failed and the old data could not be put back": "runReason.dbimportRollback",
  "database import failed: the import tool reported an error": "runReason.dbimportFailed",
  "database imported; the previous data folder was kept": "runReason.dbimportKeptOld",
  "database imported with errors": "runReason.dbimportErrors",
};

/**
 * What an import appends after "; " to name the apps it stopped, followed by
 * ": " and their names. Keep in step with the ImportTail* constants in
 * internal/store/runs.go.
 */
const IMPORT_APP_TAILS: Record<string, TranslationKey> = {
  "could not start these apps again": "runReason.dbimportAppsDown",
  "these apps stay stopped until the data folder is sorted out": "runReason.dbimportAppsStopped",
};

/** Splits the tail naming the apps an import stopped off a note or reason. */
function importAppTail(text: string): { rest: string; key: TranslationKey; apps: string } | undefined {
  for (const [tail, key] of Object.entries(IMPORT_APP_TAILS)) {
    const at = text.lastIndexOf(`; ${tail}: `);
    if (at >= 0) return { rest: text.slice(0, at), key, apps: text.slice(at + tail.length + 4) };
  }
  return undefined;
}

/**
 * The notes a successful run can carry that ask the reader to do something
 * about them. Every other note only records what was kept or skipped, unless
 * it also names an app the import could not start again.
 */
const WARNING_NOTES = [
  "database dump covers one database only",
  "database dump skipped: its run could not be recorded",
  "database imported with errors",
];

/** Whether a note of a successful run reports something worth acting on. */
export function isWarningNote(raw: string | null | undefined): boolean {
  const text = raw?.trim() ?? "";
  if (importAppTail(text)) return true;
  return WARNING_NOTES.some((note) => text === note || text.startsWith(note + ": "));
}

/**
 * A reason split into its translated head, the raw detail behind it and, for a
 * dump that may still be running in the container, the translated note saying so.
 */
export interface RunReasonParts {
  head: string;
  detail: string;
  note?: string;
}

/**
 * Splits a stored reason into the sentence BombVault wrote and the detail a
 * tool added. An exact match wins, so a reason that stands alone never goes
 * through the prefix search. Anything unknown becomes the head unchanged.
 */
export function runReasonParts(raw: string | null | undefined, t: T): RunReasonParts {
  const text = raw?.trim() ?? "";
  if (dumpLeftRunning(text)) {
    const parts = runReasonParts(text.slice(0, -(ORPHAN_NOTE.length + 2)), t);
    return { ...parts, note: t("runReason.dbdumpOrphan") };
  }
  const apps = importAppTail(text);
  if (apps) {
    return { ...runReasonParts(apps.rest, t), note: t(apps.key).replace("{apps}", apps.apps) };
  }
  if (!text) return { head: "", detail: "" };

  const exact = RUN_REASONS[text] ?? RUN_REASON_PREFIXES[text];
  if (exact) return { head: t(exact), detail: "" };

  for (const [reason, key] of Object.entries(RUN_REASON_PREFIXES)) {
    if (text.startsWith(reason + ": ")) {
      return { head: t(key), detail: text.slice(reason.length + 2) };
    }
  }
  return { head: raw ?? "", detail: "" };
}

/**
 * Renders a run's reason in the reader's language, or hands back exactly what
 * was stored. A restic or rclone error, or a reason from a version older than
 * this table, passes through untouched. Plain text for a toast, a tooltip or
 * the activity log; RunReasonText is the rendered form.
 */
export function runReason(raw: string | null | undefined, t: T): string {
  const { head, detail, note } = runReasonParts(raw, t);
  const reason = detail ? `${head}: ${detail}` : head;
  return note ? `${reason}; ${note}` : reason;
}

/**
 * RunReasonText renders a reason with its detail isolated. The head is
 * translated and follows the page, while the detail is a tool's own message:
 * on an Arabic or Hebrew page it stays left-to-right instead of scattering its
 * punctuation across the sentence.
 */
export function RunReasonText({
  reason,
  t,
}: {
  reason: string | null | undefined;
  t: T;
}): ReactElement {
  const { head, detail, note } = runReasonParts(reason, t);
  const tail = note ? ["; ", note] : [];
  if (!detail) return createElement(Fragment, null, head, ...tail);
  return createElement(Fragment, null, head, ": ", createElement("bdi", { dir: "ltr" }, detail), ...tail);
}

/**
 * Whether a reason is one of ours and stands alone, i.e. whether the whole
 * string is a sentence BombVault wrote. Callers use this for direction: a
 * translated sentence follows the page, while an untranslated restic error, or
 * one of ours with a tool's message appended, is technical text that stays
 * left-to-right even on an Arabic or Hebrew page.
 */
export function isOwnReason(raw: string | null | undefined): boolean {
  if (!raw) return false;
  const text = raw.trim();
  return text in RUN_REASONS || text in RUN_REASON_PREFIXES;
}
