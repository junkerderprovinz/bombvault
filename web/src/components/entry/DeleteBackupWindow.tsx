import { useState } from "react";
import type { TimelineRow } from "../../lib/api";
import { rowTime, type Entry } from "../../lib/entry";
import { useT } from "../../lib/i18n";
import { formatList } from "../../lib/placement";
import { placementChanged } from "../../lib/placementEvents";
import { formatRecent } from "../../lib/reltime";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { Button } from "../Button";
import { IconTrash } from "../navGlyphs";
import { TimelineDeleteDialog } from "../timeline/TimelineDeleteDialog";
import { usePlaceLabel, type TimelineState } from "./BackupPlaces";
import { Choice, Choices } from "./Choice";
import { Note } from "./Note";

const EVERYWHERE = "";

/**
 * DeleteBackupWindow deletes one backup. A backup that lies at several places
 * is first asked where it should go. What the delete takes and leaves is then
 * read live and asked once more.
 */
export function DeleteBackupWindow({
  entry,
  name,
  row,
  timeline,
  onClose,
}: {
  entry: Entry;
  name: string;
  row: TimelineRow;
  timeline: TimelineState;
  onClose: () => void;
}) {
  const { t, lang } = useT();
  const { places, reload } = timeline;
  const labelOf = usePlaceLabel(places);
  const locked = (place: string) => places.find((p) => p.place === place)?.appendOnly === true;
  const free = row.places.filter((mark) => !locked(mark.place));
  const kept = row.places.filter((mark) => locked(mark.place));
  const [scope, setScope] = useState(EVERYWHERE);
  const [asked, setAsked] = useState(free.length < 2);

  if (asked) {
    const at = scope === EVERYWHERE ? [] : [scope];
    return (
      <TimelineDeleteDialog
        domain={timeline.domain}
        itemKey={entry.item.key}
        row={row}
        places={at}
        onDone={() => {
          onClose();
          reload();
          // The cards count the copies at targets, so they read them again.
          if (scope !== "local") placementChanged();
        }}
        onClose={onClose}
      />
    );
  }

  return (
    <InstanceWindow
      title={t("entry.delete.title").replace("{when}", formatRecent(rowTime(row), lang))}
      facts={name}
      onClose={onClose}
      footer={
        <>
          <Button label={t("common.cancel")} labelKey="common.cancel" tone="subtle" onClick={onClose} />
          <Button
            label={t("common.delete")}
            labelKey="common.delete"
            glyph={<IconTrash />}
            tone="accent"
            onClick={() => setAsked(true)}
          />
        </>
      }
    >
      <Choices label={t("common.delete")}>
        {free.map((mark) => (
          <Choice
            key={mark.place}
            on={scope === mark.place}
            title={t("entry.delete.only").replace("{place}", () => labelOf(mark.place))}
            onPick={() => setScope(mark.place)}
          />
        ))}
        <Choice
          on={scope === EVERYWHERE}
          title={t("timeline.deleteRow")}
          text={formatList(lang, free.map((mark) => labelOf(mark.place)))}
          onPick={() => setScope(EVERYWHERE)}
        />
      </Choices>
      {kept.map((mark) => (
        <Note key={mark.place}>{t("offsiteRemoval.appendOnly").replace("{target}", () => labelOf(mark.place))}</Note>
      ))}
    </InstanceWindow>
  );
}
