import { LocationCards } from "../../../components/storage/LocationCards";
import { useStorageLocationsOnDemand } from "../../../lib/useStorageLocations";
import { ReposCard } from "../ReposCard";
import { SidebarCard } from "../SidebarTile";
import { OffsiteCards } from "../sections/OffsiteCards";
import { RetentionCards } from "../sections/RetentionCards";
import { hueCounter } from "../shared";

export function StoragePage() {
  const list = useStorageLocationsOnDemand();
  const nextHue = hueCounter();

  return (
    <>
      <SidebarCard pin="storage" hueIndex={nextHue()} />
      <LocationCards list={list} nextHue={nextHue} newId="settings-storage-list" flowNewId="settings-storage-flow" signIns />

      <ReposCard hueIndex={nextHue()} />

      <RetentionCards nextHue={nextHue} />
      <OffsiteCards nextHue={nextHue} />
    </>
  );
}
