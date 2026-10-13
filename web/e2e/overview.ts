// What the overview shows of a server that has been running for a while: the
// entries of every kind, their runs, the state of each section and the open
// findings. A fresh harness database has none of it, so it is staged at the
// route layer. The storage locations come from storage.ts.
import type { Page } from "@playwright/test";

import { LOCATIONS, STORAGE_BOX } from "./storage";
import { blockServerLook } from "./serverLook";

export const NOW = Math.floor(Date.now() / 1000);
const HOUR = 3600;
const DAY = 86400;
const MB = 1024 ** 2;
const GB = 1024 ** 3;

export const LONG_NAME = "Nextcloud_AIO_Datenbank_Hauptserver_Produktivsystem_Zweitinstanz";
export const FAILED_NAME = "paperless-ngx";
export const UNPROTECTED_NAME = "gitea";
export const FAIL_REASON = "restic backup abgebrochen: Fatal: unable to save snapshot: context deadline exceeded";
export const DRILL_DETAIL =
  "restic restore in die Sandbox abgebrochen: Paketdatei data/00/beispiel-paket-nummer-eins nicht lesbar";

function run(id: string, over: Record<string, unknown>) {
  return {
    id,
    targetId: "plex",
    kind: "backup",
    status: "success",
    startedAt: NOW - 8 * HOUR,
    finishedAt: NOW - 8 * HOUR + 95,
    snapshotId: "5f3a9c1e7b2d4a60",
    bytes: 212 * MB,
    error: "",
    acknowledged: false,
    groupId: "",
    target: "plex",
    domain: "container",
    ...over,
  };
}

export const RUNS = [
  run("run-zfs", { targetId: "zfs-1", target: "tank/appdata", domain: "zfs", startedAt: NOW - 6 * HOUR, finishedAt: NOW - 6 * HOUR + 40 }),
  run("run-paperless", {
    targetId: FAILED_NAME,
    target: FAILED_NAME,
    status: "failed",
    startedAt: NOW - 7 * HOUR,
    finishedAt: NOW - 7 * HOUR + 62,
    snapshotId: "",
    bytes: 0,
    error: FAIL_REASON,
  }),
  run("run-plex", {}),
  run("run-long", { targetId: LONG_NAME, target: LONG_NAME, startedAt: NOW - 9 * HOUR, finishedAt: NOW - 9 * HOUR + 310 }),
  run("run-win", { targetId: "vm-1", target: "Windows 11", domain: "vm", startedAt: NOW - DAY - 4 * HOUR, finishedAt: NOW - DAY - 3 * HOUR }),
  run("run-fotos", { targetId: "set-1", target: "Fotos", domain: "files", startedAt: NOW - DAY - 6 * HOUR, finishedAt: NOW - DAY - 6 * HOUR + 900 }),
  run("run-flash", { targetId: "flash", target: "Unraid flash", domain: "flash", startedAt: NOW - 2 * DAY, finishedAt: NOW - 2 * DAY + 20 }),
  run("run-plex-2", { startedAt: NOW - DAY - 8 * HOUR, finishedAt: NOW - DAY - 8 * HOUR + 90 }),
];

function item(over: Record<string, unknown>) {
  return {
    kind: "container",
    key: "plex",
    name: "plex",
    included: true,
    paused: false,
    effectiveSchedule: { kind: "domain", spec: "daily 03:00" },
    lastBackup: NOW - 8 * HOUR,
    lastRunStatus: "success",
    sourceBytes: 4 * GB,
    state: "running",
    installed: true,
    runs: [],
    ...over,
  };
}

const listed = (id: string, status = "success") => [{ id, kind: "backup", status, startedAt: NOW - 8 * HOUR }];

export const ITEMS = [
  item({ runs: listed("run-plex") }),
  item({ key: LONG_NAME, name: LONG_NAME, runs: listed("run-long") }),
  item({
    key: FAILED_NAME,
    name: FAILED_NAME,
    lastBackup: NOW - DAY - 7 * HOUR,
    lastRunStatus: "failed",
    runs: listed("run-paperless", "failed"),
  }),
  item({
    key: UNPROTECTED_NAME,
    name: UNPROTECTED_NAME,
    included: false,
    effectiveSchedule: { kind: "none", spec: "", reason: "excluded" },
    lastBackup: 0,
    lastRunStatus: "",
    sourceBytes: null,
  }),
  item({ key: "bombvault", name: "bombvault", self: true, included: false, lastBackup: 0, lastRunStatus: "", sourceBytes: null }),
  item({ key: "old-wiki", name: "old-wiki", installed: false, state: "not-installed", lastBackup: NOW - 40 * DAY }),
  item({ kind: "vm", key: "Windows 11", name: "Windows 11", state: "running", lastBackup: NOW - DAY - 3 * HOUR, runs: listed("run-win") }),
  item({ kind: "flash", key: "flash", name: "flash", state: undefined, installed: undefined, lastBackup: NOW - 2 * DAY, runs: listed("run-flash") }),
  item({ kind: "files", key: "set-1", name: "Fotos", state: undefined, installed: undefined, lastBackup: NOW - DAY - 6 * HOUR, runs: listed("run-fotos") }),
  item({ kind: "zfs", key: "zfs-1", name: "tank/appdata", state: undefined, installed: undefined, lastBackup: NOW - 6 * HOUR, runs: listed("run-zfs") }),
];

