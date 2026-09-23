// The /vms phone block on the real binary in device emulation, and the
// desktop page beside it. The scenarios: the card list with its summary
// line, search and the 20/40 load-more window; a card's trigger deep-linking
// its run into the sheet, which stays closed once dismissed; a stored
// per-VM override staying off the card, since only Settings shows it and only
// while the scheduler would run it; the gated-off block showing its hint and
// the settings link while Discover stays in the header; and the desktop
// carrying none of the phone chrome but always its Discover.
//
// The harness has no libvirt, so a fresh DB never holds a VM. The VM,
// settings, schedule and runs responses are staged at the route layer in the
// Go JSON shapes (api.ts); the SPA, its requests and the binary are real.
// The display-prefs abort keeps the English labels whatever order the
// workers boot in.
import { expect, test, type Page } from "@playwright/test";

// The two device projects from playwright.config.ts; everything else is a
// desktop project (desktop-untouched.spec.ts's branching pattern).
const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);
const DESKTOP_PROJECTS = new Set(["desktop-1280", "desktop-768"]);

// The assertions read English labels and US formatting, so the browser's
// locale is pinned to match.
test.use({ locale: "en-US" });

// --- staged VM domain (Go JSON shapes, api.ts) ------------------------------

function vmPayload(i: number, overrides: Record<string, unknown> = {}) {
  return {
    name: `vm-${String(i).padStart(2, "0")}`,
    libvirtName: `id-${String(i).padStart(2, "0")}`,
    state: "running",
    method: "graceful",
    includeInSchedule: true,
    lastBackup: null,
    lastBackupStarted: null,
    ...overrides,
  };
}

// GET /api/settings — the settings object mirrors the Settings interface
// field-for-field (the same discipline the desktop dom tests' typed fixtures
// follow); callers override only the fields a scenario needs.
function settingsBody(settingsOverrides: Record<string, unknown> = {}) {
  return {
    ok: true,
    hostMountRoot: "/mnt/user",
    platform: "generic",
    settings: {
      encryptionEnabled: false,
      containersEnabled: true,
      vmsEnabled: true,
      flashEnabled: true,
      configEnabled: true,
      filesEnabled: true,
      receiverEnabled: false,
      fleetEnabled: false,
      containersPath: "/mnt/user/backups/containers",
      vmsPath: "/mnt/user/backups/vms",
      flashPath: "/mnt/user/backups/flash",
      configPath: "/mnt/user/backups/config",
      filesPath: "/mnt/user/backups/files",
      restoreFolder: "/mnt/user/backups/restore",
      flashZipExportEnabled: false,
      flashZipExportPath: "",
      flashZipExportKeep: 0,
      exportEncryptEnabled: false,
      exportAgeRecipients: "",
      containersOffsite: "",
      vmsOffsite: "",
      flashOffsite: "",
      configOffsite: "",
      filesOffsite: "",
      containersOffsiteSchedule: "off",
      vmsOffsiteSchedule: "off",
      flashOffsiteSchedule: "off",
      configOffsiteSchedule: "off",
      filesOffsiteSchedule: "off",
      containersSchedule: "off",
      vmsSchedule: "off",
      flashSchedule: "off",
      configSchedule: "off",
      filesSchedule: "off",
      everythingSchedule: "off",
      everythingPreHook: "",
      everythingPostHook: "",
      everythingPreHookSet: false,
      everythingPostHookSet: false,
      defaultLanguage: "en",
      retentionKeepLast: 7,
      retentionKeepDaily: 7,
      retentionKeepWeekly: 4,
      retentionKeepMonthly: 6,
      offsiteRetentionKeepLast: 7,
      offsiteRetentionKeepDaily: 7,
      offsiteRetentionKeepWeekly: 4,
      offsiteRetentionKeepMonthly: 6,
      offsiteLimitUpload: 0,
      backupCores: 0,
      offsiteLimitDownload: 0,
      metricsEnabled: false,
      metricsToken: "",
      metricsTokenSet: false,
      widgetToken: "",
      widgetTokenSet: false,
      drillsEnabled: true,
      offsiteDrillsEnabled: true,
      drillsSchedule: "weekly Sun 04:30",
      drillsSubsetPct: 10,
      recoveryKitAck: true,
      containersOffsiteImmutable: false,
      vmsOffsiteImmutable: false,
      flashOffsiteImmutable: false,
      configOffsiteImmutable: false,
      filesOffsiteImmutable: false,
      offsiteGrowthBudgetGB: 0,
      tamperTestSchedule: "weekly Sun 05:30",
      drDrillTarget: "",
      drDrillTargetVm: "",
      instanceName: "e2e-harness",
      fleetToken: "",
      fleetTokenSet: false,
      pruneImageAfterUpdate: false,
      resticCacheMaxMB: 4096,
      digestEnabled: false,
      digestSchedule: "weekly Mon 08:00",
      catchUpMissed: true,
      watchdogEnabled: true,
      registryAuths: [],
      restartHealthWait: true,
      restartHealthTimeoutSec: 120,
      reconcileUnraidUpdateStatus: true,
      ...settingsOverrides,
    },
  };
}

