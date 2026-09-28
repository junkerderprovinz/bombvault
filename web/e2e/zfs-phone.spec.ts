// The ZFS page at phone width, with items, the host listing, restore points
// and a running backup staged at the route layer (the harness has no pools).
// On the phones: the 24px card rhythm, nothing past the edge and no text cut
// off with every section of every item open, the add dialog and the delete
// sheet inside the viewport, each item's actions on their own row below its
// name, and disclosures large enough to tap. On the desktop: the 40px rhythm,
// a one-line header, the actions beside the name and the one-line rows the
// phone lets wrap. German, because its labels run longest.
import { expect, test, type Page } from "@playwright/test";

const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

// A settings read the page makes as the test ends would otherwise fail it
// from inside a route handler whose response is already gone.
test.afterEach(async ({ page }) => {
  await page.unrouteAll({ behavior: "ignoreErrors" });
});

const NOW = Math.floor(Date.now() / 1000);
const DAY = 24 * 60 * 60;

const member = (dataset: string, relPath: string, outcome: string, isNew = false) => ({
  dataset,
  relPath,
  hostMountpoint: `/mnt/${dataset}`,
  outcome,
  isNew,
  usedByDataset: 48 * 1024 * 1024 * 1024,
  lastBackupAt: NOW - 3600,
});

const item = (id: string, dataset: string, overrides: Record<string, unknown>) => ({
  id: id.padEnd(32, "0"),
  dataset,
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
  hostMountpoint: `/mnt/${dataset}`,
  lastBackup: NOW - DAY,
  lastRunStatus: "success",
  lastCheckCode: "ok",
  lastCheckDetail: "",
  lastCheckAt: NOW - 600,
  leftoverCount: 0,
  safetyCount: 0,
  safetyOldestAt: 0,
  members: [member(dataset, "", "backed-up")],
  effectiveSchedule: { kind: "domain", spec: "daily 03:30", alsoSpec: "" },
  ...overrides,
});

const NEXTCLOUD = "cache/appdata/nextcloud-aio-mastercontainer/benutzerdaten-familie";
const WINDOWS = "tank/vms/domains/windows-11-enterprise-ltsc-arbeitsplatz";
const MEDIA = "pool/media";

const DATASETS = [
  item("a1", NEXTCLOUD, {
    excludes: ["**/cache/**", "*.tmp", "appdata_ocw3kx9y2z/preview/**"],
    stopContainers: ["nextcloud-aio-mastercontainer", "nextcloud-aio-database"],
    excludedChildren: [`${NEXTCLOUD}/alte-kalender-und-kontakte-sicherung`],
    restartPending: ["nextcloud-aio-database"],
    lastBackup: NOW - 3600,
    lastCheckCode: "leftover-snapshots",
    leftoverCount: 3,
    safetyCount: 2,
    safetyOldestAt: NOW - 40 * DAY,
    members: [
      member(NEXTCLOUD, "", "backed-up"),
      member(`${NEXTCLOUD}/nutzerdateien`, "nutzerdateien", "backed-up"),
      member(`${NEXTCLOUD}/nutzerdateien/fotos-und-videos-2019-bis-2026`, "nutzerdateien/fotos-und-videos-2019-bis-2026", "backed-up", true),
      member(`${NEXTCLOUD}/nutzerdateien/verschluesselte-steuerunterlagen`, "nutzerdateien/verschluesselte-steuerunterlagen", "key-not-loaded"),
      member(`${NEXTCLOUD}/datenbank-sicherungen`, "datenbank-sicherungen", "not-mounted"),
    ],
  }),
  item("b2", WINDOWS, {
    enabled: false,
    lastBackup: 0,
    lastRunStatus: "failed",
    lastCheckCode: "not-found",
  }),
  item("c3", MEDIA, {
    members: [member(MEDIA, "", "backed-up"), member(`${MEDIA}/filme`, "filme", "backed-up")],
  }),
];

const series = (part: string, warning: number) => ({
  part,
  learning: { samples: 12, needed: 5 },
  typical: { sourceBytes: 1e11, resticMs: 60000 },
  open: { critical: 0, warning, info: 0 },
  retentionHeld: warning > 0,
});

