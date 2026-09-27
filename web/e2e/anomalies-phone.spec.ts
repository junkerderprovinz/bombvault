// The Anomalies page, the phone Home's anomalies card and the More sheet's
// count. A fresh harness database has detection off and no history, so
// findings of every severity and kind, the watched items and the summary are
// staged at the route layer. On the phones: the 24px card rhythm, no pan and
// nothing past the edge with every card, line and panel open. On the desktop:
// one card per item, the 40px rhythm, and the actions reaching the API with
// the ids they name. German, because its labels run longest.
import { expect, test, type Page } from "@playwright/test";

const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

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

/** The cards the open findings make: one per item, restore check series and disk. */
const CARD_COUNT = new Set(FINDINGS.map((a) => a.targetId || `${a.scopeKind}:${a.scopeId}`)).size;

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

async function openPage(page: Page, url = "/anomalies"): Promise<void> {
  await page.goto(url);
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  await expect(page.getByRole("region", { name: NEXTCLOUD })).toBeVisible();
  await settle(page);
}

/** Opens every disclosure on the page: finding lines, monitoring panels, the
 *  quiet items and the closed findings. */
async function openEverything(page: Page): Promise<void> {
  // A select's trigger carries aria-expanded too, but opening one only
  // floats its list; the disclosures are what change the layout.
  const closed = page.locator("#bv-main button[aria-expanded='false']:not([role='combobox'])");
  for (;;) {
    if ((await closed.count()) === 0) break;
    await closed.first().click();
  }
  await settle(page);
}

for (const width of [320, 390]) {
  test(`anomaly cards @ ${width}px: every kind, line and panel open fits`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
    await stage(page, width);
    await openPage(page);
    await expect(page.getByRole("region")).toHaveCount(CARD_COUNT);
    await openEverything(page);
    await expect(page.getByText(`Notiz: ${NOTE}`)).toBeVisible();

    expect.soft(await cardGap(page)).toBe("24px");
    await expectNothingPansOrClips(page);
  });

  test(`anomalies @ ${width}px with detection off and nothing found`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
    await stage(page, width, { enabled: false, findings: [], closed: [], open: { critical: 0, warning: 0, info: 0 } });
    await page.goto("/anomalies");
    await expect(page.getByText("Die Anomalie-Erkennung ist ausgeschaltet.", { exact: false })).toBeVisible();
    await expect(page.getByText("Keine offenen Anomalien.")).toBeVisible();
    await settle(page);

    expect.soft(await cardGap(page)).toBe("24px");
    await expectNothingPansOrClips(page);
  });
}

test("anomaly cards read right to left without panning", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the narrow column is where RTL overflows");
  await stage(page, 390, { lang: "ar" });
  await openPage(page);
  await expect(page.locator("html")).toHaveAttribute("dir", "rtl");
  await openEverything(page);
  await expectNothingPansOrClips(page);
});

test("a finding's line is big enough to tap on a phone", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the larger targets follow the coarse pointer");
  await stage(page, 390);
  await openPage(page);
  const line = page.getByRole("region", { name: NEXTCLOUD }).locator("button[aria-expanded]").first();
  const box = (await line.boundingBox())!;
  expect(box.height, "a finding's line is shorter than a fingertip").toBeGreaterThanOrEqual(43.5);
});

