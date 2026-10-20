import { useEffect, type MouseEvent } from "react";
import type { Run, TimelineRow } from "../../lib/api";
import { actionMark, comparable, personTags, rowTags, rowTime, runOfRow } from "../../lib/entry";
import { humanBytes } from "../../lib/forecast";
import { useT } from "../../lib/i18n";
import { formatRecent } from "../../lib/reltime";
import type { MenuAction } from "../backups/ActionMenu";
import { MoreMenu } from "../backups/MoreMenu";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { IconCompare } from "../glyphs";
import { InfoBubble } from "../InfoBubble";
import { IconAdd, IconCloud, IconDownload, IconLocal, IconRestore, IconTrash } from "../navGlyphs";
import { Rows } from "../storage/rows";
import { usePlaceLabel, type TimelineState } from "./BackupPlaces";

const WEEK_MS = 7 * 86_400_000;

/** Which backup a click asks for, and from which place when it was a click
 *  on that place's mark. */
export interface OpenBackup {
  key: string;
  place?: string;
}

/** What the menu of a backup can do with it. */
export type RowAction = "compare" | "tag" | "export" | "delete";

export function rowElementId(key: string): string {
  return `entry-backup-${key}`;
}

function RowMenu({
  row,
  timeline,
  onAct,
}: {
  row: TimelineRow;
  timeline: TimelineState;
  onAct: (action: RowAction) => void;
}) {
  const { t, lang } = useT();
  const { rows, places } = timeline;
  const mark = actionMark(row, places);
  const deletable = row.places.some((m) => places.find((p) => p.place === m.place)?.appendOnly === false);

  const actions: MenuAction[] = [];
  if (comparable(rows, mark.place).length > 1) {
    actions.push({ id: "compare", label: t("entry.row.compare"), glyph: <IconCompare />, onSelect: () => onAct("compare") });
  }
  actions.push(
    {
      id: "tag",
      label: t(personTags(mark.tags).length > 0 ? "entry.row.tagChange" : "snapshot.addTag"),
      glyph: <IconAdd />,
      onSelect: () => onAct("tag"),
    },
    { id: "export", label: t("entry.export.action"), glyph: <IconDownload />, onSelect: () => onAct("export") }
  );
  if (deletable) {
    actions.push({ id: "delete", label: t("common.delete"), glyph: <IconTrash />, onSelect: () => onAct("delete") });
  }

  return <MoreMenu label={formatRecent(rowTime(row), lang)} actions={actions} />;
}

function Row({
  row,
  timeline,
  run,
  flagged,
  marked,
  onOpen,
  onAct,
}: {
  row: TimelineRow;
  timeline: TimelineState;
  run: Run | undefined;
  flagged: boolean;
  /** The row a bar of the history or a restore link points at. */
  marked: boolean;
  onOpen: (open: OpenBackup) => void;
  onAct: (row: TimelineRow, action: RowAction) => void;
}) {
  const { t, lang } = useT();
  const { places } = timeline;
  const labelOf = usePlaceLabel(places, timeline.domain);
  const orderOf = (place: string) => places.findIndex((p) => p.place === place);
  const marks = [...row.places].sort((a, b) => orderOf(a.place) - orderOf(b.place));
  const allRead = places.every((p) => p.state === "read");
  const size = row.size ?? run?.sourceBytes;

  function openAt(e: MouseEvent, place: string) {
    e.stopPropagation();
    onOpen({ key: row.key, place });
  }

  return (
    <li
      id={rowElementId(row.key)}
      data-testid="entry-backup"
      onClick={() => onOpen({ key: row.key })}
      className={`group flex cursor-pointer flex-wrap items-center gap-x-4 gap-y-1.5 rounded-control px-3 py-2.5 ${
        marked ? "bg-accentSoft" : "hover:bg-carbon-hover"
      }`}
    >
      <span className="flex min-w-0 flex-1 basis-56 flex-wrap items-center gap-2">
        <span className="text-sm font-semibold tabular-nums text-carbon-text first-letter:uppercase">
          {formatRecent(rowTime(row), lang)}
        </span>
        {flagged && (
          <Badge tone="warn" size="small">
            {t("anomaly.snapshotFlagged")}
          </Badge>
        )}
        {rowTags(row).map((tag) => (
          <Badge key={tag} tone="neutral" size="small">
            {tag}
          </Badge>
        ))}
        {allRead && places.length > 1 && row.places.length === 1 && (
          <Badge tone="neutral" size="small">
            {t("timeline.onlyHere")}
          </Badge>
        )}
      </span>
      <span className="flex flex-wrap items-center gap-1.5">
        {marks.map((mark) => (
          <span key={mark.place} className="inline-flex items-center gap-1">
            <button
              type="button"
              aria-label={t("entry.list.restoreFrom").replace("{place}", () => labelOf(mark.place))}
              onClick={(e) => openAt(e, mark.place)}
              className="inline-flex items-center gap-1.5 rounded-control bg-carbon-surface2 px-2 py-1 text-xs text-carbon-textSub hover:bg-carbon-surface3 hover:text-carbon-text [&_svg]:h-3 [&_svg]:w-3"
            >
              {places.find((p) => p.place === mark.place)?.remote ? <IconCloud /> : <IconLocal />}
              {labelOf(mark.place)}
              {mark.incomplete && ` · ${t("timeline.incomplete")}`}
            </button>
            {mark.incomplete && (
              <span onClick={(e) => e.stopPropagation()} className="flex">
                <InfoBubble tip={t("timeline.incompleteHint")} />
              </span>
            )}
          </span>
        ))}
      </span>
      <span data-testid="entry-backup-size" className="w-24 text-end text-xs tabular-nums text-carbon-textMuted max-md:w-auto">
        {size != null && humanBytes(size)}
      </span>
      {/* The menu is drawn outside the row, and a press in it would otherwise
          reach the row and open the backup. With a mouse the buttons come in
          with the pointer or the focus, and stay while their menu is open. */}
      <span
        data-testid="entry-backup-actions"
        className="flex items-center gap-1.5 pointer-fine:opacity-0 pointer-fine:group-hover:opacity-100 pointer-fine:group-focus-within:opacity-100 pointer-fine:has-[[aria-expanded=true]]:opacity-100"
        onClick={(e) => e.stopPropagation()}
      >
        <RowMenu row={row} timeline={timeline} onAct={(action) => onAct(row, action)} />
        {/* A phone opens the backup with a tap on its row. */}
        <span className="max-md:hidden">
          <Button
            label={t("snapshots.restore")}
            labelKey="snapshots.restore"
            glyph={<IconRestore />}
            onClick={() => onOpen({ key: row.key })}
          />
        </span>
      </span>
    </li>
  );
}

