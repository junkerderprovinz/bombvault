// The page of one backup entry. A fresh harness database has no entries, so
// one entry of every kind is staged at the route layer, with its runs, its
// timeline and what the checks and anomaly detection know about it. On the
// desktop: the head, the three tabs in the address, and the cards of each
// tab. On the phones: the same content with nothing past the edge and the
// actions above the bottom bar. German, because its labels run longest.
import { expect, test, type Page } from "@playwright/test";
import { mkdirSync } from "node:fs";
import { join } from "node:path";

const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

// Set to a folder to keep pictures of the page at the widths and themes the
// design is compared at.
const SHOTS = process.env.ENTRY_SHOTS;

// A handler can still wait on the real server when its test has passed;
// what it throws then is dropped.
test.afterEach(async ({ page }) => {
  await page.unrouteAll({ behavior: "ignoreErrors" });
});

const NOW = Math.floor(Date.now() / 1000);
const HOUR = 3600;
const DAY = 86400;
const MB = 1024 ** 2;

const SET_ID = "a1b2c3d4e5f60718293a4b5c6d7e8f90";
const ZFS_ID = "0f1e2d3c4b5a69788796a5b4c3d2e1f0";

const EVERYTHING = { kind: "everything", spec: "daily@03:00", alsoSpec: "" };

const PLACEMENT = {
  segment: "local-offsite",
  repo: "",
  repoLabel: "",
  repoKind: "domain",
  repoOff: false,
  homeFollows: true,
  copiesFollow: true,
  skip: [],
  locked: true,
  lockReason: "first-backup",
  segmentLocks: {},
  paused: false,
  unreadable: false,
};

// Backups today, yesterday, and three, four, eight, nine, twenty and
// thirty-five days ago: six fall into the two weeks of the history.
const RUN_DAYS = [0, 1, 3, 4, 8, 9, 20, 35];

function runsOf(prefix: string, domain: string, target: string) {
  return RUN_DAYS.map((daysAgo, i) => ({
    id: `${prefix}-r${i}`,
    targetId: `tg-${prefix}`,
    kind: "backup",
    status: "success",
    startedAt: NOW - 4 * HOUR - daysAgo * DAY - 44,
    finishedAt: NOW - 4 * HOUR - daysAgo * DAY,
    snapshotId: `${prefix}${i}aa11bb22cc33dd44`.slice(0, 16),
    bytes: (i + 1) * MB,
    sourceBytes: 65 * MB,
    error: "",
    acknowledged: false,
    target,
    domain,
  }));
}

const RUNS: Record<string, ReturnType<typeof runsOf>> = {
  "container:immich": runsOf("immich", "container", "immich"),
  "container:pihole": runsOf("pihole", "container", "pihole").slice(6),
  "container:paperless-db": runsOf("paperless", "container", "paperless-db"),
  "vm:win11": runsOf("win11", "vm", "win11"),
  [`files:${SET_ID}`]: runsOf("fotos", "files", "Fotos"),
  [`zfs:${ZFS_ID}`]: runsOf("tank", "zfs", "tank/appdata"),
  "flash:flash": runsOf("flash", "flash", "Unraid flash"),
  "config:config": runsOf("config", "config", "config"),
};

function item(over: Record<string, unknown>) {
  const key = String(over.key);
  return {
    kind: "container",
    name: key,
    included: true,
    paused: false,
    effectiveSchedule: EVERYTHING,
    lastBackup: NOW - 4 * HOUR,
    lastRunStatus: "success",
    sourceBytes: 65 * MB,
    state: "running",
    installed: true,
    runs: [],
    ...over,
  };
}

const ITEMS = [
  item({ key: "immich" }),
  item({ key: "pihole", installed: false, state: undefined, included: false, lastBackup: NOW - 20 * DAY }),
  item({ key: "paperless-db" }),
  item({ kind: "vm", key: "win11", name: "Windows 11" }),
  item({ kind: "files", key: SET_ID, name: "Fotos", state: undefined, installed: undefined }),
  item({ kind: "zfs", key: ZFS_ID, name: "tank/appdata", state: undefined, installed: undefined }),
  item({ kind: "flash", key: "flash", name: "flash", state: undefined, installed: undefined }),
  item({ kind: "config", key: "config", name: "config", state: undefined, installed: undefined }),
];

