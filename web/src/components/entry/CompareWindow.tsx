import { useState } from "react";
import { diffBackups, type BackupChange, type BackupDiff, type TimelineRow } from "../../lib/api";
import { actionMark, comparable, diffGroups, rowTime, type Entry } from "../../lib/entry";
import { humanBytes } from "../../lib/forecast";
import { useT, type TranslationKey } from "../../lib/i18n";
import { formatList } from "../../lib/placement";
import { placementErrorText } from "../../lib/placementCodes";
import { formatRecent } from "../../lib/reltime";
import { newestId, sourceOfPlace } from "../../lib/timeline";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { Button } from "../Button";
import { IconCompare } from "../glyphs";
import { SelectField } from "../SelectField";
import { Rows, SettingRow } from "../storage/rows";
import type { TimelineState } from "./BackupPlaces";
import { Note } from "./Note";

const SELECT = "rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text glim-field-focus text-start";

function Paths({ title, changes }: { title: TranslationKey; changes: BackupChange[] }) {
  const { t } = useT();
  if (changes.length === 0) return null;
  return (
    <div className="flex flex-col gap-1.5">
      <h3 className="px-1 text-xs font-medium uppercase tracking-wider text-carbon-textMuted">
        {t(title)} · {changes.length}
      </h3>
      <Rows as="ul">
        {changes.map((change) => (
          <li
            key={`${change.part ?? ""}:${change.path}`}
            dir="ltr"
            className="py-1.5 font-mono text-xs text-carbon-text text-start wrap-anywhere first:pt-0 last:pb-0"
          >
            {change.part && <span className="text-carbon-textMuted">{change.part} · </span>}
            {change.path}
          </li>
        ))}
      </Rows>
    </div>
  );
}

function Result({ diff }: { diff: BackupDiff }) {
  const { t, lang } = useT();
  const groups = diffGroups(diff.changes);
  const same = diff.changes.length === 0 && diff.addedBytes === 0 && diff.removedBytes === 0;
  const net = diff.addedBytes - diff.removedBytes;
  const size = t(net < 0 ? "entry.compare.less" : "entry.compare.more").replace("{size}", humanBytes(Math.abs(net)));
  const counts = [
    t("entry.compare.changed", diff.changedFiles),
    t("entry.compare.added", diff.addedFiles),
    t("entry.compare.removed", diff.removedFiles),
  ].join(", ");

  return (
    <div data-testid="compare-result" className="flex flex-col gap-3">
      {same ? (
        <Note>{t("entry.compare.none")}</Note>
      ) : diff.disks ? (
        <Note>
          {t("entry.compare.diskBytes").replace("{size}", humanBytes(diff.addedBytes))} {t("entry.compare.guest")}
        </Note>
      ) : (
        <Note>
          {counts} · {size}
        </Note>
      )}
      {(diff.onlyBefore ?? []).length > 0 && (
        <Note>{t("entry.compare.onlyBefore").replace("{list}", () => formatList(lang, diff.onlyBefore ?? []))}</Note>
      )}
      {(diff.onlyAfter ?? []).length > 0 && (
        <Note>{t("entry.compare.onlyAfter").replace("{list}", () => formatList(lang, diff.onlyAfter ?? []))}</Note>
      )}
      <Paths title="snapshot.added" changes={groups.added} />
      <Paths title="snapshot.removed" changes={groups.removed} />
      <Paths title="snapshot.changed" changes={groups.changed} />
      {diff.truncated && <p className="text-xs text-carbon-textMuted">{t("entry.compare.truncated")}</p>}
    </div>
  );
}

/**
 * CompareWindow says what changed from one backup of the entry to another:
 * the paths that came, went and changed, and how much the two differ in
 * size. Both backups are read at one place, so only the backups that place
 * holds are offered.
 */
export function CompareWindow({
  entry,
  name,
  row,
  timeline,
  onClose,
}: {
  entry: Entry;
  name: string;
  /** The backup the comparison was asked from. It is the later of the two. */
  row: TimelineRow;
  timeline: TimelineState;
  onClose: () => void;
}) {
  const { t, lang } = useT();
  const { place } = actionMark(row, timeline.places);
  const rows = comparable(timeline.rows, place);
  const at = rows.findIndex((r) => r.key === row.key);
  const [from, setFrom] = useState(rows[Math.min(at + 1, rows.length - 1)].key);
  const [to, setTo] = useState(row.key);
  const [busy, setBusy] = useState(false);
  const [diff, setDiff] = useState<BackupDiff | null>(null);
  const [problem, setProblem] = useState<string | null>(null);

  const options = rows.map((r) => ({ value: r.key, label: formatRecent(rowTime(r), lang) }));
  const idOf = (key: string) => {
    const mark = rows.find((r) => r.key === key)?.places.find((m) => m.place === place);
    return mark ? newestId(mark) : "";
  };

  function choose(set: (key: string) => void, key: string) {
    set(key);
    setDiff(null);
    setProblem(null);
  }

  async function compare() {
    setDiff(null);
    if (from === to) {
      setProblem(t("entry.compare.pickTwo"));
      return;
    }
    setBusy(true);
    setProblem(null);
    try {
      const res = await diffBackups(timeline.domain, entry.item.key, idOf(from), idOf(to), sourceOfPlace(place));
      if (res.ok && res.diff) setDiff(res.diff);
      else setProblem(placementErrorText(t, lang, res, "common.compareFailed"));
    } catch (err) {
      setProblem(err instanceof Error ? err.message : t("common.compareFailed"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <InstanceWindow
      title={t("entry.compare.title")}
      facts={name}
      onClose={onClose}
      footer={
        <>
          <Button label={t("common.close")} labelKey="common.close" tone="subtle" onClick={onClose} />
          <Button
            label={t("snapshot.compare")}
            labelKey="snapshot.compare"
            glyph={<IconCompare />}
            tone="accent"
            disabled={busy}
            busy={busy}
            onClick={() => void compare()}
          />
        </>
      }
    >
      <Rows>
        <SettingRow label={t("entry.compare.from")}>
          <SelectField
            value={from}
            onChange={(key) => choose(setFrom, key)}
            options={options}
            label={t("entry.compare.from")}
            disabled={busy}
            className={SELECT}
          />
        </SettingRow>
        <SettingRow label={t("entry.compare.to")}>
          <SelectField
            value={to}
            onChange={(key) => choose(setTo, key)}
            options={options}
            label={t("entry.compare.to")}
            disabled={busy}
            className={SELECT}
          />
        </SettingRow>
      </Rows>
      {problem !== null && (
        <div role="alert">
          <Note tone={from === to ? "neutral" : "fail"}>{problem}</Note>
        </div>
      )}
      {diff && <Result diff={diff} />}
    </InstanceWindow>
  );
}
