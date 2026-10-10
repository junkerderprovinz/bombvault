// The Storage locations page and the page of one location. A fresh harness
// database has one local folder and nothing off-site, so a location of every
// kind is staged at the route layer. On the phones: nothing pans and nothing
// runs past the edge, on the list with its picture and its switches and on
// every location's page. On the desktop: a row opens its location, a line of
// the picture switches a section's default after its question, an entry goes
// back to its default, a location is added in one window, a shorter rule asks
// before it is saved, and a section gets rules of its own and gives them
// back. German, because its labels run longest.
import { expect, test, type Page } from "@playwright/test";
import { B2, IMPACT, LOCAL, LOCATIONS, OWN_ENTRY, REST, STORAGE_BOX, measureLayout, settle, stageStorage } from "./storage";

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
    await expect(page.getByRole("link", { name: STORAGE_BOX.name, exact: true })).toBeVisible();
    await settle(page);
    await expect(page.getByRole("listitem").filter({ has: page.getByRole("link", { name: STORAGE_BOX.name, exact: true }) })).toHaveCount(1);
    await expect(page.getByRole("switch", { name: `Container nach ${STORAGE_BOX.name}` })).toBeAttached();
    await expect(page.getByRole("link", { name: OWN_ENTRY })).toBeVisible();
    await expectNothingPansOrClips(page);
  });

  test(`storage locations @ ${width}px: the switches fit`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone layout lives below 48rem");
    await stageStorage(page, width);
    await page.goto("/storage");
    await page.getByRole("radio", { name: "Schalter" }).click();
    await expect(page.getByRole("group", { name: "Container" })).toBeVisible();
    await settle(page);
    await expectNothingPansOrClips(page);
  });

  for (const location of LOCATIONS) {
    test(`storage location ${location.id} @ ${width}px fits`, async ({ page }, testInfo) => {
      test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone layout lives below 48rem");
      await stageStorage(page, width);
      await page.goto(locationUrl(location.id));
      await expect(page.getByRole("heading", { level: 1, name: location.name })).toBeAttached();
      await expect(page.getByText("Wie lange aufheben", { exact: true })).toBeVisible();
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

  test("a line switches a section's default after saying what the place will receive", async ({ page }) => {
    const staged = await stageStorage(page, 1280);
    await page.goto("/storage");
    const on = page.getByRole("switch", { name: `Container nach ${STORAGE_BOX.name}` });
    await expect(on).toBeChecked();
    const off = page.getByRole("switch", { name: `Container nach ${B2.name}` });
    await expect(off).not.toBeChecked();

    await off.focus();
    await page.keyboard.press("Enter");
    const dialog = page.getByRole("dialog");
    await expect(dialog).toContainText(`${B2.name} ab jetzt bekommt: 7`);
    expect(staged.writes.filter((w) => w.method === "PUT")).toEqual([]);
    await dialog.getByRole("button", { name: "Bestätigen" }).click();

    await expect.poll(() => staged.writes.filter((w) => w.method === "PUT").length).toBe(1);
    expect(staged.writes.map((w) => `${w.method} ${w.path}`)).toEqual([
      "POST /api/offsite/destinations/b2/domains/containers",
      "POST /api/placement/default/containers/preview",
      "PUT /api/placement/default/containers",
    ]);
    expect(staged.writes[2].body).toMatchObject({ skip: [], expect: IMPACT });
  });

  test("a line the server would refuse is no switch, and the choice of view survives a reload", async ({ page }) => {
    const staged = await stageStorage(page, 1280);
    await page.goto("/storage");
    const fixed = page.getByRole("switch", { name: "Flash nach bombvault" });
    await expect(fixed).toBeDisabled();
    await expect(page.getByLabel("Flash sichert immer in den eigenen Ordner.")).toBeVisible();
    await fixed.click({ force: true });
    expect(staged.writes).toEqual([]);

    await page.getByRole("radio", { name: "Schalter" }).click();
    await expect(page.getByRole("group", { name: "Flash" }).getByRole("button", { name: "bombvault" })).toBeDisabled();
    await page.reload();
    await expect(page.getByRole("radio", { name: "Schalter" })).toBeChecked();
    await expect(page.getByRole("group", { name: "Container" }).getByRole("button", { name: STORAGE_BOX.name })).toHaveAttribute("aria-pressed", "true");
  });

  test("an entry with its own choice goes back to its section's default after a question", async ({ page }) => {
    const staged = await stageStorage(page, 1280);
    await page.goto("/storage");
    const row = page.getByRole("listitem").filter({ has: page.getByRole("link", { name: OWN_ENTRY }) });
    await expect(row).toContainText(`Sichert nach Lokal · Vorgabe: Lokal und ${STORAGE_BOX.name}`);
    await expect(page.getByRole("link", { name: "gitea" })).toHaveCount(0);
    await expect(row.getByRole("link", { name: OWN_ENTRY })).toHaveAttribute("href", `/backups/container/${OWN_ENTRY}`);

    await row.getByRole("button", { name: "Vorgabe übernehmen" }).click();
    const dialog = page.getByRole("dialog");
    await expect(dialog).toContainText(`Neue Sicherungen gehen dann nach Lokal und ${STORAGE_BOX.name}`);
    await expect(dialog).toContainText(`${STORAGE_BOX.name}: etwa 12`);
    expect(staged.writes).toEqual([]);
    await dialog.getByRole("button", { name: "Vorgabe übernehmen" }).click();

    await expect.poll(() => staged.writes.length).toBe(1);
    expect(staged.writes[0]).toMatchObject({
      method: "PATCH",
      path: `/api/containers/${OWN_ENTRY}`,
      body: { home: { follow: true }, copies: { follow: true } },
    });
  });

  test("a storage location is added in one window, from the provider to the sections that use it", async ({ page }) => {
    const staged = await stageStorage(page, 1280);
    await page.goto("/storage");
    await page.getByTestId("page-actions").getByRole("button", { name: "Speicherort hinzufügen" }).click();
    const window = page.getByRole("dialog", { name: "Speicherort hinzufügen" });
    await expect(window.getByText("Aus deiner Gruppe")).toBeVisible();
    // A folder on this host comes first among the providers.
    await expect(window.locator(".glim-provider-pick button").nth(1)).toContainText("Lokaler Ordner");

    await window.getByRole("button", { name: "Suchen" }).click();
    await window.getByRole("searchbox").fill("rest");
    await expect(window.locator(".glim-provider-pick button")).toHaveCount(1);
    await window.getByRole("button", { name: /^Rest server/ }).click();

    await expect(window.getByRole("radio", { name: "Ordner" })).toBeDisabled();
    await window.getByLabel("Adresse").fill("https://nas.local:8000/tower");
    await window.getByRole("button", { name: "Verbindung testen" }).click();
    await expect(window.getByText("Verbindung steht", { exact: false }).first()).toBeVisible();
    await window.getByRole("button", { name: "Weiter" }).click();
    await expect(window.getByText("Schutz vor Löschen")).toBeVisible();
    await window.getByRole("button", { name: "Weiter" }).click();

    await expect(window.getByText("Welche Bereiche sichern hierher?")).toBeVisible();
    await expect(window.getByText("7 Einträge · bis zu 32 Sicherungen").first()).toBeVisible();
    for (const section of ["VMs", "Ordner", "Selbst-Backup"]) {
      await window.getByRole("switch", { name: section }).click();
    }
    expect(staged.writes).toEqual([]);
    await window.getByRole("button", { name: "Speicherort speichern" }).click();

    await expect(page).toHaveURL(/destination%3Anew$/);
    expect(staged.writes.map((w) => `${w.method} ${w.path}`)).toEqual([
      "POST /api/offsite/destinations",
      "POST /api/offsite/destinations/new/domains/containers",
      "POST /api/placement/default/containers/preview",
      "PUT /api/placement/default/containers",
      "POST /api/offsite/destinations/new/domains/flash",
    ]);
    expect(staged.writes[0].body).toMatchObject({ provider: "rest", settings: { url: "https://nas.local:8000/tower" } });
  });

  test("a section gets rules of its own in a window and gives them back there", async ({ page }) => {
    const staged = await stageStorage(page, 1280);
    await page.goto(locationUrl(STORAGE_BOX.id));
    const rules = page.getByText("Eigene Regeln: Wie lange aufheben, Kompression und Kopieren");
    await expect(rules).toBeVisible();
    await page.getByRole("button", { name: "Regeln ändern" }).first().click();
    const window = page.getByRole("dialog", { name: "Regeln für Flash" });
    await expect(window.getByText("rclone:storagebox:bombvault/flash")).toBeVisible();

    await window.getByRole("radiogroup", { name: "Kompression" }).getByRole("radio", { name: "Aus" }).click();
    await expect.poll(() => staged.writes.length).toBe(1);
    expect(staged.writes[0]).toMatchObject({ method: "PUT", path: "/api/offsite/targets/t-box-flash", body: { compression: "off" } });

    await window.getByRole("button", { name: "Auf den Speicherort zurücksetzen" }).click();
    await expect.poll(() => staged.writes.length).toBe(2);
    expect((staged.writes[1].body as { follow: string[] }).follow).toEqual(["retention", "compression", "limits"]);
    await window.getByRole("button", { name: "Fertig" }).click();
    await expect(page.getByRole("dialog")).toHaveCount(0);
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
    await expect(page.getByText("Wie lange aufheben", { exact: true })).toBeVisible();
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