const anomalyItem = (targetId: string, name: string, warning: number, samples: number, datasets: unknown[]) => ({
  targetId,
  domain: "zfs",
  name,
  scheduled: true,
  sensitivity: "",
  effective: "normal",
  notifyMin: "",
  effectiveNotifyMin: "warning",
  learning: { samples, needed: 5, newData: samples, source: samples, duration: samples, noData: false },
  typical: { sourceBytes: null, newDataBytes: null, resticMs: null },
  dump: null,
  datasets,
  open: { critical: 0, warning, info: 0 },
  retentionHeld: false,
  selectionSince: 0,
  expectations: [],
});

const ANOMALY_ITEMS = [
  anomalyItem(DATASETS[0].id, NEXTCLOUD, 1, 12, [series(`${NEXTCLOUD}/nutzerdateien/fotos-und-videos-2019-bis-2026`, 2)]),
  anomalyItem(DATASETS[2].id, MEDIA, 0, 2, []),
];

const RUNS = [
  {
    id: "run-a1-ok",
    targetId: DATASETS[0].id,
    kind: "backup",
    status: "success",
    startedAt: NOW - 3700,
    finishedAt: NOW - 3600,
    snapshotId: "",
    bytes: 0,
    error: "",
    acknowledged: false,
    target: NEXTCLOUD,
    domain: "zfs",
  },
  {
    id: "run-a1-failed",
    targetId: DATASETS[0].id,
    kind: "backup",
    status: "failed",
    startedAt: NOW - DAY - 900,
    finishedAt: NOW - DAY,
    snapshotId: "",
    bytes: 0,
    error: "consistency-stop-failed: nextcloud-aio-database did not stop within 120s",
    acknowledged: true,
    target: NEXTCLOUD,
    domain: "zfs",
  },
];

const RESTORE_POINTS = [
  {
    stamp: "bombvault-20260927-140500",
    time: NOW - 3600,
    members: DATASETS[0].members.map((m, i) => ({
      dataset: m.dataset,
      relPath: m.relPath,
      snapshotId: i === 3 ? "" : `5f2c9a0${i}e81b4d7c3`,
      outcome: m.outcome,
    })),
  },
];

const host = (dataset: string, overrides: Record<string, unknown> = {}) => ({
  dataset,
  type: "filesystem",
  hostMountpoint: `/mnt/${dataset}`,
  referenced: 312 * 1024 * 1024 * 1024,
  used: 640 * 1024 * 1024 * 1024,
  usedByDataset: 1024 * 1024,
  mounted: true,
  encrypted: false,
  keyLoaded: true,
  visible: true,
  writable: true,
  memberCode: "",
  managedId: "",
  coveredBy: "",
  vmDisk: false,
  system: false,
  vmVolume: false,
  blockers: [],
  ...overrides,
});

const HOST = {
  ok: true,
  available: true,
  code: "ok",
  target: "root@tower",
  datasets: [
    host("cache"),
    host("cache/appdata"),
    host("cache/appdata/nextcloud-aio-mastercontainer"),
    host(NEXTCLOUD, { managedId: DATASETS[0].id }),
    host(`${NEXTCLOUD}/nutzerdateien`, { coveredBy: DATASETS[0].id }),
    host("cache/appdata/home-assistant-konfiguration-und-datenbank", { encrypted: true }),
    host("cache/system"),
    host("cache/system/docker-image-schichten-und-volumes", { system: true }),
    host("tank"),
    host("tank/vms"),
    host("tank/vms/domains"),
    host(WINDOWS, { managedId: DATASETS[1].id, vmDisk: true }),
    host("tank/vms/domains/ubuntu-server-24-04-homelab-dienste", { vmDisk: true }),
    host("tank/vms/debian-12-zvol-fuer-testumgebung", { type: "volume", hostMountpoint: "-", vmVolume: true }),
  ],
  hiddenLegacy: 0,
  unusedZvols: 1,
  notInItem: 6,
  truncated: false,
  maxNameLength: 200,
};

const CONNECTION_OK = {
  ok: true,
  code: "ok",
  target: "root@tower.fritz.box",
  uriTarget: "",
  version: "zfs-2.2.7-1",
  detail: "",
  zfsBinary: "/usr/sbin/zfs",
  propagation: "rslave",
  unpropagated: [],
};

const CONNECTION_AUTH = {
  ...CONNECTION_OK,
  code: "ssh-auth",
  version: "",
  detail: "root@tower.fritz.box: Permission denied (publickey,password,keyboard-interactive).",
};

