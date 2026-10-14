// What the Backups list does to replicate every ZFS entry that has a target:
// one run after the other, each followed to its end on its own progress key.

import { useEffect, useRef, useState } from "react";
import { getZFSReplica, runZFSReplica, type BackupItem, type ZFSReplica } from "../../lib/api";
import type { TranslationKey } from "../../lib/i18n";
import { anyActive, progressNow, type ProgressState } from "../../lib/progress";
import { zfsCodeSentence } from "../../lib/zfsCodes";
import { canReplicate, replicaProgressKey } from "../zfs/replica/replicaModel";
import type { BatchProgress } from "./useBulkActions";

type T = (key: TranslationKey, n?: number) => string;

const POLL_MS = 500;
// A run that is over before the first look never shows as active, so the wait
// for it ends after this long.
const START_GRACE_MS = 5000;

export interface ReplicateAll {
  /** At least one of the entries has a replica that a button can start. */
  available: boolean;
  /** The runs started here have not all ended. */
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
  const [plan, setPlan] = useState<{ ids: string[]; done: number } | null>(null);
  const gone = useRef(false);
  useEffect(
    () => () => {
      gone.current = true;
    },
    []
  );

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

  const ready = items.filter((item) => replicas[item.key] && canReplicate(replicas[item.key]));

  async function untilEnded(key: string) {
    const began = Date.now();
    let seen = false;
    while (!gone.current) {
      await new Promise((done) => setTimeout(done, POLL_MS));
      const state = progressNow(key);
      if (state) seen = true;
      if (state?.active && !state.finished) continue;
      if (seen || Date.now() - began > START_GRACE_MS) return;
    }
  }

  async function start() {
    if (plan) return;
    const picked = ready.map((item) => ({ id: item.key, name: item.name }));
    if (picked.length === 0) {
      push(t("zfs.replica.allNothing"), "warn");
      return;
    }
    const ids = picked.map((p) => p.id);
    setPlan({ ids, done: 0 });
    try {
      for (const [i, { id, name }] of picked.entries()) {
        try {
          const res = await runZFSReplica(id);
          if (res.ok) await untilEnded(replicaProgressKey(id));
          else push(`${name}: ${res.code ? zfsCodeSentence(t, res.code) : (res.error ?? t("settings.error"))}`, "fail");
        } catch (err) {
          push(`${name}: ${err instanceof Error ? err.message : t("settings.error")}`, "fail");
        }
        if (gone.current) return;
        setPlan({ ids, done: i + 1 });
      }
    } finally {
      if (!gone.current) setPlan(null);
    }
  }

  let batch: BatchProgress | null = null;
  if (plan) {
    const current = plan.ids[plan.done];
    const share = current ? (progress[replicaProgressKey(current)]?.percent ?? 0) / 100 : 0;
    batch = {
      label: t("zfs.replica.batch.running"),
      percent: ((plan.done + share) / plan.ids.length) * 100,
      done: plan.done,
      of: plan.ids.length,
    };
  }

  return { available: ready.length > 0, busy: plan !== null, batch, start };
}
