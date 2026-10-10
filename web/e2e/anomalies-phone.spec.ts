// The Anomalies page, the phone Home's anomalies card and the More sheet's
// count. A fresh harness database has detection off and no history, so
// findings of every severity and kind, the watched items, their measurements
// and the summary are staged at the route layer. On the phones: the 24px card
// rhythm, no pan and nothing past the edge on the page or in a sheet. On the
// desktop: one flat list, the 40px rhythm, the tiles, the selector, the windows,
// and the actions reaching the API with the ids they name. German, because its
// labels run longest.
import { expect, test, type Locator, type Page } from "@playwright/test";

const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

// A test can pass while the settings handler still waits on the real server.
// Closing the context under it disposes the response it is about to read, and
// Playwright reports that as a failure of the test that already passed. Its
// answer no longer matters then, so what the handler throws is dropped.
test.afterEach(async ({ page }) => {
  await page.unrouteAll({ behavior: "ignoreErrors" });
});

const NOW = Math.floor(Date.now() / 1000);
const HOUR = 3600;
const DAY = 86400;
const GB = 1024 ** 3;

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
    runId: `run-${seq}`,
    lastRunId: `run-${seq}`,
    lastRunAt: NOW - HOUR,
    observed: 2 * GB,
    expected: 40 * GB,
    threshold: 20 * GB,
    samples: 14,
    sensitivity: "balanced",
    details: { median: 40 * GB, mad: GB, z: 7.4 },
    occurrences: 1,
    firstSeenAt: NOW - 3 * DAY,
    lastSeenAt: NOW - HOUR,
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

const NEXTCLOUD = "Nextcloud_AIO_Datenbank_Hauptserver_Produktivsystem";
const PAPERLESS = "Paperless-ngx-Dokumentenverwaltung";
const NOTE = "Bewusst aufgeräumt, die alten Aufnahmen der Überwachungskamera liegen jetzt auf dem Archivserver";

const FINDINGS = [
  finding({
    targetId: "tg-nextcloud",
    name: NEXTCLOUD,
    occurrences: 3,
    retentionHeld: true,
    lastGood: { runId: "run-good", snapshotId: "5f3a9c1e", at: NOW - 2 * DAY },
    details: { median: 40 * GB, mad: GB, z: 7.4, collapse: true },
  }),
  finding({
    targetId: "tg-nextcloud",
    detector: "duration",
    metric: "duration_slower",
    severity: "warning",
    name: NEXTCLOUD,
    observed: 9_660_000,
    expected: 860_000,
    threshold: 2_000_000,
  }),
  finding({
    detector: "reliability",
    metric: "failure_streak",
    domain: "vm",
    targetId: "tg-windows",
    name: "Windows-Arbeitsplatzrechner Buchhaltung",
    observed: 4,
    expected: 0,
    threshold: 3,
    expectable: false,
    recoveredAt: NOW - 2 * HOUR,
    details: {},
  }),
  finding({
    detector: "integrity",
    metric: "drill_dr",
    severity: "critical",
    scopeKind: "domain",
    scopeId: "flash:offsite:t1",
    targetId: "",
    domain: "flash",
    name: "",
    targetName: "Hetzner Storage Box Frankfurt Zweitstandort",
    samples: 3,
    expectable: false,
    details: {},
  }),
  finding({
    detector: "new_data",
    metric: "new_data_rewrite",
    severity: "warning",
    domain: "files",
    targetId: "tg-fotos",
    name: "Fotoarchiv Familienurlaube Südtirol",
    observed: 180 * GB,
    expected: 2 * GB,
    details: { sourceBytes: 190 * GB, refBytes: 2 * GB, refRate: 3_000_000 },
  }),
  finding({
    metric: "dump_bytes_shrink",
    severity: "warning",
    scopeKind: "dump",
    targetId: "tg-immich",
    name: "immich_postgres_datenbank",
    retentionHeld: true,
    lastGood: { runId: "run-good-2", snapshotId: "9e11b0a4", at: NOW - 5 * DAY },
    details: { collapse: true, median: 3 * GB },
  }),
  finding({
    detector: "duration",
    metric: "duration_slower",
    severity: "warning",
    scopeKind: "zfsds",
    domain: "zfs",
    targetId: "tg-tank",
    name: "tank",
    part: "tank/fotos/archiv/familienurlaube-suedtirol-2019-bis-2026",
    observed: 5_400_000,
    expected: 600_000,
    threshold: 1_800_000,
  }),
  finding({
    detector: "reliability",
    metric: "flaky",
    severity: "warning",
    targetId: "tg-paperless",
    name: PAPERLESS,
    details: { failed: 3, total: 10 },
  }),
  finding({
    detector: "source",
    metric: "source_bytes_growth",
    severity: "info",
    targetId: "tg-paperless",
    name: PAPERLESS,
    observed: 86 * GB,
    expected: 12 * GB,
  }),
  finding({
    detector: "integrity",
    metric: "drill_subset",
    severity: "warning",
    scopeKind: "domain",
    scopeId: "containers:offsite:t1",
    targetId: "",
    domain: "containers",
    name: "",
    targetName: "Hetzner Storage Box Frankfurt Zweitstandort",
    expectable: false,
    details: {},
  }),
  finding({
    detector: "capacity",
    metric: "capacity_eta",
    severity: "info",
    scopeKind: "volume",
    scopeId: "vol-cache",
    targetId: "",
    domain: "containers,vms,files",
    name: "",
    observed: 23,
    expected: 0,
    expectable: false,
    details: { etaGrowthDays: 23, etaFreeDays: 40, slopePerDay: 12 * GB },
  }),
  finding({
    detector: "capacity",
    metric: "capacity_low",
    severity: "info",
    scopeKind: "volume",
    scopeId: "vol-array",
    targetId: "",
    domain: "containers,vms",
    name: "",
    observed: 0.04,
    expected: 0,
    expectable: false,
    details: { freeBytes: 80 * GB },
  }),
];

