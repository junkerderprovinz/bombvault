import { Fragment } from "react";
import { Link, useNavigate } from "react-router-dom";

import { Badge } from "../../components/Badge";
import { Button } from "../../components/Button";
import { InfoBubble } from "../../components/InfoBubble";
import { LocationMark } from "../../components/storage/LocationMark";
import { Vessel } from "../../components/storage/Vessel";
import type { StorageLocation } from "../../lib/api";
import { FORECAST_CAP_WEEKS, humanBytes } from "../../lib/forecast";
import type { useT } from "../../lib/i18n";
import { fillOf, locationRoute } from "../../lib/storageLocations";
import type { StorageLocationList } from "../../lib/useStorageLocations";
import { Card } from "./Card";

type T = ReturnType<typeof useT>["t"];

/** How fast the location grows and when it would be full at that pace. */
function Forecast({ location, t }: { location: StorageLocation; t: T }) {
  const { capacity } = location;
  const fill = fillOf(capacity);
  const growth = capacity.growthBytesPerWeek;
  const weeks = fill.state === "measured" ? fill.weeks : undefined;
  const parts = [
    growth !== undefined &&
      growth !== 0 &&
      t(growth < 0 ? "dashboard.forecastShrink" : "dashboard.forecastGrowth").replace("{bytes}", humanBytes(Math.abs(growth))),
    weeks !== undefined && (
      <span className={fill.state === "measured" && fill.warn ? "text-statusWarn" : undefined}>
        {weeks > FORECAST_CAP_WEEKS ? t("storage.vessel.fullOverYear") : t("storage.vessel.fullInWeeks", weeks)}
      </span>
    ),
    fill.state === "unsupported" && (
      <span className="relative z-10 inline-flex items-center gap-1 align-middle">
        {t("dashboard.forecastFreeUnknown")}
        <InfoBubble tip={t("dashboard.forecastFreeUnknownInfo")} />
      </span>
    ),
  ].filter(Boolean);
  if (parts.length === 0) return null;
  return (
    <span className="text-xs text-carbon-textMuted">
      {parts.map((part, i) => (
        <Fragment key={i}>
          {i > 0 && " · "}
          {part}
        </Fragment>
      ))}
    </span>
  );
}

function LocationRow({ location, t }: { location: StorageLocation; t: T }) {
  const navigate = useNavigate();
  const route = locationRoute(location.id);
  return (
    <li
      data-testid="target-row"
      className="relative -mx-2 flex flex-wrap items-center gap-x-3 gap-y-2 rounded-control border-t border-carbon-border px-2 py-3 first:border-t-0 hover:bg-carbon-hover"
    >
      <LocationMark location={location} />
      <div className="flex min-w-0 flex-1 basis-40 flex-col gap-0.5">
        {/* The link's box is stretched over the row, so the whole row opens
            the location while a screen reader still hears one named link. */}
        <Link
          to={route}
          className="truncate text-sm font-semibold text-carbon-text after:absolute after:inset-0 after:rounded-control focus-visible:outline-none focus-visible:after:outline-solid focus-visible:after:outline-2 focus-visible:after:outline-(--focus-ring)"
        >
          {location.name}
        </Link>
        {location.enabled && <Forecast location={location} t={t} />}
      </div>
      {location.enabled ? (
        // Above the stretched link, so the vessel explains itself under the
        // pointer; a click on it still opens the location.
        <span className="relative z-10 min-w-0 cursor-pointer" onClick={() => navigate(route)}>
          <Vessel capacity={location.capacity} forecast={false} />
        </span>
      ) : (
        <Badge tone="neutral">{t("storage.off")}</Badge>
      )}
    </li>
  );
}

/**
 * TargetsCard shows every storage location with how full it is and when it
 * would be full at the growth of the last weeks.
 */
export function TargetsCard({ t, list, hueIndex }: { t: T; list: StorageLocationList; hueIndex?: number }) {
  const { locations, loaded, failed, retry } = list;
  return (
    <Card
      title={t("storage.title")}
      hint={t("overview.targets.hint")}
      hueIndex={hueIndex}
      foot={failed && !loaded && <Button label={t("anomaly.retry")} labelKey="anomaly.retry" onClick={retry} />}
    >
      {failed && !loaded ? (
        // A list that never arrived says nothing about what is stored where,
        // so it must not look like an empty one.
        <p className="text-sm text-statusWarn">{t("storage.loadFailed")}</p>
      ) : !loaded ? (
        <p className="text-sm text-carbon-textSub">{t("dashboard.checking")}</p>
      ) : locations.length === 0 ? (
        <p className="text-sm text-carbon-textMuted">{t("storage.empty")}</p>
      ) : (
        <ul className="flex flex-col glim-content-fade">
          {locations.map((location) => (
            <LocationRow key={location.id} location={location} t={t} />
          ))}
        </ul>
      )}
    </Card>
  );
}
