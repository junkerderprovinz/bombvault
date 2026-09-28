// The Flash page at phone width, with backups, off-site targets, a running
// backup, an open finding and the ZIP export staged at the route layer (a fresh
// harness database has none of them). On the phones: the 24px card rhythm, no
// pan, no control past the edge on either source, each backup's actions
// wrapping under its id and time, and the off-site target picker at control
// height under a coarse pointer. On the desktop: the 40px rhythm, one line per
// backup and the compact picker. German, because its labels run longest.
import { expect, test, type Page } from "@playwright/test";

const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

const hex = (seed: string) => seed.repeat(32);

const snap = (seed: string, time: string, original?: string) => ({
  id: hex(seed),
  time,
  paths: ["/boot"],
  tags: ["flash", "scheduled"],
  hostname: "Tower",
  ...(original ? { original } : {}),
});

const LOCAL = [
  snap("a1", "2026-09-26T03:15:00Z"),
  snap("b2", "2026-09-19T03:15:00Z"),
  snap("c3", "2026-09-12T03:15:00Z"),
  snap("d4", "2026-09-05T03:15:00Z"),
];
const OFFSITE = [snap("e5", "2026-09-26T04:02:00Z", LOCAL[0].id), snap("f6", "2026-09-19T04:05:00Z", LOCAL[1].id)];

