// ---------------------------------------------------------------------------
// List ergonomics e2e — retrofits onto the EXISTING long list: the Containers
// mobile card list. Six scenarios:
//
//   a. the ListToolbar search drives the page's ONE shared filter state —
//      narrowing, the honest no-match card when nothing survives, and the
//      window RESET when the search clears (useLoadMore's reset-on-identity),
//   b. the constant 20-row window → Load more → 40 → the button honestly
//      disappears when hasMore goes false,
//   c. NO auto-load: scrolling to the very bottom never appends rows — the
//      button is the only way the window grows (lib/useLoadMore.ts's ban on
//      observers/scroll listeners, proven from the outside here),
//   d. the toolbar is sticky-IN-FLOW (computed position "sticky", never
//      "fixed" — the StickyActionBar discipline) and its chips switch the
//      rendered section,
//   e. every rendered card clears the 44px touch floor (bounding-box proof,
//      not a class-name assertion), and the toolbar chips + the backup-order
//      arrows clear it too (pseudo-element bleed proof).
//
// Harness honesty — the same discipline touch-tree.spec.ts uses: the e2e
// webServer is the real bombvault binary over a wiped fresh DB, but the
// harness has no Docker, so a fresh DB can never hold a container. The
// container and settings domains are fulfilled at the Playwright route layer
// — the SPA, its fetches, the binary and every route shape are real; only
// the staged payloads are fake, mirroring the Go JSON shapes field-for-field
// (api.ts). The display-prefs abort keeps the harness default English labels
// regardless of worker order (the boot-look cut — mobile-shell.spec.ts's
// bootWithoutServerLook).
//
// Mobile projects only: the desktop invariant (unchanged above 48rem) is
// held two ways on this layout — by construction (the page renders ONE face;
// the phone card list is JSX-gated out of the desktop DOM, a fact pinned by
// the page source guard in src/pages/containersMobileSource.test.ts) and
// from the outside (desktop-untouched.spec.ts's Containers battery asserts
// the absence of this chrome from the desktop DOM).
//
// SCOPE NOTE: this file is the Containers slice of the upstream ergonomics
// suite. The Files sets-list scenarios (and the German-labels set-editor
// wrap sweep that rides with them) travel with the Files PR; the Dashboard
// activity-log scenarios travel with the log's own PR — the log itself
// shipped in the first PR without them.
// ---------------------------------------------------------------------------
import { expect, test, type Page } from "@playwright/test";

// The two device projects from playwright.config.ts; everything else is a
// desktop project (desktop-untouched.spec.ts's branching pattern).
const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

// All locale-dependent expectations in this spec are pinned to en-US (see the
// header note) so Node-side strings match the page byte-for-byte.
test.use({ locale: "en-US" });

// --- staged container domain (Go JSON shapes, api.ts) ------------------------

function containerPayload(i: number, overrides: Record<string, unknown> = {}) {
  const name = `svc-${String(i).padStart(2, "0")}`;
  return {
    name,
    image: `local/${name}:latest`,
    state: "running",
    status: "Up 2 hours",
    ip: "172.18.0.5",
    installed: true,
    includeInSchedule: true,
    lastBackup: null,
    lastBackupStarted: null,
    preHook: "",
    postHook: "",
    stopContainers: [],
    excludes: [],
    lastUpdateCheck: 0,
    lastUpdateResult: "",
    stack: "",
    ...overrides,
  };
}

function containerList(count: number, overrides: (i: number) => Record<string, unknown> = () => ({})) {
  return Array.from({ length: count }, (_, i) => containerPayload(i, overrides(i)));
}

// GET /api/settings — mirrors the Settings interface field-for-field; callers
// override only the fields a scenario needs.
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
      perItemSchedules: true,
      ...settingsOverrides,
    },
  };
}

