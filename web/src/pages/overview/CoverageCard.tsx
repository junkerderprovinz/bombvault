import type { CoverageReport } from "../../lib/api";
import type { TranslationKey, useT } from "../../lib/i18n";
import { tLtr } from "../../lib/ltrFragments";
import type { Tally } from "../../lib/protectionTally";
import { zfsCodeSentence, zfsFixKey } from "../../lib/zfsCodes";
import { InfoBubble } from "../../components/InfoBubble";
import { Card } from "./Card";

/**
 * What on this server nothing backs up.
 *
 * The protection card is per domain and answers "did the scheduled
 * backups run on time". It cannot see the container nobody ever added: that one
 * is absent from every list and every error, so no traffic light turns amber
 * for it. This card names it.
 *
 * The ratio above the list counts entries as every other card does. The list
 * itself comes from GET /api/coverage, which knows why the next scheduled run
 * leaves something out.
 */
export function CoverageCard({
  t,
  coverage,
  tally,
  loading,
  hueIndex,
}: {
  t: ReturnType<typeof useT>["t"];
  coverage: CoverageReport | null;
  tally: Tally;
  loading: boolean;
  hueIndex?: number;
}) {
  const reasonKey: Record<string, TranslationKey> = {
    "not-set-up": "coverage.reason.notSetUp",
    "not-included": "coverage.reason.notIncluded",
    "override-off": "coverage.reason.overrideOff",
    "no-schedule": "coverage.reason.noSchedule",
    "db-dump-failing": "coverage.reason.dbDumpFailing",
    "db-dump-only-copy-off": "coverage.reason.dbDumpOnlyCopyOff",
    "db-not-scheduled": "coverage.reason.dbNotScheduled",
    "zfs-member-skipped": "coverage.reason.zfsMemberSkipped",
  };

  function skippedTip(code: string): string {
    const fix = zfsFixKey(code);
    return fix ? `${zfsCodeSentence(t, code)} ${tLtr(t, fix)}` : zfsCodeSentence(t, code);
  }

  const rows = (coverage?.domains ?? [])
    .filter((d) => d.enabled)
    .flatMap((d) => d.unprotected.map((i) => ({ ...i, domain: d.domain })));

  return (
    <Card title={t("coverage.title")} hint={t("overview.coverage.hint")} hueIndex={hueIndex}>
      {loading ? (
        <p className="text-sm text-carbon-textSub">{t("folder.loading")}</p>
      ) : (
        <div className="flex flex-col gap-2">
          <p className="flex items-center gap-1.5 text-base font-semibold tabular-nums text-carbon-text">
            {t("coverage.ratio").replace("{protected}", String(tally.ok)).replace("{total}", String(tally.total))}
            <InfoBubble tip={t("overview.coverage.countHint")} />
          </p>
          {rows.length === 0 && <p className="text-sm text-carbon-textSub">{t("coverage.allProtected")}</p>}
          <ul className="flex flex-col gap-1">
            {rows.map((r) => (
              <li key={r.domain + ":" + r.name} className="flex flex-wrap items-baseline gap-x-2">
                <span className="min-w-0 text-sm text-carbon-text wrap-anywhere">{r.name}</span>
                <span className="inline-flex items-center gap-1 text-xs text-carbon-textSub">
                  {t(reasonKey[r.reason] ?? "coverage.reason.noSchedule")}
                  {r.code && <InfoBubble tip={skippedTip(r.code)} />}
                </span>
                {r.neverBackedUp && (
                  <span className="text-xs text-statusWarn">{t("coverage.neverBackedUp")}</span>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}
    </Card>
  );
}
