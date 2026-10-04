import { useState } from "react";
import { Button } from "../Button";
import { InfoBubble } from "../InfoBubble";
import {
  previewItemPlacement,
  type ItemRef,
  type OkEnvelope,
  type PlacementChange,
  type PlacementView,
  type SendToOption,
  type UploadEstimate,
} from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import {
  addsTargets,
  followLine,
  formatList,
  newTargetRefused,
  noCopyNow,
  stepForHome,
  stepForLocal,
  stepForNewTarget,
  stepForReset,
  stepForTarget,
  viewHomeLabel,
  type FollowLine,
  type PlacementStep,
} from "../../lib/placement";
import { blockedText, tickDestination, useNewDestinations } from "../../lib/placementButtons";
import { placementErrorText } from "../../lib/placementCodes";
import { placementChanged } from "../../lib/placementEvents";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { useHostLabel } from "../../lib/useHostLabel";
import { usePlacementOptions } from "../../lib/usePlacementOptions";
import { usePlacementSave } from "../../lib/usePlacementSave";
import { DirectRepoDialog } from "./DirectRepoDialog";
import { PlacementBar } from "./PlacementBar";
import { PlacementStatus } from "./PlacementStatus";

const FOLLOW_KEYS: Record<Exclude<FollowLine, "home-set">, TranslationKey> = {
  follows: "placement.followsDefault",
  "own-copies": "placement.ownCopies",
  "copies-follow": "placement.copiesFollow",
};

/** UploadLines is what a question about new copies names per target. */
export function UploadLines({ added }: { added: UploadEstimate[] }) {
  const { t, lang } = useT();
  const uncheckable = [...new Set(added.flatMap((a) => a.uncheckable))];
  return (
    <div className="flex flex-col gap-1 text-sm text-carbon-textSub">
      {added.map((a) => (
        <p key={a.targetId}>
          {t("placement.uploadLine").replace("{target}", () => a.name).replace("{n}", String(a.snapshots))}
        </p>
      ))}
      {uncheckable.length > 0 && (
        <p>{t("placement.uncheckable").replace("{list}", () => formatList(lang, uncheckable))}</p>
      )}
      <p>{t("placement.uploadCost")}</p>
    </div>
  );
}