/** Route-level staging of the container domain (the harness honesty rules in
 *  the header). Every rendered card fetches its mounts meta
 *  (MobileContainerCard's ticked-count line), so the mounts route is a
 *  wildcard fulfilled with an EMPTY mount set — "0 paths", honest, and no
 *  per-fixture route spam. The PATCH drain is fulfilled so the wiped harness
 *  DB never sees a spec-driven write. */
async function stageContainersDomain(
  page: Page,
  containers: ReturnType<typeof containerPayload>[],
) {
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.route("**/api/containers", (route) => route.fulfill({ json: { ok: true, containers } }));
  await page.route("**/api/settings", (route) => route.fulfill({ json: settingsBody() }));
  await page.route("**/api/schedule/next", (route) => route.fulfill({ json: { ok: true, runs: [] } }));
  await page.route("**/api/containers/*/mounts*", (route) =>
    route.fulfill({
      json: {
        ok: true,
        mounts: [],
        custom: [],
        excluded: [],
        excludeCaches: {},
        hostMountRoot: "/mnt/user",
        hostSourceRoot: "/mnt",
      },
    }),
  );
  await page.route(/\/api\/containers\/[^/]+$/, (route) => route.fulfill({ json: { ok: true } }));
}

// --- locators ----------------------------------------------------------------

// BOTH search inputs (desktop FilterPopover + mobile ListToolbar) share the
// containers.searchPlaceholder key — the shared state IS the point — so the
// visible-only filter is what selects the phone's toolbar input.
function toolbarSearch(page: Page) {
  return page.getByPlaceholder("Search containers…").filter({ visible: true });
}

/** The ListToolbar root: the input's SECOND div ancestor (input → the toolbar's
 *  inner flex column → the sticky chrome div). Structural, not class-based —
 *  the position assertion below is exactly the thing a class lookup would
 *  prejudge. */
function toolbarRoot(page: Page) {
  return toolbarSearch(page).locator("xpath=ancestor::div[2]");
}

// Every rendered card is a button whose accessible name starts with the
// container name (MobileContainerCard); the fillers are named svc-NN.
function cards(page: Page) {
  return page.getByRole("button", { name: /^svc-/ });
}

async function scrollMainToBottom(page: Page) {
  await page.locator("#bv-main").evaluate((el) => el.scrollTo(0, el.scrollHeight));
}

// --- the six scenarios -------------------------------------------------------

test("toolbar search filters the shared state, shows the no-match card, and resets the window", async ({
  page,
}, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile presentation only");
  await stageContainersDomain(page, containerList(40));
  await page.goto("/containers");

  const search = toolbarSearch(page);
  await expect(search).toBeVisible();

  // "svc-1" matches svc-10..svc-19 → 10 rows, and the window offers nothing
  // more (a 10-of-40 result is under the constant 20 threshold).
  await search.fill("svc-1");
  await expect(cards(page)).toHaveCount(10);
  await expect(page.getByRole("button", { name: "Load more" })).toHaveCount(0);

  // Zero matches: the honest no-match card — the same filter.noMatch copy the
  // desktop face renders — never a blank column, and the toolbar stays
  // reachable so the filter is clearable. Visible-only keeps the locator on
  // the card that is actually on screen.
  await search.fill("zzz-nothing");
  await expect(cards(page)).toHaveCount(0);
  await expect(page.getByText("No items match the current filters.").filter({ visible: true })).toBeVisible();
  await expect(search).toBeVisible();

  // Clearing resets the window to the constant 20 (useLoadMore's identity
  // reset) and the Load more affordance returns.
  await search.fill("");
  await expect(cards(page)).toHaveCount(20);
  await expect(page.getByRole("button", { name: "Load more" })).toBeVisible();
});

