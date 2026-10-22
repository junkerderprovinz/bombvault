// What the Backups list does with several entries at once: back them up, and
// take them out of the schedule or back into it.

import { useEffect, useRef, useState } from "react";
import {
  ApiError,
  backupAll,
  backupConfigNow,
  backupFilesAll,
  backupFlashNow,
  backupSelection,
  backupVMsAll,
  backupZFSAll,
  cancelBackup as cancelRunning,
  explainedError,
  type BackupItem,
  type OkEnvelope,
} from "../../lib/api";
import { ENTRY_KIND, type EntryKind } from "../../lib/backupEntry";
import { batchParts, canBackUp, itemProgressKey, runsNow, type BatchPart } from "../../lib/backupList";
import type { TranslationKey } from "../../lib/i18n";
import type { ProgressState } from "../../lib/progress";
import type { ConfirmOptions } from "../../lib/useConfirm";

type T = (key: TranslationKey, n?: number) => string;

// The server runs a batch by itself, so the page may be closed while it does.
const START: Record<EntryKind, (keys: string[]) => Promise<OkEnvelope>> = {
  container: backupAll,
  vm: backupVMsAll,
  files: backupFilesAll,
  zfs: backupZFSAll,
  flash: () => backupFlashNow(),
  config: () => backupConfigNow(),
};

// The key a kind's batch reports its progress under. Flash and the
// configuration are one entry each and report as that entry.
const BATCH_KEY: Record<EntryKind, string> = {
  container: "batch:containers",
  vm: "batch:vms",
  files: "batch:files",
  zfs: "batch:zfs",
  flash: "flash",
  config: "config",
};

// The key a selection of several kinds reports under while kinds still wait.
const SELECTION_KEY = "backup-selection";

function refusedAsBusy(err: unknown, res?: OkEnvelope): boolean {
  if (err instanceof ApiError) return err.status === 409;
  return !!res && !res.ok && (res.error ?? "").toLowerCase().includes("already running");
}

/** What the note above the list says about a batch under way. */
export interface BatchProgress {
  label: string;
  percent?: number;
  done?: number;
  of?: number;
}

export interface BulkActions {
  /** An action started here has not returned yet. */
  busy: boolean;
  batch: BatchProgress | null;
  backUp: (items: BackupItem[]) => Promise<void>;
  /** Starts the backup of one entry, which reports on its own row. */
  backUpOne: (item: BackupItem) => Promise<void>;
  cancelBackup: (item: BackupItem, name: string) => Promise<void>;
  setIncluded: (items: BackupItem[], include: boolean, nameOf: (item: BackupItem) => string) => Promise<void>;
}

