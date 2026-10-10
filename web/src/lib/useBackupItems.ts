import { useCallback, useEffect, useRef, useState } from "react";

import { listItems, type BackupItem } from "./api";
import { listedItems } from "./backupList";
import { anyActive, useProgress } from "./progress";

export interface BackupItems {
  items: BackupItem[];
  loaded: boolean;
  /** The last read failed. A list read before stays in place. */
  failed: boolean;
  reload: () => void;
}

/**
 * useBackupItems reads GET /api/items and keeps it current: it reads again
 * when a run ends and when a setting changes what is backed up. It never
 * polls, because listing the VMs costs the server a round trip to libvirt.
 */
export function useBackupItems(): BackupItems {
  const [state, setState] = useState({ items: [] as BackupItem[], loaded: false, failed: false });

  // Only the newest read may land, or a slow answer would bring back a list
  // that a later one has already replaced.
  const read = useRef(0);
  const reload = useCallback(() => {
    const n = ++read.current;
    listItems()
      .then((res) => {
        if (n !== read.current) return;
        if (res.ok) setState({ items: listedItems(res.items), loaded: true, failed: false });
        else setState((s) => ({ ...s, failed: true }));
      })
      .catch(() => {
        if (n === read.current) setState((s) => ({ ...s, failed: true }));
      });
  }, []);

  useEffect(() => {
    reload();
    window.addEventListener("bv:settings-changed", reload);
    return () => window.removeEventListener("bv:settings-changed", reload);
  }, [reload]);

  const running = anyActive(useProgress()).active;
  const wasRunning = useRef(running);
  useEffect(() => {
    if (wasRunning.current && !running) reload();
    wasRunning.current = running;
  }, [running, reload]);

  return { ...state, reload };
}
