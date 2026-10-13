// Every page spans the main area beside the sidebar, Settings included, so at
// 1920px the cards of Settings end at the same right edge as the Dashboard's
// and the page actions stand in the window's corner.
import { expect, test, type Page } from "@playwright/test";

/** The box of the column a page draws its cards in: the routed page's root,
 *  or on Settings the content beside the rail. */
async function column(page: Page, route: string, selector = "#bv-main .glim-page-enter > div"): Promise<{ right: number; width: number }> {
  await page.goto(route);
  // The heading comes with the loaded page; the loading placeholder is narrower.
  await expect(page.locator("#bv-main h1").first()).toBeVisible();
  return page
    .locator(selector)
    .first()
    .evaluate((el) => {
      const box = el.getBoundingClientRect();
      return { right: box.right, width: box.width };
    });
}

test.use({ viewport: { width: 1920, height: 1080 } });

test("Settings and the Dashboard both reach the right edge of the main area at 1920px", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop-1280", "one wide desktop run is enough");
  const dashboard = await column(page, "/dashboard");
  const settings = await column(page, "/settings/general", "[data-settings-content]");
  const main = await page.locator("#bv-main").evaluate((el) => el.getBoundingClientRect().right);
  expect(dashboard.width).toBeGreaterThan(1152);
  expect(main - dashboard.right).toBeLessThanOrEqual(48);
  expect(Math.abs(settings.right - dashboard.right)).toBeLessThanOrEqual(1);
});
