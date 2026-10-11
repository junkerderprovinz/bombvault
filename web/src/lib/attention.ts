// What needs the person, as the overview that lists it and the navigation
// entry that counts it read it. The provider stands apart in
// attentionProvider.ts: it imports everything the list is worked out from,
// which a reader of the count has no use for.

import { createContext, useContext, useEffect } from "react";
import type { Run } from "./api";
import type { Need } from "./needs";
import type { BackupItems } from "./useBackupItems";
import type { StorageLocationList } from "./useStorageLocations";

/** Dispatched after a failure was dealt with, so the list follows without
 *  waiting for the next poll. */
export const ATTENTION_CHANGED_EVENT = "bv:attention-changed";

export interface Attention {
  /** What waits for the person. The overview lists it, the navigation counts it. */
  needs: Need[];
  /** The entries are read and none of them has a backup yet. */
  fresh: boolean;
  items: BackupItems;
  /** The runs of GET /api/runs, newest first. */
  runs: Run[];
  /** The first read of the runs has come back. */
  runsReady: boolean;
  /** The last read of the runs failed. The runs read before stay in place. */
  runsFailed: boolean;
  refreshRuns: () => void;
  /** The storage locations, read once for the count and for the page. */
  locations: StorageLocationList;
}

const idle = () => undefined;

export const AttentionContext = createContext<Attention & { watch: () => () => void }>({
  needs: [],
  fresh: false,
  items: { items: [], reported: [], loaded: false, failed: false, reload: idle },
  runs: [],
  runsReady: false,
  runsFailed: false,
  refreshRuns: idle,
  locations: { locations: [], loaded: false, failed: false, retry: idle },
  watch: () => idle,
});

// The count has a context of its own, so the navigation is not drawn again
// with every poll that leaves it as it was.
export const AttentionCountContext = createContext(0);

export const FreshInstallContext = createContext(false);

/**
 * useAttention is for the page that shows the lists themselves. While it is
 * mounted the runs are read at the activity log's rate, and a visit brings
 * the entries, the runs and the storage locations up to date, because entries
 * change on other pages without an event.
 */
export function useAttention(): Attention {
  const { watch, ...attention } = useContext(AttentionContext);
  useEffect(() => watch(), [watch]);
  return attention;
}

/** useAttentionCount is how many things need the person. */
export function useAttentionCount(): number {
  return useContext(AttentionCountContext);
}

/** useFreshInstall is true from the moment the entries are read until the
 *  first of them has a backup. */
export function useFreshInstall(): boolean {
  return useContext(FreshInstallContext);
}