function container(over: Record<string, unknown>) {
  return {
    name: "immich",
    image: "ghcr.io/immich-app/immich-server:release",
    state: "running",
    status: "Up 3 days",
    ip: "192.168.20.40",
    installed: true,
    includeInSchedule: true,
    lastBackup: NOW - 4 * HOUR,
    lastBackupStarted: NOW - 4 * HOUR - 44,
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
  container({}),
  container({ name: "pihole", installed: false, state: "", ip: "", includeInSchedule: false, lastBackup: NOW - 20 * DAY }),
  container({ name: "paperless-db", image: "postgres:16", dbEngine: "postgres", dbTier: "curated", dbDataCoverage: "stopped" }),
];

const VMS = [
  {
    name: "Windows 11",
    libvirtName: "win11",
    state: "running",
    method: "graceful",
    includeInSchedule: true,
    lastBackup: NOW - 4 * HOUR,
    lastBackupStarted: NOW - 5 * HOUR,
    placement: PLACEMENT,
  },
];

const FILE_SETS = [
  {
    id: SET_ID,
    name: "Fotos",
    path: "fotos",
    excludes: [],
    enabled: true,
    lastBackup: NOW - 4 * HOUR,
    effectiveSchedule: EVERYTHING,
    pathExists: true,
    placement: PLACEMENT,
  },
];

const DATASETS = [
  {
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
  },
];

const BOX = {
  place: "offsite:t1",
  label: "Storage Box",
  kind: "target",
  remote: true,
  enabled: true,
  appendOnly: false,
  state: "unchecked",
};

function timeline(key: string) {
  return {
    ok: true,
    places: [{ place: "local", label: "", kind: "home", remote: false, enabled: true, appendOnly: false, state: "read" }, BOX],
    rows: (RUNS[key] ?? []).map((run, i) => ({
      key: run.snapshotId,
      time: new Date(run.finishedAt * 1000).toISOString(),
      size: run.sourceBytes,
      // The second backup carries a tag a person set beside the server's own.
      places: [{ place: "local", snapshotIds: [run.snapshotId], tags: i === 1 ? ["container:immich", "vor-update"] : [] }],
    })),
  };
}

function placementOptions(domain: string) {
  return {
    ok: true,
    options: {
      domain,
      unreadable: false,
      paused: false,
      homes: [{ id: "", name: "", location: `/mnt/user/backups/${domain}`, kind: "domain", scheme: "" }],
      targets: [{ id: "t1", name: "Storage Box", enabled: true, primary: false, appendOnly: false, hint: "" }],
      sendTo: [],
      segmentLocks: {},
      default: { domain, exists: true, home: "", homeKind: "domain", homeOff: false, skip: [], paused: false, confirmedAt: NOW, counts: {}, unreadable: false },
    },
  };
}

const WATCHED = [
  {
    targetId: "tg-immich",
    domain: "container",
    name: "immich",
    scheduled: true,
    sensitivity: "",
    effective: "balanced",
    notifyMin: "",
    effectiveNotifyMin: "critical",
    learning: { samples: 10, needed: 10, newData: 10, source: 10, duration: 10, noData: false },
    typical: { sourceBytes: 65 * MB, newDataBytes: 2 * MB, resticMs: 41000 },
    dump: null,
    datasets: [],
    open: { critical: 0, warning: 0, info: 0 },
    retentionHeld: false,
    selectionSince: 0,
    expectations: [],
  },
];

const CHECKS = [
  {
    targetId: "tg-immich",
    domain: "container",
    name: "immich",
    probe: { targetId: "tg-immich", domain: "container", at: NOW - 2 * DAY, ok: true, detail: "", snapshotId: "s", files: 1284, bytes: 1 },
  },
];

const TIMELINE_DOMAIN: Record<string, string> = {
  containers: "container",
  vms: "vm",
  files: "files",
  zfs: "zfs",
  flash: "flash",
  config: "config",
};

const DIFF = {
  addedFiles: 2,
  removedFiles: 1,
  changedFiles: 12,
  addedBytes: 5 * MB,
  removedBytes: MB,
  changes: [
    { path: "/config/new.yaml", modifier: "+" },
    { path: "/logs/old.log", modifier: "-" },
    { path: "/config/configuration.yaml", modifier: "M" },
    { path: "/data/library.db", modifier: "M" },
  ],
};

function exportPreview(form: string) {
  const repository = form === "repository";
  return {
    folder: "/mnt/user/backups/bombvault/export",
    files: [repository ? "immich-restic/" : `immich-20261006-034700.${form === "zip" ? "zip" : "tar.gz"}`],
    replaces: [],
    backups: repository ? 1 : undefined,
    bytes: repository ? undefined : 65 * MB,
    encryption: repository ? "password" : "none",
    key: "export:containers",
  };
}

async function stage(page: Page, theme: "dark" | "light" = "dark"): Promise<void> {
  const json = (path: string, body: unknown) =>
    page.route(
      (url) => url.pathname === path,
      (route) => (route.request().method() === "GET" ? route.fulfill({ json: body }) : route.continue()),
    );

  await json("/api/items", { ok: true, items: ITEMS, unlisted: [] });
  await page.route(
    (url) => /^\/api\/items\/[^/]+\/[^/]+$/.test(url.pathname),
    (route) => {
      if (route.request().method() !== "GET") return route.continue();
      const [kind, key] = new URL(route.request().url()).pathname.split("/").slice(3).map(decodeURIComponent);
      const found = ITEMS.find((i: Record<string, unknown>) => i.kind === kind && i.key === key);
      if (found) return route.fulfill({ json: { ok: true, item: found } });
      return route.fulfill({ status: 404, json: { ok: false, error: "no such item", code: "not-found" } });
    },
  );
  await json("/api/containers", { ok: true, containers: CONTAINERS });
  await json("/api/vms", { ok: true, vms: VMS });
  await json("/api/files", { ok: true, fileSets: FILE_SETS });
  await json("/api/zfs", { ok: true, datasets: DATASETS });
  await json("/api/checks/items", { ok: true, items: CHECKS });
  await json("/api/anomalies/items", { ok: true, items: WATCHED });
  await json("/api/anomalies", { ok: true, anomalies: [], nextCursor: "" });
  await json("/api/anomalies/summary", {
    ok: true,
    summary: {
      enabled: true,
      ready: true,
      generation: 3,
      open: { critical: 0, warning: 0, info: 0 },
      recoveredCritical: 0,
      learningItems: 0,
      retentionHeld: 0,
      evalErrors: 0,
      notifyMuted: false,
      backfill: { slots: 0, done: 0, failed: 0, filled: 0, withoutSummary: 0 },
      unmeasuredVolumes: [],
    },
  });
  await json("/api/breakdown", { ok: true, breakdown: { state: "none", path: "", size: 0, files: 0, added: 0, children: [] } });

  await page.route(
    (url) => url.pathname === "/api/runs",
    (route) => {
      const q = new URL(route.request().url()).searchParams;
      return route.fulfill({ json: { ok: true, runs: RUNS[`${q.get("itemKind")}:${q.get("itemKey")}`] ?? [] } });
    },
  );
  await page.route(
    (url) => /^\/api\/items\/[^/]+\/[^/]+\/timeline$/.test(url.pathname),
    (route) => {
      const url = new URL(route.request().url());
      const [, , , domain, key] = url.pathname.split("/");
      if (url.searchParams.has("place")) {
        return route.fulfill({ json: { ok: true, place: { ...BOX, state: "read" }, rows: [] } });
      }
      return route.fulfill({ json: timeline(`${TIMELINE_DOMAIN[domain]}:${decodeURIComponent(key)}`) });
    },
  );
  await page.route(
    (url) => /^\/api\/items\/[^/]+\/[^/]+\/tag$/.test(url.pathname),
    (route) => route.fulfill({ json: { ok: true } }),
  );
  await page.route(
    (url) => /^\/api\/items\/[^/]+\/[^/]+\/diff$/.test(url.pathname),
    (route) => route.fulfill({ json: { ok: true, diff: DIFF } }),
  );
  await page.route(
    (url) => /^\/api\/items\/[^/]+\/[^/]+\/export(\/preview)?$/.test(url.pathname),
    (route) => {
      const preview = exportPreview(route.request().postDataJSON().form);
      const starts = !route.request().url().endsWith("/preview");
      return route.fulfill({ json: { ok: true, export: preview, ...(starts ? { started: true } : {}) } });
    },
  );
  await page.route(
    (url) => url.pathname === "/api/placement/options",
    (route) => route.fulfill({ json: placementOptions(new URL(route.request().url()).searchParams.get("domain") ?? "") }),
  );
  await page.route(
    (url) => /^\/api\/anomalies\/items\/[^/]+\/series$/.test(url.pathname),
    (route) => route.fulfill({ json: { ok: true, series: [] } }),
  );
  // The harness has no Docker to ask for a container's folders.
  await page.route(
    (url) => /^\/api\/containers\/[^/]+\/mounts$/.test(url.pathname),
    (route) => route.fulfill({ json: { ok: true, mounts: [], custom: [], hostMountRoot: "/host/user" } }),
  );
  await page.route("**/api/settings", async (route) => {
    if (route.request().method() !== "GET") return route.continue();
    const res = await route.fetch();
    const body = await res.json();
    await route.fulfill({
      json: {
        ...body,
        settings: {
          ...body.settings,
          containersEnabled: true,
          vmsEnabled: true,
          flashEnabled: true,
          filesEnabled: true,
          zfsEnabled: true,
          configEnabled: true,
          anomalyEnabled: true,
        },
      },
    });
  });
  // The stored look is the one under test, so the server's must not replace it.
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.addInitScript((mode) => {
    window.localStorage.setItem("bv-lang", "de");
    window.localStorage.setItem("bv-theme", mode);
  }, theme);
}

async function settle(page: Page): Promise<void> {
  await page.evaluate(() =>
    Promise.all(
      document
        .getAnimations()
        .filter((a) => a.effect?.getTiming().iterations !== Infinity)
        .map((a) => a.finished.catch(() => {})),
    ),
  );
}

async function openEntry(page: Page, path: string, name: string): Promise<void> {
  await page.goto(path);
  await expect(page.getByRole("heading", { level: 1, name })).toBeAttached();
  await settle(page);
}

const tab = (page: Page, name: string) => page.getByRole("tablist", { name: "immich" }).getByRole("tab", { name });
// Matched from its start: one card's title can be part of another's, and a
// heading with an info bubble carries the bubble's text after its own.
const card = (page: Page, title: string) =>
  page.getByRole("heading", { level: 2, name: new RegExp(`^${title}`) }).locator("xpath=..");
const corner = (page: Page) => page.getByTestId("page-actions");

test("the head says how the entry is doing and offers its two actions", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop layout");
  await stage(page);
  await openEntry(page, "/backups/container/immich", "immich");

  await expect(page.getByText("Geschützt", { exact: true })).toBeVisible();
  await expect(page.getByText("192.168.20.40")).toBeVisible();
  await expect(page.getByRole("link", { name: "Sicherungen" }).first()).toHaveAttribute("href", "/backups");
  await expect(corner(page).getByRole("button", { name: "Jetzt sichern" })).toBeVisible();
  await expect(corner(page).getByRole("button", { name: "Zeitplan pausieren" })).toBeVisible();
});

