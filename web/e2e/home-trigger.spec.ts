// ---------------------------------------------------------------------------
// Home e2e — the real-binary contract for the phone Home.
//
// The dom twins prove the block semantics in jsdom (and stay on the desktop
// page — jsdom matchMedia answers desktop); this spec proves the four
// glanceable blocks, the thumb-zone trigger and the backup trace on the real
// binary in real device emulation. Five scenarios:
//
//   1. Home is glanceable at 360x800: identity -> next run -> recent runs ->
//      repo health in the contracted order, the off-site copy-age chip in the
//      repo-health card, and the New-backup trigger visible in the thumb zone
//      WITHOUT scrolling (the sticky bar holds it on screen),
//   2. the trigger tap presents the fail-toned confirm sheet carrying the
//      consequence copy, and Cancel keeps the server untouched,
//   3. Confirm fires POST /api/backup-everything and the correlated
//      "everything" run deep-links into the RunDetailSheet,
//   4. a recent-run row tap opens THAT run's detail sheet (component-local
//      host, no route),
//   5. desktop-1280/768 (>=48rem): NO mobile blocks, NO sticky trigger — the
//      >=48rem leakage guard for the Home surface.
//
// Harness honesty — what is mocked and why: the e2e webServer is the real
// bombvault binary over a wiped fresh DB, but the harness has no restic repo
// and no Docker, so the read models the Home blocks render are fulfilled at
// the Playwright route layer — Go JSON shapes field-for-field (api.ts), the
// same route-layer staging discipline the rest of the suite uses. The one
// WRITE the backup trace needs (POST /api/backup-everything) is fulfilled the
// same way: the harness DB stays untouched by a spec, and the run the watch
// correlates is staged into GET /api/runs only AFTER the POST (the
// baseline-id correlation contract — a run present before the fire must never
// correlate). The SPA, its fetches, the binary and every route shape are real.
// ---------------------------------------------------------------------------
import { expect, test, type Locator, type Page } from "@playwright/test";

// The two device projects from playwright.config.ts. Everything else is a
// desktop project.
const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

// --- staged read models (Go JSON shapes, api.ts) ----------------------------

const NOW = () => Math.floor(Date.now() / 1000);

/** A full DomainStatus record — every field the interface carries, so no
 *  rendering path can trip over an undefined the real server would never
 *  send. Overrides layer the per-scenario facts on top. */
function domainStatus(overrides: Record<string, unknown>) {
  return {
    domain: "containers",
    enabled: true,
    schedule: "every day",
    coveredBy: "",
    lastSuccess: NOW() - 3600,
    periodSeconds: 86400,
    status: "ok",
    lastVerified: 0,
    lastVerifiedOK: false,
    verifiedDetail: "",
    drillDetail: "",
    offsiteConfigured: false,
    offsiteImmutable: false,
    lastTamperAt: 0,
    lastTamperOK: false,
    lastReplicationAt: 0,
    lastReplicationOK: false,
    lastDrDrillAt: 0,
    lastDrDrillOK: false,
    lastOffsiteSubsetAt: 0,
    lastOffsiteSubsetOK: false,
    offsiteDrillScheduled: false,
    protection: "green",
    tamperState: "",
    replicationState: "",
    drillState: "",
    encryptionOn: true,
    pruneStrategySet: true,
    ...overrides,
  };
}

// Two protected domains: containers carries the off-site facts (the repo-
// health card's ↗ copy-age chip reads the newest lastReplicationAt), files is
// plain-local. vms/flash disabled. Worst RPO across the enabled set is "ok",
// so the repo-health four-status line reads "OK / Up to date".
function stagedDomains() {
  return [
    domainStatus({
      domain: "containers",
      offsiteConfigured: true,
      offsiteImmutable: true,
      lastReplicationAt: NOW() - 2 * 86400,
      lastReplicationOK: true,
      replicationState: "ok",
    }),
    domainStatus({ domain: "files", schedule: "every 3 days", periodSeconds: 3 * 86400 }),
    domainStatus({ domain: "vms", enabled: false, schedule: "", status: "off", protection: "" }),
    domainStatus({ domain: "flash", enabled: false, schedule: "", status: "off", protection: "" }),
  ];
}

/** A Run record (api.ts:341) — field-for-field. */
function run(overrides: Record<string, unknown>) {
  return {
    id: "a".repeat(32),
    targetId: "plex",
    kind: "backup",
    status: "success",
    startedAt: NOW() - 3600,
    finishedAt: NOW() - 3540,
    snapshotId: "f1e2d3c4b5a697887766554433322211",
    bytes: 5_000_000_000,
    error: "",
    acknowledged: false,
    target: "plex",
    domain: "container",
    ...overrides,
  };
}

