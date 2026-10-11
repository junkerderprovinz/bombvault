// The week on the Schedule page: every planned run from Monday to Sunday, one
// lane per day, so runs that crowd each other show before they collide.
import { useEffect, useMemo, useState, type CSSProperties } from "react";
import { IconAnomalies } from "../../components/navGlyphs";
import { getScheduleNext, type ScheduleNext, type Settings } from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { browserOffsetSeconds, useForeignScheduleZone } from "../../lib/scheduleZone";
import { Card } from "./shared";
import { clock, crowdOf, weekRuns, type Plan, type PlannedRun, type RunKind } from "./weekPlan";

const KINDS: { kind: RunKind; label: TranslationKey }[] = [
  { kind: "backup", label: "tile.week.backup" },
  { kind: "copy", label: "tile.week.copy" },
  { kind: "check", label: "tile.week.check" },
  { kind: "probe", label: "tile.week.probe" },
];

const DOMAINS = [
  { domain: "containers", on: "containersEnabled", label: "jobs.containersSection" },
  { domain: "vms", on: "vmsEnabled", label: "jobs.vmsSection" },
  { domain: "flash", on: "flashEnabled", label: "jobs.flashSection" },
  { domain: "files", on: "filesEnabled", label: "jobs.filesSection" },
  { domain: "zfs", on: "zfsEnabled", label: "jobs.zfsSection" },
  { domain: "config", on: "configEnabled", label: "settings.schedulesSelfBackup" },
] as const;

const DAYS = [0, 1, 2, 3, 4, 5, 6];
const HOURS = [0, 3, 6, 9, 12, 15, 18, 21, 24];
// A stroke is this many minutes wide, so a start can be seen and pointed at.
const STROKE = 14;

/** plansOf lists what the settings schedule, with the next fire of each plan
 *  the server reports one for. */
export function plansOf(settings: Settings, next: ScheduleNext[], t: (k: TranslationKey) => string): Plan[] {
  const fire = (job: string, domain: string) => {
    const hit = next.find((n) => n.job === job && n.domain === domain);
    return hit ? new Date(hit.next) : undefined;
  };
  const plans: Plan[] = [
    {
      key: "everything",
      kind: "backup",
      label: t("settings.everythingTitle"),
      cadence: settings.everythingSchedule,
      next: fire("backup", "everything"),
    },
  ];
  for (const d of DOMAINS) {
    if (!settings[d.on]) continue;
    plans.push({
      key: d.domain,
      kind: "backup",
      label: t(d.label),
      cadence: settings[`${d.domain}Schedule`],
      next: fire("backup", d.domain),
    });
    plans.push({
      key: `${d.domain}-offsite`,
      kind: "copy",
      label: `${t("settings.schedulesOffsite")}: ${t(d.label)}`,
      cadence: settings[`${d.domain}OffsiteSchedule`],
      next: fire("offsite", d.domain),
    });
  }
  if (settings.drillsEnabled) {
    plans.push({ key: "drills", kind: "probe", label: t("verify.auto"), cadence: settings.drillsSchedule, next: fire("drill", "") });
  }
  plans.push({
    key: "tamper",
    kind: "check",
    label: t("settings.tamperTestSchedule"),
    cadence: settings.tamperTestSchedule,
    next: fire("tamper", ""),
  });
  return plans;
}

function percent(minute: number): string {
  return `${((minute / 1440) * 100).toFixed(3)}%`;
}

function Lane({
  runs,
  now,
  lit,
  onPoint,
}: {
  runs: PlannedRun[];
  /** The minute of the day the needle stands at, on today's lane only. */
  now?: number;
  lit: (run: PlannedRun) => boolean | null;
  onPoint: (run: PlannedRun | null) => void;
}) {
  const { t } = useT();
  const day = [...runs].sort((a, b) => a.minute - b.minute);
  // A start that would cover the one before it drops to the next track.
  const ends: number[] = [];
  const strokes = day.map((run) => {
    let track = ends.findIndex((end) => end <= run.minute);
    if (track < 0) track = ends.length;
    ends[track] = run.minute + STROKE;
    return { run, track };
  });
  const crowd = crowdOf(day);

  return (
    <div className="glim-week-lane" style={{ "--tracks": Math.max(1, ends.length) } as CSSProperties}>
      {crowd && (
        <span
          className="glim-week-crowd"
          style={{ insetInlineStart: percent(crowd.from), width: percent(crowd.to - crowd.from + STROKE) }}
        />
      )}
      {strokes.map(({ run, track }) => {
        const state = lit(run);
        return (
          <span
            key={`${run.key}:${run.minute}`}
            tabIndex={0}
            role="img"
            aria-label={`${run.label}, ${clock(run.minute)}`}
            className={`glim-week-stroke glim-week-${run.kind}${state === true ? " glim-week-lit" : state === false ? " glim-week-dim" : ""}`}
            style={
              {
                insetInlineStart: percent(run.minute),
                width: percent(Math.min(STROKE, 1440 - run.minute)),
                "--track": track,
              } as CSSProperties
            }
            onMouseEnter={() => onPoint(run)}
            onMouseLeave={() => onPoint(null)}
            onFocus={() => onPoint(run)}
            onBlur={() => onPoint(null)}
          />
        );
      })}
      {now !== undefined && (
        <span className={`glim-week-now${now > 1260 ? " glim-week-now-late" : ""}`} style={{ insetInlineStart: percent(now) }}>
          <i>{t("tile.week.now")}</i>
        </span>
      )}
    </div>
  );
}

