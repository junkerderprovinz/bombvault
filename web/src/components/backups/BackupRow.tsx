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
  waitingLabel,
  type RunTone,
} from "../../lib/backupList";
import { humanBytes } from "../../lib/forecast";
import type { TranslationKey } from "../../lib/i18n";
import type { ProgressState } from "../../lib/progress";
import { formatRecent, relativeTime } from "../../lib/reltime";
import { runReason } from "../../lib/runReason";
import { useLongPress } from "../../lib/useLongPress";
import { useTipBubble } from "../../lib/useTipBubble";
import { zfsReplicaFailure } from "../../lib/zfsCodes";
import { BackupCancelButton } from "../BackupCancelButton";
import { Badge } from "../Badge";
import { changeLines } from "../ContainerChangeNotice";
import type { ScheduleSentence } from "../EffectiveScheduleLine";
import { InfoBubble } from "../InfoBubble";
import { IconAnomalies, IconCheck } from "../navGlyphs";
import { ProgressBar } from "../ProgressBar";
import { TAKEOVER_REASONS } from "../RenameTakeoverRow";
import { KindGlyph, kindHue } from "./kinds";

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
  "min-w-0 truncate text-sm font-semibold text-carbon-text @max-[36rem]:whitespace-normal @max-[36rem]:wrap-break-word";
// In a narrow list the name and the status share a line that wraps. The name
// asks for the room of all its words, so the status moves under it before the
// name has to break; a name wider than the row then breaks between words, and
// only a word wider than the row breaks inside. In a wide list the wrapper
// gives way and its parts take the list's columns.
const TEXT = "flex min-w-0 flex-1 flex-wrap items-center gap-x-3 gap-y-1.5 @min-[36rem]:contents";
const NAME_ROOM = "@max-[36rem]:basis-auto @max-[36rem]:w-max @max-[36rem]:max-w-full";
const BEFORE_STATUS = "flex flex-none items-center gap-2 @max-[36rem]:hidden";
const IP_COLUMN = "min-w-[7.5rem] flex-none text-end text-xs tabular-nums text-carbon-textMuted";