const CLOSED = [
  finding({
    metric: "source_files_shrink",
    severity: "warning",
    state: "acknowledged",
    targetId: "tg-homeassistant",
    name: "Home-Assistant-Konfiguration",
    observed: 120,
    expected: 48_000,
    ackedAt: NOW - DAY,
    ackNote: NOTE,
    stillPresent: true,
  }),
  finding({
    detector: "new_data",
    metric: "new_data",
    severity: "info",
    state: "expected",
    scopeKind: "item",
    targetId: "config",
    domain: "config",
    name: "",
    ackedAt: NOW - 2 * DAY,
    details: { refBytes: 50 * 1024 ** 2 },
  }),
];

function series(part: string, samples: number) {
  return {
    part,
    learning: { samples, needed: 10 },
    typical: { sourceBytes: samples >= 10 ? 12 * GB : null, resticMs: samples >= 10 ? 300_000 : null },
    open: { critical: 0, warning: 1, info: 0 },
    retentionHeld: false,
  };
}

function item(over: Record<string, unknown>) {
  return {
    targetId: "tg-item",
    domain: "container",
    name: "plex",
    scheduled: true,
    sensitivity: "",
    effective: "balanced",
    notifyMin: "",
    effectiveNotifyMin: "warning",
    learning: { samples: 10, needed: 10, newData: 10, source: 10, duration: 10, noData: false },
    typical: { sourceBytes: 40 * GB, newDataBytes: 2 * GB, resticMs: 900_000 },
    dump: null,
    datasets: [],
    open: { critical: 0, warning: 0, info: 0 },
    retentionHeld: false,
    selectionSince: 0,
    expectations: [],
    ...over,
  };
}

