import { PlaceMark } from "../placeMarks";
import { Selector } from "../Selector";
import type { SelectOption } from "../SelectField";
import type { PlacementOptions, PlacementView, SegmentId, SendToOption } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { homeOptionLabel, lockedSegments, segmentItems, sendToLabel, viewHomeLabel } from "../../lib/placement";
import { HomeSelect } from "./HomeSelect";
import { TargetChips } from "./TargetChips";

export interface PlacementBarProps {
  view: PlacementView;
  options: PlacementOptions;
  host: string;
  hueOffset?: number;
  disabled?: boolean;
  onSegment: (seg: SegmentId) => void;
  onHome: (repoId: string) => void;
  onSendTo: (opt: SendToOption) => void;
  onChip: (targetId: string, on: boolean) => void;
}

/** sendToKey tells a direct repository that does not exist yet apart from every
 *  repository id, so Send to can offer it as a value of its own. */
export function sendToKey(opt: SendToOption): string {
  return opt.repoId || `direct:${opt.targetId}`;
}

// withStored keeps a stored value that is switched off or unknown in the list,
// marked and not selectable, so the field never pretends the item is elsewhere.
function withStored(list: SelectOption<string>[], value: string, label: string): SelectOption<string>[] {
  return list.some((o) => o.value === value) ? list : [...list, { value, label, disabled: true }];
}

export function PlacementBar({
  view,
  options,
  host,
  hueOffset,
  disabled,
  onSegment,
  onHome,
  onSendTo,
  onChip,
}: PlacementBarProps) {
  const { t } = useT();
  const home = viewHomeLabel(t, host, view, options);
  const segment = view.segment === "" ? null : view.segment;
  const mark = (provider: string) => (provider ? <PlaceMark provider={provider} /> : undefined);
  const homes = options.homes.map((h) => ({ value: h.id, label: homeOptionLabel(t, host, h), glyph: mark(h.provider) }));
  const sendTo = options.sendTo.map((s) => ({ value: sendToKey(s), label: sendToLabel(t, s), glyph: mark(s.provider) }));
  return (
    <div className="flex min-w-0 flex-col gap-2">
      <Selector
        items={segmentItems(t, lockedSegments(view, options), home)}
        label={t("placement.title")}
        select="one"
        activation="manual"
        active={segment}
        hueOffset={hueOffset}
        disabled={disabled}
        onChange={(id) => onSegment(id as SegmentId)}
      />
      {segment !== "offsite-only" && (
        <HomeSelect
          label={t("placement.storedOn")}
          value={view.repo}
          options={withStored(homes, view.repo, home)}
          locked={view.locked}
          disabled={disabled}
          onCommit={onHome}
        />
      )}
      {segment === "local-offsite" && (
        <TargetChips
          label={t("placement.copyTo")}
          targets={options.targets}
          view={view}
          options={options}
          disabled={disabled}
          hueOffset={hueOffset}
          onToggle={onChip}
        />
      )}
      {segment === "offsite-only" && (
        <HomeSelect
          label={t("placement.sendTo")}
          value={view.repo}
          options={withStored(sendTo, view.repo, home)}
          locked={view.locked}
          disabled={disabled}
          onCommit={(key) => {
            const opt = options.sendTo.find((s) => sendToKey(s) === key);
            if (opt) onSendTo(opt);
          }}
        />
      )}
    </div>
  );
}
