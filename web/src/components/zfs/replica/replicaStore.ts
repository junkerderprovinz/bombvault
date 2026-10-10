// The replica card, the storage row and the plan line of one ZFS item all
// read the same answer, and a change in one of them has to show in the
// others. So every answer is held once per key for as long as anything on
// screen reads it.
import { useCallback, useEffect, useRef, useSyncExternalStore } from "react";

import { getGroup, getZFSReplica, listZFSReceiveRequests, listZFSReplicaServers } from "../../../lib/api";
import type { GroupState, ZFSReceiveRequest, ZFSReplica, ZFSReplicaServer } from "../../../lib/api";
import { useProgress, type ProgressState } from "../../../lib/progress";
import { replicaProgressKey, replicaRestoreKey } from "./replicaModel";

interface Entry<V> {
  value: V | undefined;
  listeners: Set<() => void>;
  loading: boolean;
  /** Something asked for a fresh read after the one in flight left, so its
   *  answer may predate a change. */
  stale: boolean;
  timer?: ReturnType<typeof setTimeout>;
}

function sharedResource<V>(load: (key: string) => Promise<V>) {
  const entries = new Map<string, Entry<V>>();

  function refresh(key: string) {
    const entry = entries.get(key);
    if (!entry) return;
    entry.stale = true;
    if (entry.loading) return;
    entry.loading = true;
    // Every part of the page that saw the same run end asks in the same
    // commit, and one read answers them all.
    queueMicrotask(() => {
      entry.stale = false;
      load(key)
        .then((value) => {
          if (entry.stale && entry.value !== undefined) return;
          entry.value = value;
          entry.listeners.forEach((notify) => notify());
        })
        .catch(() => undefined)
        .finally(() => {
          entry.loading = false;
          if (entry.stale && entries.get(key) === entry) refresh(key);
        });
    });
  }

  function refreshLater(key: string, delayMs: number) {
    const entry = entries.get(key);
    if (!entry || entry.timer) return;
    entry.timer = setTimeout(() => {
      entry.timer = undefined;
      refresh(key);
    }, delayMs);
  }

  return function useShared(key: string) {
    const subscribe = useCallback(
      (notify: () => void) => {
        let entry = entries.get(key);
        if (!entry) {
          entry = { value: undefined, listeners: new Set(), loading: false, stale: false };
          entries.set(key, entry);
        }
        entry.listeners.add(notify);
        if (entry.value === undefined) refresh(key);
        return () => {
          entry.listeners.delete(notify);
          if (entry.listeners.size === 0) {
            clearTimeout(entry.timer);
            entries.delete(key);
          }
        };
      },
      [key],
    );
    const value = useSyncExternalStore(subscribe, () => entries.get(key)?.value);
    const reload = useCallback(() => refresh(key), [key]);
    const reloadLater = useCallback((delayMs: number) => refreshLater(key, delayMs), [key]);
    return { value, reload, reloadLater };
  };
}

const useReplicaResource = sharedResource(getZFSReplica);
const useServersResource = sharedResource(() => listZFSReplicaServers());
const useGroupResource = sharedResource<GroupState | null>(() => getGroup().then((g) => (g.ok ? g : null)));
const useReceiveResource = sharedResource(() => listZFSReceiveRequests());

// The server records a run and sends its notifications after the progress
// has ended, and holds the item until then.
const RUNNING_RECHECK_MS = 2000;

/** useReplica is one item's replica, read again when a run or a bring back
 *  ends. restore is the bring back's progress while it has any. */
export function useReplica(itemId: string): {
  replica: ZFSReplica | undefined;
  reload: () => void;
  progressActive: boolean;
  restoreActive: boolean;
  restore: ProgressState | undefined;
} {
  const { value, reload, reloadLater } = useReplicaResource(itemId);
  const progress = useProgress();
  const progressActive = progress[replicaProgressKey(itemId)]?.active ?? false;
  const restore = progress[replicaRestoreKey(itemId)];
  const restoreActive = restore?.active ?? false;
  const busy = progressActive || restoreActive;
  const wasBusy = useRef(busy);
  useEffect(() => {
    if (wasBusy.current && !busy) reload();
    wasBusy.current = busy;
  }, [busy, reload]);
  useEffect(() => {
    if (value?.state === "running" && !busy) reloadLater(RUNNING_RECHECK_MS);
  }, [value, busy, reloadLater]);
  return { replica: value, reload, progressActive, restoreActive, restore };
}

export function useReplicaServers(): { servers: ZFSReplicaServer[]; loaded: boolean; reload: () => void } {
  const { value, reload } = useServersResource("all");
  return { servers: value ?? [], loaded: value !== undefined, reload };
}

/** The pairing group: the instances a replica can be sent to, and this
 *  instance's own name, which names its folder on every target. */
export function useGroup(): GroupState | null | undefined {
  return useGroupResource("group").value;
}

/** The replicas paired instances ask to send here, and the ones allowed. The
 *  grid counts them per instance and an instance's window answers them, so
 *  both read the one list. */
export function useReceiveRequests(): { requests: ZFSReceiveRequest[]; reload: () => void } {
  const { value, reload } = useReceiveResource("all");
  return { requests: value ?? [], reload };
}
