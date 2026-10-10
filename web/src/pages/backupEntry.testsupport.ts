// One staged entry of every kind for the tests of the entry page, and the
// answers of the list calls the page reads them through.
import type { Mock } from "vitest";
import type {
  AnomalyItem,
  BackupItem,
  Container,
  FileSetView,
  PlacementDomain,
  PlacementOptions,
  PlacementView,
  Run,
  Settings,
  TimelinePlace,
  TimelineRow,
  VM,
  ZFSDatasetView,
} from "../lib/api";

export const NOW = Math.floor(Date.now() / 1000);
const HOUR = 3600;
const DAY = 86400;

export const SET_ID = "a1b2c3d4e5f60718293a4b5c6d7e8f90";
export const ZFS_ID = "0f1e2d3c4b5a69788796a5b4c3d2e1f0";

const EVERYTHING = { kind: "everything", spec: "daily@03:00", alsoSpec: "" } as const;

export const PLACEMENT: PlacementView = {
  segment: "local-offsite",
  repo: "",
  repoLabel: "",
  repoKind: "domain",
  repoOff: false,
  homeFollows: true,
  copiesFollow: true,
  skip: [],
  locked: false,
  lockReason: "",
  segmentLocks: {} as PlacementView["segmentLocks"],
  paused: false,
  unreadable: false,
};

export function entryItem(over: Partial<BackupItem> = {}): BackupItem {
  return {
    kind: "container",
    key: "immich",
    name: over.key ?? "immich",
    included: true,
    paused: false,
    effectiveSchedule: EVERYTHING,
    lastBackup: NOW - 4 * HOUR,
    lastRunStatus: "success",
    sourceBytes: 65 * 1024 ** 2,
    state: "running",
    installed: true,
    runs: [],
    ...over,
  };
}

export const IMMICH = entryItem();
export const GITEA = entryItem({ key: "gitea", lastBackup: 0, lastRunStatus: "", sourceBytes: null, included: false });
export const PIHOLE = entryItem({ key: "pihole", installed: false, state: undefined, included: false, lastBackup: NOW - 24 * DAY });
export const SELF = entryItem({ key: "bombvault", self: true, lastBackup: 0, lastRunStatus: "", sourceBytes: null });
export const WIN11 = entryItem({ kind: "vm", key: "win11", name: "Windows 11" });
export const PHOTOS = entryItem({ kind: "files", key: SET_ID, name: "Photos", state: undefined, installed: undefined });
export const TANK = entryItem({ kind: "zfs", key: ZFS_ID, name: "tank/appdata", state: undefined, installed: undefined });
export const FLASH = entryItem({ kind: "flash", key: "flash", name: "flash", state: undefined, installed: undefined });
export const CONFIG = entryItem({ kind: "config", key: "config", name: "config", state: undefined, installed: undefined });

const ITEMS = [IMMICH, GITEA, PIHOLE, SELF, WIN11, PHOTOS, TANK, FLASH, CONFIG];

export function containerView(over: Partial<Container> = {}): Container {
  return {
    name: "immich",
    image: "ghcr.io/immich-app/immich-server:release",
    state: "running",
    status: "Up 3 days",
    ip: "192.168.20.40",
    installed: true,
    includeInSchedule: true,
    lastBackup: NOW - 4 * HOUR,
    lastBackupStarted: NOW - 4 * HOUR - 40,
    preHook: "",
    postHook: "",
    stopContainers: [],
    excludes: [],
    lastUpdateCheck: 0,
    lastUpdateResult: "",
    stack: "",
    placement: PLACEMENT,
    dbEngine: "",
    dbSuggestedEngine: "",
    dbTier: "",
    dbDumpOff: false,
    dbDumpEngine: "",
    dbDumpLabelOff: false,
    dbDumpsGlobalOff: false,
    dbDataCoverage: "",
    dbDumpHookOverlap: false,
    dumpOnly: false,
    ...over,
  };
}