/** Route-level staging of the VM domain. Registered list-first, then the
 *  specific write routes (Playwright consults the LAST matching handler
 *  first). The PATCHes are fulfilled so the harness DB stays untouched by a
 *  spec, exactly like the container staging's PATCHes. */
async function stageVmsDomain(
  page: Page,
  vms: ReturnType<typeof vmPayload>[],
  opts: { settings?: Record<string, unknown> } = {},
) {
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.route("**/api/vms", (route) => route.fulfill({ json: { ok: true, vms } }));
  await page.route("**/api/settings", (route) =>
    route.fulfill({ json: settingsBody(opts.settings) }),
  );
  await page.route("**/api/schedule/next", (route) =>
    route.fulfill({ json: { ok: true, runs: [] } }),
  );
  await page.route(/\/api\/vms\/[^/]+$/, (route) => route.fulfill({ json: { ok: true } }));
  await page.route("**/api/vms/*/backup", (route) =>
    route.fulfill({ json: { ok: true, started: true } }),
  );
}

// ---------------------------------------------------------------------------

test("mobile /vms: card list, search filter and the 20/40 load-more window", async ({ page }, testInfo) => {
  test.skip(
    !MOBILE_PROJECTS.has(testInfo.project.name),
    "mobile-only: the phone card list is this spec's surface",
  );
  await stageVmsDomain(page, Array.from({ length: 40 }, (_, i) => vmPayload(i)));
  await page.goto("/vms");

  // The summary counts line: derived from the same list payload the desktop
  // reads (no new endpoint), all 40 live and scheduled.
  await expect(page.getByText("40 VMs · 40 Scheduled")).toBeVisible();
  // The ONE toolbar: lifted search state + chip filters.
  const searchBox = page.getByPlaceholder("Search VMs…").filter({ visible: true });
  await expect(searchBox).toBeVisible();

  // One compact row per VM, its accessible name leading with the VM name and
  // every control in the detail the row opens. The desktop is not mounted at
  // this width, so the rows count the window.
  const cards = page.getByRole("button", { name: /^vm-\d{2}/ }).filter({ visible: true });
  await expect(cards).toHaveCount(20);
  await expect(page.getByText("vm-39", { exact: true }).filter({ visible: true })).toHaveCount(0);
  const loadMore = page.getByRole("button", { name: "Load more" });
  await expect(loadMore).toBeVisible();
  await loadMore.tap();
  await expect(cards).toHaveCount(40);
  await expect(page.getByText("vm-39", { exact: true }).filter({ visible: true })).toBeVisible();
  // Exhausted: hasMore is the ONLY signal the button may gate on — no rows
  // beyond the window, no button.
  await expect(loadMore).toHaveCount(0);

  // Search narrows the same windowed list…
  await searchBox.fill("vm-00");
  await expect(cards).toHaveCount(1);
  await expect(loadMore).toHaveCount(0);
  // …and clearing it resets the window (useLoadMore's reset-on-identity).
  await searchBox.fill("");
  await expect(cards).toHaveCount(20);
  await expect(loadMore).toBeVisible();
});

