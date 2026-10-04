import { Selector, type SelectorItem } from "../Selector";
import type { SelectOption } from "../SelectField";
import type { Destination, PlacementDomain, PlacementOptions, PlacementView } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { homeOptionLabel, lockHint, offQualifier, placementButtons, viewHomeLabel } from "../../lib/placement";
import { HomeSelect } from "./HomeSelect";

export interface PlacementBarProps {
  domain: PlacementDomain;
  context: "item" | "default" | "draft";
  view: PlacementView;
  options: PlacementOptions;
  host: string;
  /** Destinations with no target in this domain yet. */
  destinations: Destination[];
  hueOffset?: number;
  disabled?: boolean;
  onLocal: (on: boolean) => void;
  onTarget: (targetId: string, on: boolean) => void;
  onDestination: (destinationId: string) => void;
  /** Picks among several local repositories. */
  onHome: (repoId: string) => void;
}

const LOCAL = "local";
const DESTINATION = "destination:";
const REMOTE_HOME = "home:";

// withStored keeps a stored value that is switched off or unknown in the list,
// marked and not selectable, so the field never pretends the item is elsewhere.
function withStored(list: SelectOption<string>[], value: string, label: string): SelectOption<string>[] {
  return list.some((o) => o.value === value) ? list : [...list, { value, label, disabled: true }];
}

/**
 * PlacementBar is one row of buttons: Local and every target. A lit button
 * gets the backups, Local or the first lit target as the place they are
 * written to and every other lit one as a copy.
 */
export function PlacementBar({
  view,
  options,
  host,
  destinations,
  hueOffset,
  disabled,
  onLocal,
  onTarget,
  onDestination,
  onHome,
}: PlacementBarProps) {
  const { t } = useT();
  const b = placementButtons(view, options);
  const home = viewHomeLabel(t, host, view, options);
  const fixed = view.locked ? lockHint(t, "home-fixed", home) : undefined;
  const items: SelectorItem[] = [
    { id: LOCAL, label: t("placement.segLocal"), title: b.local ? fixed : undefined },
    ...options.targets.map((x) => ({
      id: x.id,
      label: x.enabled ? x.name : `${x.name} ${offQualifier(t)}`,
      disabled: !x.enabled && !b.ticked.includes(x.id),
      title: x.id === b.home ? fixed : x.hint === "creds-differ" ? t("placement.credsDiffer") : undefined,
    })),
    ...destinations.map((d) => ({ id: DESTINATION + d.id, label: d.name })),
  ];
  if (view.repoKind === "remote") {
    items.push({ id: REMOTE_HOME + view.repo, label: home, disabled: true, title: lockHint(t, "own-credentials", home) });
  }
  const active = new Set<string>(b.ticked);
  if (b.local) active.add(LOCAL);
  if (view.repoKind === "remote") active.add(REMOTE_HOME + view.repo);
  const localHomes = options.homes.map((h) => ({ value: h.id, label: homeOptionLabel(t, host, h) }));
  return (
    <div className="flex min-w-0 flex-col gap-2">
      <Selector
        items={items}
        label={t("placement.title")}
        select="many"
        inline
        active={active}
        hueOffset={hueOffset}
        disabled={disabled}
        onChange={(id) => {
          if (id === LOCAL) onLocal(!b.local);
          else if (id.startsWith(DESTINATION)) onDestination(id.slice(DESTINATION.length));
          else if (!id.startsWith(REMOTE_HOME)) onTarget(id, !active.has(id));
        }}
      />
      {b.local && localHomes.length > 1 && (
        <HomeSelect
          label={t("placement.storedOn")}
          value={view.repo}
          options={withStored(localHomes, view.repo, home)}
          locked={view.locked}
          disabled={disabled}
          onCommit={onHome}
        />
      )}
    </div>
  );
}
