import { useCallback, useEffect, useRef, useState } from "react";
import {
  ApiError,
  getStorageLocation,
  listDestinations,
  listOffsiteTargets,
  listStorageLocations,
  type StorageLocation,
} from "./api";
import { anyActive, useProgress } from "./progress";
import type { LocationRecords } from "./storageWrites";

const STORAGE_LOCATIONS_CHANGED = "bv:storage-locations-changed";

/** Tells every mounted reader that a storage location was written. */
export function storageLocationsChanged(): void {
  window.dispatchEvent(new Event(STORAGE_LOCATIONS_CHANGED));
}

/**
 * useReloadCount counts the reasons to read again: a write announced with
 * storageLocationsChanged, a retry, and the end of a backup or a copy, which
 * moves the fill level and the last copy.
 */
function useReloadCount(): [number, () => void] {
  const [count, setCount] = useState(0);
  const bump = useCallback(() => setCount((n) => n + 1), []);
  const running = anyActive(useProgress()).active;
  const wasRunning = useRef(running);

  useEffect(() => {
    if (wasRunning.current && !running) bump();
    wasRunning.current = running;
  }, [running, bump]);

  useEffect(() => {
    window.addEventListener(STORAGE_LOCATIONS_CHANGED, bump);
    return () => window.removeEventListener(STORAGE_LOCATIONS_CHANGED, bump);
  }, [bump]);

  return [count, bump];
}

export interface StorageLocationList {
  locations: StorageLocation[];
  loaded: boolean;
  /** The last read failed. A list read before stays on screen. */
  failed: boolean;
  retry: () => void;
}

export function useStorageLocations(): StorageLocationList {
  const [reloads, retry] = useReloadCount();
  const [state, setState] = useState({ locations: [] as StorageLocation[], loaded: false, failed: false });

  useEffect(() => {
    let alive = true;
    listStorageLocations()
      .then((res) => {
        if (!alive) return;
        if (res.ok) setState({ locations: res.locations ?? [], loaded: true, failed: false });
        else setState((s) => ({ ...s, failed: true }));
      })
      .catch(() => {
        if (alive) setState((s) => ({ ...s, failed: true }));
      });
    return () => {
      alive = false;
    };
  }, [reloads]);

  return { ...state, retry };
}

export type LocationStatus = "loading" | "ready" | "missing" | "failed";

export interface StorageLocationRead {
  location: StorageLocation | null;
  records: LocationRecords;
  status: LocationStatus;
  retry: () => void;
}

async function readRecords(location: StorageLocation): Promise<LocationRecords> {
  if (location.object !== "destination" && location.object !== "target") return { targets: [] };
  const [targets, destinations] = await Promise.all([
    listOffsiteTargets(),
    location.object === "destination" ? listDestinations() : null,
  ]);
  return {
    targets: targets.targets ?? [],
    destination: destinations?.destinations?.find((d) => `destination:${d.id}` === location.id),
  };
}

/**
 * useStorageLocation reads one location with the records its routes are
 * written through. A remote is asked for its room once per visit, after the
 * page is up, because that probe can take as long as the remote does.
 */
export function useStorageLocation(id: string): StorageLocationRead {
  const [reloads, retry] = useReloadCount();
  const [state, setState] = useState<Omit<StorageLocationRead, "retry">>({
    location: null,
    records: { targets: [] },
    status: "loading",
  });
  const probed = useRef("");

  useEffect(() => {
    let alive = true;
    (async () => {
      const res = await getStorageLocation(id);
      if (!res.ok || !res.location) throw new Error(res.error);
      const records = await readRecords(res.location);
      if (!alive) return;
      setState({ location: res.location, records, status: "ready" });

      const remote = res.location.backend !== "local" && !res.location.capacity.unsupported;
      if (!remote || probed.current === id) return;
      const measured = await getStorageLocation(id, true);
      if (alive && measured.ok && measured.location) {
        probed.current = id;
        setState({ location: measured.location, records, status: "ready" });
      }
    })().catch((err: unknown) => {
      if (!alive) return;
      const missing = err instanceof ApiError && err.status === 404;
      setState((s) => (missing ? { ...s, location: null, status: "missing" } : { ...s, status: s.location ? "ready" : "failed" }));
    });
    return () => {
      alive = false;
    };
  }, [id, reloads]);

  return { ...state, retry };
}