// The served baseline: the two recent runs the Recent-runs card lists BEFORE
// anything fires. Newest first, as listRuns returns them.
const BASE_RUNS = () => [
  run({ id: "a".repeat(32) }),
  run({
    id: "b".repeat(32),
    targetId: "f".repeat(32),
    target: "photos",
    domain: "files",
    status: "failed",
    startedAt: NOW() - 7200,
    finishedAt: NOW() - 7100,
    snapshotId: "",
    bytes: 0,
  }),
];

// The correlated pass: the "Backup Everything" PARENT run (domain "everything",
// target "Backup Everything" — store.EverythingTargetID's human name). It only
// ever appears in /api/runs after the POST armed the route (see stageHome).
const EVERYTHING_RUN = () =>
  run({
    id: "c".repeat(32),
    targetId: "c".repeat(32),
    target: "Backup Everything",
    domain: "everything",
    status: "running",
    startedAt: NOW(),
    finishedAt: null,
    snapshotId: "",
    bytes: 0,
  });

/** Route-level staging of the Home read models + the boot-look cut
 *  (display-prefs abort, so this spec keeps the harness default English labels
 *  regardless of worker order). The backup POST is staged here too because the
 *  Cancel scenario must prove it never fires: `fired` flips only on a real
 *  request, and `armed` makes the correlated run appear in /api/runs only
 *  after that request — the baseline-id correlation contract. */
async function stageHome(page: Page): Promise<{ fired: () => boolean }> {
  let postFired = false;
  let armed = false;

  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.route("**/api/status", (route) => route.fulfill({ json: { ok: true, domains: stagedDomains() } }));
  // One scheduled entry, a container-domain backup ~2h out — the next-run
  // card's name + "in 2h" chip derivation. (nextBackupFireAt takes the first
  // job==="backup" entry whose domain is on.)
  await page.route("**/api/schedule/next", (route) =>
    route.fulfill({
      json: { ok: true, runs: [{ job: "backup", domain: "containers", next: new Date(Date.now() + 7_200_000).toISOString() }] },
    })
  );
  await page.route("**/api/runs", (route) =>
    route.fulfill({ json: { ok: true, runs: armed ? [EVERYTHING_RUN(), ...BASE_RUNS()] : BASE_RUNS() } })
  );
  // The repo-health card's four /api/stats reads (same endpoint the desktop
  // Storage card uses): one latest sample per domain, sized so the summed
  // line reads 3.8 TB · Dedup 0.3x · 12 Snapshots.
  await page.route("**/api/stats*", (route) =>
    route.fulfill({
      json: {
        ok: true,
        latest: { domain: "containers", source: "local", at: NOW(), rawSize: 1_050_000_000_000, restoreSize: 260_000_000_000, snapshots: 3 },
      },
    })
  );
  await page.route("**/api/backup-everything", (route) => {
    postFired = true;
    armed = true;
    return route.fulfill({ json: { ok: true, started: true } });
  });
  return { fired: () => postFired };
}

// --- shared locators ---------------------------------------------------------

/** The thumb-zone trigger: the Button engine's accent tone on the caller's
 *  full-width stage — the same signature selector the desktop leakage guard
 *  uses, now asserted PRESENT on the phone. */
function newBackupTrigger(page: Page) {
  return page.locator("button.w-full.bg-accent");
}

/** The section card that carries the named mobile section label. */
function sectionWith(page: Page, label: string) {
  return page.locator("section").filter({ has: page.getByRole("heading", { name: label, exact: true }) });
}

async function yOf(locator: Locator): Promise<number> {
  const box = await locator.boundingBox();
  expect(box).not.toBeNull();
  return box!.y;
}

// ---------------------------------------------------------------------------

