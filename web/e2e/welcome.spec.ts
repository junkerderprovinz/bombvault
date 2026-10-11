// First start in a real browser: a new install opens on the welcome page, its
// navigation carries no counts, "Let's go" leads to the first steps on the
// overview, and an install with a backup never sees any of it. The harness
// has no Docker, so the entries a server would find are staged at the route
// layer, along with a failed run, an open finding and an unconfirmed recovery
// kit: everything that raises a count on an install in use. What the first
// steps save is answered there too, the way the server would.
import { expect, test, type Page } from "@playwright/test";

import { measureLayout, settle } from "./storage";

const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);
const NOW = Math.floor(Date.now() / 1000);
const NEEDS = /^Things that need you/;
const ANOMALIES = /^Open critical and warning anomalies/;

test.afterEach(async ({ page }) => {
  await page.unrouteAll({ behavior: "ignoreErrors" });
});

interface Entry {
  kind: string;
  self?: boolean;
  effectiveSchedule: Record<string, string>;
  [field: string]: unknown;
}

function entry(over: Partial<Entry>): Entry {
  return {
    kind: "container",
    key: "plex",
    name: "plex",
    included: false,
    paused: false,
    effectiveSchedule: { kind: "none", spec: "", reason: "schedule-off" },
    lastBackup: 0,
    lastRunStatus: "",
    sourceBytes: null,
    state: "running",
    installed: true,
    runs: [],
    ...over,
  };
}

const FOUND = [
  entry({}),
  entry({ key: "gitea", name: "gitea" }),
  entry({ key: "immich", name: "immich", lastRunStatus: "failed", runs: [{ id: "run-1", kind: "backup", status: "failed", startedAt: NOW - 600 }] }),
  entry({ key: "bombvault", name: "bombvault", self: true }),
  entry({ kind: "vm", key: "Windows 11", name: "Windows 11", kindDisabled: true }),
];

const IN_USE = [...FOUND, entry({ key: "paperless", name: "paperless", lastBackup: NOW - 3600, lastRunStatus: "success" })];

const FAILED_RUN = {
  id: "run-1",
  targetId: "immich",
  kind: "backup",
  status: "failed",
  startedAt: NOW - 600,
  finishedAt: NOW - 540,
  snapshotId: "",
  bytes: 0,
  error: "restic exited 1",
  acknowledged: false,
  groupId: "",
  target: "immich",
  domain: "container",
};

interface Staged {
  /** The settings the page saved last. */
  saved: () => Record<string, unknown>;
  /** The kinds whose entries the page asked to take into the schedule, one
   *  for each request. */
  included: () => string[];
  /** Takes every entry out of the schedule again, as pausing each in the
   *  list does. */
  pauseAll: () => void;
  /** Whether the page asked the server to back up everything. */
  fired: () => boolean;
}

