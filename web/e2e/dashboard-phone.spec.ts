// The overview's cards on a phone and on the desktop, with a server that has
// something to say: long names, a failed run, an entry nothing backs up, a
// section without an off-site copy. On the phones: one column in one order,
// no pan, nothing past the edge of the screen or of its card, and actions a
// finger can hit. On the desktop: the grid in its default layout, with no
// phone block beside it.
import { expect, test, type Page } from "@playwright/test";

import { FAIL_REASON, LONG_NAME, openOverview, stageOverview } from "./overview";

const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

const PHONE_ORDER = ["needs", "nextRun", "storageGlance", "protection", "targets", "history"];
const DESKTOP_ORDER = ["needs", "protection", "targets", "history"];

function cards(page: Page): Promise<(string | null)[]> {
  return page.locator("[data-grid-block]").evaluateAll((cells) => cells.map((c) => c.getAttribute("data-grid-block")));
}

for (const width of [320, 360]) {
  test(`the overview @ ${width}px stands in one column and nothing pans`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the one-column order is the phone's");
    await stageOverview(page, width);
    await openOverview(page);
    await expect(page.getByText(FAIL_REASON).first()).toBeVisible();
    await expect(page.getByText(LONG_NAME).first()).toBeAttached();

    expect(await cards(page)).toEqual(PHONE_ORDER);

    const layout = await page.evaluate(() => {
      const main = document.querySelector("#bv-main")!;
      const vw = window.innerWidth;
      const name = (el: Element) => (el.getAttribute("aria-label") ?? el.textContent ?? "").trim().slice(0, 40);
      const outside = (el: Element, box: DOMRect) => {
        const r = el.getBoundingClientRect();
        return r.width > 0 && (r.right > box.right + 1 || r.left < box.left - 1);
      };
      const viewport = new DOMRect(0, 0, vw, window.innerHeight);
      const cells = [...main.querySelectorAll("[data-grid-block]")];
      return {
        docPan: document.documentElement.scrollWidth - vw,
        mainPan: main.scrollWidth - main.clientWidth,
        // Every card takes the whole column, whatever a desktop stored.
        widths: cells.map((c) => Math.round(c.getBoundingClientRect().width)),
        pastViewport: [...main.querySelectorAll("button, a, input, [tabindex], span, p")]
          .filter((el) => !el.closest("[data-testid='page-actions']") && outside(el, viewport))
          .map(name),
        pastCard: cells.flatMap((c) => {
          const box = c.getBoundingClientRect();
          return [...c.querySelectorAll("button, a, [tabindex], span, p, li")]
            .filter((el) => outside(el, box))
            .map((el) => `${c.getAttribute("data-grid-block")}: ${name(el)}`);
        }),
      };
    });
    expect.soft(layout.docPan, "the document scrolls horizontally").toBeLessThanOrEqual(1);
    expect.soft(layout.mainPan, "#bv-main scrolls horizontally").toBeLessThanOrEqual(1);
    expect.soft(new Set(layout.widths).size, "the cards differ in width").toBe(1);
    expect.soft(layout.pastViewport, "boxes run past the viewport edge").toEqual([]);
    expect.soft(layout.pastCard, "boxes run past their card").toEqual([]);
    // A backtick in JSX text compiles fine and renders on the page.
    await expect.soft(page.locator("#bv-main")).not.toContainText("`");
  });
}

test("what needs the person on a phone: every action is big enough to tap", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the larger targets follow the coarse pointer");
  await stageOverview(page, 360);
  await openOverview(page);

  const targets = [
    ...(await page.locator("[data-grid-block='needs']").getByRole("button").all()),
    ...(await page.getByTestId("protection-row").all()),
    ...(await page.getByTestId("target-row").all()),
    ...(await page.getByTestId("page-actions").getByRole("button").all()),
  ];
  expect(targets.length, "the staged server leaves nothing to tap").toBeGreaterThan(8);
  for (const target of targets) {
    await target.scrollIntoViewIfNeeded();
    const box = (await target.boundingBox())!;
    expect(box.height, `${(await target.textContent())?.slice(0, 30)} is shorter than a fingertip`).toBeGreaterThanOrEqual(39.5);
  }
  for (const button of await page.locator("[data-grid-block='needs']").getByRole("button").all()) {
    const box = (await button.boundingBox())!;
    expect(box.height, `${await button.textContent()} is shorter than a fingertip`).toBeGreaterThanOrEqual(43.5);
  }
});

test("the page action floats over the rows on the page colour, without a band of its own", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the corner is under the thumb here");
  await stageOverview(page, 360);
  await openOverview(page);

  const corner = page.getByTestId("page-actions");
  const background = await corner.evaluate((el) => getComputedStyle(el).backgroundColor);
  expect(background).toBe("rgba(0, 0, 0, 0)");
  // The corner is as wide as the column, and a tap beside its button reaches
  // the row underneath.
  await expect(corner).toHaveCSS("pointer-events", "none");
});

test("the overview on the desktop keeps its grid, each card once and no phone block", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "desktop-only");
  await stageOverview(page, undefined);
  await openOverview(page);

  expect(await cards(page)).toEqual(DESKTOP_ORDER);
  // The phone's glance blocks are sections under a section label; the cards
  // of the grid are not.
  await expect(page.locator("#bv-main section.glim-notch-card")).toHaveCount(0);
});
