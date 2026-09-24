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
  noCopyNow,
  stepForChip,
  stepForHome,
  stepForReset,
  stepForSegment,
  stepForSendTo,
  viewHomeLabel,
  type FollowLine,
  type PlacementStep,
} from "../../lib/placement";
import { placementErrorText } from "../../lib/placementCodes";
import { placementChanged } from "../../lib/placementEvents";
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
  const [direct, setDirect] = useState<SendToOption | null>(null);
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
      return confirm(t("placement.uploadUnknown").replace("{name}", () => name), { extra });
    }
    const added = (res.added ?? []).filter((a) => a.snapshots > 0);
    if (added.length === 0) return true;
    const intro = t("placement.uploadIntro").replace("{name}", () => name);
    return confirm(intro, { extra: <UploadLines added={added} /> });
  }

  async function run(step: PlacementStep) {
    if (step.kind === "none") return;
    if (step.kind === "direct") {
      setDirect(step.target);
      return;
    }
    // The bar stays put until the question is answered: a second choice would
    // take the dialog over and leave the first one hanging.
    setAsking(true);
    try {
      const home = step.confirmHome;
      if (home !== null) {
        const question = t("placement.confirmHome").replace("{name}", () => name).replace("{home}", () => home);
        if (!(await confirm(question, { confirmLabel: t("placement.saveHome"), confirmLabelKey: "placement.saveHome" }))) return;
      }
      if (addsTargets(shown, step.change) && !(await uploadsAgreed(step.change))) return;
    } finally {
      setAsking(false);
    }
    save(step.change, step.optimistic);
  }

  const unreadable = shown.unreadable || options?.unreadable === true || (options === null && error !== null);
  const line = followLine(shown);
  // An item fixed on a remote or direct repository is never copied and cannot
  // move, so a line about its copies and a Reset would only mislead.
  const copiesLine = !(shown.locked && shown.segment === "offsite-only");
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
            <div key={shake} className={`min-w-0 flex-1${shake ? " glim-shake" : ""}`}>
              <PlacementBar
                domain={item.domain}
                context="item"
                view={shown}
                options={options}
                host={host}
                disabled={asking}
                onSegment={(seg) => void run(stepForSegment(seg, shown, options, t, host))}
                onHome={(id) => void run(stepForHome(id, shown, options, t, host))}
                onSendTo={(opt) => void run(stepForSendTo(opt, shown, t))}
                onChip={(id, on) => void run(stepForChip(id, on, shown, options))}
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
          target={{ id: direct.targetId, name: direct.name }}
          mode="create"
          onClose={() => setDirect(null)}
          onDone={(result) => {
            setDirect(null);
            if (result.kind !== "created") return;
            save(
              { home: { repo: result.repo.id }, copies: { skip: ["*"] } },
              {
                segment: "offsite-only",
                repo: result.repo.id,
                repoKind: "direct",
                repoLabel: result.repo.name,
                homeFollows: false,
                skip: ["*"],
                copiesFollow: false,
              }
            );
          }}
        />
      )}
    </div>
  );
}
