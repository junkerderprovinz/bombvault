// The page of an entry of every kind at phone width. Each kind brings cards
// of its own to the three tabs, and a long German label has to wrap on all of
// them: nothing pans, no control reaches past the edge, the cards keep the
// 24px rhythm of a phone, and what opens over the page stays inside the
// screen. On the desktop the same pages keep the 40px rhythm.
import { expect, test, type Page } from "@playwright/test";
import { EVERY_KIND, MOBILE_PROJECTS, SET_ID, ZFS_ID, overflow, settle, stageEntries } from "./entries";

// A handler can still wait on the real server when its test has passed;
// what it throws then is dropped.
test.afterEach(async ({ page }) => {
  await page.unrouteAll({ behavior: "ignoreErrors" });
});

const ENTRIES = [
  ["a container", "/backups/container/immich", "immich"],
  ["a VM", "/backups/vm/win11", "Windows 11"],
  ["a folder set", `/backups/files/${SET_ID}`, "Fotos"],
  ["a ZFS item", `/backups/zfs/${ZFS_ID}`, "tank/appdata"],
  ["the flash drive", "/backups/flash/flash", "Unraid-Flash"],
  ["the self-backup", "/backups/config/config", "BombVault-Einstellungen"],
] as const;

const TABS = ["", "?tab=settings", "?tab=verify"];

async function open(page: Page, path: string, name: string): Promise<void> {
  await page.goto(path);
  await expect(page.getByRole("heading", { level: 1, name })).toBeAttached();
  await expect(page.getByRole("heading", { level: 2 }).first()).toBeAttached();
  await page.waitForLoadState("networkidle");
  await settle(page);
}

function cardGap(page: Page): Promise<string> {
  return page
    .locator("#bv-main .max-w-6xl")
    .first()
    .evaluate((root) => getComputedStyle(root).rowGap);
}

/** Asserts that what opened over the page, and every button in it, is inside the screen. */
async function expectInsideScreen(page: Page, width: number): Promise<void> {
  const over = page.getByRole("dialog").last();
  await expect(over).toBeVisible();
  await settle(page);
  const box = (await over.boundingBox())!;
  expect(box.x).toBeGreaterThanOrEqual(-1);
  expect(box.x + box.width).toBeLessThanOrEqual(width + 1);
  for (const button of await over.getByRole("button").all()) {
    const at = await button.boundingBox();
    if (at) expect(at.x + at.width, "a button runs past the edge").toBeLessThanOrEqual(width + 1);
  }
}

for (const width of [320, 360]) {
  for (const [what, path, name] of ENTRIES) {
    test(`${what} @ ${width}px: no tab pans or reaches past the edge`, async ({ page }, testInfo) => {
      test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "the phone layout lives below 48rem");
      await stageEntries(page, { ...EVERY_KIND, lang: "de" });
      await page.setViewportSize({ width, height: 800 });

      for (const tab of TABS) {
        await open(page, `${path}${tab}`, name);
        expect(await cardGap(page), `${tab || "the first tab"}: the gap between cards`).toBe("24px");
        const layout = await overflow(page);
        expect(layout.pan, `${tab || "the first tab"} scrolls sideways`).toBeLessThanOrEqual(1);
        expect(layout.past, `${tab || "the first tab"}: controls past the edge`).toEqual([]);
        // A backtick in JSX text compiles and shows on the page.
        await expect(page.locator("#bv-main")).not.toContainText("`");
      }
    });
  }

  test(`a backup of the flash drive @ ${width}px opens with both places and its download inside the screen`, async ({
    page,
  }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "the phone layout lives below 48rem");
    await stageEntries(page, { ...EVERY_KIND, lang: "de" });
    await page.setViewportSize({ width, height: 800 });
    await open(page, "/backups/flash/flash", "Unraid-Flash");

    await page.getByTestId("entry-backup").first().click();
    const window = page.getByRole("dialog", { name: "Unraid-Flash wiederherstellen" });
    await expect(window.getByRole("tab", { name: "Storage Box" })).toBeVisible();
    await expect(window.getByRole("button", { name: "Download (.zip)" })).toBeVisible();
    await expectInsideScreen(page, width);
  });

  test(`deleting a backup of the self-backup @ ${width}px asks inside the screen`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "the phone layout lives below 48rem");
    await stageEntries(page, { ...EVERY_KIND, lang: "de" });
    await page.route(
      (url) => /^\/api\/items\/config\/config\/timeline\/[^/]+\/delete$/.test(url.pathname),
      (route) => route.fulfill({ json: { ok: true, delete: [], others: [] } }),
    );
    await page.setViewportSize({ width, height: 800 });
    await open(page, "/backups/config/config", "BombVault-Einstellungen");

    await page.getByTestId("entry-backup").first().click();
    await page.getByRole("dialog").getByRole("button", { name: "Löschen", exact: true }).click();
    await expectInsideScreen(page, width);
  });

  test(`removing a ZFS item @ ${width}px asks inside the screen`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "the phone layout lives below 48rem");
    await stageEntries(page, { ...EVERY_KIND, lang: "de" });
    await page.setViewportSize({ width, height: 800 });
    await open(page, `/backups/zfs/${ZFS_ID}`, "tank/appdata");

    // The menu beside the name stands before the menu of any backup.
    await page.locator("#bv-main").getByRole("button", { name: "Mehr" }).first().click();
    await page.getByRole("menuitem", { name: "Aus der Liste entfernen" }).click();
    await expect(page.getByRole("dialog").getByRole("radio")).toHaveCount(2);
    await expectInsideScreen(page, width);
    await page.getByRole("dialog").getByRole("button", { name: "Abbrechen" }).click();
    await expect(page.getByRole("dialog")).toHaveCount(0);
  });
}

test("the self-backup on a phone saves its switch as it changes, without a save button", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "the phone layout lives below 48rem");
  await stageEntries(page, { ...EVERY_KIND, lang: "de" });
  let saved = 0;
  await page.route("**/api/settings", (route) => {
    if (route.request().method() !== "PUT") return route.fallback();
    saved += 1;
    return route.fulfill({ json: { ok: true } });
  });
  await open(page, "/backups/config/config?tab=settings", "BombVault-Einstellungen");

  await page.getByRole("switch", { name: "BombVaults Einstellungen sichern" }).click();
  await expect.poll(() => saved).toBeGreaterThan(0);
  await expect(page.locator("#bv-main").getByRole("button", { name: "Speichern" })).toHaveCount(0);
});

for (const [what, path, name] of ENTRIES) {
  test(`${what} on the desktop keeps the 40px rhythm inside the window`, async ({ page }, testInfo) => {
    test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "desktop-only");
    await stageEntries(page, { ...EVERY_KIND, lang: "de" });
    for (const tab of TABS) {
      await open(page, `${path}${tab}`, name);
      expect(await cardGap(page)).toBe("40px");
      expect((await overflow(page)).pan, `${tab || "the first tab"} scrolls sideways`).toBeLessThanOrEqual(1);
    }
  });
}
