// The scheduled backups held back until their app is idle. One module-level
// poll serves the activity log and every container card, so a long list of
// cards costs one request per round, not one each.

import { useEffect, useState } from "react";
import { getScheduleWaiting } from "./api";
import type { IdleReason, IdleWait } from "./api";
import type { TranslationKey } from "./i18n";

const POLL_MS = 15000;

let current: IdleWait[] = [];
const listeners = new Set<(waits: IdleWait[]) => void>();
let timer: ReturnType<typeof setInterval> | null = null;

function load(): void {
  getScheduleWaiting()
    .then((waits) => {
      current = waits;
      for (const listener of listeners) listener(current);
    })
    .catch(() => {
      /* keep the last known list */
    });
}

/** The backups waiting for an idle app, refreshed while anything shows them. */
export function useIdleWaits(): IdleWait[] {
  const [waits, setWaits] = useState<IdleWait[]>(current);
  useEffect(() => {
    listeners.add(setWaits);
    if (listeners.size === 1) {
      load();
      timer = setInterval(load, POLL_MS);
    }
    setWaits(current);
    return () => {
      listeners.delete(setWaits);
      if (listeners.size === 0 && timer) {
        clearInterval(timer);
        timer = null;
      }
    };
  }, []);
  return waits;
}

/** What a waiting line says about why the app counts as busy. */
export const IDLE_REASON_KEYS: Record<IdleReason, TranslationKey> = {
  streaming: "idle.reasonStreaming",
  cpu: "idle.reasonCpu",
  network: "idle.reasonNetwork",
  measuring: "idle.reasonMeasuring",
};