const PUBLIC_KEY =
  "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIM3q8f0b2sUq7Yk6hJx1r9LwT4cVn2pZ5eG8dK0aB1cD bombvault@tower";

interface Stage {
  datasets?: unknown[];
  connection?: unknown;
  runningBackup?: boolean;
}

async function stage(page: Page, opts: Stage = {}): Promise<void> {
  const datasets = opts.datasets ?? DATASETS;
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.route("**/api/settings", async (route) => {
    const res = await route.fetch();
    const body = await res.json();
    body.settings.zfsEnabled = true;
    await route.fulfill({ response: res, json: body });
  });
  await page.route("**/api/zfs", (route) => route.fulfill({ json: { ok: true, datasets } }));
  await page.route("**/api/zfs/host", (route) => route.fulfill({ json: HOST }));
  await page.route("**/api/zfs/connection", (route) => route.fulfill({ json: opts.connection ?? CONNECTION_OK }));
  await page.route("**/api/vm/ssh", (route) => route.fulfill({ json: { ok: true, host: "tower", publicKey: PUBLIC_KEY } }));
  await page.route("**/api/zfs/datasets/*/restore-points*", (route) =>
    route.fulfill({ json: { ok: true, points: RESTORE_POINTS } }),
  );
  await page.route("**/api/zfs/datasets/*/safety-snapshots", (route) =>
    route.fulfill({
      json: {
        ok: true,
        snapshots: [
          { dataset: NEXTCLOUD, name: "bombvault-safety-20260815-101500", createdAt: NOW - 43 * DAY, usedBytes: 2.4e9 },
          { dataset: `${NEXTCLOUD}/nutzerdateien`, name: "bombvault-safety-20260921-181200", createdAt: NOW - 6 * DAY, usedBytes: 7.1e8 },
        ],
      },
    }),
  );
  await page.route("**/api/zfs/runs/*/members", (route) =>
    route.fulfill({
      json: {
        ok: true,
        windowSeconds: 42,
        members: DATASETS[0].members.map((m) => ({
          dataset: m.dataset,
          outcome: m.outcome,
          resticSnapshot: "",
          isNew: m.isNew,
          bytesAdded: 3.2e8,
          filesNew: 12,
          filesChanged: 3,
          filesUnmodified: 40000,
          durationMs: 61000,
        })),
      },
    }),
  );
  await page.route("**/api/runs", (route) => route.fulfill({ json: { ok: true, runs: RUNS } }));
  await page.route("**/api/anomalies/items", (route) => route.fulfill({ json: { ok: true, items: ANOMALY_ITEMS } }));
  if (opts.runningBackup) {
    await page.route("**/api/progress", (route) =>
      route.fulfill({
        headers: { "content-type": "text/event-stream", "cache-control": "no-cache" },
        body: `retry: 60000\ndata: ${JSON.stringify({ key: `zfs:${MEDIA}`, phase: "backup", percent: 37.5, active: true, startedAt: NOW - 90 })}\n\n`,
      }),
    );
  }
}

