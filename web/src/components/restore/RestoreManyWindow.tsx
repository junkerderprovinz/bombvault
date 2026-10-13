import { useEffect, useMemo, useRef, useState, type CSSProperties, type ReactNode } from "react";

import { flashDownloadURL, getTimeline, type BackupItem, type TimelinePlace, type TimelineRow } from "../../lib/api";
import { hueVars } from "../../lib/appearance";
import { ENTRY_KIND } from "../../lib/backupEntry";
import { isGone, itemProgressKey } from "../../lib/backupList";
import { fireAndWaitRun } from "../../lib/backupWatch";
import { rowTime } from "../../lib/entry";
import { useT } from "../../lib/i18n";
import { useProgress } from "../../lib/progress";
import { formatRecent } from "../../lib/reltime";
import { commonTimes, downloadsOnly, rowAt, runTarget, snapshotOf, whereFor, type ManyWhere } from "../../lib/restoreMany";
import { RunReasonText } from "../../lib/runReason";
import { secretsSentence } from "../../lib/secretsRefusal";
import { useConfirm } from "../../lib/useConfirm";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { ToggleRow } from "../../pages/settings/shared";
import { entryName, KindGlyph, kindHue } from "../backups/kinds";
import { Button } from "../Button";
import { CheckDraw } from "../CheckDraw";
import { IconPause } from "../glyphs";
import { InfoBubble } from "../InfoBubble";
import { IconClose, IconDownload, IconRestore } from "../navGlyphs";
import { ProgressBar } from "../ProgressBar";
import { Selector } from "../Selector";
import { SelectField } from "../SelectField";
import { startRestore } from "./restoreJob";
import { WhereOption } from "./RestoreWindow";

interface Timeline {
  places: TimelinePlace[];
  rows: TimelineRow[];
  error?: string;
}

type Outcome = { is: "waiting" } | { is: "running" } | { is: "done" } | { is: "failed"; error: string } | { is: "skipped" };

const NEWEST = "newest";

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section aria-label={title} className="flex flex-col gap-2">
      <h3 className="text-sm font-semibold text-carbon-textSub">{title}</h3>
      {children}
    </section>
  );
}

/** The backup an entry takes, and the snapshot and place it is read from. */
function backupOf(timeline: Timeline | undefined, at: number | null) {
  if (!timeline) return undefined;
  const row = rowAt(timeline.rows, at);
  const from = row && snapshotOf(row, timeline.places);
  return row && from ? { row, ...from } : null;
}

function EntryLine({
  item,
  timeline,
  at,
  where,
  outcome,
}: {
  item: BackupItem;
  timeline: Timeline | undefined;
  at: number | null;
  where: ManyWhere;
  outcome: Outcome | undefined;
}) {
  const { t, lang } = useT();
  const live = useProgress()[itemProgressKey(item)];
  const backup = backupOf(timeline, at);
  const flash = downloadsOnly(item);

  let line: string;
  if (timeline === undefined) line = t("common.loadingBackups");
  else if (timeline.error !== undefined) line = timeline.error || t("common.loadBackupsFailed");
  else if (!backup) line = t("restoreMany.noneBefore");
  else line = formatRecent(rowTime(backup.row), lang);
  const recreated = !flash && whereFor(item, where) !== where;

  return (
    <li
      data-testid="restore-many-entry"
      data-outcome={outcome?.is}
      style={hueVars(kindHue(item.kind)) as CSSProperties}
      className="glim-hue flex flex-wrap items-center gap-x-3 gap-y-2 py-2.5 first:pt-0 last:pb-0"
    >
      <span className="flex h-9 w-9 flex-none items-center justify-center rounded-control bg-carbon-surface3 text-accentText [&_svg]:h-[18px] [&_svg]:w-[18px]">
        <KindGlyph kind={item.kind} />
      </span>
      <span className="flex min-w-40 flex-1 flex-col gap-1">
        <span className="flex items-center gap-1.5 text-sm font-semibold text-carbon-text">
          <span className="min-w-0 wrap-anywhere">{entryName(item, t)}</span>
          {flash && <InfoBubble tip={t("restoreMany.flashTip")} />}
        </span>
        <span className={`text-xs ${backup === null ? "text-statusWarn" : "text-carbon-textMuted"}`}>
          {recreated ? `${line} · ${t("restoreMany.recreated")}` : line}
        </span>
        {outcome?.is === "running" && (
          <span className="block w-full max-w-xs">
            <ProgressBar percent={live?.percent ?? 0} active inline />
          </span>
        )}
        {outcome?.is === "failed" && (
          <span className="text-xs text-statusFail wrap-break-word">
            <RunReasonText reason={outcome.error} t={t} />
          </span>
        )}
      </span>
      {flash && backup ? (
        <Button
          label={t("tile.info.download")}
          labelKey={null}
          glyph={<IconDownload />}
          tone="neutral"
          onClick={() => {
            const a = document.createElement("a");
            a.href = flashDownloadURL(backup.snapshotId, backup.source);
            a.download = `flash-${backup.snapshotId.slice(0, 8)}.zip`;
            a.click();
          }}
        />
      ) : (
        outcome && <OutcomeMark outcome={outcome} />
      )}
    </li>
  );
}