test("a tab stays in the address over a reload, and Back leaves the entry", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop layout");
  await stage(page);
  await page.goto("/backups");
  await page.getByTestId("backup-row").filter({ hasText: "immich" }).getByRole("link", { name: "immich" }).click();
  await expect(page).toHaveURL(/\/backups\/container\/immich$/);
  await expect(tab(page, "Sicherungen")).toHaveAttribute("aria-selected", "true");

  await tab(page, "Einstellungen").click();
  await expect(page).toHaveURL(/\/backups\/container\/immich\?tab=settings$/);
  await expect(card(page, "Zeitplan und Speicherort")).toBeVisible();

  await page.reload();
  await expect(tab(page, "Einstellungen")).toHaveAttribute("aria-selected", "true");
  await expect(card(page, "Zeitplan und Speicherort")).toBeVisible();

  await tab(page, "Prüfung").click();
  await expect(page).toHaveURL(/\?tab=verify$/);
  await page.goBack();
  await expect(page).toHaveURL(/\/backups$/);
});

test("the Backups tab shows the history, the storage locations and the list a restore starts from", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop layout");
  await stage(page);
  await openEntry(page, "/backups/container/immich", "immich");

  await expect(card(page, "Sicherungsverlauf").getByTestId("history-bar")).toHaveCount(6);
  const places = card(page, "Speicherorte");
  await expect(places).toContainText("8 Sicherungen");
  await expect(places).toContainText("Noch nicht geprüft");
  await places.getByRole("button", { name: "Prüfen" }).click();
  await expect(places).toContainText("0 Sicherungen");

  const rows = page.getByTestId("entry-backup");
  await expect(rows).toHaveCount(8);
  // At its start, where the row holds no control of its own.
  await rows.first().click({ position: { x: 16, y: 16 } });
  const restore = page.getByRole("dialog", { name: "immich wiederherstellen" });
  await expect(restore).toBeVisible();
  await restore.getByRole("button", { name: "Schließen" }).click();
  await expect(restore).toBeHidden();
});

