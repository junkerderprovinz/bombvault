import { useState, type ReactNode } from "react";
import { getStackDir } from "../../lib/api";
import type { StackGroup } from "../../lib/backupList";
import { useT, type TranslationKey } from "../../lib/i18n";
import { placementErrorText } from "../../lib/placementCodes";
import { useProgress } from "../../lib/progress";
import { useToast } from "../../lib/toast";
import { useHostLabel } from "../../lib/useHostLabel";
import { offsiteTargetLabel, useOffsiteTargets } from "../../lib/useOffsiteTargets";
import { restoreBlockReason, useRestoreCheck } from "../../lib/useRestoreCheck";
import { stackRestoreQuestion, useStackRestore } from "../../lib/useStackRestore";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { Card, ToggleRow } from "../../pages/settings/shared";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { IconBack, IconForward } from "../glyphs";
import { IconContainers, IconRestore } from "../navGlyphs";
import { ProgressBar } from "../ProgressBar";
import { RadioMark } from "../RadioMark";
import { RestoreCancelButton } from "../RestoreCancelButton";
import { RestoreCheckPanel } from "../restore/RestoreCheckPanel";
import { RuntimeRetry } from "../restore/RuntimeRetry";
import { SourceToggle, isOffsiteSource, type RepoSource } from "../SourceToggle";
import { ListNote } from "./ListNotes";

type T = (key: TranslationKey, n?: number) => string;

function membersLine(stack: StackGroup, t: T): string {
  return `${t("stack.members", stack.members.length)} · ${stack.members.map((m) => m.name).join(", ")}`;
}

/** StacksCard lists the compose projects that can come back as one. */
export function StacksCard({ stacks, onRestore, t }: { stacks: StackGroup[]; onRestore: (project: string) => void; t: T }) {
  return (
    <Card title={t("stack.title")} hint={t("backups.stack.hint")} hueIndex={1}>
      <ul data-testid="stacks">
        {stacks.map((stack) => (
          <li
            key={stack.project}
            className="flex min-h-[50px] flex-wrap items-center gap-3 border-t border-carbon-border px-1.5 py-2 first:border-t-0"
          >
            <span className="flex h-[34px] w-[34px] flex-none items-center justify-center rounded-control bg-carbon-surface2 text-accentText [&_svg]:h-[18px] [&_svg]:w-[18px]">
              <IconContainers />
            </span>
            <span className="flex min-w-0 flex-1 flex-col">
              <span className="truncate text-sm font-semibold text-carbon-text">{stack.project}</span>
              <span className="text-xs text-carbon-textMuted wrap-anywhere">{membersLine(stack, t)}</span>
            </span>
            <Button
              label={t("backups.stack.restore")}
              labelKey={null}
              glyph={<IconRestore />}
              tone="subtle"
              onClick={() => onRestore(stack.project)}
            />
          </li>
        ))}
      </ul>
    </Card>
  );
}

type Step = "pick" | "check" | "run";

/**
 * StackWindow restores a compose stack in three steps: which stack, from
 * where and how it starts; what the check before the restore found; and the
 * members coming back one after the other.
 */
