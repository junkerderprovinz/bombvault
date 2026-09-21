import type { NamedRepo, OffsiteTarget } from "./api";
import type { useT } from "./i18n";
import { offsiteTargetLabel } from "./useOffsiteTargets";

type T = ReturnType<typeof useT>["t"];

type Retention = Pick<OffsiteTarget, "retentionKeepLast" | "retentionKeepDaily" | "retentionKeepWeekly" | "retentionKeepMonthly">;

/** A target with its direct repository, while items back up to it. */
export interface DirectUse {
  target: OffsiteTarget;
  repo: NamedRepo;
}

/** retentionLowered is the server's rule: all zero keeps everything and the
 *  dimensions add up, so any one that shrinks keeps less. */
export function retentionLowered(before: Retention, after: Retention): boolean {
  const values = (r: Retention) => [r.retentionKeepLast, r.retentionKeepDaily, r.retentionKeepWeekly, r.retentionKeepMonthly];
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

/** primaryDirects lists each domain's field target whose direct repository is
 *  in use; the off-site retention settings edit exactly those rows. */
export function primaryDirects(targets: OffsiteTarget[], repos: NamedRepo[]): DirectUse[] {
  return targets.flatMap((target) => {
    const use = target.sortOrder === 0 ? directUse(target, repos) : undefined;
    return use ? [use] : [];
  });
}