async function bootGerman(page: Page, width: number, opts: Stage = {}): Promise<void> {
  await stage(page, opts);
  await page.addInitScript(() => {
    window.localStorage.setItem("bv-lang", "de");
    // Advanced mode puts every control of the page on screen.
    window.localStorage.setItem("bombvault.advanced", "1");
  });
  await page.setViewportSize({ width, height: 800 });
  await page.goto("/zfs");
  await expect(page.getByRole("heading", { level: 1, name: "ZFS-Datasets" })).toBeVisible();
  await settle(page);
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

async function cardGap(page: Page): Promise<string> {
  return page
    .getByRole("heading", { level: 1 })
    .locator("xpath=../../..")
    .evaluate((root) => getComputedStyle(root).rowGap);
}

/** The pan of the document and of #bv-main, and inside `scope` every control
 *  past the viewport's edge, every piece of content past it (a card's
 *  overflow-hidden would otherwise swallow it silently), and every text cut
 *  off with an ellipsis. The scroller is read at its start: a focused control
 *  past the edge scrolls it sideways. */
async function layout(page: Page, scope: string) {
  return page.evaluate((selector) => {
    const main = document.querySelector("#bv-main");
    main?.scrollTo({ left: 0 });
    const vw = window.innerWidth;
    const outside = (el: Element) => {
      const r = el.getBoundingClientRect();
      return r.width > 0 && (r.right > vw + 1 || r.left < -1);
    };
    const name = (el: Element) => (el.getAttribute("aria-label") ?? el.textContent ?? "").trim().slice(0, 40);
    const roots = [...document.querySelectorAll(selector)];
    const all = roots.flatMap((root) => [...root.querySelectorAll("*")]);
    return {
      docPan: document.documentElement.scrollWidth - vw,
      mainPan: main ? main.scrollWidth - main.clientWidth : null,
      clipped: all
        .filter((el) => el.matches("button, a, input, select, textarea, [role='switch'], [role='treeitem']") && outside(el))
        .map(name),
      spilled: all
        .filter((el) => el instanceof HTMLElement && outside(el) && ![...el.children].some(outside))
        .map(name),
      // A select shows its value cut short; the open list has it whole.
      cut: all
        .filter(
          (el) =>
            el instanceof HTMLElement &&
            !el.closest("[role='combobox']") &&
            getComputedStyle(el).textOverflow === "ellipsis" &&
            el.scrollWidth > el.clientWidth + 1,
        )
        .map(name),
    };
  }, scope);
}

async function expectFits(page: Page, scope = "#bv-main"): Promise<void> {
  const box = await layout(page, scope);
  expect(box.mainPan, "#bv-main is missing").not.toBeNull();
  expect(box.docPan, "the document scrolls horizontally").toBeLessThanOrEqual(1);
  expect(box.mainPan!, "#bv-main scrolls horizontally").toBeLessThanOrEqual(1);
  expect(box.clipped, "controls clip the viewport edge").toEqual([]);
  expect(box.spilled, "content runs past the viewport edge").toEqual([]);
  expect(box.cut, "text is cut off").toEqual([]);
}

function card(page: Page, dataset: string) {
  return page.locator("#bv-main div.rounded-card.glim-hue").filter({ has: page.getByText(dataset, { exact: true }) }).first();
}

for (const width of [320, 360]) {
  test(`zfs @ ${width}px: every section open, nothing pans, clips or renders stray markup`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
    await bootGerman(page, width, { runningBackup: true });
    await expect(page.getByRole("progressbar")).toBeVisible();

    expect(await cardGap(page)).toBe("24px");

    const nextcloud = card(page, NEXTCLOUD);
    await nextcloud.getByRole("button", { name: "Bearbeiten" }).click();
    await nextcloud.getByRole("button", { name: "5 Datasets" }).click();
    await nextcloud.getByRole("button", { name: "2 Sicherheits-Snapshots" }).click();
    await nextcloud.getByRole("button", { name: "Backups", exact: true }).click();
    await expect(nextcloud.getByText("Backup vom", { exact: true })).toBeVisible();
    await nextcloud.getByRole("button", { expanded: false }).filter({ hasText: "→" }).last().click();
    await card(page, WINDOWS).getByRole("button", { name: "Backups", exact: true }).click();
    await settle(page);

    await expectFits(page);
    // A backtick in JSX text compiles fine and renders on the page.
    await expect(page.locator("#bv-main")).not.toContainText("`");

    // The actions take their own row under the name, so a long name keeps
    // the width it needs.
    for (const { dataset } of DATASETS) {
      const row = card(page, dataset);
      const nameBox = (await row.getByText(dataset, { exact: true }).first().boundingBox())!;
      const editBox = (await row.getByRole("button", { name: "Bearbeiten" }).boundingBox())!;
      expect(editBox.y, `the actions of "${dataset}" share the name's row`).toBeGreaterThanOrEqual(nameBox.y + nameBox.height);
    }
  });

  test(`zfs @ ${width}px: the add dialog and the delete sheet stay inside the screen`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
    await bootGerman(page, width);

    await page.getByRole("button", { name: "Datasets hinzufügen" }).first().click();
    const dialog = page.getByRole("dialog", { name: "ZFS-Datasets hinzufügen" });
    await expect(dialog.getByRole("tree")).toBeVisible();
    const closed = dialog.getByRole("treeitem", { expanded: false });
    while ((await closed.count()) > 0) await closed.first().click();
    await settle(page);
    await expectFits(page, "[role='dialog']");
    // A deep dataset's name gets the row's width, its size moves under it.
    const deep = dialog.getByRole("treeitem").filter({ hasText: "home-assistant-konfiguration-und-datenbank" });
    const nameBox = (await deep.locator("span.font-mono").first().boundingBox())!;
    const sizeBox = (await deep.getByText(/zu sichern/).boundingBox())!;
    expect(sizeBox.y, "the size shares the name's line").toBeGreaterThanOrEqual(nameBox.y + nameBox.height - 1);
    await dialog.getByRole("button", { name: "Abbrechen" }).click();

    await card(page, NEXTCLOUD).getByRole("button", { name: "Löschen" }).first().click();
    const sheet = page.getByRole("dialog").filter({ hasText: "Dieses Element aus der Liste nehmen?" });
    await expect(sheet.getByRole("switch", { name: "Auch seine Sicherheits-Snapshots auf dem Server löschen" })).toBeVisible();
    await settle(page);
    await expectFits(page, "[role='dialog']");
  });
}