export function WeekCard({ settings, hueIndex }: { settings: Settings; hueIndex: number }) {
  const { t, lang } = useT();
  const [next, setNext] = useState<ScheduleNext[]>([]);
  const [pointed, setPointed] = useState<PlannedRun | null>(null);
  const [legend, setLegend] = useState<RunKind | null>(null);
  const foreign = useForeignScheduleZone();
  const offset = foreign ? foreign.offsetSeconds : browserOffsetSeconds();
  // Counts the minutes, so the needle moves while the page stays open.
  const [, setMinutes] = useState(0);

  useEffect(() => {
    let alive = true;
    getScheduleNext()
      .then((runs) => {
        if (alive) setNext(runs);
      })
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, []);

  useEffect(() => {
    const id = window.setInterval(() => setMinutes((n) => n + 1), 60000);
    return () => window.clearInterval(id);
  }, []);

  const runs = useMemo(() => weekRuns(plansOf(settings, next, t), offset), [settings, next, t, offset]);
  // The needle stands on the server's clock, like the cadences.
  const server = new Date(Date.now() + offset * 1000);
  const today = (server.getUTCDay() + 6) % 7;
  const nowMinute = server.getUTCHours() * 60 + server.getUTCMinutes();

  const dayName = (day: number) =>
    new Intl.DateTimeFormat(lang, { weekday: "short", timeZone: "UTC" }).format(new Date(Date.UTC(2024, 0, 1 + day)));
  const range = (day: number, from: number, to: number) =>
    t("tile.week.range").replace("{day}", dayName(day)).replace("{from}", clock(from)).replace("{to}", clock(to));

  const crowded = DAYS.map((day) => ({ day, crowd: crowdOf(runs.filter((r) => r.day === day)) }))
    .filter((c): c is { day: number; crowd: NonNullable<ReturnType<typeof crowdOf>> } => c.crowd !== null)
    .sort((a, b) => b.crowd.plans - a.crowd.plans)[0];

  function lit(run: PlannedRun): boolean | null {
    if (pointed) return run.key === pointed.key;
    if (legend) return run.kind === legend;
    return null;
  }

  const total = t("tile.week.runs", runs.length);
  let readout = <span>{total}</span>;
  if (pointed) {
    readout = (
      <span>
        {dayName(pointed.day)} {clock(pointed.minute)} · {pointed.label}
      </span>
    );
  } else if (legend) {
    const kind = KINDS.find((k) => k.kind === legend);
    readout = (
      <span>
        {kind && t(kind.label)}: {t("tile.week.runs", runs.filter((r) => r.kind === legend).length)}
      </span>
    );
  } else if (crowded) {
    readout = (
      <>
        <span>{total}</span>
        <span className="ms-1.5 inline-flex items-center gap-1 text-statusWarn before:me-1.5 before:text-carbon-textSub before:content-['·']">
          <IconAnomalies />
          {range(crowded.day, crowded.crowd.from, crowded.crowd.to)}: {t("tile.week.crowd", crowded.crowd.plans)}
        </span>
      </>
    );
  }

  return (
    <Card title={t("tile.week.title")} hint={t("tile.week.hint")} hueIndex={hueIndex} newId="settings-week">
      <div className="glim-week-scroll">
        <div className="glim-week-grid">
          <div className="glim-week-row">
            <span />
            <div className="glim-week-hours" aria-hidden="true">
              {HOURS.map((h) => (
                <span key={h} style={{ insetInlineStart: percent(h * 60) }}>
                  {String(h).padStart(2, "0")}
                </span>
              ))}
            </div>
          </div>
          {DAYS.map((day) => (
            <div key={day} className="glim-week-row">
              <span className={`glim-week-day${day === today ? " text-accentText" : ""}`}>{dayName(day)}</span>
              <Lane
                runs={runs.filter((r) => r.day === day)}
                now={day === today ? nowMinute : undefined}
                lit={lit}
                onPoint={setPointed}
              />
            </div>
          ))}
        </div>
      </div>
      <div className="flex flex-col gap-2">
        <div className="flex flex-wrap gap-x-2.5 gap-y-1">
          {KINDS.map(({ kind, label }) => (
            <span
              key={kind}
              tabIndex={0}
              className="glim-week-legend"
              onMouseEnter={() => setLegend(kind)}
              onMouseLeave={() => setLegend(null)}
              onFocus={() => setLegend(kind)}
              onBlur={() => setLegend(null)}
            >
              <i className={`glim-week-${kind}`} />
              {t(label)}
            </span>
          ))}
        </div>
        <p aria-live="polite" className="flex min-h-[1.45em] flex-wrap items-center text-dense text-carbon-textSub">
          {readout}
        </p>
      </div>
    </Card>
  );
}
