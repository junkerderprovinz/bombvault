import type { CSSProperties, ReactNode } from "react";

import { KIND_LABEL, KindGlyph } from "../../components/backups/kinds";
import { IconShieldOn } from "../../components/glyphs";
import { IconCloud, IconSchedules } from "../../components/navGlyphs";
import type { Run } from "../../lib/api";
import { kindOfDomain } from "../../lib/backupEntry";
import type { TranslationKey, useT } from "../../lib/i18n";
import { lastBackupOf, type Tally } from "../../lib/protectionTally";
import { formatRecent } from "../../lib/reltime";
import { runTargetText } from "../../lib/runDisplay";
import { VERDICT, type CheckTone, type Section } from "./protectionChecks";
import { STATIONS, passShare, type Headline, type LiveRun, type Pass, type Station } from "./headState";

type T = ReturnType<typeof useT>["t"];

const RADIUS = 27;
const ROUND = 2 * Math.PI * RADIUS;
const GAP = 4;

const SEGMENT: Record<CheckTone, string> = {
  ok: "stroke-statusOkSolid",
  warn: "stroke-statusWarnSolid",
  fail: "stroke-statusFailSolid",
  run: "stroke-accent",
  neutral: "stroke-carbon-surface3",
};

const HEADLINE: Record<Exclude<Headline, "needs">, TranslationKey> = {
  waiting: "backups.status.waiting",
  partly: "overview.head.partly",
  notes: "overview.head.notes",
  allGood: "overview.head.allGood",
};

const STATION_LABEL: Record<Station, TranslationKey> = {
  stop: "overview.station.stop",
  read: "overview.station.read",
  start: "overview.station.start",
  copy: "common.copy",
};

function RingFrame({ children, centre, tone }: { children: ReactNode; centre: ReactNode; tone: string }) {
  return (
    <span data-new="overview-head" className="relative block h-16 w-16 flex-none">
      {/* A circle's stroke starts at three o'clock; the turn puts the first
          part at the top. */}
      <svg viewBox="0 0 64 64" aria-hidden="true" className="block h-full w-full -rotate-90 overflow-visible fill-none stroke-[6]">
        {children}
      </svg>
      <span className={`absolute inset-0 flex items-center justify-center [&_svg]:h-6 [&_svg]:w-6 ${tone}`}>{centre}</span>
    </span>
  );
}

/**
 * ProtectionRing has one part per section, coloured like that section's row
 * on the protection card. The middle counts what needs the person.
 */
function ProtectionRing({ sections, needs, anyBackup, t }: { sections: Section[]; needs: number; anyBackup: boolean; t: T }) {
  const each = ROUND / Math.max(1, sections.length);
  const gap = sections.length > 1 ? GAP : 0;
  const centre =
    needs > 0 ? (
      <b className="text-status font-bold leading-none tabular-nums">{needs}</b>
    ) : anyBackup ? (
      <IconShieldOn />
    ) : (
      <IconSchedules />
    );
  return (
    <RingFrame centre={centre} tone={needs > 0 ? "text-statusWarn" : anyBackup ? "text-statusOk" : "text-carbon-textMuted"}>
      {sections.map((section, i) => (
        <circle
          key={section.kind}
          data-tone={section.tone}
          cx="32"
          cy="32"
          r={RADIUS}
          strokeDasharray={`${(each - gap).toFixed(2)} ${ROUND.toFixed(2)}`}
          strokeDashoffset={(-(i * each + gap / 2)).toFixed(2)}
          style={{ "--ring-i": i } as CSSProperties}
          className={`glim-ring-grow ${SEGMENT[section.tone]}`}
        >
          <title>{`${t(KIND_LABEL[section.kind])}: ${t(VERDICT[section.tone])}`}</title>
        </circle>
      ))}
    </RingFrame>
  );
}

function ProgressRing({ percent, onOpen, t }: { percent: number; onOpen: () => void; t: T }) {
  const shown = Math.min(100, Math.max(0, Math.round(percent)));
  return (
    <button
      type="button"
      onClick={onOpen}
      aria-label={t("overview.head.viewRun")}
      className="flex-none rounded-pill focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-[3px] focus-visible:outline-(--focus-ring)"
    >
      <RingFrame centre={<b className="text-sm font-semibold tabular-nums">{shown} %</b>} tone="text-carbon-text">
        <circle cx="32" cy="32" r={RADIUS} className="stroke-carbon-surface2" />
        <circle
          cx="32"
          cy="32"
          r={RADIUS}
          strokeLinecap="round"
          strokeDasharray={ROUND.toFixed(2)}
          strokeDashoffset={(ROUND * (1 - shown / 100)).toFixed(2)}
          className="glim-ring-progress stroke-accent"
        />
      </RingFrame>
    </button>
  );
}

/**
 * StationStrip names where the run stands. An entry that is read while it
 * runs skips the stop and the start, and those two stand back. Between two
 * entries of a pass nothing stands at a station.
 */
