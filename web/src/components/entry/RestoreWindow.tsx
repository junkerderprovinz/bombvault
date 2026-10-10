import { useState, type ReactNode } from "react";
import { useNavigate } from "react-router-dom";
import type { DBDumpView, TimelineRow } from "../../lib/api";
import { pairsWith } from "../../lib/dbdump";
import type { Entry } from "../../lib/entry";
import { useT } from "../../lib/i18n";
import { placementChanged } from "../../lib/placementEvents";
import { anyActive, useProgress } from "../../lib/progress";
import { formatTs } from "../../lib/reltime";
import { autoMark, newestId, sourceOfPlace } from "../../lib/timeline";
import { useConfirm } from "../../lib/useConfirm";
import { AnomalyWindow } from "../anomalies/AnomalyWindow";
import { Button } from "../Button";
import { FileSetRestoreControl } from "../files/FileSetRestoreControl";
import { FlashActions } from "../flash/FlashActions";
import { IconRecovery, IconTrash } from "../navGlyphs";
import { DEFAULT_RESTORE_FOLDER, SnapshotActions } from "../RestorePanel";
import { Selector } from "../Selector";
import type { TimelinePick } from "../timeline/Timeline";
import { TimelineDeleteDialog } from "../timeline/TimelineDeleteDialog";
import { VMSnapshotActions } from "../vms/VMRestorePanel";
import { usePlaceLabel, type TimelineState } from "./BackupPlaces";

/**
 * RestoreWindow opens one backup: the place to take it from, what the kind
 * can do with it, and deleting it. What the kind can do is the restore panel
 * of that kind.
 */
export function RestoreWindow({
  entry,
  name,
  row,
  place: asked,
  timeline,
  dumps,
  onClose,
}: {
  entry: Entry;
  name: string;
  row: TimelineRow;
  /** The place a click on its mark asked for. Without one the first place
   *  that holds the whole backup is taken. */
  place?: string;
  timeline: TimelineState;
  /** The container's database dumps, to say which backup one belongs to. */
  dumps: DBDumpView[];
  onClose: () => void;
}) {
  const { t } = useT();
  const navigate = useNavigate();
  const { confirm, confirmDialog } = useConfirm();
  const running = anyActive(useProgress());
  const { places, loadPlace, reload } = timeline;
  const labelOf = usePlaceLabel(places);
  const [chosen, setChosen] = useState(asked);
  const [deleting, setDeleting] = useState<string[] | null>(null);
  const { detail, settings, hostMountRoot } = entry;

  const mark = row.places.find((m) => m.place === chosen) ?? autoMark(row, places) ?? row.places[0];
  const place = places.find((p) => p.place === mark.place);
  const orderOf = (id: string) => places.findIndex((p) => p.place === id);
  const marks = [...row.places].sort((a, b) => orderOf(a.place) - orderOf(b.place));
  const restoreFolder = settings.restoreFolder || DEFAULT_RESTORE_FOLDER;

  // The place answered that it no longer holds the backup. It is read again,
  // and the next place that holds it is offered.
  async function missing() {
    const gone = mark.place;
    const next = autoMark({ ...row, places: row.places.filter((m) => m.place !== gone) }, places);
    await loadPlace(gone);
    if (next === null) {
      onClose();
      return;
    }
    const yes = await confirm(
      t("timeline.snapshotMissing")
        .replace("{place}", () => labelOf(gone))
        .replace("{next}", () => labelOf(next.place))
    );
    if (yes) setChosen(next.place);
    else onClose();
  }

  // Between a delete and the read after it the row can name a place the
  // list has dropped.
  if (!place) return null;

  const pick: TimelinePick = {
    row,
    mark,
    place,
    snapshotId: newestId(mark),
    source: sourceOfPlace(mark.place),
    onMissing: () => void missing(),
    refresh: () => void loadPlace(mark.place),
    lead: true,
  };

  let actions: ReactNode;
  switch (detail.kind) {
    case "container":
      actions = (
        <SnapshotActions
          pick={pick}
          containerName={detail.container.name}
          aliases={detail.container.aliases ?? []}
          hostMountRoot={hostMountRoot}
          defaultFolder={restoreFolder}
          paired={dumps.some((dump) => pairsWith(dump, { id: pick.snapshotId, original: row.key }))}
          coverage={detail.container.dbDataCoverage}
          preselected
          t={t}
        />
      );
      break;
    case "vm":
      actions = (
        <VMSnapshotActions pick={pick} vmName={detail.vm.libvirtName} vmDisplayName={detail.vm.name} preselected t={t} />
      );
      break;
    case "files":
      actions = (
        <div className="basis-full">
          <FileSetRestoreControl
            set={detail.set}
            snapshotId={pick.snapshotId}
            source={pick.source}
            hostMountRoot={hostMountRoot}
            restoreFolder={restoreFolder}
            otherActive={running}
            onMissing={pick.onMissing}
            lead
            t={t}
          />
        </div>
      );
      break;
    case "flash":
      actions = <FlashActions pick={pick} t={t} />;
      break;
    case "config":
      actions = (
        <>
          <p className="basis-full text-sm text-carbon-textSub">{t("entry.config.restoreText")}</p>
          <Button
            label={t("entry.config.startRecovery")}
            labelKey="entry.config.startRecovery"
            glyph={<IconRecovery />}
            tone="accent"
            onClick={() => navigate("/recovery")}
          />
        </>
      );
      break;
    default:
      actions = null;
  }

  return (
    <AnomalyWindow
      title={t("entry.restore.title").replace("{name}", () => name)}
      onClose={onClose}
      actions={
        <>
          <Button
            label={t("common.delete")}
            labelKey="common.delete"
            glyph={<IconTrash />}
            disabled={place.appendOnly}
            hint={place.appendOnly ? t("placementCode.appendOnly") : undefined}
            onClick={() => setDeleting([mark.place])}
          />
          {row.places.length > 1 && (
            <Button
              label={t("timeline.deleteRow")}
              labelKey="timeline.deleteRow"
              glyph={<IconTrash />}
              onClick={() => setDeleting([])}
            />
          )}
        </>
      }
    >
      <p className="text-sm text-carbon-textMuted">
        {t("entry.restore.sub")
          .replace("{when}", formatTs(Math.floor(Date.parse(row.time) / 1000)))
          .replace("{source}", () => labelOf(mark.place))}
      </p>
      {marks.length > 1 && (
        <Selector
          items={marks.map((m) => ({
            id: m.place,
            label: m.incomplete ? `${labelOf(m.place)} · ${t("timeline.incomplete")}` : labelOf(m.place),
            tip: m.incomplete ? t("timeline.incompleteHint") : undefined,
          }))}
          label={t("source.label")}
          select="one"
          active={mark.place}
          onChange={setChosen}
          inline
        />
      )}
      {/* Keyed on the place, so switching it resets what the panel holds for
          the other one, a typed folder or ticked files. */}
      <div key={mark.place} className="flex flex-wrap items-center gap-2">
        {actions}
      </div>
      {deleting && (
        <TimelineDeleteDialog
          key={deleting.join(",")}
          domain={timeline.domain}
          itemKey={entry.item.key}
          row={row}
          places={deleting}
          onDone={() => {
            const everywhere = deleting.length === 0;
            setDeleting(null);
            onClose();
            reload();
            // The cards count the copies at targets, so they read them again.
            if (everywhere || deleting.some((p) => p !== "local")) placementChanged();
          }}
          onClose={() => setDeleting(null)}
        />
      )}
      {confirmDialog}
    </AnomalyWindow>
  );
}
