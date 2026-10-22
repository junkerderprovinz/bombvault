// What the Backups list does to replicate every ZFS entry that has a target.
// The server runs them one after the other, so the batch goes on when the
// page is left, and its bar comes back from the batch's progress key.

import { useEffect, useState } from "react";
import { getZFSReplica, runZFSReplicaAll, type BackupItem, type ZFSReplica } from "../../lib/api";
import type { TranslationKey } from "../../lib/i18n";
import { anyActive, type ProgressState } from "../../lib/progress";
import { zfsCodeSentence } from "../../lib/zfsCodes";
import { canReplicate, replicaProgressKey } from "../zfs/replica/replicaModel";
import type { BatchProgress } from "./useBulkActions";

type T = (key: TranslationKey, n?: number) => string;

const REPLICATE_ALL_KEY = "zfs-replica-all";

// The batch is started before its first frame arrives, so the bar stands in
// for it until then, but no longer than this.
const START_GRACE_MS = 5000;

export interface ReplicateAll {
  /** At least one of the entries has a replica that a button can start. */
  available: boolean;
  /** A batch is starting or running on the server. */
  busy: boolean;
  batch: BatchProgress | null;
  start: () => Promise<void>;
}

export function useReplicateAll({
  items,
  progress,
  push,
  t,
}: {
  items: BackupItem[];
  progress: Record<string, ProgressState>;
  push: (message: string, severity?: "success" | "warn" | "fail") => void;
  t: T;
}): ReplicateAll {
  const [replicas, setReplicas] = useState<Record<string, ZFSReplica>>({});
  const [starting, setStarting] = useState<number | null>(null);

  // Read again whenever a run starts or ends, which is when a replica's
  // state changes without the page being told.
  const working = anyActive(progress).active;
  const idsKey = items.map((item) => item.key).join("\n");
  useEffect(() => {
    let alive = true;
    const ids = idsKey === "" ? [] : idsKey.split("\n");
    void Promise.all(ids.map((id) => getZFSReplica(id).then((replica) => [id, replica] as const, () => null))).then(
      (read) => {
        if (!alive) return;
        setReplicas(Object.fromEntries(read.filter((entry) => entry !== null)));
      }
    );
    return () => {
      alive = false;
    };
  }, [idsKey, working]);

  const state = progress[REPLICATE_ALL_KEY];
  const running = !!state?.active && !state.finished;
  useEffect(() => {
    if (starting === null) return;
    if (running) {
      setStarting(null);
      return;
    }
    const timer = setTimeout(() => setStarting(null), START_GRACE_MS);
    return () => clearTimeout(timer);
  }, [starting, running]);

  const ready = items.filter((item) => replicas[item.key] && canReplicate(replicas[item.key]));

  async function start() {
    if (starting !== null || running) return;
    if (ready.length === 0) {
      push(t("zfs.replica.allNothing"), "warn");
      return;
    }
    setStarting(ready.length);
    const nameOf = (id: string) => ready.find((item) => item.key === id)?.name ?? id;
    try {
      const res = await runZFSReplicaAll(ready.map((item) => item.key));
      if (!res.ok) {
        setStarting(null);
        push(res.busy ? t("zfs.replica.allBusy") : (res.error ?? t("settings.error")), res.busy ? "warn" : "fail");
        return;
      }
      if (!res.started) setStarting(null);
      for (const refused of res.refused ?? []) {
        push(`${nameOf(refused.id)}: ${refused.code ? zfsCodeSentence(t, refused.code) : refused.error}`, "fail");
      }
    } catch (err) {
      setStarting(null);
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    }
  }

  let batch: BatchProgress | null = null;
  if (running) {
    const done = state.done ?? 0;
    const of = state.total ?? 0;
    // The entry being sent adds its own share, so the bar moves during a long
    // first send.
    const share = Math.max(
      0,
      ...items.map((item) => {
        const run = progress[replicaProgressKey(item.key)];
        return run?.active && !run.finished ? run.percent / 100 : 0;
      })
    );
    batch = {
      label: t("zfs.replica.batch.running"),
      percent: of > 0 ? (Math.min(done + share, of) / of) * 100 : state.percent,
      done,
      of,
    };
  } else if (starting !== null) {
    batch = { label: t("zfs.replica.batch.running"), percent: 0, done: 0, of: starting };
  }

  return { available: ready.length > 0, busy: starting !== null || running, batch, start };
}
