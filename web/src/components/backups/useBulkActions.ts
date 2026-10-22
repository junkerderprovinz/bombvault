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
  failureOf,
  type BackupItem,
  type OkEnvelope,
  type SelectionRefused,
} from "../../lib/api";
import { ENTRY_KIND, type EntryKind } from "../../lib/backupEntry";
import { batchParts, canBackUp, itemProgressKey, runsNow, type BatchPart } from "../../lib/backupList";
import type { TranslationKey } from "../../lib/i18n";
import type { ProgressState } from "../../lib/progress";
import type { ConfirmOptions } from "../../lib/useConfirm";

type T = (key: TranslationKey, n?: number) => string;

// The server runs a batch by itself, so the page may be closed while it does.
// A selection of several kinds goes to the server as a whole for the same
// reason, and reports on SELECTION_KEY.
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

const SELECTION_KEY = "backup-selection";

// A coded refusal says why the entry cannot go at all, except the one for a
// section another task holds, which ends by itself like a running batch.
function refusedAsBusy(err: unknown, res?: OkEnvelope): boolean {
  if (err instanceof ApiError) return err.status === 409 && (!err.code || err.code === "domain-busy" || err.code === "batch-busy");
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
  const selection = progress[SELECTION_KEY];
  const selectionLive = live(SELECTION_KEY);
  useEffect(() => {
    if (!plan) return;
    if (liveAt > plan.seen) setPlan({ ...plan, seen: liveAt });
    else if (liveAt < 0 && plan.seen >= 0 && !busy && !selectionLive) setPlan(null);
  }, [plan, liveAt, busy, selectionLive]);

  // One kind goes to its own batch, several go to the server as a whole.
  async function start(parts: BatchPart[]): Promise<{ res?: OkEnvelope & { refused?: SelectionRefused[] }; thrown?: unknown }> {
    try {
      return { res: parts.length > 1 ? await backupSelection(parts) : await START[parts[0].kind](parts[0].keys) };
    } catch (err) {
      return { thrown: err };
    }
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
    const { res, thrown } = await start(parts);
    if (gone.current) return;
    setBusy(false);
    if (res?.ok) {
      const refused = res.refused ?? [];
      const left = parts.filter((part) => !refused.some((r) => r.kind === part.kind));
      setPlan(left.length > 0 ? { parts: left, seen: -1 } : null);
      if (left.length > 0) {
        const said = t(parts.length > 1 ? "backups.batch.startedInTurn" : "containers.batchStarted");
        push(skipped > 0 ? `${said} ${t("backups.batch.skipped", skipped)}` : said, "success");
      }
      for (const r of refused) push(failureOf(409, r.code, r.error).message, "fail");
      return;
    }
    setPlan(null);
    if (refusedAsBusy(thrown, res)) {
      push(thrown instanceof ApiError && thrown.code ? thrown.message : t("containers.batchAlreadyRunning"), "warn");
      return;
    }
    const message = thrown instanceof Error ? thrown.message : res?.error;
    push(message ?? t("containers.backupStartFailed"), "fail");
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
  if (plan && (liveAt >= 0 || busy || selectionLive)) {
    // Between two kinds, the ones that have reported are done.
    const sizes = plan.parts.map((part) => part.keys.length);
    const of = sizes.reduce((sum, n) => sum + n, 0);
    const before = sizes.slice(0, liveAt < 0 ? plan.seen + 1 : liveAt).reduce((sum, n) => sum + n, 0);
    const share = liveAt < 0 ? 0 : (progress[BATCH_KEY[plan.parts[liveAt].kind]].percent / 100) * sizes[liveAt];
    batch = { label: t("backups.batch.running"), percent: ((before + share) / of) * 100, done: Math.floor(before + share), of };
  } else if (selectionLive) {
    // A selection this page did not start, or one it started before it was
    // opened again, counts by the kinds that are done.
    batch = { label: t("backups.batch.running"), percent: selection.percent, done: selection.done, of: selection.total };
  } else if (liveBatch) {
    batch = { label: t("backups.batch.running"), percent: progress[liveBatch].percent };
  }

  return { busy, batch, backUp, backUpOne, cancelBackup, setIncluded };
}