test("mobile /vms: trigger deep-links the correlated run and a dismissed sheet is never re-opened", async ({
  page,
}, testInfo) => {
  test.skip(
    !MOBILE_PROJECTS.has(testInfo.project.name),
    "mobile-only: the deep-link + latch is the phone contract",
  );
  await stageVmsDomain(page, [vmPayload(0)]);

  // Flips the staged runs list as the story advances — [] is the pre-fire
  // baseline, running is the correlated watch, done is the terminal record
  // (run-detail-visibility.spec.ts's phase machine).
  const RUN_ID = "5f4e3d2c1b0a99887766554433221100";
  const startedAt = Math.floor(Date.now() / 1000) - 60;
  let runsPhase: "empty" | "running" | "done" = "empty";
  await page.route("**/api/runs", (route) =>
    route.fulfill({
      json: {
        ok: true,
        runs:
          runsPhase === "empty"
            ? []
            : [
                {
                  id: RUN_ID,
                  targetId: "aaaa0000bbbb1111cccc2222dddd3333",
                  kind: "backup",
                  status: runsPhase === "running" ? "running" : "success",
                  startedAt,
                  finishedAt: runsPhase === "done" ? startedAt + 45 : null,
                  snapshotId: runsPhase === "done" ? RUN_ID : "",
                  bytes: runsPhase === "done" ? 5_000_000_000 : 0,
                  error: "",
                  acknowledged: false,
                  // The backend records the run's target as the identifier the
                  // action sent — the RAW libvirt name (VMs.test.tsx's whole
                  // point), never the display name — and matchRun correlates
                  // on exactly that (r.target === libvirtName).
                  target: "id-00",
                  domain: "vm",
                },
              ],
      },
    }),
  );

  await page.goto("/vms");
  // The row opens the detail; the trigger lives in the detail, as on the
  // Containers page.
  await page.getByRole("button", { name: /^vm-\d{2}/ }).filter({ visible: true }).tap();
  const trigger = page.getByRole("button", { name: "Back up now" }).filter({ visible: true });
  await expect(trigger).toBeVisible();

  // fire(): the pre-fire runs baseline lands BEFORE the POST (baseline-id
  // correlation, never a client clock).
  const posted = page.waitForRequest(
    (r) => r.method() === "POST" && /\/api\/vms\/id-00\/backup$/.test(r.url()),
  );
  await trigger.tap();
  await posted;
  runsPhase = "running";

  // The watch's first poll correlates the NEW run id → the component-local
  // RunDetailSheet stacks in over the list (kind + target title grammar —
  // the target text is the run's recorded identifier, the libvirt name).
  const sheet = page.getByRole("dialog", { name: "Backup · id-00" });
  await expect(sheet).toBeVisible();

  // Dismiss. Later polls of the SAME watch still refresh the record — the
  // latch must keep the sheet closed through them. (Scoped to the sheet: the
  // close affordance is the sheet's own, never a page-level query.)
  await sheet.getByRole("button", { name: "Close" }).tap();
  await expect(sheet).toHaveCount(0);
  runsPhase = "done"; // the run finishes behind the closed sheet

  // The terminal poll provably landed (the button's success toast carries the
  // recorded snapshot id's mono slice) — yet the sheet stays closed.
  await expect(page.getByText(`Done · ${RUN_ID.slice(0, 8)}`)).toBeVisible();
  await expect(page.getByRole("dialog", { name: "Backup · id-00" })).toHaveCount(0);
});

