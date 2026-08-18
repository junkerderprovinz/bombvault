// ---------------------------------------------------------------------------
// ActivityLog — the dashboard "activity log": one flat, scrollable,
// docker-logs-style list of timestamped lines. NO zones: history, live
// progress and the next scheduled run are one merged, filterable list (see
// web/src/lib/activityLog.ts for the pure merge/dedupe/order logic this
// component just fetches data for and renders).
//
// Mounted on Dashboard.tsx as its own customizable block, directly below the
// summary tier — self-contained (its own card chrome + heading), so it drops
// in as `<ActivityLog />` with no further changes.
// ---------------------------------------------------------------------------

import { useEffect, useMemo, useRef, useState } from "react";
import { listRuns, getScheduleNext } from "../lib/api";
import type { Run, ScheduleNext } from "../lib/api";
import { useProgress } from "../lib/progress";
import { useT } from "../lib/i18n";
import type { TranslationKey } from "../lib/i18n";
import { buildLogLines, filterLogLines, formatLogDate } from "../lib/activityLog";
import type { LogFilterDomain, LogFilterKind, LogStatus, ResolveName } from "../lib/activityLog";
import { formatClockTime } from "../lib/reltime";

const POLL_RUNS_MS = 10000;
const POLL_SCHEDULE_MS = 30000;
// The idle line's countdown ("in 2h 14m") only needs to visibly tick at
// minute granularity — no point re-rendering more often just for that.
const TICK_MS = 60000;
// How close to the bottom (px) still counts as "at the bottom" for
// auto-follow — a few pixels of rounding slack, not a hard 0.
const BOTTOM_THRESHOLD_PX = 24;

function glyphFor(status: LogStatus): string {
  switch (status) {
    case "running":
      return "⋯";
    case "success":
      return "✓";
    case "failed":
      return "✗";
    case "offsite":
      return "↗";
    case "info":
      return "▶";
  }
}

// Reuses the exact hex values Dashboard's StatusChip uses for success/failed/
// running so a log line reads as the same colour language as the rest of the
// app (#66-style shared vocabulary), not a new palette.
function colorFor(status: LogStatus): string {
  switch (status) {
    case "success":
      return "text-statusOk";
    case "failed":
      return "text-statusFail";
    case "running":
    case "offsite":
      return "text-statusInfo";
    case "info":
      return "text-statusWarn";
  }
}

function glyphLabelKey(status: LogStatus): TranslationKey {
  switch (status) {
    case "running":
      return "activityLog.glyphRunning";
    case "success":
      return "activityLog.glyphSuccess";
    case "failed":
      return "activityLog.glyphFailed";
    case "offsite":
      return "activityLog.glyphOffsite";
    case "info":
      return "activityLog.glyphInfo";
  }
}