const ITEMS = [
  item({
    targetId: "tg-nextcloud",
    name: NEXTCLOUD,
    dump: series("", 4),
    open: { critical: 1, warning: 1, info: 0 },
    retentionHeld: true,
    selectionSince: NOW - 9 * DAY,
    expectations: [
      { scopeKind: "item", part: "", family: "new_data", sinceAt: NOW - 4 * DAY, ceiling: 0, updatedAt: NOW },
      { scopeKind: "dump", part: "", family: "source", sinceAt: NOW - 4 * DAY, ceiling: 30 * GB, updatedAt: NOW },
    ],
  }),
  item({
    targetId: "tg-learning",
    name: "Jellyfin-Mediathek Wohnzimmer",
    learning: { samples: 3, needed: 10, newData: 3, source: 3, duration: 2, noData: false },
    typical: { sourceBytes: null, newDataBytes: null, resticMs: null },
  }),
  item({ targetId: "tg-unscheduled", name: "Testcontainer ohne Zeitplan", scheduled: false }),
  item({
    targetId: "tg-tank",
    domain: "zfs",
    name: "tank",
    datasets: [
      series("tank/fotos/archiv/familienurlaube-suedtirol-2019-bis-2026", 12),
      series("tank/dokumente/steuererklaerungen", 2),
    ],
    expectations: [
      {
        scopeKind: "zfsds",
        part: "tank/fotos/archiv/familienurlaube-suedtirol-2019-bis-2026",
        family: "duration",
        sinceAt: NOW - 3 * DAY,
        ceiling: 0,
        updatedAt: NOW,
      },
    ],
  }),
  item({
    targetId: "tg-archiv",
    domain: "zfs",
    name: "archiv",
    datasets: [series("archiv/dokumente/steuererklaerungen-und-belege-2019-bis-2026", 12)],
  }),
  item({
    targetId: "tg-nodata",
    domain: "vm",
    name: "Windows-Arbeitsplatzrechner Buchhaltung Zweitgerät",
    learning: { samples: 0, needed: 10, newData: 0, source: 0, duration: 0, noData: true },
    typical: { sourceBytes: null, newDataBytes: null, resticMs: null },
  }),
];

interface Stage {
  enabled?: boolean;
  findings?: typeof FINDINGS;
  closed?: typeof CLOSED;
  open?: { critical: number; warning: number; info: number };
  lang?: string;
}

function summary({ enabled = true, open = { critical: 3, warning: 6, info: 3 } }: Stage) {
  return {
    enabled,
    ready: true,
    generation: 7,
    open,
    recoveredCritical: 1,
    learningItems: 2,
    retentionHeld: 2,
    evalErrors: 0,
    notifyMuted: false,
    backfill: { slots: 0, done: 0, failed: 0, filled: 0, withoutSummary: 0 },
    unmeasuredVolumes: [],
  };
}

/** What the backups of an item measured: a steady size that the newest run
 *  left, and for the item still learning three runs without a usual range. */
function measured(targetId: string) {
  const learning = targetId === "tg-learning";
  const runs = learning ? 3 : 28;
  return [
    {
      scopeKind: "item",
      part: "",
      failed: [{ runId: "run-failed", at: NOW - 6 * DAY - HOUR }],
      quantities: [
        {
          quantity: "sourceBytes",
          points: Array.from({ length: runs }, (_, i) => ({
            runId: i === runs - 1 ? "run-1" : `run-old-${i}`,
            at: NOW - HOUR - (runs - 1 - i) * DAY,
            value: i === runs - 1 && !learning ? 2 * GB : (40 + Math.sin(i)) * GB,
          })),
          learning,
          samples: learning ? 3 : 10,
          needed: 10,
          band: learning
            ? null
            : { low: { metric: "source_bytes_shrink", expected: 40 * GB, threshold: 20 * GB, samples: 14 }, high: null },
        },
      ],
    },
  ];
}

