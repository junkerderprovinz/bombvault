// Settings shares the 1152px column of every other page. At 1920px the cap is
// the only thing holding a page in, so an uncapped Settings would stretch its
// cards across the window while the Dashboard stays a column.
import { expect, test, type Page } from "@playwright/test";

/** The width of the column a page draws its cards in: the routed page's root,
 *  or on Settings the content beside the rail. */
async function columnWidth(page: Page, route: string, column = "#bv-main .glim-page-enter > div"): Promise<number> {
  await page.goto(route);
  // The heading comes with the loaded page; the loading placeholder is narrower.
  await expect(page.locator("#bv-main h1").first()).toBeVisible();
  return page.locator(column).first().evaluate((el) => el.getBoundingClientRect().width);
}

test.use({ viewport: { width: 1920, height: 1080 } });

test("the Settings column is no wider than the Dashboard's at 1920px", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop-1280", "one wide desktop run is enough");
  const dashboard = await columnWidth(page, "/dashboard");
  const settings = await columnWidth(page, "/settings/general", "[data-settings-content]");
  expect(dashboard).toBeLessThanOrEqual(1152);
  expect(settings).toBeLessThanOrEqual(dashboard + 1);
});
