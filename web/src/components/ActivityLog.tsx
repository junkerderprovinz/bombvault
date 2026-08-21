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
import { Badge } from "./Badge";
import { formatClockTime } from "../lib/reltime";

const POLL_RUNS_MS = 10000;
const POLL_SCHEDULE_MS = 30000;
// The idle line's countdown ("in 2h 14m") only needs to visibly tick at
// minute granularity — no point re-rendering more often just for that.
// Deliberately UNCHANGED by issue #159's live-duration fix below — this cadence
// still governs the idle countdown and the staleness check exactly as before.
const TICK_MS = 60000;
// How often the live off-site line's elapsed duration re-renders — see
// liveNow's doc comment below. Matches OffsiteIndicator's own ELAPSED_TICK_MS
// so the two surfaces tick at the same visible rate.
const LIVE_TICK_MS = 1000;
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

// Reuses the exact hex values Dashboard's Badge uses for success/failed/
// running so a log line reads as the same colour language as the rest of the
// app (#66-style shared vocabulary), not a new palette.
//
// "running" (Task 7: resolve the fifth hue) — was text-statusInfo, the old
// fifth hue. It means genuine activity happening right now, so accent-derived
// text — but plain colour on text in a scrolling list, never a solid fill:
// this log routinely shows several "running" lines at once (independent
// domains backing up concurrently all merge into one list), and rule 3's "at
// most one solid accent" cap is specifically about SOLID accent claiming the
// page's one primary-action weight. Coloured text reads at the same register
// as the success/failed lines right next to it. text-accentText, not the flat
// text-accent: that same commit's spec-compliance follow-up found the flat
// accent gold measures ~1.6:1 in light theme against this log's surfaces —
// badly under the 4.5:1 text minimum (dark theme is fine). Same fix as
// Recovery.tsx's identical pattern; see index.css's --accent-text comment for
// the measured numbers.
//
// "offsite" (issue #164) — deliberately NOT the accent, and split back out of
// the "running" arm Task 7 merged it into. Task 7's stated premise was "both
// mean genuine activity happening right now"; for this status that is simply
// false. lib/activityLog.ts's finishedLineText returns status "offsite" for a
// FINISHED, successful replication run ("Off-site replication done —
// Containers (4m 17s)"), so the merged arm painted completed runs with the
// "in progress" accent — and, because colorFor("info") is text-statusWarn
// #f1c21b and the default accent is #FCC419 (~11 RGB / 1.05:1 apart, see
// index.css's --status-warn-text KNOWN LIMITATION), it also made off-site
// lines and info lines near-indistinguishable amber in the same log, which is
// exactly the glance value #164 reported losing. text-statusOffsite is a
// DOMAIN IDENTITY colour for one job type, not a resurrected fifth state hue
// — index.css's --color-statusOffsite comment has the full reasoning and the
// pairing note for internal/api/widget.html, which hard-codes the same hex.
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

  // liveNow is a SEPARATE, faster clock feeding ONLY the off-site live line's
  // elapsed-duration computation (lib/activityLog.ts's buildLiveLines takes it
  // as an explicit param distinct from `now`) — issue #159 review: `now`'s
  // 60s TICK_MS above is coarse on purpose for the idle countdown, but reusing
  // it for a live run's elapsed duration meant that for the run's first ~60s,
  // `now` could still sit BEHIND the backend-stamped startedAt (captured
  // before this component's next 60s tick), making the computed elapsed span
  // go NEGATIVE — reltime.ts's elapsedSince/formatDuration reject a negative
  // span by returning "", so the duration rendered blank, then jumped straight
  // to a large value once `now` finally caught up. Only ticks while at least
  // one progress entry is active, so the log pays zero extra render cost while
  // idle — TICK_MS above is UNCHANGED for the idle countdown and the
  // staleness check, which don't need this.
  const hasActiveProgress = Object.values(progressMap).some((s) => s.active);
  const [liveNow, setLiveNow] = useState<number>(() => Date.now());
  useEffect(() => {
    if (!hasActiveProgress) return;
    const id = setInterval(() => setLiveNow(Date.now()), LIVE_TICK_MS);
    return () => clearInterval(id);
  }, [hasActiveProgress]);

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
    () => buildLogLines(runs, progressMap, scheduleNext, resolveName, now, liveNow),
    [runs, progressMap, scheduleNext, now, liveNow, t]
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
      <h2 className="flex items-center">
        <Badge tone="heading" size="heading" wrap>{t("activityLog.title")}</Badge>
      </h2>

      {/* Filter bar — narrows the ONE list below; never a second zone. */}
      <div className="flex flex-wrap items-center gap-2">
        <input
          type="text"
          value={filterText}
          onChange={(e) => setFilterText(e.target.value)}
          placeholder={t("activityLog.filterPlaceholder")}
          aria-label={t("activityLog.filterPlaceholder")}
          className="flex-1 min-w-[10rem] rounded-control bg-carbon-surface2 px-2 py-1 text-xs text-carbon-text placeholder:text-carbon-textMuted bv-field-focus"
        />
        <select
          value={filterDomain}
          onChange={(e) => setFilterDomain(e.target.value as LogFilterDomain)}
          className="rounded-control bg-carbon-surface2 px-2 py-1 text-xs text-carbon-text bv-field-focus"
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
          className="rounded-control bg-carbon-surface2 px-2 py-1 text-xs text-carbon-text bv-field-focus"
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
            className="absolute bottom-3 end-3 rounded-pill bg-carbon-surface2 px-3 py-1 text-xs text-carbon-text shadow-lg hover:bg-carbon-hover"
          >
            ↓ {t("activityLog.jumpToLatest")}
          </button>
        )}
      </div>
    </div>
  );
}
