import { useRef, useState } from "react";
import {
  BACKUP_FILTERS,
  INSTALLED_FILTERS,
  SCHEDULE_FILTERS,
  isFiltered,
  type ListSort,
  type ListView,
} from "../../lib/backupList";
import type { TranslationKey } from "../../lib/i18n";
import { Button } from "../Button";
import { ChipFilter } from "../ChipFilter";
import { DropdownListbox } from "../DropdownListbox";
import { FilterPopover } from "../FilterPopover";
import { IconSort } from "../glyphs";
import { SearchFold } from "../SearchFold";

type T = (key: TranslationKey, n?: number) => string;

const SCHEDULE_LABEL: Record<ListView["schedule"], TranslationKey> = {
  all: "filter.all",
  scheduled: "filter.scheduled",
  paused: "backups.filter.paused",
};

const BACKUP_LABEL: Record<ListView["backup"], TranslationKey> = {
  all: "filter.all",
  backedUp: "filter.backedUp",
  never: "filter.neverBackedUp",
};

const INSTALLED_LABEL: Record<ListView["installed"], TranslationKey> = {
  all: "filter.all",
  installed: "containers.filterInstalled",
  notInstalled: "containers.notInstalled",
};

const SORTS: (ListSort & { label: TranslationKey })[] = [
  { field: "name", reversed: false, label: "backups.sort.nameAz" },
  { field: "name", reversed: true, label: "backups.sort.nameZa" },
  { field: "status", reversed: false, label: "backups.sort.problemsFirst" },
  { field: "status", reversed: true, label: "backups.sort.problemsLast" },
  { field: "lastBackup", reversed: false, label: "backups.sort.newestFirst" },
  { field: "lastBackup", reversed: true, label: "backups.sort.oldestFirst" },
  { field: "size", reversed: false, label: "backups.sort.largestFirst" },
  { field: "size", reversed: true, label: "backups.sort.smallestFirst" },
  { field: "schedule", reversed: false, label: "backups.sort.scheduledFirst" },
  { field: "schedule", reversed: true, label: "backups.sort.pausedFirst" },
];

function SortButton({ sort, onChange, t }: { sort: ListSort; onChange: (next: ListSort) => void; t: T }) {
  const [open, setOpen] = useState(false);
  const trigger = useRef<HTMLButtonElement>(null);
  const current = SORTS.find((s) => s.field === sort.field && s.reversed === sort.reversed) ?? SORTS[0];

  return (
    <>
      <Button
        ref={trigger}
        label={t(current.label)}
        labelKey={null}
        glyph={<IconSort />}
        tone="subtle"
        ariaExpanded={open}
        onClick={() => setOpen((was) => !was)}
        className="max-md:min-h-11"
      />
      <DropdownListbox open={open} onClose={() => setOpen(false)} triggerRef={trigger} label={t("backups.sort.label")}>
        {SORTS.map((s) => {
          const on = s === current;
          return (
            <button
              key={s.label}
              type="button"
              role="option"
              aria-selected={on}
              onClick={() => {
                onChange({ field: s.field, reversed: s.reversed });
                setOpen(false);
              }}
              className={`flex w-full items-center px-3 py-2 text-start text-sm transition-colors ${
                on ? "bg-carbon-surface3 text-carbon-text" : "text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
              }`}
            >
              {t(s.label)}
            </button>
          );
        })}
      </DropdownListbox>
    </>
  );
}

/**
 * ListBar stands above the list: what it leaves out, how it is ordered, and
 * the search, which waits as a button until it is wanted.
 */
export function ListBar({
  view,
  onView,
  query,
  onQuery,
  t,
}: {
  view: ListView;
  onView: (next: ListView) => void;
  query: string;
  onQuery: (next: string) => void;
  t: T;
}) {
  return (
    <div className="relative flex flex-wrap items-center justify-end gap-2">
      <FilterPopover label={t("filter.button")} active={isFiltered(view)} align="row-end">
        <ChipFilter
          label={t("filter.schedule")}
          options={SCHEDULE_FILTERS.map((key) => ({ key, label: t(SCHEDULE_LABEL[key]) }))}
          value={view.schedule}
          onChange={(schedule) => onView({ ...view, schedule })}
        />
        <ChipFilter
          label={t("filter.backup")}
          options={BACKUP_FILTERS.map((key) => ({ key, label: t(BACKUP_LABEL[key]) }))}
          value={view.backup}
          onChange={(backup) => onView({ ...view, backup })}
        />
        <ChipFilter
          label={t("backups.filter.installed")}
          options={INSTALLED_FILTERS.map((key) => ({ key, label: t(INSTALLED_LABEL[key]) }))}
          value={view.installed}
          onChange={(installed) => onView({ ...view, installed })}
        />
      </FilterPopover>
      <SortButton sort={view.sort} onChange={(sort) => onView({ ...view, sort })} t={t} />
      <SearchFold
        value={query}
        onChange={onQuery}
        label={t("backups.search")}
        placeholder={t("backups.searchPlaceholder")}
      />
    </div>
  );
}
