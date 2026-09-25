// The dashboard activity log: one scrollable, docker-logs-style list of
// timestamped lines. Finished runs, live progress and the next scheduled run
// share the list; lib/activityLog.ts merges, dedupes and orders them.

import { useEffect, useMemo, useRef, useState, type CSSProperties } from "react";
import { hueVars } from "../lib/appearance";
import { listRuns, getScheduleNext } from "../lib/api";
import type { Run, ScheduleNext } from "../lib/api";
import { useProgress } from "../lib/progress";
import { useOpenAnomalies } from "../lib/useAnomalies";
import { RunAnomalyBadge } from "./RunAnomalyBadge";
import { useT } from "../lib/i18n";
import { SelectField } from "./SelectField";
import type { TranslationKey } from "../lib/i18n";
import { buildLogLines, domainLabel, filterLogLines, formatLogDate, LOG_FILTER_DOMAINS, LOG_FILTER_KINDS } from "../lib/activityLog";
import type { LogFilterDomain, LogFilterKind, LogStatus, ResolveName } from "../lib/activityLog";
import { Badge } from "./Badge";
import { formatClockTime } from "../lib/reltime";
import { Button } from "./Button";

const POLL_RUNS_MS = 10000;
const POLL_SCHEDULE_MS = 30000;
// The idle countdown ("in 2h 14m") only shows minutes.
const TICK_MS = 60000;
// Same rate as OffsiteIndicator's elapsed time, so both tick together.
const LIVE_TICK_MS = 1000;
// Rounding slack for "scrolled to the bottom".
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

