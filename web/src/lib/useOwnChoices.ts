import { useEffect, useState } from "react";
import type { DefaultRow, PlacementDomain } from "./api";
import { hasOwnChoice, readPlacedEntries, type PlacedEntry } from "./ownChoice";
import { subscribePlacement } from "./placementEvents";

/**
 * useOwnChoices lists the entries of the given sections that left their
 * section's default, read again after every placement write. It is empty
 * until both the entries and the defaults have answered.
 */
export function useOwnChoices(domains: PlacementDomain[], defaults: DefaultRow[] | null): PlacedEntry[] {
  const [entries, setEntries] = useState<PlacedEntry[]>([]);
  const wanted = domains.join(",");

  useEffect(() => {
    let alive = true;
    const load = () => {
      void readPlacedEntries(wanted === "" ? [] : (wanted.split(",") as PlacementDomain[])).then((read) => {
        if (alive) setEntries(read);
      });
    };
    load();
    const off = subscribePlacement(load);
    return () => {
      alive = false;
      off();
    };
  }, [wanted]);

  return entries.filter((entry) => {
    const row = defaults?.find((r) => r.domain === entry.item.domain);
    return row !== undefined && !row.unreadable && hasOwnChoice(entry.view, row);
  });
}
