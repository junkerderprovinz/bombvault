// The overview in a real browser: the grid of six columns, what the customize
// mode does to it with a real pointer, and the windows its cards open. What
// the page reads is staged at the route layer (overview.ts).
import { expect, test, type Locator, type Page } from "@playwright/test";

import {
  DRILL_DETAIL,
  EVERYTHING_RUN,
  FAILING_BOX,
  ITEMS,
  RUNS,
  UNPROTECTED_NAME,
  openOverview,
  settle,
  stageOverview,
} from "./overview";
import { LOCATIONS, STORAGE_BOX } from "./storage";

// A handler can still wait on the real server when its test has passed;
// what it throws then is dropped.
test.afterEach(async ({ page }) => {
  await page.unrouteAll({ behavior: "ignoreErrors" });
});

const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);
const LAYOUT_KEY = "bombvault.dashboardLayout";
const ALL = ["needs", "protection", "targets", "history", "anomalies", "coverage", "heatmap", "storage", "summary", "stats", "spike"];

const cell = (page: Page, id: string) => page.locator(`[data-grid-block='${id}']`);
const stored = (page: Page) => page.evaluate((key) => JSON.parse(window.localStorage.getItem(key) ?? "null"), LAYOUT_KEY);

async function box(locator: Locator) {
  const b = await locator.boundingBox();
  if (!b) throw new Error("no box");
  return b;
}

async function customize(page: Page) {
  await page.getByTestId("page-actions").getByRole("button", { name: "Customize" }).click();
  await expect(page.getByTestId("hidden-cards")).toBeVisible();
  await settle(page);
}