async function stage(page: Page, items: Entry[] = FOUND, includeRefused = ""): Promise<Staged> {
  // The harness database serves every project of the run, so what the first
  // steps save stays here and is read back from here.
  let saved: Record<string, unknown> = {};
  await page.route("**/api/settings", async (route) => {
    if (route.request().method() === "PUT") {
      saved = route.request().postDataJSON();
      return route.fulfill({ json: { ok: true } });
    }
    if (route.request().method() !== "GET") return route.continue();
    const res = await route.fetch();
    const body = await res.json();
    await route.fulfill({
      json: {
        ...body,
        platform: "unraid",
        settings: { ...body.settings, anomalyEnabled: true, encryptionEnabled: true, recoveryKitAck: false, ...saved },
      },
    });
  });
  const included: string[] = [];
  const inSchedule = new Set<string>();
  let fired = false;
  for (const [path, kind] of [
    ["containers", "container"],
    ["vms", "vm"],
  ]) {
    await page.route(`**/api/${path}/schedule-include`, (route) => {
      if (includeRefused) return route.fulfill({ json: { ok: false, error: includeRefused } });
      included.push(kind);
      inSchedule.add(kind);
      return route.fulfill({ json: { ok: true } });
    });
  }
  await page.route("**/api/backup-everything", (route) => {
    fired = true;
    return route.fulfill({ json: { ok: true, started: true } });
  });
  // An entry taken into the schedule follows the schedule of its section.
  const listed = () =>
    items.map((item) => {
      const spec = saved[`${item.kind}sSchedule`];
      if (!inSchedule.has(item.kind) || item.self) return item;
      const runs = typeof spec === "string" && spec !== "off";
      return { ...item, included: true, effectiveSchedule: runs ? { kind: "domain", spec } : item.effectiveSchedule };
    });
  await page.route(
    (url) => url.pathname === "/api/items",
    (route) => route.fulfill({ json: { ok: true, items: listed(), unlisted: [] } }),
  );
  await page.route(
    (url) => url.pathname === "/api/runs",
    (route) => route.fulfill({ json: { ok: true, runs: [FAILED_RUN] } }),
  );
  await page.route("**/api/anomalies/summary", (route) =>
    route.fulfill({
      json: {
        ok: true,
        summary: {
          enabled: true,
          ready: true,
          generation: 1,
          open: { critical: 1, warning: 0, info: 0 },
          recoveredCritical: 0,
          learningItems: 0,
          retentionHeld: 0,
          evalErrors: 0,
          notifyMuted: false,
          backfill: { slots: 0, done: 0, failed: 0, filled: 0, withoutSummary: 0 },
          unmeasuredVolumes: [],
        },
      },
    }),
  );
  // The stored language is the look, and the server's display preferences
  // must not replace it while the page boots.
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.addInitScript(() => window.localStorage.setItem("bv-lang", "en"));
  return { saved: () => saved, included: () => included, pauseAll: () => inSchedule.clear(), fired: () => fired };
}

const step = (page: Page, name: string) => page.locator(`[data-step='${name}']`);

async function openApp(page: Page): Promise<void> {
  await page.goto("/");
  await expect(page).toHaveURL(/\/welcome$/);
  await expect(page.getByRole("button", { name: "Let's go" })).toBeVisible();
  await settle(page);
}

test("a new install opens on the welcome page", async ({ page }) => {
  await stage(page);
  await openApp(page);

  await expect(page.getByRole("heading", { level: 1, name: "Welcome to BombVault" })).toBeAttached();
  await expect(page.getByText("You're protected in three steps.")).toBeVisible();
  await expect(page.getByTestId("welcome-found")).toHaveText("BombVault found 3 containers, 1 VM, and the flash drive.");

  await expect(step(page, "sections")).toHaveAttribute("data-state", "next");
  await expect(step(page, "location")).toHaveAttribute("data-state", "open");
  await expect(step(page, "backup")).toHaveAttribute("data-state", "open");
  await expect(step(page, "location")).toContainText("Add storage location");
  await expect(step(page, "backup")).toContainText("First backup");
});

test("the two buttons stand at the taller height, the way forward last", async ({ page }) => {
  await stage(page);
  await openApp(page);

  const restore = (await page.getByRole("button", { name: "Restore from a backup" }).boundingBox())!;
  const start = (await page.getByRole("button", { name: "Let's go" }).boundingBox())!;
  expect(restore.height).toBeGreaterThanOrEqual(39.5);
  expect(Math.abs(restore.height - start.height)).toBeLessThanOrEqual(0.5);
  // Side by side the way forward is on the right, stacked it is below.
  expect(start.x > restore.x || start.y > restore.y).toBe(true);
  expect(await page.locator("#bv-main button.bg-accent").count()).toBe(1);
});

test("the navigation carries no count before the first backup", async ({ page }) => {
  await stage(page);
  await openApp(page);

  await expect(page.getByLabel(NEEDS)).toHaveCount(0);
  await expect(page.getByLabel(ANOMALIES)).toHaveCount(0);

  await page.getByRole("button", { name: "Let's go" }).click();
  await expect(page.getByTestId("first-steps")).toBeVisible();
  await expect(page.getByLabel(NEEDS)).toHaveCount(0);
  await expect(page.getByLabel(ANOMALIES)).toHaveCount(0);
});

