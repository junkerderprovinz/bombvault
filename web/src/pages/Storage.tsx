// The Storage locations page: every place backups are kept, each with how full
// it is, which section is backed up to which of them, and what applies to the
// copies to all of them.
import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";

import { Badge } from "../components/Badge";
import { Button } from "../components/Button";
import { IconForward } from "../components/glyphs";
import { IconAdd } from "../components/navGlyphs";
import { PageAction, PageActions } from "../components/PageActions";
import { PageTitle } from "../components/PageTitle";
import { AddLocationWindow } from "../components/storage/AddLocationWindow";
import { FlowCard } from "../components/storage/FlowCard";
import { LocationMark } from "../components/storage/LocationMark";
import { OwnChoiceCard } from "../components/storage/OwnChoiceCard";
import { StreamingCard } from "../components/storage/StreamingCard";
import { Vessel } from "../components/storage/Vessel";
import type { StorageLocation } from "../lib/api";
import { useT } from "../lib/i18n";
import { PAGE_SHELL_RESPONSIVE } from "../lib/pageShell";
import { isPlacementDomain } from "../lib/placement";
import { subscribePlacement } from "../lib/placementEvents";
import { flowDomains } from "../lib/storageFlow";
import { locationRoute, usedByText } from "../lib/storageLocations";
import { subscribeOffsiteTargets } from "../lib/useOffsiteTargets";
import { useOwnChoices } from "../lib/useOwnChoices";
import { usePlacementDefaults } from "../lib/usePlacementDefaults";
import { storageLocationsChanged, useStorageLocations } from "../lib/useStorageLocations";
import { Card, hueCounter } from "./settings/shared";

function LocationRow({ location }: { location: StorageLocation }) {
  const { t, lang } = useT();
  const navigate = useNavigate();
  const route = locationRoute(location.id);
  return (
    <li
      data-new-way="location"
      className="relative -mx-2 flex flex-wrap items-center gap-x-3 gap-y-2 rounded-control px-2 py-3 hover:bg-carbon-hover"
    >
      <LocationMark location={location} />
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        {/* The link's box is stretched over the row, so the whole row opens
            the location while a screen reader still hears one named link. */}
        <Link
          to={route}
          className="truncate text-sm font-semibold text-carbon-text after:absolute after:inset-0 after:rounded-control focus-visible:outline-none focus-visible:after:outline-solid focus-visible:after:outline-2 focus-visible:after:outline-(--focus-ring)"
        >
          {location.name}
        </Link>
        {/* Two lines on a phone, where one would cut both halves short. */}
        <span className="flex min-w-0 flex-col text-xs text-carbon-textMuted md:flex-row md:gap-1.5">
          <span dir="ltr" className="min-w-0 truncate text-start">
            {location.where}
          </span>
          <span className="min-w-0 truncate md:before:me-1.5 md:before:content-['·']">{usedByText(t, lang, location)}</span>
        </span>
      </div>
      <div className="flex min-w-0 items-center gap-3 max-md:w-full max-md:ps-[52px]">
        {location.protection.immutable && (
          <Badge tone="ok" className="max-md:hidden">
            {t("storage.protected")}
          </Badge>
        )}
        {location.enabled ? (
          // Above the stretched link, so the vessel explains itself under the
          // pointer; a click on it still opens the location.
          <span className="relative z-10 min-w-0 cursor-pointer" onClick={() => navigate(route)}>
            <Vessel capacity={location.capacity} />
          </span>
        ) : (
          <Badge tone="neutral">{t("storage.off")}</Badge>
        )}
        <span className="ms-auto inline-flex text-carbon-textMuted">
          <IconForward />
        </span>
      </div>
    </li>
  );
}

export function StoragePage() {
  const { t } = useT();
  const { locations, loaded, failed, retry } = useStorageLocations();
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
        {failed && !loaded ? (
          // A list that never arrived says nothing about what is stored where,
          // so it must not look like an empty one.
          <div className="flex flex-col items-start gap-2">
            <p className="text-sm text-statusWarn">{t("storage.loadFailed")}</p>
            <Button label={t("anomaly.retry")} labelKey="anomaly.retry" onClick={retry} />
          </div>
        ) : !loaded ? (
          <p className="text-sm text-carbon-textSub">{t("dashboard.checking")}</p>
        ) : locations.length === 0 ? (
          <p className="text-sm text-carbon-textMuted">{t("storage.empty")}</p>
        ) : (
          <ul className="flex flex-col divide-y divide-carbon-border/60">
            {locations.map((location) => (
              <LocationRow key={location.id} location={location} />
            ))}
          </ul>
        )}
      </Card>
      {sections.length > 0 && <FlowCard locations={locations} defaults={defaults} hueIndex={nextHue()} />}
      {own.length > 0 && defaults.rows && <OwnChoiceCard entries={own} defaults={defaults.rows} hueIndex={nextHue()} />}
      <StreamingCard hueIndex={nextHue()} />
      {adding && <AddLocationWindow domains={sections} defaults={defaults.rows} onClose={() => setAdding(false)} />}
      <PageActions>
        <PageAction label={t("storage.add.title")} glyph={<IconAdd />} primary onClick={() => setAdding(true)} />
      </PageActions>
    </div>
  );
}
