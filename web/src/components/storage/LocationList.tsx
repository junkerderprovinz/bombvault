import { Link, useNavigate } from "react-router-dom";

import type { StorageLocation } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { locationRoute, usedByText } from "../../lib/storageLocations";
import type { StorageLocationList } from "../../lib/useStorageLocations";
import { CardFoot } from "../../pages/settings/shared";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { IconForward } from "../glyphs";
import { LocationMark } from "./LocationMark";
import { Vessel } from "./Vessel";

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

/** LocationList lists the storage locations, each row opening its page. */
export function LocationList({ list }: { list: StorageLocationList }) {
  const { t } = useT();
  const { locations, loaded, failed, retry } = list;
  if (failed && !loaded) {
    // A list that never arrived says nothing about what is stored where, so
    // it must not look like an empty one.
    return (
      <div className="flex flex-col gap-2">
        <p className="text-sm text-statusWarn">{t("storage.loadFailed")}</p>
        <CardFoot>
          <Button label={t("anomaly.retry")} labelKey="anomaly.retry" onClick={retry} />
        </CardFoot>
      </div>
    );
  }
  if (!loaded) return <p className="text-sm text-carbon-textSub">{t("dashboard.checking")}</p>;
  if (locations.length === 0) return <p className="text-sm text-carbon-textMuted">{t("storage.empty")}</p>;
  return (
    <ul className="flex flex-col divide-y divide-carbon-border/60">
      {locations.map((location) => (
        <LocationRow key={location.id} location={location} />
      ))}
    </ul>
  );
}