test("Let's go leads to the first steps, Back returns, and a reload stays where it is", async ({ page }) => {
  await stage(page);
  await openApp(page);

  await page.getByRole("button", { name: "Let's go" }).click();
  await expect(page).toHaveURL(/\/dashboard$/);
  const card = page.getByTestId("first-steps");
  await expect(card.getByRole("switch", { name: "Containers" })).toHaveAttribute("aria-checked", "true");
  await expect(card.getByRole("switch", { name: "VMs" })).toHaveAttribute("aria-checked", "false");
  await expect(card).toContainText("3 found");

  await page.reload();
  await expect(page).toHaveURL(/\/dashboard$/);
  await expect(card).toBeVisible();

  await page.goBack();
  await expect(page).toHaveURL(/\/welcome$/);
  await expect(page.getByRole("button", { name: "Let's go" })).toBeVisible();

  await page.reload();
  await expect(page).toHaveURL(/\/welcome$/);
  await expect(page.getByRole("button", { name: "Let's go" })).toBeVisible();
});

test("a section switched on in the first steps gets a nightly schedule, and loses it when switched off", async ({ page }) => {
  const staged = await stage(page);
  await page.goto("/dashboard");
  const card = page.getByTestId("first-steps");
  const selfBackup = card.getByRole("switch", { name: "Self-backup" });
  await expect(selfBackup).toHaveAttribute("aria-checked", "false");
  await expect(card.getByLabel("The sections switched on here run every night, which you can change under Settings › Schedule.")).toBeVisible();

  await selfBackup.click();
  await expect(selfBackup).toHaveAttribute("aria-checked", "true");
  await expect.poll(() => staged.saved().configSchedule).toBe("daily 02:00");
  expect(staged.saved().configEnabled).toBe(true);
  // Containers were on before and nobody switched them.
  expect(staged.saved().containersSchedule).toBe("off");

  await selfBackup.click();
  await expect(selfBackup).toHaveAttribute("aria-checked", "false");
  await expect.poll(() => staged.saved().configSchedule).toBe("off");
});

test("the welcome page promises nightly backups once an entry is in a schedule", async ({ page }) => {
  await stage(page);
  await openApp(page);
  await expect(step(page, "backup")).not.toContainText("After that it runs by itself every night");

  await page.unrouteAll({ behavior: "ignoreErrors" });
  const scheduled = { kind: "domain", spec: "daily 02:00" };
  await stage(page, [entry({ included: true, effectiveSchedule: scheduled }), ...FOUND.slice(1)]);
  await openApp(page);
  await expect(step(page, "backup")).toContainText("After that it runs by itself every night");
});

test("Next ticks off the first step, here and on the welcome page", async ({ page }) => {
  const staged = await stage(page);
  await page.goto("/dashboard");
  const card = page.getByTestId("first-steps");
  const steps = card.locator("li[data-state]");
  await expect(steps).toHaveCount(3);
  await expect(steps.nth(0)).toHaveAttribute("data-state", "next");

  await steps.nth(0).getByRole("button", { name: "Next" }).click();

  await expect(steps.nth(0)).toHaveAttribute("data-state", "done");
  // The section that was on from the start gets its schedule here, and the
  // containers found for it are taken in. The VMs' section is off.
  expect(staged.saved().containersSchedule).toBe("daily 02:00");
  expect(staged.included()).toEqual(["container"]);
  await expect(steps.nth(1)).toHaveAttribute("data-state", "next");
  await expect(steps.nth(1).getByRole("button", { name: "Add storage location" })).toBeVisible();

  await openApp(page);
  await expect(step(page, "sections")).toHaveAttribute("data-state", "done");
  await expect(step(page, "location")).toHaveAttribute("data-state", "next");
});

test("the first backup and the nightly run hold what the first steps found", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "desktop-only: a phone asks before it starts the pass");
  const staged = await stage(page);
  await page.goto("/dashboard");
  const steps = page.getByTestId("first-steps").locator("li[data-state]");
  await steps.nth(0).getByRole("button", { name: "Next" }).click();
  await expect(steps.nth(0)).toHaveAttribute("data-state", "done");

  // BombVault's own container and the VMs of the section that is off stay out.
  await expect(page.getByTestId("status-head")).toContainText("0 of 3 protected");
  await page.getByTestId("page-actions").getByRole("button", { name: "Back up everything now" }).click();
  await expect.poll(() => staged.fired()).toBe(true);

  await openApp(page);
  await expect(step(page, "backup")).toContainText("After that it runs by itself every night");
});

