import { useState } from "react";
import type { DestinationConnectionEdit, FollowedSetting, StorageLocation, StorageLocationSection } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { placementErrorText, pushSaveWarnings } from "../../lib/placementCodes";
import {
  followLocation,
  saveConnection,
  saveLocation,
  saveSection,
  writable,
  type LocationPatch,
  type LocationRecords,
  type LocationSetting,
  type SaveAnswer,
  type SectionPatch,
} from "../../lib/storageWrites";
import { useToast } from "../../lib/toast";
import { credSetsChanged } from "../../lib/useCloudCredSets";
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
  /** Gives a section a value of its own. Resolves like `save`. */
  saveSection: (section: StorageLocationSection, patch: SectionPatch) => Promise<boolean>;
  /** Gives back everything a section holds for itself. */
  followAll: (section: StorageLocationSection) => void;
  /** Changes how a destination is reached. The answer comes back whole, so
   *  the caller can say why the server refused. */
  saveConnection: (change: DestinationConnectionEdit) => Promise<SaveAnswer>;
  busy: boolean;
}

export function useLocationEdit(location: StorageLocation, records: LocationRecords): LocationEdit {
  const { t, lang } = useT();
  const { push } = useToast();
  const [busy, setBusy] = useState(false);
  const settings = writable(location, records);

  async function attempt(run: () => Promise<SaveAnswer>): Promise<SaveAnswer> {
    setBusy(true);
    const res = await run().catch((e: unknown): SaveAnswer => ({ ok: false, error: e instanceof Error ? e.message : undefined }));
    setBusy(false);
    return res;
  }

  function taken(res: SaveAnswer, done: string) {
    push(done, "success");
    pushSaveWarnings(push, t, res.warnings);
    // The location is one view of rows other readers hold as well.
    storageLocationsChanged();
    destinationsChanged();
    offsiteTargetsChanged();
    reposChanged();
  }

  async function write(run: () => Promise<SaveAnswer>, done: string): Promise<boolean> {
    const res = await attempt(run);
    if (!res.ok) {
      push(placementErrorText(t, lang, res, "settings.error"), "fail");
      return false;
    }
    taken(res, done);
    return true;
  }

  return {
    location,
    records,
    can: (setting) => settings.has(setting),
    save: (patch, done) =>
      write(() => saveLocation(location, records, patch), done ?? t("dest.saved").replace("{name}", () => patch.name ?? location.name)),
    follow: (section, setting) => void write(() => followLocation(location, records, section, [setting]), t("storage.own.resetDone")),
    saveSection: (section, patch) =>
      write(() => saveSection(location, records, section, patch), t("dest.saved").replace("{name}", () => location.name)),
    followAll: (section) => void write(() => followLocation(location, records, section, section.own), t("storage.own.resetDone")),
    saveConnection: async (change) => {
      const res = await attempt(() => saveConnection(location, change));
      if (res.ok) {
        taken(res, t("dest.saved").replace("{name}", () => location.name));
        // A changed sign-in is written into the credential set behind it.
        credSetsChanged();
      }
      return res;
    },
    busy,
  };
}
