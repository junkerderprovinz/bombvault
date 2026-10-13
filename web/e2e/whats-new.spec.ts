// The green dots after an update, followed once from the navigation to the
// change: the dot on the entry in the rail or the phone's bar, the dot on the
// settings tile, the dot on the changed card, the click that hides it for
// good, and the button that counts what is left. The harness runs a build
// without a version, so the update is staged: the server says it is the
// release, and the browser remembers the one before. After it: the way into
// the restore window, the dots of an entry's page, a change whose page has
// nothing to show for it, and the dots on a row of the overview and on the
// move of a storage location.
import { expect, test, type Locator, type Page } from "@playwright/test";
import { mkdirSync } from "node:fs";
import { join } from "node:path";

import { EVERY_KIND, MOBILE_PROJECTS, NOW, stageEntries } from "./entries";
import { openOverview, stageOverview } from "./overview";
import { STORAGE_BOX, stageStorage } from "./storage";
import { blockServerLook } from "./serverLook";

// Set to a folder to keep pictures of the dots.
const SHOTS = process.env.WHATS_NEW_SHOTS;

const VERSION_NEWS =
  "Info holds Version, Help and About. Version lists BombVault, GlimStone, restic and rclone. Its New button opens the list of changes, or the release notes once you have seen them all.";
const WEEK_NEWS = "The week as a band, with every planned run from Monday to Sunday.";
const HEAD_NEWS = "Green dots mark what is new.";
const DECLINE_NEWS = "Don't back up takes an entry off this list";
const TILES_NEWS = "Restore is one page for everything.";
const WHERE_NEWS = "One window asks what to restore and where to.";
const KEEP_NEWS = "Retention is Short, Balanced, Long or your own values.";
const DEFAULTS_NEWS = "An entry follows the default schedule and storage location";

function entry(lastBackup: number) {
  return {
    kind: "flash",
    key: "flash",
    name: "flash",
    included: true,
    paused: false,
    effectiveSchedule: { kind: "everything", spec: "daily@03:00" },
    lastBackup,
    lastRunStatus: lastBackup ? "success" : "",
    sourceBytes: lastBackup ? 1024 ** 3 : null,
    runs: [],
  };
}

interface Staged {
  /** The release this browser saw last, "" for a browser that saw none. */
  from?: string;
  /** No entry has a backup yet. */
  firstStart?: boolean;
}

/** Stages the update alone, for a test that brings its own entries. */
async function stageUpdate(page: Page, from = "v9.9.1"): Promise<void> {
  await page.route("**/api/health", (route) => route.fulfill({ json: { ok: true, version: "v10.0.0" } }));
  await page.route("**/api/release-notes*", (route) => route.fulfill({ json: { ok: true, body: "## Added\n\n- One list.", htmlUrl: "" } }));
  // The stored look is the one under test, so the server's must not replace it.
  await blockServerLook(page);
  // Once per tab: a reload has to find what the first visit left behind.
  await page.addInitScript((seen) => {
    if (window.sessionStorage.getItem("staged")) return;
    window.sessionStorage.setItem("staged", "1");
    window.localStorage.setItem("bv-lang", "en");
    if (seen) window.localStorage.setItem("bombvault.lastSeenVersion", seen);
  }, from);
}

async function stage(page: Page, { from = "v9.9.1", firstStart = false }: Staged = {}): Promise<void> {
  await stageUpdate(page, from);
  await page.route("**/api/items", (route) =>
    route.fulfill({ json: { ok: true, items: [entry(firstStart ? 0 : NOW - 3600)], unlisted: [] } }),
  );
  await page.route("**/api/settings", async (route) => {
    if (route.request().method() !== "GET") return route.continue();
    const res = await route.fetch();
    const body = await res.json();
    await route.fulfill({ json: { ...body, settings: { ...body.settings, flashEnabled: true } } });
  });
}

