import { useState } from "react";
import { InfoBubble } from "../InfoBubble";
import type { CopiesChoice, PlacementChange, PlacementOptions, PlacementView, SendToOption } from "../../lib/api";
import { useT } from "../../lib/i18n";
import {
  draftView,
  stepForChip,
  stepForHome,
  stepForSegment,
  stepForSendTo,
  viewSegment,
  type PlacementStep,
} from "../../lib/placement";
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
    view.repoLabel = home.direct.name;
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
  const { t } = useT();
  const host = useHostLabel();
  const { options } = usePlacementOptions("files");
  const [direct, setDirect] = useState<SendToOption | null>(null);

  if (!options) return null;
  if (options.unreadable) return <p className="text-xs text-statusWarn">{t("placement.unreadable")}</p>;
  const view = valueView(value, options);

  function run(step: PlacementStep) {
    if (step.kind === "direct") setDirect(step.target);
    else if (step.kind === "save") onChange(withChange(value, step.change));
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
        onSegment={(seg) => run(stepForSegment(seg, view, options, t, host))}
        onHome={(id) => run(stepForHome(id, view, options, t, host))}
        onSendTo={(opt) => run(stepForSendTo(opt, view, t))}
        onChip={(id, on) => run(stepForChip(id, on, view, options))}
      />
      {direct && (
        <DirectRepoDialog
          target={{ id: direct.targetId, name: direct.name }}
          mode="remember"
          onClose={() => setDirect(null)}
          onDone={(result) => {
            setDirect(null);
            if (result.kind !== "remembered") return;
            onChange({
              home: { direct: { targetId: direct.targetId, name: result.name, location: result.location } },
              copies: { skip: ["*"] },
            });
          }}
        />
      )}
    </div>
  );
}
