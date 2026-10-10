import { useEffect, useRef, useState, type CSSProperties } from "react";
import { listRuns, restoreStack, getStackDir } from "../../lib/api";
import type { Container, Run } from "../../lib/api";
import { IconTipButton } from "../IconTipButton";
import { useT } from "../../lib/i18n";
import { RestoreCancelButton } from "../RestoreCancelButton";
import { SourceToggle, isOffsiteSource, type RepoSource } from "../SourceToggle";
import { Button } from "../Button";
import { useProgress } from "../../lib/progress";
import { useConfirm } from "../../lib/useConfirm";
import { offsiteTargetLabel, useOffsiteTargets } from "../../lib/useOffsiteTargets";
import { useHostLabel } from "../../lib/useHostLabel";
import { placementErrorText } from "../../lib/placementCodes";
import { hueVars } from "../../lib/appearance";
import { useToast } from "../../lib/toast";
import { Toggle } from "../Toggle";
import { checkRestoreOnce, restoreBlockReason } from "../../lib/useRestoreCheck";
import { RestoreCheckPanel } from "../restore/RestoreCheckPanel";
import { RuntimeRetry } from "../restore/RuntimeRetry";
import { isRuntimeRefusal } from "../../lib/runReason";

type T = ReturnType<typeof useT>["t"];

export interface StackGroup {
  project: string;
  members: Container[];
}

/** Joins names the way the reader's language joins a list, "a, b and c". */
function listSeparated(lang: string, names: string[]): string {
  return new Intl.ListFormat(lang, { style: "long", type: "conjunction" }).format(names);
}

// Grace after the last member goes inactive before a stack restore is treated as
// finished. Comfortably longer than the per-member progress linger (~800ms) plus
// the gap before the next member starts, so the cancel button doesn't flicker out
// between sequential members.
const STACK_DONE_GRACE_MS = 8000;

// How often a stack card reads the run history for its members' outcomes.
const STACK_RUNS_POLL_MS = 2000;