function Group({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-1.5">
      <h3 className="px-1 text-xs font-medium uppercase tracking-wider text-carbon-textMuted">{label}</h3>
      <Rows as="ul">{children}</Rows>
    </div>
  );
}

/**
 * BackupList is every backup of the entry, the newest first and split at one
 * week. A row opens its backup. The mark of a place opens it at that place,
 * and the menu holds what else can be done with it.
 */
export function BackupList({
  timeline,
  runs,
  isFlagged,
  located,
  requested,
  onOpen,
  onAct,
}: {
  timeline: TimelineState;
  runs: Run[];
  /** Whether an open finding was raised on the backup. */
  isFlagged: (row: TimelineRow, run: Run | undefined) => boolean;
  /** The row a bar of the history asked for. It comes into view. */
  located: string | null;
  /** The backup a restore link asked for. */
  requested: string;
  onOpen: (open: OpenBackup) => void;
  onAct: (row: TimelineRow, action: RowAction) => void;
}) {
  const { t } = useT();
  const { rows, places, reading, loadPlace } = timeline;

  useEffect(() => {
    // jsdom has no scrollIntoView.
    if (located) document.getElementById(rowElementId(located))?.scrollIntoView?.({ block: "center", behavior: "smooth" });
  }, [located]);

  const weekAgo = Date.now() - WEEK_MS;
  const recent = rows.filter((row) => Date.parse(row.time) >= weekAgo);
  const older = rows.filter((row) => Date.parse(row.time) < weekAgo);
  const unchecked = places.filter((p) => p.state === "unchecked");
  const line = (row: TimelineRow) => {
    const run = runOfRow(runs, row);
    return (
      <Row
        key={row.key}
        row={row}
        timeline={timeline}
        run={run}
        flagged={isFlagged(row, run)}
        marked={row.key === located || row.key === requested}
        onOpen={onOpen}
        onAct={onAct}
      />
    );
  };

  return (
    <div data-new="lists-as-rows" className="flex flex-col gap-4">
      {recent.length > 0 && <Group label={t("entry.list.thisWeek")}>{recent.map(line)}</Group>}
      {older.length > 0 && <Group label={t("entry.list.older")}>{older.map(line)}</Group>}
      {unchecked.length > 0 && (
        <div className="flex justify-end">
          <Button
            label={t("timeline.showOlder")}
            labelKey="timeline.showOlder"
            disabled={unchecked.some((p) => reading.has(p.place))}
            onClick={() => unchecked.forEach((p) => void loadPlace(p.place))}
          />
        </div>
      )}
    </div>
  );
}