/** Opens the app on a page and puts away the window the update opens on its release notes. */
async function openUpdated(page: Page, path: string): Promise<void> {
  await page.goto(path);
  const notes = page.getByRole("dialog", { name: "New in 10.0.0" });
  await expect(notes.getByRole("tab", { name: "Release notes" })).toHaveAttribute("aria-selected", "true");
  await expect(notes.getByText("One list.")).toBeVisible();
  await notes.getByRole("button", { name: "Close" }).click();
  await expect(notes).toHaveCount(0);
}

const dots = (page: Page) => page.locator("[data-new-dot]");
const dotOf = (page: Page, id: string) => page.locator(`[data-new-dot="${id}"]`);
const marked = (page: Page, id: string) => page.locator(`[data-new="${id}"]`).first();
const absent = (page: Page) =>
  page.evaluate(() => (JSON.parse(window.localStorage.getItem("bv-whats-new@10.0.0") ?? "{}") as { absent?: string[] }).absent ?? []);

async function picture(page: Page, name: string): Promise<void> {
  if (!SHOTS) return;
  mkdirSync(SHOTS, { recursive: true });
  await page.screenshot({ path: join(SHOTS, `${name}.png`) });
}
const markAll = (page: Page, n: number) => page.getByRole("button", { name: `Mark all as seen · ${n}` });
const showAll = (page: Page, n: number) => page.getByRole("button", { name: `Show all again · ${n}` });
const newButton = (page: Page, n?: number) =>
  page.locator("#bv-main").getByRole("button", { name: n === undefined ? /^New( · \d+)?$/ : n > 0 ? `New · ${n}` : "New", exact: true });
const list = (page: Page) => page.getByRole("dialog", { name: "New in 10.0.0" });

/** Opens the list of changes from the Version card. With nothing left to see
 *  the button opens the release notes, and the list is the tab beside them. */
async function openList(page: Page, n?: number): Promise<Locator> {
  await newButton(page, n).click();
  await expect(list(page)).toBeVisible();
  if (n === 0) {
    await expect(list(page).getByRole("tab", { name: "Release notes" })).toHaveAttribute("aria-selected", "true");
    await list(page).getByRole("tab", { name: /^Changes/ }).click();
  }
  return list(page);
}

async function closeList(page: Page): Promise<void> {
  await list(page).getByRole("button", { name: "Close" }).click();
  await expect(list(page)).toHaveCount(0);
}
const wayTo = (page: Page, news: string) => page.locator(`[data-new-dot="way"][aria-label*="${news}"]`);

/** Expects a dot on the top corner at the end of what it marks. */
async function expectOn(dot: Locator, marked: Locator): Promise<void> {
  await expect(dot).toBeVisible();
  await expect(marked).toBeVisible();
  await expect
    .poll(async () => {
      const [d, m] = [await dot.boundingBox(), await marked.boundingBox()];
      // Between two placements a dot is not there for a moment.
      if (!d || !m) return false;
      const [x, y] = [d.x + d.width / 2, d.y + d.height / 2];
      return Math.abs(x - (m.x + m.width - 4)) <= 2 && Math.abs(y - (m.y + 4)) <= 2;
    })
    .toBe(true);
}

