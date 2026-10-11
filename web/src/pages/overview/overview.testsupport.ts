// What the overview's tests share: the records a server answers with, a
// staged api module the tests steer, and the two browser interfaces jsdom
// lacks. The mock of the api module loads this file, so nothing here may
// import a module that imports the api.
import { vi } from "vitest";

import type {
  AnomalySummary,
  AnomalyView,
  BackupItem,
  DomainStatus,
  Run,
  Settings,
  StorageLocation,
  StorageLocationSection,
} from "../../lib/api";
import { DESKTOP_QUERY } from "../../lib/useMediaQuery";

export const NOW = Math.floor(Date.now() / 1000);
export const HOUR = 3600;
export const DAY = 86400;

/** A section whose four checks are all green. */
export function domain(over: Partial<DomainStatus> = {}): DomainStatus {
  return {
    domain: "containers",
    enabled: true,
    schedule: "daily 03:00",
    coveredBy: "",
    lastSuccess: NOW - 2 * HOUR,
    periodSeconds: DAY,
    status: "ok",
    lastVerified: 0,
    lastVerifiedOK: false,
    verifiedDetail: "",
    drillDetail: "",
    offsiteConfigured: true,
    offPremisesCovered: false,
    offsiteImmutable: true,
    lastTamperAt: NOW - 3 * DAY,
    lastTamperOK: true,
    lastReplicationAt: NOW - 3 * HOUR,
    lastReplicationOK: true,
    lastDrDrillAt: NOW - 5 * DAY,
    lastDrDrillOK: true,
    lastOffsiteSubsetAt: 0,
    lastOffsiteSubsetOK: false,
    offsiteDrillScheduled: true,
    protection: "green",
    tamperState: "ok",
    replicationState: "ok",
    drillState: "ok",
    encryptionOn: true,
    pruneStrategySet: true,
    ...over,
  };
}

/** A protected container that is in the schedule. */
export function item(over: Partial<BackupItem> = {}): BackupItem {
  return {
    kind: "container",
    key: "plex",
    name: "plex",
    included: true,
    paused: false,
    effectiveSchedule: { kind: "domain", spec: "daily 03:00", alsoSpec: "" },
    lastBackup: NOW - 2 * HOUR,
    lastRunStatus: "success",
    sourceBytes: 1024,
    state: "running",
    installed: true,
    runs: [],
    ...over,
  };
}

/** A finished, successful backup of plex. */
export function run(over: Partial<Run> & { id: string }): Run {
  return {
    targetId: "plex",
    kind: "backup",
    status: "success",
    startedAt: NOW - 60,
    finishedAt: NOW - 30,
    snapshotId: "abc12345",
    bytes: 1024,
    error: "",
    acknowledged: false,
    target: "plex",
    domain: "container",
    ...over,
  };
}

export function finding(over: Partial<AnomalyView> & { id: string }): AnomalyView {
  return {
    detector: "source",
    metric: "source_bytes_shrink",
    severity: "critical",
    state: "open",
    scopeKind: "item",
    scopeId: "tg-plex",
    targetId: "tg-plex",
    domain: "container",
    part: "",
    targetName: "",
    name: "plex",
    runId: "run-1",
    lastRunId: "run-1",
    lastRunAt: NOW - HOUR,
    observed: 2 * 1024 ** 3,
    expected: 40 * 1024 ** 3,
    threshold: 20 * 1024 ** 3,
    samples: 14,
    sensitivity: "balanced",
    details: {},
    occurrences: 1,
    firstSeenAt: NOW - HOUR,
    lastSeenAt: NOW - HOUR,
    recoveredAt: 0,
    resolvedAt: 0,
    ackedAt: 0,
    clearedAt: 0,
    ackNote: "",
    notifiedAt: 0,
    expectable: false,
    retentionHeld: false,
    stillPresent: false,
    ...over,
  } as AnomalyView;
}

