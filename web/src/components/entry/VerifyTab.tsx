import type { AnomalyItem, Run } from "../../lib/api";
import { sortOpenAnomalies } from "../../lib/anomalies";
import { ENTRY_KIND } from "../../lib/backupEntry";
import { isGone } from "../../lib/backupList";
import { backupRuns, type Entry } from "../../lib/entry";
import { useT } from "../../lib/i18n";
import { useOpenAnomalies } from "../../lib/useAnomalies";
import { useItemChecks } from "../../lib/useItemChecks";
import { Card } from "../../pages/settings/shared";
import { ItemMonitoring } from "../anomalies/ItemMonitoring";
import { ItemChecksLine } from "../ItemChecksLine";
import { SizeBreakdown } from "../SizeBreakdown";
import { DedupScene } from "./DedupScene";
import { EntryRuns } from "./EntryRuns";
import { Note } from "./Note";

/**
 * VerifyTab says whether the entry's backups are any good: the checks that
 * read a backup back, what anomaly detection has learned and found, the
 * latest runs, and what the newest backup holds.
 */
export function VerifyTab({
  entry,
  runs,
  anomaly,
  anomalyEnabled,
  onRestore,
  nextHue,
}: {
  entry: Entry;
  runs: Run[];
  anomaly?: AnomalyItem;
  anomalyEnabled: boolean;
  /** Opens the restore window on the backup a run wrote. */
  onRestore: (run: Run) => void;
  nextHue: () => number;
}) {
  const { t } = useT();
  const { item, detail, settings } = entry;
  const itemChecks = useItemChecks();
  const { list } = useOpenAnomalies();
  const checks = itemChecks.find(item.kind, item.key);
  const hasBackup = item.lastBackup > 0;
  // The curve follows the entry's most pressing open finding.
  const finding = anomaly ? sortOpenAnomalies(list.filter((a) => a.targetId === anomaly.targetId))[0] : undefined;
  const newest = backupRuns(runs)[0] as Run | undefined;
  const breakdownDomain = ENTRY_KIND[item.kind].placementDomain;

  return (
    <>
      <Card title={t("entry.verify.checks")} hueIndex={nextHue()}>
        {!hasBackup ? (
          <p className="text-sm text-carbon-textMuted">{t("breakdown.none")}</p>
        ) : !checks ? (
          <p className="text-sm text-carbon-textMuted">{t("checks.restoreNever")}</p>
        ) : (
          <ItemChecksLine
            checks={checks}
            hasBackup
            onChanged={itemChecks.reload}
            startTest={item.kind === "container" && !isGone(item)}
          />
        )}
      </Card>

      <Card title={t("anomaly.title")} hint={t("entry.verify.anomalyHint")} hueIndex={nextHue()}>
        {anomaly ? (
          <>
            <ItemMonitoring
              t={t}
              item={anomaly}
              globals={{ sensitivity: settings.anomalySensitivity, notifyMin: settings.anomalyNotifyMin }}
              enabled={anomalyEnabled}
              finding={finding}
            />
            {anomaly.retentionHeld && <Note>{t("anomaly.retentionPaused")}</Note>}
          </>
        ) : (
          <p className="text-sm text-carbon-textMuted">{t(anomalyEnabled ? "entry.verify.notWatched" : "anomaly.off")}</p>
        )}
      </Card>

      <Card title={t("run.recentTitle")} hint={t("entry.verify.runsHint")} hueIndex={nextHue()}>
        <EntryRuns runs={runs} dataset={detail.kind === "zfs" ? detail.dataset : undefined} onRestore={onRestore} />
      </Card>

      <Card
        title={t("breakdown.title")}
        hint={newest ? `${t("entry.dedup.hint")}${breakdownDomain ? ` ${t("breakdown.hint")}` : ""}` : undefined}
        hueIndex={nextHue()}
      >
        {!newest && <p className="text-sm text-carbon-textMuted">{t("breakdown.none")}</p>}
        {newest?.sourceBytes != null && (
          <DedupScene id={`${item.kind}:${item.key}`} read={newest.sourceBytes} added={newest.bytes} />
        )}
        {newest && breakdownDomain && <SizeBreakdown domain={breakdownDomain} item={item.key} t={t} />}
      </Card>
    </>
  );
}
