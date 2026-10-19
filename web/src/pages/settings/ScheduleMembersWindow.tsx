import { useState } from "react";
import type { BackupItem } from "../../lib/api";
import { ENTRY_KIND, type EntryKind } from "../../lib/backupEntry";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { Button } from "../../components/Button";
import { KIND_LABEL, KIND_SCHEDULE_LABEL, entryName } from "../../components/backups/kinds";
import { useScheduleSentence } from "../../components/EffectiveScheduleLine";
import { IconCheck } from "../../components/navGlyphs";
import { Rows } from "../../components/storage/rows";
import { Toggle } from "../../components/Toggle";
import { InstanceWindow } from "../instances/InstanceWindow";

/**
 * ScheduleMembersWindow switches the entries of one section in and out of the
 * schedule. Each row says what happens to the entry as things stand, so the
 * effect of a switch shows at once.
 */
export function ScheduleMembersWindow({
  kind,
  items,
  onChanged,
  onClose,
}: {
  kind: EntryKind;
  /** The entries of this kind that a schedule can reach. */
  items: BackupItem[];
  /** A switch was stored; the list should be fetched again. */
  onChanged: () => void;
  onClose: () => void;
}) {
  const { t } = useT();
  const { push } = useToast();
  const sentence = useScheduleSentence();
  const [busy, setBusy] = useState<Record<string, boolean>>({});
  const setIncluded = ENTRY_KIND[kind].setIncluded;

  async function flip(item: BackupItem) {
    if (!setIncluded) return;
    setBusy((b) => ({ ...b, [item.key]: true }));
    try {
      const res = await setIncluded(item.key, !item.included);
      if (res.ok) onChanged();
      else push(res.error ?? t("schedule.updateFailed"), "fail");
    } catch (err) {
      push(err instanceof Error ? err.message : t("schedule.updateFailed"), "fail");
    } finally {
      setBusy((b) => ({ ...b, [item.key]: false }));
    }
  }

  return (
    <InstanceWindow
      title={t("schedules.membersTitle").replace("{area}", t(KIND_LABEL[kind]))}
      facts={t("schedules.membersHint")}
      onClose={onClose}
      footer={<Button label={t("common.done")} labelKey="common.done" glyph={<IconCheck />} tone="accent" onClick={onClose} />}
    >
      <Rows as="ul">
        {items.map((item) => {
          const name = entryName(item, t);
          const said = sentence(item.effectiveSchedule, KIND_SCHEDULE_LABEL[kind]);
          return (
            <li key={item.key} className="flex items-center justify-between gap-4 py-3 first:pt-0 last:pb-0">
              <span className="flex min-w-0 flex-col gap-0.5">
                <span className="text-sm font-medium text-carbon-text wrap-anywhere">{name}</span>
                {said && <span className={`text-xs ${said.tone}`}>{said.text}</span>}
              </span>
              <Toggle
                hideLabel
                label={`${t("entry.settings.inSchedule")}: ${name}`}
                checked={item.included}
                disabled={!!busy[item.key]}
                onChange={() => void flip(item)}
              />
            </li>
          );
        })}
      </Rows>
    </InstanceWindow>
  );
}