test("a dot leads from the navigation over the settings tile to the change, and a click hides it for good", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name === "desktop-768", "the path runs once at desktop width and on both phones");
  const phone = MOBILE_PROJECTS.has(testInfo.project.name);
  await stage(page);
  await openUpdated(page, "/dashboard");

  // The entry in the rail, or its slot in the phone's bar.
  const settings = phone
    ? page.getByRole("navigation", { name: "Mobile navigation" }).locator('a[href="/settings"]')
    : page.locator('a[href="/settings"]').first();
  const fromNavigation = page.locator(`[data-new-dot="way"][aria-label*="${VERSION_NEWS}"][aria-label*="${WEEK_NEWS}"]`);
  await expectOn(fromNavigation, settings);
  await expect(fromNavigation).toHaveAttribute("aria-label", /^This way to \d+ changes:/);

  await fromNavigation.click();
  await expect(page).toHaveURL(/\/settings\/\w+$/);
  const tile = page.locator('a[href="/settings/info"]');
  const fromTile = page.locator(`[data-new-dot="way"][aria-label^="This way to 2 changes:"][aria-label*="${VERSION_NEWS}"]`);
  await expectOn(fromTile, tile);

  await fromTile.click();
  await expect(page).toHaveURL(/\/settings\/info$/);
  const card = page.locator('[data-new="settings-version"]');
  const dot = page.locator('[data-new-dot="settings-version"]');
  await expectOn(dot, card);
  await expect(dot).toHaveAttribute("aria-label", VERSION_NEWS);
  await expect(fromTile).toHaveCount(0);

  // The Version card's button counts what is left and opens the list.
  await openList(page);
  const all = await page.locator("[data-change]").count();
  await expect(markAll(page, all)).toBeVisible();
  const line = page.locator('[data-change="settings-version"]');
  await expect(line).not.toHaveAttribute("data-seen", "true");
  await closeList(page);
  await expect(newButton(page, all)).toBeVisible();

  await dot.click();
  await expect(dot).toHaveCount(0);
  await expect(newButton(page, all - 1)).toBeVisible();
  await openList(page, all - 1);
  await expect(markAll(page, all - 1)).toBeVisible();
  await expect(line).toHaveAttribute("data-seen", "true");
  await closeList(page);

  await page.reload();
  await expect(newButton(page, all - 1)).toBeVisible();
  await expect(dot).toHaveCount(0);
  // Nothing leads to the change any more, and the navigation still leads to the rest.
  await expect(wayTo(page, WEEK_NEWS).first()).toBeVisible();
  await expect(wayTo(page, VERSION_NEWS)).toHaveCount(0);

  await openList(page, all - 1);
  await markAll(page, all - 1).click();
  await expect(showAll(page, all)).toBeVisible();
  await closeList(page);
  await expect(dots(page)).toHaveCount(0);
  await expect(newButton(page, 0)).toBeVisible();

  await openList(page, 0);
  await showAll(page, all).click();
  await expect(markAll(page, all)).toBeVisible();
  // A change's Go there closes the list on the way.
  await line.getByRole("button", { name: "Go there" }).click();
  await expect(list(page)).toHaveCount(0);
  // A dot shows while its element is in view.
  await card.scrollIntoViewIfNeeded();
  await expectOn(dot, card);
});

test("Go there takes the place of the list in the history, so Back returns to where the list was opened", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop-1280", "one desktop project is enough for the history");
  await stage(page);
  await openUpdated(page, "/dashboard");
  await page.locator('a[href="/settings"]').first().click();
  await expect(page).toHaveURL(/\/settings\/\w+$/);
  const before = page.url();
  await page.locator('a[href="/settings/info"]').click();
  await expect(page).toHaveURL(/\/settings\/info$/);
  expect(before).not.toMatch(/\/settings\/info$/);

  await openList(page);
  await list(page).locator("[data-change]").filter({ hasText: TILES_NEWS }).getByRole("button", { name: "Go there" }).click();
  await expect(list(page)).toHaveCount(0);
  await expect(page).not.toHaveURL(/\/settings\/info$/);
  await page.goBack();
  await expect(page).toHaveURL(/\/settings\/info$/);
  await expect(list(page)).toHaveCount(0);
  await page.goBack();
  await expect(page).toHaveURL(before);
});

test("the dots breathe, and stand still where motion is reduced", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop-1280", "the motion is the same at every width");
  await stage(page);
  await openUpdated(page, "/settings/info");
  const dot = page.locator('[data-new-dot="settings-version"]');
  await expect(dot).toBeVisible();
  const breathing = () => dot.evaluate((el) => getComputedStyle(el).animationName);
  expect(await breathing()).toBe("glim-new-glow");
  await page.emulateMedia({ reducedMotion: "reduce" });
  expect(await breathing()).toBe("none");
});

