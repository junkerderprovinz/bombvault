// The overview's main action, "Back up everything now", on the real binary:
// where it stands, what it asks, and how the run it starts reaches the screen.
//
//   1. On a phone the page reads at a glance: the status head, what needs the
//      person, the next backup and the repositories, then the cards, with the
//      action in the corner under the thumb.
//   2. A tap on the action asks first, and Cancel leaves the server alone.
//   3. A yes starts the pass, and its run opens as a sheet.
//   4. A row of the history opens that run.
//   5. On a desktop the action starts at once and the run opens as a window.
//
// The harness has no repository and no Docker, so what the page reads is
// staged at the route layer (overview.ts), and so is the one write: the start
// is accepted without reaching the server, and the pass's run joins the list
// only after it, because a run the page knew before it asked must never pass
// for the one it started.
import { expect, test, type Page } from "@playwright/test";

import { openOverview, stageOverview } from "./overview";

const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

// A handler can still wait on the real server when its test has passed;
// what it throws then is dropped.
test.afterEach(async ({ page }) => {
  await page.unrouteAll({ behavior: "ignoreErrors" });
});

const START = "Back up everything now";

function action(page: Page) {
  return page.getByTestId("page-actions").getByRole("button", { name: START });
}

async function open(page: Page) {
  const staged = await stageOverview(page, undefined, { lang: "en" });
  await openOverview(page);
  return staged;
}

test("the overview reads at a glance on a phone, with its action under the thumb", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the one-column order is the phone's");
  await open(page);

  await expect(page.getByText("BombVault", { exact: true })).toBeVisible();
  await expect(page.getByTestId("status-head")).toContainText("5 things need you");

  const order = await page.locator("[data-grid-block]").evaluateAll((cells) => cells.map((c) => c.getAttribute("data-grid-block")));
  expect(order).toEqual(["needs", "nextRun", "storageGlance", "protection", "targets", "history"]);

  // The next backup names what runs and when.
  const next = page.locator("[data-grid-block='nextRun']");
  await expect(next.getByText("Containers")).toBeVisible();
  await expect(next.getByText(/^in /)).toBeVisible();

  // The action floats in the corner, inside the window and in its lower half.
  const box = (await action(page).boundingBox())!;
  const viewport = page.viewportSize()!;
  expect(box.y + box.height).toBeLessThanOrEqual(viewport.height);
  expect(box.y).toBeGreaterThan(viewport.height / 2);
  expect(box.x + box.width / 2).toBeGreaterThan(viewport.width / 2);
});

test("a tap on the action asks first, and Cancel leaves the server alone", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: a desktop starts without the question");
  const staged = await open(page);

  await action(page).tap();

  const question = page.getByRole("dialog", { name: "Confirm" });
  await expect(question).toBeVisible();
  await expect(question.getByText(/Containers are stopped and restarted/)).toBeVisible();

  await question.getByRole("button", { name: "Cancel" }).tap();
  await expect(question).toHaveCount(0);
  await page.waitForTimeout(500);
  expect(staged.fired()).toBe(false);
});

test("a yes starts the pass, and its run opens as a sheet", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the sheet is the phone's form of the run window");
  const staged = await open(page);

  await action(page).tap();
  await page.getByRole("dialog", { name: "Confirm" }).getByRole("button", { name: "Full backup" }).tap();

  // The watch polls every two seconds, so the run takes a few to appear.
  const sheet = page.getByRole("dialog", { name: "Backup · Full backup" });
  await expect(sheet).toBeVisible({ timeout: 15_000 });
  await expect(sheet.getByText("Running").first()).toBeVisible();
  expect(staged.fired()).toBe(true);
});

test("a row of the history opens that run", async ({ page }) => {
  await open(page);

  await page.locator("[data-grid-block='history']").getByRole("button", { name: "plex", exact: true }).first().click();

  await expect(page.getByRole("dialog", { name: "Backup · plex" })).toBeVisible();
});

test("on a desktop the action starts at once and the run opens as a window", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "desktop-only: a phone asks first");
  const staged = await open(page);

  await action(page).click();

  const window = page.getByRole("dialog", { name: "Backup · Full backup" });
  await expect(window).toBeVisible({ timeout: 15_000 });
  expect(staged.fired()).toBe(true);
  await expect(page.getByRole("dialog", { name: "Confirm" })).toHaveCount(0);
  // The window is a card over the page, not a sheet from the bottom edge.
  const box = (await window.boundingBox())!;
  const viewport = page.viewportSize()!;
  expect(box.width).toBeLessThan(viewport.width);
  expect(box.y + box.height).toBeLessThan(viewport.height);
  // While the pass runs the action leads to its run.
  await expect(action(page)).toHaveCount(0);
  await expect(page.getByTestId("page-actions").getByRole("button", { name: "Backing up now" })).toBeVisible();
});
