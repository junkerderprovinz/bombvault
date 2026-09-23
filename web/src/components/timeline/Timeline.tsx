import { Fragment, useState, type ReactNode } from "react";
import type { TimelineDomain, TimelineMark, TimelinePlace, TimelineRow } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { autoMark, newestId, sourceOfPlace } from "../../lib/timeline";
import { useConfirm } from "../../lib/useConfirm";
import { useHostLabel } from "../../lib/useHostLabel";
import { useTimeline } from "../../lib/useTimeline";
import { Button } from "../Button";
import { InfoBubble } from "../InfoBubble";
import { Selector } from "../Selector";
import type { RepoSource } from "../SourceToggle";
import { TimelineDeleteDialog } from "./TimelineDeleteDialog";

export interface TimelinePick {
  row: TimelineRow;
  mark: TimelineMark;
  place: TimelinePlace;
  snapshotId: string;
  source: RepoSource;
  /** Call when the place answered snapshot-missing. */
  onMissing: () => void;
  /** Call when the snapshot changed at its place, after a new tag for instance. */
  refresh: () => void;
}

/** Timeline shows one row per backup of an item, with a mark for every place
 *  that holds it. Remote places are read only when asked for, so a page opens
 *  without waiting for a bucket. */
export function Timeline({
  domain,
  itemKey,
  itemName,
  open,
  renderActions,
  header,
}: {
  domain: TimelineDomain;
  itemKey: string;
  itemName: string;
  open: boolean;
  renderActions: (pick: TimelinePick) => ReactNode;
  /** Rendered above the rows once they are loaded. The places come with it so
   *  a header can tell an empty list from one whose places nobody has read. */
  header?: (rows: TimelineRow[], places: TimelinePlace[]) => ReactNode;
}) {
  const { t } = useT();
  const host = useHostLabel();
  const { places, rows, loading, error, reading, loadPlace, reload } = useTimeline(domain, itemKey, open);
  const [chosen, setChosen] = useState<Record<string, string | null>>({});
  const [deleting, setDeleting] = useState<{ row: TimelineRow; places: string[] } | null>(null);
  const { confirm, confirmDialog } = useConfirm();

  if (!open) return null;

  const labelOf = (place: string) => places.find((p) => p.place === place)?.label || host;
  const orderOf = (place: string) => places.findIndex((p) => p.place === place);
  const pending = places.filter((p) => p.state !== "read");
  const unchecked = pending.filter((p) => p.state === "unchecked");

  function selected(row: TimelineRow): TimelineMark | null {
    const want = chosen[row.key];
    if (want === null) return null;
    return row.places.find((m) => m.place === want) ?? autoMark(row, places);
  }

  async function missing(row: TimelineRow, gone: string) {
    const next = autoMark({ ...row, places: row.places.filter((m) => m.place !== gone) }, places);
    await loadPlace(gone);
    if (next === null) {
      setChosen((c) => ({ ...c, [row.key]: null }));
      return;
    }
    const yes = await confirm(
      t("timeline.snapshotMissing")
        .replace("{place}", () => labelOf(gone))
        .replace("{next}", () => labelOf(next.place))
    );
    setChosen((c) => ({ ...c, [row.key]: yes ? next.place : null }));
  }

  return (
    <div role="group" aria-label={itemName} className="flex flex-col">
      {/* A place that was not read keeps its line rather than disappearing, so
          nobody reads an empty list as "there is nothing left". */}
      {pending.map((p) => (
        <div
          key={p.place}
          className="flex items-center gap-2 py-1.5 border-b border-carbon-border text-xs text-carbon-textMuted"
        >
          <span>
            {t(p.state === "unreadable" ? "timeline.unreadable" : "timeline.unchecked").replace(
              "{place}",
              () => labelOf(p.place)
            )}
          </span>
          {p.error && <InfoBubble tip={p.error} />}
          <Button
            label={t("timeline.check")}
            labelKey="timeline.check"
            tone="neutral"
            disabled={reading.has(p.place)}
            onClick={() => void loadPlace(p.place)}
            className="ms-auto"
          />
        </div>
      ))}
      {loading && <p className="py-3 text-xs text-carbon-textMuted">{t("common.loadingBackups")}</p>}
      {error !== null && <p className="py-3 text-xs text-statusFail">{error || t("common.loadBackupsFailed")}</p>}
      {!loading && error === null && header?.(rows, places)}
      {!loading && error === null && rows.length === 0 && pending.length === 0 && (
        <p className="py-3 text-xs text-carbon-textMuted">{t("snapshots.none")}</p>
      )}
      {!loading &&
        rows.map((row) => {
          const mark = selected(row);
          const place = mark ? places.find((p) => p.place === mark.place) : undefined;
          const marks = [...row.places].sort((a, b) => orderOf(a.place) - orderOf(b.place));
          return (
            <div key={row.key} className="flex flex-col gap-1 py-1.5 border-b border-carbon-border last:border-0">
              <div className="flex items-center gap-3 flex-wrap text-sm">
                <span dir="ltr" className="font-mono text-start text-carbon-text text-xs w-20 shrink-0">
                  {(mark ? newestId(mark) : row.key).slice(0, 8)}
                </span>
                <span className="text-carbon-textMuted text-xs">{new Date(row.time).toLocaleString()}</span>
                <Selector
                  items={marks.map((m) => ({
                    id: m.place,
                    label: m.incomplete ? `${labelOf(m.place)} · ${t("timeline.incomplete")}` : labelOf(m.place),
                    tip: m.incomplete ? t("timeline.incompleteHint") : undefined,
                  }))}
                  label={t("source.label")}
                  select="one"
                  active={mark?.place ?? null}
                  onChange={(id) => setChosen((c) => ({ ...c, [row.key]: id }))}
                  size="sm"
                />
                {pending.length === 0 && row.places.length === 1 && (
                  <span className="text-caption text-carbon-textMuted">{t("timeline.onlyHere")}</span>
                )}
              </div>
              {mark && place && (
                <div className="flex items-center gap-2 flex-wrap">
                  {/* Keyed on the place so switching it resets whatever the page
                      keeps in its own controls, a typed name or a ticked box. */}
                  <Fragment key={mark.place}>
                    {renderActions({
                      row,
                      mark,
                      place,
                      snapshotId: newestId(mark),
                      source: sourceOfPlace(mark.place),
                      onMissing: () => void missing(row, mark.place),
                      refresh: () => void loadPlace(mark.place),
                    })}
                  </Fragment>
                  <Button
                    label={t("common.delete")}
                    labelKey="common.delete"
                    tone="neutral"
                    disabled={place.appendOnly}
                    title={place.appendOnly ? t("placementCode.appendOnly") : undefined}
                    onClick={() => setDeleting({ row, places: [mark.place] })}
                  />
                  <Button
                    label={t("timeline.deleteRow")}
                    labelKey="timeline.deleteRow"
                    tone="neutral"
                    onClick={() => setDeleting({ row, places: [] })}
                  />
                </div>
              )}
            </div>
          );
        })}
      {!loading && unchecked.length > 0 && (
        <Button
          label={t("timeline.showOlder")}
          labelKey="timeline.showOlder"
          tone="neutral"
          disabled={unchecked.some((p) => reading.has(p.place))}
          onClick={() => unchecked.forEach((p) => void loadPlace(p.place))}
          className="self-start my-2"
        />
      )}
      {deleting && (
        // The dialog runs its whole question once per mount, so a second
        // delete asked before the first has closed needs a fresh instance.
        <TimelineDeleteDialog
          key={`${deleting.row.key}:${deleting.places.join(",")}`}
          domain={domain}
          itemKey={itemKey}
          row={deleting.row}
          places={deleting.places}
          onDone={() => {
            setDeleting(null);
            reload();
          }}
          onClose={() => setDeleting(null)}
        />
      )}
      {confirmDialog}
    </div>
  );
}