test("Home is glanceable: blocks in order, offsite chip, trigger in the thumb zone", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the four-block Home is the phone contract");
  await stageHome(page);
  await page.goto("/");

  // Identity: the h1 and the app wordmark sit at the top of the column.
  await expect(page.getByRole("heading", { level: 1, name: "Dashboard" })).toBeVisible();
  await expect(page.getByText("BombVault")).toBeVisible();

  // The four blocks in the contracted order (the heading role only matches
  // the mobile section labels — the desktop grid is unmounted below the
  // breakpoint, the isDesktop gate, and so out of the accessibility tree).
  const nextY = await yOf(page.getByRole("heading", { name: "Next backup", exact: true }));
  const runsY = await yOf(page.getByRole("heading", { name: "Recent Runs", exact: true }));
  const storageY = await yOf(page.getByRole("heading", { name: "Storage", exact: true }));
  expect(nextY).toBeLessThan(runsY);
  expect(runsY).toBeLessThan(storageY);

  // Next-run card: the scheduled domain's name and the live "in ..." chip.
  const nextCard = sectionWith(page, "Next backup");
  await expect(nextCard.getByText("Containers")).toBeVisible();
  await expect(nextCard.getByText(/^in /)).toBeVisible();

  // Recent-runs card: the two served runs, each row a >=44px labelled target.
  await expect(page.getByRole("button", { name: "OK · Backup plex" })).toBeVisible();
  await expect(page.getByRole("button", { name: "FAIL · Backup photos" })).toBeVisible();

  // Repo-health card: the four-status line (badge + label, never color alone)
  // and the off-site copy-age chip in the offsite DOMAIN color treatment —
  // the "↗ <relative age>" line language.
  const storageCard = sectionWith(page, "Storage");
  await expect(storageCard.getByText("OK", { exact: true })).toBeVisible();
  await expect(storageCard.getByText("Up to date")).toBeVisible();
  await expect(storageCard.getByText(/↗/)).toBeVisible();
  await expect(storageCard.getByText(/Dedup 0\.\dx/)).toBeVisible();

  // The trigger sits in the thumb zone WITHOUT scrolling: the sticky bar
  // pins to the scrollport bottom, so at 360x800 / 390x844 it is fully
  // inside the viewport and in its lower half.
  const trigger = newBackupTrigger(page);
  await expect(trigger).toBeVisible();
  const box = (await trigger.boundingBox())!;
  const vp = page.viewportSize()!;
  expect(box.y).toBeGreaterThanOrEqual(0);
  expect(box.y + box.height).toBeLessThanOrEqual(vp.height);
  expect(box.y).toBeGreaterThan(vp.height / 2);
});

test("trigger tap presents the consequence confirm; Cancel keeps the server untouched", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the ConfirmSheet face of the confirm is the phone contract");
  const stage = await stageHome(page);
  await page.goto("/");

  await newBackupTrigger(page).tap();

  // The fail-toned ConfirmSheet: titled by the shared confirm dialog title,
  // body carrying the consequence copy (containers stop and restart — the
  // accident mitigation is this copy being UNSKIPPABLE on the path to the
  // trigger's effect).
  const confirm = page.getByRole("dialog", { name: "Confirm" });
  await expect(confirm).toBeVisible();
  await expect(confirm.getByText(/Containers are stopped and restarted/)).toBeVisible();

  // Cancel is the safe exit: the sheet closes, no request ever left.
  await confirm.getByRole("button", { name: "Cancel" }).tap();
  await expect(confirm).toHaveCount(0);
  await page.waitForTimeout(500);
  expect(stage.fired()).toBe(false);
});

test("Confirm fires the everything pass; the correlated run deep-links into the run sheet", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone backup trace");
  await stageHome(page);
  await page.goto("/");

  await newBackupTrigger(page).tap();
  const confirm = page.getByRole("dialog", { name: "Confirm" });
  await expect(confirm).toBeVisible();

  // The destructive control carries the trigger's own verb; one tap starts
  // the pass and arms the correlated run into /api/runs.
  await confirm.getByRole("button", { name: "New backup" }).tap();

  // The watch correlates the new "everything" run by baseline-id (never a
  // client clock) and the page deep-links it into the component-local
  // RunDetailSheet — the live run's own sheet, titled by the shared run
  // vocabulary. Poll belt ticks at 2s; allow a few cycles.
  const sheet = page.getByRole("dialog", { name: "Backup · Backup Everything" });
  await expect(sheet).toBeVisible({ timeout: 15_000 });
  // The live statement: the correlated run is in flight. The "everything"
  // parent streams no SSE key of its own (RunDetailSheet's own domain-honesty
  // note), so the sheet's live face is the status Badge + running log tail —
  // both carry the shared statusLabel word; .first() because either matching
  // satisfies the claim.
  await expect(sheet.getByText("Running").first()).toBeVisible();
});

test("a recent-run row tap opens that run's detail sheet", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: tappable run rows are the phone contract");
  await stageHome(page);
  await page.goto("/");

  await page.getByRole("button", { name: "OK · Backup plex" }).tap();
  await expect(page.getByRole("dialog", { name: "Backup · plex" })).toBeVisible();
});

test("desktop Home: no mobile blocks, no sticky trigger", async ({ page }, testInfo) => {
  test.skip(
    testInfo.project.name !== "desktop-1280" && testInfo.project.name !== "desktop-768",
    "desktop-only: the >=48rem leakage guard",
  );
  // No staging: the real fresh-DB answers are exactly what the desktop page
  // has always rendered with (the guard is about DOM surface, not data).
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.goto("/");

  await expect(page.getByRole("heading", { level: 1, name: "Dashboard" })).toBeVisible();
  // The phone-only surfaces exist NOWHERE in the desktop DOM: the mobile
  // section labels, the labelled run rows, the trigger and its exact
  // full-width-accent signature.
  await expect(page.getByRole("heading", { name: "Recent Runs", exact: true })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "OK · Backup plex" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "New backup" })).toHaveCount(0);
  await expect(newBackupTrigger(page)).toHaveCount(0);
});