const CONTAINERS = [
  containerView(),
  containerView({ name: "gitea", ip: "", lastBackup: null, lastBackupStarted: null, includeInSchedule: false }),
  containerView({ name: "pihole", installed: false, state: "", ip: "", includeInSchedule: false, lastBackup: NOW - 24 * DAY }),
  containerView({ name: "bombvault", self: true, lastBackup: null, lastBackupStarted: null }),
];

export function vmView(over: Partial<VM> = {}): VM {
  return {
    name: "Windows 11",
    libvirtName: "win11",
    state: "running",
    method: "stop",
    includeInSchedule: true,
    lastBackup: NOW - 4 * HOUR,
    lastBackupStarted: NOW - 5 * HOUR,
    placement: PLACEMENT,
    ...over,
  };
}

export function fileSetView(over: Partial<FileSetView> = {}): FileSetView {
  return {
    id: SET_ID,
    name: "Photos",
    path: "photos",
    excludes: [],
    enabled: true,
    lastBackup: NOW - 4 * HOUR,
    effectiveSchedule: EVERYTHING,
    pathExists: true,
    placement: PLACEMENT,
    ...over,
  };
}

export function zfsView(over: Partial<ZFSDatasetView> = {}): ZFSDatasetView {
  return {
    id: ZFS_ID,
    dataset: "tank/appdata",
    enabled: true,
    excludes: [],
    scheduleCadence: "",
    repo: "",
    repoEffective: "/mnt/user/backups/bombvault/zfs",
    stopContainers: [],
    restartPending: [],
    excludedChildren: [],
    hookContainer: "",
    preSnapshot: "",
    postSnapshot: "",
    hostMountpoint: "/mnt/tank/appdata",
    lastBackup: NOW - 4 * HOUR,
    lastRunStatus: "success",
    lastCheckCode: "ok",
    lastCheckDetail: "",
    lastCheckAt: NOW - 4 * HOUR,
    leftoverCount: 0,
    safetyCount: 0,
    safetyOldestAt: 0,
    members: [
      {
        dataset: "tank/appdata",
        relPath: "",
        hostMountpoint: "/mnt/tank/appdata",
        outcome: "backed-up",
        isNew: false,
        usedByDataset: 1024 ** 3,
        lastBackupAt: NOW - 4 * HOUR,
      },
    ],
    effectiveSchedule: EVERYTHING,
    sites: 1,
    rule321: "one-copy",
    ...over,
  } as ZFSDatasetView;
}

export function backupRun(id: string, daysAgo: number, over: Partial<Run> = {}): Run {
  return {
    id,
    targetId: "tg-immich",
    kind: "backup",
    status: "success",
    startedAt: NOW - 4 * HOUR - daysAgo * DAY - 44,
    finishedAt: NOW - 4 * HOUR - daysAgo * DAY,
    snapshotId: `${id}00aa11bb22cc`,
    bytes: 2 * 1024 ** 2,
    sourceBytes: 65 * 1024 ** 2,
    error: "",
    acknowledged: false,
    target: "immich",
    domain: "container",
    ...over,
  };
}

export function watched(over: Partial<AnomalyItem> = {}): AnomalyItem {
  return {
    targetId: "tg-immich",
    domain: "container",
    name: "immich",
    scheduled: true,
    sensitivity: "",
    effective: "balanced",
    notifyMin: "",
    effectiveNotifyMin: "warning",
    learning: { samples: 10, needed: 10, newData: 10, source: 10, duration: 10, noData: false },
    typical: { sourceBytes: 1024, newDataBytes: 1024, resticMs: 1000 },
    dump: null,
    datasets: [],
    open: { critical: 0, warning: 0, info: 0 },
    retentionHeld: false,
    selectionSince: 0,
    expectations: [],
    ...over,
  };
}

export const SETTINGS = {
  containersEnabled: true,
  vmsEnabled: true,
  flashEnabled: true,
  filesEnabled: true,
  zfsEnabled: true,
  configEnabled: true,
  dbDumpsEnabled: true,
  anomalyEnabled: true,
  anomalySensitivity: "balanced",
  anomalyNotifyMin: "critical",
  perItemSchedules: false,
  restoreFolder: "user/bombvault/restore",
  flashZipExportEnabled: false,
  flashZipExportPath: "",
  flashZipExportKeep: 0,
  everythingSchedule: "daily@03:00",
} as Settings;