test("the Settings tab gives a container the cards of its kind", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop layout");
  await stage(page);
  await openEntry(page, "/backups/container/immich?tab=settings", "immich");

  for (const title of ["Zeitplan und Speicherort", "Repository", "Container", "Gesicherte Ordner", "Ausschlussmuster"]) {
    await expect(card(page, title)).toBeVisible();
  }
  await expect(card(page, "Zeitplan und Speicherort")).toContainText("Folgt der Vorgabe");
  await expect(card(page, "Zeitplan und Speicherort").getByRole("switch", { name: "Im Zeitplan" })).toBeChecked();
});

test("the Verify tab shows the checks, the runs and the size, and opens a run in a window", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop layout");
  await stage(page);
  await openEntry(page, "/backups/container/immich?tab=verify", "immich");

  await expect(card(page, "Prüfungen").getByRole("button", { name: "Wiederherstellung prüfen" })).toBeVisible();
  await expect(card(page, "Anomalien")).toBeVisible();
  await expect(card(page, "Größe nach Ordner").getByRole("img")).toBeVisible();

  const runs = card(page, "Letzte Läufe").getByTestId("entry-run");
  await expect(runs).toHaveCount(8);
  await runs.first().click();
  const run = page.getByRole("dialog");
  await expect(run).toContainText("immich0a");
  await run.getByRole("button", { name: "Schließen" }).click();
  await expect(run).toBeHidden();
});

