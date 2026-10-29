import type { NamedRepo, OffsiteTarget } from "./api";
import type { TranslationKey, useT } from "./i18n";
import { DOMAIN_LABEL } from "./storageLocations";
import { offsiteTargetLabel } from "./useOffsiteTargets";

type T = ReturnType<typeof useT>["t"];

type Retention = Pick<
  OffsiteTarget,
  "retentionKeepLast" | "retentionKeepDaily" | "retentionKeepWeekly" | "retentionKeepMonthly" | "retentionKeepYearly"
>;

/** A target with its direct repository, while items back up to it. */
export interface DirectUse {
  target: OffsiteTarget;
  repo: NamedRepo;
}

/** retentionLowered is the server's rule: all zero keeps everything and the
 *  dimensions add up, so any one that shrinks keeps less. */
export function retentionLowered(before: Retention, after: Retention): boolean {
  const values = (r: Retention) => [
    r.retentionKeepLast,
    r.retentionKeepDaily,
    r.retentionKeepWeekly,
    r.retentionKeepMonthly,
    r.retentionKeepYearly,
  ];
  const b = values(before);
  const a = values(after);
  if (a.every((n) => n === 0)) return false;
  if (b.every((n) => n === 0)) return true;
  return a.some((n, i) => n < b[i]);
}

/** directUse is the target's direct repository when an item backs up to it. An
 *  unreadable count counts as in use. */
export function directUse(target: OffsiteTarget, repos: NamedRepo[]): DirectUse | undefined {
  const repo = repos.find((r) => r.companionOf === target.id && r.inUse !== 0);
  return repo ? { target, repo } : undefined;
}

/** itemsText adds up the items on direct repositories, "?" when a count could
 *  not be read. */
export function itemsText(repos: NamedRepo[]): string {
  return repos.some((r) => r.inUse < 0) ? "?" : String(repos.reduce((n, r) => n + r.inUse, 0));
}

/** directAsk is the question before a save that weakens direct repositories. */
export function directAsk(
  t: T,
  lang: string,
  key: "offsite.directRetentionAsk" | "offsite.directAppendOnlyAsk",
  uses: DirectUse[]
): string {
  const names = new Intl.ListFormat(lang, { type: "conjunction" }).format(uses.map((u) => offsiteTargetLabel(u.target)));
  return t(key).replace("{target}", names).replace("{n}", itemsText(uses.map((u) => u.repo)));
}

/** alsoDirectText is the hint that a target's rules bind its direct repository too. */
export function alsoDirectText(t: T, use: DirectUse): string {
  return t("offsite.alsoDirect").replace("{target}", offsiteTargetLabel(use.target)).replace("{n}", itemsText([use.repo]));
}

/** repoDisplayName is a repository's name on screen: its own, or for a direct
 *  repository without one its target's, with "direct" in the reader's language. */
export function repoDisplayName(t: (key: TranslationKey) => string, name: string, directOf?: string): string {
  return directOf ? t("placement.homeDirect").replace("{target}", () => directOf) : name;
}

/** idleDirect reports whether a direct repository takes no backups at all:
 *  no entry and no default writes there. */
export function idleDirect(r: NamedRepo): boolean {
  return r.companionOf !== "" && r.inUse === 0 && r.defaultFor?.length === 0;
}

/** inUseText says how many entries back up to a repository. A count the
 *  server could not read never reads as unused. */
export function inUseText(t: (key: TranslationKey) => string, r: NamedRepo): string {
  if (r.inUse < 0) return t("repos.inUseUnknown");
  return r.inUse > 0 ? t("repos.inUse").replace("{n}", String(r.inUse)) : t("repos.unused");
}

/** keptText says what backs up to a repository: the entries kept there, and
 *  the sections whose default writes there, whose entries the count leaves out. */
export function keptText(t: (key: TranslationKey) => string, lang: string, r: NamedRepo): string {
  const sections = r.defaultFor ?? [];
  if (sections.length === 0) return inUseText(t, r);
  const names = new Intl.ListFormat(lang, { type: "conjunction" }).format(sections.map((d) => t(DOMAIN_LABEL[d])));
  const byDefault = t("repos.byDefault").replace("{section}", () => names);
  return r.inUse !== 0 ? `${inUseText(t, r)} · ${byDefault}` : byDefault;
}
