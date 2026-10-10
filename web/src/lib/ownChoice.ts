// The entries that are backed up somewhere other than their section's default,
// and the way back for each.

import {
  listContainers,
  listFileSets,
  listVMs,
  type DefaultRow,
  type ItemRef,
  type PlacementChange,
  type PlacementDomain,
  type PlacementOptions,
  type PlacementView,
} from "./api";
import type { EntryKind } from "./backupEntry";
import type { useT } from "./i18n";
import { placementButtons, stepForReset } from "./placement";

type T = ReturnType<typeof useT>["t"];

export interface PlacedEntry {
  kind: EntryKind;
  item: ItemRef;
  name: string;
  view: PlacementView;
}

const READERS: Record<PlacementDomain, () => Promise<PlacedEntry[]>> = {
  containers: async () =>
    (await listContainers()).containers
      .filter((c) => !c.self)
      .map((c) => ({ kind: "container", item: { domain: "containers", key: c.name }, name: c.name, view: c.placement })),
  vms: async () =>
    (await listVMs()).vms.map((vm) => ({
      kind: "vm",
      item: { domain: "vms", key: vm.libvirtName },
      name: vm.name,
      view: vm.placement,
    })),
  files: async () =>
    (await listFileSets()).fileSets.map((set) => ({
      kind: "files",
      item: { domain: "files", key: set.id },
      name: set.name,
      view: set.placement,
    })),
};

/** The entries of the given sections with where each is backed up. A section
 *  whose list does not answer contributes none. */
export async function readPlacedEntries(domains: PlacementDomain[]): Promise<PlacedEntry[]> {
  const lists = await Promise.all(domains.map((domain) => READERS[domain]().catch((): PlacedEntry[] => [])));
  return lists.flat();
}

/**
 * hasOwnChoice reports whether an entry left its section's default: it copies
 * by a rule of its own, or it is stored somewhere else. The first backup fixes
 * where an entry is stored, so that alone is no choice of its own.
 */
export function hasOwnChoice(view: PlacementView, row: DefaultRow): boolean {
  if (view.unreadable) return false;
  return !view.copiesFollow || view.repo !== row.home;
}

/** The change that brings an entry back to its default, or null where nothing
 *  can change: an entry that copies like its section and is stored where its
 *  first backup put it. */
export function followChange(view: PlacementView): PlacementChange | null {
  if (view.locked && view.copiesFollow) return null;
  const step = stepForReset(view);
  return step.kind === "save" ? step.change : null;
}

/** Where a placement puts backups, by name: the place they are written to,
 *  then every copy. */
export function placeNames(t: T, view: PlacementView, options: PlacementOptions): string[] {
  const buttons = placementButtons(view, options);
  const names: string[] = [];
  if (buttons.local) names.push(view.repo === "" ? t("placement.segLocal") : view.repoLabel || view.repo);
  else if (view.repoKind === "remote") names.push(view.repoLabel || view.repo);
  for (const target of options.targets) {
    if (buttons.ticked.includes(target.id)) names.push(target.name);
  }
  return names;
}