test.describe("the grid of the overview", () => {
  test.beforeEach(async ({ page }, testInfo) => {
    test.skip(testInfo.project.name !== "desktop-1280", "the grid is laid out and arranged at desktop width");
    await stageOverview(page, undefined, { lang: "en" });
  });

  test("fills each row and gives the cards of one row one height", async ({ page }) => {
    await openOverview(page);

    const [needs, protection, targets, history] = await Promise.all(
      ["needs", "protection", "targets", "history"].map((id) => box(cell(page, id).locator(".glim-notch-card > div").last())),
    );
    // Protection and the storage locations share a row and end level, though
    // they hold a different number of rows.
    expect(Math.abs(protection.y - targets.y)).toBeLessThanOrEqual(1);
    expect(Math.abs(protection.height - targets.height)).toBeLessThanOrEqual(1);
    expect(Math.abs(protection.width - targets.width)).toBeLessThanOrEqual(1);
    // The full rows are as wide as the two halves with their gap.
    expect(Math.abs(needs.width - history.width)).toBeLessThanOrEqual(1);
    expect(Math.abs(targets.x + targets.width - (needs.x + needs.width))).toBeLessThanOrEqual(1);
    expect(needs.y).toBeLessThan(protection.y);
    expect(protection.y).toBeLessThan(history.y);
  });

  test("keeps the cards a browser hid and shows the rest as chips to bring back", async ({ page }) => {
    await openOverview(page);
    await customize(page);

    const tray = page.getByTestId("hidden-cards");
    await expect(tray.locator("[data-hidden-card]")).toHaveCount(7);

    await tray.locator("[data-hidden-card='coverage']").getByRole("button").click();
    await expect(cell(page, "coverage")).toBeVisible();
    await expect(tray.locator("[data-hidden-card]")).toHaveCount(6);

    await cell(page, "targets").getByRole("button", { name: "Hide" }).click();
    await expect(cell(page, "targets")).toHaveCount(0);
    // The row is filled again: protection takes the place the card left.
    await expect(cell(page, "protection")).toHaveAttribute("data-span", "6");

    // The layout outlives a reload.
    await page.getByTestId("page-actions").getByRole("button", { name: "Done" }).click();
    await page.reload();
    await expect(cell(page, "coverage")).toBeVisible();
    await expect(cell(page, "targets")).toHaveCount(0);
  });

  test("resizes a card by its handle: the width snaps to the columns, the height to 20px", async ({ page }) => {
    await openOverview(page);
    await customize(page);

    const history = cell(page, "history");
    const grid = await box(page.getByTestId("overview-grid"));
    const before = await box(history);
    const handle = await box(history.getByTestId("resize-handle"));
    // The handle is a filled dot at the bottom end corner of the card.
    expect(Math.abs(handle.x + handle.width / 2 - (before.x + before.width))).toBeLessThanOrEqual(10);
    expect(Math.abs(handle.y + handle.height / 2 - (before.y + before.height))).toBeLessThanOrEqual(10);

    // Drag to a little past half the grid and to 333px below the card's top.
    // The hover scrolls the handle into view, so the card is measured after it.
    await history.getByTestId("resize-handle").hover();
    const top = (await box(history)).y;
    await page.mouse.down();
    await page.mouse.move(grid.x + grid.width * 0.53, top + 333, { steps: 8 });
    await page.mouse.up();
    await settle(page);

    const layout = await stored(page);
    expect(layout.widths.history).toBe(3);
    expect(layout.heights.history).toBe(340);
    expect(Math.round((await box(history)).height)).toBe(340);
    // Alone in its row the card is widened to fill it, and keeps its width for
    // the day another card stands beside it.
    await expect(history).toHaveAttribute("data-span", "6");

    // A card with a height scrolls inside, below its title.
    const scrolls = await history.locator(".glim-notch-card > div").last().evaluate((el) => {
      const style = getComputedStyle(el);
      return { overflow: style.overflowY, clipped: el.scrollHeight > el.clientHeight };
    });
    expect(scrolls.overflow).toBe("auto");
    const title = await box(history.locator(".glim-notch-card h2 > *").first());
    expect(title.y).toBeGreaterThanOrEqual((await box(history)).y);

    // A double click gives the natural height back.
    await history.getByTestId("resize-handle").dblclick();
    await settle(page);
    expect((await stored(page)).heights).toEqual({});
    await expect(history).not.toHaveAttribute("data-sized", "");
  });

  test("steps a card's width from the keyboard", async ({ page }) => {
    await openOverview(page);
    await customize(page);

    const step = cell(page, "protection").getByRole("button", { name: /^Width ½/ });
    await step.focus();
    await page.keyboard.press("Enter");

    expect((await stored(page)).widths).toEqual({ protection: 4 });
    await expect(cell(page, "protection").getByRole("button", { name: /^Width full/ })).toBeVisible();
  });

  test("brings a layout from before the grid along", async ({ page }) => {
    await stageOverview(page, undefined, {
      lang: "en",
      layout: {
        order: ["protection", "storage", "summary", "activityLog", "runHistory", "ransomware", "lastBackups", "stats", "coverage", "anomalies", "heatmap", "spike"],
        hidden: ["stats", "spike", "heatmap", "coverage", "anomalies"],
        widths: { protection: "half", storage: "half" },
      },
    });
    await openOverview(page);

    const order = await page.locator("[data-grid-block]").evaluateAll((cells) => cells.map((c) => c.getAttribute("data-grid-block")));
    expect(order).toEqual(["needs", "protection", "targets", "storage", "summary", "history"]);
  });

  test("opens a section's window with its checks and a trial restore to start", async ({ page }) => {
    let drilled = "";
    await page.route("**/api/verify/**", (route) => {
      drilled = new URL(route.request().url()).search;
      return route.fulfill({ json: { ok: true, drill: { domain: "containers", source: "offsite", at: 1, ok: true, detail: "", kind: "dr" } } });
    });
    await openOverview(page);

    await page.getByTestId("protection-row").first().getByRole("button", { name: "Containers" }).click();

    const window = page.getByRole("dialog", { name: "Containers" });
    await expect(window.getByText("Trial restore: failed")).toBeVisible();
    await expect(window.getByText(DRILL_DETAIL)).toBeVisible();
    await expect(window.getByText("Off-site: copy overdue")).toBeVisible();
    await expect(window.getByRole("link", { name: /paperless-ngx/ })).toBeVisible();

    await window.getByRole("button", { name: "Run trial restore again" }).click();
    await expect.poll(() => drilled).toContain("kind=dr");

    await page.keyboard.press("Escape");
    await expect(window).toHaveCount(0);
  });

  test("shows a running backup in the head and leads to its run", async ({ page }) => {
    await page.route("**/api/progress", (route) =>
      route.fulfill({
        status: 200,
        contentType: "text/event-stream",
        body: `data: ${JSON.stringify({ key: "container:plex", phase: "backup", percent: 42, active: true })}\n\n`,
      }),
    );
    await openOverview(page);

    const head = page.getByTestId("status-head");
    await expect(head).toContainText("Backing up now");
    await expect(head).toContainText("plex");
    await expect(head.getByRole("img", { name: "Read" })).toBeVisible();
    await expect(head.getByRole("button", { name: "View run" })).toContainText("42");
    await expect(page.getByTestId("page-actions").getByRole("button", { name: "Backing up now" })).toBeVisible();
  });

  test("stands at the station the server names, whatever the percentage says", async ({ page }) => {
    await page.route("**/api/progress", (route) =>
      route.fulfill({
        status: 200,
        contentType: "text/event-stream",
        body: `data: ${JSON.stringify({ key: "container:plex", phase: "backup", percent: 42, active: true, step: "stop" })}

`,
      }),
    );
    await openOverview(page);

    await expect(page.getByTestId("status-head").getByRole("img", { name: "Stop" })).toBeVisible();
  });

  test("counts through a pass over everything and stacks a mark for each finished entry", async ({ page }) => {
    const done = [
      { ...RUNS[2], id: "pass-plex", groupId: EVERYTHING_RUN.id },
      { ...RUNS[1], id: "pass-paperless", groupId: EVERYTHING_RUN.id },
    ];
    await stageOverview(page, undefined, { lang: "en", runs: [...done, EVERYTHING_RUN, ...RUNS] });
    const frames = [
      { key: "everything", phase: "backup", percent: 14, active: true, done: 2, total: 14, unit: "items" },
      { key: "vm:Windows 11", phase: "backup", percent: 50, active: true, step: "read" },
    ];
    await page.route("**/api/progress", (route) =>
      route.fulfill({
        status: 200,
        contentType: "text/event-stream",
        body: frames.map((frame) => `data: ${JSON.stringify(frame)}

`).join(""),
      }),
    );
    await openOverview(page);

    const head = page.getByTestId("status-head");
    await expect(head).toContainText("Backing up now: 2 of 14");
    await expect(head).toContainText("Backing up Windows 11");
    await expect(head.getByRole("img", { name: "Read" })).toBeVisible();
    // Two entries of fourteen and half of the third.
    await expect(head.getByRole("button", { name: "View run" })).toContainText("18");
    const marks = head.getByTestId("pass-marks").locator("i");
    await expect(marks).toHaveCount(2);
    expect(await marks.evaluateAll((all) => all.map((mark) => mark.getAttribute("data-tone")))).toEqual(["fail", "ok"]);
    // The stack stands beside the stations, not under them.
    const strip = await box(head.getByRole("img", { name: "Read" }));
    const stack = await box(head.getByTestId("pass-marks"));
    expect(stack.x).toBeGreaterThanOrEqual(strip.x + strip.width);
    expect(stack.y + stack.height).toBeLessThanOrEqual(strip.y + strip.height);
  });

  test("lists a failed off-site copy and counts it until it is marked as seen", async ({ page }) => {
    let acked: string[] | null = null;
    const locations = () => LOCATIONS.map((location) => (location !== STORAGE_BOX ? location : acked ? STORAGE_BOX : FAILING_BOX));
    await page.route(
      (url) => url.pathname === "/api/storage/locations",
      (route) => route.fulfill({ json: { ok: true, locations: locations() } }),
    );
    await page.route("**/api/runs/ack", (route) => {
      acked = route.request().postDataJSON().copyTargets;
      return route.fulfill({ json: { ok: true, count: 2 } });
    });
    await openOverview(page);

    const count = (n: number) => page.getByLabel(`Things that need you: ${n}`).first();
    await expect(page.getByTestId("status-head")).toContainText("6 things need you");
    await expect(count(6)).toBeVisible();
    const row = page.getByTestId("need-failed-copy");
    await expect(row).toContainText("Off-site copy failed");
    await expect(row).toContainText("Storage Box · Containers and Flash · since ");
    await expect(row.getByRole("button", { name: "View" })).toBeVisible();
    await expect(row.getByRole("link", { name: "Off-site copy failed" })).toHaveAttribute("href", "/storage/destination%3Abox");

    await row.getByRole("button", { name: "More" }).click();
    await page.getByRole("menuitem", { name: "Resolve" }).click();

    await expect(row).toHaveCount(0);
    expect(acked).toEqual(["t-box-containers", "t-box-flash"]);
    await expect(page.getByTestId("status-head")).toContainText("5 things need you");
    await expect(count(5)).toBeVisible();
  });

  test("leaves an entry nothing backs up that way on request, and counts it again after Undo", async ({ page }) => {
    const sent: boolean[] = [];
    const declined = () => sent[sent.length - 1] ?? false;
    await page.route(
      (url) => url.pathname === "/api/items",
      (route) =>
        route.fulfill({
          json: {
            ok: true,
            items: ITEMS.map((item) => (item.key === UNPROTECTED_NAME ? { ...item, backupDeclined: declined() } : item)),
            unlisted: [],
          },
        }),
    );
    await page.route(`**/api/items/container/${UNPROTECTED_NAME}/declined`, (route) => {
      sent.push(route.request().postDataJSON().declined);
      return route.fulfill({ json: { ok: true } });
    });
    await openOverview(page);

    const head = page.getByTestId("status-head");
    const count = (n: number) => page.getByLabel(`Things that need you: ${n}`).first();
    const row = page.getByTestId("need-unprotected");
    await expect(head).toContainText("5 things need you");
    await expect(head).toContainText("7 of 8 protected");
    await expect(row.getByRole("button", { name: "Start backing up" })).toBeVisible();

    // Its only other action stands beside it, with no More menu to open.
    await expect(row.getByRole("button", { name: "More" })).toHaveCount(0);
    await row.getByRole("button", { name: "Don't back up" }).click();

    await expect(row).toHaveCount(0);
    await expect(head).toContainText("4 things need you");
    await expect(count(4)).toBeVisible();
    await expect(head).toContainText("7 of 7 protected");
    const toast = page.getByRole("status").filter({ hasText: `${UNPROTECTED_NAME} will not be backed up` });
    await expect(toast).toBeVisible();

    await toast.getByRole("button", { name: "Undo" }).click();

    await expect(row).toBeVisible();
    await expect(head).toContainText("5 things need you");
    await expect(count(5)).toBeVisible();
    expect(sent).toEqual([true, false]);
  });
});

