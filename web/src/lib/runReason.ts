// The run reasons BombVault writes itself, in the reader's language.
//
// Only our own sentences are translated. Most of what lands in runs.error comes
// from restic, rclone or Docker, and translating those would go stale the first
// time one of them rewords an error.
//
// The match is on the exact English string rather than a stored code, because
// the string is what `docker logs`, a support paste or a sqlite3 shell show, and
// it also covers rows written by older versions without a migration.

import { createElement, Fragment, type ReactElement, type ReactNode } from "react";
import { dumpLeftRunning, ORPHAN_NOTE } from "./dbdump";
import { DEPOSIT_CODE_KEY } from "./depositCodes";
import { refusalKey } from "./emergency";
import type { TranslationKey, useT } from "./i18n";

/** The code a refused key run stores, as its sentence. */
function keyRunKey(code: string): TranslationKey | undefined {
  return refusalKey(code) ?? (code in DEPOSIT_CODE_KEY ? DEPOSIT_CODE_KEY[code as keyof typeof DEPOSIT_CODE_KEY] : undefined);
}

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
  "this would write into BombVault's own data folder or a folder that holds it; choose another folder": "runReason.ownDataFolder",
  "the app was still busy when the wait for it ended": "runReason.deferralGaveUp",
  "the wait for the app ended because the container left the schedule": "runReason.deferralUnscheduled",
  "the wait for the app ended because the containers were switched off": "runReason.deferralSectionOff",
};

/**
 * Why a backup waits for its app, with the name or number it carries. Keep in
 * step with the Deferred* functions in internal/store/runs.go. The count picks
 * the plural, and the other values fill the placeholders.
 */
const DEFERRALS: { re: RegExp; key: TranslationKey; fill: (m: RegExpExecArray) => { count?: number; values: Record<string, string> } }[] = [
  {
    re: /^(Plex|Jellyfin|Emby) has (\d+) active sessions?$/,
    key: "runReason.deferredSessions",
    fill: (m) => ({ count: Number(m[2]), values: { server: m[1] } }),
  },
  { re: /^(Plex|Jellyfin|Emby) did not answer$/, key: "runReason.deferredNoAnswer", fill: (m) => ({ values: { server: m[1] } }) },
  { re: /^(Plex|Jellyfin|Emby) has no API key set$/, key: "runReason.deferredNoKey", fill: (m) => ({ values: { server: m[1] } }) },
  {
    re: /^the container sent (\d+) KiB\/s or more within the last (?:(\d+) minutes|1 minute)$/,
    key: "runReason.deferredSending",
    fill: (m) => ({ count: Number(m[2] ?? 1), values: { kib: m[1] } }),
  },
  {
    re: /^BombVault has watched the container's traffic for less than (?:(\d+) minutes|1 minute)$/,
    key: "runReason.deferredMeasuring",
    fill: (m) => ({ count: Number(m[1] ?? 1), values: {} }),
  },
];

function deferralReason(text: string, t: T): string | undefined {
  for (const { re, key, fill } of DEFERRALS) {
    const m = re.exec(text);
    if (!m) continue;
    const { count, values } = fill(m);
    let out: string = t(key, count);
    for (const [name, value] of Object.entries(values)) out = out.replaceAll(`{${name}}`, value);
    return out;
  }
  return undefined;
}

/** What a container restore writes when Docker refused the GPU or runtime of
 *  the container, followed by ": " and Docker's refusal. Keep in step with
 *  ReasonRestoreNoRuntime in internal/store/runs.go. */
export const RESTORE_NO_RUNTIME = "restore failed: the container used a GPU or runtime this host does not have";

/** The note of the restore that left them out. Keep in step with
 *  NoteRestoredWithoutRuntime. */
export const RESTORED_WITHOUT_RUNTIME = "restored without the GPU or runtime the container used";

/** The note of a restore that left out links Docker would refuse, followed by
 *  ": " and the containers. Keep in step with NoteRestoredWithoutLinks. */
export const RESTORED_WITHOUT_LINKS = "restored without links to containers that are not running";

/** The note of a backup, update or restore whose other containers did not
 *  start again, followed by ": " and their names. Keep in step with
 *  NoteContainersNotBack. */
export const CONTAINERS_NOT_BACK = "some containers did not start again";

/** The note of a backup that left containers running on the network of one it
 *  stopped and started, followed by ": " and their names. Keep in step with
 *  NoteNoNetworkUntilRestart. */
export const NO_NETWORK_UNTIL_RESTART = "some containers have no network until they are restarted";

