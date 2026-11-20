import type { ReactNode } from "react";
import type { TileMark } from "@glimstone/core";
import { Badge as GlimBadge } from "@glimstone/react/Badge";
import { ConnectionRow } from "@glimstone/react/ConnectionRow";
import { Link, useNavigate } from "react-router-dom";

import type { StorageLocation } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { amountText, fillOf, locationRoute, usedByText } from "../../lib/storageLocations";
import type { StorageLocationList } from "../../lib/useStorageLocations";
import { CardFoot } from "../../pages/settings/shared";
import { Button } from "../Button";
import { ProviderMark, storageEntry } from "../destinations/providers";
import { IconForward } from "../glyphs";
import { InfoBubble } from "../InfoBubble";
import { IconCloud, IconLocal } from "../navGlyphs";
import { fillTip, forecastText } from "./Vessel";

/** The tile a location wears: its provider's mark on the brand's colour, or
 *  a glyph on the accent for a folder on this host and an address typed in. */
function locationMark(location: StorageLocation): TileMark | ReactNode {
  const brand = storageEntry(location.provider)?.mark;
  if (brand) return brand;
  if (location.provider) return <ProviderMark id={location.provider} />;
  return location.kind === "local" ? <IconLocal /> : <IconCloud />;
}

/** LocationRow is one storage location as a connection row. The whole row
 *  opens the location's page, as does Open. `note` stands in for the sections
 *  that use it. */
export function LocationRow({ location, note }: { location: StorageLocation; note?: string }) {
  const { t, lang } = useT();
  const navigate = useNavigate();
  const folder = location.object === "path";
  // Only a backend that opens several connections at once has the setting.
  const network = location.connections !== undefined;
  const route = locationRoute(location.id);
  const fill = fillOf(location.capacity);
  const measured = fill.state === "measured";
  const soon = location.enabled && measured && fill.warn;
  const amount = amountText(t, location.capacity);
  const outlook = measured ? (soon ? "" : forecastText(t, fill.weeks)) : t("storage.vessel.unknown");
  return (
    <li data-new-way={folder ? "location folder" : network ? "location network" : "location"} className="glim-row-linked">
      {/* Stretched over the row below it, so the whole row opens the location
          while a screen reader hears one named link. */}
      <Link to={route} className="glim-row-link">
        {location.name}
      </Link>
      <ConnectionRow
        name={location.name}
        title={location.name}
        mark={locationMark(location)}
        status={note ?? usedByText(t, lang, location)}
        badges={
          <>
            {location.protection.immutable && (
              <GlimBadge tone="ok" size="small">
                {t("storage.protected")}
              </GlimBadge>
            )}
            {!location.enabled && (
              <GlimBadge tone="neutral" size="small">
                {t("storage.off")}
              </GlimBadge>
            )}
            {soon && (
              <GlimBadge tone="warn" size="small">
                {forecastText(t, fill.weeks)}
              </GlimBadge>
            )}
          </>
        }
        actions={[{ id: "open", label: t("instances.open"), labelKey: "instances.open", glyph: <IconForward />, onClick: () => navigate(route) }]}
        text={{ test: t("offsite.test"), edit: t("common.edit"), remove: t("offsite.targets.remove") }}
      >
        {/* On a phone what it holds goes under the address instead of
            pushing past the edge, and drops the dot that would start its line. */}
        <span className="flex min-w-0 flex-wrap items-center gap-x-1.5 sm:flex-nowrap">
          <span dir="ltr" title={location.where} className="min-w-0 max-w-full truncate text-start">
            {location.where}
          </span>
          {location.enabled && (
            <>
              {amount && <span className="shrink-0 before:me-1.5 before:content-['·'] max-sm:before:content-none">{amount}</span>}
              {outlook && <span className="shrink-0 before:me-1.5 before:content-['·'] max-sm:before:content-none">{outlook}</span>}
              <InfoBubble tip={fillTip(t, fill)} />
            </>
          )}
        </span>
      </ConnectionRow>
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
    <ul className="flex flex-col gap-2">
      {locations.map((location) => (
        <LocationRow key={location.id} location={location} />
      ))}
    </ul>
  );
}