export function ActivityLog({
  dayFilter = null,
  onClearDayFilter,
}: {
  /** Externally-controlled day filter (ISO YYYY-MM-DD, local calendar day) —
   *  the Dashboard sets it when a heatmap cell is clicked; null = off. Shown
   *  as a filled chip next to the filter bar and combined with the local
   *  text/domain/type filters. */
  dayFilter?: string | null;
  /** Invoked by the chip's × — the owner (Dashboard) clears its state. */
  onClearDayFilter?: () => void;
} = {}) {
  const { t } = useT();
  const [runs, setRuns] = useState<Run[]>([]);
  const [scheduleNext, setScheduleNext] = useState<ScheduleNext[]>([]);
  const [now, setNow] = useState<number>(() => Date.now());
  const progressMap = useProgress();

  const [filterText, setFilterText] = useState("");
  const [filterDomain, setFilterDomain] = useState<LogFilterDomain>("all");
  const [filterType, setFilterType] = useState<LogFilterKind>("all");

  const scrollRef = useRef<HTMLDivElement>(null);
  const [autoFollow, setAutoFollow] = useState(true);

  // Finished runs — polled; the live tail comes from useProgress()'s SSE push.
  useEffect(() => {
    let alive = true;
    const load = () => {
      listRuns()
        .then((res) => {
          if (alive && res.ok) setRuns(res.runs ?? []);
        })
        .catch(() => {
          /* non-fatal — keep showing the last known runs */
        });
    };
    load();
    const id = setInterval(load, POLL_RUNS_MS);
    return () => {
      alive = false;
      clearInterval(id);
    };
  }, []);

  // Next scheduled fire — only needed for the trailing idle line.
  useEffect(() => {
    let alive = true;
    const load = () => {
      getScheduleNext()
        .then((next) => {
          if (alive) setScheduleNext(next);
        })
        .catch(() => {
          /* non-fatal */
        });
    };
    load();
    const id = setInterval(load, POLL_SCHEDULE_MS);
    return () => {
      alive = false;
      clearInterval(id);
    };
  }, []);

  // Advance `now` periodically so the idle line's countdown keeps ticking
  // even when nothing else (a run, an SSE event) triggers a re-render.
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), TICK_MS);
    return () => clearInterval(id);
  }, []);

  // Resolves a translation key (+ optional {placeholder} params) — the only
  // i18n dependency buildLogLines takes, so its merge/dedupe/order logic
  // stays pure and testable without a live I18nProvider.
  const resolveName: ResolveName = (key, params) => {
    let s = t(key as TranslationKey);
    if (params) {
      for (const [name, value] of Object.entries(params)) s = s.split(`{${name}}`).join(value);
    }
    return s;
  };

  const lines = useMemo(
    () => buildLogLines(runs, progressMap, scheduleNext, resolveName, now),
    [runs, progressMap, scheduleNext, now, t]
  );

  // Date locale is deliberately OMITTED (undefined): formatLogDate and the
  // filter haystack then run the browser's default-locale negotiation — the
  // exact same one every other date in the app uses (formatTs), so the log
  // can never disagree with the rest of the UI again (#108).
  const filteredLines = useMemo(
    () =>
      filterLogLines(lines, {
        domain: filterDomain,
        kind: filterType,
        text: filterText,
        day: dayFilter ?? undefined,
      }),
    [lines, filterDomain, filterType, filterText, dayFilter]
  );

  // Auto-follow tail: while pinned to the bottom, stay pinned as new lines
  // arrive. The moment the user scrolls up (handleScroll below), stop —
  // "jump to latest" returns to the tail.
  useEffect(() => {
    if (!autoFollow) return;
    const el = scrollRef.current;
    if (!el) return;
    el.scrollTop = el.scrollHeight;
  }, [filteredLines, autoFollow]);

  const handleScroll = () => {
    const el = scrollRef.current;
    if (!el) return;
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight <= BOTTOM_THRESHOLD_PX;
    setAutoFollow(atBottom);
  };

  const jumpToLatest = () => {
    const el = scrollRef.current;
    if (el) el.scrollTop = el.scrollHeight;
    setAutoFollow(true);
  };

  return (
    <div className="bg-carbon-surface rounded-card p-5 flex flex-col gap-3">
      <h2 className="text-sm font-semibold text-carbon-textSub uppercase tracking-widest">
        {t("activityLog.title")}
      </h2>

      {/* Filter bar — narrows the ONE list below; never a second zone. */}
      <div className="flex flex-wrap items-center gap-2">
        <input
          type="text"
          value={filterText}
          onChange={(e) => setFilterText(e.target.value)}
          placeholder={t("activityLog.filterPlaceholder")}
          aria-label={t("activityLog.filterPlaceholder")}
          className="flex-1 min-w-[10rem] rounded-control bg-carbon-surface2 px-2 py-1 text-xs text-carbon-text placeholder:text-carbon-textMuted focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid"
        />
        <select
          value={filterDomain}
          onChange={(e) => setFilterDomain(e.target.value as LogFilterDomain)}
          className="rounded-control bg-carbon-surface2 px-2 py-1 text-xs text-carbon-text focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid"
        >
          <option value="all">{t("activityLog.filterAllDomains")}</option>
          <option value="containers">{t("activityLog.domainContainers")}</option>
          <option value="vms">{t("activityLog.domainVMs")}</option>
          <option value="flash">{t("activityLog.domainFlash")}</option>
          <option value="config">{t("activityLog.domainConfig")}</option>
          <option value="files">{t("activityLog.domainFiles")}</option>
        </select>
        <select
          value={filterType}
          onChange={(e) => setFilterType(e.target.value as LogFilterKind)}
          className="rounded-control bg-carbon-surface2 px-2 py-1 text-xs text-carbon-text focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid"
        >
          <option value="all">{t("activityLog.filterAllTypes")}</option>
          <option value="backup">{t("activityLog.typeBackup")}</option>
          <option value="restore">{t("activityLog.typeRestore")}</option>
          <option value="prune">{t("activityLog.typePrune")}</option>
          <option value="verify">{t("activityLog.typeVerify")}</option>
          <option value="offsite">{t("activityLog.typeOffsite")}</option>
          {/* Persisted kinds since the everything-in-the-log wave. Drill/tamper
              reuse the existing job-label keys; the off-site DR check ("drdrill")
              is its own kind and reuses Run History's kind label. */}
          <option value="drill">{t("activityLog.jobDrill")}</option>
          <option value="drdrill">{t("run.kindDRDrill")}</option>
          <option value="tamper">{t("activityLog.jobTamper")}</option>
          <option value="export">{t("activityLog.typeExport")}</option>
        </select>
        {/* Heatmap day-filter chip — a filled pill (accent, no border, same
            language as the heatmap's active domain toggle) showing which day
            the Dashboard heatmap narrowed the log to; its × hands the clear
            back to the owner. The ISO day is parsed as LOCAL midnight so the
            label always names the same calendar day the cell was. */}
        {dayFilter && (
          <span className="inline-flex items-center gap-1 rounded-pill bg-accent text-accentContrast ps-2.5 pe-1 py-0.5 text-xs font-medium">
            {resolveName("activityLog.dayFilterChip", {
              date: new Date(dayFilter + "T00:00:00").toLocaleDateString(),
            })}
            <button
              type="button"
              onClick={onClearDayFilter}
              aria-label={t("activityLog.clearDayFilter")}
              title={t("activityLog.clearDayFilter")}
              className="cursor-pointer rounded-full px-1 leading-none hover:bg-black/10"
            >
              ×
            </button>
          </span>
        )}
      </div>

      {/* The log itself — a single scrollable, monospace, newest-at-bottom list. */}
      <div className="relative">
        <div
          ref={scrollRef}
          onScroll={handleScroll}
          className="max-h-96 overflow-y-auto rounded-card bg-black/20 font-mono text-xs leading-relaxed px-3 py-2 flex flex-col gap-0.5"
        >
          {filteredLines.map((l) => (
            <div key={l.id} className="flex items-start gap-2">
              <span className="text-carbon-textMuted shrink-0 tabular-nums">
                {formatLogDate(l.atMs)} {formatClockTime(l.atMs / 1000, true)}
              </span>
              <span className={`shrink-0 w-4 text-center ${colorFor(l.status)}`} aria-label={t(glyphLabelKey(l.status))}>
                {glyphFor(l.status)}
              </span>
              <span className={`flex-1 min-w-0 wrap-break-word ${colorFor(l.status)}`}>{l.text}</span>
            </div>
          ))}
        </div>
        {!autoFollow && (
          <button
            type="button"
            onClick={jumpToLatest}
            className="absolute bottom-3 right-3 rounded-pill bg-carbon-surface2 px-3 py-1 text-xs text-carbon-text shadow-lg hover:bg-carbon-hover"
          >
            ↓ {t("activityLog.jumpToLatest")}
          </button>
        )}
      </div>
    </div>
  );
}