/** The note of a container backup whose data folders sit outside Host Data,
 *  followed by ": " and their host paths. Keep in step with NoteDataOutsideHost. */
export const DATA_OUTSIDE_HOST = "data folders outside Host Data were not backed up";

/** The note of a backup that left out files restic could not read, followed by
 *  ": " and restic's reason. Keep in step with NoteFilesUnreadable. */
export const FILES_UNREADABLE = "some files could not be read and are missing from this backup";

/** The note of a backup that found selected folders gone or emptied, followed by
 *  ": " and their host paths. Keep in step with NoteFoldersMissing. */
export const FOLDERS_MISSING = "selected folders are missing from this backup";

/** The note of a backup that read no files where earlier ones held some. Keep
 *  in step with NoteBackupEmpty. */
export const BACKUP_EMPTY = "this backup holds no files, while earlier backups held some";

/** Follows any note of a backup that holds less than it should. Keep in step
 *  with NoteOlderBackupsKept. */
export const OLDER_BACKUPS_KEPT = "older backups were kept";

/** The note on a member's run when its stack's project folder is empty,
 *  followed by ": " and the folder. Keep in step with NoteStackFolderEmpty. */
export const STACK_FOLDER_EMPTY = "the project folder of this stack holds no files and was not backed up";

/** The note of a restore that left folders of the backup alone, followed by
 *  ": " and the folders. Keep in step with NoteRestoreLeftOut. */
export const RESTORE_LEFT_OUT = "folders in this backup were left out, because they hold files now and are no longer selected";

/** The note on a member's restore when the project folder of its stack did not
 *  come back, followed by ": " and the reason. Keep in step with
 *  NoteStackFolderFailed. */
export const STACK_FOLDER_FAILED = "the project folder of this stack did not come back, so the container was left stopped";

/** The note on a stack member recreated without its data. Keep in step with
 *  NoteRestoredWithoutData. */
export const RESTORED_WITHOUT_DATA = "restored without its data, because only database dumps of it exist: start it and import a dump";

/** The note on a restored stack member that stays stopped, followed by ": " and
 *  why. Keep in step with NoteStackNotStarted. */
export const STACK_NOT_STARTED = "restored and left stopped";

/** The note of a VM backup that does not hold the state of the VM's TPM. Keep
 *  in step with NoteVMTPMLeftOut. */
export const VM_TPM_LEFT_OUT = "the state of the VM's TPM is not in this backup";

/** The note of a VM backup that does not read some of the VM's disks, followed
 *  by ": " and the disks with their kinds. Keep in step with NoteVMDisksLeftOut. */
export const VM_DISKS_LEFT_OUT = "some disks of the VM are not in this backup";

/** Whether a run's note says the container's data was out of reach. */
export function dataOutsideHost(raw: string | null | undefined): boolean {
  const text = raw?.trim() ?? "";
  return text === DATA_OUTSIDE_HOST || text.startsWith(DATA_OUTSIDE_HOST + ": ");
}

/** Whether a restore failed because this host lacks the GPU or runtime of the
 *  container. */
export function isRuntimeRefusal(raw: string | null | undefined): boolean {
  return !!raw && (raw === RESTORE_NO_RUNTIME || raw.startsWith(RESTORE_NO_RUNTIME + ": "));
}

/** What a successful container restore left out, read from its notes, which
 *  it joins with "; ". folderFailed is the first part of restic's reason. */
export function restoreNotes(raw: string | null | undefined): {
  withoutRuntime: boolean;
  unlinked: string[];
  notBack: string[];
  folderFailed?: string;
  stayedStopped: boolean;
  leftOut?: string;
} {
  const out = {
    withoutRuntime: false,
    unlinked: [] as string[],
    notBack: [] as string[],
    folderFailed: undefined as string | undefined,
    stayedStopped: false,
    leftOut: undefined as string | undefined,
  };
  for (const note of (raw ?? "").split("; ")) {
    if (note === RESTORED_WITHOUT_RUNTIME) out.withoutRuntime = true;
    else if (note.startsWith(RESTORE_LEFT_OUT + ": ")) out.leftOut = note;
    else if (note === RESTORED_WITHOUT_DATA || note === STACK_NOT_STARTED || note.startsWith(STACK_NOT_STARTED + ": ")) out.stayedStopped = true;
    else if (note === STACK_FOLDER_FAILED || note.startsWith(STACK_FOLDER_FAILED + ": ")) {
      out.stayedStopped = true;
      out.folderFailed = note.slice(STACK_FOLDER_FAILED.length + 2);
    }
    else if (note.startsWith(RESTORED_WITHOUT_LINKS + ": ")) out.unlinked = note.slice(RESTORED_WITHOUT_LINKS.length + 2).split(", ");
    else if (note.startsWith(CONTAINERS_NOT_BACK + ": ")) out.notBack = note.slice(CONTAINERS_NOT_BACK.length + 2).split(", ");
  }
  return out;
}

