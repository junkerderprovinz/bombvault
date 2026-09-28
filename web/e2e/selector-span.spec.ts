// Every horizontal selector spans the card it sits in, and one in a toolbar
// row hugs its segments. The track of a spanning selector is as wide as its
// card's content box; the Theme picker and the Dashboard's backup history
// strip stand for the page-level selectors on the desktop. The VMs sort strip
// in the phone's list toolbar stands for the toolbar ones: it stays as wide as
// its own segments, well short of the toolbar.
import { expect, test, type Locator, type Page } from "@playwright/test";

const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

async function stage(page: Page): Promise<void> {
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.addInitScript(() => window.localStorage.setItem("bv-lang", "en"));
  await page.route("**/api/settings", async (route) => {
    if (route.request().method() !== "GET") return route.continue();
    const response = await route.fetch();
    const body = await response.json();
    await route.fulfill({ response, json: { ...body, settings: { ...body.settings, vmsEnabled: true } } });
  });
  await page.route("**/api/vms", (route) =>
    route.fulfill({
      json: {
        ok: true,
        vms: ["haos", "win11"].map((name) => ({
          name,
          libvirtName: name,
          state: "running",
          method: "graceful",
          includeInSchedule: true,
          lastBackup: null,
          lastBackupStarted: null,
          scheduleCadence: "",
        })),
      },
    }),
  );
}

/** The track's width beside the content box of the element it sits in. */
function widths(strip: Locator): Promise<{ track: number; box: number }> {
  return strip.evaluate((el) => {
    const box = el.parentElement!;
    const style = getComputedStyle(box);
    return {
      track: el.getBoundingClientRect().width,
      box: box.clientWidth - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight),
    };
  });
}

test("a card's selector spans the card's content box", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "desktop-only: the phone Dashboard has no history strip");
  await stage(page);

  await page.goto("/settings#look");
  const theme = page.getByRole("tablist", { name: "Theme" });
  await expect(theme).toBeVisible();
  const themeWidths = await widths(theme);
  expect(Math.abs(themeWidths.track - themeWidths.box), JSON.stringify(themeWidths)).toBeLessThanOrEqual(1);

  await page.goto("/dashboard");
  const history = page.getByRole("tablist", { name: "Backup health" });
  await expect(history).toBeVisible();
  expect(await history.evaluate((el) => el.parentElement!.classList.contains("rounded-card"))).toBe(true);
  const historyWidths = await widths(history);
  expect(Math.abs(historyWidths.track - historyWidths.box), JSON.stringify(historyWidths)).toBeLessThanOrEqual(1);
});

test("a toolbar's selector hugs its segments", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the list toolbar is the phone's");
  await stage(page);

  await page.goto("/vms");
  const sort = page.getByRole("tablist", { name: "Sort:" });
  await expect(sort).toBeVisible();
  // The strip shares its row with the "Sort:" caption, so a strip that
  // spanned the row would take all of it and drop under the caption.
  const sortWidths = await widths(sort);
  expect(sortWidths.track, JSON.stringify(sortWidths)).toBeLessThan(sortWidths.box - 20);
});
