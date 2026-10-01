// The self-backup page at phone width, with its data staged at the route
// layer: a running backup, five snapshots with one flagged by a finding, two
// off-site targets, a restore link naming a pruned backup and the item's
// anomaly fields. On the phones: the 24px card rhythm, no pan, no control past
// the viewport or its card, the source label above the switch it names, and
// each snapshot's delete action on its own row at the end. On the desktop: the
// 40px rhythm and both rows on one line. German, because its labels run
// longest.
import { expect, test, type Page } from "@playwright/test";

const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

const hex = (c: string) => c.repeat(32);
const HOUR = 3600_000;
const snap = (c: string, hoursAgo: number) => ({
  id: hex(c),
  time: new Date(Date.now() - hoursAgo * HOUR).toISOString(),
  paths: ["/config"],
  tags: ["config"],
  hostname: "tower",
});
// The oldest backup falls on 2 January: a one-digit month and day give the
// shortest date a locale prints, the row with the most room beside its time.
const SNAPSHOTS = [
  snap("a1", 2),
  snap("b2", 26),
  snap("c3", 50),
  snap("d4", 74),
  { ...snap("e5", 0), time: new Date(2026, 0, 2, 12).toISOString() },
];
const FLAGGED = SNAPSHOTS[1].id;

const target = (id: string, name: string, sortOrder: number) => ({
  id,
  domain: "config",
  name,
  repo: `rclone:${id}:bombvault/config`,
  credsRef: "",
  storageClass: "",
  immutable: false,
  schedule: "",
  retentionKeepLast: 0,
  retentionKeepDaily: 7,
  retentionKeepWeekly: 4,
  retentionKeepMonthly: 6,
  limitUpload: 0,
  limitDownload: 0,
  growthBudgetGb: 0,
  enabled: true,
  createdAt: 1,
  sortOrder,
});

const CONFIG_ITEM = {
  targetId: "config",
  domain: "config",
  name: "",
  scheduled: true,
  sensitivity: "",
  effective: "balanced",
  notifyMin: "",
  effectiveNotifyMin: "critical",
  learning: { samples: 12, needed: 8, newData: 0, source: 0, duration: 0, noData: false },
  typical: { sourceBytes: 2_400_000, newDataBytes: 18_000, resticMs: 3100 },
  dump: null,
  datasets: [],
  open: { critical: 1, warning: 0, info: 0 },
  retentionHeld: true,
  selectionSince: 0,
  expectations: [],
};

const FINDING = {
  id: "f1",
  detector: "source",
  metric: "source_bytes",
  severity: "critical",
  state: "open",
  scopeKind: "item",
  scopeId: "config",
  targetId: "config",
  domain: "config",
  part: "",
  targetName: "",
  name: "",
  runId: "r1",
  lastRunId: "r1",
  lastRunAt: 1,
  flaggedSnapshots: [FLAGGED],
  observed: 1,
  expected: 2,
  threshold: 1,
  samples: 12,
  sensitivity: "balanced",
  details: {},
  occurrences: 1,
  firstSeenAt: 1,
  lastSeenAt: 1,
  recoveredAt: 0,
  resolvedAt: 0,
  ackedAt: 0,
  clearedAt: 0,
  ackNote: "",
  notifiedAt: 0,
  expectable: true,
  retentionHeld: true,
  stillPresent: true,
};

