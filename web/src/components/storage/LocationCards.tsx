import { useEffect, useState } from "react";
import { useT } from "../../lib/i18n";
import { isPlacementDomain } from "../../lib/placement";
import { subscribePlacement } from "../../lib/placementEvents";
import { flowDomains } from "../../lib/storageFlow";
import { subscribeOffsiteTargets } from "../../lib/useOffsiteTargets";
import { useOwnChoices } from "../../lib/useOwnChoices";
import { usePlacementDefaults } from "../../lib/usePlacementDefaults";
import { storageLocationsChanged, type StorageLocationList } from "../../lib/useStorageLocations";
import { Card } from "../../pages/settings/shared";
import { SignInsCard } from "../../pages/settings/SignInsCard";
import { Button } from "../Button";
import { AddLocationWindow } from "./AddLocationWindow";
import { FlowCard } from "./FlowCard";
import { LocationList } from "./LocationList";

/**
 * LocationCards are the storage locations as a list with the way to add one,
 * and under it which section is backed up to which of them. The Storage
 * locations page and its settings page both show them, each reading the list
 * the way it hears of finished runs.
 */
export function LocationCards({
  list,
  nextHue,
  newId,
  flowNewId,
  signIns = false,
}: {
  list: StorageLocationList;
  nextHue: () => number;
  newId: string;
  flowNewId?: string;
  /** Shows the saved sign-ins under the list, as the settings page does. */
  signIns?: boolean;
}) {
  const { t } = useT();
  const { locations } = list;
  const defaults = usePlacementDefaults();
  const sections = flowDomains(locations);
  const own = useOwnChoices(sections.filter(isPlacementDomain), defaults.rows);
  const [adding, setAdding] = useState(false);

  // A location is one view of the targets and defaults the cards write.
  useEffect(() => {
    const offs = [subscribeOffsiteTargets(storageLocationsChanged), subscribePlacement(storageLocationsChanged)];
    return () => offs.forEach((off) => off());
  }, []);

  return (
    <>
      <Card
        title={t("storage.title")}
        hint={t("storage.listHint")}
        hueIndex={nextHue()}
        newId={newId}
        foot={
          <span data-new="card-foot" data-new-way="add-location" className="flex">
            <Button label={t("storage.add.title")} labelKey="storage.add.title" tone="accent" onClick={() => setAdding(true)} />
          </span>
        }
      >
        <LocationList list={list} />
      </Card>
      {signIns && <SignInsCard hueIndex={nextHue()} />}
      {sections.length > 0 && <FlowCard locations={locations} defaults={defaults} own={own} hueIndex={nextHue()} newId={flowNewId} />}
      {adding && <AddLocationWindow domains={sections} defaults={defaults.rows} onClose={() => setAdding(false)} />}
    </>
  );
}