/**
 * The reasons a dump, an import or a restore writes, which may be followed by
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
  [DATA_OUTSIDE_HOST]: "runReason.dataOutsideHost",
  [FILES_UNREADABLE]: "runReason.filesUnreadable",
  [FOLDERS_MISSING]: "runReason.foldersMissing",
  [BACKUP_EMPTY]: "runReason.backupEmpty",
  [OLDER_BACKUPS_KEPT]: "runReason.olderBackupsKept",
  [STACK_FOLDER_EMPTY]: "runReason.stackFolderEmpty",
  "the flash backup held less than it should, so the last ZIP was kept": "runReason.flashZipKept",
  "database imported; the previous data folder was kept": "runReason.dbimportKeptOld",
  "database imported with errors": "runReason.dbimportErrors",
  [RESTORE_NO_RUNTIME]: "runReason.restoreNoRuntime",
  "restore did not finish: the container was created again, stopped, on partly restored data": "runReason.restoreHalfDone",
  "restore did not finish: the VM was left off on partly restored disks": "runReason.vmRestoreHalfDone",
  "restore did not finish: the container was left stopped, because the picked files may be partly written": "runReason.restoreFilesHalfDone",
  "restore failed: some files could not be written": "runReason.restoreFilesNotWritten",
  [RESTORE_LEFT_OUT]: "runReason.restoreLeftOut",
  [STACK_FOLDER_FAILED]: "runReason.stackFolderFailed",
  [RESTORED_WITHOUT_DATA]: "runReason.restoredWithoutData",
  [STACK_NOT_STARTED]: "runReason.stackNotStarted",
  [VM_TPM_LEFT_OUT]: "runReason.vmTpmLeftOut",
  [VM_DISKS_LEFT_OUT]: "runReason.vmDisksLeftOut",
  [RESTORED_WITHOUT_RUNTIME]: "runReason.restoredWithoutRuntime",
  [RESTORED_WITHOUT_LINKS]: "runReason.restoredWithoutLinks",
  [CONTAINERS_NOT_BACK]: "runReason.containersNotBack",
  [NO_NETWORK_UNTIL_RESTART]: "runReason.noNetworkUntilRestart",
  "the pre-backup command says the app is busy": "runReason.deferredHook",
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

/** Splits the tail naming the apps an import stopped off a note or reason. The
 *  names come back separated, because how many there are picks the sentence. */
function importAppTail(text: string): { rest: string; apps: StoppedApps } | undefined {
  for (const [tail, key] of Object.entries(IMPORT_APP_TAILS)) {
    const at = text.lastIndexOf(`; ${tail}: `);
    if (at >= 0) {
      return { rest: text.slice(0, at), apps: { key, names: text.slice(at + tail.length + 4).split(", ") } };
    }
  }
  return undefined;
}

/**
 * What a backup the stall guard cancelled writes: the hours it went without
 * progress and, for a ZFS item, the dataset it was reading. Keep in step with
 * StalledReason in internal/store/runs.go.
 */
const STALLED = "stopped by the stall guard";
const STALLED_REASON = new RegExp(`^${STALLED} after (\\d+) hours? without progress(?: while reading (.+))?$`);

interface Stall {
  hours: number;
  dataset?: string;
}

function stallOf(text: string): Stall | undefined {
  const m = STALLED_REASON.exec(text);
  return m ? { hours: Number(m[1]), dataset: m[2] } : undefined;
}

function stallKey(stall: Stall): TranslationKey {
  return stall.dataset === undefined ? "runReason.stalled" : "runReason.stalledReading";
}

/** What an import writes when its tool reported errors: how many it counted and
 *  where the previous data folder waits. The count picks the sentence. */
const IMPORT_ERRORS = /^database imported with errors: (\d+), the previous data folder is kept at (.+)$/;

