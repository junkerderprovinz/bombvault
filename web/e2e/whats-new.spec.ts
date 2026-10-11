// The green dots after an update, followed once from the navigation to the
// change: the dot on the entry in the rail or the phone's bar, the dot on the
// settings tile, the dot on the changed card, the click that hides it for
// good, and the list that counts what is left. The harness runs a build
// without a version, so the update is staged: the server says it is the
// release, and the browser remembers the one before.
import { expect, test, type Locator, type Page } from "@playwright/test";

const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);
const NOW = Math.floor(Date.now() / 1000);

const VERSION_NEWS = "Info holds Version, Help and About. Version lists BombVault, GlimStone, restic and rclone.";
const WEEK_NEWS = "The week as a band, with every planned run from Monday to Sunday.";

function entry(lastBackup: number) {
  return {
    kind: "flash",
    key: "flash",
    name: "flash",
    included: true,
    paused: false,
    effectiveSchedule: { kind: "everything", spec: "daily@03:00", alsoSpec: "" },
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

async function stage(page: Page, { from = "v9.9.1", firstStart = false }: Staged = {}): Promise<void> {
  await page.route("**/api/health", (route) => route.fulfill({ json: { ok: true, version: "v10.0.0" } }));
  await page.route("**/api/release-notes*", (route) => route.fulfill({ json: { ok: true, body: "## Added\n\n- One list.", htmlUrl: "" } }));
  await page.route("**/api/items", (route) =>
    route.fulfill({ json: { ok: true, items: [entry(firstStart ? 0 : NOW - 3600)], unlisted: [] } }),
  );
  await page.route("**/api/settings", async (route) => {
    if (route.request().method() !== "GET") return route.continue();
    const res = await route.fetch();
    const body = await res.json();
    await route.fulfill({ json: { ...body, settings: { ...body.settings, flashEnabled: true } } });
  });
  // The stored look is the one under test, so the server's must not replace it.
  await page.route("**/api/display-prefs*", (route) => route.abort());
  // Once per tab: a reload has to find what the first visit left behind.
  await page.addInitScript((seen) => {
    if (window.sessionStorage.getItem("staged")) return;
    window.sessionStorage.setItem("staged", "1");
    window.localStorage.setItem("bv-lang", "en");
    if (seen) window.localStorage.setItem("bombvault.lastSeenVersion", seen);
  }, from);
}

/** Opens the app on a page and puts away the release notes the update opens. */
async function openUpdated(page: Page, path: string): Promise<void> {
  await page.goto(path);
  const notes = page.getByRole("dialog", { name: /What's new/ });
  await notes.getByRole("button", { name: "Close" }).click();
  await expect(notes).toHaveCount(0);
}

const dots = (page: Page) => page.locator("[data-new-dot]");
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
  const fromTile = page.locator(`[data-new-dot="way"][aria-label="This way to 1 change: ${VERSION_NEWS}"]`);
  await expectOn(fromTile, tile);

  await fromTile.click();
  await expect(page).toHaveURL(/\/settings\/info$/);
  const card = page.locator('[data-new="settings-version"]');
  const dot = page.locator('[data-new-dot="settings-version"]');
  await expectOn(dot, card);
  await expect(dot).toHaveAttribute("aria-label", VERSION_NEWS);
  await expect(fromTile).toHaveCount(0);

  const left = page.getByTestId("whats-new-left");
  const all = Number((await left.textContent())!.match(/of (\d+)/)![1]);
  await expect(left).toHaveText(`${all} of ${all} left`);
  const line = page.locator('[data-change="settings-version"]');
  await expect(line).not.toHaveAttribute("data-seen", "true");

  await dot.click();
  await expect(dot).toHaveCount(0);
  await expect(left).toHaveText(`${all - 1} of ${all} left`);
  await expect(line).toHaveAttribute("data-seen", "true");

  await page.reload();
  await expect(left).toHaveText(`${all - 1} of ${all} left`);
  await expect(dot).toHaveCount(0);
  // Nothing leads to the change any more, and the navigation still leads to the rest.
  await expect(wayTo(page, WEEK_NEWS).first()).toBeVisible();
  await expect(wayTo(page, VERSION_NEWS)).toHaveCount(0);

  await page.getByRole("button", { name: "Mark all as seen" }).click();
  await expect(dots(page)).toHaveCount(0);
  await expect(left).toHaveText(`0 of ${all} left`);

  await page.getByRole("button", { name: "Show all again" }).click();
  await expect(left).toHaveText(`${all} of ${all} left`);
  // A dot shows while its element is in view.
  await card.scrollIntoViewIfNeeded();
  await expectOn(dot, card);

  await page.getByRole("switch", { name: "Show glowing dots" }).click();
  await expect(dots(page)).toHaveCount(0);
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
  await expect(page.getByTestId("whats-new-left")).toBeVisible();
  await expect(page.getByRole("switch", { name: "Show glowing dots" })).toHaveAttribute("aria-checked", "false");
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(dots(page)).toHaveCount(0);
});

test("nothing is marked during the first start, though the browser saw an older release", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop-1280", "what decides it is the same at every width");
  await stage(page, { firstStart: true });
  await openUpdated(page, "/welcome");
  await expect(page.getByRole("button", { name: "Restore from a backup" })).toBeVisible();
  await expect(dots(page)).toHaveCount(0);
  await page.goto("/settings/info");
  await expect(page.getByRole("switch", { name: "Show glowing dots" })).toHaveAttribute("aria-checked", "true");
  await expect(page.locator('[data-new="settings-version"]')).toBeVisible();
  await expect(dots(page)).toHaveCount(0);
});
