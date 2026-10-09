// The replica card, the storage row and the plan line of one ZFS item all
// read the same answer, and a change in one of them has to show in the
// others. So every answer is held once per key for as long as anything on
// screen reads it.
import { useCallback, useEffect, useRef, useSyncExternalStore } from "react";

import { getGroup, getZFSReplica, listZFSReplicaServers } from "../../../lib/api";
import type { GroupState, ZFSReplica, ZFSReplicaServer } from "../../../lib/api";
import { useProgress } from "../../../lib/progress";
import { replicaProgressKey, replicaRestoreKey } from "./replicaModel";

interface Entry<V> {
  value: V | undefined;
  listeners: Set<() => void>;
  loading: boolean;
}

function sharedResource<V>(load: (key: string) => Promise<V>) {
  const entries = new Map<string, Entry<V>>();

  function refresh(key: string) {
    const entry = entries.get(key);
    if (!entry || entry.loading) return;
    entry.loading = true;
    load(key)
      .then((value) => {
        entry.value = value;
        entry.listeners.forEach((notify) => notify());
      })
      .catch(() => undefined)
      .finally(() => {
        entry.loading = false;
      });
  }

  return function useShared(key: string) {
    const subscribe = useCallback(
      (notify: () => void) => {
        let entry = entries.get(key);
        if (!entry) {
          entry = { value: undefined, listeners: new Set(), loading: false };
          entries.set(key, entry);
        }
        entry.listeners.add(notify);
        if (entry.value === undefined) refresh(key);
        return () => {
          entry.listeners.delete(notify);
          if (entry.listeners.size === 0) entries.delete(key);
        };
      },
      [key],
    );
    const value = useSyncExternalStore(subscribe, () => entries.get(key)?.value);
    const reload = useCallback(() => refresh(key), [key]);
    return { value, reload };
  };
}

const useReplicaResource = sharedResource(getZFSReplica);
const useServersResource = sharedResource(() => listZFSReplicaServers());
const useGroupResource = sharedResource<GroupState | null>(() => getGroup().then((g) => (g.ok ? g : null)));

/** useReplica is one item's replica, read again when a run on its progress
 *  key ends. restoreActive is a bring back in flight. */
export function useReplica(itemId: string): {
  replica: ZFSReplica | undefined;
  reload: () => void;
  progressActive: boolean;
  restoreActive: boolean;
} {
  const { value, reload } = useReplicaResource(itemId);
  const progress = useProgress();
  const progressActive = progress[replicaProgressKey(itemId)]?.active ?? false;
  const restoreActive = progress[replicaRestoreKey(itemId)]?.active ?? false;
  const wasActive = useRef(progressActive);
  useEffect(() => {
    if (wasActive.current && !progressActive) reload();
    wasActive.current = progressActive;
  }, [progressActive, reload]);
  return { replica: value, reload, progressActive, restoreActive };
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