function StationStrip({ run, t }: { run: LiveRun | null; t: T }) {
  const at = run ? STATIONS.indexOf(run.station) : -1;
  return (
    <div
      role="img"
      aria-label={t(run ? STATION_LABEL[run.station] : "overview.head.preparing")}
      className="relative grid max-w-[26rem] min-w-0 flex-1 grid-cols-4 pt-[30px]"
    >
      <span className="absolute inset-x-[12.5%] top-[33px] h-0.5 rounded-full bg-carbon-surface3" />
      {STATIONS.map((station, i) => {
        const skipped = run !== null && !run.stops && (station === "stop" || station === "start");
        return (
          <span
            key={station}
            data-station={station}
            data-here={i === at}
            className={`relative flex min-w-0 flex-col items-center gap-1.5 text-meta ${
              i === at ? "text-carbon-text" : "text-carbon-textSub"
            }${skipped ? " opacity-45" : ""}`}
          >
            <i className={`h-2 w-2 rounded-full ${i === at ? "bg-accent" : "bg-carbon-surface3"}`} />
            <span className="max-w-full truncate">{t(STATION_LABEL[station])}</span>
          </span>
        );
      })}
      {run && (
        <span
          className="glim-station-mover absolute top-0 -ms-3 flex h-6 w-6 items-center justify-center rounded-[7px] bg-accent text-accentContrast [&_svg]:h-4 [&_svg]:w-4"
          style={{ insetInlineStart: `${12.5 + 25 * at}%` }}
        >
          {run.kind ? <KindGlyph kind={run.kind} /> : <IconCloud />}
        </span>
      )}
    </div>
  );
}

/**
 * DoneStack collects one mark per backup a pass has finished. The headline
 * carries the count, so the stack is a picture only.
 */
function DoneStack({ runs, t }: { runs: Run[]; t: T }) {
  return (
    <div aria-hidden="true" className="flex flex-none flex-col items-center gap-1.5 text-meta text-carbon-textMuted">
      <span data-testid="pass-marks" className="flex min-h-[22px] w-[50px] flex-wrap-reverse content-start gap-0.5">
        {runs.map((run, i) => (
          <i
            key={run.id}
            data-tone={run.status === "success" ? "ok" : "fail"}
            title={runTargetText(t, run)}
            className={`h-1.5 w-2 rounded-sm ${run.status === "success" ? "bg-statusOkSolid" : "bg-statusFailSolid"}${
              i === runs.length - 1 ? " glim-mark-drop" : ""
            }`}
          />
        ))}
      </span>
      {t("overview.station.done")}
    </div>
  );
}

function runName(run: LiveRun, t: T): string {
  if (run.kind) return run.name || t(run.kind === "flash" ? "backups.name.flash" : "backups.name.config");
  const kind = kindOfDomain(run.name);
  return kind ? t(KIND_LABEL[kind]) : run.name;
}

export interface StatusHeadProps {
  t: T;
  lang: string;
  /** False until the entries and the sections have both answered once. */
  ready: boolean;
  /** Neither of them has ever answered. */
  failed: boolean;
  headline: Headline;
  needs: number;
  tally: Tally;
  sections: Section[];
  /** The next scheduled backup in Unix seconds, null without one. */
  nextRun: number | null;
  run: LiveRun | null;
  /** The pass over everything while it runs. */
  pass: Pass | null;
  /** The backups that pass has finished, oldest first. */
  passRuns: Run[];
  onOpenRun: () => void;
}

/**
 * StatusHead says in one sentence how the backups are doing, beside the ring
 * that shows how much is protected. While a backup or an off-site copy runs it
 * follows that run instead, and during a pass over everything it counts the
 * entries the pass is through.
 */
export function StatusHead({
  t,
  lang,
  ready,
  failed,
  headline,
  needs,
  tally,
  sections,
  nextRun,
  run,
  pass,
  passRuns,
  onOpenRun,
}: StatusHeadProps) {
  const last = lastBackupOf(tally);
  const copying = run !== null && run.kind === null;
  const busy = run !== null || pass !== null;

  let title: string;
  let line: ReactNode;
  if (busy) {
    title = pass
      ? t("overview.head.pass").replace("{done}", String(pass.done)).replace("{total}", String(pass.total))
      : t(copying ? "overview.head.copying" : "overview.head.backingUp");
    const named = run ? runName(run, t) : "";
    const name = named && <b className="font-semibold text-carbon-text">{named}</b>;
    const [before, after] = t("overview.head.passItem").split("{name}");
    line = !run ? (
      t("overview.head.preparing")
    ) : pass && !copying ? (
      <>
        {before}
        {name}
        {after}
      </>
    ) : (
      <>
        {name}
        {run.step && (
          <>
            {named && " · "}
            {t("overview.head.copyStep")
              .replace("{index}", String(run.step.index))
              .replace("{total}", String(run.step.total))}
          </>
        )}
      </>
    );
  } else if (!ready) {
    title = t(failed ? "dashboard.statusLoadFailed" : "dashboard.checking");
    line = null;
  } else {
    title = headline === "needs" ? t("overview.head.needs", needs) : t(HEADLINE[headline]);
    line = [
      t("coverage.ratio").replace("{protected}", String(tally.ok)).replace("{total}", String(tally.total)),
      last > 0 && t("overview.head.last").replace("{when}", formatRecent(last, lang)),
      nextRun !== null && t("overview.head.next").replace("{when}", formatRecent(nextRun, lang)),
    ]
      .filter(Boolean)
      .join(" · ");
  }

  return (
    <div data-testid="status-head" className="flex items-center gap-4 md:gap-5">
      {busy ? (
        <ProgressRing percent={pass ? passShare(pass, run) : (run?.percent ?? 0)} onOpen={onOpenRun} t={t} />
      ) : (
        <ProtectionRing sections={ready ? sections : []} needs={ready ? needs : 0} anyBackup={tally.ok > 0} t={t} />
      )}
      <div className="min-w-0 flex-1">
        <p className="text-status font-semibold leading-tight text-carbon-text">{title}</p>
        {line && <p className="mt-1 text-sm text-carbon-textSub">{line}</p>}
        {busy && (
          <div className="mt-3 flex items-end gap-4 max-md:gap-2.5">
            <StationStrip run={run} t={t} />
            {pass && <DoneStack runs={passRuns} t={t} />}
          </div>
        )}
      </div>
    </div>
  );
}