test("nothing is marked for a browser that saw no older release", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop-1280", "what decides it is the same at every width");
  await stage(page, { from: "" });
  await page.goto("/settings/info");
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(dots(page)).toHaveCount(0);
  await openList(page, 0);
  await expect(page.getByRole("button", { name: /^Show all again · \d+$/ })).toBeVisible();
});

test("nothing is marked during the first start, though the browser saw an older release", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop-1280", "what decides it is the same at every width");
  await stage(page, { firstStart: true });
  await openUpdated(page, "/welcome");
  await expect(page.getByRole("button", { name: "Restore from a backup" })).toBeVisible();
  await expect(dots(page)).toHaveCount(0);
  await page.goto("/settings/info");
  await expect(page.locator('[data-new="settings-version"]')).toBeVisible();
  await expect(dots(page)).toHaveCount(0);
  await expect(newButton(page)).toHaveText(/^New · \d+$/);
});

test("the dots lead from the navigation over a backup's button into the restore window", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name === "desktop-768", "the path runs once at desktop width and on both phones");
  const phone = MOBILE_PROJECTS.has(testInfo.project.name);
  await stageEntries(page, EVERY_KIND);
  await stageUpdate(page);
  await openUpdated(page, "/dashboard");

  const restore = phone
    ? page.getByRole("navigation", { name: "Mobile navigation" }).locator('a[href="/restore"]')
    : page.locator('a[href="/restore"]').first();
  const fromNavigation = page.locator(`[data-new-dot="way"][aria-label*="${TILES_NEWS}"][aria-label*="${WHERE_NEWS}"]`);
  await expectOn(fromNavigation, restore);
  // One change to a line.
  expect((await fromNavigation.getAttribute("aria-label"))!.split("\n")).toHaveLength(5);
  await fromNavigation.hover();
  await picture(page, `way-bubble-${testInfo.project.name}`);

  await fromNavigation.click();
  await expect(page).toHaveURL(/\/restore$/);
  await expectOn(dotOf(page, "restore-tiles"), marked(page, "restore-tiles"));
  await expect(fromNavigation).toHaveCount(0);
  // Picking several entries starts at the button beside the search.
  await expectOn(dotOf(page, "restore-many"), marked(page, "restore-many"));

  const button = page.locator('[data-new-way="restore-window"]');
  await button.scrollIntoViewIfNeeded();
  const toWindow = wayTo(page, WHERE_NEWS);
  await expectOn(toWindow, button);
  await picture(page, `restore-page-${testInfo.project.name}`);

  await toWindow.click();
  const win = page.getByRole("dialog");
  await expect(win).toBeVisible();
  await win.locator('[data-new="restore-where"]').scrollIntoViewIfNeeded();
  await expectOn(dotOf(page, "restore-where"), win.locator('[data-new="restore-where"]'));
  // The window covers the page, so the page's dots wait behind it.
  await expect(dotOf(page, "restore-tiles")).toHaveCount(0);
  await expect(toWindow).toHaveCount(0);
  await picture(page, `restore-window-${testInfo.project.name}`);

  await dotOf(page, "restore-where").click();
  await expect(dotOf(page, "restore-where")).toHaveCount(0);
  await win.getByRole("button", { name: "Cancel" }).click();
  await expect(toWindow).toHaveCount(0);

  await marked(page, "restore-emergency").scrollIntoViewIfNeeded();
  await expectOn(dotOf(page, "restore-emergency"), marked(page, "restore-emergency"));
  await picture(page, `restore-emergency-${testInfo.project.name}`);
});

