// The Storage tab's places and domain rows at phone width, staged with the
// longest names and addresses a place brings. On the phones: every place's
// details and every row's exceptions open with nothing panning, clipping, cut
// short or too small to tap; a place's actions under its name; a folder name
// under its switch at a width that can be read; the add window's tiles filling
// their rows and its form's buttons stacked like a sheet's answers; and a
// place's removal asked in a sheet. On the desktop: the 40px rhythm, the
// actions beside the name, one-line chips and one row of window buttons.
// German, because its labels run longest.
import { expect, test, type Page } from "@playwright/test";
import { PLACES, stagePlaces } from "./places";

const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

// A settings read the page makes as the test ends would otherwise fail it
// from inside a route handler whose response is already gone.
test.afterEach(async ({ page }) => {
  await page.unrouteAll({ behavior: "ignoreErrors" });
});

async function settle(page: Page): Promise<void> {
  await page.evaluate(() =>
    Promise.all(
      document
        .getAnimations()
        .filter((a) => a.effect?.getTiming().iterations !== Infinity)
        .map((a) => a.finished.catch(() => {})),
    ),
  );
}

async function boot(page: Page, width: number): Promise<void> {
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.addInitScript(() => {
    window.localStorage.setItem("bv-lang", "de");
    window.localStorage.setItem("bombvault.advanced", "1");
  });
  // Every domain on, so each row and each folder switch is there.
  await page.route("**/api/settings", async (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    const response = await route.fetch();
    const body = await response.json();
    const on = { vmsEnabled: true, flashEnabled: true, configEnabled: true, filesEnabled: true, zfsEnabled: true };
    await route.fulfill({ response, json: { ...body, platform: "unraid", settings: { ...body.settings, ...on } } });
  });
  await stagePlaces(page);
  await page.setViewportSize({ width, height: 800 });
  await page.goto("/settings#storage");
  await expect(page.getByRole("tab", { name: "Speicher", selected: true })).toBeVisible();
  await page.waitForLoadState("networkidle");
  await settle(page);
}

function placeRow(page: Page, name: string) {
  return page
    .locator("#bv-main div.rounded-card.glim-hue")
    .filter({ hasText: name })
    .filter({ has: page.getByRole("button", { name: "Testen", exact: true }) })
    .last();
}

async function openEverything(page: Page): Promise<void> {
  for (const { name } of PLACES) await placeRow(page, name).getByRole("button", { name: "Details", exact: true }).click();
  await page.getByRole("region", { name: "Container", exact: true }).getByRole("button", { name: "2 Einträge" }).click();
  await settle(page);
}

/** What must not show inside `scope`: a pan, a control past the viewport or
 *  past the rounded surface around it, text cut short by an ellipsis, a
 *  control under WCAG's 24px, and a stray backtick. A picker's value may end in
 *  an ellipsis; its open list shows it whole. */
async function expectFits(page: Page, scope: string): Promise<void> {
  const found = await page.evaluate((selector) => {
    const root = document.querySelector(selector);
    if (!root) return null;
    const vw = window.innerWidth;
    const name = (el: Element) => (el.getAttribute("aria-label") ?? el.textContent ?? "").trim().replace(/\s+/g, " ").slice(0, 40);
    const controls = [...root.querySelectorAll("button, a, input, select, textarea, [role='switch'], [role='combobox']")].filter(
      (el) => el.getBoundingClientRect().width > 1,
    );
    return {
      docPan: document.documentElement.scrollWidth - vw,
      scopePan: root.scrollWidth - root.clientWidth,
      pastViewport: controls
        .filter((el) => {
          const r = el.getBoundingClientRect();
          return r.right > vw + 1 || r.left < -1;
        })
        .map(name),
      pastSurface: controls
        .filter((el) => {
          const around = el.parentElement?.closest(".rounded-card")?.getBoundingClientRect();
          const r = el.getBoundingClientRect();
          return around && (r.right > around.right + 1 || r.left < around.left - 1);
        })
        .map(name),
      cutShort: [...root.querySelectorAll("*")]
        .filter((el) => !el.closest("[role='combobox']") && el.clientWidth > 1)
        .filter((el) => getComputedStyle(el).textOverflow === "ellipsis" && el.scrollWidth > el.clientWidth + 1)
        .map(name),
      tooSmall: controls
        .filter((el) => !(el.tagName === "A" && getComputedStyle(el).display === "inline"))
        .filter((el) => {
          const r = el.getBoundingClientRect();
          return r.width < 23.5 || r.height < 23.5;
        })
        .map(name),
      backtick: (root.textContent ?? "").includes("`"),
    };
  }, scope);
  expect(found, `${scope} is missing`).not.toBeNull();
  expect.soft(found!.docPan, "the document scrolls horizontally").toBeLessThanOrEqual(1);
  expect.soft(found!.scopePan, `${scope} scrolls horizontally`).toBeLessThanOrEqual(1);
  expect.soft(found!.pastViewport, "controls clip the viewport edge").toEqual([]);
  expect.soft(found!.pastSurface, "controls stick out of their card or row").toEqual([]);
  expect.soft(found!.cutShort, "text is cut short").toEqual([]);
  expect.soft(found!.tooSmall, "controls under 24px").toEqual([]);
  expect.soft(found!.backtick, "a stray backtick renders").toBe(false);
}

async function openAddWindow(page: Page) {
  await page.locator("#bv-main").getByRole("button", { name: "Ort hinzufügen" }).click();
  const dialog = page.getByRole("dialog", { name: "Ort hinzufügen" });
  await expect(dialog.getByRole("option").first()).toBeVisible();
  await settle(page);
  await dialog.evaluate((el) => el.setAttribute("data-probe", ""));
  return dialog;
}

