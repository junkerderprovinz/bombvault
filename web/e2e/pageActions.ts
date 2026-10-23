// The page actions on a phone. They stand on a band of the page's colour, so
// no row shows in pieces around the buttons, and the band is no taller than
// the scroll padding the scroller keeps, so a control brought into view rests
// above it in full.
import { expect, type Locator, type Page } from "@playwright/test";

/** Nothing but the band answers anywhere on it, and it fits the padding. */
export async function expectPageActionsCoverNothing(page: Page): Promise<void> {
  const found = await page.evaluate(() => {
    const bar = document.querySelector<HTMLElement>("[data-testid='page-actions']")!;
    const r = bar.getBoundingClientRect();
    const through: string[] = [];
    for (let x = r.left + 2; x < r.right - 1; x += 6) {
      for (const y of [r.top + 1, r.top + r.height / 2, r.bottom - 1]) {
        const hit = document.elementFromPoint(x, y);
        if (hit && !bar.contains(hit)) through.push(`${Math.round(x)},${Math.round(y)} ${(hit.textContent ?? hit.tagName).trim().slice(0, 30)}`);
      }
    }
    const main = document.querySelector<HTMLElement>("#bv-main")!;
    return { through, height: r.height, padding: parseFloat(getComputedStyle(main).scrollPaddingBottom) };
  });
  expect(found.through, "the page shows around the page actions").toEqual([]);
  expect(found.height, "the page actions are taller than the scroll padding").toBeLessThanOrEqual(found.padding + 0.5);
}

/** Scrolls a control into view from below and checks it rests above the band. */
export async function expectClearOfPageActions(page: Page, control: Locator): Promise<void> {
  await control.evaluate((el) => el.scrollIntoView({ block: "end" }));
  const box = (await control.boundingBox())!;
  const bar = (await page.getByTestId("page-actions").boundingBox())!;
  expect(box.y + box.height, "the control rests under the page actions").toBeLessThanOrEqual(bar.y + 0.5);
}
