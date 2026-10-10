import type { CSSProperties, MouseEvent, ReactNode } from "react";
import { Link, useNavigate } from "react-router-dom";
import { anomalyLearningText, itemOpenCounts } from "../../lib/anomalies";
import type { AnomalyItem, BackupItem } from "../../lib/api";
import { hueVars } from "../../lib/appearance";
import {
  entryTarget,
  inSchedule,
  isGone,
  itemProgressKey,
  protection,
  rowStatus,
  runsNow,
  type RunTone,
} from "../../lib/backupList";
import { humanBytes } from "../../lib/forecast";
import type { TranslationKey } from "../../lib/i18n";
import type { ProgressState } from "../../lib/progress";
import { formatRecent } from "../../lib/reltime";
import { useTipBubble } from "../../lib/useTipBubble";
import { BackupCancelButton } from "../BackupCancelButton";
import { Badge } from "../Badge";
import type { ScheduleSentence } from "../EffectiveScheduleLine";
import { InfoBubble } from "../InfoBubble";
import { IconAnomalies } from "../navGlyphs";
import { ProgressBar } from "../ProgressBar";
import { KindGlyph, kindHue } from "./kinds";

type T = (key: TranslationKey, n?: number) => string;

const ROW = "flex min-h-[50px] items-center gap-3 border-t border-carbon-border px-1.5 py-2 first:border-t-0";
const GLYPH_TILE =
  "flex h-[34px] w-[34px] flex-none items-center justify-center rounded-control bg-carbon-surface2 [&_svg]:h-[18px] [&_svg]:w-[18px]";
// One width for every status, so the strips before them line up in a column.
// A longer one takes a second line.
const STATUS_WIDTH = "w-[9.5rem] @max-[36rem]:w-auto @max-[36rem]:max-w-[9.5rem]";
const NAME =
  "min-w-0 truncate text-sm font-semibold text-carbon-text @max-[36rem]:whitespace-normal @max-[36rem]:wrap-anywhere";

const STRIP_TONE: Record<RunTone, string> = {
  ok: "bg-statusOkSolid",
  warn: "bg-statusWarnSolid",
  fail: "bg-statusFailSolid",
  none: "bg-carbon-surface3",
};

const STRIP_COUNT: [RunTone, TranslationKey][] = [
  ["ok", "backups.strip.ok"],
  ["warn", "backups.strip.anomaly"],
  ["fail", "backups.strip.failed"],
];

// A click on something inside the row that acts by itself must not open the row.
function keepInRow(e: MouseEvent) {
  e.stopPropagation();
}

function RunStrip({ tones, t }: { tones: RunTone[]; t: T }) {
  const parts = STRIP_COUNT.map(([tone, key]) => [tones.filter((x) => x === tone).length, key] as const)
    .filter(([n]) => n > 0)
    .map(([n, key]) => t(key, n));
  const label = parts.length > 0 ? `${t("run.recentTitle")}: ${parts.join(", ")}` : t("dashboard.noRuns");
  const tip = useTipBubble(label);

  return (
    <>
      <span
        ref={tip.ref}
        role="img"
        aria-label={label}
        onMouseEnter={tip.handlers.onMouseEnter}
        onMouseLeave={tip.handlers.onMouseLeave}
        className="grid flex-none grid-cols-[repeat(14,8px)] gap-0.5 @max-[36rem]:hidden"
      >
        {tones.map((tone, i) => (
          <i key={i} className={`h-2 w-2 rounded-xs ${STRIP_TONE[tone]}`} />
        ))}
      </span>
      {tip.bubble}
    </>
  );
}

function runningLabel(progress: ProgressState, t: T): string {
  if (progress.phase === "restore") return t("backups.status.restoring");
  if (progress.stage === "dbdump") {
    return t("dbdump.progress").replace("{bytes}", humanBytes(progress.bytes ?? 0));
  }
  return t("backups.status.running").replace("{pct}", String(Math.round(progress.percent)));
}

function loudAnomalies(anomaly: AnomalyItem | undefined): number {
  if (!anomaly) return 0;
  const open = itemOpenCounts(anomaly);
  return open.critical + open.warning;
}

/** Everything a row knows besides its status, as sentences for the bubble
 *  beside its name. */
function rowFacts({
  item,
  running,
  anomaly,
  anomalyEnabled,
  scheduleSentence,
  t,
  lang,
}: {
  item: BackupItem;
  running: boolean;
  anomaly: AnomalyItem | undefined;
  anomalyEnabled: boolean;
  scheduleSentence: (item: BackupItem) => ScheduleSentence | null;
  t: T;
  lang: string;
}): string[] {
  const facts: string[] = [];
  const gone = isGone(item);
  if (!gone && item.kind === "container" && ["exited", "created", "dead"].includes(item.state ?? "")) {
    facts.push(t("backups.fact.stopped"));
  }
  if (!gone && item.kind === "vm" && /^shut ?off$/.test(item.state ?? "")) facts.push(t("backups.fact.vmOff"));
  if (running || gone) return facts;

  const state = protection(item);
  const failed = item.lastRunStatus === "failed";
  if (failed && state === "ok") {
    facts.push(`${t("backups.fact.lastGood").replace("{when}", formatRecent(item.lastBackup, lang))}.`);
  }
  if (failed && loudAnomalies(anomaly) > 0) facts.push(t("backups.fact.anomalyOpen"));
  if (state === "pending") facts.push(`${t("backups.fact.nextRun")}.`);

  const learning = anomaly?.learning;
  if (state === "ok" && anomalyEnabled && anomaly?.scheduled && learning && !learning.noData && learning.samples < learning.needed) {
    facts.push(`${anomalyLearningText(t, learning.samples, learning.needed, false)}.`);
  }

  if (!inSchedule(item)) {
    facts.push(`${t("schedule.paused")}.`);
    return facts;
  }
  // The schedule every entry follows unless something says otherwise is not
  // worth a bubble on every row.
  const schedule = item.effectiveSchedule.kind === "everything" ? null : scheduleSentence(item);
  if (schedule) facts.push(`${schedule.text}.`);
  return facts;
}

