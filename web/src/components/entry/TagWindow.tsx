import { useState } from "react";
import { tagBackup, type TimelineRow } from "../../lib/api";
import { actionMark, isOwnTag, parseTags, personTags, rowTime, type Entry } from "../../lib/entry";
import { useT } from "../../lib/i18n";
import { placementErrorText } from "../../lib/placementCodes";
import { formatRecent } from "../../lib/reltime";
import { newestId, sourceOfPlace } from "../../lib/timeline";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { Button } from "../Button";
import { IconSave } from "../glyphs";
import type { TimelineState } from "./BackupPlaces";
import { Note } from "./Note";

/**
 * TagWindow marks one backup with a tag, changes the tag or takes it away.
 * The tag is written at the place a restore would take the backup from, and
 * the copies of that place carry it on.
 */
export function TagWindow({
  entry,
  name,
  row,
  timeline,
  onSaved,
  onClose,
}: {
  entry: Entry;
  name: string;
  row: TimelineRow;
  timeline: TimelineState;
  onSaved: () => void;
  onClose: () => void;
}) {
  const { t, lang } = useT();
  const mark = actionMark(row, timeline.places);
  const had = personTags(mark.tags);
  const [text, setText] = useState(had.join(", "));
  const [busy, setBusy] = useState(false);
  const [refusal, setRefusal] = useState<string | null>(null);

  async function save() {
    const wanted = parseTags(text);
    const add = wanted.filter((tag) => !had.includes(tag));
    const remove = had.filter((tag) => !wanted.includes(tag));
    if (add.length + remove.length === 0) {
      onClose();
      return;
    }
    setBusy(true);
    setRefusal(null);
    try {
      const res = await tagBackup(
        timeline.domain,
        entry.item.key,
        newestId(mark),
        add,
        remove,
        sourceOfPlace(mark.place)
      );
      if (res.ok) {
        onSaved();
        return;
      }
      const own = add.find(isOwnTag);
      setRefusal(
        own !== undefined
          ? t("entry.tag.reserved").replace("{tag}", () => own)
          : placementErrorText(t, lang, res, "common.saveFailed")
      );
    } catch (err) {
      setRefusal(err instanceof Error ? err.message : t("common.saveFailed"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <InstanceWindow
      title={t("entry.tag.title")}
      facts={`${name}, ${formatRecent(rowTime(row), lang)}`}
      onClose={onClose}
      footer={
        <>
          <Button label={t("common.cancel")} labelKey="common.cancel" tone="subtle" onClick={onClose} />
          <Button
            label={t("settings.save")}
            labelKey="settings.save"
            glyph={<IconSave />}
            tone="accent"
            disabled={busy}
            busy={busy}
            onClick={() => void save()}
          />
        </>
      }
    >
      <input
        type="text"
        value={text}
        autoFocus
        spellCheck={false}
        aria-label={t("entry.tag.title")}
        onChange={(e) => setText(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && !busy) void save();
        }}
        className="w-full rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text glim-field-focus"
      />
      {refusal !== null && (
        <div role="alert">
          <Note tone="fail">{refusal}</Note>
        </div>
      )}
    </InstanceWindow>
  );
}
