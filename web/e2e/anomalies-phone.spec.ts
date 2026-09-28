// The Anomalies page, the phone Home's anomalies card and the More sheet's
// count at phone width. A fresh harness database has detection off and no
// history, so findings of every severity and kind, the watched items and the
// summary are staged at the route layer. On the phones: the 24px card rhythm,
// no pan, nothing past the edge, and open findings where a phone user looks
// first. On the desktop: the 40px rhythm and a dashboard with one anomalies
// card, in its grid. German, because its labels run longest.
import { expect, test, type Page } from "@playwright/test";

const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

// A settings read the page makes as the test ends would otherwise fail it
// from inside a route handler whose response is already gone.
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
    detector: "reliability",
    metric: "failure_streak",
    domain: "vm",
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
    name: "Fotoarchiv Familienurlaube Südtirol",
    observed: 180 * GB,
    expected: 2 * GB,
    details: { sourceBytes: 190 * GB, refBytes: 2 * GB, refRate: 3_000_000 },
  }),
  finding({
    metric: "dump_bytes_shrink",
    severity: "warning",
    scopeKind: "dump",
    name: "immich_postgres_datenbank",
    lastGood: { runId: "run-good-2", snapshotId: "9e11b0a4", at: NOW - 5 * DAY },
    details: { collapse: true, median: 3 * GB },
  }),
  finding({
    detector: "duration",
    metric: "duration_slower",
    severity: "warning",
    scopeKind: "zfsds",
    domain: "zfs",
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
    name: "Paperless-ngx-Dokumentenverwaltung",
    details: { failed: 3, total: 10 },
  }),
  finding({
    detector: "integrity",
    metric: "drill_subset",
    severity: "warning",
    scopeKind: "domain",
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
    domain: "containers,vms",
    name: "",
    observed: 0.04,
    expected: 0,
    expectable: false,
    details: { freeBytes: 80 * GB },
  }),
  finding({
    metric: "source_files_shrink",
    severity: "warning",
    state: "acknowledged",
    name: "Home-Assistant-Konfiguration",
    observed: 120,
    expected: 48_000,
    ackedAt: NOW - DAY,
    ackNote: "Bewusst aufgeräumt, die alten Aufnahmen der Überwachungskamera liegen jetzt auf dem Archivserver",
    stillPresent: true,
  }),
  finding({
    detector: "new_data",
    metric: "new_data",
    severity: "info",
    state: "expected",
    scopeKind: "domain",
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
    open: { critical: 1, warning: 0, info: 0 },
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
    targetId: "tg-nodata",
    domain: "vm",
    name: "Windows-Arbeitsplatzrechner Buchhaltung",
    learning: { samples: 0, needed: 10, newData: 0, source: 0, duration: 0, noData: true },
    typical: { sourceBytes: null, newDataBytes: null, resticMs: null },
  }),
];

interface Stage {
  enabled?: boolean;
  findings?: typeof FINDINGS;
  open?: { critical: number; warning: number; info: number };
}

function summary({ enabled = true, open = { critical: 3, warning: 5, info: 2 } }: Stage) {
  return {
    enabled,
    ready: true,
    generation: 7,
    open,
    recoveredCritical: 1,
    learningItems: 2,
    retentionHeld: 1,
    evalErrors: 0,
    notifyMuted: false,
    backfill: { slots: 0, done: 0, failed: 0, filled: 0, withoutSummary: 0 },
    unmeasuredVolumes: [],
  };
}

async function stage(page: Page, width: number, s: Stage = {}): Promise<void> {
  const enabled = s.enabled ?? true;
  const findings = s.findings ?? FINDINGS;
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
      const rows = state === "open" ? findings.filter((a) => a.state === "open") : findings;
      return route.fulfill({ json: { ok: true, anomalies: rows, nextCursor: "" } });
    },
  );
  // Same seeding as narrow-viewport.spec.ts: the stored locale is the look,
  // and the server's display prefs must not overwrite it mid-boot.
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.addInitScript(() => window.localStorage.setItem("bv-lang", "de"));
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
    .locator("xpath=../..")
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
      overflowing: main ? [...main.querySelectorAll("span, p, dd, dt, h3, label")].filter(past).map(name) : null,
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

