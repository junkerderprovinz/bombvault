import type { ReactNode } from "react";
import type { TimelineDomain, TimelinePlace, ZFSDatasetView } from "../../lib/api";
import { repoDisplayName } from "../../lib/directRepo";
import { useT } from "../../lib/i18n";
import { useHostLabel } from "../../lib/useHostLabel";
import type { useTimeline } from "../../lib/useTimeline";
import { Card } from "../../pages/settings/shared";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { IconRefresh } from "../glyphs";
import { InfoBubble } from "../InfoBubble";
import { IconCheck, IconCloud, IconLocal } from "../navGlyphs";
import { Rows } from "../storage/rows";
import { ReplicaPlaceRow } from "../zfs/replica/ReplicaPlaceRow";
import { ZFSSitesLine } from "../zfs/ZFSSitesLine";

/** An entry's timeline as the page reads it once for all its cards. */
export type TimelineState = ReturnType<typeof useTimeline> & { domain: TimelineDomain };

/** usePlaceLabel names a place of the timeline the way the placement bar does. */
export function usePlaceLabel(places: TimelinePlace[]): (place: string) => string {
  const { t } = useT();
  const host = useHostLabel();
  return (place) => {
    const p = places.find((x) => x.place === place);
    return (p && repoDisplayName(t, p.label, p.directOf)) || host;
  };
}

function PlaceRow({ glyph, name, children }: { glyph: ReactNode; name: ReactNode; children: ReactNode }) {
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-2 py-3 first:pt-0 last:pb-0">
      <span className="flex h-9 w-9 flex-none items-center justify-center rounded-control bg-carbon-surface2 text-accentText [&_svg]:h-[18px] [&_svg]:w-[18px]">
        {glyph}
      </span>
      <span className="min-w-24 flex-1 text-sm font-semibold text-carbon-text wrap-anywhere">{name}</span>
      <span className="flex flex-wrap items-center justify-end gap-2">{children}</span>
    </div>
  );
}


/**
 * BackupPlaces lists the storage locations that hold the entry's backups and
 * how many each holds. A location off the premises is read only when asked,
 * because listing it takes time, and says so until then. A ZFS entry lists
 * its replica below them, and how many sites the server counts with it.
 */
export function BackupPlaces({
  timeline,
  replicaOf,
  hueIndex,
}: {
  timeline: TimelineState;
  replicaOf?: ZFSDatasetView;
  hueIndex: number;
}) {
  const { t } = useT();
  const { places, rows, reading, loadPlace } = timeline;
  const labelOf = usePlaceLabel(places);
  if (places.length === 0 && !replicaOf) return null;

  return (
    <Card title={t("storage.title")} hint={t("entry.places.hint")} hueIndex={hueIndex}>
      <Rows>
        {places.map((p) => {
          const count = rows.filter((row) => row.places.some((m) => m.place === p.place)).length;
          const busy = reading.has(p.place);
          return (
            <PlaceRow key={p.place} glyph={p.remote ? <IconCloud /> : <IconLocal />} name={labelOf(p.place)}>
              {busy ? (
                <Badge tone="active">{t("entry.places.reading")}</Badge>
              ) : p.state === "read" ? (
                <Badge tone="muted">
                  <i aria-hidden="true" className="h-[7px] w-[7px] flex-none rounded-full bg-statusOkSolid" />
                  {t("entry.places.count", count)}
                </Badge>
              ) : p.state === "unreadable" ? (
                <>
                  <Badge tone="warn">
                    {t("entry.places.unreadable")}
                    <InfoBubble tip={p.error || t("entry.places.unreadableHint")} onAccent />
                  </Badge>
                  <Button
                    label={t("restoreCheck.recheck")}
                    labelKey="restoreCheck.recheck"
                    glyph={<IconRefresh />}
                    onClick={() => void loadPlace(p.place)}
                  />
                </>
              ) : (
                <>
                  <Badge tone="neutral">{t("zfs.notChecked")}</Badge>
                  <Button
                    label={t("timeline.check")}
                    labelKey="timeline.check"
                    glyph={<IconCheck />}
                    onClick={() => void loadPlace(p.place)}
                  />
                </>
              )}
            </PlaceRow>
          );
        })}
      </Rows>
      {replicaOf && (
        <>
          <ReplicaPlaceRow itemId={replicaOf.id} name={replicaOf.dataset} />
          <ZFSSitesLine item={replicaOf} />
        </>
      )}
    </Card>
  );
}