test("load-more window: 20 rows, Load more extends to 40, then the button is honestly gone", async ({
  page,
}, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile presentation only");
  await stageContainersDomain(page, containerList(40));
  await page.goto("/containers");

  // The summary line survives the retrofit (same derived copy as before).
  await expect(page.getByText("40 Containers · 40 Scheduled")).toBeVisible();

  // Initial constant window: 20 of 40; the tail is NOT in the DOM.
  await expect(cards(page)).toHaveCount(20);
  await expect(page.getByText("svc-39", { exact: true }).filter({ visible: true })).toHaveCount(0);

  const loadMore = page.getByRole("button", { name: "Load more" });
  await expect(loadMore).toBeVisible();
  await loadMore.tap();

  await expect(cards(page)).toHaveCount(40);
  await expect(page.getByText("svc-39", { exact: true }).filter({ visible: true })).toBeVisible();
  // Exhausted: hasMore is the ONLY signal the button may gate on — no rows
  // beyond the window, no button.
  await expect(loadMore).toHaveCount(0);
});

test("no auto-load: scrolling to the bottom never grows the window", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile presentation only");
  await stageContainersDomain(page, containerList(40));
  await page.goto("/containers");
  await expect(cards(page)).toHaveCount(20);

  // Bottom of the scroller, three times, with settle time — the pattern an
  // IntersectionObserver-driven list would have answered with rows 21+.
  for (let i = 0; i < 3; i++) {
    await scrollMainToBottom(page);
    await page.waitForTimeout(300);
  }

  // Still 20, and the button is still the ONLY way forward.
  await expect(cards(page)).toHaveCount(20);
  await expect(page.getByRole("button", { name: "Load more" })).toBeVisible();
});

test("toolbar is sticky-in-flow (never fixed) and its chips switch the rendered section", async ({
  page,
}, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile presentation only");
  // Mixed staging: three installed + four orphans, so the installed/not-
  // installed chips have two real sections to switch between.
  const mixed = [
    containerPayload(0, { name: "plex" }),
    containerPayload(1, { name: "jellyfin" }),
    containerPayload(2, { name: "sonarr" }),
    ...containerList(4, (i) => ({ name: `ghost-${i}`, installed: false, state: "missing" })),
  ];
  await stageContainersDomain(page, mixed);
  await page.goto("/containers");

  // Sticky-IN-FLOW: the computed position is "sticky" — never "fixed", which
  // would fight the visualViewport keyboard mechanism (the StickyActionBar
  // discipline ListToolbar documents).
  const toolbar = toolbarRoot(page);
  await expect(toolbar).toHaveCSS("position", "sticky");

  // Fresh context → the persisted installed-toggle defaults to "all": seven
  // cards across both sections. The mixed fixtures carry SPEAKING names
  // (plex/jellyfin/sonarr/ghost-N), so the count locator spans all four.
  const allCards = page.getByRole("button", { name: /^(plex|jellyfin|sonarr|ghost-\d+)/ });
  await expect(allCards).toHaveCount(7);
  await expect(page.getByRole("button", { name: /^ghost-/ })).toHaveCount(4);

  // Chip tap switches the section — same state, two presentations (the chips
  // are the page's OWN FilterControl, not a parallel mobile copy).
  await toolbar.getByRole("tab", { name: "Not installed" }).tap();
  await expect(page.getByRole("button", { name: /^ghost-/ })).toHaveCount(4);
  await expect(page.getByRole("button", { name: /^plex/ })).toHaveCount(0);
  // The section heading via its Badge text: the h2 itself is the zero-geometry
  // positioning box of the overlapping Badge-heading pattern, which Playwright
  // computes as hidden even though the Badge shows — so the text locator must
  // be visible-only to resolve to the rendered Badge.
  await expect(page.getByText("Not installed (backups only)").filter({ visible: true })).toBeVisible();

  await toolbar.getByRole("tab", { name: "Installed", exact: true }).tap();
  await expect(page.getByRole("button", { name: /^ghost-/ })).toHaveCount(0);
  await expect(page.getByRole("button", { name: /^plex/ })).toHaveCount(1);

  // Scoped to the installed-toggle's own tablist — the Schedule/Backup chip
  // groups each carry their own "All" tab with the same role.
  await toolbar
    .getByRole("tablist", { name: "Filter" })
    .getByRole("tab", { name: "All", exact: true })
    .tap();
  await expect(allCards).toHaveCount(7);
});

