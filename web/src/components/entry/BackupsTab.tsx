import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import type { Run } from "../../lib/api";
import { isGone } from "../../lib/backupList";
import { historyDays, rowHolds, rowOfRun, rowTime, type Entry } from "../../lib/entry";
import { useT } from "../../lib/i18n";
import type { RestoreRequest } from "../../lib/restoreRequest";
import { restoreFolderFor, snapshotIds } from "../../lib/restorePlan";
import { useOpenAnomalies } from "../../lib/useAnomalies";
import { Card } from "../../pages/settings/shared";
import { DatabaseDumpList } from "../restore/DatabaseDumpList";
import { MissingRestorePoint } from "../restore/MissingRestorePoint";
import { RecreateButton } from "../restore/RecreateButton";
import { RestoreWindow } from "../restore/RestoreWindow";
import { Selector } from "../Selector";
import { SourceToggle, type RepoSource } from "../SourceToggle";
import { BackupHistory, hasHistory } from "./BackupHistory";
import { BackupList, type OpenBackup, type RowAction } from "./BackupList";
import { BackupPlaces, type TimelineState } from "./BackupPlaces";
import { CompareWindow } from "./CompareWindow";
import { DeleteBackupWindow } from "./DeleteBackupWindow";
import { ExportWindow } from "./ExportWindow";
import { TagWindow } from "./TagWindow";

type View = "backups" | "dumps";

// How long the row a bar pointed at stays marked.
const LOCATED_MS = 1400;

function useFlaggedRun(): (run: Run) => boolean {
  const { byRunId } = useOpenAnomalies();
  return (run) => (byRunId.get(run.id) ?? []).some((a) => a.severity !== "info");
}

function KeepLine() {
  const { t } = useT();
  const [before, after] = t("entry.list.keep").split("{location}");
  return (
    <p className="text-xs text-carbon-textMuted">
      {before}
      <Link to="/storage" className="underline hover:text-carbon-text">
        {t("entry.list.keepLink")}
      </Link>
      {after}
    </p>
  );
}

/**
 * BackupsTab shows what the entry has in store: the history of the last two
 * weeks, the storage locations that hold its backups and the backups
 * themselves. A container that is a database lists its dumps beside them.
 */