for (const [what, path, name] of [
  ["a VM", "/backups/vm/win11", "Windows 11"],
  ["a folder set", `/backups/files/${SET_ID}`, "Fotos"],
  ["a ZFS entry", `/backups/zfs/${ZFS_ID}`, "tank/appdata"],
  ["the flash drive", "/backups/flash/flash", "Unraid-Flash"],
  ["the self-backup", "/backups/config/config", "BombVault-Einstellungen"],
]) {
  test(`${what} opens on the same page with its backups`, async ({ page }, testInfo) => {
    test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop layout");
    await stage(page);
    await openEntry(page, path, name);
    await expect(card(page, "Sicherungsverlauf").getByTestId("history-bar")).toHaveCount(6);
    await expect(card(page, "Speicherorte")).toBeVisible();
    await expect(corner(page).getByRole("button", { name: "Jetzt sichern" })).toBeVisible();
  });
}

test("the menu of a backup marks it, compares it and exports it", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop layout");
  await stage(page);
  await openEntry(page, "/backups/container/immich", "immich");

  const rows = page.getByTestId("entry-backup");
  await expect(rows.nth(1)).toContainText("vor-update");
  await expect(rows.nth(1)).not.toContainText("container:immich");
  await expect(rows.first().getByTestId("entry-backup-size")).toHaveText(/65/);

  await rows.first().getByRole("button", { name: "Mehr" }).click();
  const menu = page.getByRole("menu");
  await expect(menu.getByRole("menuitem")).toHaveText([
    "Mit anderer Sicherung vergleichen",
    "Tag hinzufügen",
    "Exportieren",
    "Löschen",
  ]);
  await menu.getByRole("menuitem", { name: "Tag hinzufügen" }).click();
  const tag = page.getByRole("dialog", { name: "Tag für diese Sicherung" });
  await tag.getByRole("textbox").fill("vor-umzug");
  const tagged = page.waitForRequest((req) => req.url().endsWith("/api/items/containers/immich/tag"));
  await tag.getByRole("button", { name: "Speichern" }).click();
  expect((await tagged).postDataJSON()).toEqual({ snapshotId: RUNS["container:immich"][0].snapshotId, tags: ["vor-umzug"], remove: [] });
  await expect(tag).toBeHidden();

  await rows.first().getByRole("button", { name: "Mehr" }).click();
  await menu.getByRole("menuitem", { name: "Mit anderer Sicherung vergleichen" }).click();
  const compare = page.getByRole("dialog", { name: "Sicherungen vergleichen" });
  await compare.getByRole("button", { name: "Vergleichen" }).click();
  const result = compare.getByTestId("compare-result");
  await expect(result).toContainText("12 Dateien geändert, 2 neu, 1 gelöscht");
  await expect(result).toContainText("/config/configuration.yaml");
  await compare.getByRole("button", { name: "Schließen" }).click();

  await rows.first().getByRole("button", { name: "Mehr" }).click();
  await menu.getByRole("menuitem", { name: "Exportieren" }).click();
  const exported = page.getByRole("dialog", { name: "immich exportieren" });
  const preview = exported.getByTestId("export-preview");
  await expect(preview).toContainText("/mnt/user/backups/bombvault/export");
  await expect(preview).toContainText("immich-20261006-034700.tar.gz");
  await exported.getByRole("tab", { name: "restic" }).click();
  await expect(preview).toContainText("immich-restic/");
  await expect(preview).toContainText("abgeleitetes restic-Passwort");
  await expect(preview.getByRole("link", { name: "Wiederherstellung" })).toHaveAttribute("href", "/settings/restore");
  await exported.getByRole("button", { name: "Exportieren" }).click();
  await expect(exported.getByRole("button", { name: "Export abbrechen" })).toBeVisible();
});

