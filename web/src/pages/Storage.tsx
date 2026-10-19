// The Storage locations page: every place backups are kept, each with how full
// it is, which section is backed up to which of them, and what applies to the
// copies to all of them.
import { useEffect, useState } from "react";

import { IconAdd } from "../components/navGlyphs";
import { PageAction, PageActions } from "../components/PageActions";
import { PageTitle } from "../components/PageTitle";
import { AddLocationWindow } from "../components/storage/AddLocationWindow";
import { FlowCard } from "../components/storage/FlowCard";
import { LocationList } from "../components/storage/LocationList";
import { StreamingCard } from "../components/storage/StreamingCard";
import { useT } from "../lib/i18n";
import { PAGE_SHELL_RESPONSIVE } from "../lib/pageShell";
import { isPlacementDomain } from "../lib/placement";
import { subscribePlacement } from "../lib/placementEvents";
import { flowDomains } from "../lib/storageFlow";
import { subscribeOffsiteTargets } from "../lib/useOffsiteTargets";
import { useOwnChoices } from "../lib/useOwnChoices";
import { usePlacementDefaults } from "../lib/usePlacementDefaults";
import { storageLocationsChanged, useStorageLocations } from "../lib/useStorageLocations";
import { Card, hueCounter } from "./settings/shared";

export function StoragePage() {
  const { t } = useT();
  const list = useStorageLocations();
  const { locations } = list;
  const defaults = usePlacementDefaults();
  const sections = flowDomains(locations);
  const own = useOwnChoices(sections.filter(isPlacementDomain), defaults.rows);
  const [adding, setAdding] = useState(false);
  const nextHue = hueCounter();

  // A location is one view of the targets and defaults the cards below write.
  useEffect(() => {
    const offs = [subscribeOffsiteTargets(storageLocationsChanged), subscribePlacement(storageLocationsChanged)];
    return () => offs.forEach((off) => off());
  }, []);

  return (
    <div className={PAGE_SHELL_RESPONSIVE}>
      <PageTitle>{t("storage.title")}</PageTitle>
      <Card title={t("storage.title")} hint={t("storage.listHint")} hueIndex={nextHue()} newId="storage-list">
        <LocationList list={list} />
      </Card>
      {sections.length > 0 && <FlowCard locations={locations} defaults={defaults} own={own} hueIndex={nextHue()} />}
      <StreamingCard hueIndex={nextHue()} />
      {adding && <AddLocationWindow domains={sections} defaults={defaults.rows} onClose={() => setAdding(false)} />}
      <PageActions>
        <PageAction label={t("storage.add.title")} glyph={<IconAdd />} primary onClick={() => setAdding(true)} />
      </PageActions>
    </div>
  );
}