/** The sentence about the apps an import stopped, and the names it lists. */
export interface StoppedApps {
  key: TranslationKey;
  names: string[];
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
  DATA_OUTSIDE_HOST,
  FILES_UNREADABLE,
  CONTAINERS_NOT_BACK,
  NO_NETWORK_UNTIL_RESTART,
  FOLDERS_MISSING,
  BACKUP_EMPTY,
  OLDER_BACKUPS_KEPT,
  STACK_FOLDER_EMPTY,
  RESTORE_LEFT_OUT,
  STACK_FOLDER_FAILED,
  RESTORED_WITHOUT_DATA,
  STACK_NOT_STARTED,
  VM_TPM_LEFT_OUT,
  VM_DISKS_LEFT_OUT,
];

/**
 * Splits the notes a backup run joins with "; " (internal/backup/orchestrator.go
 * and withUnreadableNote). A note starts at a dump note or one of the heads
 * below; any other "; " belongs to the note before it.
 */
export function backupNotes(raw: string): string[] {
  const heads = [FILES_UNREADABLE, DATA_OUTSIDE_HOST, CONTAINERS_NOT_BACK, NO_NETWORK_UNTIL_RESTART, FOLDERS_MISSING, BACKUP_EMPTY, OLDER_BACKUPS_KEPT, STACK_FOLDER_EMPTY, VM_TPM_LEFT_OUT, VM_DISKS_LEFT_OUT];
  const notes: string[] = [];
  for (const part of raw.split("; ")) {
    const starts = part.startsWith("database dump ") || heads.some((h) => part === h || part.startsWith(h + ": "));
    if (notes.length > 0 && !starts) notes[notes.length - 1] += "; " + part;
    else notes.push(part);
  }
  return notes;
}

/** Whether a note of a successful run reports something worth acting on. */
export function isWarningNote(raw: string | null | undefined): boolean {
  const text = raw?.trim() ?? "";
  const notes = backupNotes(text);
  if (notes.length > 1) return notes.some(isWarningNote);
  if (importAppTail(text)) return true;
  if (WARNING_NOTES.some((note) => text === note || text.startsWith(note + ": "))) return true;
  const left = restoreNotes(text);
  return left.withoutRuntime || left.unlinked.length > 0;
}

/**
 * A reason split into its translated head, the raw detail behind it and, for a
 * dump that may still be running in the container, the translated note saying so.
 */
export interface RunReasonParts {
  head: string;
  detail: string;
  note?: string;
  /** Set when the note names stopped apps, so a renderer can isolate each name. */
  apps?: StoppedApps;
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
  const stopped = importAppTail(text);
  if (stopped) {
    const { key, names } = stopped.apps;
    return {
      ...runReasonParts(stopped.rest, t),
      note: t(key, names.length).replace("{apps}", names.join(", ")),
      apps: stopped.apps,
    };
  }
  if (!text) return { head: "", detail: "" };

  // A refused run about a key stores its code, and an older one the server's
  // sentence, which the lookups below leave as it is.
  const keyRefusal = keyRunKey(text);
  if (keyRefusal) return { head: t(keyRefusal), detail: "" };

  const stall = stallOf(text);
  if (stall) {
    return { head: t(stallKey(stall), stall.hours).replace("{dataset}", stall.dataset ?? ""), detail: "" };
  }

  const errors = IMPORT_ERRORS.exec(text);
  if (errors) return { head: t("runReason.dbimportErrors", Number(errors[1])), detail: errors[2] };

  const deferral = deferralReason(text, t);
  if (deferral) return { head: deferral, detail: "" };

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
 * The note about stopped apps with every container name in its own isolate.
 * The names come from the host, so an Arabic or Hebrew page must not reorder
 * them or drag their underscores to the far end of the sentence.
 */
function stoppedAppsNote(apps: StoppedApps, t: T): ReactElement {
  const [before, after = ""] = t(apps.key, apps.names.length).split("{apps}");
  const listed = apps.names.flatMap((name, i): ReactNode[] => {
    const isolated = createElement("bdi", { dir: "ltr" }, name);
    return i === 0 ? [isolated] : [", ", isolated];
  });
  return createElement(Fragment, null, before, ...listed, after);
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
  const stall = stallOf(reason?.trim() ?? "");
  if (stall?.dataset !== undefined) {
    const [before, after = ""] = t(stallKey(stall), stall.hours).split("{dataset}");
    return createElement(Fragment, null, before, createElement("bdi", { dir: "ltr" }, stall.dataset), after);
  }
  const { head, detail, note, apps } = runReasonParts(reason, t);
  const tail: ReactNode[] = note ? ["; ", apps ? stoppedAppsNote(apps, t) : note] : [];
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
  return text in RUN_REASONS || text in RUN_REASON_PREFIXES || stallOf(text) !== undefined;
}