test("Choose takes the found entries in and opens their list, where one can be left out", async ({ page }) => {
  const staged = await stage(page);
  await page.goto("/dashboard");
  const card = page.getByTestId("first-steps");
  await expect(card.locator("[data-section='vm']").getByRole("button")).toHaveCount(0);

  await card.locator("[data-section='container']").getByRole("button", { name: /^Choose/ }).click();

  await expect(page).toHaveURL(/\/backups\?kind=container$/);
  expect(staged.included()).toEqual(["container"]);
  expect(staged.saved().containersSchedule).toBe("daily 02:00");
});

test("entries that were all paused after Choose stay out when Next is pressed", async ({ page }) => {
  const staged = await stage(page);
  await page.goto("/dashboard");
  await page.getByTestId("first-steps").locator("[data-section='container']").getByRole("button", { name: /^Choose/ }).click();
  await expect(page).toHaveURL(/\/backups\?kind=container$/);
  expect(staged.included()).toEqual(["container"]);

  staged.pauseAll();
  await page.goto("/dashboard");
  const steps = page.getByTestId("first-steps").locator("li[data-state]");
  await expect(steps.nth(0).getByRole("switch", { name: "Containers" })).toBeVisible();
  await steps.nth(0).getByRole("button", { name: "Next" }).click();

  await expect(steps.nth(0)).toHaveAttribute("data-state", "done");
  expect(staged.included()).toEqual(["container"]);
});

test("the first step stays open and says why when the entries cannot be taken in", async ({ page }) => {
  await stage(page, FOUND, "list containers: the Docker socket did not answer");
  await page.goto("/dashboard");
  const steps = page.getByTestId("first-steps").locator("li[data-state]");

  await steps.nth(0).getByRole("button", { name: "Next" }).click();

  await expect(page.getByText("list containers: the Docker socket did not answer")).toBeVisible();
  await expect(steps.nth(0)).toHaveAttribute("data-state", "next");
});

test("an install with a backup opens on the overview and carries its counts", async ({ page }, testInfo) => {
  await stage(page, IN_USE);
  await page.goto("/");

  await expect(page).toHaveURL(/\/dashboard$/);
  await expect(page.getByTestId("status-head")).toBeVisible();
  await expect(page.getByTestId("first-steps")).toHaveCount(0);
  // The failed run, the two entries no schedule reaches and the recovery kit.
  await expect(page.getByLabel(NEEDS)).toHaveText("4");
  if (!MOBILE_PROJECTS.has(testInfo.project.name)) await expect(page.getByLabel(ANOMALIES)).toHaveText("1");

  await page.goto("/welcome");
  await expect(page).toHaveURL(/\/dashboard$/);
});

for (const width of [320, 390]) {
  test(`first start @ ${width}px: nothing pans and every button takes a fingertip`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone layout lives below 48rem");
    await page.setViewportSize({ width, height: 800 });
    await stage(page);
    await openApp(page);

    const fits = async () => {
      const layout = await measureLayout(page);
      expect.soft(layout.docPan, "the document scrolls horizontally").toBeLessThanOrEqual(1);
      expect.soft(layout.mainPan!, "#bv-main scrolls horizontally").toBeLessThanOrEqual(1);
      expect.soft(layout.clipped, "controls clip the viewport edge").toEqual([]);
      expect.soft(layout.overflowing, "text runs past the viewport edge").toEqual([]);
    };
    await fits();
    // The three steps stand below each other.
    const tops = await page.locator("[data-step]").evaluateAll((els) => els.map((el) => el.getBoundingClientRect().top));
    expect(tops[0]).toBeLessThan(tops[1]);
    expect(tops[1]).toBeLessThan(tops[2]);

    await page.getByRole("button", { name: "Let's go" }).click();
    const card = page.getByTestId("first-steps");
    await expect(card.getByRole("switch", { name: "Containers" })).toBeVisible();
    await settle(page);
    await fits();
    for (const control of await card.getByRole("switch").all()) {
      const box = (await control.boundingBox())!;
      expect(box.height, "a switch is shorter than a fingertip").toBeGreaterThanOrEqual(31.5);
    }
  });
}