test("every rendered card clears the 44px touch floor", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile presentation only");
  await stageContainersDomain(page, containerList(40));
  await page.goto("/containers");

  const loadMore = page.getByRole("button", { name: "Load more" });
  await loadMore.tap();
  await expect(cards(page)).toHaveCount(40);

  // Bounding-box proof over the whole window (min-h-[2.75rem] is the class,
  // 44 real pixels is the contract).
  const minHeight = await cards(page).evaluateAll((els) =>
    Math.min(...els.map((el) => el.getBoundingClientRect().height)),
  );
  expect(minHeight).toBeGreaterThanOrEqual(44);
});

test("toolbar chips and reorder arrows clear the 44px touch floor", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile presentation only");
  // THREE orderable containers, no more: the panel lists every orderable
  // container (the explicitly ordered ones first, the rest appended
  // alphabetically — BackupOrderPanel's hydration effect), so the staged
  // list size IS the arrow count. The chips need no cards at all.
  await stageContainersDomain(page, containerList(3));
  // Registered AFTER the domain staging on purpose: Playwright lets the LAST
  // matching route win, and `/api/containers/backup-order` also matches the
  // `/api/containers/{name}$` catch-all above — the panel needs the order
  // payload, not the generic ok envelope. Shape mirrors getBackupOrder's
  // ContainerOrder[] (api.ts) and the Go handler field-for-field (the
  // header's staging rule): {container, order} pairs, not bare names.
  await page.route("**/api/containers/backup-order", (route) =>
    route.fulfill({
      json: {
        ok: true,
        order: [
          { container: "svc-00", order: 0 },
          { container: "svc-01", order: 1 },
          { container: "svc-02", order: 2 },
        ],
      },
    }),
  );
  // The reorder card lives inside <Advanced> (default OFF, persisted in
  // localStorage — lib/advanced.tsx), so the harness visitor opts in before
  // boot; the display-prefs abort the sibling tests rely on stays untouched.
  await page.addInitScript(() => localStorage.setItem("bombvault.advanced", "1"));
  await page.goto("/containers");

  // Chips: the tap area is the ::after bleed pseudo, NOT the host box — a
  // bounding-box read like the card test's above would measure the 24px host
  // and miss the whole point, which is why this reads the pseudo's USED
  // height per element instead (a bleed that regresses to nothing reads 0px
  // here, not 24px).
  const tabs = toolbarRoot(page).getByRole("tab");
  await expect(
    tabs,
    "the mobile toolbar's four chip groups (filter/schedule/backup/sort, 3 segments each) must render for this floor proof",
  ).toHaveCount(12);
  const minChipBleed = await tabs.evaluateAll((els) =>
    Math.min(...els.map((el) => parseFloat(getComputedStyle(el, "::after").height))),
  );
  expect(minChipBleed).toBeGreaterThanOrEqual(44);

  // Reorder arrows: 20px hosts, so 12px of bleed on both vertical sides is
  // exactly the 44px floor — no headroom, which is the point. Three rows
  // staged, so the counts are asserted before the min (an empty evaluateAll
  // would take Math.min of nothing and pass vacuously).
  const arrowBleeds = async (name: string) => {
    const btns = page.getByRole("button", { name });
    await expect(btns, `three "${name}" rows must render`).toHaveCount(3);
    return btns.evaluateAll((els) =>
      els.map((el) => parseFloat(getComputedStyle(el, "::after").height)),
    );
  };
  const ups = await arrowBleeds("Move up");
  const downs = await arrowBleeds("Move down");
  expect(Math.min(...ups, ...downs)).toBeGreaterThanOrEqual(44);
});