const CHANGES = {
  state: "ready",
  fromAt: NOW - 2 * DAY,
  toAt: NOW - HOUR,
  summary: {
    total: { path: "", removedBytes: 38 * GB, removedFiles: 91_204, addedBytes: 0, addedFiles: 0, changedBytes: GB, changedFiles: 12 },
    folders: [
      {
        path: "nextcloud_aio_volumes/datenbank/sicherung/archiv-2019-bis-2026",
        removedBytes: 38 * GB,
        removedFiles: 91_204,
        addedBytes: 0,
        addedFiles: 0,
        changedBytes: 0,
        changedFiles: 0,
      },
    ],
    other: { path: "", removedBytes: 0, removedFiles: 0, addedBytes: 0, addedFiles: 0, changedBytes: GB, changedFiles: 12 },
    focus: "nextcloud_aio_volumes/datenbank/sicherung/archiv-2019-bis-2026",
  },
};

async function stage(page: Page, width: number, s: Stage = {}): Promise<void> {
  const enabled = s.enabled ?? true;
  const findings = s.findings ?? FINDINGS;
  const closed = s.closed ?? CLOSED;
  await page.route("**/api/settings", async (route) => {
    if (route.request().method() !== "GET") return route.continue();
    const res = await route.fetch();
    const body = await res.json();
    await route.fulfill({ json: { ...body, settings: { ...body.settings, anomalyEnabled: enabled } } });
  });
  await page.route("**/api/anomalies/summary", (route) =>
    route.fulfill({ json: { ok: true, summary: summary(s) } }),
  );
  await page.route("**/api/anomalies/items", (route) => route.fulfill({ json: { ok: true, items: ITEMS } }));
  await page.route(
    (url) => url.pathname === "/api/anomalies",
    (route) => {
      const state = new URL(route.request().url()).searchParams.get("state");
      const rows = state === "open" ? findings : state === "closed" ? closed : [...findings, ...closed];
      return route.fulfill({ json: { ok: true, anomalies: rows, nextCursor: "" } });
    },
  );
  await page.route("**/api/anomalies/items/*/series", (route) => {
    const targetId = decodeURIComponent(new URL(route.request().url()).pathname.split("/")[4]);
    return route.fulfill({ json: { ok: true, series: measured(targetId) } });
  });
  await page.route("**/api/anomalies/*/changes*", (route) => route.fulfill({ json: { ok: true, changes: CHANGES } }));
  // Same seeding as narrow-viewport.spec.ts: the stored locale is the look,
  // and the server's display prefs must not overwrite it mid-boot.
  await page.route("**/api/display-prefs*", (route) => route.abort());
  const lang = s.lang ?? "de";
  await page.addInitScript((code) => window.localStorage.setItem("bv-lang", code), lang);
  await page.setViewportSize({ width, height: 800 });
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
    .locator("xpath=..")
    .evaluate((root) => getComputedStyle(root).rowGap);
}

// Controls past the edge, and any box at all that runs out of the column: a
// name without spaces widens its span without always widening the scroller.
async function expectNothingPansOrClips(page: Page): Promise<void> {
  const layout = await page.evaluate(() => {
    const main = document.querySelector("#bv-main");
    const vw = window.innerWidth;
    const past = (el: Element) => {
      const r = el.getBoundingClientRect();
      return r.width > 0 && (r.right > vw + 1 || r.left < -1);
    };
    const name = (el: Element) => (el.getAttribute("aria-label") ?? el.textContent ?? "").trim().slice(0, 40);
    return {
      docPan: document.documentElement.scrollWidth - vw,
      mainPan: main ? main.scrollWidth - main.clientWidth : null,
      clipped: main
        ? [...main.querySelectorAll("button, a, input, select, textarea, [role='switch']")].filter(past).map(name)
        : null,
      overflowing: main ? [...main.querySelectorAll("span, p, dd, dt, h2, h3, label")].filter(past).map(name) : null,
    };
  });
  expect(layout.mainPan, "#bv-main is missing").not.toBeNull();
  expect.soft(layout.docPan, "the document scrolls horizontally").toBeLessThanOrEqual(1);
  expect.soft(layout.mainPan!, "#bv-main scrolls horizontally").toBeLessThanOrEqual(1);
  expect.soft(layout.clipped, "controls clip the viewport edge").toEqual([]);
  expect.soft(layout.overflowing, "text runs past the viewport edge").toEqual([]);
  // A backtick in JSX text compiles fine and renders on the page.
  await expect.soft(page.locator("#bv-main")).not.toContainText("`");
}