export interface EntryMocks {
  listItems: Mock;
  listContainers: Mock;
  listVMs: Mock;
  listFileSets: Mock;
  listZFSDatasets: Mock;
  listItemRuns: Mock;
  getSettings: Mock;
}

export interface Staged {
  /** Replace the staged items of the same kind and key. */
  items?: BackupItem[];
  containers?: Container[];
  vms?: VM[];
  fileSets?: FileSetView[];
  datasets?: ZFSDatasetView[];
  runs?: Run[];
  settings?: Partial<Settings>;
}

function replaced<T>(base: T[], over: T[] | undefined, same: (a: T, b: T) => boolean): T[] {
  if (!over) return base;
  return [...base.filter((b) => !over.some((o) => same(o, b))), ...over];
}

/** serveEntries answers the calls the page reads an entry through. */
export function serveEntries(mocks: EntryMocks, staged: Staged = {}): void {
  mocks.listItems.mockResolvedValue({
    ok: true,
    items: replaced(ITEMS, staged.items, (a, b) => a.kind === b.kind && a.key === b.key),
    unlisted: [],
  });
  mocks.listContainers.mockResolvedValue({
    ok: true,
    containers: replaced(CONTAINERS, staged.containers, (a, b) => a.name === b.name),
  });
  mocks.listVMs.mockResolvedValue({ ok: true, vms: staged.vms ?? [vmView()] });
  mocks.listFileSets.mockResolvedValue({ ok: true, fileSets: staged.fileSets ?? [fileSetView()] });
  mocks.listZFSDatasets.mockResolvedValue({ ok: true, datasets: staged.datasets ?? [zfsView()] });
  mocks.listItemRuns.mockResolvedValue({ ok: true, runs: staged.runs ?? [] });
  mocks.getSettings.mockResolvedValue({
    ok: true,
    settings: { ...SETTINGS, ...staged.settings },
    hostMountRoot: "/host/user",
    platform: "unraid",
  });
}

const LOCAL: TimelinePlace = {
  place: "local",
  label: "",
  kind: "home",
  remote: false,
  enabled: true,
  appendOnly: false,
  state: "read",
};

export const BOX: TimelinePlace = {
  place: "offsite:t1",
  label: "Storage Box",
  kind: "target",
  remote: true,
  enabled: true,
  appendOnly: false,
  state: "unchecked",
};

/** timelineOf is the timeline the successful runs left: one row each at the
 *  local place, and the given further places not read yet. */
export function timelineOf(runs: Run[], more: TimelinePlace[] = []) {
  const rows: TimelineRow[] = runs
    .filter((run) => run.status === "success")
    .map((run) => ({
      key: run.snapshotId,
      time: new Date((run.finishedAt ?? run.startedAt) * 1000).toISOString(),
      places: [{ place: "local", snapshotIds: [run.snapshotId], tags: [] }],
    }));
  return { ok: true, places: [LOCAL, ...more], rows };
}

/** The storage locations a kind can use: its own repository and one target. */
export function placementOptions(domain: PlacementDomain): PlacementOptions {
  return {
    domain,
    unreadable: false,
    paused: false,
    homes: [{ id: "", name: "", location: `/mnt/user/backups/${domain}`, kind: "domain", scheme: "" }],
    targets: [{ id: "t1", name: "Storage Box", enabled: true, primary: false, appendOnly: false, hint: "" }],
    sendTo: [],
    segmentLocks: {} as PlacementOptions["segmentLocks"],
    default: {
      domain,
      exists: true,
      home: "",
      homeKind: "domain",
      homeOff: false,
      skip: [],
      paused: false,
      confirmedAt: NOW,
      counts: {} as PlacementOptions["default"]["counts"],
      unreadable: false,
    },
  };
}
