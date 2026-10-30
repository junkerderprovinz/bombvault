import { useT } from "../../lib/i18n";
import { Button } from "../Button";
import { usePlaceLabel, type TimelineState } from "../entry/BackupPlaces";
import { IconRetry } from "../glyphs";
import { IconCheck } from "../navGlyphs";

/** PlaceChecks offers to read the storage locations nobody has read yet.
 *  Listing one off the premises takes time, so it waits to be asked. */
export function PlaceChecks({ timeline }: { timeline: TimelineState }) {
  const { t } = useT();
  const { places, reading, loadPlace } = timeline;
  const labelOf = usePlaceLabel(places, timeline.domain);
  const open = places.filter((p) => p.state !== "read");
  if (open.length === 0) return null;
  return (
    <ul className="flex flex-col gap-1.5">
      {open.map((p) => (
        <li key={p.place} className="flex flex-wrap items-center gap-2 text-xs text-carbon-textMuted">
          <span className="min-w-0 wrap-anywhere">
            {t(p.state === "unreadable" ? "timeline.unreadable" : "timeline.unchecked").replace("{place}", () => labelOf(p.place))}
          </span>
          <Button
            label={t(p.state === "unreadable" ? "restoreCheck.recheck" : "timeline.check")}
            labelKey={p.state === "unreadable" ? "restoreCheck.recheck" : "timeline.check"}
            glyph={p.state === "unreadable" ? <IconRetry /> : <IconCheck />}
            tone="subtle"
            busy={reading.has(p.place)}
            disabled={reading.has(p.place)}
            onClick={() => void loadPlace(p.place)}
          />
        </li>
      ))}
    </ul>
  );
}
