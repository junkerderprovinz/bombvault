import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { listItems, type BackupItem } from "./api";
import { listedItems } from "./backupList";

export interface BackupItems {
  items: BackupItem[];
  loaded: boolean;
  /** The last read failed. A list read before stays in place. */
  failed: boolean;
  reload: () => void;
}

/**
 * useBackupItems reads GET /api/items, and reads it again when a setting
 * changes what is backed up. It never polls, because listing the VMs costs the
 * server a round trip to libvirt.
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

  return useMemo(() => ({ ...state, reload }), [state, reload]);
}
