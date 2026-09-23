import { useCallback, useEffect, useRef, useState } from "react";
import { getTimeline, getTimelinePlace, type TimelineDomain, type TimelinePlace, type TimelineRow } from "./api";
import { mergePlaceRows } from "./timeline";

/** useTimeline reads an item's timeline while open: the local places at once,
 *  every other place only through loadPlace. */
export function useTimeline(domain: TimelineDomain, key: string, open: boolean) {
  const [places, setPlaces] = useState<TimelinePlace[]>([]);
  const [rows, setRows] = useState<TimelineRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);
  // Places read on request are read again on reload, never on a fresh open.
  const checked = useRef(new Set<string>());
  const generation = useRef(0);

  const fetchPlace = useCallback(
    async (place: string, gen: number) => {
      const res = await getTimelinePlace(domain, key, place).catch((err: unknown) => ({
        ok: false,
        error: err instanceof Error ? err.message : "",
        place: undefined,
        rows: undefined,
      }));
      if (gen !== generation.current) return;
      const fresh = res.ok ? res.place : undefined;
      setPlaces((ps) =>
        ps.map((p) => (p.place !== place ? p : (fresh ?? { ...p, state: "unreadable" as const, error: res.error ?? "" })))
      );
      setRows((rs) => mergePlaceRows(rs, place, fresh ? (res.rows ?? []) : []));
    },
    [domain, key]
  );

  const loadPlace = useCallback(
    (place: string) => {
      checked.current.add(place);
      return fetchPlace(place, generation.current);
    },
    [fetchPlace]
  );

  useEffect(() => {
    checked.current = new Set();
  }, [open, domain, key]);

  useEffect(() => {
    if (!open) return;
    const gen = ++generation.current;
    setLoading(true);
    setError(null);
    getTimeline(domain, key)
      .then(async (res) => {
        if (gen !== generation.current) return;
        if (!res.ok) {
          setError(res.error ?? "");
          return;
        }
        setPlaces(res.places ?? []);
        setRows(res.rows ?? []);
        setLoading(false);
        for (const place of checked.current) await fetchPlace(place, gen);
      })
      .catch((err: unknown) => {
        if (gen === generation.current) setError(err instanceof Error ? err.message : "");
      })
      .finally(() => {
        if (gen === generation.current) setLoading(false);
      });
  }, [open, domain, key, tick, fetchPlace]);

  const reload = useCallback(() => setTick((n) => n + 1), []);

  return { places, rows, loading, error, loadPlace, reload };
}