export function useBulkActions({
  progress,
  reload,
  confirm,
  push,
  t,
}: {
  progress: Record<string, ProgressState>;
  reload: () => Promise<void>;
  confirm: (message: string, options?: ConfirmOptions) => Promise<boolean>;
  push: (message: string, severity?: "success" | "warn" | "fail") => void;
  t: T;
}): BulkActions {
  const [busy, setBusy] = useState(false);
  // The batches this page started, so the note can count entries. `seen` is
  // the last of them that has reported, -1 before the first does. The plan
  // ends when one has reported and none does any more.
  const [plan, setPlan] = useState<{ parts: BatchPart[]; seen: number } | null>(null);
  const gone = useRef(false);
  useEffect(
    () => () => {
      gone.current = true;
    },
    []
  );

  // An entry that has ended lingers for a moment so its bar can reach the end.
  const live = (key: string) => !!progress[key]?.active && !progress[key].finished;
  const liveBatch = Object.values(BATCH_KEY).find((key) => key.startsWith("batch:") && live(key));
  const liveAt = plan ? plan.parts.findIndex((part) => live(BATCH_KEY[part.kind])) : -1;
  // Between two kinds of a selection, the selection itself is still live.
  const waiting = busy || live(SELECTION_KEY);
  useEffect(() => {
    if (!plan) return;
    if (liveAt > plan.seen) setPlan({ ...plan, seen: liveAt });
    else if (liveAt < 0 && plan.seen >= 0 && !waiting) setPlan(null);
  }, [plan, liveAt, waiting]);

  // Several kinds go to the server as one selection, which starts each kind
  // once the one before it has let go, so leaving the page stops none of them.
  async function start(parts: BatchPart[]): Promise<"ok" | { busy: string } | { error: string }> {
    let res: OkEnvelope | undefined;
    let thrown: unknown;
    try {
      res = await (parts.length === 1 ? START[parts[0].kind](parts[0].keys) : backupSelection(parts));
    } catch (err) {
      thrown = err;
    }
    if (res?.ok) return "ok";
    if (refusedAsBusy(thrown, res)) return { busy: explainedError(thrown) ?? t("containers.batchAlreadyRunning") };
    const message = thrown instanceof Error ? thrown.message : res?.error;
    return { error: message ?? t("containers.backupStartFailed") };
  }

  async function backUp(items: BackupItem[]) {
    const fit = items.filter((item) => canBackUp(item) && !runsNow(progress[itemProgressKey(item)]));
    if (fit.length === 0) {
      push(t("backups.batch.nothing"), "warn");
      return;
    }
    const parts = batchParts(fit);
    const skipped = items.length - fit.length;
    setBusy(true);
    setPlan({ parts, seen: -1 });
    try {
      const started = await start(parts);
      if (started === "ok") {
        const said = t("containers.batchStarted");
        push(skipped > 0 ? `${said} ${t("backups.batch.skipped", skipped)}` : said, "success");
        return;
      }
      setPlan(null);
      if ("busy" in started) push(started.busy, "warn");
      else push(started.error, "fail");
    } finally {
      if (!gone.current) setBusy(false);
    }
  }

  async function backUpOne(item: BackupItem) {
    let res: OkEnvelope | undefined;
    let thrown: unknown;
    try {
      res = await ENTRY_KIND[item.kind].backup(item.key);
    } catch (err) {
      thrown = err;
    }
    if (res?.ok) return;
    const message = thrown instanceof Error ? thrown.message : res?.error;
    push(message ?? t("containers.backupStartFailed"), refusedAsBusy(thrown, res) ? "warn" : "fail");
  }

  async function cancelBackup(item: BackupItem, name: string) {
    const named = (key: TranslationKey) => t(key).replace(/\{name\}/g, name);
    if (!(await confirm(named("backup.cancelConfirm")))) return;
    try {
      const res = await cancelRunning(itemProgressKey(item));
      if (!res.cancelled) push(named(res.reason === "committed" ? "backup.cancelTooLate" : "backup.cancelNotRunning"), "warn");
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.networkError"), "fail");
    }
  }

  async function setIncluded(items: BackupItem[], include: boolean, nameOf: (item: BackupItem) => string) {
    // Flash and the configuration have no switch of their own.
    const saves = items.flatMap((item) => {
      const save = ENTRY_KIND[item.kind].setIncluded;
      return save ? [{ item, saved: save(item.key, include).then((res) => res.ok, () => false) }] : [];
    });
    if (saves.length === 0) return;
    setBusy(true);
    const saved = await Promise.all(saves.map((s) => s.saved));
    setBusy(false);
    if (saved.includes(false)) {
      push(t("schedule.updateFailed"), "fail");
    } else if (saves.length === 1) {
      const said = t(include ? "backups.schedule.resumedOne" : "backups.schedule.pausedOne");
      push(said.replace("{name}", nameOf(saves[0].item)), "success");
    } else {
      push(t(include ? "backups.schedule.resumedMany" : "backups.schedule.pausedMany", saves.length), "success");
    }
    await reload();
  }

  let batch: BatchProgress | null = null;
  if (plan && (liveAt >= 0 || waiting)) {
    // Between two kinds, the ones that have reported are done.
    const sizes = plan.parts.map((part) => part.keys.length);
    const of = sizes.reduce((sum, n) => sum + n, 0);
    const before = sizes.slice(0, liveAt < 0 ? plan.seen + 1 : liveAt).reduce((sum, n) => sum + n, 0);
    const share = liveAt < 0 ? 0 : (progress[BATCH_KEY[plan.parts[liveAt].kind]].percent / 100) * sizes[liveAt];
    batch = { label: t("backups.batch.running"), percent: ((before + share) / of) * 100, done: Math.floor(before + share), of };
  } else if (liveBatch) {
    batch = { label: t("backups.batch.running"), percent: progress[liveBatch].percent };
  }

  return { busy, batch, backUp, backUpOne, cancelBackup, setIncluded };
}