test("the menu beside the name takes a folder set off the list in one of two ways", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop layout");
  await stage(page);
  await openEntry(page, `/backups/files/${SET_ID}`, "Fotos");

  await page.getByRole("button", { name: "Mehr" }).first().click();
  await expect(page.getByRole("menu").getByRole("menuitem")).toHaveText(["Neueste Sicherung exportieren", "Aus der Liste entfernen"]);
  await page.getByRole("menuitem", { name: "Aus der Liste entfernen" }).click();
  const remove = page.getByRole("dialog", { name: "Fotos aus der Liste nehmen?" });
  await expect(remove.getByRole("radio")).toHaveCount(2);
  await expect(remove.getByRole("button", { name: "Aus der Liste nehmen" })).toBeVisible();
  await remove.getByRole("radio", { name: /Auch alle Sicherungen löschen/ }).click();
  await expect(remove.getByRole("button", { name: "Sicherungen löschen" })).toBeVisible();
  await remove.getByRole("button", { name: "Abbrechen" }).click();
  await expect(remove).toBeHidden();
});

test("a ZFS entry lists its runs on the Backups tab like every other kind", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop layout");
  await stage(page);
  await openEntry(page, `/backups/zfs/${ZFS_ID}`, "tank/appdata");
  await expect(page.getByTestId("entry-backup")).toHaveCount(8);
  await expect(card(page, "Speicherorte")).toContainText("8 Sicherungen");
});