test.describe("on the desktop", () => {
  test.skip(({ viewport, isMobile }) => isMobile || viewport?.width !== 1280, "the desktop checks run once, at 1280");

  test("one card per item, in the 40px rhythm, worst first", async ({ page }) => {
    await stage(page, 1280);
    await openPage(page);

    const cards = page.getByRole("region");
    await expect(cards).toHaveCount(CARD_COUNT);
    await expect(cards.first()).toHaveAttribute("aria-label", NEXTCLOUD);
    const lines = page.getByRole("region", { name: NEXTCLOUD }).getByRole("button", { expanded: false });
    await expect(lines.filter({ hasNotText: "Überwachung" })).toHaveCount(2);
    expect(await cardGap(page)).toBe("40px");
    await expectNothingPansOrClips(page);
  });

  test("a finding's line opens to the whole sentence and its actions", async ({ page }) => {
    await stage(page, 1280);
    await openPage(page);
    const nextcloud = page.getByRole("region", { name: NEXTCLOUD });
    const line = nextcloud.getByRole("button", { name: /Fast leer/ });
    await expect(line).toHaveAttribute("aria-expanded", "false");
    await line.click();
    await expect(line).toHaveAttribute("aria-expanded", "true");
    await expect(nextcloud.getByText(new RegExp(`^${NEXTCLOUD} ist fast leer`))).toBeVisible();
    await expect(nextcloud.getByRole("button", { name: "Als erwartet markieren" })).toBeVisible();
    await expect(nextcloud.getByRole("button", { name: "Quittieren" })).toBeVisible();
    // The card's own main action is the restore, so the line does not repeat it.
    await expect(nextcloud.getByRole("button", { name: /wiederherstellen/ })).toHaveCount(1);
  });

  test("Alle N quittieren sends exactly that item's ids", async ({ page }) => {
    await stage(page, 1280);
    const sent: string[][] = [];
    await page.route("**/api/anomalies/acknowledge", async (route) => {
      sent.push((route.request().postDataJSON() as { ids: string[] }).ids);
      await route.fulfill({ json: { ok: true, changed: 2, skipped: 0, released: 0 } });
    });
    await openPage(page);

    await page.getByRole("region", { name: PAPERLESS }).getByRole("button", { name: "Alle 2 quittieren" }).click();
    await expect.poll(() => sent.length).toBe(1);
    const paperless = FINDINGS.filter((a) => a.targetId === "tg-paperless").map((a) => a.id);
    expect(sent[0]).toEqual(paperless);
  });

  test("a tile hides its severity and brings it back", async ({ page }) => {
    await stage(page, 1280);
    await openPage(page);
    const tile = page.getByRole("group", { name: "Schweregrad" }).getByRole("button", { name: /Warnungen/ });
    await expect(tile).toHaveAttribute("aria-pressed", "true");
    await expect(tile).toContainText("6");

    await tile.click();
    await expect(tile).toHaveAttribute("aria-pressed", "false");
    await expect(page.getByRole("region", { name: "immich_postgres_datenbank" })).toHaveCount(0);
    // A card keeps its findings of the severities still shown.
    const paperless = page.getByRole("region", { name: PAPERLESS });
    await expect(paperless.locator("button[aria-expanded]")).toHaveCount(1);

    await tile.click();
    await expect(page.getByRole("region", { name: "immich_postgres_datenbank" })).toBeVisible();
  });

  test("the closed row lists what was settled, with its note", async ({ page }) => {
    await stage(page, 1280);
    await openPage(page);
    const row = page.getByRole("button", { name: /Geschlossen in den letzten 30 Tagen/ });
    await expect(row).toContainText(String(CLOSED.length));
    await row.click();
    await page.getByRole("button", { name: /^Home-Assistant-Konfiguration:/ }).click();
    await expect(page.getByText(`Notiz: ${NOTE}`)).toBeVisible();
  });

  test("a link to one item opens its card and brings it into view", async ({ page }) => {
    await stage(page, 1280);
    await page.goto("/anomalies?scope=item:tg-paperless");
    const paperless = page.getByRole("region", { name: PAPERLESS });
    await expect(paperless.locator("button[aria-expanded='true']")).toHaveCount(2);
    await expect(paperless).toBeInViewport();
    await expect(
      page.getByRole("region", { name: NEXTCLOUD }).locator("button[aria-expanded='true']"),
    ).toHaveCount(0);
  });

  test("an item's badge on its own page leads to its card", async ({ page }) => {
    await stage(page, 1280);
    await page.goto("/anomalies?scope=item:tg-tank");
    await expect(page.getByRole("region", { name: "tank" }).locator("button[aria-expanded='true']")).toHaveCount(1);
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
