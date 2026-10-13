import { useCallback, useEffect, useMemo, useRef, useState, type CSSProperties, type ReactNode } from "react";

import {
  flashDownloadURL,
  getTimeline,
  getTimelinePlace,
  openedAsPairedPhone,
  type AnomalyView,
  type BackupItem,
  type TimelinePlace,
  type TimelineRow,
} from "../../lib/api";
import { hueVars } from "../../lib/appearance";
import { ENTRY_KIND } from "../../lib/backupEntry";
import { isGone, itemProgressKey } from "../../lib/backupList";
import { fireAndWaitRun } from "../../lib/backupWatch";
import { rowTime } from "../../lib/entry";
import { useT } from "../../lib/i18n";
import { useProgress } from "../../lib/progress";
import { formatRecent } from "../../lib/reltime";
import {
  clearNotStarted,
  commonTimes,
  downloadsOnly,
  holdRunLock,
  noteNotStarted,
  pickRow,
  runTarget,
  snapshotOf,
  whereFor,
  type ManyWhere,
} from "../../lib/restoreMany";
import { findingsOf, recommendationWhy, snapshotIds } from "../../lib/restorePlan";
import { isRuntimeRefusal, RunReasonText } from "../../lib/runReason";
import { secretsSentence } from "../../lib/secretsRefusal";
import { mergePlaceRows } from "../../lib/timeline";
import { useToast } from "../../lib/toast";
import { useOpenAnomalies } from "../../lib/useAnomalies";
import { useConfirm } from "../../lib/useConfirm";
import { restoreBlockReason, useRestoreCheck } from "../../lib/useRestoreCheck";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { ToggleRow } from "../../pages/settings/shared";
import { entryName, KindGlyph, kindHue } from "../backups/kinds";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { CheckDraw } from "../CheckDraw";
import { IconPause } from "../glyphs";
import { InfoBubble } from "../InfoBubble";
import { IconClose, IconDownload, IconRestore } from "../navGlyphs";
import { ProgressBar } from "../ProgressBar";
import { RestoreCancelButton } from "../RestoreCancelButton";
import { Selector } from "../Selector";
import { SelectField } from "../SelectField";
import { Toggle } from "../Toggle";
import { checkRequest, startRestore, type RestoreChoice } from "./restoreJob";
import { WhereOption } from "./RestoreWindow";

interface Timeline {
  places: TimelinePlace[];
  rows: TimelineRow[];
  error?: string;
}

type Outcome =
  | { is: "waiting" }
  | { is: "running" }
  | { is: "done" }
  | { is: "failed"; error: string }
  | { is: "cancelled" }
  | { is: "skipped" };

/** What the check of an entry says about starting it. */
type Verdict = "running" | "ready" | "blocked";

const RECOMMENDED = "recommended";
const AT = "at";

/** A second click this soon after the start lands on the button that took
 *  the start button's place, and is not meant for it. */
const DOUBLE_CLICK_MS = 500;

function refusedRuntime(outcome: Outcome | undefined): boolean {
  return outcome?.is === "failed" && isRuntimeRefusal(outcome.error);
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section aria-label={title} className="flex flex-col gap-2">
      <h3 className="text-sm font-semibold text-carbon-textSub">{title}</h3>
      {children}
    </section>
  );
}

/** readTimeline reads an entry's backups at its local places, and at the
 *  others only when those hold none, which is when the local copy is lost. */
async function readTimeline(item: BackupItem): Promise<Timeline> {
  const domain = ENTRY_KIND[item.kind].timelineDomain;
  const res = await getTimeline(domain, item.key);
  if (!res.ok) return { places: [], rows: [], error: res.error ?? "" };
  let places = res.places ?? [];
  let rows = res.rows ?? [];
  if (rows.length > 0) return { places, rows };
  for (const place of places.filter((p) => p.state === "unchecked" && p.enabled)) {
    const one = await getTimelinePlace(domain, item.key, place.place).catch(() => null);
    if (!one?.ok || !one.place) continue;
    const read = one.place;
    places = places.map((p) => (p.place === read.place ? read : p));
    rows = mergePlaceRows(rows, read.place, one.rows ?? []);
  }
  return { places, rows };
}

