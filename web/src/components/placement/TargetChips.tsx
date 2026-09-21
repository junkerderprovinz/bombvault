import { Selector, type SelectorItem } from "../Selector";
import type { PlacementOptions, PlacementView, TargetOption } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { chipTicked, lastChipLocked } from "../../lib/placement";

export function TargetChips({
  label,
  targets,
  view,
  options,
  disabled,
  hueOffset,
  onToggle,
}: {
  label: string;
  targets: TargetOption[];
  view: PlacementView;
  options: PlacementOptions;
  disabled?: boolean;
  hueOffset?: number;
  onToggle: (targetId: string, on: boolean) => void;
}) {
  const { t } = useT();
  const known = new Set(targets.map((x) => x.id));
  const deleted = view.skip.filter((id) => id !== "*" && !known.has(id));
  const items: SelectorItem[] = [
    ...targets.map((x) => {
      const last = lastChipLocked(view, options, x.id);
      const tips = [last ? t("placement.lastChip") : "", x.hint === "creds-differ" ? t("placement.credsDiffer") : ""];
      return {
        id: x.id,
        label: x.enabled ? x.name : t("placement.off").replace("{name}", () => x.name),
        disabled: !x.enabled || last,
        title: tips.filter(Boolean).join(" ") || undefined,
      };
    }),
    ...deleted.map((id) => ({ id, label: t("placement.deleted"), disabled: true })),
  ];
  const active = new Set(targets.filter((x) => chipTicked(view, x.id)).map((x) => x.id));
  return (
    <div className="flex items-center gap-2 flex-wrap">
      <span className="text-xs text-carbon-textSub">{label}</span>
      <Selector
        items={items}
        label={label}
        select="many"
        active={active}
        hueOffset={hueOffset}
        disabled={disabled}
        onChange={(id) => onToggle(id, !active.has(id))}
      />
    </div>
  );
}
