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
import { runReason } from "../../lib/runReason";
import { useLongPress } from "../../lib/useLongPress";
import { useTipBubble } from "../../lib/useTipBubble";
import { BackupCancelButton } from "../BackupCancelButton";
import { Badge } from "../Badge";
import { changeLines } from "../ContainerChangeNotice";
import type { ScheduleSentence } from "../EffectiveScheduleLine";
import { InfoBubble } from "../InfoBubble";
import { IconAnomalies } from "../navGlyphs";
import { ProgressBar } from "../ProgressBar";
import { TAKEOVER_REASONS } from "../RenameTakeoverRow";
import { KindGlyph, kindHue } from "./kinds";
import { SelectMark } from "./SelectMark";

type T = (key: TranslationKey, n?: number) => string;

// Where the list has room for the run strips, its rows share their columns:
// the widest status sets the width of them all, so every status stays on one
// line in any language and the strips before them line up. In a narrower
// list each row is a line of its own and a long status wraps. LIST_SPAN is
// what a sub-list, a row or a line across the list takes the columns with.
export const LIST_SPAN = "@min-[36rem]:col-span-full @min-[36rem]:grid @min-[36rem]:grid-cols-subgrid";
const ROW = `flex min-h-[50px] items-center gap-3 border-t border-carbon-border px-1.5 py-2 first:border-t-0 ${LIST_SPAN}`;
const GLYPH_TILE =
  "flex h-[34px] w-[34px] flex-none items-center justify-center rounded-control bg-carbon-surface2 [&>svg]:h-[18px] [&>svg]:w-[18px]";
const STATUS = "@min-[36rem]:w-full @min-[36rem]:whitespace-nowrap @max-[36rem]:max-w-[9.5rem]";
const NAME =
  "min-w-0 truncate text-sm font-semibold text-carbon-text @max-[36rem]:whitespace-normal @max-[36rem]:wrap-anywhere";
const BEFORE_STATUS = "flex flex-none items-center gap-2 @max-[36rem]:hidden";
const IP_COLUMN = "min-w-[7.5rem] flex-none text-end text-xs tabular-nums text-carbon-textMuted";

/** The columns of the list, which its sub-lists and their rows pass on. A
 *  list whose rows are being picked has one more for their marks. */
export function listColumns(selecting: boolean): string {
  return selecting
    ? "@min-[36rem]:grid @min-[36rem]:grid-cols-[auto_auto_minmax(0,1fr)_auto_minmax(9.5rem,max-content)]"
    : "@min-[36rem]:grid @min-[36rem]:grid-cols-[auto_minmax(0,1fr)_auto_minmax(9.5rem,max-content)]";
}

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
        className="grid flex-none grid-cols-[repeat(14,8px)] gap-0.5"
      >
        {tones.map((tone, i) => (
          <i key={i} className={`h-2 w-2 rounded-xs ${STRIP_TONE[tone]}`} />
        ))}
      </span>
      {tip.bubble}
    </>
  );
}

