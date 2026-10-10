import { useRef, useState, type CSSProperties } from "react";
import { getStackDir } from "../../lib/api";
import type { Container } from "../../lib/api";
import { IconTipButton } from "../IconTipButton";
import { useT } from "../../lib/i18n";
import { RestoreCancelButton } from "../RestoreCancelButton";
import { SourceToggle, isOffsiteSource, type RepoSource } from "../SourceToggle";
import { Button } from "../Button";
import { useConfirm } from "../../lib/useConfirm";
import { offsiteTargetLabel, useOffsiteTargets } from "../../lib/useOffsiteTargets";
import { useHostLabel } from "../../lib/useHostLabel";
import { placementErrorText } from "../../lib/placementCodes";
import { hueVars } from "../../lib/appearance";
import { useToast } from "../../lib/toast";
import { Toggle } from "../Toggle";
import { checkRestoreOnce, restoreBlockReason } from "../../lib/useRestoreCheck";
import { stackRestoreQuestion, useStackRestore } from "../../lib/useStackRestore";
import { RestoreCheckPanel } from "../restore/RestoreCheckPanel";
import { RuntimeRetry } from "../restore/RuntimeRetry";

type T = ReturnType<typeof useT>["t"];

export interface StackGroup {
  project: string;
  members: Container[];
}

/** StackCard is one compose stack: its name, its members and a "Restore
 *  stack" action that restores every member stopped and can then start them
 *  in dependency order. */
export function StackCard({
  group,
  onRestored,
  t,
  index,
}: {
  group: StackGroup;
  onRestored: () => void;
  t: T;
  /** Rainbow position of this card among the stacks rendered together. */
  index: number;
}) {
  const [open, setOpen] = useState(false);
  const [source, setSource] = useState<RepoSource>("local");
  const [startInOrder, setStartInOrder] = useState(true);
  const [busy, setBusy] = useState(false);
  const { lang } = useT();
  const { push } = useToast();
  // run() reads both after its own awaits, by which time a slow target list
  // may have arrived. The values of the render that owned the click would be
  // stale.
  const host = useHostLabel();
  const hostRef = useRef(host);
  hostRef.current = host;
  const targets = useOffsiteTargets("containers");
  const targetsRef = useRef(targets);
  targetsRef.current = targets;
  const [shake, setShake] = useState(0);
  const { started, finished, runtimeRefused, start } = useStackRestore(
    group.project,
    group.members.map((m) => m.name)
  );
  const { confirm, confirmDialog } = useConfirm();

  async function run() {
    setBusy(true);
    const check = await checkRestoreOnce({ kind: "stack", name: group.project, source });
    setBusy(false);
    const refusal = restoreBlockReason(check, t);
    const extra = <RestoreCheckPanel check={check} t={t} />;
    if (!(await confirm(stackRestoreQuestion(group.members, t, lang), { extra, confirmBlocked: refusal }))) return;
    setBusy(true);
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
      const res = await start({ source, startInOrder, stackDirSource });
      if (res.ok) {
        onRestored();
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