export function BackupsTab({
  entry,
  name,
  runs,
  timeline,
  opened,
  onOpen,
  request,
  nextHue,
}: {
  entry: Entry;
  name: string;
  runs: Run[];
  timeline: TimelineState;
  opened: OpenBackup | null;
  onOpen: (open: OpenBackup | null) => void;
  /** The backup a finding's restore link asks for. */
  request: RestoreRequest;
  nextHue: () => number;
}) {
  const { t } = useT();
  const { flagged } = useOpenAnomalies();
  const isFlagged = useFlaggedRun();
  const { item, detail, settings, hostMountRoot } = entry;
  const container = detail.kind === "container" ? detail.container : null;
  const installed = detail.kind === "container" ? detail.installed : [];
  const days = historyDays(runs, isFlagged);
  const hasDumps = container !== null && (container.dbTier !== "" || request.dump);
  const [view, setView] = useState<View>(request.dump ? "dumps" : "backups");
  const [dumpSource, setDumpSource] = useState<RepoSource>("local");
  const [located, setLocated] = useState<string | null>(null);
  const [acting, setActing] = useState<{ key: string; action: RowAction } | null>(null);

  useEffect(() => {
    if (!located) return;
    const timer = setTimeout(() => setLocated(null), LOCATED_MS);
    return () => clearTimeout(timer);
  }, [located]);

  const { rows, places, loading, error } = timeline;
  const pending = places.filter((p) => p.state !== "read");
  const empty = !loading && error === null && rows.length === 0 && pending.length === 0;
  const openedRow = opened ? rows.find((row) => rowHolds(row, opened.key)) : undefined;
  const actingRow = acting ? rows.find((row) => row.key === acting.key) : undefined;
  const wanted = request.dump ? "" : request.snapshot;
  const requestedRow = wanted ? rows.find((row) => rowHolds(row, wanted)) : undefined;
  const done = () => setActing(null);

  // A finding names one backup, for a ZFS entry that of one dataset. The
  // window of a ZFS entry opens even where that backup is gone, on the
  // nearest one, and says so.
  const linked =
    opened !== null && wanted !== "" && (opened.key === wanted || (openedRow !== undefined && openedRow === requestedRow));
  const restoring =
    opened !== null && (openedRow !== undefined || (linked && detail.kind === "zfs" && rows.length > 0));

  function locate(run: Run) {
    const row = rowOfRun(rows, run);
    if (!row) return;
    setView("backups");
    setLocated(row.key);
  }

  return (
    <>
      {hasHistory(days) && <BackupHistory days={days} onLocate={locate} hueIndex={nextHue()} />}
      <BackupPlaces
        timeline={timeline}
        replicaOf={detail.kind === "zfs" ? detail.dataset : undefined}
        hueIndex={nextHue()}
      />
      <Card
        title={t(view === "dumps" ? "dbdump.listTitle" : "backups.title")}
        hint={t("entry.list.hint")}
        hueIndex={nextHue()}
      >
        {hasDumps && (
          <div className="flex flex-wrap items-center gap-2">
            <Selector
              items={[
                { id: "backups", label: t("backups.title") },
                { id: "dumps", label: t("dbdump.listTitle") },
              ]}
              label={t("backups.title")}
              select="one"
              variant="well"
              inline
              active={view}
              onChange={(id) => setView(id as View)}
            />
            {view === "dumps" && (
              <span className="flex flex-wrap items-center gap-2 text-xs text-carbon-textMuted">
                {t("source.label")}
                <SourceToggle source={dumpSource} onChange={setDumpSource} domain="containers" inline />
              </span>
            )}
          </div>
        )}

        <div hidden={view !== "backups"} className="flex flex-col gap-4">
          {loading && <p className="text-sm text-carbon-textMuted">{t("common.loadingBackups")}</p>}
          {error !== null && <p className="text-sm text-statusFail">{error || t("common.loadBackupsFailed")}</p>}
          {/* A ZFS entry says this in its window, which opens on the nearest backup. */}
          {wanted && detail.kind !== "zfs" && places.length > 0 && pending.length === 0 && (
            <MissingRestorePoint
              requested={wanted}
              requestedAt={request.at}
              points={rows.map((row) => ({ id: row === requestedRow ? wanted : row.key, at: rowTime(row) }))}
              t={t}
            />
          )}
          {empty &&
            (container && isGone(item) ? (
              <RecreateButton name={container.name} source="local" t={t} />
            ) : (
              <p className="text-sm text-carbon-textMuted">
                {container && item.lastBackup > 0
                  ? t("snapshots.configOnlyHint")
                  : t("entry.list.empty").replace("{name}", () => name)}
              </p>
            ))}
          <BackupList
            timeline={timeline}
            runs={runs}
            isFlagged={(row, run) => snapshotIds(row).some((id) => flagged.has(id)) || (run !== undefined && isFlagged(run))}
            located={located}
            requested={requestedRow?.key ?? ""}
            onOpen={onOpen}
            onAct={(row, action) => setActing({ key: row.key, action })}
          />
          <KeepLine />
        </div>

        {hasDumps && container && (
          <div hidden={view !== "dumps"}>
            <DatabaseDumpList
              containerName={container.name}
              source={dumpSource}
              recognised={container.dbTier !== ""}
              canImport={container.installed && container.state === "running"}
              importStops={(container.stopContainers ?? []).filter((dep) =>
                installed.some((c) => c.name === dep && c.state === "running")
              )}
              hostMountRoot={hostMountRoot}
              defaultFolder={restoreFolderFor(settings, item.kind)}
              reloadTick={0}
              preselect={request.dump ? request.snapshot : ""}
              preselectAt={request.at}
              t={t}
            />
          </div>
        )}
      </Card>

      {opened && restoring && (
        <RestoreWindow
          key={opened.key}
          item={item}
          entry={entry}
          name={name}
          timeline={timeline}
          ask={
            linked
              ? { snapshot: wanted, at: request.at, place: opened.place, dataset: request.dataset }
              : { snapshot: opened.key, place: opened.place }
          }
          deletable
          onClose={() => onOpen(null)}
        />
      )}
      {actingRow && acting?.action === "tag" && (
        <TagWindow
          entry={entry}
          name={name}
          row={actingRow}
          timeline={timeline}
          onSaved={() => {
            done();
            timeline.reload();
          }}
          onClose={done}
        />
      )}
      {actingRow && acting?.action === "compare" && (
        <CompareWindow entry={entry} name={name} row={actingRow} timeline={timeline} onClose={done} />
      )}
      {actingRow && acting?.action === "export" && (
        <ExportWindow entry={entry} name={name} row={actingRow} timeline={timeline} onClose={done} />
      )}
      {actingRow && acting?.action === "delete" && (
        <DeleteBackupWindow entry={entry} name={name} row={actingRow} timeline={timeline} onClose={done} />
      )}
    </>
  );
}