// StackCard is one compose stack: its name, its members and a collapsible
// "Restore stack" action that restores every member stopped, then optionally
// starts them in dependency order. The server runs the restore detached and
// its answer carries no member results, so the card shows a "restore started"
// note and reads the per-member outcomes from the run history.
export function StackCard({
  group,
  onRestored,
  t,
  index,
}: {
  group: StackGroup;
  onRestored: () => void;
  t: T;
  /** Rainbow position of this card among the stacks rendered together. It
   *  counts separately from the container rows and from the page-wide
   *  sequence the panel heading takes, as every headed list on this page
   *  does. */
  index: number;
}) {
  const [open, setOpen] = useState(false);
  const [source, setSource] = useState<RepoSource>("local");
  const [startInOrder, setStartInOrder] = useState(true);
  const [busy, setBusy] = useState(false);
  const { lang } = useT();
  const { push } = useToast();
  // Read through a ref rather than the hook value directly: run() reads this
  // after its own await (the restore confirm, then getStackDir), by which
  // point a slow-to-load target list may have caught up. Closing over the
  // hook value itself would freeze it at the render that owned the click.
  const host = useHostLabel();
  const hostRef = useRef(host);
  hostRef.current = host;
  const targets = useOffsiteTargets("containers");
  const targetsRef = useRef(targets);
  targetsRef.current = targets;
  const [shake, setShake] = useState(0);
  const [started, setStarted] = useState(false);
  // The restore runs its members detached, so their outcomes are read back
  // from the run history, counting only runs newer than watchFrom holds. A
  // member Docker refused for its GPU or runtime is offered again without them.
  const [watchFrom, setWatchFrom] = useState<Set<string> | null>(null);
  const [runtimeRefused, setRuntimeRefused] = useState<string[]>([]);
  // A stack restore has no progress entry of its own: members are restored one
  // by one under their "container:<name>" keys, and `finished` is derived from
  // those. Cancelling targets the synthetic "stack:<project>" key, which aborts
  // the member loop at the current member.
  const [finished, setFinished] = useState(false);
  const progress = useProgress();
  const anyMemberActive =
    started &&
    group.members.some((m) => {
      const p = progress[`container:${m.name}`];
      return !!p && p.active && p.phase === "restore";
    });
  // Once a member has been active the cancel button stays up through the gaps
  // between sequential members. The restore counts as finished only after no
  // member has been active for the grace window.
  const sawActive = useRef(false);
  const { confirm, confirmDialog } = useConfirm();
  useEffect(() => {
    if (!started) return;
    if (anyMemberActive) {
      sawActive.current = true;
      return; // the next member started; the cleanup dropped the pending timer
    }
    if (!sawActive.current) return; // nothing has run yet: don't finish early
    const timer = setTimeout(() => {
      sawActive.current = false;
      setStarted(false); // reset so a later single-member restore can't resurrect
      setFinished(true); //   the stack cancel button
    }, STACK_DONE_GRACE_MS);
    return () => clearTimeout(timer);
  }, [started, anyMemberActive]);

  const memberNames = group.members.map((m) => m.name).join(",");
  useEffect(() => {
    if (!watchFrom) return;
    const names = new Set(memberNames.split(","));
    let alive = true;
    let timer: ReturnType<typeof setTimeout>;
    const poll = async () => {
      const res = await listRuns().catch(() => null);
      if (!alive) return;
      // The list is newest first, so the first run of a member is its latest.
      const latest = new Map<string, Run>();
      for (const r of res?.runs ?? []) {
        if (r.kind === "restore" && r.domain === "container" && names.has(r.target) && !watchFrom.has(r.id) && !latest.has(r.target)) {
          latest.set(r.target, r);
        }
      }
      const done = [...latest.values()].filter((r) => r.status !== "running");
      if (done.length === names.size || done.some((r) => r.status === "cancelled") || finished) {
        setRuntimeRefused(done.filter((r) => isRuntimeRefusal(r.error)).map((r) => r.target));
        setWatchFrom(null);
        return;
      }
      timer = setTimeout(() => void poll(), STACK_RUNS_POLL_MS);
    };
    timer = setTimeout(() => void poll(), STACK_RUNS_POLL_MS);
    return () => {
      alive = false;
      clearTimeout(timer);
    };
  }, [watchFrom, finished, memberNames]);

  async function run() {
    // A member whose data folder is copied while the stack runs comes back as
    // files that may not start, so the question names it.
    const live = group.members.filter((m) => m.dbDataCoverage === "live").map((m) => m.name);
    const question = live.length
      ? `${t("stack.restoreConfirm")} ${t("dbdump.stackRestoreWarn", live.length).replace("{names}", listSeparated(lang, live))}`
      : t("stack.restoreConfirm");
    setBusy(true);
    const check = await checkRestoreOnce({ kind: "stack", name: group.project, source });
    setBusy(false);
    const refusal = restoreBlockReason(check, t);
    const extra = <RestoreCheckPanel check={check} t={t} />;
    if (!(await confirm(question, { extra, confirmBlocked: refusal }))) return;
    setRuntimeRefused([]);
    setBusy(true);
    setStarted(false);
    setFinished(false);
    sawActive.current = false;
    try {
      let stackDirSource: string | undefined;
      if (isOffsiteSource(source)) {
        const dir = await getStackDir(group.project, source);
        if (!dir.ok) {
          push(placementErrorText(t, lang, dir, "settings.error"), "fail");
          setShake((n) => n + 1);
          return;
        }
        const ts = targetsRef.current;
        const picked = source === "offsite" ? ts[0] : ts.find((x) => `offsite:${x.id}` === source);
        const placeName = picked ? offsiteTargetLabel(picked) : t("source.offsite");
        const ask = t("timeline.stackDirMissing")
          .replace("{place}", () => placeName)
          .replace("{home}", () => hostRef.current);
        if (!dir.found) {
          if (!(await confirm(ask))) return;
          stackDirSource = "local";
        }
      }
      const before = await listRuns().catch(() => null);
      const res = await restoreStack(group.project, startInOrder, true, source, stackDirSource);
      if (res.ok) {
        setStarted(true);
        setWatchFrom(new Set((before?.runs ?? []).map((r) => r.id)));
        onRestored(); // refresh the main list so run-state/orphan rows update
      } else {
        push(res.error ?? t("settings.error"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      setShake((n) => n + 1);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div
      style={{ ...hueVars(index), "--row-i": String(index) } as CSSProperties}
      // glim-stagger-row reads --row-i, so the entrance stagger follows the
      // same index as the hue.
      className="relative overflow-hidden bg-carbon-surface rounded-card p-4 flex flex-col gap-2 glim-hue glim-stagger-row"
    >
      <div className="flex items-start justify-between gap-3 flex-wrap">
        <div className="min-w-0">
          <span className="font-semibold text-carbon-text text-sm wrap-break-word">{group.project}</span>
          <span className="ms-2 text-xs text-carbon-textMuted">
            {t("stack.members", group.members.length)}
          </span>
          <p className="mt-0.5 text-caption text-carbon-textMuted truncate">
            {group.members.map((m) => m.name).join(", ")}
          </p>
        </div>
        {/* Icon only, so the one "Restore stack" label on the card is the
            action button inside the panel. */}
        <IconTipButton
          tip={t("stack.restore")}
          onClick={() => setOpen((p) => !p)}
          ariaExpanded={open}
          className="shrink-0 inline-flex items-center rounded-pill p-1.5 text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors"
        >
          <svg width="14" height="14" viewBox="0 0 12 12" fill="none" className={`transition-transform ${open ? "rotate-90" : "rtl:rotate-180"}`}>
            <path fill="currentColor" d="M4 1.3 8.5 6 4 10.7Z" />
          </svg>
        </IconTipButton>
      </div>

      {open && (
        <div className="mt-1 rounded-card bg-carbon-background p-3 flex flex-col gap-2">
          <p className="text-xs text-carbon-textMuted">{t("stack.restoreHint")}</p>
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-xs text-carbon-textMuted">{t("source.label")}</span>
            <SourceToggle source={source} onChange={setSource} disabled={busy} domain="containers" />
          </div>
          <Toggle checked={startInOrder} onChange={setStartInOrder} label={t("stack.startInOrder")} />
          <div className="flex items-center gap-3 pt-0.5">
            <Button
              key={shake}
              label={t("stack.restore")}
              labelKey="stack.restore"
              tone="accent"
              onClick={() => void run()}
              disabled={busy}
              busy={busy}
              title={busy ? t("stack.restoring") : undefined}
              className={shake ? "glim-shake" : ""}
            />
          </div>

          {started && !busy && (
            <div className="flex flex-col gap-1">
              <p className="text-xs text-carbon-textSub">{t("restore.started")}</p>
              <p className="text-caption text-carbon-textMuted">{t("restore.bgHint")}</p>
              {/* A stack restore is in place, so cancelling gets the hard
                  warning. */}
              <RestoreCancelButton cancelKey={`stack:${group.project}`} inPlace name={group.project} t={t} />
            </div>
          )}
          {finished && !busy && (
            <p className="text-xs text-carbon-textSub">{t("stack.restoreFinished")}</p>
          )}
          {runtimeRefused.length > 0 && (
            <RuntimeRetry
              key={runtimeRefused.join(",")}
              names={runtimeRefused}
              source={source}
              leaveStopped={!startInOrder}
              onDone={onRestored}
              t={t}
            />
          )}
        </div>
      )}
      {confirmDialog}
    </div>
  );
}
