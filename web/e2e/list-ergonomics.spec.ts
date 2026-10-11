// A long Backups list on a phone. The search narrows the one list and says
// when nothing is left, the list shows twenty rows and grows only when asked,
// never because the page was scrolled, every row is big enough for a finger,
// and the bar of picked rows stays in view without leaving the page's flow.
import { expect, test, type Page } from "@playwright/test";
import { container, item, stageEntries, type Staged } from "./entries";

// A handler can still wait on the real server when its test has passed;
// what it throws then is dropped.
test.afterEach(async ({ page }) => {
  await page.unrouteAll({ behavior: "ignoreErrors" });
});

const name = (i: number) => `svc-${String(i).padStart(2, "0")}`;

function long(count: number, kind: "container" | "vm" = "container"): Staged {
  const names = Array.from({ length: count }, (_, i) => name(i));
  return {
    items: names.map((key) => item({ kind, key })),
    containers: kind === "container" ? names.map((n) => container({ name: n })) : [],
  };
}

const rows = (page: Page) => page.getByTestId("backup-row");
const loadMore = (page: Page) => page.getByRole("button", { name: "Load more" });

async function openList(page: Page, staged: Staged, path = "/backups"): Promise<void> {
  await stageEntries(page, staged);
  await page.goto(path);
  await expect(rows(page).first()).toBeVisible();
}

test.skip(({ isMobile }) => !isMobile, "the phone list");

test("the search narrows the list, says when nothing matches and starts the window over", async ({ page }) => {
  await openList(page, long(45));
  await page.getByRole("button", { name: "Search" }).tap();
  const search = page.getByRole("searchbox", { name: "Search" });

  // svc-10 to svc-19: ten rows, fewer than one window.
  await search.fill("svc-1");
  await expect(rows(page)).toHaveCount(10);
  await expect(loadMore(page)).toHaveCount(0);

  await search.fill("zzz-nothing");
  await expect(rows(page)).toHaveCount(0);
  await expect(page.getByText("No items match the current filters.")).toBeVisible();
  await expect(search).toBeVisible();

  await search.fill("");
  await expect(rows(page)).toHaveCount(20);
  await expect(loadMore(page)).toBeVisible();
});

for (const kind of ["container", "vm"] as const) {
  test(`twenty ${kind} rows, forty after Load more, and no button once all are shown`, async ({ page }) => {
    await openList(page, long(40, kind), `/backups?kind=${kind}`);
    await expect(rows(page)).toHaveCount(20);
    await expect(page.getByText(name(39), { exact: true })).toHaveCount(0);

    await loadMore(page).tap();
    await expect(rows(page)).toHaveCount(40);
    await expect(page.getByText(name(39), { exact: true })).toBeVisible();
    await expect(loadMore(page)).toHaveCount(0);
  });
}

test("scrolling to the bottom never grows the list", async ({ page }) => {
  await openList(page, long(45));
  await expect(rows(page)).toHaveCount(20);

  // Three times with time to settle, which a list that loads on scroll
  // would have answered with more rows.
  for (let i = 0; i < 3; i++) {
    await page.locator("#bv-main").evaluate((el) => el.scrollTo(0, el.scrollHeight));
    await page.waitForTimeout(300);
  }
  await expect(rows(page)).toHaveCount(20);
  await expect(loadMore(page)).toBeVisible();
});

test("every row and the button under them are tall enough for a finger", async ({ page }) => {
  await openList(page, long(45));
  const heights = await rows(page).evaluateAll((all) => all.map((el) => el.getBoundingClientRect().height));
  expect(Math.min(...heights)).toBeGreaterThanOrEqual(44);
  expect((await loadMore(page).boundingBox())!.height).toBeGreaterThanOrEqual(44);
});

test("the bar of picked rows stays in view while the list scrolls, and is never fixed", async ({ page }) => {
  await openList(page, long(45));
  const first = rows(page).first();
  const at = (await first.boundingBox())!;
  const point = { clientX: at.x + 60, clientY: at.y + at.height / 2, pointerType: "touch", isPrimary: true, bubbles: true };
  await first.dispatchEvent("pointerdown", point);
  await page.waitForTimeout(650);
  await first.dispatchEvent("pointerup", point);

  const bar = page.getByTestId("bulk-bar");
  await expect(bar).toContainText("1 selected");
  // Fixed would take the bar out of the flow and under the on-screen keyboard.
  await expect(bar).toHaveCSS("position", "sticky");
  await page.locator("#bv-main").evaluate((el) => el.scrollTo(0, el.scrollHeight));
  await expect(bar).toBeInViewport();
});