/** The backup an entry takes, the snapshot and place it is read from, and why. */
interface Plan {
  row: TimelineRow;
  snapshotId: string;
  source: string;
  finding?: AnomalyView;
  passedOver: boolean;
}

function planOf(timeline: Timeline | undefined, at: number | null, findings: AnomalyView[]): Plan | null | undefined {
  if (!timeline) return undefined;
  const { row, finding, passedOver } = pickRow(timeline.rows, at, findings);
  const from = row && snapshotOf(row, timeline.places);
  return row && from ? { row, ...from, finding, passedOver } : null;
}

function EntryLine({
  item,
  timeline,
  plan,
  flagged,
  where,
  backupFirst,
  outcome,
  recreate,
  onRecreate,
  onVerdict,
  cancelledRef,
  onWithoutRuntime,
}: {
  item: BackupItem;
  timeline: Timeline | undefined;
  plan: Plan | null | undefined;
  flagged: boolean;
  where: ManyWhere;
  backupFirst: boolean;
  outcome: Outcome | undefined;
  recreate: boolean;
  onRecreate: (yes: boolean) => void;
  onVerdict: (key: string, verdict: Verdict | null) => void;
  cancelledRef: { current: boolean };
  /** Restores this entry again without its GPU and runtime, offered once
   *  Docker refused it for them and nothing else runs. */
  onWithoutRuntime?: () => void;
}) {
  const { t, lang } = useT();
  const key = itemProgressKey(item);
  const live = useProgress()[key];
  const flash = downloadsOnly(item);
  const name = entryName(item, t);
  const target = whereFor(item, where);
  const gone = !flash && isGone(item);

  const choice: RestoreChoice | null =
    plan && !flash && outcome === undefined
      ? {
          kind: item.kind,
          id: item.key,
          snapshotId: plan.snapshotId,
          source: plan.source,
          what: "all",
          where: target,
          paths: [],
          folder: "",
          leaveStopped: false,
          withoutRuntime: false,
          backupFirst: target === "replace" && !isGone(item) && backupFirst,
        }
      : null;
  const check = useRestoreCheck(choice ? checkRequest(choice) : null);
  const verdict: Verdict | null =
    choice === null ? null : check.state.phase === "done" ? (check.ready ? "ready" : "blocked") : check.state.phase === "error" ? "blocked" : "running";
  useEffect(() => onVerdict(key, verdict), [key, verdict, onVerdict]);

  let line: string;
  if (timeline === undefined) line = t("common.loadingBackups");
  else if (timeline.error !== undefined) line = timeline.error || t("common.loadBackupsFailed");
  else if (!plan) line = t("restoreMany.noneBefore");
  else line = formatRecent(rowTime(plan.row), lang);

  const notes: { text: string; warn: boolean }[] = [];
  if (plan?.finding) notes.push({ text: recommendationWhy({ index: 0, finding: plan.finding }, t, lang), warn: true });
  if (plan?.passedOver) notes.push({ text: t("restoreMany.passedOver"), warn: false });
  if (outcome === undefined && verdict === "running") notes.push({ text: t("restoreCheck.waiting"), warn: false });
  const done = check.state.phase === "done" ? check.state.result : null;
  if (outcome === undefined && verdict === "blocked") {
    const failed = done?.checks?.find((c) => c.status === "fail");
    const why = check.state.phase === "error" ? t("restoreCheck.failed").replace("{error}", check.state.error) : restoreBlockReason(check, t);
    notes.push({ text: [why, failed?.detail].filter(Boolean).join(" "), warn: true });
  }
  if (outcome === undefined && done?.plan) {
    for (const shared of done.plan.shared ?? []) {
      notes.push({ text: t("restoreCheck.shared.line").replace("{path}", shared.path).replace("{names}", shared.containers.join(", ")), warn: true });
    }
  }
  const coverage = item.kind === "container" ? item.dbDataCoverage : undefined;
  if (outcome === undefined && plan && (coverage === "none" || coverage === "live")) {
    notes.push({ text: t(coverage === "none" ? "restoreWindow.dbNone" : "restoreWindow.dbLive"), warn: true });
  }
  const paired = openedAsPairedPhone();

  return (
    <li
      data-testid="restore-many-entry"
      data-outcome={outcome?.is}
      data-verdict={verdict ?? undefined}
      style={hueVars(kindHue(item.kind)) as CSSProperties}
      className="glim-hue flex flex-wrap items-center gap-x-3 gap-y-2 py-2.5 first:pt-0 last:pb-0"
    >
      <span className="flex h-9 w-9 flex-none items-center justify-center rounded-control bg-carbon-surface3 text-accentText [&_svg]:h-[18px] [&_svg]:w-[18px]">
        <KindGlyph kind={item.kind} />
      </span>
      <span className="flex min-w-40 flex-1 flex-col gap-1">
        <span className="flex flex-wrap items-center gap-1.5 text-sm font-semibold text-carbon-text">
          <span className="min-w-0 wrap-anywhere">{name}</span>
          {flash && <InfoBubble tip={t("restoreMany.flashTip")} />}
          {flagged && (
            <Badge tone="warn" size="small">
              {t("anomaly.snapshotFlagged")}
            </Badge>
          )}
        </span>
        <span className={`text-xs ${plan === null ? "text-statusWarn" : "text-carbon-textMuted"}`}>
          {gone ? `${line} · ${t("restoreMany.recreated")}` : line}
        </span>
        {notes.map((note) => (
          <span key={note.text} className={`text-xs wrap-break-word ${note.warn ? "text-statusWarn" : "text-carbon-textMuted"}`}>
            {note.text}
          </span>
        ))}
        {gone && plan && outcome === undefined && (
          <Toggle label={t("restoreMany.recreateYes")} checked={recreate} onChange={onRecreate} className="mt-1" />
        )}
        {outcome?.is === "running" && (
          <span className="flex w-full max-w-xs flex-col gap-2">
            <ProgressBar percent={live?.percent ?? 0} active inline accessibleName={`${name}: ${t("restoreMany.running")}`} />
            <RestoreCancelButton cancelKey={key} inPlace={target === "replace"} name={name} t={t} cancelledRef={cancelledRef} />
          </span>
        )}
        {outcome?.is === "failed" && (
          <span className="text-xs text-statusFail wrap-break-word">
            <RunReasonText reason={outcome.error} t={t} />
          </span>
        )}
        {flash && plan && paired && <span className="text-xs text-carbon-textMuted">{t("restoreMany.flashOwnLogin")}</span>}
      </span>
      {flash && plan && !paired ? (
        <Button
          label={t("tile.info.download")}
          labelKey={null}
          glyph={<IconDownload />}
          tone="neutral"
          onClick={() => {
            const a = document.createElement("a");
            a.href = flashDownloadURL(plan.snapshotId, plan.source);
            a.download = `flash-${plan.snapshotId.slice(0, 8)}.zip`;
            a.click();
          }}
        />
      ) : onWithoutRuntime ? (
        <Button
          label={t("restore.withoutRuntime")}
          labelKey="restore.withoutRuntime"
          tone="neutral"
          hint={t("restore.withoutRuntimeHint")}
          onClick={onWithoutRuntime}
          className="glim-btn-wrap"
        />
      ) : (
        outcome && <OutcomeMark outcome={outcome} />
      )}
    </li>
  );
}