test("the Database card draws how a backup of a database container runs", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop layout");
  await stage(page);
  await openEntry(page, "/backups/container/paperless-db?tab=settings", "paperless-db");
  const picture = card(page, "Datenbank").getByRole("img", { name: /zwei Wiederherstellungspunkte/ });
  await expect(picture).toBeVisible();
  await expect(card(page, "Datenbank")).toContainText("Ablauf einer Sicherung");
});

test("an entry that is gone offers its newest backup instead of a new one", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop layout");
  await stage(page);
  await openEntry(page, "/backups/container/pihole", "pihole");

  await expect(page.getByText("Nicht installiert", { exact: true })).toBeVisible();
  await expect(corner(page).getByRole("button", { name: "Jetzt sichern" })).toHaveCount(0);
  await corner(page).getByRole("button", { name: "Wiederherstellen" }).click();
  await expect(page.getByRole("dialog", { name: "pihole wiederherstellen" })).toBeVisible();
});

test("an entry the server does not know says so", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop layout");
  await stage(page);
  await page.goto("/backups/container/vanished");
  await expect(page.getByText("Diesen Eintrag gibt es nicht mehr.")).toBeVisible();
});

for (const width of [320, 390]) {
  test(`the entry @ ${width}px: every tab, nothing past the edge, the actions above the bar`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "the phone layout lives below 48rem");
    await stage(page);
    await page.setViewportSize({ width, height: 800 });

    for (const [query, title] of [
      ["", "Sicherungsverlauf"],
      ["?tab=settings", "Zeitplan und Speicherort"],
      ["?tab=verify", "Letzte Läufe"],
    ]) {
      await openEntry(page, `/backups/container/immich${query}`, "immich");
      await expect(card(page, title)).toBeVisible();
      const pan = await page.evaluate(() => {
        const main = document.querySelector("#bv-main")!;
        return Math.max(document.documentElement.scrollWidth - window.innerWidth, main.scrollWidth - main.clientWidth);
      });
      expect(pan, `the ${title} tab scrolls sideways`).toBeLessThanOrEqual(1);
    }

    await openEntry(page, "/backups/container/immich", "immich");
    await expect(tab(page, "Einstellungen")).toBeVisible();
    const backup = (await corner(page).getByRole("button", { name: "Jetzt sichern" }).boundingBox())!;
    const nav = (await page.getByRole("navigation", { name: "Mobile Navigation" }).boundingBox())!;
    expect(backup.y + backup.height).toBeLessThanOrEqual(nav.y);

    // The menu of a backup opens inside the window, and its tag window as a sheet.
    const first = page.getByTestId("entry-backup").first();
    await first.getByRole("button", { name: "Mehr" }).click();
    const menu = (await page.getByRole("menu").boundingBox())!;
    expect(menu.x).toBeGreaterThanOrEqual(0);
    expect(menu.x + menu.width).toBeLessThanOrEqual(width);
    await page.getByRole("menuitem", { name: "Tag hinzufügen" }).click();
    const sheet = page.getByRole("dialog", { name: "Tag für diese Sicherung" });
    await expect(sheet.getByRole("textbox")).toBeVisible();
    await sheet.getByRole("button", { name: "Abbrechen" }).click();
    await expect(sheet).toBeHidden();

    // A phone opens a backup with a tap on its row.
    await first.click({ position: { x: 16, y: 16 } });
    await expect(page.getByRole("dialog", { name: "immich wiederherstellen" })).toBeVisible();
  });
}