export function location(over: Partial<StorageLocation> = {}): StorageLocation {
  return {
    id: "path:1",
    object: "path",
    kind: "local",
    provider: "",
    backend: "local",
    name: "bombvault",
    where: "/mnt/user/backups",
    enabled: true,
    offPremises: false,
    sections: [],
    protection: { immutable: false, testable: false },
    capacity: {
      usedBytes: 400 * 1024 ** 3,
      totalBytes: 1000 * 1024 ** 3,
      growthBytesPerWeek: 10 * 1024 ** 3,
      weeksToFull: 33,
    },
    ...over,
  };
}

/** An off-site copy of the containers that has failed since it last worked. */
export function failedCopy(over: Partial<StorageLocationSection> = {}): StorageLocationSection {
  return {
    domain: "containers",
    use: "copy",
    targetId: "t-containers",
    where: "rclone:box:containers",
    enabled: true,
    immutable: false,
    retention: { keepLast: 0, keepDaily: 7, keepWeekly: 4, keepMonthly: 6, keepYearly: 1 },
    compression: "auto",
    limitUpload: 0,
    limitDownload: 0,
    own: [],
    lastCopy: { at: NOW - 2 * DAY, ok: false, failingSince: NOW - DAY },
    ...over,
  };
}

type Answer<T> = T | "refused" | "pending";

/**
 * What the staged server holds. A test sets the fields it cares about before
 * it renders; "refused" answers with ok false and "pending" never answers.
 */
export const server = {
  domains: [] as Answer<DomainStatus[]>,
  items: [] as Answer<BackupItem[]>,
  runs: [] as Answer<Run[]>,
  locations: [] as Answer<StorageLocation[]>,
  findings: [] as AnomalyView[],
  /** Whether anomaly detection is on. */
  detection: false,
  /** Encryption is on and the recovery kit is not confirmed. */
  kitDue: false,
  /** What the settings hold beyond the recovery kit. */
  settings: {} as Partial<Settings>,
  platform: "unraid",
  /** Why the server refuses to save the settings, "" while it saves them. */
  saveRefused: "",
  /** Why the server refuses to take every entry of a kind into the schedule. */
  includeRefused: "",
  /** Every call to a function that changes something, by its name. */
  calls: [] as { name: string; args: unknown[] }[],
};

export function resetServer(): void {
  Object.assign(server, {
    domains: [],
    items: [],
    runs: [],
    locations: [],
    findings: [],
    detection: false,
    kitDue: false,
    settings: {},
    platform: "unraid",
    saveRefused: "",
    includeRefused: "",
    calls: [],
  });
}

function answer<T, R extends object>(value: Answer<T>, wrap: (v: T) => R): Promise<R | { ok: false }> {
  if (value === "pending") return new Promise(() => {});
  if (value === "refused") return Promise.resolve({ ok: false });
  return Promise.resolve(wrap(value));
}

function summary(): AnomalySummary {
  const open = (severity: string) => server.findings.filter((f) => f.severity === severity).length;
  return {
    enabled: server.detection,
    ready: true,
    generation: 1,
    open: { critical: open("critical"), warning: open("warning"), info: open("info") },
    recoveredCritical: 0,
    learningItems: 0,
    retentionHeld: 0,
    evalErrors: 0,
    notifyMuted: false,
    backfill: { slots: 0, done: 0, failed: 0, filled: 0, withoutSummary: 0 },
    unmeasuredVolumes: [],
  } as AnomalySummary;
}

// Takes every entry of a kind into the schedule or out of it, as the server
// does for the containers and the VMs its host lists.
function includeAll(name: string, kind: BackupItem["kind"], include: boolean) {
  server.calls.push({ name, args: [include] });
  if (server.includeRefused) return Promise.resolve({ ok: false, error: server.includeRefused });
  if (Array.isArray(server.items)) {
    server.items = server.items.map((entry) => (entry.kind === kind && !entry.self ? { ...entry, included: include } : entry));
  }
  return Promise.resolve({ ok: true });
}

