// The Settings rail is a column beside the page while the page keeps 40rem
// beside it, and a strip above the page that scrolls sideways once a column
// would squeeze the cards.
import { expect, test, type Page } from "@playwright/test";

const RAIL = "Settings pages";

async function openSettings(page: Page, width: number, id: string): Promise<void> {
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.addInitScript(() => window.localStorage.setItem("bv-lang", "en"));
  await page.setViewportSize({ width, height: 800 });
  await page.goto(`/settings/${id}`);
  await expect(page.getByRole("navigation", { name: RAIL }).locator('[aria-current="page"]')).toHaveCount(1);
}

/** How the rail and the page stand: the rail's tile rows and its scroll, the
 *  page's width against the box both share, and any sideways pan. */
async function layout(page: Page) {
  return page.getByRole("navigation", { name: RAIL }).evaluate((nav) => {
    const list = nav.firstElementChild as HTMLElement;
    const tiles = [...list.querySelectorAll("a")].map((a) => a.getBoundingClientRect());
    const current = list.querySelector('[aria-current="page"]')!.getBoundingClientRect();
    const view = list.getBoundingClientRect();
    const content = document.querySelector("[data-settings-content]")!.getBoundingClientRect();
    const frame = nav.parentElement!.getBoundingClientRect();
    const main = document.getElementById("bv-main")!;
    return {
      rows: new Set(tiles.map((t) => Math.round(t.top))).size,
      columns: new Set(tiles.map((t) => Math.round(t.left))).size,
      scrolls: list.scrollWidth > list.clientWidth + 1,
      currentInView: current.left >= view.left - 1 && current.right <= view.right + 1,
      above: nav.getBoundingClientRect().bottom <= content.top + 1,
      contentShare: content.width / frame.width,
      docPan: document.documentElement.scrollWidth - window.innerWidth,
      mainPan: main.scrollWidth - main.clientWidth,
    };
  });
}

test("settings @ 1280px: the rail is a column beside the page", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop-1280", "the widths are set per test; one desktop project runs them");
  await openSettings(page, 1280, "general");
  const at = await layout(page);
  expect(at.columns, "tiles side by side").toBe(1);
  expect(at.rows, "a row per page").toBe(14);
  expect(at.above).toBe(false);
  expect(at.docPan).toBeLessThanOrEqual(1);
  expect(at.mainPan).toBeLessThanOrEqual(1);
});

for (const width of [1024, 768]) {
  test(`settings @ ${width}px: the rail is one scrolling row above a full-width page`, async ({ page }, testInfo) => {
    test.skip(testInfo.project.name !== "desktop-1280", "the widths are set per test; one desktop project runs them");
    // The last page, so the strip has to scroll to show its tile.
    await openSettings(page, width, "info");
    const at = await layout(page);
    expect(at.rows, "rows of tiles").toBe(1);
    expect(at.scrolls, "the strip scrolls sideways").toBe(true);
    expect(at.currentInView, "the open page's tile is in view").toBe(true);
    expect(at.above, "the strip stands above the page").toBe(true);
    expect(at.contentShare, "the page takes the whole width").toBeGreaterThan(0.99);
    expect(at.docPan, "the document pans sideways").toBeLessThanOrEqual(1);
    expect(at.mainPan, "#bv-main pans sideways").toBeLessThanOrEqual(1);

    // Picking a page at the other end brings its tile into view as well.
    await page.getByRole("navigation", { name: RAIL }).getByRole("link", { name: "Info", exact: true }).focus();
    await page.keyboard.press("Home");
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(/\/settings\/general$/);
    await expect.poll(async () => (await layout(page)).currentInView).toBe(true);
  });
}