for (const width of [320, 360, 390]) {
  test(`storage @ ${width}px: every place and row open, nothing pans, clips or is cut short`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
    await boot(page, width);
    await openEverything(page);

    await expectFits(page, "#bv-main");

    // The actions take their own row, so a long name keeps the width it needs.
    for (const { name } of PLACES) {
      const row = placeRow(page, name);
      const nameBox = (await row.getByText(name, { exact: true }).boundingBox())!;
      const actionBox = (await row.getByRole("button", { name: "Details schließen" }).boundingBox())!;
      expect(actionBox.y, `the actions of "${name}" share the name's row`).toBeGreaterThanOrEqual(nameBox.y + nameBox.height);
    }

    // A folder name sits under its switch rather than in the sliver beside it.
    const widths = await page.getByRole("textbox", { name: /^Ordner für / }).evaluateAll((els) => els.map((el) => el.getBoundingClientRect().width));
    expect(widths.length, "no place lists its folders").toBeGreaterThan(0);
    for (const w of widths) expect(w, "a folder field is too narrow to read").toBeGreaterThanOrEqual(160);
  });

  test(`storage @ ${width}px: the add window's tiles fill their rows and its form fits`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
    await boot(page, width);
    const dialog = await openAddWindow(page);
    await expectFits(page, "[data-probe]");

    const rows = await dialog.getByRole("group").first().evaluate((group) => {
      const grid = group.querySelector(".grid")!;
      const tiles = [...grid.children].map((tile) => tile.getBoundingClientRect());
      const first = tiles.filter((b) => Math.round(b.top) === Math.round(tiles[0].top));
      const used = first.reduce((sum, b) => sum + b.width, 0) + (first.length - 1) * 8;
      return { perRow: first.length, spare: grid.getBoundingClientRect().width - used, side: Math.min(...tiles.map((b) => b.height)) };
    });
    expect(rows.spare, "the tiles leave a margin beside the row").toBeLessThanOrEqual(1);
    expect(rows.perRow, "tiles per row").toBe(width >= 360 ? 3 : 2);
    expect(rows.side, "a tile is too small for its mark and name").toBeGreaterThanOrEqual(87.5);

    await dialog.getByRole("option", { name: "Wasabi" }).click();
    await expect(dialog.getByRole("button", { name: "Verbindung testen" })).toBeVisible();
    await settle(page);
    await expectFits(page, "[data-probe]");
    const buttons = await dialog
      .getByRole("button", { name: /^(Zurück|Abbrechen|Verbindung testen|Hinzufügen)$/ })
      .evaluateAll((els) => els.map((el) => el.getBoundingClientRect()).map((b) => ({ width: b.width, top: b.top })));
    expect(buttons).toHaveLength(4);
    for (let i = 1; i < buttons.length; i++) {
      expect(buttons[i].top, "two buttons share a row").toBeGreaterThan(buttons[i - 1].top);
      expect(Math.abs(buttons[i].width - buttons[0].width), "the buttons differ in width").toBeLessThanOrEqual(1);
    }
  });
}

test("storage on a phone: removing a place is asked in a sheet inside the screen", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only");
  await boot(page, 320);
  await placeRow(page, PLACES[1].name).getByRole("button", { name: "Entfernen", exact: true }).click();
  const sheet = page.getByRole("dialog").filter({ hasText: PLACES[1].name });
  await expect(sheet).toBeVisible();
  await settle(page);
  await sheet.evaluate((el) => el.setAttribute("data-probe", ""));
  await expectFits(page, "[data-probe]");
  for (const answer of ["Abbrechen", "Entfernen"]) {
    const box = (await sheet.getByRole("button", { name: answer, exact: true }).boundingBox())!;
    expect(box.x, `${answer} starts past the left edge`).toBeGreaterThanOrEqual(0);
    expect(box.x + box.width, `${answer} ends past the right edge`).toBeLessThanOrEqual(320);
  }
  await sheet.getByRole("button", { name: "Abbrechen" }).click();
  await expect(sheet).toBeHidden();
});

test("storage on the desktop keeps the 40px rhythm, the actions beside the name and one-line chips", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "desktop-only");
  await boot(page, testInfo.project.use.viewport!.width);

  const gap = await page
    .getByRole("heading", { level: 1 })
    .locator("xpath=../../..")
    .evaluate((root) => getComputedStyle(root).rowGap);
  expect(gap).toBe("40px");

  // At 768px a place's four lines leave its actions too little room beside
  // them; at 1280px there is room.
  if (testInfo.project.use.viewport!.width >= 1280) {
    const row = placeRow(page, PLACES[0].name);
    const nameBox = (await row.getByText(PLACES[0].name, { exact: true }).boundingBox())!;
    const detailsBox = (await row.getByRole("button", { name: "Details", exact: true }).boundingBox())!;
    expect(detailsBox.x, "the actions left the name's side").toBeGreaterThan(nameBox.x + nameBox.width);
  }

  const heights = await page
    .getByRole("region", { name: "Container", exact: true })
    .getByRole("group", { name: "Kopiert nach" })
    .getByRole("button")
    .evaluateAll((chips) => [...new Set(chips.map((chip) => Math.round(chip.getBoundingClientRect().height)))]);
  expect(heights, "a chip grew a second line").toHaveLength(1);

  const dialog = await openAddWindow(page);
  await dialog.getByRole("option", { name: "Wasabi" }).click();
  const tops = await dialog
    .getByRole("button", { name: /^(Zurück|Abbrechen|Verbindung testen|Hinzufügen)$/ })
    .evaluateAll((els) => [...new Set(els.map((el) => Math.round(el.getBoundingClientRect().top)))]);
  expect(tops, "the window's buttons wrapped").toHaveLength(1);
});
