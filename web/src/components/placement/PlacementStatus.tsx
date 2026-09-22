import { Fragment, useState } from "react";
import { Button } from "../Button";
import type { ItemRef, PlacementView } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { observedLine, planLines, stackNoteText, type StatusLine } from "../../lib/placement";
import { useHostLabel } from "../../lib/useHostLabel";
import { usePlacementOptions } from "../../lib/usePlacementOptions";
import { OffsiteRemovalDialog } from "./OffsiteRemovalDialog";

const TONE_CLASS: Record<StatusLine["tone"], string> = {
  normal: "text-carbon-textSub",
  warn: "text-statusFail",
  unconfirmed: "text-statusWarn",
  muted: "text-carbon-textMuted",
};

/** PlacementStatus is the result line under a card's placement bar: where the
 *  next backup goes, where its copies actually are, and what a target still
 *  holds that nothing sends to any more. */
export function PlacementStatus({
  item,
  name,
  view,
  onChanged,
}: {
  item: ItemRef;
  name: string;
  view: PlacementView;
  onChanged: () => void;
}) {
  const { t, lang } = useT();
  const host = useHostLabel();
  const { options } = usePlacementOptions(item.domain);
  const [removing, setRemoving] = useState<{ id: string; name: string } | null>(null);

  const plan = view.plan ? planLines(t, lang, host, item.domain, view.plan, options?.targets.length === 0) : [];
  const observed = view.observed ? observedLine(t, lang, view.observed) : [];

  return (
    <div className="flex flex-col gap-1 text-xs">
      {plan.map((line, i) => (
        <p key={i} className={TONE_CLASS[line.tone]}>
          {line.text}
        </p>
      ))}
      {observed.length > 0 && (
        <p>
          {observed.map((line, i) => (
            <Fragment key={i}>
              {i > 0 && <span className="text-carbon-textMuted"> · </span>}
              <span className={TONE_CLASS[line.tone]}>{line.text}</span>
            </Fragment>
          ))}
        </p>
      )}
      {view.observed?.older.map((o) => (
        <div key={o.targetId} className="flex items-center gap-2 flex-wrap">
          <span className="text-carbon-textSub">
            {t("placement.older")
              .replace("{target}", () => o.name)
              .replace("{n}", String(o.count))
              .replace("{date}", () => new Date(o.seenAt * 1000).toLocaleDateString(lang))}
          </span>
          <Button
            label={t("offsiteRemoval.delete").replace("{target}", () => o.name)}
            labelKey="offsiteRemoval.delete"
            tone="neutral"
            disabled={o.appendOnly}
            title={o.appendOnly ? t("offsiteRemoval.appendOnly").replace("{target}", () => o.name) : undefined}
            onClick={() => setRemoving({ id: o.targetId, name: o.name })}
          />
        </div>
      ))}
      {view.stackNote && <p className="text-carbon-textSub">{stackNoteText(t, lang, host, view.stackNote)}</p>}
      {removing && (
        <OffsiteRemovalDialog
          item={item}
          name={name}
          target={removing}
          onDone={() => {
            setRemoving(null);
            onChanged();
          }}
          onClose={() => setRemoving(null)}
        />
      )}
    </div>
  );
}