async function stage(page: Page, snapshots: typeof SNAPSHOTS, running: boolean): Promise<void> {
  await page.route("**/api/config/snapshots*", (route) => route.fulfill({ json: { ok: true, snapshots } }));
  await page.route("**/api/offsite/targets*", (route) =>
    route.fulfill({
      json: { ok: true, targets: [target("b2fra", "Backblaze B2 Frankfurt", 0), target("hetzner", "Hetzner Storage Box Falkenstein", 1)] },
    }),
  );
  await page.route("**/api/anomalies/summary", (route) =>
    route.fulfill({
      json: {
        ok: true,
        summary: {
          enabled: true,
          ready: true,
          generation: 1,
          open: { critical: 1, warning: 0, info: 0 },
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
  await page.route("**/api/anomalies/items", (route) => route.fulfill({ json: { ok: true, items: [CONFIG_ITEM] } }));
  await page.route(/\/api\/anomalies\?/, (route) =>
    route.fulfill({ json: { ok: true, anomalies: [FINDING], nextCursor: "" } }),
  );
  if (running) {
    await page.route("**/api/progress", (route) =>
      route.fulfill({
        headers: { "content-type": "text/event-stream" },
        body: `data: ${JSON.stringify({ key: "config", phase: "backup", percent: 42, active: true })}\n\n`,
      }),
    );
  }
}

// Seeded the way narrow-viewport.spec.ts seeds it: the stored locale is the
// look, and the server's display prefs must not overwrite it mid-boot.
async function bootGerman(page: Page, width: number, path = "/config"): Promise<void> {
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.addInitScript(() => window.localStorage.setItem("bv-lang", "de"));
  await page.setViewportSize({ width, height: 800 });
  await page.goto(path);
  await expect(page.getByRole("heading", { level: 1, name: "Selbst-Backup" })).toBeVisible();
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
    .locator("#bv-main .max-w-6xl")
    .first()
    .evaluate((root) => getComputedStyle(root).rowGap);
}

// Controls past the viewport, and controls past the padding box of the card
// they sit in: a control can overhang its card long before it reaches the
// edge of the screen.
async function overflow(page: Page) {
  return page.evaluate(() => {
    const main = document.querySelector("#bv-main");
    const vw = window.innerWidth;
    const name = (el: Element) => (el.getAttribute("aria-label") ?? el.textContent ?? "").trim().slice(0, 40);
    const controls = main
      ? [...main.querySelectorAll("button, a, input, select, textarea, [role='switch']")].filter(
          (el) => el.getBoundingClientRect().width > 0,
        )
      : [];
    const pastCard = controls.filter((el) => {
      const card = el.closest(".rounded-card");
      if (!card) return false;
      const box = card.getBoundingClientRect();
      const pad = parseFloat(getComputedStyle(card).paddingInlineEnd);
      return el.getBoundingClientRect().right > box.right - pad + 1;
    });
    return {
      docPan: document.documentElement.scrollWidth - vw,
      mainPan: main ? main.scrollWidth - main.clientWidth : null,
      clipped: controls
        .filter((el) => {
          const r = el.getBoundingClientRect();
          return r.right > vw + 1 || r.left < -1;
        })
        .map(name),
      pastCard: pastCard.map(name),
    };
  });
}

async function expectNothingPans(page: Page): Promise<void> {
  const layout = await overflow(page);
  expect(layout.mainPan, "#bv-main is missing").not.toBeNull();
  expect(layout.docPan, "the document scrolls horizontally").toBeLessThanOrEqual(1);
  expect(layout.mainPan!, "#bv-main scrolls horizontally").toBeLessThanOrEqual(1);
  expect(layout.clipped, "controls clip the viewport edge").toEqual([]);
  expect(layout.pastCard, "controls overhang their card").toEqual([]);
  // A backtick in JSX text compiles fine and renders on the page.
  await expect(page.locator("#bv-main")).not.toContainText("`");
}

function snapshotRows(page: Page) {
  return page.locator("#bv-main div.border-b").filter({ has: page.getByRole("button", { name: "Löschen" }) });
}

for (const width of [320, 360]) {
  test(`self-backup @ ${width}px: a running backup and a full list neither pan nor clip`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
    await stage(page, SNAPSHOTS, true);
    // The restore link names a backup retention has since removed.
    const pruned = Math.floor((Date.now() - 96 * HOUR) / 1000);
    await bootGerman(page, width, `/config?restore=${hex("f9")}&at=${pruned}`);
    await expect(page.getByRole("button", { name: "Backup abbrechen" })).toBeVisible();
    await expect(page.getByText("gibt es nicht mehr")).toBeVisible();
    await expect(page.getByText("Als Anomalie markiert")).toBeVisible();
    await expect(page.getByRole("combobox", { name: "Empfindlichkeit" })).toBeVisible();

    await page.getByRole("tab", { name: "Offsite" }).click();
    await expect(page.getByRole("combobox", { name: "Offsite-Ziel" })).toBeVisible();
    await expect(snapshotRows(page)).toHaveCount(SNAPSHOTS.length);
    await settle(page);

    expect(await cardGap(page)).toBe("24px");
    await expectNothingPans(page);

    // The label heads the switch instead of standing beside a stack of it.
    const labelBox = (await page.getByText("Quelle:", { exact: true }).boundingBox())!;
    const switchBox = (await page.getByRole("tablist", { name: "Quelle:" }).boundingBox())!;
    expect(switchBox.y, "the source switch shares its label's row").toBeGreaterThanOrEqual(labelBox.y + labelBox.height);

    // Each delete action sits below the backup's time, at the row's end.
    for (const row of await snapshotRows(page).all()) {
      const rowBox = (await row.boundingBox())!;
      const timeBox = (await row.locator("span.flex-1").boundingBox())!;
      const deleteBox = (await row.getByRole("button", { name: "Löschen" }).boundingBox())!;
      expect(deleteBox.y, "the delete action shares the time's row").toBeGreaterThanOrEqual(timeBox.y + timeBox.height);
      expect(Math.abs(deleteBox.x + deleteBox.width - (rowBox.x + rowBox.width)), "the delete action left the row's end").toBeLessThanOrEqual(1);
    }
  });

  test(`self-backup @ ${width}px: the delete confirmation opens as a sheet that fits`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the confirm sheet replaces the dialog below 48rem");
    await stage(page, SNAPSHOTS, false);
    await bootGerman(page, width);
    await snapshotRows(page).nth(1).getByRole("button", { name: "Löschen" }).click();

    const sheet = page.getByRole("dialog");
    await expect(sheet).toBeVisible();
    await settle(page);
    const sheetBox = (await sheet.boundingBox())!;
    expect(sheetBox.x).toBeGreaterThanOrEqual(-1);
    expect(sheetBox.x + sheetBox.width).toBeLessThanOrEqual(width + 1);
    for (const button of await sheet.getByRole("button").all()) {
      const box = (await button.boundingBox())!;
      expect(box.x + box.width, "a sheet action runs past the edge").toBeLessThanOrEqual(width + 1);
    }
    await sheet.getByRole("button", { name: "Abbrechen" }).click();
    await expect(sheet).toHaveCount(0);
    await expectNothingPans(page);
  });

  test(`self-backup @ ${width}px: the empty list fits`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
    await stage(page, [], false);
    await bootGerman(page, width);
    await expect(page.getByText("Noch keine Einstellungs-Backups.")).toBeVisible();
    await settle(page);

    expect(await cardGap(page)).toBe("24px");
    await expectNothingPans(page);
  });
}

test("self-backup on a phone saves the switch and the anomaly fields as they change", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only");
  await stage(page, SNAPSHOTS, false);
  const saves: string[] = [];
  await page.route("**/api/settings", (route) => {
    if (route.request().method() !== "PUT") return route.fallback();
    saves.push("settings");
    return route.fulfill({ json: { ok: true } });
  });
  await page.route("**/api/anomalies/items/config/prefs", (route) => {
    saves.push("prefs");
    return route.fulfill({ json: { ok: true } });
  });
  await bootGerman(page, 360);

  await page.getByRole("switch", { name: "BombVaults Einstellungen sichern" }).click();
  await expect.poll(() => saves).toContain("settings");
  await page.getByRole("combobox", { name: "Empfindlichkeit" }).click();
  await page.getByRole("option", { name: "Streng" }).click();
  await expect.poll(() => saves).toContain("prefs");
  await expect(page.locator("#bv-main").getByRole("button", { name: "Speichern" })).toHaveCount(0);
});

test("self-backup on the desktop keeps the 40px rhythm and one-line rows", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "desktop-only");
  await stage(page, SNAPSHOTS, false);
  await bootGerman(page, testInfo.project.use.viewport!.width);
  await expect(snapshotRows(page)).toHaveCount(SNAPSHOTS.length);

  expect(await cardGap(page)).toBe("40px");

  const labelBox = (await page.getByText("Quelle:", { exact: true }).boundingBox())!;
  const switchBox = (await page.getByRole("tablist", { name: "Quelle:" }).boundingBox())!;
  expect(switchBox.x, "the source switch left its label's row").toBeGreaterThan(labelBox.x + labelBox.width);

  for (const row of await snapshotRows(page).all()) {
    const timeBox = (await row.locator("span.flex-1").boundingBox())!;
    const deleteBox = (await row.getByRole("button", { name: "Löschen" }).boundingBox())!;
    expect(deleteBox.y, "the delete action wrapped below the time").toBeLessThan(timeBox.y + timeBox.height);
  }
});