function outcomeWord(outcome: Outcome, t: ReturnType<typeof useT>["t"]): string {
  switch (outcome.is) {
    case "done":
      return t("restoreMany.done");
    case "failed":
      return t("verdict.failed");
    case "cancelled":
      return t("run.statusCancelled");
    case "skipped":
      return t("restoreMany.skipped");
    case "running":
      return t("restoreMany.running");
    default:
      return t("restoreMany.waiting");
  }
}

function OutcomeMark({ outcome }: { outcome: Outcome }) {
  const { t } = useT();
  const word = outcomeWord(outcome, t);
  switch (outcome.is) {
    case "done":
      return (
        <span className="flex items-center gap-1.5 text-xs text-statusOk [&_svg]:h-3.5 [&_svg]:w-3.5">
          <CheckDraw />
          {word}
        </span>
      );
    case "failed":
      return <span className="text-xs text-statusFail">{word}</span>;
    case "running":
      return <span className="text-xs text-accentText">{word}</span>;
    default:
      return <span className="text-xs text-carbon-textMuted">{word}</span>;
  }
}

/**
 * RestoreManyWindow restores the picked entries one after the other, since the
 * server restores one entry at a time. Every entry takes the backup the window
 * for one entry recommends, or its newest from one moment, and is checked the
 * way that window checks it before anything starts. Next to the original is
 * the default, and an entry gone from the server is created again in its old
 * place only once its row says so. A paired phone reaches the same routes as
 * the window for one entry, so the server holds it to the same rules.
 */
