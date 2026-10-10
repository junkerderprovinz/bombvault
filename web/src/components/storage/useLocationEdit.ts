import { useState } from "react";
import type { FollowedSetting, StorageLocation, StorageLocationSection } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { pushSaveWarnings } from "../../lib/placementCodes";
import {
  followLocation,
  saveLocation,
  writable,
  type LocationPatch,
  type LocationRecords,
  type LocationSetting,
  type SaveAnswer,
} from "../../lib/storageWrites";
import { useToast } from "../../lib/toast";
import { destinationsChanged } from "../../lib/useDestinations";
import { reposChanged } from "../../lib/useNamedRepos";
import { offsiteTargetsChanged } from "../../lib/useOffsiteTargets";
import { storageLocationsChanged } from "../../lib/useStorageLocations";

/** What a card needs to change its location: what may be changed, the save,
 *  and the way back for a section that holds a value of its own. */
export interface LocationEdit {
  location: StorageLocation;
  records: LocationRecords;
  can: (setting: LocationSetting) => boolean;
  /** Saves and reports the outcome in a toast, `done` in place of the usual
   *  "saved". Resolves to whether the server took the change. */
  save: (patch: LocationPatch, done?: string) => Promise<boolean>;
  follow: (section: StorageLocationSection, setting: FollowedSetting) => void;
  busy: boolean;
}

export function useLocationEdit(location: StorageLocation, records: LocationRecords): LocationEdit {
  const { t } = useT();
  const { push } = useToast();
  const [busy, setBusy] = useState(false);
  const settings = writable(location, records);

  async function write(run: () => Promise<SaveAnswer>, done: string): Promise<boolean> {
    setBusy(true);
    const res = await run().catch((e: unknown): SaveAnswer => ({ ok: false, error: e instanceof Error ? e.message : undefined }));
    setBusy(false);
    if (!res.ok) {
      push(res.error ?? t("settings.error"), "fail");
      return false;
    }
    push(done, "success");
    pushSaveWarnings(push, t, res.warnings);
    // The location is one view of rows other readers hold as well.
    storageLocationsChanged();
    destinationsChanged();
    offsiteTargetsChanged();
    reposChanged();
    return true;
  }

  return {
    location,
    records,
    can: (setting) => settings.has(setting),
    save: (patch, done) =>
      write(() => saveLocation(location, records, patch), done ?? t("dest.saved").replace("{name}", () => patch.name ?? location.name)),
    follow: (section, setting) => void write(() => followLocation(location, records, section, setting), t("storage.own.resetDone")),
    busy,
  };
}