const target = (id: string, name: string, sortOrder: number) => ({
  id,
  domain: "flash",
  name,
  repo: `s3:https://s3.eu-central-1.amazonaws.com/tower-backup/${id}`,
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

const TARGETS = [target("primary", "Hetzner Storage Box Falkenstein", 0), target("second", "Backblaze B2 Amsterdam", 1)];

const NO_OPEN = { critical: 0, warning: 0, info: 0 };

const FLASH_ITEM = {
  targetId: "flash",
  domain: "flash",
  name: "flash",
  scheduled: true,
  sensitivity: "",
  effective: "balanced",
  notifyMin: "",
  effectiveNotifyMin: "critical",
  learning: { samples: 12, needed: 5, newData: 12, source: 12, duration: 12, noData: false },
  typical: { sourceBytes: 1_073_741_824, newDataBytes: 4_194_304, resticMs: 42_000 },
  dump: null,
  datasets: [],
  open: { ...NO_OPEN, critical: 1 },
  retentionHeld: true,
  selectionSince: 0,
  expectations: [],
};

// A data-loss finding on the second local backup, so that row and its
// off-site copy carry the flagged badge, the widest thing a row can hold.
const FINDING = {
  id: "f1",
  detector: "shrink",
  metric: "sourceBytes",
  severity: "critical",
  state: "open",
  scopeKind: "item",
  scopeId: "flash",
  targetId: "flash",
  domain: "flash",
  part: "",
  targetName: "",
  name: "flash",
  runId: "r1",
  lastRunId: "r1",
  lastRunAt: 1_789_787_700,
  flaggedSnapshots: [LOCAL[1].id],
  observed: 1,
  expected: 1_073_741_824,
  threshold: 0.5,
  samples: 12,
  sensitivity: "balanced",
  details: {},
  occurrences: 1,
  firstSeenAt: 1_789_787_700,
  lastSeenAt: 1_789_787_700,
  recoveredAt: 0,
  resolvedAt: 0,
  ackedAt: 0,
  clearedAt: 0,
  ackNote: "",
  notifiedAt: 0,
  expectable: false,
  retentionHeld: true,
  stillPresent: true,
};

async function bootGerman(page: Page, width: number, opts: { empty?: boolean; query?: string } = {}): Promise<void> {
  await page.route("**/api/flash/snapshots*", (route) => {
    const offsite = new URL(route.request().url()).searchParams.has("source");
    const snapshots = opts.empty ? [] : offsite ? OFFSITE : LOCAL;
    return route.fulfill({ json: { ok: true, snapshots } });
  });
  await page.route("**/api/offsite/targets*", (route) => route.fulfill({ json: { ok: true, targets: TARGETS } }));
  // The real settings with the ZIP export switched on, so the card shows its
  // path and keep fields. Every write is answered without touching the store.
  await page.route("**/api/settings", async (route) => {
    if (route.request().method() !== "GET") return route.fulfill({ json: { ok: true } });
    const body = await (await route.fetch()).json();
    body.settings.flashZipExportEnabled = true;
    body.settings.flashZipExportPath = "user/backups/unraid-flash/zip-exporte-taeglich";
    body.settings.flashZipExportKeep = 14;
    return route.fulfill({ json: body });
  });
  if (!opts.empty) {
    await page.route("**/api/progress", (route) =>
      route.fulfill({
        headers: { "content-type": "text/event-stream" },
        body: `data: ${JSON.stringify({ key: "flash", phase: "backup", percent: 42.5, active: true })}\n\n`,
      }),
    );
  }
  await page.route("**/api/anomalies/summary", async (route) => {
    const body = await (await route.fetch()).json();
    body.summary = { ...body.summary, enabled: true, ready: true, generation: 7, open: { ...NO_OPEN, critical: 1 } };
    return route.fulfill({ json: body });
  });
  await page.route("**/api/anomalies/items", (route) => route.fulfill({ json: { ok: true, items: [FLASH_ITEM] } }));
  await page.route("**/api/anomalies?*", (route) => route.fulfill({ json: { ok: true, anomalies: [FINDING], nextCursor: "" } }));
  // Same seeding as narrow-viewport.spec.ts: the stored locale is the look,
  // and the server's display prefs must not overwrite it mid-boot.
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.addInitScript(() => window.localStorage.setItem("bv-lang", "de"));
  await page.setViewportSize({ width, height: 800 });
  await page.goto(`/flash${opts.query ?? ""}`);
  await expect(page.getByRole("heading", { level: 1, name: "Flash-Backup" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Backup abbrechen" }).or(page.getByText("Noch keine Flash-Backups"))).toBeVisible();
  await expect(page.getByText("Export-Ordner")).toBeVisible();
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
    .locator("xpath=..")
    .evaluate((root) => getComputedStyle(root).rowGap);
}

async function expectNothingPansOrClips(page: Page): Promise<void> {
  const layout = await page.evaluate(() => {
    const main = document.querySelector("#bv-main");
    const vw = window.innerWidth;
    const clipped = main
      ? [...main.querySelectorAll("button, a, input, select, textarea, [role='switch']")]
          .filter((el) => {
            const r = el.getBoundingClientRect();
            return r.width > 0 && (r.right > vw + 1 || r.left < -1);
          })
          .map((el) => (el.getAttribute("aria-label") ?? el.textContent ?? "").trim().slice(0, 40))
      : null;
    return {
      docPan: document.documentElement.scrollWidth - vw,
      mainPan: main ? main.scrollWidth - main.clientWidth : null,
      clipped,
    };
  });
  expect(layout.mainPan, "#bv-main is missing").not.toBeNull();
  expect(layout.docPan, "the document scrolls horizontally").toBeLessThanOrEqual(1);
  expect(layout.mainPan!, "#bv-main scrolls horizontally").toBeLessThanOrEqual(1);
  expect(layout.clipped, "controls clip the viewport edge").toEqual([]);
  // A backtick in JSX text compiles fine and renders on the page.
  await expect(page.locator("#bv-main")).not.toContainText("`");
}

// Each backup row by its short id, with the id's box and its two actions.
async function rowBoxes(page: Page, id: string) {
  const idText = page.getByText(id.slice(0, 8), { exact: true });
  const row = idText.locator("xpath=..");
  return {
    id: (await idText.boundingBox())!,
    download: (await row.getByRole("button", { name: "Download (.zip)" }).boundingBox())!,
    remove: (await row.getByRole("button", { name: "Löschen" }).boundingBox())!,
  };
}

for (const width of [320, 360]) {
  test(`flash @ ${width}px: both sources fit, and each backup's actions wrap under it`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
    // A finding's restore link picks the flagged backup, which then stands out.
    await bootGerman(page, width, { query: `?restore=${LOCAL[1].id}` });

    expect(await cardGap(page)).toBe("24px");
    await expect(page.getByText("Als Anomalie markiert")).toBeVisible();
    await expectNothingPansOrClips(page);
    for (const { id } of LOCAL) {
      const box = await rowBoxes(page, id);
      expect(box.download.y, `the actions of ${id.slice(0, 8)} share the id's line`).toBeGreaterThanOrEqual(box.id.y + box.id.height);
      expect(box.remove.x + box.remove.width, `Löschen of ${id.slice(0, 8)} passes the edge`).toBeLessThanOrEqual(width);
    }

    await page.getByRole("tab", { name: "Offsite" }).first().click();
    const picker = page.getByRole("combobox", { name: "Offsite-Ziel" });
    await expect(picker).toBeVisible();
    await expect(page.getByText(OFFSITE[0].id.slice(0, 8), { exact: true })).toBeVisible();
    await settle(page);
    await expectNothingPansOrClips(page);
    for (const { id } of OFFSITE) {
      const box = await rowBoxes(page, id);
      expect(box.download.y, `the actions of ${id.slice(0, 8)} share the id's line`).toBeGreaterThanOrEqual(box.id.y + box.id.height);
    }
    const pickerBox = (await picker.boundingBox())!;
    expect(pickerBox.height, "the target picker is shorter than a control").toBeGreaterThanOrEqual(31.5);

    // useConfirm answers with the bottom sheet on a phone, whose buttons have
    // to stay on screen with the long German question above them.
    await page.getByRole("button", { name: "Löschen" }).first().click();
    const dialog = page.getByRole("dialog");
    await expect(dialog.getByText("Dieses Backup löschen?", { exact: false })).toBeVisible();
    // WebKit reports the sheet visible before it slides in, so wait until its
    // answers are on screen and have held still for three frames.
    await dialog.evaluate(async (sheet) => {
      const frame = () => new Promise((resolve) => requestAnimationFrame(resolve));
      let last = "";
      for (let still = 0; still < 3; ) {
        await frame();
        const boxes = [...sheet.querySelectorAll("button")].map((b) => b.getBoundingClientRect());
        const now = JSON.stringify(boxes.map((r) => [r.top, r.bottom]));
        const onScreen = boxes.every((r) => r.top >= 0 && r.bottom <= innerHeight);
        still = now === last && onScreen ? still + 1 : 0;
        last = now;
      }
    });
    for (const name of ["Abbrechen", "Löschen"]) {
      const box = (await dialog.getByRole("button", { name, exact: true }).boundingBox())!;
      expect(box.x, `${name} starts off screen`).toBeGreaterThanOrEqual(0);
      expect(box.x + box.width, `${name} passes the edge`).toBeLessThanOrEqual(width);
      expect(box.y + box.height, `${name} sits below the fold`).toBeLessThanOrEqual(800);
    }
  });
}

test("flash on a phone with no backups and a restore link to a lost one", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
  await bootGerman(page, 320, { empty: true, query: `?restore=${hex("9f")}&at=1789787700` });

  await expect(page.getByText("Noch keine Flash-Backups. Oben eines starten.")).toBeVisible();
  await expect(page.getByRole("status").filter({ hasText: "2026" })).toBeVisible();
  expect(await cardGap(page)).toBe("24px");
  await expectNothingPansOrClips(page);
});

test("flash on the desktop keeps the 40px rhythm, one line per backup and the compact picker", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "desktop-only");
  await bootGerman(page, testInfo.project.use.viewport!.width);

  expect(await cardGap(page)).toBe("40px");
  for (const { id } of LOCAL) {
    const box = await rowBoxes(page, id);
    const middle = (b: { y: number; height: number }) => Math.round(b.y + b.height / 2);
    expect(middle(box.download), `the actions of ${id.slice(0, 8)} left the id's line`).toBe(middle(box.id));
    expect(middle(box.remove)).toBe(middle(box.id));
  }

  await page.getByRole("tab", { name: "Offsite" }).first().click();
  const picker = page.getByRole("combobox", { name: "Offsite-Ziel" });
  await expect(picker).toBeVisible();
  expect(Math.round((await picker.boundingBox())!.height)).toBe(24);
});
