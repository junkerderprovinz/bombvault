// Settings > Schedule with a filled instance staged at the route layer: the
// full backup reads as one plan sentence whose editor opens beside it, a
// section joins the exceptions from the Exception list, and a section's
// entries come up in a window. On a phone nothing reaches past the viewport.
import { expect, test, type Page, type Route } from "@playwright/test";

const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

test.afterEach(async ({ page }) => {
  await page.unrouteAll({ behavior: "ignoreErrors" });
});

const SETTINGS = {
  containersEnabled: true,
  vmsEnabled: true,
  flashEnabled: true,
  filesEnabled: true,
  zfsEnabled: true,
  configEnabled: true,
  everythingSchedule: "daily 03:00",
  containersSchedule: "off",
  vmsSchedule: "weekly Sun 04:00",
  flashSchedule: "off",
  filesSchedule: "off",
  zfsSchedule: "off",
  configSchedule: "off",
  zfsOffsiteSchedule: "daily 05:00",
  perItemSchedules: true,
};

const ITEMS = [
  { kind: "container", key: "immich", name: "immich", included: true },
  { kind: "container", key: "paperless-ngx", name: "paperless-ngx", included: false },
  { kind: "vm", key: "win11", name: "Windows 11", included: true },
].map((item) => ({
  paused: false,
  effectiveSchedule: { kind: "everything", spec: "daily 03:00", alsoSpec: "" },
  lastBackup: 0,
  lastRunStatus: "",
  sourceBytes: null,
  installed: true,
  ...item,
}));

async function stage(page: Page): Promise<void> {
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.addInitScript(() => window.localStorage.setItem("bv-lang", "en"));
  await page.route("**/api/settings", async (route: Route) => {
    if (route.request().method() !== "GET") return route.fulfill({ json: { ok: true } });
    const response = await route.fetch();
    const body = await response.json();
    await route.fulfill({ response, json: { ...body, platform: "unraid", settings: { ...body.settings, ...SETTINGS } } });
  });
  await page.route("**/api/items", (route) => route.fulfill({ json: { ok: true, items: ITEMS, unlisted: [] } }));
}

// A list opened while the page still scrolls closes again, as it does for a
// person who scrolls away from it.
async function scrollRest(page: Page): Promise<void> {
  await page.evaluate(
    () =>
      new Promise<void>((done) => {
        const main = document.getElementById("bv-main")!;
        let last = -1;
        let still = 0;
        const tick = () => {
          still = main.scrollTop === last ? still + 1 : 0;
          last = main.scrollTop;
          if (still >= 5) done();
          else requestAnimationFrame(tick);
        };
        requestAnimationFrame(tick);
      })
  );
}

async function open(page: Page): Promise<void> {
  await stage(page);
  await page.goto("/settings/schedules");
  await expect(page.locator("[data-search-card='Full backup']")).toBeVisible();
  await page.waitForLoadState("networkidle");
  await expect(page.getByText("1 of 2 scheduled")).toBeVisible();
}

test("the full backup opens its plan editor from the sentence", async ({ page }) => {
  await open(page);
  await page.getByRole("button", { name: /^Full backup: daily at 3:00/i }).click();
  const editor = page.getByRole("dialog", { name: "Full backup" });
  await expect(editor).toBeVisible();
  await expect(editor.getByRole("tab", { name: "Weekly" })).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(editor).toBeHidden();
});

test("a section joins the exceptions from the Exception list", async ({ page }) => {
  await open(page);
  const plans = page.locator("[data-search-card='Plans per section']");
  await expect(plans.getByText("All other sections run in the full backup.")).toBeVisible();
  const add = plans.getByRole("button", { name: "Exception", exact: true }).first();
  await add.scrollIntoViewIfNeeded();
  await scrollRest(page);
  await add.click();
  const list = page.getByRole("listbox", { name: "Plans per section" });
  await expect(list.getByRole("option")).toHaveText(["Containers", "Flash", "Folders", "ZFS", "Self-backup"]);
  await list.getByRole("option", { name: "Flash" }).click();
  await expect(plans.getByRole("button", { name: /^Flash: weekly/i })).toBeVisible();
});

test("a section's entries come up in a window with a switch each", async ({ page }) => {
  await open(page);
  await expect(page.getByText("1 of 2 scheduled")).toBeVisible();
  await page.locator("[data-kind='container']").getByRole("button", { name: "Entries" }).click();
  const window = page.getByRole("dialog", { name: /Containers on the schedule/ });
  await expect(window.getByRole("switch")).toHaveCount(2);
});

test("on a phone nothing on the page reaches past the viewport", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only");
  await open(page);
  const wide = await page.evaluate(() => {
    const limit = document.documentElement.clientWidth + 1;
    return [...document.querySelectorAll("[data-settings-page] button")]
      .filter((el) => el.getBoundingClientRect().right > limit || el.getBoundingClientRect().left < -1)
      .map((el) => el.textContent?.trim() ?? "");
  });
  expect(wide).toEqual([]);
});
