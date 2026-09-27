// useItemChecks loads the newest check results of every item and loads them
// again whenever a check starts or ends anywhere, so a card updates when the
// probe after a backup finishes on the server.

import { useCallback, useEffect, useMemo, useState } from "react";

import { getItemChecks, type ItemChecks } from "./api";
import { useProgress } from "./progress";

/** The progress keys a check runs under; see internal/api item_probe.go. */
export function isCheckKey(key: string): boolean {
  return key.startsWith("probe:");
}

export interface ItemChecksState {
  items: ItemChecks[];
  /** The checks of one item, by the name or id the anomaly list uses. */
  find: (domain: string, nameOrId: string) => ItemChecks | undefined;
  reload: () => void;
}

export function useItemChecks(): ItemChecksState {
  const [items, setItems] = useState<ItemChecks[]>([]);
  const progress = useProgress();
  // Changes when a check starts or ends, which is when there is something new
  // to read.
  const running = Object.keys(progress)
    .filter((k) => isCheckKey(k) && !progress[k].finished)
    .sort()
    .join("|");

  const reload = useCallback(() => {
    getItemChecks()
      .then((r) => {
        if (r.ok) setItems(r.items ?? []);
      })
      .catch(() => undefined);
  }, []);

  useEffect(() => {
    reload();
  }, [reload, running]);

  useEffect(() => {
    window.addEventListener("bv:settings-changed", reload);
    return () => window.removeEventListener("bv:settings-changed", reload);
  }, [reload]);

  return useMemo(() => {
    const byTarget = new Map(items.map((i) => [i.targetId, i]));
    // The domain is checked on the id path too: a container may be called
    // "flash", which is also the flash drive's target id.
    const find = (domain: string, nameOrId: string) => {
      const byId = byTarget.get(nameOrId);
      if (byId?.domain === domain) return byId;
      return items.find((i) => i.domain === domain && i.name === nameOrId);
    };
    return { items, find, reload };
  }, [items, reload]);
}