export function StackWindow({
  stacks,
  initial,
  onClose,
  onStarted,
}: {
  stacks: StackGroup[];
  /** The stack chosen when the window opens. */
  initial: string;
  onClose: () => void;
  onStarted: () => void;
}) {
  const { t, lang } = useT();
  const { push } = useToast();
  const [project, setProject] = useState(initial);
  const [source, setSource] = useState<RepoSource>("local");
  const [startInOrder, setStartInOrder] = useState(true);
  const [step, setStep] = useState<Step>("pick");
  const [busy, setBusy] = useState(false);
  // An off-site source may hold the containers but not the project folder.
  const [folder, setFolder] = useState<"there" | "missing" | "local">("there");

  const stack = stacks.find((s) => s.project === project) ?? stacks[0];
  const names = stack.members.map((m) => m.name);
  const restore = useStackRestore(stack.project, names);
  const check = useRestoreCheck(step === "check" ? { kind: "stack", name: stack.project, source } : null);
  const refusal = restoreBlockReason(check, t);
  const progress = useProgress();
  const host = useHostLabel();
  const targets = useOffsiteTargets("containers");

  const target = source === "offsite" ? targets[0] : targets.find((x) => `offsite:${x.id}` === source);
  const place = source === "local" ? t("source.local") : target ? offsiteTargetLabel(target) : t("source.offsite");

  function choose(next: { project?: string; source?: RepoSource }) {
    if (next.project !== undefined) setProject(next.project);
    if (next.source !== undefined) setSource(next.source);
    setFolder("there");
  }

  async function toCheck() {
    if (isOffsiteSource(source) && folder === "there") {
      setBusy(true);
      try {
        const dir = await getStackDir(stack.project, source);
        if (!dir.ok) {
          push(placementErrorText(t, lang, dir, "settings.error"), "fail");
          return;
        }
        if (!dir.found) {
          setFolder("missing");
          return;
        }
      } catch (err) {
        push(err instanceof Error ? err.message : t("settings.error"), "fail");
        return;
      } finally {
        setBusy(false);
      }
    }
    setStep("check");
  }

  function takeLocalFolder() {
    setFolder("local");
    setStep("check");
  }

  async function start() {
    setBusy(true);
    try {
      const res = await restore.start({ source, startInOrder, stackDirSource: folder === "local" ? "local" : undefined });
      if (res.ok) {
        setStep("run");
        onStarted();
      } else {
        push(res.error ?? t("settings.error"), "fail");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    } finally {
      setBusy(false);
    }
  }

  let footer: ReactNode;
  if (step === "pick") {
    footer = (
      <>
        <Button label={t("common.cancel")} labelKey="common.cancel" tone="subtle" onClick={onClose} />
        <Button
          label={t("zfs.replica.add.next")}
          labelKey={null}
          glyph={<IconForward />}
          tone="accent"
          onClick={() => void toCheck()}
          disabled={busy || folder === "missing"}
          busy={busy}
        />
      </>
    );
  } else if (step === "check") {
    footer = (
      <>
        <Button label={t("common.back")} labelKey={null} glyph={<IconBack />} tone="subtle" onClick={() => setStep("pick")} disabled={busy} />
        <Button
          label={t("backups.stack.restore")}
          labelKey={null}
          glyph={<IconRestore />}
          tone="accent"
          onClick={() => void start()}
          disabled={busy || !check.ready}
          busy={busy}
          title={refusal}
        />
      </>
    );
  } else {
    footer = (
      <>
        {restore.started && <RestoreCancelButton cancelKey={`stack:${stack.project}`} inPlace name={stack.project} t={t} />}
        <Button label={t("common.close")} labelKey="common.close" tone="accent" onClick={onClose} />
      </>
    );
  }

  return (
    <InstanceWindow
      title={t("backups.stack.restore")}
      facts={step === "pick" ? t("backups.stack.intro") : `${stack.project} · ${place}`}
      footer={footer}
      onClose={onClose}
    >
      {step === "pick" && (
        <>
          <div role="radiogroup" aria-label={t("stack.title")} className="flex flex-col gap-2">
            {stacks.map((s) => {
              const on = s.project === stack.project;
              return (
                <button
                  key={s.project}
                  type="button"
                  role="radio"
                  aria-checked={on}
                  onClick={() => choose({ project: s.project })}
                  className={`flex items-center gap-3 rounded-control bg-carbon-surface2 p-3 text-start ring-2 ring-inset transition-colors hover:bg-carbon-surface3 ${
                    on ? "ring-accent" : "ring-transparent"
                  }`}
                >
                  <RadioMark on={on} />
                  <span className="flex min-w-0 flex-col">
                    <span className="text-sm font-semibold text-carbon-text wrap-anywhere">{s.project}</span>
                    <span className="text-xs text-carbon-textMuted wrap-anywhere">{membersLine(s, t)}</span>
                  </span>
                </button>
              );
            })}
          </div>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <span className="text-sm text-carbon-text">{t("retentionPreview.sourceLabel")}</span>
            <SourceToggle source={source} onChange={(next) => choose({ source: next })} disabled={busy} domain="containers" />
          </div>
          {folder === "missing" && (
            <ListNote tone="warn">
              <span className="min-w-[14rem] flex-1">
                {t("timeline.stackDirMissing")
                  .replace("{place}", () => place)
                  .replace("{home}", () => host)}
              </span>
              <Button label={t("backups.stack.useLocal")} labelKey="backups.stack.useLocal" onClick={takeLocalFolder} />
            </ListNote>
          )}
          <ToggleRow
            label={t("stack.startInOrder")}
            hint={t("backups.stack.orderHint")}
            checked={startInOrder}
            onChange={setStartInOrder}
          />
        </>
      )}

      {step === "check" && (
        <>
          <p className="text-sm text-carbon-text">{stackRestoreQuestion(stack.members, t, lang)}</p>
          <RestoreCheckPanel check={check} t={t} />
        </>
      )}

      {step === "run" && (
        <>
          <ul>
            {names.map((name) => {
              const live = progress[`container:${name}`];
              const restoring = !!live && live.active && live.phase === "restore";
              const outcome = restore.outcomes.get(name);
              const ok = outcome?.status === "success";
              return (
                <li key={name} className="flex min-h-11 items-center gap-3 border-t border-carbon-border py-2 first:border-t-0">
                  <span className={`h-2 w-2 flex-none rounded-pill ${ok ? "bg-statusOkSolid" : "bg-carbon-surface3"}`} />
                  <span className="min-w-0 flex-1 text-sm text-carbon-text wrap-anywhere">{name}</span>
                  {restoring ? (
                    <span className="w-24 flex-none">
                      <ProgressBar percent={live.percent} active inline />
                    </span>
                  ) : ok ? (
                    <span className="text-sm text-carbon-textSub">
                      {t(startInOrder ? "stack.memberStarted" : "stack.memberRestored")}
                    </span>
                  ) : (
                    outcome?.status === "failed" && <Badge tone="fail">{t("run.statusFailed")}</Badge>
                  )}
                </li>
              );
            })}
          </ul>
          {restore.finished ? (
            <ListNote tone="ok">{t("stack.restoreFinished")}</ListNote>
          ) : (
            <p className="text-sm text-carbon-textMuted">{t("restore.bgHint")}</p>
          )}
          {restore.runtimeRefused.length > 0 && (
            <RuntimeRetry
              key={restore.runtimeRefused.join(",")}
              names={restore.runtimeRefused}
              source={source}
              leaveStopped={!startInOrder}
              onDone={onStarted}
              t={t}
            />
          )}
        </>
      )}
    </InstanceWindow>
  );
}