export function RestoreManyWindow({ items, onClose }: { items: BackupItem[]; onClose: () => void }) {
  const { t, lang } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const { list: openFindings, flagged: flaggedIds } = useOpenAnomalies();
  const [timelines, setTimelines] = useState<Record<string, Timeline>>({});
  const [mode, setMode] = useState<typeof RECOMMENDED | typeof AT>(RECOMMENDED);
  const [momentPick, setMomentPick] = useState<number | null>(null);
  const [where, setWhere] = useState<ManyWhere>("beside");
  const [backupFirst, setBackupFirst] = useState(true);
  const [verdicts, setVerdicts] = useState<Record<string, Verdict>>({});
  const [recreate, setRecreate] = useState<Record<string, boolean>>({});
  const [outcomes, setOutcomes] = useState<Record<string, Outcome> | null>(null);
  const [running, setRunning] = useState(false);
  const [announcement, setAnnouncement] = useState("");
  const stopping = useRef(false);
  const [stopAsked, setStopAsked] = useState(false);
  const startedAt = useRef(0);
  const cancelAsked = useRef(false);
  const order = useRef<BackupItem[]>([]);
  const releaseLock = useRef<() => void>(() => undefined);
  const gone = useRef(false);
  useEffect(() => {
    gone.current = false;
    return () => {
      gone.current = true;
      releaseLock.current();
    };
  }, []);

  useEffect(() => {
    for (const item of items) {
      readTimeline(item)
        .catch((err: unknown) => ({ places: [], rows: [], error: err instanceof Error ? err.message : "" }))
        .then((timeline) => {
          if (!gone.current) setTimelines((was) => ({ ...was, [itemProgressKey(item)]: timeline }));
        });
    }
  }, [items]);

  // Closing the tab would drop the entries that have not started yet.
  useEffect(() => {
    if (!running) return;
    const warn = (e: BeforeUnloadEvent) => e.preventDefault();
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [running]);

  const onVerdict = useCallback((key: string, verdict: Verdict | null) => {
    setVerdicts((was) => {
      if ((was[key] ?? null) === verdict) return was;
      const next = { ...was };
      if (verdict === null) delete next[key];
      else next[key] = verdict;
      return next;
    });
  }, []);

  const times = useMemo(() => commonTimes(Object.values(timelines).map((tl) => tl.rows)), [timelines]);
  const at = mode === RECOMMENDED ? null : (momentPick ?? times[0] ?? 0);
  const findingsFor = (item: BackupItem) => findingsOf(openFindings, item.kind, item.key);
  const plans: Record<string, Plan | null | undefined> = {};
  for (const item of items) plans[itemProgressKey(item)] = planOf(timelines[itemProgressKey(item)], at, findingsFor(item));

  const queue = items.filter((item) => !downloadsOnly(item));
  const startable = (item: BackupItem) => {
    const key = itemProgressKey(item);
    if (!plans[key] || verdicts[key] !== "ready") return false;
    return !isGone(item) || recreate[key] === true;
  };
  const ready = queue.filter(startable);
  const reading = items.some((item) => timelines[itemProgressKey(item)] === undefined);
  const checking = queue.some((item) => plans[itemProgressKey(item)] && verdicts[itemProgressKey(item)] !== "ready" && verdicts[itemProgressKey(item)] !== "blocked");

  function mark(item: BackupItem, outcome: Outcome) {
    setOutcomes((was) => ({ ...was, [itemProgressKey(item)]: outcome }));
    const n = order.current.indexOf(item);
    if (n === -1) return;
    setAnnouncement(
      t("restoreMany.announce")
        .replace("{n}", String(n + 1))
        .replace("{total}", String(order.current.length))
        .replace("{name}", entryName(item, t))
        .replace("{state}", outcomeWord(outcome, t))
    );
  }

  // One entry from its backup and to the place the window was set to. Returns
  // false once the window is gone.
  async function restoreOne(item: BackupItem, withoutRuntime: boolean): Promise<boolean> {
    const plan = plans[itemProgressKey(item)];
    if (!plan) return true;
    const target = whereFor(item, where);
    cancelAsked.current = false;
    mark(item, { is: "running" });
    const res = await fireAndWaitRun({
      kind: "restore",
      matchRun: (r) => r.domain === item.kind && r.target === runTarget(item),
      progressKey: itemProgressKey(item),
      start: async () => {
        const answer = await startRestore({
          kind: item.kind,
          id: item.key,
          snapshotId: plan.snapshotId,
          source: plan.source,
          what: "all",
          where: target,
          paths: [],
          folder: "",
          leaveStopped: false,
          withoutRuntime,
          // The first attempt already kept today's state.
          backupFirst: !withoutRuntime && target === "replace" && !isGone(item) && backupFirst,
        });
        const refusal = secretsSentence(t, answer.code);
        return refusal && !answer.ok ? { ...answer, error: refusal } : answer;
      },
      t,
    });
    if (gone.current) return false;
    if (res.cancelled || cancelAsked.current) {
      mark(item, { is: "cancelled" });
      stopping.current = true;
    } else mark(item, res.ok ? { is: "done" } : { is: "failed", error: res.error ?? "" });
    return true;
  }

  function begin() {
    stopping.current = false;
    setStopAsked(false);
    startedAt.current = Date.now();
    releaseLock.current = holdRunLock();
    setRunning(true);
  }

  function end() {
    clearNotStarted();
    releaseLock.current();
    setRunning(false);
  }

  async function run() {
    const originals = ready.filter((item) => whereFor(item, where) === "replace" && !isGone(item)).length;
    if (originals > 0 && !(await confirm(t("restoreMany.replaceConfirm", originals), { confirmKey: "snapshots.restore" }))) {
      return;
    }
    const start: Record<string, Outcome> = {};
    for (const item of queue) start[itemProgressKey(item)] = ready.includes(item) ? { is: "waiting" } : { is: "skipped" };
    setOutcomes(start);
    order.current = ready;
    begin();

    for (const [i, item] of ready.entries()) {
      if (stopping.current) break;
      noteNotStarted(ready.slice(i + 1).map((next) => entryName(next, t)));
      if (!(await restoreOne(item, false))) return;
    }
    end();
  }

  async function withoutRuntime(item: BackupItem) {
    order.current = [item];
    begin();
    if (await restoreOne(item, true)) end();
  }

  function held() {
    push(t("restoreMany.held").replace("{stop}", t("restoreMany.stop")), "warn");
    return false;
  }

  const finished = outcomes !== null && !running;
  const results = outcomes ? Object.values(outcomes) : [];
  const ok = results.filter((o) => o.is === "done").length;
  const failed = results.filter((o) => o.is === "failed").length;
  const cancelled = results.filter((o) => o.is === "cancelled").length;
  const left = results.filter((o) => o.is === "waiting").length;
  const summary = [
    t("restoreMany.result").replace("{ok}", String(ok)).replace("{fail}", String(failed)),
    cancelled > 0 ? t("restoreMany.cancelledCount", cancelled) : "",
    left > 0 ? t("restoreMany.notStarted", left) : "",
  ]
    .filter(Boolean)
    .join(" ");

  const footer = running ? (
    <Button
      label={t("restoreMany.stop")}
      labelKey={null}
      glyph={<IconPause />}
      tone="subtle"
      disabled={stopAsked}
      hint={t("restoreMany.stopHint")}
      onClick={() => {
        if (Date.now() - startedAt.current < DOUBLE_CLICK_MS) return;
        stopping.current = true;
        setStopAsked(true);
      }}
    />
  ) : finished ? (
    <Button label={t("common.close")} labelKey="common.close" glyph={<IconClose />} tone="accent" onClick={onClose} />
  ) : (
    <>
      <Button label={t("common.cancel")} labelKey="common.cancel" tone="subtle" onClick={onClose} />
      <Button
        label={`${t("snapshots.restore")} · ${ready.length}`}
        labelKey="snapshots.restore"
        glyph={<IconRestore />}
        tone="accent"
        disabled={reading || checking || ready.length === 0}
        hint={checking ? t("restoreCheck.waiting") : undefined}
        onClick={() => void run()}
      />
    </>
  );

  return (
    <InstanceWindow
      title={t("restoreMany.title", items.length)}
      facts={t("restoreMany.hint")}
      footer={footer}
      // The restores are started from here one by one, so the window stays
      // while they run.
      onClose={running ? held : onClose}
    >
      {outcomes === null && (
        <>
          <Section title={t("restoreMany.which")}>
            <Selector
              items={[
                { id: RECOMMENDED, label: t("restoreMany.recommended"), tip: t("restoreMany.recommendedHint") },
                { id: AT, label: t("restoreMany.at") },
              ]}
              label={t("restoreMany.which")}
              select="one"
              fit
              active={mode}
              onChange={(id) => setMode(id === AT ? AT : RECOMMENDED)}
            />
            {mode === AT && times.length > 0 && (
              <div className="flex flex-col gap-1.5">
                <span className="flex items-center gap-1 text-xs text-carbon-textSub">
                  {t("restoreMany.atLabel")}
                  <InfoBubble tip={t("restoreMany.atHint")} />
                </span>
                <SelectField
                  value={String(at)}
                  onChange={(value) => setMomentPick(Number(value))}
                  label={t("restoreMany.atLabel")}
                  options={times.map((time) => ({
                    value: String(time),
                    label: new Date(time * 1000).toLocaleString(lang, { dateStyle: "medium", timeStyle: "short" }),
                  }))}
                  className="h-10 w-full rounded-control bg-carbon-surface2 px-3 text-sm text-carbon-text glim-field-focus"
                />
              </div>
            )}
          </Section>

          <Section title={t("restoreWindow.where")}>
            <div role="radiogroup" aria-label={t("restoreWindow.where")} className="flex flex-col gap-2">
              <WhereOption
                where="beside"
                on={where === "beside"}
                title={t("restoreWindow.beside")}
                sub={t("restoreWindow.untouched")}
                kept={false}
                disabled={false}
                onPick={() => setWhere("beside")}
              />
              <WhereOption
                where="replace"
                on={where === "replace"}
                title={t("restoreWindow.replace")}
                sub={t(backupFirst ? "restoreWindow.replaceSubBackup" : "restoreWindow.replaceSub")}
                kept={backupFirst}
                disabled={false}
                onPick={() => setWhere("replace")}
              />
            </div>
            {where === "replace" && (
              <ToggleRow
                label={t("restoreWindow.backupFirst")}
                hint={t("restoreWindow.backupFirstHint")}
                checked={backupFirst}
                onChange={setBackupFirst}
              />
            )}
          </Section>
        </>
      )}

      <Section title={t("restoreMany.entries")}>
        <p role="status" aria-live="polite" className="sr-only">
          {finished ? summary : announcement}
        </p>
        {finished && (
          <p aria-hidden="true" className={`text-sm ${failed > 0 || cancelled > 0 ? "text-statusWarn" : "text-statusOk"}`}>
            {summary}
          </p>
        )}
        <ul className="flex flex-col divide-y divide-carbon-border/60 rounded-card bg-carbon-surface2 px-3 py-2.5">
          {items.map((item) => {
            const key = itemProgressKey(item);
            const plan = plans[key];
            return (
              <EntryLine
                key={key}
                item={item}
                timeline={timelines[key]}
                plan={plan}
                flagged={plan ? snapshotIds(plan.row).some((id) => flaggedIds.has(id)) : false}
                where={where}
                backupFirst={backupFirst}
                outcome={outcomes?.[key]}
                recreate={recreate[key] === true}
                onRecreate={(yes) => setRecreate((was) => ({ ...was, [key]: yes }))}
                onVerdict={onVerdict}
                cancelledRef={cancelAsked}
                onWithoutRuntime={
                  !running && item.kind === "container" && refusedRuntime(outcomes?.[key]) ? () => void withoutRuntime(item) : undefined
                }
              />
            );
          })}
        </ul>
      </Section>
      {confirmDialog}
    </InstanceWindow>
  );
}