test("pictures of the page", async ({ page }, testInfo) => {
  test.skip(!SHOTS || testInfo.project.name !== "desktop-1280", "only when a folder for them is named");
  test.setTimeout(180_000);
  mkdirSync(SHOTS!, { recursive: true });
  for (const [name, width, theme] of [
    ["1440-dark", 1440, "dark"],
    ["1440-light", 1440, "light"],
    ["390", 390, "dark"],
  ] as const) {
    await page.unrouteAll({ behavior: "ignoreErrors" });
    await stage(page, theme);
    await page.setViewportSize({ width, height: width < 500 ? 3600 : 2600 });
    for (const [tabName, query] of [
      ["backups", ""],
      ["settings", "?tab=settings"],
      ["verify", "?tab=verify"],
    ]) {
      await openEntry(page, `/backups/container/immich${query}`, "immich");
      await page.screenshot({ path: join(SHOTS!, `immich-${tabName}-${name}.png`) });
    }

    // The windows are taken at the height of a screen, where they stand.
    await page.setViewportSize({ width, height: width < 500 ? 844 : 1000 });
    const shot = async (what: string) => {
      // A sheet starts to slide in a frame after it mounts.
      await page.waitForTimeout(300);
      await settle(page);
      await page.screenshot({ path: join(SHOTS!, `${what}-${name}.png`) });
    };
    const fromRow = async (action: string) => {
      await page.getByTestId("entry-backup").nth(1).getByRole("button", { name: "Mehr" }).click();
      await page.getByRole("menuitem", { name: action }).click();
    };
    await openEntry(page, "/backups/container/immich", "immich");
    await page.getByTestId("entry-backup").nth(1).scrollIntoViewIfNeeded();
    await page.getByTestId("entry-backup").nth(1).getByRole("button", { name: "Mehr" }).click();
    await shot("rowmenu");
    await page.getByRole("menuitem", { name: "Tag ändern" }).click();
    await shot("tag");
    await page.getByRole("dialog").getByRole("button", { name: "Abbrechen" }).click();
    await fromRow("Mit anderer Sicherung vergleichen");
    await page.getByRole("dialog").getByRole("button", { name: "Vergleichen" }).click();
    await expect(page.getByTestId("compare-result")).toBeVisible();
    await shot("compare");
    await page.getByRole("dialog").getByRole("button", { name: "Schließen" }).click();
    await fromRow("Exportieren");
    await expect(page.getByTestId("export-preview")).toBeVisible();
    await shot("export");
    await page.getByRole("dialog").getByRole("tab", { name: "restic" }).click();
    await expect(page.getByTestId("export-preview")).toContainText("immich-restic/");
    await shot("export-restic");
    await page.getByRole("dialog").getByRole("button", { name: "Abbrechen" }).click();
    await page.getByRole("button", { name: "Mehr" }).first().click();
    await page.getByRole("menuitem", { name: "Container exportieren" }).click();
    await shot("export-live");

    await openEntry(page, `/backups/files/${SET_ID}`, "Fotos");
    await shot("entry-folder");
    await page.getByRole("button", { name: "Mehr" }).first().click();
    await page.getByRole("menuitem", { name: "Aus der Liste entfernen" }).click();
    await shot("remove-folder");

    await openEntry(page, `/backups/zfs/${ZFS_ID}`, "tank/appdata");
    await shot("entry-zfs");
    await openEntry(page, "/backups/vm/win11", "Windows 11");
    await shot("entry-vm");
    await openEntry(page, "/backups/container/paperless-db?tab=settings", "paperless-db");
    await card(page, "Datenbank").scrollIntoViewIfNeeded();
    await shot("dbcard");
    await card(page, "Datenbank").screenshot({ path: join(SHOTS!, `dbgrid-${name}.png`) });
  }
});