// Matches Badge. "running" is coloured text rather than a solid fill because
// several runs can be live at once, and it uses accentText because the flat
// accent falls under 4.5:1 in the light theme. A finished "offsite" run gets its
// domain colour, since the accent would read as in progress and is close to the
// amber of "info".
export function colorFor(status: LogStatus): string {
  switch (status) {
    case "success":
      return "text-statusOk";
    case "failed":
      return "text-statusFail";
    case "running":
      return "text-accentText";
    case "offsite":
      return "text-statusOffsite";
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
  runFilter = null,
  onClearRunFilter,
  hueIndex,
}: {
  /** Local calendar day (YYYY-MM-DD) picked in the Dashboard heatmap, or null.
   *  Combines with the text, domain and type filters. */
  dayFilter?: string | null;
  /** Called by the day chip's clear button; the Dashboard owns the state. */
  onClearDayFilter?: () => void;
  /** One run, linked from a key's log on the MCP card, or null. */
  runFilter?: string | null;
  onClearRunFilter?: () => void;
  /** Rainbow position of the heading, from Dashboard's nextHue() counter.
   *  Omit for the plain accent. */
  hueIndex?: number;
} = {}) {
  const { t } = useT();
  const [runs, setRuns] = useState<Run[]>([]);
  const [scheduleNext, setScheduleNext] = useState<ScheduleNext[]>([]);
  const [now, setNow] = useState<number>(() => Date.now());
  const progressMap = useProgress();
  const { byRunId } = useOpenAnomalies();

  const [filterText, setFilterText] = useState("");
  const [filterDomain, setFilterDomain] = useState<LogFilterDomain>("all");
  const [filterType, setFilterType] = useState<LogFilterKind>("all");

  const scrollRef = useRef<HTMLDivElement>(null);
  const [autoFollow, setAutoFollow] = useState(true);

  // Finished runs are polled; live progress arrives through useProgress().
  useEffect(() => {
    let alive = true;
    const load = () => {
      listRuns()
        .then((res) => {
          if (alive && res.ok) setRuns(res.runs ?? []);
        })
        .catch(() => {
          /* keep showing the last known runs */
        });
    };
    load();
    const id = setInterval(load, POLL_RUNS_MS);
    return () => {
      alive = false;
      clearInterval(id);
    };
  }, []);

  // The next scheduled run, for the idle line at the end.
  useEffect(() => {
    let alive = true;
    const load = () => {
      getScheduleNext()
        .then((next) => {
          if (alive) setScheduleNext(next);
        })
        .catch(() => {
          /* keep the last known schedule */
        });
    };
    load();
    const id = setInterval(load, POLL_SCHEDULE_MS);
    return () => {
      alive = false;
      clearInterval(id);
    };
  }, []);

  // Keeps the idle countdown moving when nothing else re-renders.
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), TICK_MS);
    return () => clearInterval(id);
  }, []);

  // A faster clock for the elapsed time of a live off-site line. The minute
  // clock can still be behind the run's server-side start, and a negative span
  // renders as an empty duration.
  const hasActiveProgress = Object.values(progressMap).some((s) => s.active);
  const [liveNow, setLiveNow] = useState<number>(() => Date.now());
  useEffect(() => {
    if (!hasActiveProgress) return;
    const id = setInterval(() => setLiveNow(Date.now()), LIVE_TICK_MS);
    return () => clearInterval(id);
  }, [hasActiveProgress]);

  // buildLogLines takes this instead of `t`, so it can be tested without an
  // I18nProvider.
  const resolveName: ResolveName = (key, params, count) => {
    let s = t(key as TranslationKey, count);
    if (params) {
      for (const [name, value] of Object.entries(params)) s = s.split(`{${name}}`).join(value);
    }
    return s;
  };

  const lines = useMemo(
    () => buildLogLines(runs, progressMap, scheduleNext, resolveName, now, liveNow),
    [runs, progressMap, scheduleNext, now, liveNow, t]
  );

  // Without lang the date search uses the browser locale, the same one
  // formatLogDate and every other date in the app render with.
  const filteredLines = useMemo(
    () =>
      filterLogLines(lines, {
        domain: filterDomain,
        kind: filterType,
        text: filterText,
        day: dayFilter ?? undefined,
        runId: runFilter ?? undefined,
      }),
    [lines, filterDomain, filterType, filterText, dayFilter, runFilter]
  );

  // Stay at the bottom as lines arrive, until the user scrolls up.
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
    // One box carries the position context, the notch hook and the padding, so
    // the heading badge lines up with the content. Nothing needs clipping to the
    // rounded corners, so unlike Dashboard's Card() there is no outer/inner split.
    <div
      className={`relative glim-notch-card bg-carbon-surface rounded-card p-5 flex flex-col gap-3${
        hueIndex !== undefined ? " glim-hue" : ""
      }`}
      style={hueIndex !== undefined ? (hueVars(hueIndex) as CSSProperties) : undefined}
    >
      {/* glim-hue on the card carries the rainbow accent to the focus rings
          and the day chip; glim-notch-card does not set --accent. */}
      <h2 className="flex items-center">
        <Badge tone="heading" size="heading" wrap hueIndex={hueIndex}>{t("activityLog.title")}</Badge>
      </h2>
      <div className="flex flex-wrap items-center gap-2">
        <input
          type="text"
          value={filterText}
          onChange={(e) => setFilterText(e.target.value)}
          placeholder={t("activityLog.filterPlaceholder")}
          aria-label={t("activityLog.filterPlaceholder")}
          className="flex-1 min-w-[10rem] rounded-control bg-carbon-surface2 px-2 py-1 text-xs text-carbon-text placeholder:text-carbon-textMuted glim-field-focus"
        />
        <SelectField
          value={filterDomain}
          onChange={(v) => setFilterDomain(v as LogFilterDomain)}
          label={t("activityLog.filterAllDomains")}
          options={LOG_FILTER_DOMAINS.map((o) => ({ value: o.value, label: t(o.key as TranslationKey) }))}
          className="rounded-control bg-carbon-surface2 px-2 py-1 text-xs text-carbon-text glim-field-focus"
        />
        <SelectField
          value={filterType}
          onChange={(v) => setFilterType(v as LogFilterKind)}
          label={t("activityLog.filterAllTypes")}
          options={LOG_FILTER_KINDS.map((o) => ({ value: o.value, label: t(o.key as TranslationKey) }))}
          className="rounded-control bg-carbon-surface2 px-2 py-1 text-xs text-carbon-text glim-field-focus"
        />
        {/* Parsed as local midnight, so the label names the same calendar day
            as the heatmap cell. */}
        {dayFilter && (
          <span className="inline-flex items-center gap-1 rounded-pill bg-accent text-accentContrast ps-2.5 pe-1 py-0.5 text-xs font-medium">
            {resolveName("activityLog.dayFilterChip", {
              date: new Date(dayFilter + "T00:00:00").toLocaleDateString(),
            })}
            <Button
              label={t("activityLog.clearDayFilter")}
              labelKey="activityLog.clearDayFilter"
              variant="chip"
              onClick={onClearDayFilter}
            />
          </span>
        )}
        {runFilter && (
          <span className="inline-flex items-center gap-1 rounded-pill bg-accent text-accentContrast ps-2.5 pe-1 py-0.5 text-xs font-medium">
            {t("activityLog.runFilterChip")}
            <Button
              label={t("activityLog.clearRunFilter")}
              labelKey="activityLog.clearRunFilter"
              variant="chip"
              onClick={onClearRunFilter}
            />
          </span>
        )}
      </div>

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
              {/* A container and a folder set can share a name, so every line
                  but the idle one names its domain. */}
              {!l.idle && (
                <span className="shrink-0 text-carbon-textMuted">{domainLabel(resolveName, l.domain)}</span>
              )}
              <span className={`flex-1 min-w-0 wrap-break-word ${l.warn ? "text-statusWarn" : colorFor(l.status)}`}>{l.text}</span>
              {l.runId && <RunAnomalyBadge findings={byRunId.get(l.runId)} t={t} />}
            </div>
          ))}
        </div>
        {!autoFollow && (
          <Button
            label={t("activityLog.jumpToLatest")}
            labelKey="activityLog.jumpToLatest"
            tone="neutral"
            onClick={jumpToLatest}
            className="absolute bottom-3 end-3"
          />
        )}
      </div>
    </div>
  );
}