function findings(page: Page): Locator {
  return page.locator("#bv-main li[id^='finding-']");
}

/** The finding filed under an item's name, the first of them if it has several. */
function findingOf(page: Page, name: string): Locator {
  return findings(page).filter({ hasText: name }).first();
}

async function openPage(page: Page, url = "/anomalies"): Promise<void> {
  await page.goto(url);
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  await expect(findingOf(page, NEXTCLOUD)).toBeVisible();
  await settle(page);
}

// A window is portalled to the body, outside #bv-main, so it gets its own
// check: no box inside it may reach past the viewport.
async function expectWindowFits(page: Page): Promise<void> {
  const window = page.getByRole("dialog");
  await expect(window).toBeVisible();
  await settle(page);
  const past = await window.evaluate((root) => {
    const vw = innerWidth;
    return [...root.querySelectorAll("button, a, p, span, dt, dd, h3, svg, [role='combobox']")]
      .filter((el) => {
        const r = el.getBoundingClientRect();
        return r.width > 0 && (r.right > vw + 1 || r.left < -1);
      })
      .map((el) => (el.getAttribute("aria-label") ?? el.textContent ?? el.tagName).trim().slice(0, 40));
  });
  expect.soft(past, "the window runs past the viewport edge").toEqual([]);
}

async function closeWindow(page: Page): Promise<void> {
  await page.getByRole("dialog").getByRole("button", { name: "Schließen" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
}

for (const width of [320, 390]) {
  test(`anomalies @ ${width}px: every kind of finding, the rings and both windows fit`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
    await stage(page, width);
    await openPage(page);
    await expect(findings(page)).toHaveCount(FINDINGS.length);
    await expect(page.getByRole("button", { name: /^Jellyfin-Mediathek Wohnzimmer · / })).toBeVisible();
    expect.soft(await cardGap(page)).toBe("24px");
    await expectNothingPansOrClips(page);

    await findingOf(page, NEXTCLOUD).getByRole("button", { name: "Details" }).click();
    await expect(page.getByRole("dialog").getByRole("img", { name: /Größe der Quelle je Sicherung/ })).toBeVisible();
    await page.getByRole("dialog").getByRole("button", { name: /vergleichen/ }).click();
    await expect(page.getByRole("dialog").getByText(/Fast alles davon liegt in/)).toBeVisible();
    await expectWindowFits(page);

    await page.getByRole("dialog").getByRole("button", { name: "Überwachung" }).click();
    await expect(page.getByRole("dialog").getByRole("combobox", { name: "Empfindlichkeit" })).toBeVisible();
    await expectWindowFits(page);
    await closeWindow(page);

    await page.getByRole("tab", { name: "Geschlossen in den letzten 30 Tagen" }).click();
    await expect(page.getByText(`Notiz: ${NOTE}`, { exact: false })).toBeVisible();
    await expectNothingPansOrClips(page);
  });

  test(`anomalies @ ${width}px with detection off and nothing found`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
    await stage(page, width, { enabled: false, findings: [], closed: [], open: { critical: 0, warning: 0, info: 0 } });
    await page.goto("/anomalies");
    await expect(page.getByText("Die Anomalie-Erkennung ist ausgeschaltet.", { exact: false })).toBeVisible();
    await expect(page.getByRole("button", { name: "Einstellungen öffnen" })).toBeVisible();
    await expect(page.getByText("Keine offenen Anomalien.")).toBeVisible();
    await settle(page);

    expect.soft(await cardGap(page)).toBe("24px");
    await expectNothingPansOrClips(page);
  });
}

test("the findings and a finding's window read right to left without panning", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the narrow column is where RTL overflows");
  await stage(page, 390, { lang: "ar" });
  await openPage(page);
  await expect(page.locator("html")).toHaveAttribute("dir", "rtl");
  await expectNothingPansOrClips(page);

  // Details leads the second group of a finding's buttons, whatever it is called.
  await findingOf(page, NEXTCLOUD).locator(":scope > div:last-child > span:last-child button").first().click();
  await expect(page.getByRole("dialog").getByRole("img")).toBeVisible();
  await expectWindowFits(page);
});