async function openFindings(page: Page, url: string): Promise<void> {
  await page.goto(url);
  await expect(page.getByRole("heading", { level: 1, name: "Anomalien" })).toBeVisible();
  await expect(page.getByText(NEXTCLOUD, { exact: false }).first()).toBeVisible();
  await settle(page);
}

for (const width of [320, 360]) {
  test(`anomaly findings @ ${width}px: every kind, filter and detail fits`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
    // Every state with the period filter showing, narrowed to one item, so
    // all five filters and the item chip are on screen at once.
    await page.addInitScript(() =>
      window.localStorage.setItem(
        "bv-anomalies-filter",
        JSON.stringify({ state: "all", period: "90", severity: "", detector: "", domain: "" }),
      ),
    );
    await stage(page, width);
    await openFindings(page, "/anomalies?scope=item:tg-nextcloud#findings");
    await expect(page.getByRole("button", { name: "Wieder alle Elemente anzeigen" })).toBeVisible();

    const details = page.getByRole("button", { name: "Details" });
    const count = await details.count();
    expect(count).toBe(FINDINGS.length);
    for (let i = 0; i < count; i++) await details.nth(i).click();
    await page.getByRole("checkbox", { name: "Alle angezeigten auswählen" }).check();
    await settle(page);

    expect.soft(await cardGap(page)).toBe("24px");
    await expectNothingPansOrClips(page);
  });

  test(`anomaly items @ ${width}px: learning, series and expectations fit`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
    await stage(page, width);
    await page.goto("/anomalies#items");
    await expect(page.getByText("Jellyfin-Mediathek Wohnzimmer")).toBeVisible();
    await settle(page);

    expect.soft(await cardGap(page)).toBe("24px");
    await expectNothingPansOrClips(page);
  });

  test(`anomalies @ ${width}px with detection off and nothing found`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
    await stage(page, width, { enabled: false, findings: [], open: { critical: 0, warning: 0, info: 0 } });
    await page.goto("/anomalies");
    await expect(page.getByText("Die Anomalie-Erkennung ist ausgeschaltet.", { exact: false })).toBeVisible();
    await expect(page.getByText("Keine offenen Anomalien.")).toBeVisible();
    await settle(page);

    expect.soft(await cardGap(page)).toBe("24px");
    await expectNothingPansOrClips(page);
  });
}

test("anomaly findings on a phone: the small controls are big enough to tap", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the larger targets follow the coarse pointer");
  await stage(page, 360);
  await openFindings(page, "/anomalies?scope=item:tg-nextcloud#findings");

  const detailsBox = (await page.getByRole("button", { name: "Details" }).first().boundingBox())!;
  expect(detailsBox.height, "Details is shorter than a fingertip").toBeGreaterThanOrEqual(43.5);

  // A tap a finger's width beside the row's checkbox still ticks it.
  const box = page.getByRole("checkbox", { name: `${NEXTCLOUD} auswählen` });
  await box.scrollIntoViewIfNeeded();
  const r = (await box.boundingBox())!;
  await page.touchscreen.tap(r.x + r.width / 2, r.y + r.height + 10);
  await expect(box).toBeChecked();

  // The same for the item chip's remove control, the only way back to every
  // item.
  const chip = page.getByRole("button", { name: "Wieder alle Elemente anzeigen" });
  await chip.scrollIntoViewIfNeeded();
  const c = (await chip.boundingBox())!;
  await page.touchscreen.tap(c.x + c.width / 2, c.y + c.height + 10);
  await expect(chip).toHaveCount(0);
  expect(new URL(page.url()).searchParams.get("scope")).toBeNull();
});

test("anomalies on the desktop keep the 40px rhythm and the small controls", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "desktop-only");
  await stage(page, testInfo.project.use.viewport!.width);
  await openFindings(page, "/anomalies#findings");

  expect(await cardGap(page)).toBe("40px");
  const detailsBox = (await page.getByRole("button", { name: "Details" }).first().boundingBox())!;
  expect(Math.round(detailsBox.height)).toBe(16);
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
