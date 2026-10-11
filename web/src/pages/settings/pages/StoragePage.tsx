import { useEffect, useState } from "react";
import { IconTabStorage } from "../../../components/navGlyphs";
import { useT } from "../../../lib/i18n";
import { listStorageLocations } from "../../../lib/api";
import { ReposCard } from "../ReposCard";
import { PageLinkCard, SidebarCard } from "../SidebarTile";
import { CloudCards } from "../sections/CloudCards";
import { OffsiteCards } from "../sections/OffsiteCards";
import { RetentionCards } from "../sections/RetentionCards";
import { hueCounter } from "../shared";

export function StoragePage() {
  const { t } = useT();
  const [counts, setCounts] = useState<{ all: number; offsite: number } | null>(null);

  useEffect(() => {
    let alive = true;
    listStorageLocations()
      .then((res) => {
        const locations = res.locations ?? [];
        if (alive && res.ok) setCounts({ all: locations.length, offsite: locations.filter((l) => l.kind === "offsite").length });
      })
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, []);

  const nextHue = hueCounter();

  return (
    <>
      <SidebarCard pin="storage" hueIndex={nextHue()} />
      <PageLinkCard
        to="/storage"
        label={t("storage.title")}
        sub={
          counts
            ? t("tile.storage.sub").replace("{n}", String(counts.all)).replace("{m}", String(counts.offsite))
            : t("dashboard.checking")
        }
        glyph={<IconTabStorage />}
        hueIndex={nextHue()}
      />

      <ReposCard hueIndex={nextHue()} />

      <RetentionCards nextHue={nextHue} />
      <OffsiteCards nextHue={nextHue} />
      <CloudCards nextHue={nextHue} />
    </>
  );
}