test("a learning ring is big enough to tap on a phone", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the larger targets follow the coarse pointer");
  await stage(page, 390);
  await openPage(page);
  const ring = page.getByRole("button", { name: /^Jellyfin-Mediathek Wohnzimmer · / });
  const box = (await ring.boundingBox())!;
  expect(box.height, "a ring is shorter than a fingertip").toBeGreaterThanOrEqual(43.5);
  expect(box.width, "a ring is narrower than a fingertip").toBeGreaterThanOrEqual(43.5);
});

test.describe("on the desktop", () => {
  test.skip(({ viewport, isMobile }) => isMobile || viewport?.width !== 1280, "the desktop checks run once, at 1280");

  test("one flat list in the 40px rhythm, worst first", async ({ page }) => {
    await stage(page, 1280);
    await openPage(page);

    await expect(findings(page)).toHaveCount(FINDINGS.length);
    await expect(findings(page).first()).toContainText(NEXTCLOUD);
    await expect(findings(page).first()).toContainText("Kritisch");
    await expect(findings(page).last()).toContainText("Hinweis");
    await expect(page.locator("#bv-main [aria-expanded]")).toHaveCount(0);
    expect(await cardGap(page)).toBe("40px");
    await expectNothingPansOrClips(page);
  });

  test("the first finding carries the page's one filled button, and it is the restore", async ({ page }) => {
    await stage(page, 1280);
    await openPage(page);
    const restore = findings(page).first().getByRole("button", { name: /wiederherstellen/ });
    await expect(restore).toBeVisible();
    const filled = await page
      .locator("#bv-main li[id^='finding-'] button")
      .evaluateAll((all) => all.filter((b) => b.className.includes("bg-accent")).length);
    expect(filled).toBe(1);

    await restore.click();
    await expect(page).toHaveURL(/\/containers\?restore=5f3a9c1e&at=\d+&item=Nextcloud_AIO/);
  });

  test("Details opens a window with the whole sentence, the curve and every action", async ({ page }) => {
    await stage(page, 1280);
    await openPage(page);
    await findingOf(page, NEXTCLOUD).getByRole("button", { name: "Details" }).click();

    const window = page.getByRole("dialog");
    await expect(window.getByText(new RegExp(`^${NEXTCLOUD} ist fast leer`))).toBeVisible();
    await expect(window.getByRole("img", { name: /Größe der Quelle je Sicherung/ })).toBeVisible();
    await expect(window.locator("[data-part='band']")).toHaveCount(1);
    await expect(window.locator("[data-part='marked']")).toHaveCount(1);
    await expect(window.locator("[data-part='failed']")).toHaveCount(1);
    await expect(window.getByText("Üblicher Bereich: über", { exact: false })).toBeVisible();
    await expect(window.getByRole("button", { name: "Als erwartet markieren" })).toBeVisible();
    await expect(window.getByRole("button", { name: "Quittieren" })).toBeVisible();
    await expect(window.getByRole("button", { name: /wiederherstellen/ })).toBeVisible();
    await expectWindowFits(page);

    await page.keyboard.press("Escape");
    await expect(window).toHaveCount(0);
  });

  test("the comparison opens from the list and names the folder", async ({ page }) => {
    await stage(page, 1280);
    await openPage(page);
    await findingOf(page, "Fotoarchiv").getByRole("button", { name: /vergleichen/ }).click();
    const window = page.getByRole("dialog");
    await expect(window.getByText(/Fast alles davon liegt in/)).toBeVisible();
    await expect(window.getByRole("button", { name: /vergleichen/ })).toHaveCount(0);
  });

  test("Alle N quittieren sends exactly that item's ids", async ({ page }) => {
    await stage(page, 1280);
    const sent: string[][] = [];
    await page.route("**/api/anomalies/acknowledge", async (route) => {
      sent.push((route.request().postDataJSON() as { ids: string[] }).ids);
      await route.fulfill({ json: { ok: true, changed: 2, skipped: 0, released: 0 } });
    });
    await openPage(page);

    const all = page.getByRole("button", { name: "Alle 2 quittieren" });
    await expect(all).toHaveCount(2);
    await findingOf(page, PAPERLESS).getByRole("button", { name: "Alle 2 quittieren" }).click();
    await expect.poll(() => sent.length).toBe(1);
    const paperless = FINDINGS.filter((a) => a.targetId === "tg-paperless").map((a) => a.id);
    expect(sent[0]).toEqual(paperless);
  });

  test("a tile shows its severity, and All brings the rest back", async ({ page }) => {
    await stage(page, 1280);
    await openPage(page);
    const tiles = page.getByRole("group", { name: "Schweregrad" });
    const warnings = tiles.getByRole("button", { name: /^Warnungen/ });
    await expect(tiles.getByRole("button")).toHaveCount(4);
    await expect(tiles.getByRole("button", { name: /^Alle/ })).toHaveAttribute("aria-pressed", "true");
    await expect(warnings).toHaveAttribute("aria-pressed", "false");
    await expect(warnings).toContainText("6 offen");

    await warnings.click();
    await expect(warnings).toHaveAttribute("aria-pressed", "true");
    await expect(tiles.getByRole("button")).toHaveCount(4);
    await expect(findings(page)).toHaveCount(6);
    await expect(findingOf(page, "immich_postgres_datenbank")).toBeVisible();
    // Only one of the item's two findings is a warning, so there is no pair to settle.
    await expect(findings(page).filter({ hasText: PAPERLESS })).toHaveCount(1);
    await expect(findingOf(page, PAPERLESS).getByRole("button", { name: /^Alle \d+ quittieren/ })).toHaveCount(0);
    await expect(page.getByRole("button", { name: /^Jellyfin-Mediathek Wohnzimmer · / })).toHaveCount(0);

    await tiles.getByRole("button", { name: /^Alle/ }).click();
    await expect(findings(page)).toHaveCount(FINDINGS.length);
  });

  test("the selector lists what was closed, with its note", async ({ page }) => {
    await stage(page, 1280);
    await openPage(page);
    await page.getByRole("tab", { name: "Geschlossen in den letzten 30 Tagen" }).click();

    await expect(findings(page)).toHaveCount(CLOSED.length);
    await expect(page.getByRole("group", { name: "Schweregrad" }).getByRole("button", { name: /^Alle/ })).toContainText(
      `${CLOSED.length} geschlossen`,
    );
    const settled = findingOf(page, "Home-Assistant-Konfiguration");
    await expect(settled.getByText(`Notiz: ${NOTE}`, { exact: false })).toBeVisible();
    await expect(settled.getByText("Quittiert", { exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "Quittieren" })).toHaveCount(0);

    await page.getByRole("tab", { name: "Offen" }).click();
    await expect(findings(page)).toHaveCount(FINDINGS.length);
  });

  test("a link to an item with several findings brings the first into view", async ({ page }) => {
    await stage(page, 1280);
    await page.goto("/anomalies?scope=item:tg-paperless");
    await expect(findingOf(page, PAPERLESS)).toBeInViewport();
    await expect(page.getByRole("dialog")).toHaveCount(0);
  });

  test("an item's badge on its own page opens its only finding", async ({ page }) => {
    await stage(page, 1280);
    await page.goto("/anomalies?scope=item:tg-tank");
    const window = page.getByRole("dialog");
    await expect(window).toBeVisible();
    await expect(window.getByText("tank/fotos/archiv/familienurlaube-suedtirol-2019-bis-2026").first()).toBeVisible();
  });

  test("a learning ring opens the item's monitoring, without a usual range while it learns", async ({ page }) => {
    await stage(page, 1280);
    await openPage(page);
    const ring = page.getByRole("button", { name: /^Jellyfin-Mediathek Wohnzimmer · / });
    await expect(ring).toHaveAttribute("aria-label", /Lernt noch: 3 von 10 Sicherungen/);
    await expect(ring.locator("circle[data-lit='true']")).toHaveCount(3);
    await ring.click();

    const window = page.getByRole("dialog");
    await expect(window.getByText("Lernt noch: 3 von 10 Sicherungen")).toBeVisible();
    await expect(window.getByRole("combobox", { name: "Empfindlichkeit" })).toBeVisible();
    await expect(window.getByText("Üblicher Bereich: steht nach 10 Sicherungen fest")).toBeVisible();
    await expect(window.locator("[data-part='band']")).toHaveCount(0);
    await expect(window.locator("[data-part='to-learn']")).toHaveCount(7);
    await closeWindow(page);
  });
});

