// What the Backups list does with several entries at once: back them up, take
// them out of the schedule or back into it, and restore their newest backup.

import { useEffect, useRef, useState } from "react";
import {
  ApiError,
  backupAll,
  backupConfigNow,
  backupFilesAll,
  backupFlashNow,
  backupVMsAll,
  backupZFSAll,
  cancelBackup as cancelRunning,
  restore,
  restoreVM,
  type BackupItem,
  type OkEnvelope,
} from "../../lib/api";
import { ENTRY_KIND, type EntryKind } from "../../lib/backupEntry";
import {
  batchParts,
  canBackUp,
  itemProgressKey,
  restoresInBulk,
  runsNow,
  type BatchPart,
} from "../../lib/backupList";
import { fireAndWaitRun } from "../../lib/backupWatch";
import type { TranslationKey } from "../../lib/i18n";
import type { ProgressState } from "../../lib/progress";
import { isRuntimeRefusal } from "../../lib/runReason";
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

// One backup runs at a time on the server. The batch of the next kind is
// offered again at this pace until the one before it has let go.
const RETRY_MS = 2000;

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
  /** The containers the last restore could not recreate for a GPU or runtime
   *  this host lacks. */
  runtimeRefused: string[];
  backUp: (items: BackupItem[]) => Promise<void>;
  /** Starts the backup of one entry, which reports on its own row. */
  backUpOne: (item: BackupItem) => Promise<void>;
  cancelBackup: (item: BackupItem, name: string) => Promise<void>;
  setIncluded: (items: BackupItem[], include: boolean, nameOf: (item: BackupItem) => string) => Promise<void>;
  restoreNewest: (items: BackupItem[]) => Promise<void>;
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
  const [restoring, setRestoring] = useState<{ done: number; of: number } | null>(null);
  const [runtimeRefused, setRuntimeRefused] = useState<string[]>([]);
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
  useEffect(() => {
    if (!plan) return;
    if (liveAt > plan.seen) setPlan({ ...plan, seen: liveAt });
    else if (liveAt < 0 && plan.seen >= 0 && !busy) setPlan(null);
  }, [plan, liveAt, busy]);

  // Starts one kind's batch. `wait` keeps offering it while another backup
  // holds the server, which is what the batch of the kind before it does.
  async function startPart(part: BatchPart, wait: boolean): Promise<"ok" | "busy" | "gone" | { error: string }> {
    for (;;) {
      let res: OkEnvelope | undefined;
      let thrown: unknown;
      try {
        res = await START[part.kind](part.keys);
      } catch (err) {
        thrown = err;
      }
      if (res?.ok) return "ok";
      if (!refusedAsBusy(thrown, res)) {
        const message = thrown instanceof Error ? thrown.message : res?.error;
        return { error: message ?? t("containers.backupStartFailed") };
      }
      if (!wait) return "busy";
      await new Promise((done) => setTimeout(done, RETRY_MS));
      if (gone.current) return "gone";
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
    try {
      for (const [i, part] of parts.entries()) {
        const started = await startPart(part, i > 0);
        if (started === "gone") return;
        if (started === "busy") {
          push(t("containers.batchAlreadyRunning"), "warn");
          setPlan(null);
          return;
        }
        if (started !== "ok") {
          push(started.error, "fail");
          if (i === 0) setPlan(null);
          return;
        }
        if (i === 0) {
          const said = t(parts.length > 1 ? "backups.batch.startedInTurn" : "containers.batchStarted");
          push(skipped > 0 ? `${said} ${t("backups.batch.skipped", skipped)}` : said, "success");
        }
      }
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

  // Restores share the server's one slot, so each waits for the run of the
  // one before it.
  async function restoreNewest(items: BackupItem[]) {
    const fit = items.filter(restoresInBulk);
    if (fit.length === 0) return;
    const containers = fit.some((item) => item.kind === "container");
    const vms = fit.some((item) => item.kind === "vm");
    const question = !containers
      ? t("vms.restoreSelectedConfirm")
      : vms
        ? `${t("backups.restore.confirm")} ${t("backups.restore.alsoVMs")}`
        : t("backups.restore.confirm");
    if (!(await confirm(question, { confirmKey: "snapshots.restore" }))) return;

    setBusy(true);
    setRuntimeRefused([]);
    setRestoring({ done: 0, of: fit.length });
    let ok = 0;
    const refused: string[] = [];
    for (const [i, item] of fit.entries()) {
      const res = await fireAndWaitRun({
        kind: "restore",
        matchRun: (run) => run.domain === item.kind && run.target === item.key,
        start: () => (item.kind === "vm" ? restoreVM(item.key, "latest", true) : restore(item.key, "latest", true)),
        t,
      });
      if (gone.current) return;
      if (res.ok) ok++;
      else if (isRuntimeRefusal(res.error)) refused.push(item.key);
      setRestoring({ done: i + 1, of: fit.length });
    }
    setBusy(false);
    setRestoring(null);
    setRuntimeRefused(refused);
    const failed = fit.length - ok;
    push(
      t("containers.bulkResult").replace("{ok}", String(ok)).replace("{fail}", String(failed)),
      failed > 0 ? "warn" : "success"
    );
    await reload();
  }

  let batch: BatchProgress | null = null;
  if (restoring) {
    batch = { label: t("backups.restore.running"), done: restoring.done, of: restoring.of };
  } else if (plan && (liveAt >= 0 || busy)) {
    // Between two kinds, the ones that have reported are done.
    const sizes = plan.parts.map((part) => part.keys.length);
    const of = sizes.reduce((sum, n) => sum + n, 0);
    const before = sizes.slice(0, liveAt < 0 ? plan.seen + 1 : liveAt).reduce((sum, n) => sum + n, 0);
    const share = liveAt < 0 ? 0 : (progress[BATCH_KEY[plan.parts[liveAt].kind]].percent / 100) * sizes[liveAt];
    batch = { label: t("backups.batch.running"), percent: ((before + share) / of) * 100, done: Math.floor(before + share), of };
  } else if (liveBatch) {
    batch = { label: t("backups.batch.running"), percent: progress[liveBatch].percent };
  }

  return { busy, batch, runtimeRefused, backUp, backUpOne, cancelBackup, setIncluded, restoreNewest };
}
