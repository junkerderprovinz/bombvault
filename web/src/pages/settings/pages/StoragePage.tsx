import { LocationList } from "../../../components/storage/LocationList";
import { useT } from "../../../lib/i18n";
import { useStorageLocationsOnDemand } from "../../../lib/useStorageLocations";
import { ReposCard } from "../ReposCard";
import { SidebarCard } from "../SidebarTile";
import { CloudCards } from "../sections/CloudCards";
import { OffsiteCards } from "../sections/OffsiteCards";
import { RetentionCards } from "../sections/RetentionCards";
import { Card, hueCounter } from "../shared";

export function StoragePage() {
  const { t } = useT();
  const list = useStorageLocationsOnDemand();
  const nextHue = hueCounter();

  return (
    <>
      <SidebarCard pin="storage" hueIndex={nextHue()} />
      <Card title={t("storage.title")} hint={t("storage.listHint")} hueIndex={nextHue()} newId="settings-storage-list">
        <LocationList list={list} />
      </Card>

      <ReposCard hueIndex={nextHue()} />

      <RetentionCards nextHue={nextHue} />
      <OffsiteCards nextHue={nextHue} />
      <CloudCards nextHue={nextHue} />
    </>
  );
}
