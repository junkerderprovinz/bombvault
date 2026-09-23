// The recovery wizard at phone width. The page is a single column already, so
// these checks cover what a phone adds: the 24px card rhythm, nothing panning
// or clipping with every optional section open, and the secret fields' eye and
// the encryption switch staying large enough to hit under a coarse pointer.
// German, because its labels run longest. The desktop half pins the 40px
// rhythm and the unchanged 15px eye.
import { expect, test, type Page } from "@playwright/test";

const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

// Seeded the way narrow-viewport.spec.ts seeds it: the stored locale is the
// look, and the server's display prefs must not overwrite it mid-boot.
async function bootGerman(page: Page, width: number): Promise<void> {
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.addInitScript(() => window.localStorage.setItem("bv-lang", "de"));
  await page.setViewportSize({ width, height: 800 });
  await page.goto("/recovery");
  await expect(page.getByRole("heading", { level: 1, name: "Wiederherstellung" })).toBeVisible();
}

async function cardGap(page: Page): Promise<string> {
  return page
    .getByRole("heading", { level: 1 })
    .locator("xpath=../..")
    .evaluate((root) => getComputedStyle(root).rowGap);
}

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

for (const width of [320, 360]) {
  test(`recovery @ ${width}px: every section open, nothing pans or clips`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
    await bootGerman(page, width);

    await page.getByRole("tab", { name: "Offsite" }).first().click();
    await page.getByRole("button", { name: "Offsite-Kopie (optional)", exact: true }).click();
    await page.getByRole("button", { name: "Cloud-Zugangsdaten (optional)", exact: true }).click();
    await settle(page);

    expect(await cardGap(page)).toBe("24px");

    const layout = await page.evaluate(() => {
      const main = document.querySelector("#bv-main");
      const vw = window.innerWidth;
      const clipped = main
        ? [...main.querySelectorAll("button, a, input, select, textarea, [role='switch']")]
            .filter((el) => {
              const r = el.getBoundingClientRect();
              return r.width > 0 && (r.right > vw + 1 || r.left < -1);
            })
            .map((el) => (el.getAttribute("aria-label") ?? el.textContent ?? "").trim().slice(0, 40))
        : null;
      return {
        docPan: document.documentElement.scrollWidth - vw,
        mainPan: main ? main.scrollWidth - main.clientWidth : null,
        clipped,
      };
    });
    expect(layout.mainPan, "#bv-main is missing").not.toBeNull();
    expect(layout.docPan, "the document scrolls horizontally").toBeLessThanOrEqual(1);
    expect(layout.mainPan!, "#bv-main scrolls horizontally").toBeLessThanOrEqual(1);
    expect(layout.clipped, "controls clip the viewport edge").toEqual([]);
  });
}

test("recovery on a phone: the eye and the encryption switch are big enough to tap", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the larger targets follow the coarse pointer");
  await bootGerman(page, 360);

  const eye = page.getByRole("button", { name: "Wert anzeigen" }).first();
  // touchscreen.tap takes viewport coordinates, so the box is read in view.
  await eye.scrollIntoViewIfNeeded();
  const eyeBox = (await eye.boundingBox())!;
  const fieldBox = (await eye.locator("xpath=preceding-sibling::input").boundingBox())!;
  expect(eyeBox.width, "the eye is narrower than a fingertip").toBeGreaterThanOrEqual(43.5);
  expect(eyeBox.height, "the eye is shorter than its field").toBeGreaterThanOrEqual(fieldBox.height - 0.5);

  // A tap at the strip's inner edge, a finger's width from the glyph, still
  // reaches the eye. Not a corner: the pill's rounded corners are outside its
  // hit area. The tap renames the eye, so it is pinned by an attribute first.
  await eye.evaluate((el) => el.setAttribute("data-probe", ""));
  await page.touchscreen.tap(eyeBox.x + 4, eyeBox.y + eyeBox.height / 2);
  const tapped = page.locator("[data-probe]");
  await expect(tapped).toHaveAttribute("aria-pressed", "true");
  await expect(tapped).toHaveAccessibleName("Wert verbergen");

  const switchBox = (await page.getByRole("switch", { name: "Passwort" }).boundingBox())!;
  expect(switchBox.height, "the switch is shorter than a control").toBeGreaterThanOrEqual(31.5);
});

test("recovery on the desktop keeps the 40px rhythm and the small eye", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "desktop-only");
  await bootGerman(page, testInfo.project.use.viewport!.width);

  expect(await cardGap(page)).toBe("40px");
  const eyeBox = (await page.getByRole("button", { name: "Wert anzeigen" }).first().boundingBox())!;
  expect(Math.round(eyeBox.width)).toBe(15);
  const switchBox = (await page.getByRole("switch", { name: "Passwort" }).boundingBox())!;
  expect(Math.round(switchBox.height)).toBe(20);
});
