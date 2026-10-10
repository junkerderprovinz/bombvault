import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import {
  zfsHostDatasets,
  type DBDumpView,
  type Run,
  type TimelineRow,
  type ZFSDatasetView,
  type ZFSHostDataset,
} from "../../lib/api";
import { isGone } from "../../lib/backupList";
import { historyDays, rowOfRun, type Entry } from "../../lib/entry";
import { useT } from "../../lib/i18n";
import type { RestoreRequest } from "../../lib/restoreRequest";
import { useOpenAnomalies } from "../../lib/useAnomalies";
import { Card } from "../../pages/settings/shared";
import { AnomalyWindow } from "../anomalies/AnomalyWindow";
import { Button } from "../Button";
import { IconCompare } from "../glyphs";
import { DatabaseDumpList } from "../restore/DatabaseDumpList";
import { MissingRestorePoint } from "../restore/MissingRestorePoint";
import { CompareSnapshotsForm, DEFAULT_RESTORE_FOLDER, RecreateButton, displayTags } from "../RestorePanel";
import { Selector } from "../Selector";
import { SourceToggle, type RepoSource } from "../SourceToggle";
import { ZFSRestorePanel } from "../zfs/ZFSRestorePanel";
import { BackupHistory, hasHistory } from "./BackupHistory";
import { BackupList, type OpenBackup } from "./BackupList";
import { BackupPlaces, ZFSPlaces, type TimelineState } from "./BackupPlaces";
import { RestoreWindow } from "./RestoreWindow";

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
  const [dumps, setDumps] = useState<DBDumpView[]>([]);
  const [located, setLocated] = useState<string | null>(null);
  const [comparing, setComparing] = useState(false);

  useEffect(() => {
    if (!located) return;
    const timer = setTimeout(() => setLocated(null), LOCATED_MS);
    return () => clearTimeout(timer);
  }, [located]);

  const { rows, places, loading, error } = timeline;
  const pending = places.filter((p) => p.state !== "read");
  const empty = !loading && error === null && rows.length === 0 && pending.length === 0;
  const openedRow = opened ? rows.find((row) => row.key === opened.key) : undefined;
  const restoreFolder = settings.restoreFolder || DEFAULT_RESTORE_FOLDER;

  function locate(run: Run) {
    const row = rowOfRun(rows, run);
    if (!row) return;
    setView("backups");
    setLocated(row.key);
  }

  const tagsOf = (row: TimelineRow) =>
    container ? displayTags([...new Set(row.places.flatMap((m) => m.tags))], container.name, container.aliases ?? []) : [];

  return (
    <>
      {hasHistory(days) && <BackupHistory days={days} onLocate={locate} hueIndex={nextHue()} />}
      <BackupPlaces timeline={timeline} hueIndex={nextHue()} />
      <Card
        title={t(view === "dumps" ? "dbdump.listTitle" : "backups.title")}
        hint={t("entry.list.hint")}
        hueIndex={nextHue()}
      >
        {(hasDumps || (container && rows.length > 1)) && (
          <div className="flex flex-wrap items-center gap-2">
            {hasDumps && (
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
            )}
            {view === "dumps" && (
              <span className="flex flex-wrap items-center gap-2 text-xs text-carbon-textMuted">
                {t("source.label")}
                <SourceToggle source={dumpSource} onChange={setDumpSource} domain="containers" inline />
              </span>
            )}
            {view === "backups" && container && rows.length > 1 && (
              <span className="ms-auto">
                <Button
                  label={t("snapshot.compare")}
                  labelKey="snapshot.compare"
                  glyph={<IconCompare />}
                  onClick={() => setComparing(true)}
                />
              </span>
            )}
          </div>
        )}

        <div hidden={view !== "backups"} className="flex flex-col gap-4">
          {loading && <p className="text-sm text-carbon-textMuted">{t("common.loadingBackups")}</p>}
          {error !== null && <p className="text-sm text-statusFail">{error || t("common.loadBackupsFailed")}</p>}
          {request.snapshot && !request.dump && places.length > 0 && pending.length === 0 && (
            <MissingRestorePoint
              requested={request.snapshot}
              requestedAt={request.at}
              points={rows.map((row) => ({ id: row.key, at: Math.floor(Date.parse(row.time) / 1000) }))}
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
            isFlagged={(row, run) => flagged.has(row.key) || (run !== undefined && isFlagged(run))}
            tagsOf={tagsOf}
            located={located}
            requested={request.dump ? "" : request.snapshot}
            onOpen={onOpen}
          />
          <KeepLine />
        </div>

        {hasDumps && container && (
          // Mounted on both views, because the list of backups marks the
          // backup a dump was taken with.
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
              defaultFolder={restoreFolder}
              reloadTick={0}
              onDumps={setDumps}
              preselect={request.dump ? request.snapshot : ""}
              preselectAt={request.at}
              t={t}
            />
          </div>
        )}
      </Card>

      {opened && openedRow && (
        <RestoreWindow
          key={opened.key}
          entry={entry}
          name={name}
          row={openedRow}
          place={opened.place}
          timeline={timeline}
          dumps={dumps}
          onClose={() => onOpen(null)}
        />
      )}
      {comparing && container && (
        <AnomalyWindow title={t("snapshot.compare")} onClose={() => setComparing(false)}>
          <CompareSnapshotsForm containerName={container.name} t={t} />
        </AnomalyWindow>
      )}
    </>
  );
}

/**
 * ZFSBackupsTab is the Backups tab of a ZFS entry, whose backups are restore
 * points of a whole dataset tree and come with a panel of their own.
 */
export function ZFSBackupsTab({
  entry,
  dataset,
  runs,
  request,
  nextHue,
}: {
  entry: Entry;
  dataset: ZFSDatasetView;
  runs: Run[];
  request: RestoreRequest;
  nextHue: () => number;
}) {
  const { t } = useT();
  const isFlagged = useFlaggedRun();
  const days = historyDays(runs, isFlagged);
  const [host, setHost] = useState<ReadonlyMap<string, ZFSHostDataset>>(new Map());

  // The listing goes over SSH. Without it the panel still restores, and the
  // server checks again whether a dataset can be written into.
  useEffect(() => {
    zfsHostDatasets()
      .then((res) => {
        if (res.ok && res.available) setHost(new Map(res.datasets.map((d) => [d.dataset, d])));
      })
      .catch(() => undefined);
  }, []);

  return (
    <>
      {hasHistory(days) && <BackupHistory days={days} hueIndex={nextHue()} />}
      <ZFSPlaces dataset={dataset} hueIndex={nextHue()} />
      <Card title={t("backups.title")} hueIndex={nextHue()}>
        <ZFSRestorePanel
          item={dataset}
          host={host}
          hostMountRoot={entry.hostMountRoot}
          restoreFolder={entry.settings.restoreFolder || DEFAULT_RESTORE_FOLDER}
          preselect={request.snapshot ? request : undefined}
        />
      </Card>
    </>
  );
}