/** stagedApi replaces every call the overview makes with one that reads `server`. */
export function stagedApi<M extends object>(actual: M): M {
  const record =
    (name: string, result: unknown = { ok: true }) =>
    (...args: unknown[]) => {
      server.calls.push({ name, args });
      return Promise.resolve(result);
    };
  return {
    ...actual,
    getStatus: () => answer(server.domains, (domains) => ({ ok: true, domains })),
    listItems: () => answer(server.items, (items) => ({ ok: true, items, unlisted: [] })),
    listRuns: vi.fn(() => answer(server.runs, (runs) => ({ ok: true, runs }))),
    listStorageLocations: vi.fn(() => answer(server.locations, (locations) => ({ ok: true, locations }))),
    getCoverage: () => Promise.resolve({ ok: true, coverage: { total: 0, protected: 0, domains: [] } }),
    getScheduleNext: () => Promise.resolve([]),
    getSettings: () =>
      Promise.resolve({
        ok: true,
        settings: { encryptionEnabled: server.kitDue, recoveryKitAck: false, ...server.settings },
        platform: server.platform,
      }),
    putSettings: (settings: Settings) => {
      server.calls.push({ name: "putSettings", args: [settings] });
      if (server.saveRefused) return Promise.resolve({ ok: false, error: server.saveRefused });
      server.settings = settings;
      return Promise.resolve({ ok: true });
    },
    getAnomalySummary: () => Promise.resolve({ ok: true, summary: summary() }),
    getAnomalies: () => Promise.resolve({ ok: true, anomalies: server.findings, nextCursor: "" }),
    getStats: () => Promise.resolve({ ok: false }),
    getHistory: () => Promise.resolve({ ok: true, days: [] }),
    getSpike: () => Promise.resolve({ checks: [], allOk: true }),
    backupEverythingNow: record("backupEverythingNow", { ok: true, started: true }),
    backupNow: record("backupNow"),
    setInclude: record("setInclude"),
    setIncludeAll: (include: boolean) => includeAll("setIncludeAll", "container", include),
    setVMIncludeAll: (include: boolean) => includeAll("setVMIncludeAll", "vm", include),
    ackRecoveryKit: (...args: unknown[]) => {
      server.kitDue = false;
      return record("ackRecoveryKit")(...args);
    },
    downloadRecoveryKit: record("downloadRecoveryKit", null),
    replicateOffsite: record("replicateOffsite"),
    ackFailedCopies: (targetIds: string[]) => {
      if (Array.isArray(server.locations)) {
        server.locations = server.locations.map((place) => ({
          ...place,
          sections: place.sections.map((section) =>
            section.lastCopy && targetIds.includes(section.targetId ?? "")
              ? { ...section, lastCopy: { ...section.lastCopy, acknowledged: true } }
              : section
          ),
        }));
      }
      return record("ackFailedCopies")(targetIds);
    },
    runDrill: record("runDrill", { ok: true, drill: { ok: true } }),
  };
}

const mediaListeners = new Set<() => void>();
let desktop = true;

/** Answers the desktop width query with what the test last set. */
export function installMatchMedia(): void {
  desktop = true;
  window.matchMedia = ((query: string) => ({
    get matches() {
      return query === DESKTOP_QUERY ? desktop : false;
    },
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: (_type: string, l: () => void) => mediaListeners.add(l),
    removeEventListener: (_type: string, l: () => void) => mediaListeners.delete(l),
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia;
}

export function setDesktop(on: boolean): void {
  desktop = on;
  mediaListeners.forEach((l) => l());
}

/** The progress stream of the page under test. */
export class FakeEventSource {
  static last: FakeEventSource | null = null;
  onmessage: ((ev: { data: string }) => void) | null = null;
  onerror: ((ev: unknown) => void) | null = null;
  constructor() {
    FakeEventSource.last = this;
  }
  close() {}
  /** Delivers one progress frame as the server would. */
  send(frame: Record<string, unknown>): void {
    this.onmessage?.({ data: JSON.stringify(frame) });
  }
}
