import type { DefaultImpact } from "./api";
import type { useT } from "./i18n";
import type { CopiesPreview, HomePreview } from "./places";
import { formatList } from "./placement";

type T = ReturnType<typeof useT>["t"];

/** homeExpect is a "Stored in" preview as its write takes it back. The preview
 *  arrives inside the answer's envelope, and the server refuses a body with a
 *  field it does not know. */
export function homeExpect(p: HomePreview): HomePreview {
  return {
    mode: p.mode,
    placeId: p.placeId,
    homePlace: p.homePlace,
    homeHasBackups: p.homeHasBackups,
    repoId: p.repoId,
    creates: p.creates,
    impact: p.impact,
    backups: p.backups,
  };
}

/** copiesExpect is a chip's preview as its write takes it back. */
export function copiesExpect(p: CopiesPreview): CopiesPreview {
  return {
    placeId: p.placeId,
    on: p.on,
    targetId: p.targetId,
    suffix: p.suffix,
    skip: p.skip,
    enabled: p.enabled,
    newTarget: p.newTarget,
    impact: p.impact,
  };
}

/** impactLines says what a change of a domain's default does to each target,
 *  and how many items without a location take `home` at their first backup.
 *  Only containers have project folders, so only their lines name them. */
export function impactLines(t: T, lang: string, domain: string, impact: DefaultImpact, home: string): string[] {
  const containers = domain === "containers";
  const uncheckable = (names: string[]) => t("placement.uncheckable").replace("{list}", () => formatList(lang, names));
  const lines: string[] = [];
  for (const d of impact.dropped) {
    lines.push(
      d.unknown
        ? t(containers ? "placementDefaults.dropAskUnknownContainers" : "placementDefaults.dropAskUnknown")
            .replace(/\{target\}/g, () => d.name)
            .replace("{n}", String(d.items))
        : t(containers ? "placementDefaults.dropAskContainers" : "placementDefaults.dropAsk")
            .replace("{target}", () => d.name)
            .replace("{n}", String(d.items))
            .replace("{copies}", String(d.snapshots))
    );
    if (d.uncheckable.length > 0) lines.push(uncheckable(d.uncheckable));
  }
  for (const a of impact.added) {
    lines.push(
      t(containers ? "placementDefaults.addAskContainers" : "placementDefaults.addAsk")
        .replace("{target}", () => a.name)
        .replace("{n}", String(a.items))
        .replace("{snapshots}", String(a.snapshots))
    );
    if (a.uncheckable.length > 0) lines.push(uncheckable(a.uncheckable));
  }
  if (impact.openTakeHome > 0) {
    lines.push(t("placementDefaults.openTakeHome").replace("{home}", () => home).replace("{n}", String(impact.openTakeHome)));
  }
  return lines;
}