export function runningLabel(progress: ProgressState, t: T): string {
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

interface RowFacts {
  sentences: string[];
  /** A fact that asks for a look colours the bubble: a container that changed
   *  since its backup, or an entry that looks like a renamed one. */
  tone: "warn" | "accent" | "plain";
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
}): RowFacts {
  const facts: string[] = [];
  const gone = isGone(item);
  const changes = gone ? [] : changeLines(item.changedSinceBackup, t);
  const tone = changes.length > 0 ? "warn" : item.renameFrom ? "accent" : "plain";

  if (item.renameFrom) {
    const reason = TAKEOVER_REASONS[item.renameReason ?? ""];
    const looksLike = t("takeover.looksLike").replace("{old}", item.renameFrom);
    facts.push(`${reason ? `${looksLike}: ${t(reason)}` : looksLike}. ${t("backups.fact.takeOver")}`);
  }
  if (item.formerNames?.length) {
    facts.push(t("backups.fact.renamed").replace("{names}", item.formerNames.join(", ")));
  }
  if (changes.length > 0) facts.push(`${t("changeNotice.badge")}: ${changes.join(", ")}.`);
  if (!gone && item.kind === "container" && ["exited", "created", "dead"].includes(item.state ?? "")) {
    facts.push(t("backups.fact.stopped"));
  }
  if (!gone && item.kind === "vm" && /^shut ?off$/.test(item.state ?? "")) facts.push(t("backups.fact.vmOff"));
  if (running || gone) return { sentences: facts, tone };

  const state = protection(item);
  const failed = item.lastRunStatus === "failed";
  if (failed && state === "ok") {
    facts.push(`${t("backups.fact.lastGood").replace("{when}", formatRecent(item.lastBackup, lang))}.`);
  }
  if (failed && item.lastRunError) facts.push(runReason(item.lastRunError, t));
  if (failed && loudAnomalies(anomaly) > 0) facts.push(t("backups.fact.anomalyOpen"));
  if (state === "pending" && !item.renameFrom) facts.push(`${t("backups.fact.nextRun")}.`);
  if (item.backupDeclined) facts.push(`${t("entry.reason.declined")}.`);

  const learning = anomaly?.learning;
  if (state === "ok" && anomalyEnabled && anomaly?.scheduled && learning && !learning.noData && learning.samples < learning.needed) {
    facts.push(`${anomalyLearningText(t, learning.samples, learning.needed, false)}.`);
  }

  if (!inSchedule(item)) {
    facts.push(`${t("schedule.paused")}.`);
    return { sentences: facts, tone };
  }
  // The schedule every entry follows unless something says otherwise is not
  // worth a bubble on every row.
  const schedule = item.effectiveSchedule.kind === "everything" ? null : scheduleSentence(item);
  if (schedule) facts.push(`${schedule.text}.`);
  return { sentences: facts, tone };
}

const BUBBLE_TONE: Record<RowFacts["tone"], string> = {
  warn: "text-statusWarn",
  accent: "text-accentText",
  plain: "",
};

function Status({ children, tone }: { children: ReactNode; tone: "fail" | "warn" | "neutral" }) {
  return (
    <Badge tone={tone} size="large" wrap className={`text-center ${STATUS}`}>
      {children}
    </Badge>
  );
}

/**
 * BackupRow is one entry of the Backups list: the glyph of its kind, its name
 * with what else there is to say about it in a bubble, its last runs and the
 * one state that matters most. A plain click opens the entry. The page says
 * through onClick when a click picks the row instead.
 */
