// Every horizontal selector spans the card it sits in, and one beside a
// caption hugs its segments. The track of a spanning selector is as wide as
// its card's content box; the Theme picker and the overview's calendar strip
// stand for the page-level selectors on the desktop. A filter of the Backups
// list stands for the ones beside a caption: it stays as wide as its own
// segments, well short of its row.
import { expect, test, type Locator, type Page } from "@playwright/test";
import { EVERY_KIND, MOBILE_PROJECTS, stageEntries } from "./entries";

// A test can end while a route handler still waits on the real server. Closing
// the context then disposes the response it is about to read, and Playwright
// fails a test that already passed, so what the handler throws is dropped.
test.afterEach(async ({ page }) => {
  await page.unrouteAll({ behavior: "ignoreErrors" });
});

async function stage(page: Page): Promise<void> {
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.addInitScript(() => window.localStorage.setItem("bv-lang", "en"));
  await page.route("**/api/settings", async (route) => {
    if (route.request().method() !== "GET") return route.continue();
    const response = await route.fetch();
    const body = await response.json();
    await route.fulfill({ response, json: { ...body, settings: { ...body.settings, receiverEnabled: true, fleetEnabled: true, pullEnabled: true } } });
  });
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
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "desktop-only: the strips are measured at desktop width");
  await stage(page);
  // The calendar is one of the cards a browser starts without.
  await page.addInitScript(() =>
    window.localStorage.setItem(
      "bombvault.dashboardLayout",
      JSON.stringify({ v: 2, order: ["heatmap"], hidden: [], widths: { heatmap: 6 }, heights: {} }),
    ),
  );

  await page.goto("/settings/look");
  const theme = page.getByRole("tablist", { name: "Theme" });
  await expect(theme).toBeVisible();
  const themeWidths = await widths(theme);
  expect(Math.abs(themeWidths.track - themeWidths.box), JSON.stringify(themeWidths)).toBeLessThanOrEqual(1);

  await page.goto("/dashboard");
  const history = page.getByRole("tablist", { name: "Calendar" });
  await expect(history).toBeVisible();
  expect(await history.evaluate((el) => el.parentElement!.classList.contains("rounded-card"))).toBe(true);
  const historyWidths = await widths(history);
  expect(Math.abs(historyWidths.track - historyWidths.box), JSON.stringify(historyWidths)).toBeLessThanOrEqual(1);
});

test("a page-level tab strip never wraps", async ({ page }, testInfo) => {
  // backup-entry.spec.ts covers the same strip at phone width.
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "desktop-only: backup-entry.spec.ts covers the phone scale");
  await stageEntries(page, EVERY_KIND);

  await page.goto("/backups/vm/win11");
  const tabs = page.getByRole("tablist", { name: "Windows 11" });
  await expect(tabs).toBeVisible();
  expect(await tabs.evaluate((el) => getComputedStyle(el).flexWrap)).toBe("nowrap");
  const tops = await tabs.getByRole("tab").evaluateAll((els) => new Set(els.map((el) => Math.round(el.getBoundingClientRect().top))).size);
  expect(tops, "the tabs share one row").toBe(1);
});

test("a selector beside its caption hugs its segments", async ({ page }) => {
  await stageEntries(page, EVERY_KIND);

  await page.goto("/backups");
  await page.getByRole("button", { name: "Filters" }).click();
  const schedule = page.getByRole("tablist", { name: "Schedule" });
  await expect(schedule).toBeVisible();
  // The strip shares its row with its caption, so a strip that spanned the
  // row would take all of it and drop under the caption.
  const scheduleWidths = await widths(schedule);
  expect(scheduleWidths.track, JSON.stringify(scheduleWidths)).toBeLessThan(scheduleWidths.box - 20);
});

test("a VM's method strip keeps its size through a save", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the phone renders the same component");
  await stageEntries(page, EVERY_KIND);
  let release = () => {};
  const held = new Promise<void>((resolve) => (release = resolve));
  await page.route("**/api/vms/win11", async (route) => {
    if (route.request().method() !== "PATCH") return route.continue();
    await held;
    await route.fulfill({ json: { ok: true } });
  });

  await page.goto("/backups/vm/win11?tab=settings");
  const strip = page.getByRole("tablist", { name: "Method", exact: true });
  await expect(strip).toBeVisible();
  const width = () => strip.evaluate((el) => el.getBoundingClientRect().width);
  const before = await width();

  await strip.getByRole("tab", { name: "Live snapshot" }).click();
  await expect(strip.getByRole("tab", { name: "Live snapshot" })).toBeDisabled();
  expect(await width(), "while saving").toBeCloseTo(before, 0);
  release();
  await expect(strip.getByRole("tab", { name: "Live snapshot" })).toBeEnabled();
  // The strip measures itself again in a resize callback, a frame or two on.
  await page.evaluate(() => new Promise((done) => requestAnimationFrame(() => requestAnimationFrame(done))));
  expect(await width(), "after saving").toBeCloseTo(before, 0);
});