// The phone Home column, top to bottom, by its section headings.
async function homeOrder(page: Page): Promise<string[]> {
  return page
    .locator("#bv-main h2")
    .evaluateAll((hs) => hs.map((h) => (h.textContent ?? "").trim()).filter(Boolean));
}

test("the phone Home leads with the anomalies card while findings are open", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the Home blocks replace the grid below 48rem");
  await stage(page, 360);
  await page.goto("/dashboard");
  await expect(page.getByText(NEXTCLOUD, { exact: false }).first()).toBeVisible();
  await settle(page);

  const order = await homeOrder(page);
  expect(order[0]).toBe("Anomalien");
  expect(order.filter((h) => h === "Anomalien")).toHaveLength(1);
  await expect(page.getByRole("button", { name: "Quittieren" }).first()).toBeVisible();
  await expectNothingPansOrClips(page);
});

test("the phone Home keeps a quiet anomalies card below storage", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the Home blocks replace the grid below 48rem");
  await stage(page, 360, { findings: [], open: { critical: 0, warning: 0, info: 2 } });
  await page.goto("/dashboard");
  await expect(page.getByText("Keine Anomalien in deinen Backups.")).toBeVisible();

  const order = await homeOrder(page);
  expect(order.indexOf("Anomalien")).toBe(order.indexOf("Speicher") + 1);
});