test("mobile /vms: a stored override stays off the card", async ({ page }, testInfo) => {
  test.skip(
    !MOBILE_PROJECTS.has(testInfo.project.name),
    "mobile-only: the phone card is this test's surface",
  );
  // The scheduler only runs a per-VM override while per-item schedules are on
  // and the VM is included, and Settings is where it is edited under exactly
  // those conditions. A card that showed it would paint a cadence that never
  // fires as a live schedule, so none of the three cards may carry one.
  await stageVmsDomain(
    page,
    [
      vmPayload(0, { name: "vm-on", libvirtName: "id-on", scheduleCadence: "daily 03:00" }),
      vmPayload(1, { name: "vm-excluded", libvirtName: "id-excluded", scheduleCadence: "daily 03:00", includeInSchedule: false }),
      vmPayload(2, { name: "vm-gone", libvirtName: "id-gone", scheduleCadence: "daily 03:00", state: "not-installed" }),
    ],
    { settings: { perItemSchedules: true } },
  );
  await page.goto("/vms");

  await expect(page.getByText("vm-on", { exact: true }).filter({ visible: true })).toBeVisible();
  await expect(page.getByText("vm-gone", { exact: true }).filter({ visible: true })).toBeVisible();
  await expect(page.getByText("Schedule override:")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Set override" })).toHaveCount(0);
  // The removed VM only logs a skip when its run comes, so it is not counted.
  await expect(page.getByText("2 VMs · 1 Scheduled")).toBeVisible();
});

test("mobile /vms: a removed VM offers no selection", async ({ page }, testInfo) => {
  test.skip(
    !MOBILE_PROJECTS.has(testInfo.project.name),
    "mobile-only: the phone rows are this test's surface",
  );
  // A bulk backup of a VM the host no longer defines parks a run for hours,
  // which is why the desktop gives removed rows no checkbox either.
  await stageVmsDomain(page, [
    vmPayload(0),
    vmPayload(1, { name: "vm-gone", libvirtName: "id-gone", state: "not-installed", lastBackup: 1_700_000_000 }),
  ]);
  await page.goto("/vms");

  await expect(page.getByRole("checkbox", { name: "Select vm-00" })).toBeVisible();
  await expect(page.getByText("vm-gone", { exact: true }).filter({ visible: true })).toBeVisible();
  await expect(page.getByRole("checkbox", { name: "Select vm-gone" })).toHaveCount(0);
});

test("mobile /vms: the summary row and its controls fit a 320px phone", async ({ page }, testInfo) => {
  test.skip(
    !MOBILE_PROJECTS.has(testInfo.project.name),
    "mobile-only: the phone summary row is this test's surface",
  );
  await page.setViewportSize({ width: 320, height: 800 });
  await stageVmsDomain(page, [vmPayload(0), vmPayload(1)]);
  await page.goto("/vms");
  await expect(page.getByText("2 VMs · 2 Scheduled")).toBeVisible();
  await expect(page.getByRole("button", { name: /Exclude all/ }).filter({ visible: true })).toBeVisible();

  const overflow = await page.evaluate(() => {
    const main = document.getElementById("bv-main")!;
    return main.scrollWidth - main.clientWidth;
  });
  expect(overflow).toBe(0);
});

test("mobile /vms: an open detail hides the list's selection bar", async ({ page }, testInfo) => {
  test.skip(
    !MOBILE_PROJECTS.has(testInfo.project.name),
    "mobile-only: the phone detail is this test's surface",
  );
  await stageVmsDomain(page, [vmPayload(0), vmPayload(1)]);
  await page.goto("/vms");

  await page.getByRole("checkbox", { name: "Select vm-00" }).tap();
  const bulkBackup = page.getByRole("button", { name: "Back up selected" }).filter({ visible: true });
  await expect(bulkBackup).toBeVisible();

  await page.getByRole("button", { name: /^vm-01/ }).filter({ visible: true }).tap();
  await expect(page.getByRole("heading", { level: 2, name: "vm-01" })).toBeVisible();
  await expect(bulkBackup).toHaveCount(0);
});

test("mobile /vms: the detail stays open when a backup takes its VM out of the filter", async ({
  page,
}, testInfo) => {
  test.skip(
    !MOBILE_PROJECTS.has(testInfo.project.name),
    "mobile-only: the phone detail is this test's surface",
  );
  // The list shows only VMs that were never backed up, so the first backup
  // takes this one out of it while its detail is open.
  await page.addInitScript(() => localStorage.setItem("bv-vms-backup-filter", "neverBackedUp"));
  await stageVmsDomain(page, [vmPayload(0)]);

  const RUN_ID = "0a1b2c3d4e5f66778899aabbccddeeff";
  const startedAt = Math.floor(Date.now() / 1000) - 60;
  let done = false;
  await page.route("**/api/vms", (route) =>
    route.fulfill({
      json: { ok: true, vms: [vmPayload(0, done ? { lastBackup: startedAt + 45 } : {})] },
    }),
  );
  let fired = false;
  await page.route("**/api/runs", (route) =>
    route.fulfill({
      json: {
        ok: true,
        runs: fired
          ? [
              {
                id: RUN_ID,
                targetId: "aaaa0000bbbb1111cccc2222dddd3333",
                kind: "backup",
                status: "success",
                startedAt,
                finishedAt: startedAt + 45,
                snapshotId: RUN_ID,
                bytes: 1_000_000,
                error: "",
                acknowledged: false,
                target: "id-00",
                domain: "vm",
              },
            ]
          : [],
      },
    }),
  );

  await page.goto("/vms");
  await page.getByRole("button", { name: /^vm-00/ }).filter({ visible: true }).tap();
  const heading = page.getByRole("heading", { level: 2, name: "vm-00" });
  await expect(heading).toBeVisible();

  const posted = page.waitForRequest(
    (r) => r.method() === "POST" && /\/api\/vms\/id-00\/backup$/.test(r.url()),
  );
  await page.getByRole("button", { name: "Back up now" }).filter({ visible: true }).tap();
  await posted;
  // The finished run reloads the list, which no longer matches the filter.
  const reloaded = page.waitForResponse((r) => /\/api\/vms$/.test(r.url()));
  done = true;
  fired = true;
  await reloaded;
  // The detail shows the reloaded record, so it outlived the reload.
  await expect(page.getByText(/^Last backup: (?!Never)/).first()).toBeVisible();
  await expect(heading).toBeVisible();
  await expect(page.getByText(`Done · ${RUN_ID.slice(0, 8)}`)).toBeVisible();
});

test("mobile /vms: gate-off shows the honest outline card and nothing else", async ({
  page,
}, testInfo) => {
  test.skip(
    !MOBILE_PROJECTS.has(testInfo.project.name),
    "mobile-only: the destinations-gate mobile face",
  );
  await stageVmsDomain(page, [vmPayload(0)], { settings: { vmsEnabled: false } });
  await page.goto("/vms");

  // The block shows the domain's hint and the settings row that turns it
  // back on, and no cards. Discover stays in the header: it is the way back
  // after a lost database and does not depend on the domain being on.
  await expect(
    page.getByText("Back up and restore virtual machines over SSH via libvirt."),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: "Load more" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Back up now" }).filter({ visible: true })).toHaveCount(
    0,
  );
  await expect(page.getByRole("button", { name: "Discover backups" }).filter({ visible: true })).toBeVisible();
  await expect(page.getByText("1 VMs · 1 Scheduled")).toHaveCount(0);

  // The gate card's one action: the settings row that turns VMs on. Scoped to
  // the main scroller — the mobile bottom-nav's own Settings tab would
  // otherwise make this a strict-mode collision, and the gate card's row is
  // the surface under test, not the app chrome.
  const settingsLink = page
    .locator("#bv-main")
    .getByRole("link", { name: "Settings" })
    .filter({ visible: true });
  await expect(settingsLink).toBeVisible();
  await settingsLink.tap();
  await expect(page).toHaveURL(/\/settings$/);
});

test("desktop /vms: no mobile chrome on either desktop project", async ({
  page,
}, testInfo) => {
  test.skip(
    !DESKTOP_PROJECTS.has(testInfo.project.name),
    "desktop-only: the >=48rem guard (desktop-768 boundary + desktop-1280)",
  );

  // The desktop page, with the list NOT windowed — all 40 rows render, which
  // is the counter-proof of the mobile block's 20-row window.
  await stageVmsDomain(page, Array.from({ length: 40 }, (_, i) => vmPayload(i)));
  await page.goto("/vms");
  await expect(page.getByTestId("desktop-sidebar")).toBeVisible();
  await expect(page.getByRole("heading", { level: 1, name: "Virtual Machines" })).toBeVisible();
  await expect(page.getByText("vm-00", { exact: true }).filter({ visible: true })).toBeVisible();
  await expect(page.getByText("vm-39", { exact: true }).filter({ visible: true })).toBeVisible();
  // Nothing the phone block adds is in the desktop DOM: no load-more window,
  // no summary line.
  await expect(page.getByRole("button", { name: "Load more" })).toHaveCount(0);
  await expect(page.getByText("40 VMs · 40 Scheduled")).toHaveCount(0);
});

test("desktop /vms: Discover stays in the header while VM backups are off", async ({
  page,
}, testInfo) => {
  test.skip(
    !DESKTOP_PROJECTS.has(testInfo.project.name),
    "desktop-only: the header on the desktop face",
  );
  // A fresh database has VM backups off, and Discover is how a lost
  // database's VM backups are found again, so it must not depend on the
  // switch.
  await stageVmsDomain(page, [vmPayload(0)], { settings: { vmsEnabled: false } });
  await page.goto("/vms");
  await expect(page.getByRole("heading", { level: 1, name: "Virtual Machines" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Discover backups" }).filter({ visible: true })).toBeVisible();
});
