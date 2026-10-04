import { useState } from "react";
import { InfoBubble } from "../InfoBubble";
import type { CopiesChoice, PlacementChange, PlacementOptions, PlacementView, SendToOption } from "../../lib/api";
import { useT } from "../../lib/i18n";
import {
  draftView,
  newTargetRefused,
  stepForHome,
  stepForLocal,
  stepForNewTarget,
  stepForTarget,
  viewSegment,
  type PlacementStep,
} from "../../lib/placement";
import { blockedText, tickDestination, useNewDestinations } from "../../lib/placementButtons";
import { placementErrorText } from "../../lib/placementCodes";
import { useToast } from "../../lib/toast";
import { useHostLabel } from "../../lib/useHostLabel";
import { usePlacementOptions } from "../../lib/usePlacementOptions";
import { DirectRepoDialog } from "./DirectRepoDialog";
import { PlacementBar } from "./PlacementBar";

export interface PlacementDraftValue {
  home?: { repo: string } | { direct: { targetId: string; name: string; location: string } };
  copies?: CopiesChoice;
}

// withChange applies a step to the draft. Following the default drops the axis,
// so a draft that went back to the default is as untouched as one never changed.
function withChange(value: PlacementDraftValue, change: PlacementChange): PlacementDraftValue {
  const next = { ...value };
  if (change.home) {
    if ("follow" in change.home) delete next.home;
    else next.home = { repo: change.home.repo };
  }
  if (change.copies) {
    if ("follow" in change.copies) delete next.copies;
    else next.copies = change.copies;
  }
  return next;
}

// valueView lays the draft over the default the way the bar shows a card.
function valueView(value: PlacementDraftValue, options: PlacementOptions): PlacementView {
  const view = draftView(options);
  const home = value.home;
  if (home && "direct" in home) {
    view.repo = `direct:${home.direct.targetId}`;
    view.repoKind = "direct";
    view.repoTarget = home.direct.targetId;
    view.repoLabel = home.direct.name;
    view.repoDirectOf = home.direct.name ? undefined : options.sendTo.find((s) => s.targetId === home.direct.targetId)?.name;
    view.homeFollows = false;
  } else if (home) {
    const listed = options.homes.find((h) => h.id === home.repo);
    const sent = options.sendTo.find((s) => s.repoId !== "" && s.repoId === home.repo);
    view.repo = home.repo;
    view.repoKind = listed?.kind ?? sent?.kind ?? view.repoKind;
    view.repoLabel = listed?.name ?? sent?.name ?? "";
    view.homeFollows = false;
  }
  if (value.copies && "skip" in value.copies) {
    view.skip = value.copies.skip;
    view.copiesFollow = false;
  }
  view.segment = viewSegment(view.repoKind, view.skip, options.targets.length > 0);
  return view;
}

/** PlacementDraft is the bar in the window for a new folder set: it starts at
 *  the default and only collects what the create sends along. */
export function PlacementDraft({
  value,
  onChange,
}: {
  value: PlacementDraftValue;
  onChange: (next: PlacementDraftValue) => void;
}) {
  const { t, lang } = useT();
  const host = useHostLabel();
  const { push } = useToast();
  const { options } = usePlacementOptions("files");
  const destinations = useNewDestinations("files", options);
  const [direct, setDirect] = useState<{ target: SendToOption; skip: string[] } | null>(null);

  if (!options) return null;
  if (options.unreadable) return <p className="text-xs text-statusWarn">{t("placement.unreadable")}</p>;
  const ready = options;
  const view = valueView(value, ready);

  function run(step: PlacementStep) {
    if (step.kind === "blocked") push(blockedText(t, step, host), "warn");
    else if (step.kind === "direct") setDirect({ target: step.target, skip: step.skip ?? ["*"] });
    else if (step.kind === "save") onChange(withChange(value, step.change));
  }

  async function addDestination(id: string) {
    const refused = newTargetRefused(view);
    if (refused) return run(refused);
    const made = await tickDestination(id, "files");
    if ("error" in made) {
      push(placementErrorText(t, lang, made.error, "settings.error"), "fail");
      return;
    }
    run(stepForNewTarget(made.targetId, view, ready));
  }

  return (
    <div className="flex flex-col gap-1.5">
      <span className="flex items-center gap-1 text-xs text-carbon-textSub">
        {t("placement.title")}
        <InfoBubble tip={t("placement.titleHint")} />
      </span>
      <PlacementBar
        domain="files"
        context="draft"
        view={view}
        options={options}
        host={host}
        destinations={destinations}
        onLocal={(on) => run(stepForLocal(on, view, ready, t, host))}
        onTarget={(id, on) => run(stepForTarget(id, on, view, ready, t))}
        onDestination={(id) => void addDestination(id)}
        onHome={(id) => run(stepForHome(id, view, ready, t, host))}
      />
      {direct && (
        <DirectRepoDialog
          target={{ id: direct.target.targetId, name: direct.target.name }}
          mode="remember"
          onClose={() => setDirect(null)}
          onDone={(result) => {
            const { target, skip } = direct;
            setDirect(null);
            if (result.kind !== "remembered") return;
            onChange({
              home: { direct: { targetId: target.targetId, name: result.name, location: result.location } },
              copies: { skip },
            });
          }}
        />
      )}
    </div>
  );
}
