import { useCallback, useState } from "react";

import type { StorageLocation } from "./api";
import { useAttention } from "./attention";
import {
  addedLocations,
  foundOnServer,
  rememberSectionsChosen,
  sectionsChosen,
  stepStates,
  type FirstStep,
  type Found,
  type StepState,
} from "./firstSteps";
import type { BackupItems } from "./useBackupItems";
import { usePlatform } from "./useHostLabel";

export interface FirstSteps {
  states: Record<FirstStep, StepState>;
  /** The entries as the server sent them, and the way to read them again. */
  items: BackupItems;
  found: Found;
  /** "unraid", "truenas" or "generic", and "" until the server has said. */
  platform: string;
  /** Every storage location, and those among them that somebody set up. */
  locations: StorageLocation[];
  added: StorageLocation[];
  /** Ticks off the first step. */
  chooseSections: () => void;
}

/** useFirstSteps reads where a new install stands on the way to its first
 *  backup, for the welcome page and the card on the overview. */
export function useFirstSteps(): FirstSteps {
  const { items, locations: list } = useAttention();
  const locations = list.locations;
  const platform = usePlatform();
  const [chosen, setChosen] = useState(sectionsChosen);
  const chooseSections = useCallback(() => {
    rememberSectionsChosen();
    setChosen(true);
  }, []);

  const added = addedLocations(locations);
  return {
    states: stepStates({ sections: chosen, location: added.length > 0 }),
    items,
    found: foundOnServer(items.reported, platform),
    platform,
    locations,
    added,
    chooseSections,
  };
}
