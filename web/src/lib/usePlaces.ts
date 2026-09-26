import { useEffect, useState } from "react";
import { listPlaces, subscribePlaces, type Place } from "./places";

/** usePlaces is the list of storage places, read again after every place
 *  write. A read that fails keeps the last list, and one that lands after a
 *  newer read is dropped. */
export function usePlaces(): Place[] {
  const [places, setPlaces] = useState<Place[]>([]);
  useEffect(() => {
    let live = true;
    let latest = 0;
    const load = () => {
      const mine = ++latest;
      listPlaces()
        .then((r) => {
          if (live && mine === latest && r.ok) setPlaces(r.places ?? []);
        })
        .catch(() => undefined);
    };
    load();
    const stop = subscribePlaces(load);
    return () => {
      live = false;
      stop();
    };
  }, []);
  return places;
}
