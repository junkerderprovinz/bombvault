// The Flash page at phone width, with a timeline of backups and their copies
// at two off-site targets, a running backup, an open finding and the ZIP
// export staged at the route layer (a fresh harness database has none of
// them). On the phones: the 24px card rhythm, no pan, no control past the edge
// at home or at a target, each backup's actions wrapping under its id and
// time, and the delete question as a sheet that fits. On the desktop: the 40px
// rhythm and each backup's actions on one line. German, because its labels
// run longest.
import { expect, test, type Page } from "@playwright/test";

const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

// A settings read the page makes as the test ends would otherwise fail it
// from inside a route handler whose response is already gone.
test.afterEach(async ({ page }) => {
  await page.unrouteAll({ behavior: "ignoreErrors" });
});

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

const place = (id: string, label: string) => ({
  place: id === "local" ? "local" : `offsite:${id}`,
  label,
  kind: id === "local" ? "home" : "target",
  remote: id !== "local",
  enabled: true,
  appendOnly: false,
  state: "read",
});

// The two newest backups reached the first target as well.
function timeline(empty: boolean) {
  return {
    ok: true,
    places: empty ? [place("local", "")] : [place("local", ""), place("primary", "Hetzner Storage Box Falkenstein"), place("second", "Backblaze B2 Amsterdam")],
    rows: empty
      ? []
      : LOCAL.map((s) => {
          const copy = OFFSITE.find((o) => o.original === s.id);
          return {
            key: s.id,
            time: s.time,
            places: [
              { place: "local", snapshotIds: [s.id], tags: s.tags },
              ...(copy ? [{ place: "offsite:primary", snapshotIds: [copy.id], tags: copy.tags }] : []),
            ],
          };
        }),
  };
}

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
  await page.route("**/api/items/flash/flash/timeline*", (route) => route.fulfill({ json: timeline(!!opts.empty) }));
  await page.route("**/api/items/flash/flash/timeline/*/delete*", (route) =>
    route.fulfill({
      json: {
        ok: true,
        delete: [{ place: "local", label: "", snapshotIds: [LOCAL[0].id] }],
        others: [{ place: "offsite:primary", label: "Hetzner Storage Box Falkenstein", state: "holds" }],
      },
    }),
  );
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
  await expect(page.getByRole("button", { name: "Backup abbrechen" }).or(page.getByText("Keine Backups gefunden"))).toBeVisible();
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
    .locator("xpath=../../..")
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

// Each backup row by its short id, with the id's box and its two actions. One
// read covers all rows, because WebKit on a busy machine spends most of a
// second on each round trip.
async function rowBoxes(page: Page, ids: string[]) {
  return page.evaluate(
    (shorts) =>
      shorts.map((short) => {
        const idText = [...document.querySelectorAll("#bv-main span")].find((el) => el.textContent === short);
        if (!idText) throw new Error(`no backup row ${short}`);
        const row = idText.parentElement!.parentElement!;
        const box = (el: Element | undefined) => {
          if (!el) throw new Error(`backup row ${short} lacks an action`);
          const r = el.getBoundingClientRect();
          return { x: r.x, y: r.y, width: r.width, height: r.height };
        };
        const button = (name: string) => box([...row.querySelectorAll("button")].find((b) => b.textContent?.trim() === name));
        return { short, id: box(idText), download: button("Download (.zip)"), remove: button("Löschen") };
      }),
    ids.map((id) => id.slice(0, 8)),
  );
}

for (const width of [320, 360]) {
  test(`flash @ ${width}px: both sources fit, and each backup's actions wrap under it`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
    test.slow(testInfo.project.name === "mobile-iphone", "WebKit runs this walk through both sources near the 30s budget when the machine is busy");
    // A finding's restore link picks the flagged backup, which then stands out.
    await bootGerman(page, width, { query: `?restore=${LOCAL[1].id}` });

    expect(await cardGap(page)).toBe("24px");
    await expect(page.getByText("Als Anomalie markiert")).toBeVisible();
    await expectNothingPansOrClips(page);
    for (const box of await rowBoxes(page, LOCAL.map((s) => s.id))) {
      expect(box.download.y, `the actions of ${box.short} share the id's line`).toBeGreaterThanOrEqual(box.id.y + box.id.height);
      expect(box.remove.x + box.remove.width, `Löschen of ${box.short} passes the edge`).toBeLessThanOrEqual(width);
    }

    // The copies at a target, picked on each backup's own place switch.
    for (const copy of OFFSITE) {
      const row = page.getByText(copy.original!.slice(0, 8), { exact: true }).locator("xpath=../..");
      await row.getByRole("tab", { name: "Hetzner Storage Box Falkenstein" }).click();
      await expect(page.getByText(copy.id.slice(0, 8), { exact: true })).toBeVisible();
    }
    await settle(page);
    await expectNothingPansOrClips(page);
    for (const box of await rowBoxes(page, OFFSITE.map((s) => s.id))) {
      expect(box.download.y, `the actions of ${box.short} share the id's line`).toBeGreaterThanOrEqual(box.id.y + box.id.height);
    }

    // useConfirm answers with the bottom sheet on a phone, whose buttons have
    // to stay on screen with the long German question above them.
    await page.getByRole("button", { name: "Löschen", exact: true }).last().click();
    const dialog = page.getByRole("dialog");
    await expect(dialog.getByText("Diese Sicherung hier löschen", { exact: false })).toBeVisible();
    // WebKit reports the sheet visible before it slides in, so wait until its
    // answers are on screen and have held still for three frames.
    const answers = await dialog.evaluate(async (sheet) => {
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
      return [...sheet.querySelectorAll("button")].map((b) => {
        const r = b.getBoundingClientRect();
        return { name: b.textContent?.trim(), x: r.x, y: r.y, width: r.width, height: r.height };
      });
    });
    for (const name of ["Abbrechen", "Löschen"]) {
      const box = answers.find((b) => b.name === name);
      expect(box, `the sheet has no ${name}`).toBeDefined();
      expect(box!.x, `${name} starts off screen`).toBeGreaterThanOrEqual(0);
      expect(box!.x + box!.width, `${name} passes the edge`).toBeLessThanOrEqual(width);
      expect(box!.y + box!.height, `${name} sits below the fold`).toBeLessThanOrEqual(800);
    }
  });
}

test("flash on a phone with no backups and a restore link to a lost one", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
  await bootGerman(page, 320, { empty: true, query: `?restore=${hex("9f")}&at=1789787700` });

  await expect(page.getByText("Keine Backups gefunden")).toBeVisible();
  await expect(page.getByRole("status").filter({ hasText: "2026" })).toBeVisible();
  expect(await cardGap(page)).toBe("24px");
  await expectNothingPansOrClips(page);
});

test("flash on the desktop keeps the 40px rhythm and each backup's actions on one line", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "desktop-only");
  await bootGerman(page, testInfo.project.use.viewport!.width);

  expect(await cardGap(page)).toBe("40px");
  const middle = (b: { y: number; height: number }) => Math.round(b.y + b.height / 2);
  for (const box of await rowBoxes(page, LOCAL.map((s) => s.id))) {
    expect(middle(box.remove), `Löschen of ${box.short} left the download's line`).toBe(middle(box.download));
  }
});