function domain(over: Record<string, unknown>) {
  return {
    domain: "containers",
    enabled: true,
    schedule: "daily 03:00",
    coveredBy: "",
    lastSuccess: NOW - 8 * HOUR,
    periodSeconds: DAY,
    status: "ok",
    lastVerified: NOW - DAY,
    lastVerifiedOK: true,
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
    lastOffsiteSubsetAt: NOW - 2 * DAY,
    lastOffsiteSubsetOK: true,
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

export const DOMAINS = [
  domain({
    offsiteImmutable: false,
    tamperState: "",
    lastTamperAt: 0,
    replicationState: "overdue",
    lastReplicationAt: NOW - 4 * DAY,
    drillState: "failed",
    lastDrDrillOK: false,
    drillDetail: DRILL_DETAIL,
    protection: "amber",
    lastStartTest: { container: "vaultwarden", ok: true, at: NOW - 2 * DAY, detail: "" },
  }),
  domain({
    domain: "vms",
    schedule: "weekly sun 04:00",
    lastSuccess: NOW - DAY - 3 * HOUR,
    lastDrDrillAt: 0,
    lastDrDrillOK: false,
    offsiteDrillScheduled: false,
    drillState: "",
  }),
  domain({
    domain: "flash",
    lastSuccess: NOW - 2 * DAY,
    offsiteConfigured: false,
    offsiteImmutable: false,
    lastTamperAt: 0,
    lastReplicationAt: 0,
    lastDrDrillAt: 0,
    lastOffsiteSubsetAt: 0,
    offsiteDrillScheduled: false,
    protection: "red",
    tamperState: "",
    replicationState: "",
    drillState: "",
  }),
  domain({ domain: "config", enabled: false, status: "off", schedule: "", protection: "" }),
  domain({ domain: "files", lastSuccess: NOW - DAY - 6 * HOUR }),
  domain({ domain: "zfs", schedule: "daily 01:45", lastSuccess: NOW - 6 * HOUR }),
];

const COVERAGE = {
  total: 23,
  protected: 17,
  domains: [
    {
      domain: "containers",
      enabled: true,
      total: 14,
      protected: 10,
      unprotected: [{ name: UNPROTECTED_NAME, reason: "not-included", neverBackedUp: true }],
    },
  ],
};

let seq = 0;
function finding(over: Record<string, unknown>) {
  seq += 1;
  return {
    id: `an-${seq}`,
    detector: "source",
    metric: "source_bytes_shrink",
    severity: "critical",
    state: "open",
    scopeKind: "item",
    scopeId: `tg-${seq}`,
    targetId: `tg-${seq}`,
    domain: "container",
    part: "",
    targetName: "",
    name: `element-${seq}`,
    runId: "run-plex",
    lastRunId: "run-plex",
    lastRunAt: NOW - 8 * HOUR,
    observed: 2 * GB,
    expected: 18 * GB,
    threshold: 9 * GB,
    samples: 14,
    sensitivity: "balanced",
    details: { median: 18 * GB, mad: GB, z: 7.4 },
    occurrences: 1,
    firstSeenAt: NOW - 8 * HOUR,
    lastSeenAt: NOW - 8 * HOUR,
    recoveredAt: 0,
    resolvedAt: 0,
    ackedAt: 0,
    clearedAt: 0,
    ackNote: "",
    notifiedAt: 0,
    expectable: true,
    retentionHeld: false,
    stillPresent: false,
    ...over,
  };
}

export const FINDINGS = [
  finding({ targetId: "tg-immich", name: "immich", retentionHeld: true }),
  finding({
    targetId: "tg-nextcloud",
    name: "nextcloud",
    detector: "new_data",
    metric: "new_data",
    severity: "warning",
    observed: 1.2 * GB,
    expected: 22 * MB,
    threshold: 40 * MB,
    details: { median: 22 * MB, mad: 9 * MB, z: 8.4, refBytes: 40 * MB },
  }),
  finding({ targetId: "tg-plex", name: "plex", detector: "duration", metric: "duration_slower", severity: "info", observed: 240_000, expected: 38_000, threshold: 60_000 }),
];

/** The parent run of a "back up everything" pass, as it stands while it runs. */
export const EVERYTHING_RUN = run("run-everything", {
  targetId: "everything",
  target: "Backup Everything",
  domain: "everything",
  status: "running",
  startedAt: NOW,
  finishedAt: null,
  snapshotId: "",
  bytes: 0,
});

/** The Storage Box with copies of the containers and the flash that have failed since yesterday. */
export const FAILING_BOX = {
  ...STORAGE_BOX,
  sections: STORAGE_BOX.sections.map((section) =>
    section.domain === "config" ? section : { ...section, lastCopy: { at: NOW - 2 * DAY, ok: false, failingSince: NOW - DAY } },
  ),
};

export interface Staged {
  /** Whether the page asked the server to back up everything. */
  fired: () => boolean;
}

export interface OverviewStage {
  items?: unknown[];
  runs?: unknown[];
  findings?: unknown[];
  /** Encryption is on and the recovery kit is not confirmed. */
  kitDue?: boolean;
  lang?: string;
  /** What localStorage holds for the layout before the page loads. */
  layout?: unknown;
}

/**
 * stageOverview answers every route the overview reads with the fixtures. A
 * start of the pass is accepted without reaching the server, and its run joins
 * the list only then, as a run the page did not know before it asked.
 */
export async function stageOverview(page: Page, width: number | undefined, s: OverviewStage = {}): Promise<Staged> {
  const findings = s.findings ?? FINDINGS;
  let fired = false;
  const loud = (severity: string) => findings.filter((f) => (f as { severity: string }).severity === severity).length;

  await page.route("**/api/settings", async (route) => {
    if (route.request().method() !== "GET") return route.continue();
    const res = await route.fetch();
    const body = await res.json();
    await route.fulfill({
      json: {
        ...body,
        settings: { ...body.settings, anomalyEnabled: true, encryptionEnabled: s.kitDue ?? true, recoveryKitAck: false },
      },
    });
  });
  await page.route("**/api/status", (route) => route.fulfill({ json: { ok: true, domains: DOMAINS } }));
  await page.route("**/api/coverage", (route) => route.fulfill({ json: { ok: true, coverage: COVERAGE } }));
  await page.route(
    (url) => url.pathname === "/api/items",
    (route) => route.fulfill({ json: { ok: true, items: s.items ?? ITEMS, unlisted: [] } }),
  );
  await page.route(
    (url) => url.pathname === "/api/runs",
    (route) => route.fulfill({ json: { ok: true, runs: [...(fired ? [EVERYTHING_RUN] : []), ...(s.runs ?? RUNS)] } }),
  );
  await page.route("**/api/backup-everything", (route) => {
    fired = true;
    return route.fulfill({ json: { ok: true, started: true } });
  });
  await page.route("**/api/schedule/next", (route) =>
    route.fulfill({
      json: { ok: true, runs: [{ job: "backup", domain: "containers", next: new Date(Date.now() + 2 * HOUR * 1000).toISOString() }] },
    }),
  );
  await page.route(
    (url) => url.pathname === "/api/storage/locations",
    (route) => route.fulfill({ json: { ok: true, locations: LOCATIONS } }),
  );
  await page.route("**/api/anomalies/summary", (route) =>
    route.fulfill({
      json: {
        ok: true,
        summary: {
          enabled: true,
          ready: true,
          generation: 7,
          open: { critical: loud("critical"), warning: loud("warning"), info: loud("info") },
          recoveredCritical: 0,
          learningItems: 0,
          retentionHeld: 1,
          evalErrors: 0,
          notifyMuted: false,
          backfill: { slots: 0, done: 0, failed: 0, filled: 0, withoutSummary: 0 },
          unmeasuredVolumes: [],
        },
      },
    }),
  );
  await page.route("**/api/anomalies/items", (route) => route.fulfill({ json: { ok: true, items: [] } }));
  await page.route(
    (url) => url.pathname === "/api/anomalies",
    (route) => route.fulfill({ json: { ok: true, anomalies: findings, nextCursor: "" } }),
  );
  // The stored language is the look, and the server's display preferences
  // must not replace it while the page boots.
  await blockServerLook(page);
  await page.addInitScript(
    ([lang, layout]) => {
      window.localStorage.setItem("bv-lang", lang as string);
      if (layout) window.localStorage.setItem("bombvault.dashboardLayout", JSON.stringify(layout));
    },
    [s.lang ?? "de", s.layout ?? null] as const,
  );
  if (width) await page.setViewportSize({ width, height: 900 });
  return { fired: () => fired };
}

/** Opens the overview and waits until its cards have their data. */
export async function openOverview(page: Page): Promise<void> {
  await page.goto("/dashboard");
  await page.getByTestId("protection-row").first().waitFor();
  await page.getByTestId("target-row").first().waitFor();
  await settle(page);
}

/** Waits until the animations that end have ended. */
export async function settle(page: Page): Promise<void> {
  await page.evaluate(() =>
    Promise.all(
      document
        .getAnimations()
        .filter((a) => a.effect?.getTiming().iterations !== Infinity)
        .map((a) => a.finished.catch(() => {})),
    ),
  );
}