function OutcomeMark({ outcome }: { outcome: Outcome }) {
  const { t } = useT();
  switch (outcome.is) {
    case "done":
      return (
        <span className="flex items-center gap-1.5 text-xs text-statusOk [&_svg]:h-3.5 [&_svg]:w-3.5">
          <CheckDraw />
          {t("restoreMany.done")}
        </span>
      );
    case "failed":
      return <span className="text-xs text-statusFail">{t("verdict.failed")}</span>;
    case "skipped":
      return <span className="text-xs text-carbon-textMuted">{t("restoreMany.skipped")}</span>;
    case "running":
      return <span className="text-xs text-accentText">{t("restoreMany.running")}</span>;
    default:
      return <span className="text-xs text-carbon-textMuted">{t("restoreMany.waiting")}</span>;
  }
}

/**
 * RestoreManyWindow restores the picked entries one after the other, since the
 * server restores one entry at a time. Every entry takes its newest backup or
 * its newest from one moment, and next to the original is the default. A
 * paired phone reaches the same routes as the window for one entry, so the
 * server holds it to the same rules.
 */
export function RestoreManyWindow({ items, onClose }: { items: BackupItem[]; onClose: () => void }) {
  const { t, lang } = useT();
  const { confirm, confirmDialog } = useConfirm();
  const [timelines, setTimelines] = useState<Record<string, Timeline>>({});
  const [moment, setMoment] = useState<string>(NEWEST);
  const [where, setWhere] = useState<ManyWhere>("beside");
  const [backupFirst, setBackupFirst] = useState(true);
  const [outcomes, setOutcomes] = useState<Record<string, Outcome> | null>(null);
  const [running, setRunning] = useState(false);
  const stopping = useRef(false);
  const [stopAsked, setStopAsked] = useState(false);
  const gone = useRef(false);
  useEffect(() => {
    gone.current = false;
    return () => {
      gone.current = true;
    };
  }, []);

  useEffect(() => {
    for (const item of items) {
      getTimeline(ENTRY_KIND[item.kind].timelineDomain, item.key)
        .then((res) => (res.ok ? { places: res.places ?? [], rows: res.rows ?? [] } : { places: [], rows: [], error: res.error ?? "" }))
        .catch((err: unknown) => ({ places: [], rows: [], error: err instanceof Error ? err.message : "" }))
        .then((timeline) => {
          if (!gone.current) setTimelines((was) => ({ ...was, [itemProgressKey(item)]: timeline }));
        });
    }
  }, [items]);

  const times = useMemo(() => commonTimes(Object.values(timelines).map((tl) => tl.rows)), [timelines]);
  const at = moment === NEWEST ? null : Number(moment);
  const queue = items.filter((item) => !downloadsOnly(item));
  const ready = queue.filter((item) => backupOf(timelines[itemProgressKey(item)], at));
  const reading = items.some((item) => timelines[itemProgressKey(item)] === undefined);

  async function run() {
    if (where === "replace" && !(await confirm(t("restoreMany.replaceConfirm", ready.length), { confirmKey: "snapshots.restore" }))) {
      return;
    }
    const start: Record<string, Outcome> = {};
    for (const item of queue) start[itemProgressKey(item)] = ready.includes(item) ? { is: "waiting" } : { is: "skipped" };
    setOutcomes(start);
    setRunning(true);
    stopping.current = false;
    const mark = (item: BackupItem, outcome: Outcome) => setOutcomes((was) => ({ ...was, [itemProgressKey(item)]: outcome }));

    for (const item of ready) {
      if (stopping.current) break;
      const backup = backupOf(timelines[itemProgressKey(item)], at);
      if (!backup) continue;
      const target = whereFor(item, where);
      mark(item, { is: "running" });
      const res = await fireAndWaitRun({
        kind: "restore",
        matchRun: (r) => r.domain === item.kind && r.target === runTarget(item),
        start: async () => {
          const answer = await startRestore({
            kind: item.kind,
            id: item.key,
            snapshotId: backup.snapshotId,
            source: backup.source,
            what: "all",
            where: target,
            paths: [],
            folder: "",
            leaveStopped: false,
            withoutRuntime: false,
            backupFirst: where === "replace" && !isGone(item) && backupFirst,
          });
          const refusal = secretsSentence(t, answer.code);
          return refusal && !answer.ok ? { ...answer, error: refusal } : answer;
        },
        t,
      });
      if (gone.current) return;
      mark(item, res.ok ? { is: "done" } : { is: "failed", error: res.error ?? "" });
    }
    setRunning(false);
  }

  const finished = outcomes !== null && !running;
  const results = outcomes ? Object.values(outcomes) : [];
  const ok = results.filter((o) => o.is === "done").length;
  const failed = results.filter((o) => o.is === "failed").length;
  const left = results.filter((o) => o.is === "waiting").length;

  const footer = running ? (
    <Button
      label={t("restoreMany.stop")}
      labelKey={null}
      glyph={<IconPause />}
      tone="subtle"
      disabled={stopAsked}
      hint={t("restoreMany.stopHint")}
      onClick={() => {
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
        disabled={reading || ready.length === 0}
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
      onClose={running ? () => undefined : onClose}
    >
      {outcomes === null && (
        <>
          <Section title={t("restoreMany.which")}>
            <Selector
              items={[
                { id: NEWEST, label: t("restoreMany.newest") },
                { id: "at", label: t("restoreMany.at") },
              ]}
              label={t("restoreMany.which")}
              select="one"
              fit
              active={moment === NEWEST ? NEWEST : "at"}
              onChange={(id) => setMoment(id === NEWEST ? NEWEST : String(times[0] ?? ""))}
            />
            {moment !== NEWEST && times.length > 0 && (
              <div className="flex flex-col gap-1.5">
                <span className="flex items-center gap-1 text-xs text-carbon-textSub">
                  {t("restoreMany.atLabel")}
                  <InfoBubble tip={t("restoreMany.atHint")} />
                </span>
                <SelectField
                  value={moment}
                  onChange={setMoment}
                  label={t("restoreMany.atLabel")}
                  options={times.map((time) => ({
                    value: String(time),
                    label: new Date(time * 1000).toLocaleString(lang, { dateStyle: "medium", timeStyle: "short" }),
                  }))}
                  className="h-10 w-full max-w-sm rounded-control bg-carbon-surface2 px-3 text-sm text-carbon-text glim-field-focus"
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
        {finished && (
          <p role="status" className={`text-sm ${failed > 0 ? "text-statusWarn" : "text-statusOk"}`}>
            {t("restoreMany.result").replace("{ok}", String(ok)).replace("{fail}", String(failed))}
            {left > 0 && ` ${t("restoreMany.notStarted", left)}`}
          </p>
        )}
        <ul className="flex flex-col divide-y divide-carbon-border/60 rounded-card bg-carbon-surface2 px-3 py-2.5">
          {items.map((item) => (
            <EntryLine
              key={itemProgressKey(item)}
              item={item}
              timeline={timelines[itemProgressKey(item)]}
              at={at}
              where={where}
              outcome={outcomes?.[itemProgressKey(item)]}
            />
          ))}
        </ul>
      </Section>
      {confirmDialog}
    </InstanceWindow>
  );
}