function Status({ children, tone }: { children: ReactNode; tone: "fail" | "warn" | "neutral" }) {
  return (
    <Badge tone={tone} size="large" wrap className={`text-center ${STATUS_WIDTH}`}>
      {children}
    </Badge>
  );
}

/**
 * BackupRow is one entry of the Backups list: the glyph of its kind, its name
 * with what else there is to say about it in a bubble, its last runs and the
 * one state that matters most. The whole row opens the entry.
 */
export function BackupRow({
  item,
  name,
  tones,
  progress,
  anomaly,
  anomalyEnabled,
  scheduleSentence,
  t,
  lang,
}: {
  item: BackupItem;
  name: string;
  tones: RunTone[];
  progress: ProgressState | undefined;
  anomaly: AnomalyItem | undefined;
  anomalyEnabled: boolean;
  scheduleSentence: (item: BackupItem) => ScheduleSentence | null;
  t: T;
  lang: string;
}) {
  const navigate = useNavigate();
  const target = entryTarget(item);
  const running = runsNow(progress);
  const facts = rowFacts({ item, running, anomaly, anomalyEnabled, scheduleSentence, t, lang });
  const status = rowStatus(item, loudAnomalies(anomaly));
  const dim = isGone(item) || item.kindDisabled;

  let cell: ReactNode;
  if (running) {
    cell = (
      <span className={`flex items-center gap-1.5 ${STATUS_WIDTH}`} onClick={keepInRow}>
        <span className="flex min-w-0 flex-1 flex-col gap-1">
          <span className="flex items-center gap-2 text-dense tabular-nums text-carbon-textSub">
            <span
              className="inline-block h-2.5 w-2.5 shrink-0 animate-spin rounded-full border-2"
              style={{ borderColor: "var(--accent)", borderTopColor: "transparent" }}
            />
            <span className="min-w-0 leading-tight wrap-break-word">{runningLabel(progress, t)}</span>
          </span>
          <ProgressBar percent={progress.percent} active inline />
        </span>
        {progress.active && progress.phase !== "restore" && (
          <BackupCancelButton cancelKey={itemProgressKey(item)} name={name} t={t} compact />
        )}
      </span>
    );
  } else if (status.is === "failed") {
    cell = <Status tone="fail">{t("run.statusFailed")}</Status>;
  } else if (status.is === "anomalies" && anomaly) {
    cell = (
      <Link to={`/anomalies?scope=item:${encodeURIComponent(anomaly.targetId)}`} onClick={keepInRow}>
        <Status tone="warn">
          <span className="flex [&_svg]:h-3 [&_svg]:w-3">
            <IconAnomalies />
          </span>
          {status.count === 1 ? t("anomaly.runBadge") : t("backups.status.anomalies", status.count)}
        </Status>
      </Link>
    );
  } else if (status.is === "notProtected") {
    cell = <Status tone="warn">{t("backups.status.notProtected")}</Status>;
  } else if (status.is === "waiting") {
    cell = <Status tone="neutral">{t("backups.status.waiting")}</Status>;
  } else {
    cell = (
      <Status tone="neutral">
        {status.is === "last" && status.quiet && <span className="h-[7px] w-[7px] flex-none rounded-full bg-statusOkSolid" />}
        <span className="tabular-nums">
          {item.lastBackup > 0 ? formatRecent(item.lastBackup, lang) : t("containers.never")}
        </span>
      </Status>
    );
  }

  return (
    <li
      data-testid="backup-row"
      onClick={() => navigate(target)}
      style={hueVars(kindHue(item.kind)) as CSSProperties}
      className={`glim-hue cursor-pointer rounded-control hover:bg-carbon-hover ${ROW}${dim ? " opacity-55" : ""}`}
    >
      <span className={`glim-row-glyph ${GLYPH_TILE}`}>
        <KindGlyph kind={item.kind} />
      </span>
      <span className="flex min-w-0 flex-1 items-center gap-2">
        <Link to={target} onClick={keepInRow} className={NAME}>
          {name}
        </Link>
        {facts.length > 0 && (
          <span className="flex" onClick={keepInRow}>
            <InfoBubble tip={facts.join(" ")} />
          </span>
        )}
      </span>
      <span className="ms-auto flex flex-none items-center gap-2">
        <RunStrip tones={tones} t={t} />
        {cell}
      </span>
    </li>
  );
}

/**
 * SelfRow is BombVault's own container. It stands in the list so nobody looks
 * for it, and says why it has no backup instead of leading anywhere.
 */
export function SelfRow({ item, t }: { item: BackupItem; t: T }) {
  return (
    <li data-testid="backup-self-row" className={`${ROW} opacity-70`}>
      <span className={`${GLYPH_TILE} text-carbon-textSub`}>
        <KindGlyph kind={item.kind} />
      </span>
      <span className={`flex-1 ${NAME}`}>{item.name}</span>
      <span className="ms-auto flex flex-none items-center gap-2">
        <Badge tone="neutral" size="large" className="whitespace-nowrap">
          {t("backups.status.self")}
          <InfoBubble tip={t("backups.selfNote")} />
        </Badge>
      </span>
    </li>
  );
}