test("the customize mode is a desktop tool: a phone has no button for it", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only");
  await stageOverview(page, undefined, {
    lang: "en",
    layout: { v: 2, order: ALL, hidden: ALL.slice(4), widths: { history: 2 }, heights: { history: 200 } },
  });
  await openOverview(page);

  await expect(page.getByTestId("page-actions").getByRole("button")).toHaveCount(1);
  // The sizes somebody set on a desktop do not reach the phone.
  const viewport = page.viewportSize()!;
  const history = await box(cell(page, "history"));
  expect(history.width).toBeGreaterThan(viewport.width * 0.8);
  expect(history.height).not.toBe(200);
});

test("at the end of the page a phone's floating action leaves the last row uncovered", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only");
  await stageOverview(page, undefined, { lang: "en" });
  await openOverview(page);
  await page.locator("#bv-main").evaluate((el) => el.scrollTo(0, el.scrollHeight));
  await settle(page);

  const action = await box(page.getByTestId("page-actions").getByRole("button").last());
  const last = await box(page.locator("[data-grid-block]").last());
  expect(last.y + last.height, "the last card reaches under the floating action").toBeLessThanOrEqual(action.y);
  for (const row of await page.getByTestId("protection-row").all()) {
    const r = await box(row);
    const overlaps = r.x < action.x + action.width && action.x < r.x + r.width && r.y < action.y + action.height && action.y < r.y + r.height;
    expect(overlaps, "a protection row lies under the floating action").toBe(false);
  }
});
