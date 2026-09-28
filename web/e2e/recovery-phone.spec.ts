// The recovery wizard at phone width. The page is a single column already, so
// these checks cover what a phone adds: the 24px card rhythm, nothing panning,
// clipping or cut short on a fresh install and with the Self-Backup's copy and
// the domain rows of the storage places on screen, the add-place window inside
// the screen, and the secret fields' eye and the encryption switch staying
// large enough to hit under a coarse pointer. German, because its labels run
// longest. The desktop half pins the 40px rhythm and the unchanged 15px eye.
import { expect, test, type Page } from "@playwright/test";
import { CONFIG_TARGETS, stagePlaces } from "./places";

const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

// Seeded the way narrow-viewport.spec.ts seeds it: the stored locale is the
// look, and the server's display prefs must not overwrite it mid-boot.
async function bootGerman(page: Page, width: number, { places = false } = {}): Promise<void> {
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.addInitScript(() => window.localStorage.setItem("bv-lang", "de"));
  if (places) await stagePlaces(page);
  await page.setViewportSize({ width, height: 800 });
  await page.goto("/recovery");
  await expect(page.getByRole("heading", { level: 1, name: "Wiederherstellung" })).toBeVisible();
  // Step 3's domain rows land after the heading and move everything below.
  await page.waitForLoadState("networkidle");
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

/** What must not show on the page or in `scope`: a pan, a control past the
 *  viewport or past the rounded surface around it, and text cut short by an
 *  ellipsis. A picker's value may end in one; its open list shows it whole. */
async function layout(page: Page, scope: string) {
  return page.evaluate((selector) => {
    const main = document.querySelector("#bv-main");
    const root = document.querySelector(selector);
    const vw = window.innerWidth;
    const name = (el: Element) => (el.getAttribute("aria-label") ?? el.textContent ?? "").trim().slice(0, 40);
    const controls = root ? [...root.querySelectorAll("button, a, input, select, textarea, [role='switch']")] : [];
    return {
      docPan: document.documentElement.scrollWidth - vw,
      mainPan: main ? main.scrollWidth - main.clientWidth : null,
      clipped: controls
        .filter((el) => {
          const r = el.getBoundingClientRect();
          return r.width > 0 && (r.right > vw + 1 || r.left < -1);
        })
        .map(name),
      pastSurface: controls
        .filter((el) => {
          const around = el.parentElement?.closest(".rounded-card")?.getBoundingClientRect();
          const r = el.getBoundingClientRect();
          return around && r.width > 0 && (r.right > around.right + 1 || r.left < around.left - 1);
        })
        .map(name),
      cut: root
        ? [...root.querySelectorAll("*")]
            .filter((el) => !el.closest("[role='combobox']"))
            .filter((el) => getComputedStyle(el).textOverflow === "ellipsis" && el.scrollWidth > el.clientWidth + 1)
            .map(name)
        : [],
    };
  }, scope);
}

async function expectFits(page: Page, scope = "#bv-main"): Promise<void> {
  const found = await layout(page, scope);
  expect(found.mainPan, "#bv-main is missing").not.toBeNull();
  expect(found.docPan, "the document scrolls horizontally").toBeLessThanOrEqual(1);
  expect(found.mainPan!, "#bv-main scrolls horizontally").toBeLessThanOrEqual(1);
  expect(found.clipped, "controls clip the viewport edge").toEqual([]);
  expect(found.pastSurface, "controls stick out of their card or row").toEqual([]);
  expect(found.cut, "text is cut short").toEqual([]);
}

for (const width of [320, 360, 390]) {
  test(`recovery @ ${width}px: every section open, nothing pans, clips or is cut short`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
    await bootGerman(page, width, { places: true });

    // Step 2 restores the settings from the Self-Backup's copy at another
    // site, and step 3 lists where each domain's backups lie.
    await page.getByRole("tab", { name: "Kopiert nach" }).first().click();
    await expect(page.getByText(CONFIG_TARGETS[0].repo, { exact: true })).toBeVisible();
    await page.getByRole("region", { name: "Container", exact: true }).getByRole("button", { name: "2 Einträge" }).click();
    await settle(page);

    expect(await cardGap(page)).toBe("24px");
    await expectFits(page);
  });

  test(`recovery @ ${width}px on a fresh install: nothing pans, clips or is cut short`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
    await bootGerman(page, width);
    await settle(page);

    expect(await cardGap(page)).toBe("24px");
    await expectFits(page);
  });

  test(`recovery @ ${width}px: the add-place window stays inside the screen`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
    await bootGerman(page, width, { places: true });

    await page.locator("#bv-main").getByRole("button", { name: "Ort hinzufügen" }).click();
    const dialog = page.getByRole("dialog", { name: "Ort hinzufügen" });
    await expect(dialog.getByRole("option").first()).toBeVisible();
    await settle(page);
    await dialog.evaluate((el) => el.setAttribute("data-probe", ""));
    await expectFits(page, "[data-probe]");
    const box = (await dialog.boundingBox())!;
    expect(box.x, "the window starts past the left edge").toBeGreaterThanOrEqual(0);
    expect(box.x + box.width, "the window ends past the right edge").toBeLessThanOrEqual(width);
    expect(box.y + box.height, "the window runs past the bottom").toBeLessThanOrEqual(800);
  });
}

test("recovery on a phone: the eye and the encryption switch are big enough to tap", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the larger targets follow the coarse pointer");
  await bootGerman(page, 360);

  const eye = page.getByRole("button", { name: "Wert anzeigen" }).first();
  // touchscreen.tap takes viewport coordinates, so the box is read in view,
  // and only once the page has stopped sliding in, or the tap can land
  // beside the eye.
  await eye.scrollIntoViewIfNeeded();
  await eye.evaluate(async (el) => {
    const frame = () => new Promise((resolve) => requestAnimationFrame(resolve));
    let last = "";
    for (let still = 0; still < 3; ) {
      await frame();
      const r = el.getBoundingClientRect();
      const now = `${r.left},${r.top},${r.width},${r.height}`;
      still = now === last ? still + 1 : 0;
      last = now;
    }
  });
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
