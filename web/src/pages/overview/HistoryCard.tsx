import { useState } from "react";

import { ActivityLog } from "../../components/ActivityLog";
import { Badge } from "../../components/Badge";
import { RunAnomalyBadge } from "../../components/RunAnomalyBadge";
import { SelectField } from "../../components/SelectField";
import { Selector } from "../../components/Selector";
import type { AnomalyView, Run } from "../../lib/api";
import type { useT } from "../../lib/i18n";
import { formatClockTime, formatDuration, formatRecent } from "../../lib/reltime";
import { runTargetText, statusLabel, statusTone } from "../../lib/runDisplay";
import { runKindLabel } from "../../lib/runKind";
import { isOwnReason, isWarningNote, RunReasonText } from "../../lib/runReason";
import { useOpenAnomalies } from "../../lib/useAnomalies";
import { replicaRunSentence, zfsRollbackTargets } from "../../lib/zfsCodes";
import { RollbackLine } from "../../components/zfs/replica/RollbackLine";
import { Card } from "./Card";

type T = ReturnType<typeof useT>["t"];

export type HistoryView = "runs" | "log";

/** The word a run's badge carries: what happened, in full. */
const RESULT: Record<string, "run.statusSuccess" | "run.statusFailed" | "run.statusRunning"> = {
  success: "run.statusSuccess",
  failed: "run.statusFailed",
  running: "run.statusRunning",
};

const DOT: Record<string, string> = {
  success: "bg-statusOkSolid",
  failed: "bg-statusFailSolid",
  running: "bg-accent",
  deferred: "bg-statusWarnSolid",
};

/** What a run has to add under its line: why it failed, was skipped or
 *  waits, or the note of a successful one. */
function RunReasonLine({ t, run }: { t: T; run: Run }) {
  if (!run.error) return null;
  let tone: string;
  if (run.status === "failed") tone = "text-statusFail";
  else if (run.status === "skipped") tone = "text-carbon-textMuted";
  else if (run.status === "deferred") tone = "text-statusWarn";
  else if (run.status === "success") tone = isWarningNote(run.error) ? "text-statusWarn" : "text-carbon-textMuted";
  else return null;
  const replicaSentence = run.status === "failed" ? replicaRunSentence(t, run) : null;
  if (replicaSentence) {
    return (
      <span className={`block text-xs wrap-break-word ${tone}`}>
        {replicaSentence}
        {zfsRollbackTargets(run.error).map((snapshot) => (
          <RollbackLine key={snapshot} snapshot={snapshot} />
        ))}
      </span>
    );
  }
  return (
    <span dir={isOwnReason(run.error) ? undefined : "ltr"} className={`block text-xs wrap-break-word text-start ${tone}`}>
      <RunReasonText reason={run.error} t={t} />
    </span>
  );
}

function RunRow({
  t,
  lang,
  run,
  anomalies,
  onOpen,
}: {
  t: T;
  lang: string;
  run: Run;
  /** Open findings raised on this run. */
  anomalies?: AnomalyView[];
  onOpen: () => void;
}) {
  const kind = runKindLabel(t, run.kind);
  const target = runTargetText(t, run);
  const span =
    run.finishedAt == null
      ? formatClockTime(run.startedAt, false)
      : `${t("entry.runs.span")
          .replace("{start}", formatClockTime(run.startedAt, false))
          .replace("{end}", formatClockTime(run.finishedAt, false))} · ${formatDuration(run.finishedAt - run.startedAt)}`;
  const result = RESULT[run.status];
  return (
    <li className="relative flex items-center gap-3 rounded-control border-t border-carbon-border px-1.5 py-2.5 first:border-t-0 hover:bg-carbon-hover">
      <span aria-hidden="true" className={`h-2 w-2 flex-none rounded-full ${DOT[run.status] ?? "bg-carbon-surface3"}`} />
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-sm">
          {/* The button's box is stretched over the row, so the whole row
              opens the run while a screen reader hears one named button. */}
          <button
            type="button"
            onClick={onOpen}
            className="min-w-0 truncate text-start font-semibold text-carbon-text after:absolute after:inset-0 after:rounded-control focus-visible:outline-none focus-visible:after:outline-solid focus-visible:after:outline-2 focus-visible:after:outline-(--focus-ring)"
          >
            {target}
          </button>
          <span className="text-carbon-textMuted">{kind}</span>
          <span className="relative z-10">
            <RunAnomalyBadge findings={anomalies} t={t} />
          </span>
        </div>
        <span dir="ltr" className="text-xs tabular-nums text-carbon-textMuted text-start">
          {span}
        </span>
        <RunReasonLine t={t} run={run} />
      </div>
      <div className="flex flex-none items-center gap-3 max-md:flex-col max-md:items-end max-md:gap-1">
        <Badge tone={statusTone(run.status)}>{result ? t(result) : statusLabel(run.status, t)}</Badge>
        <span className="text-xs tabular-nums text-carbon-textMuted">{formatRecent(run.finishedAt ?? run.startedAt, lang)}</span>
      </div>
    </li>
  );
}