test("the dashboard on the desktop shows the anomalies card once, in its grid", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "desktop-only");
  await stage(page, testInfo.project.use.viewport!.width);
  await page.goto("/dashboard");
  await expect(page.getByText(NEXTCLOUD, { exact: false }).first()).toBeVisible();

  const order = await homeOrder(page);
  expect(order.filter((h) => h === "Anomalien")).toHaveLength(1);
  expect(order[0]).not.toBe("Anomalien");
});

test("the More sheet counts the open findings and the bar marks its trigger while there are any", async ({
  page,
}, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the bar and its sheet live below 48rem");
  const quiet = { critical: 0, warning: 0, info: 4 };
  const loud = { critical: 3, warning: 5, info: 2 };
  let open = quiet;
  await stage(page, 360);
  await page.route("**/api/anomalies/summary", (route) =>
    route.fulfill({ json: { ok: true, summary: summary({ open }) } }),
  );
  await page.goto("/containers");

  const more = page.getByRole("button", { name: "Mehr", exact: true });
  const sheet = page.getByTestId("more-sheet");
  const row = sheet.getByRole("link", { name: /Anomalien/ });
  await expect(more).toBeVisible();
  await expect(more.getByTestId("more-anomaly-dot")).toHaveCount(0);
  await more.click();
  await expect(row).toHaveText("Anomalien");
  await page.keyboard.press("Escape");
  await expect(sheet).toBeHidden();

  // Notes alone leave the bar alone; a warning or a critical marks it.
  open = loud;
  await page.evaluate(() => window.dispatchEvent(new Event("bv:anomalies-changed")));
  await expect(more.getByTestId("more-anomaly-dot")).toBeVisible();
  await more.click();
  await expect(row.getByLabel("Offene kritische Anomalien und Warnungen: 8")).toHaveText("8");
});