test("a container's page carries its dots, and the tab leads to the one on its settings", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name === "desktop-768", "the path runs once at desktop width and on both phones");
  await stageEntries(page, EVERY_KIND);
  await stageUpdate(page);
  await openUpdated(page, "/backups");

  // The first row leads to the pages of the entries.
  const row = page.getByTestId("backup-row").first();
  const fromRow = wayTo(page, DEFAULTS_NEWS);
  await expect(fromRow).toHaveCount(1);
  await fromRow.click();
  await expect(page).toHaveURL(/\/backups\/\w+\/[^/]+$/);
  await expect(row).toHaveCount(0);

  await page.goto("/backups/container/immich");
  await expectOn(dotOf(page, "entry-page"), marked(page, "entry-page"));
  await expectOn(dotOf(page, "entry-menu"), marked(page, "entry-menu"));
  const tab = page.getByRole("tablist", { name: "immich" }).getByRole("tab", { name: "Settings" });
  const fromTab = wayTo(page, DEFAULTS_NEWS);
  await expectOn(fromTab, tab);
  await picture(page, `entry-container-${testInfo.project.name}`);

  await fromTab.click();
  await expect(tab).toHaveAttribute("aria-selected", "true");
  await marked(page, "entry-defaults").scrollIntoViewIfNeeded();
  await expectOn(dotOf(page, "entry-defaults"), marked(page, "entry-defaults"));
  await expect(fromTab).toHaveCount(0);
  await picture(page, `entry-container-settings-${testInfo.project.name}`);

  await marked(page, "entry-menu").scrollIntoViewIfNeeded();
  await expect(marked(page, "entry-menu").getByRole("button", { name: "Export container" })).toBeVisible();
  await picture(page, `entry-container-actions-${testInfo.project.name}`);
});

test("a change its page has nothing for is no longer promised, and is back when the page draws it", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop-1280", "what decides it is the same at every width");
  // Every entry is backed up, so the overview has no row to decline.
  await stage(page);
  await openUpdated(page, "/backups");
  const toOverview = wayTo(page, HEAD_NEWS);
  await expect(toOverview).toHaveAttribute("aria-label", new RegExp(DECLINE_NEWS));

  await toOverview.click();
  await expect(page).toHaveURL(/\/dashboard$/);
  await expect(dotOf(page, "overview-head")).toBeVisible();
  await expect.poll(() => absent(page)).toContain("overview-decline");
  expect(await absent(page)).not.toContain("overview-head");

  await page.locator('a[href="/backups"]').first().click();
  await expect(toOverview).toBeVisible();
  await expect(toOverview).not.toHaveAttribute("aria-label", new RegExp(DECLINE_NEWS));

  await page.goto("/settings/info");
  await openList(page);
  const line = page.locator('[data-change="overview-decline"]');
  await expect(line).toHaveAttribute("data-absent", "true");
  await expect(line).toContainText("Not on its page at the moment");
  await expect(markAll(page, await page.locator("[data-change]").count())).toBeVisible();
  await line.scrollIntoViewIfNeeded();
  await picture(page, "list-absent");

  // An entry nobody backs up brings the row, and the row its dot.
  await page.unrouteAll({ behavior: "ignoreErrors" });
  await stageOverview(page, undefined, { lang: "en" });
  await stageUpdate(page);
  await openOverview(page);
  const needs = page.getByTestId("need-unprotected").first();
  await needs.scrollIntoViewIfNeeded();
  await expectOn(dotOf(page, "overview-decline"), needs);
  await expect.poll(() => absent(page)).not.toContain("overview-decline");
  await picture(page, "overview-decline");
});

test("a storage location's row leads to its page, where the move of a section carries a dot", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop-1280", "what decides it is the same at every width");
  // The staged entry has a backup, so this is no first start.
  await stage(page);
  await stageStorage(page, 1280, "en");
  await openUpdated(page, "/storage");

  const fromRow = wayTo(page, KEEP_NEWS);
  await expect(fromRow).toHaveCount(1);
  await picture(page, "storage-row");
  await fromRow.click();
  await expect(page).toHaveURL(/\/storage\/[^/]+$/);

  await page.goto(`/storage/${encodeURIComponent(STORAGE_BOX.id)}`);
  await marked(page, "location-move").scrollIntoViewIfNeeded();
  await expectOn(dotOf(page, "location-move"), marked(page, "location-move"));
  await picture(page, "location-move");
});
