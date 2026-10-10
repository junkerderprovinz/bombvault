// The Storage locations page and the page of one location. A fresh harness
// database has one local folder and nothing off-site, so a location of every
// kind is staged at the route layer. On the phones: nothing pans and nothing
// runs past the edge, on the list and on every location's page. On the
// desktop: a row opens its location, a shorter rule asks before it is saved,
// and a section goes back to its location's value. German, because its labels
// run longest.
import { expect, test, type Page } from "@playwright/test";
import { B2, LOCAL, LOCATIONS, REST, STORAGE_BOX, measureLayout, settle, stageStorage } from "./storage";

const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

// A test can pass while a handler still waits on the real server. Closing the
// context under it disposes the response it is about to read, and Playwright
// reports that as a failure of the test that already passed.
test.afterEach(async ({ page }) => {
  await page.unrouteAll({ behavior: "ignoreErrors" });
});

function locationUrl(id: string): string {
  return `/storage/${encodeURIComponent(id)}`;
}

async function expectNothingPansOrClips(page: Page): Promise<void> {
  const layout = await measureLayout(page);
  expect(layout.mainPan, "#bv-main is missing").not.toBeNull();
  expect.soft(layout.docPan, "the document scrolls horizontally").toBeLessThanOrEqual(1);
  expect.soft(layout.mainPan!, "#bv-main scrolls horizontally").toBeLessThanOrEqual(1);
  expect.soft(layout.clipped, "controls clip the viewport edge").toEqual([]);
  expect.soft(layout.overflowing, "text runs past the viewport edge").toEqual([]);
}

for (const width of [320, 390]) {
  test(`storage locations @ ${width}px: the list fits`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone layout lives below 48rem");
    await stageStorage(page, width);
    await page.goto("/storage");
    await expect(page.getByRole("link", { name: STORAGE_BOX.name })).toBeVisible();
    await settle(page);
    await expect(page.getByRole("listitem").filter({ has: page.getByRole("link") })).toHaveCount(LOCATIONS.length);
    await expectNothingPansOrClips(page);
  });

  for (const location of LOCATIONS) {
    test(`storage location ${location.id} @ ${width}px fits`, async ({ page }, testInfo) => {
      test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone layout lives below 48rem");
      await stageStorage(page, width);
      await page.goto(locationUrl(location.id));
      await expect(page.getByRole("heading", { level: 1, name: location.name })).toBeAttached();
      await expect(page.getByText("Wie lange aufheben")).toBeVisible();
      await settle(page);
      await expectNothingPansOrClips(page);
    });
  }
}

test.describe("on the desktop", () => {
  test.beforeEach(async ({ page }, testInfo) => {
    test.skip(testInfo.project.name !== "desktop-1280", "one desktop project is enough for the behaviour");
    await page.setViewportSize({ width: 1280, height: 900 });
  });

  test("a row opens its location, and a location that is gone says so", async ({ page }) => {
    await stageStorage(page, 1280);
    await page.goto("/storage");
    const row = page.getByRole("listitem").filter({ has: page.getByRole("link", { name: B2.name }) });
    await expect(row).toContainText("Genutzt von Ordner");
    await expect(row).toContainText("Größe unbekannt");
    // Anywhere on the row, not only on the name.
    await row.click();
    await expect(page).toHaveURL(new RegExp(`${encodeURIComponent(B2.id)}$`));
    await expect(page.getByText("Zugangsdaten")).toBeVisible();

    await page.goto(locationUrl("destination:gone"));
    await expect(page.getByText("Diesen Speicherort gibt es nicht mehr.")).toBeVisible();
  });

  test("a shorter rule asks first, and the answer can switch the question off", async ({ page }) => {
    const staged = await stageStorage(page, 1280);
    await page.goto(locationUrl(STORAGE_BOX.id));
    const keep = page.getByRole("radiogroup", { name: "Wie lange aufheben" });
    await expect(keep.getByRole("radio", { name: "Lang" })).toBeChecked();

    await keep.getByRole("radio", { name: "Kurz" }).click();
    const dialog = page.getByRole("dialog");
    await expect(dialog).toContainText("Weniger aufheben?");
    expect(staged.writes).toEqual([]);
    await dialog.getByRole("button", { name: "Zurück" }).click();
    await expect(keep.getByRole("radio", { name: "Lang" })).toBeChecked();
    expect(staged.writes).toEqual([]);

    await keep.getByRole("radio", { name: "Kurz" }).click();
    await page.getByRole("dialog").getByRole("switch", { name: "Nicht mehr fragen" }).click();
    await page.getByRole("dialog").getByRole("button", { name: "Trotzdem speichern" }).click();
    await expect.poll(() => staged.writes.length).toBe(1);
    expect(staged.writes[0]).toMatchObject({
      method: "PUT",
      path: "/api/offsite/destinations/box",
      body: { retention: { keepLast: 0, keepDaily: 7, keepWeekly: 4, keepMonthly: 3, keepYearly: 0 } },
    });
    await expect(page.getByRole("switch", { name: "Vor dem Verkürzen nachfragen" })).not.toBeChecked();
  });

  test("a section with a value of its own goes back to the location's, the primary copy cannot", async ({ page }) => {
    const staged = await stageStorage(page, 1280);
    await page.goto(locationUrl(STORAGE_BOX.id));
    const reset = page.getByRole("button", { name: "Auf den Speicherort zurücksetzen" });
    // Flash holds retention, compression and limits. Containers is the primary
    // copy and holds all four without a way back.
    await expect(reset).toHaveCount(3);
    await reset.first().click();
    await expect.poll(() => staged.writes.length).toBe(1);
    expect(staged.writes[0]).toMatchObject({ method: "PUT", path: "/api/offsite/targets/t-box-flash" });
    expect((staged.writes[0].body as { follow: string[] }).follow).toHaveLength(1);
  });

  test("a folder on this host has no copies, credentials or delete protection to set", async ({ page }) => {
    await stageStorage(page, 1280);
    await page.goto(locationUrl(LOCAL.id));
    await expect(page.getByText("Wie lange aufheben")).toBeVisible();
    await expect(page.getByText("Verbindung", { exact: true })).toBeVisible();
    for (const absent of ["Kopieren", "Zugangsdaten", "Löschschutz", "rclone.conf"]) {
      await expect(page.getByText(absent, { exact: true })).toHaveCount(0);
    }
  });

  test("the tamper test runs against the location's own target", async ({ page }) => {
    const staged = await stageStorage(page, 1280);
    await page.goto(locationUrl(REST.id));
    await expect(page.getByText("Zuletzt: Löschen verweigert", { exact: false })).toBeVisible();
    await page.getByRole("button", { name: "Testen", exact: true }).click();
    await expect.poll(() => staged.writes.map((w) => w.path)).toEqual(["/api/offsite/targets/t-rest/tamper-test"]);
    await expect(page.getByRole("button", { name: "Bestanden" })).toBeVisible();
  });
});