export function PlacementRow({
  item,
  name,
  view,
  onView,
}: {
  item: ItemRef;
  name: string;
  view: PlacementView;
  onView: (next: PlacementView) => void;
}) {
  const { t, lang } = useT();
  const host = useHostLabel();
  const { options, error } = usePlacementOptions(item.domain);
  const { shown, shake, save } = usePlacementSave(item, view, onView);
  const { confirm, confirmDialog } = useConfirm();
  const { push } = useToast();
  const destinations = useNewDestinations(item.domain, options);
  const [direct, setDirect] = useState<{ target: SendToOption; skip: string[] } | null>(null);
  const [asking, setAsking] = useState(false);

  async function uploadsAgreed(change: PlacementChange): Promise<boolean> {
    let res: OkEnvelope & { added?: UploadEstimate[] };
    try {
      res = await previewItemPlacement(item, change);
    } catch (err) {
      res = { ok: false, error: err instanceof Error ? err.message : undefined };
    }
    // Without an estimate the question is all that stands between the click and
    // a whole history going up, so it is asked with the reason instead.
    if (!res.ok) {
      const extra = (
        <div className="flex flex-col gap-1 text-sm text-carbon-textSub">
          <p>{placementErrorText(t, lang, res, "settings.error")}</p>
          <p>{t("placement.uploadCost")}</p>
        </div>
      );
      return confirm(t("placement.uploadUnknown").replace("{name}", () => name), { extra, cancelTone: "neutral" });
    }
    const added = (res.added ?? []).filter((a) => a.snapshots > 0);
    if (added.length === 0) return true;
    const intro = t("placement.uploadIntro").replace("{name}", () => name);
    return confirm(intro, { extra: <UploadLines added={added} />, cancelTone: "neutral" });
  }

  async function run(step: PlacementStep) {
    if (step.kind === "none") return;
    if (step.kind === "blocked") {
      push(blockedText(t, step, options ? viewHomeLabel(t, host, shown, options) : host), "warn");
      return;
    }
    if (step.kind === "direct") {
      setDirect({ target: step.target, skip: step.skip ?? ["*"] });
      return;
    }
    // The bar stays put until the question is answered: a second choice would
    // take the dialog over and leave the first one hanging.
    setAsking(true);
    try {
      const home = step.confirmHome;
      if (home !== null) {
        const question = t("placement.confirmHome").replace("{name}", () => name).replace("{home}", () => home);
        const yes = await confirm(question, {
          confirmLabel: t("placement.saveHome"),
          confirmLabelKey: "placement.saveHome",
          cancelTone: "neutral",
        });
        if (!yes) return;
      }
      if (addsTargets(shown, step.change) && !(await uploadsAgreed(step.change))) return;
    } finally {
      setAsking(false);
    }
    save(step.change, step.optimistic);
  }

  async function addDestination(id: string) {
    if (!options) return;
    const refused = newTargetRefused(shown);
    if (refused) return void run(refused);
    setAsking(true);
    const made = await tickDestination(id, item.domain);
    setAsking(false);
    if ("error" in made) {
      push(placementErrorText(t, lang, made.error, "settings.error"), "fail");
      return;
    }
    void run(stepForNewTarget(made.targetId, shown, options));
  }

  const unreadable = shown.unreadable || options?.unreadable === true || (options === null && error !== null);
  const line = followLine(shown);
  // An item fixed on a remote repository is never copied and cannot move, so
  // a line about its copies and a Reset would only mislead.
  const copiesLine = !(shown.locked && shown.repoKind === "remote");
  const warn = options ? noCopyNow(shown, options) : [];

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-start gap-2 flex-wrap">
        <span className="flex items-center gap-1 pt-1.5 text-xs text-carbon-textSub shrink-0">
          {t("placement.title")}
          <InfoBubble tip={t("placement.titleHint")} />
        </span>
        {unreadable ? (
          <span className="pt-1.5 text-xs text-statusWarn">{t("placement.unreadable")}</span>
        ) : (
          options && (
            // Beside the label a phone leaves the bar too narrow for its
            // buttons, so there it takes a line of its own.
            <div key={shake} className={`min-w-0 flex-1 max-md:basis-full${shake ? " glim-shake" : ""}`}>
              <PlacementBar
                domain={item.domain}
                context="item"
                view={shown}
                options={options}
                host={host}
                destinations={destinations}
                disabled={asking}
                onLocal={(on) => void run(stepForLocal(on, shown, options, t, host))}
                onTarget={(id, on) => void run(stepForTarget(id, on, shown, options, t))}
                onDestination={(id) => void addDestination(id)}
                onHome={(id) => void run(stepForHome(id, shown, options, t, host))}
              />
            </div>
          )
        )}
      </div>
      {!unreadable && options && (
        <div className="flex flex-col gap-1">
          {copiesLine && (
            <p className="flex items-center gap-2 flex-wrap text-xs text-carbon-textMuted">
              <span>
                {line === "home-set"
                  ? t("placement.homeSet").replace("{home}", () => viewHomeLabel(t, host, shown, options))
                  : t(FOLLOW_KEYS[line])}
              </span>
              {(line === "own-copies" || line === "home-set") && (
                <Button
                  label={t("placement.reset")}
                  labelKey="placement.reset"
                  tone="neutral"
                  disabled={asking}
                  onClick={() => void run(stepForReset(shown))}
                />
              )}
            </p>
          )}
          {warn.length > 0 && (
            <p className="text-xs text-statusWarn">
              {t("placement.noCopyNow").replace("{targets}", () => formatList(lang, warn))}
            </p>
          )}
          <PlacementStatus item={item} name={name} view={shown} onChanged={placementChanged} />
        </div>
      )}
      {confirmDialog}
      {direct && (
        <DirectRepoDialog
          target={{ id: direct.target.targetId, name: direct.target.name }}
          mode="create"
          onClose={() => setDirect(null)}
          onDone={(result) => {
            const { target, skip } = direct;
            setDirect(null);
            if (result.kind !== "created") return;
            save(
              { home: { repo: result.repo.id }, copies: { skip } },
              {
                segment: "offsite-only",
                repo: result.repo.id,
                repoKind: "direct",
                repoTarget: target.targetId,
                repoLabel: result.repo.name,
                homeFollows: false,
                skip,
                copiesFollow: false,
              }
            );
          }}
        />
      )}
    </div>
  );
}
