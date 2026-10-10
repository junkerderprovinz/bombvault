import type { Run } from "../../lib/api";
import { HISTORY_DAYS, barHeight, type HistoryDay } from "../../lib/entry";
import { humanBytes } from "../../lib/forecast";
import { useT } from "../../lib/i18n";
import { useTipBubble } from "../../lib/useTipBubble";
import { Card } from "../../pages/settings/shared";

const BAR = "w-full rounded-t-xs";
const COLUMNS = "grid grid-cols-[repeat(14,minmax(0,1fr))] gap-1.5 max-md:gap-0.5";

function dayLabel(day: HistoryDay, lang: string): string {
  return new Date(day.start).toLocaleDateString(lang, { day: "2-digit", month: "2-digit" });
}

function Day({
  day,
  largest,
  lang,
  onLocate,
}: {
  day: HistoryDay;
  largest: number;
  lang: string;
  onLocate?: (run: Run) => void;
}) {
  const { t } = useT();
  const date = dayLabel(day, lang);
  const empty = day.runs.length === 0;
  const label = empty
    ? t("entry.history.dayNone").replace("{day}", date)
    : t("entry.history.dayNew").replace("{day}", date).replace("{size}", humanBytes(day.bytes));
  const tip = useTipBubble(label);

  const tone = day.flagged ? "bg-statusWarnSolid" : day.recommended ? "bg-accent" : "bg-accent opacity-50";
  const bar = empty ? (
    <i className="h-0.5 w-full rounded-pill bg-carbon-surface3" />
  ) : (
    <i data-testid="history-bar" className={`${BAR} ${tone}`} style={{ height: `${barHeight(day.bytes, largest)}%` }} />
  );

  if (empty || !onLocate) {
    return (
      <span
        ref={tip.ref}
        role="img"
        aria-label={label}
        onMouseEnter={tip.handlers.onMouseEnter}
        onMouseLeave={tip.handlers.onMouseLeave}
        className="flex h-full items-end"
      >
        {bar}
        {tip.bubble}
      </span>
    );
  }
  return (
    <>
      <button
        ref={tip.ref}
        type="button"
        aria-label={label}
        aria-describedby={tip.describedBy}
        {...tip.handlers}
        onClick={() => onLocate(day.runs[0])}
        className="flex h-full items-end rounded-control hover:bg-carbon-hover"
      >
        {bar}
      </button>
      {tip.bubble}
    </>
  );
}

function Swatch({ className }: { className: string }) {
  return <i aria-hidden="true" className={`inline-block h-2 w-2 rounded-xs ${className}`} />;
}

/** Whether the two weeks hold a backup at all. Without one the card stays away. */
export function hasHistory(days: HistoryDay[]): boolean {
  return days.some((day) => day.runs.length > 0);
}

/**
 * BackupHistory draws the last two weeks as one bar per day, as tall as the
 * new data that day's backups wrote. A day without a backup shows as a gap.
 * With onLocate a bar leads to its backup in the list below.
 */
export function BackupHistory({
  days,
  onLocate,
  hueIndex,
}: {
  days: HistoryDay[];
  onLocate?: (run: Run) => void;
  hueIndex: number;
}) {
  const { t, lang } = useT();
  const largest = Math.max(...days.map((day) => day.bytes));
  const today = new Intl.RelativeTimeFormat(lang, { numeric: "auto" }).format(0, "day");

  return (
    <Card title={t("entry.history.title")} hint={t("entry.history.hint")} hueIndex={hueIndex}>
      <div className="flex flex-col gap-2 rounded-control bg-carbon-surface2 px-3 pb-2 pt-3">
        <div role="group" aria-label={t("entry.history.label")} className={`${COLUMNS} h-24 items-end`}>
          {days.map((day) => (
            <Day key={day.start} day={day} largest={largest} lang={lang} onLocate={onLocate} />
          ))}
        </div>
        <div aria-hidden="true" className={`${COLUMNS} text-center text-caption text-carbon-textMuted`}>
          {days.map((day, i) => {
            const last = i === HISTORY_DAYS - 1;
            // A phone has room for every fourth date and no weekdays.
            const onPhone = (HISTORY_DAYS - 1 - i) % 4 === 0;
            return (
              <span key={day.start} className={`flex flex-col leading-tight ${onPhone ? "" : "max-md:invisible"}`}>
                <b className="font-semibold text-carbon-textSub capitalize max-md:hidden">
                  {last ? today : new Date(day.start).toLocaleDateString(lang, { weekday: "short" })}
                </b>
                <span className="tabular-nums max-md:whitespace-nowrap">{dayLabel(day, lang)}</span>
              </span>
            );
          })}
        </div>
      </div>
      <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-carbon-textMuted">
        <span className="inline-flex items-center gap-1.5">
          <Swatch className="bg-accent opacity-50" />
          {t("entry.history.new")}
        </span>
        <span className="inline-flex items-center gap-1.5">
          <Swatch className="bg-accent" />
          {t("entry.history.recommended")}
        </span>
        {days.some((day) => day.flagged) && (
          <span className="inline-flex items-center gap-1.5">
            <Swatch className="bg-statusWarnSolid" />
            {t("anomaly.snapshotFlagged")}
          </span>
        )}
        <span className="inline-flex items-center gap-1.5">
          <i aria-hidden="true" className="inline-block h-0.5 w-2 rounded-pill bg-carbon-surface3" />
          {t("entry.history.none")}
        </span>
      </div>
    </Card>
  );
}