test("zfs on a phone: the empty page and a failed connection fit at 320px", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
  await bootGerman(page, 320, { datasets: [], connection: CONNECTION_AUTH });
  await expect(page.getByText("Noch keine ZFS-Datasets")).toBeVisible();
  await expect(page.locator("#bv-main input[readonly]")).toHaveValue(PUBLIC_KEY);
  await page.getByRole("button", { name: "Details" }).click();
  await settle(page);

  expect(await cardGap(page)).toBe("24px");
  await expectFits(page);
});

test("zfs on a phone: the disclosures are big enough to tap", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the larger targets follow the coarse pointer");
  await bootGerman(page, 360, { connection: CONNECTION_AUTH });
  const nextcloud = card(page, NEXTCLOUD);
  const targets = [
    nextcloud.getByRole("button", { name: "5 Datasets" }),
    nextcloud.getByRole("button", { name: "2 Sicherheits-Snapshots" }),
    nextcloud.getByRole("button", { expanded: false }).filter({ hasText: "→" }).first(),
    page.getByRole("button", { name: "Details" }),
  ];
  for (const target of targets) {
    const box = (await target.boundingBox())!;
    expect(box.height, `${target} is shorter than a fingertip`).toBeGreaterThanOrEqual(43.5);
  }
});

test("zfs on the desktop keeps the 40px rhythm, a one-line header and the actions beside the name", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "desktop-only");
  await bootGerman(page, testInfo.project.use.viewport!.width);

  expect(await cardGap(page)).toBe("40px");
  const tops = await page
    .getByRole("heading", { level: 1 })
    .locator("xpath=../following-sibling::div[1]")
    .locator("button")
    .evaluateAll((buttons) => [...new Set(buttons.map((b) => Math.round(b.getBoundingClientRect().top)))]);
  expect(tops, "the header actions wrapped").toHaveLength(1);

  const row = card(page, MEDIA);
  const nameBox = (await row.getByText(MEDIA, { exact: true }).boundingBox())!;
  const editBox = (await row.getByRole("button", { name: "Bearbeiten" }).boundingBox())!;
  expect(editBox.y, "the actions left the name's row").toBeLessThan(nameBox.y + nameBox.height);
  const toggle = (await row.getByRole("button", { name: "2 Datasets" }).boundingBox())!;
  expect(Math.round(toggle.height), "the disclosure grew on the desktop").toBeLessThan(24);
  const run = (await card(page, NEXTCLOUD).getByRole("button", { expanded: false }).filter({ hasText: "→" }).first().boundingBox())!;
  expect(Math.round(run.height), "a run's times broke onto two lines").toBeLessThan(24);

  await page.getByRole("button", { name: "Datasets hinzufügen" }).first().click();
  const dialog = page.getByRole("dialog", { name: "ZFS-Datasets hinzufügen" });
  const deep = dialog.getByRole("treeitem").filter({ hasText: "cache/appdata" }).first();
  await deep.click();
  const home = dialog.getByRole("treeitem").filter({ hasText: "home-assistant-konfiguration-und-datenbank" });
  const homeName = (await home.locator("span.font-mono").first().boundingBox())!;
  const homeSize = (await home.getByText(/zu sichern/).boundingBox())!;
  expect(homeSize.y, "a dataset's size left its name's line").toBeLessThan(homeName.y + homeName.height);
  const footer = await dialog
    .getByRole("button", { name: "Abbrechen" })
    .locator("xpath=..")
    .locator("button")
    .evaluateAll((buttons) => [...new Set(buttons.map((b) => Math.round(b.getBoundingClientRect().top)))]);
  expect(footer, "the dialog's buttons wrapped").toHaveLength(1);
});