const dayOf = (run: Run, lang: string) => new Date(run.startedAt * 1000).toLocaleDateString(lang);

function RunList({
  t,
  lang,
  runs,
  day,
  loading,
  failed,
  onOpenRun,
}: {
  t: T;
  lang: string;
  runs: Run[];
  /** The day the list is narrowed to, as dayOf writes it, or "all". */
  day: string;
  loading: boolean;
  failed: boolean;
  onOpenRun: (run: Run) => void;
}) {
  const { byRunId } = useOpenAnomalies();
  const shown = day === "all" ? runs : runs.filter((run) => dayOf(run, lang) === day);

  if (loading) return <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>;
  // A list that never arrived must not read as an empty history.
  if (failed && runs.length === 0) return <p className="text-sm text-statusFail">{t("dashboard.loadRunsFailed")}</p>;
  if (runs.length === 0) return <p className="text-sm text-carbon-textMuted">{t("dashboard.noRuns")}</p>;
  return (
    <div className="flex flex-col gap-2 glim-content-fade">
      <ul className="flex max-h-[26rem] flex-col overflow-y-auto pe-1 [[data-sized]_&]:max-h-none">
        {shown.map((run) => (
          <RunRow key={run.id} t={t} lang={lang} run={run} anomalies={byRunId.get(run.id)} onOpen={() => onOpenRun(run)} />
        ))}
      </ul>
    </div>
  );
}

/**
 * HistoryCard holds every run twice: as a list that reads at a glance and
 * opens a run, and as the log with each line and its time. A day or a run
 * picked elsewhere opens the log.
 */
export function HistoryCard({
  t,
  lang,
  hueIndex,
  runs,
  loading,
  failed,
  view,
  onView,
  logDay,
  onClearLogDay,
  logRun,
  onClearLogRun,
  onOpenRun,
}: {
  t: T;
  lang: string;
  hueIndex?: number;
  /** The page's polled runs, newest first. */
  runs: Run[];
  loading: boolean;
  failed: boolean;
  view: HistoryView;
  onView: (view: HistoryView) => void;
  /** The local day (YYYY-MM-DD) the log is narrowed to, or null. */
  logDay: string | null;
  onClearLogDay: () => void;
  /** The one run the log is narrowed to, or null. */
  logRun: string | null;
  onClearLogRun: () => void;
  onOpenRun: (run: Run) => void;
}) {
  const [day, setDay] = useState("all");
  // Runs arrive newest first, so the days come out in that order too.
  const days = [...new Set(runs.map((run) => dayOf(run, lang)))];
  return (
    <Card title={t("overview.history.title")} hint={t("overview.history.hint")} hueIndex={hueIndex}>
      <div className="flex flex-wrap items-center justify-end gap-2">
      <Selector
        items={[
          { id: "runs", label: t("overview.history.runs") },
          { id: "log", label: t("mcp.log") },
        ]}
        label={t("overview.history.title")}
        select="one"
        active={view}
        onChange={(id) => onView(id as HistoryView)}
        variant="well"
        inline
      />
      {view === "runs" && runs.length > 0 && (
        <>
          <span className="text-xs text-carbon-textMuted">{t("run.filterDay")}</span>
          <SelectField
            value={day}
            onChange={setDay}
            label={t("run.filterDay")}
            options={[{ value: "all", label: t("run.allDays") }, ...days.map((d) => ({ value: d, label: d }))]}
            className="rounded-control bg-carbon-surface2 px-2 py-1 text-xs text-carbon-text glim-field-focus"
          />
        </>
      )}
      </div>
      {view === "log" ? (
        <ActivityLog dayFilter={logDay} onClearDayFilter={onClearLogDay} runFilter={logRun} onClearRunFilter={onClearLogRun} />
      ) : (
        <RunList t={t} lang={lang} runs={runs} day={day} loading={loading} failed={failed} onOpenRun={onOpenRun} />
      )}
    </Card>
  );
}