export function BackupRow({
  item,
  name,
  tones,
  progress,
  anomaly,
  anomalyEnabled,
  scheduleSentence,
  picked,
  selecting,
  showIp,
  onClick,
  onPick,
  onMenu,
  onLongPress,
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
  picked: boolean;
  /** Rows are being picked, so every row shows its mark. */
  selecting: boolean;
  showIp: boolean;
  /** Answers true when the click picked rows and must not open the entry. */
  onClick: (e: MouseEvent) => boolean;
  onPick: (next: boolean) => void;
  onMenu: (e: MouseEvent) => void;
  onLongPress: () => void;
  t: T;
  lang: string;
}) {
  const navigate = useNavigate();
  const press = useLongPress(onLongPress);
  const target = entryTarget(item);
  const running = runsNow(progress);
  const facts = rowFacts({ item, running, anomaly, anomalyEnabled, scheduleSentence, t, lang });
  const status = rowStatus(item, loudAnomalies(anomaly));
  const dim = isGone(item) || item.kindDisabled || item.backupDeclined;

  let cell: ReactNode;
  if (running) {
    cell = (
      <span
        className="flex items-center gap-1.5 @min-[36rem]:w-0 @min-[36rem]:min-w-full @max-[36rem]:max-w-[9.5rem]"
        onClick={keepInRow}
      >
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
      <Link
        to={`/anomalies?scope=item:${encodeURIComponent(anomaly.targetId)}`}
        onClick={keepInRow}
        className="flex min-w-0 @min-[36rem]:w-full"
      >
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

  // The tap that ends a long press has already picked the row.
  const taken = (e: MouseEvent) => press.took() || onClick(e);
  const pickLabel = t("common.selectItem").replace("{name}", name);
  const mark = <SelectMark checked={picked} label={pickLabel} onChange={onPick} />;

  return (
    <li
      data-testid="backup-row"
      data-picked={picked || undefined}
      onClick={(e) => {
        if (!taken(e)) navigate(target);
      }}
      // Shift+click would otherwise mark the text between the two rows.
      onMouseDown={(e) => {
        if (e.shiftKey) e.preventDefault();
      }}
      onContextMenu={(e) => {
        e.preventDefault();
        if (!press.byTouch()) onMenu(e);
      }}
      {...press.handlers}
      style={hueVars(kindHue(item.kind)) as CSSProperties}
      className={`group glim-hue relative cursor-pointer rounded-control pointer-coarse:select-none pointer-coarse:[-webkit-touch-callout:none] ${ROW} ${
        picked
          ? "bg-accentSoft shadow-[inset_3px_0_0_var(--accent)] rtl:shadow-[inset_-3px_0_0_var(--accent)]"
          : "hover:bg-carbon-hover"
      }${dim ? " opacity-55" : ""}`}
    >
      {selecting && mark}
      <span className={`glim-row-glyph relative ${GLYPH_TILE}`}>
        <KindGlyph kind={item.kind} />
        {!selecting && (
          // Until a row is picked, the mark of the row under the pointer lies
          // on the corner of its glyph, as on an icon in a file manager. No
          // row moves when it appears, and the rest of the row still opens
          // the entry.
          <span className="absolute -start-1.5 -top-1.5 hidden rounded-full ring-2 ring-carbon-surface pointer-fine:group-hover:flex pointer-fine:group-focus-within:flex">
            <SelectMark checked={false} label={pickLabel} onChange={onPick} bare />
          </span>
        )}
      </span>
      <span className="flex min-w-0 flex-1 items-center gap-2">
        <Link
          to={target}
          onClick={(e) => {
            e.stopPropagation();
            if (taken(e)) e.preventDefault();
          }}
          className={NAME}
        >
          {name}
        </Link>
        {facts.sentences.length > 0 && (
          <span className={`flex ${BUBBLE_TONE[facts.tone]}`} onClick={keepInRow}>
            <InfoBubble tip={facts.sentences.join(" ")} onAccent={facts.tone !== "plain"} />
          </span>
        )}
      </span>
      <span className={BEFORE_STATUS}>
        {showIp && <span className={IP_COLUMN}>{item.ip}</span>}
        <RunStrip tones={tones} t={t} />
      </span>
      <span className="flex min-w-0 flex-none justify-end">{cell}</span>
    </li>
  );
}

/**
 * SelfRow is BombVault's own container. It stands in the list so nobody looks
 * for it, and says why it has no backup instead of leading anywhere.
 */
export function SelfRow({
  item,
  selecting,
  showIp,
  t,
}: {
  item: BackupItem;
  /** Keeps the name in line with the rows that show a mark. */
  selecting: boolean;
  showIp: boolean;
  t: T;
}) {
  return (
    <li data-testid="backup-self-row" className={`${ROW} opacity-70`}>
      {selecting && <span className="w-[18px] flex-none" />}
      <span className={`${GLYPH_TILE} text-carbon-textSub`}>
        <KindGlyph kind={item.kind} />
      </span>
      <span className={`flex-1 ${NAME}`}>{item.name}</span>
      <span className={BEFORE_STATUS}>{showIp && <span className={IP_COLUMN}>{item.ip}</span>}</span>
      <Badge tone="neutral" size="large" className="whitespace-nowrap @min-[36rem]:w-full">
        {t("backups.status.self")}
        <InfoBubble tip={t("backups.selfNote")} />
      </Badge>
    </li>
  );
}