/** The columns of the list, which its sub-lists and their rows pass on. */
export const LIST_COLUMNS = "@min-[36rem]:grid @min-[36rem]:grid-cols-[auto_minmax(0,1fr)_auto_minmax(9.5rem,max-content)]";

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
  if (failed && item.lastBackup > 0) {
    facts.push(`${t("backups.fact.lastGood").replace("{when}", formatRecent(item.lastBackup, lang))}.`);
  }
  if (failed && item.lastRunError) facts.push(runReason(item.lastRunError, t));
  const replicaFailure = item.replica?.lastStatus === "failed" ? zfsReplicaFailure(t, item.replica.code) : null;
  if (replicaFailure) facts.push(replicaFailure);
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
  replicaProgress,
  anomaly,
  anomalyEnabled,
  scheduleSentence,
  picked,
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
  /** The item's replica run, which reports apart from its backup. */
  replicaProgress?: ProgressState;
  anomaly: AnomalyItem | undefined;
  anomalyEnabled: boolean;
  scheduleSentence: (item: BackupItem) => ScheduleSentence | null;
  picked: boolean;
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
  const replicating = !running && replicaProgress?.active === true;
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
  } else if (replicating) {
    cell = (
      <span className="flex min-w-0 flex-col gap-1 @min-[36rem]:w-0 @min-[36rem]:min-w-full @max-[36rem]:max-w-[9.5rem]">
        <span className="flex items-center gap-2 text-dense tabular-nums text-carbon-textSub">
          <span
            className="inline-block h-2.5 w-2.5 shrink-0 animate-spin rounded-full border-2"
            style={{ borderColor: "var(--accent)", borderTopColor: "transparent" }}
          />
          <span className="min-w-0 leading-tight wrap-break-word">{t("zfs.replica.state.running")}</span>
        </span>
        <ProgressBar percent={replicaProgress.percent} active inline />
      </span>
    );
  } else if (status.is === "failed") {
    cell = <Status tone="fail">{t("run.statusFailed")}</Status>;
  } else if (status.is === "replicaFailed") {
    cell = <Status tone="fail">{t("zfs.replica.state.failed")}</Status>;
  } else if (status.is === "replicated") {
    cell = (
      <Status tone="neutral">
        <span className="h-[7px] w-[7px] flex-none rounded-full bg-statusOkSolid" />
        <span className="tabular-nums">{t("zfs.replica.state.ok").replace("{when}", () => relativeTime(t, status.at))}</span>
      </Status>
    );
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
    cell = <Status tone="neutral">{t(waitingLabel(item))}</Status>;
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

  return (
    <li
      data-testid="backup-row"
      data-new="backups-row"
      // A ZFS entry with a replica is also the way to what is new about replicas.
      data-new-way={item.replica ? "entry zfs-replica" : "entry"}
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
      <span
        className={`glim-row-glyph ${GLYPH_TILE} has-focus-visible:outline-2 has-focus-visible:outline-offset-2 has-focus-visible:outline-(--focus-ring)`}
      >
        {picked ? <IconCheck /> : <KindGlyph kind={item.kind} />}
        {/* A mouse picks with Ctrl or Shift and a finger with a long press,
            so the row draws no mark. The switch is there for the keyboard
            and a screen reader, and its focus rings the glyph tile. */}
        <button
          type="button"
          role="checkbox"
          aria-checked={picked}
          aria-label={pickLabel}
          onClick={(e) => {
            e.stopPropagation();
            onPick(!picked);
          }}
          className="sr-only"
        />
      </span>
      <span className={TEXT}>
        <span className={`flex min-w-0 flex-1 items-center gap-2 ${NAME_ROOM}`}>
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
            <>
              <span className={`flex ${BUBBLE_TONE[facts.tone]}`} onClick={keepInRow}>
                <InfoBubble tip={facts.sentences.join(" ")} onAccent={facts.tone !== "plain"} />
              </span>
              {/* The bubble's text in the row under a mouse, which has room to
                  spare there. The bubble already names it to a screen reader. */}
              <span
                aria-hidden="true"
                data-testid="backup-row-facts"
                className={`hidden min-w-0 flex-1 truncate text-xs pointer-fine:group-hover:block @max-[36rem]:w-0 ${
                  facts.tone === "plain" ? "text-carbon-textMuted" : BUBBLE_TONE[facts.tone]
                }`}
              >
                {facts.sentences.join(" ")}
              </span>
            </>
          )}
        </span>
        <span className={BEFORE_STATUS}>
          {showIp && <span className={IP_COLUMN}>{item.ip}</span>}
          <RunStrip tones={tones} t={t} />
        </span>
        <span className="flex min-w-0 flex-none justify-end">{cell}</span>
      </span>
    </li>
  );
}

/**
 * SelfRow is BombVault's own container. It stands in the list so nobody looks
 * for it, and says why it has no backup instead of leading anywhere.
 */
export function SelfRow({
  item,
  showIp,
  t,
}: {
  item: BackupItem;
  showIp: boolean;
  t: T;
}) {
  return (
    <li data-testid="backup-self-row" className={`${ROW} opacity-70`}>
      <span className={`${GLYPH_TILE} text-carbon-textSub`}>
        <KindGlyph kind={item.kind} />
      </span>
      <span className={TEXT}>
        <span className={`flex-1 ${NAME} ${NAME_ROOM}`}>{item.name}</span>
        <span className={BEFORE_STATUS}>{showIp && <span className={IP_COLUMN}>{item.ip}</span>}</span>
        <Badge tone="neutral" size="large" className="whitespace-nowrap @min-[36rem]:w-full">
          {t("backups.status.self")}
          <InfoBubble tip={t("backups.selfNote")} />
        </Badge>
      </span>
    </li>
  );
}
