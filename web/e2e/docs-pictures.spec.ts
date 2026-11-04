// The pictures of the user documentation, each in the dark and the light
// theme, taken from the demo server in demo.ts. They write files and take a
// few minutes, so they are made on request and skipped in the gate:
//
//   node e2e/ensure-binary.mjs
//   DOCS_SHOTS=../docs/assets/screenshots npx playwright test docs-pictures --project=desktop-1280
//
// Pages are found by test id and role, and buttons by their translation key,
// so DOCS_LANG=de renders the same pictures in another language. The Android
// app's start screen comes from `node android/store/render.mjs docs`.
import { expect, test, type Browser, type Locator, type Page } from "@playwright/test";
import { mkdirSync } from "node:fs";
import { dirname, join } from "node:path";

import { settle } from "./entries";
import {
  CRITICAL,
  FRESH_DOMAINS,
  FRESH_LOCAL,
  INFO,
  WARNING,
  ZFS_APPDATA,
  freshItems,
  night,
  progress,
  stageDemo,
  stageRestores,
  words,
  type Demo,
} from "./demo";

const SHOTS = process.env.DOCS_SHOTS;
const LANG = process.env.DOCS_LANG ?? "en";
const t = words(LANG);

const DESKTOP = { width: 1440, height: 900 };
const PHONE = { width: 390, height: 844 };
const THEMES = ["dark", "light"] as const;
type Theme = (typeof THEMES)[number];

/** A part of the page to keep. Tight keeps only what is drawn in it, for a
 *  head that spans the page but fills a corner of it. */
type Part = Locator | { tight: Locator };

/**
 * picture runs one shot in both themes, each in a context of its own so no
 * stored look carries over, and writes <name>-<theme>.png. The shot returns
 * the part of the page to keep, or nothing for the whole viewport.
 */
function picture(
  name: string,
  viewport: { width: number; height: number },
  shot: (page: Page) => Promise<Part | void>,
  demo: Omit<Demo, "theme"> = {},
) {
  test(name, async ({ browser }, testInfo) => {
    test.skip(!SHOTS || testInfo.project.name !== "desktop-1280", "only when a folder for the pictures is named");
    test.setTimeout(120_000);
    for (const theme of THEMES) {
      const page = await open(browser, viewport, theme);
      await stageDemo(page, { ...demo, theme, lang: LANG });
      const part = await shot(page);
      // The pointer would leave a hover or a tooltip on whatever it clicked last.
      await page.mouse.move(0, 0);
      await settle(page);
      // A sheet or window starts to slide a frame after it mounts.
      await page.waitForTimeout(400);
      await settle(page);
      const path = join(SHOTS!, `${name}-${theme}.png`);
      mkdirSync(dirname(path), { recursive: true });
      await page.screenshot({ path, animations: "disabled", clip: part ? await around(part) : undefined });
      await page.context().close();
    }
  });
}

async function open(browser: Browser, viewport: { width: number; height: number }, theme: Theme): Promise<Page> {
  const context = await browser.newContext({ viewport, colorScheme: theme, reducedMotion: "reduce" });
  return context.newPage();
}

/** The box around a part with a margin of the page around it. A card's
 *  heading badge stands above its edge, so the box takes in every element
 *  drawn inside and not just the part's own. */
async function around(part: Part) {
  const tight = "tight" in part;
  const el = tight ? part.tight : part;
  await el.evaluate((node) => node.scrollIntoView({ block: "center" }));
  await settle(el.page());
  const box = await el.evaluate((node, onlyDrawn) => {
    const drawn = [node, ...node.querySelectorAll("*")].filter(
      (e) => !onlyDrawn || e.tagName === "svg" || (e.children.length === 0 && !e.closest("svg") && e.textContent?.trim()),
    );
    // A line of text fills its block, so what it draws is measured by a range.
    const measure = (e: Element) => {
      if (!onlyDrawn || e.tagName === "svg") return e.getBoundingClientRect();
      const range = document.createRange();
      range.selectNodeContents(e);
      return range.getBoundingClientRect();
    };
    const rects = drawn.map(measure).filter((r) => r.width > 0 && r.height > 0);
    const own = node.getBoundingClientRect();
    return {
      left: Math.min(...rects.map((r) => r.left)),
      top: Math.min(...rects.map((r) => r.top)),
      right: Math.min(Math.max(...rects.map((r) => r.right)), own.right),
      bottom: Math.max(...rects.map((r) => r.bottom)),
      width: window.innerWidth,
      height: window.innerHeight,
    };
  }, tight);
  const pad = 16;
  const x = Math.max(0, Math.floor(box.left - pad));
  const y = Math.max(0, Math.floor(box.top - pad));
  return { x, y, width: Math.min(box.width, Math.ceil(box.right + pad)) - x, height: Math.min(box.height, Math.ceil(box.bottom + pad)) - y };
}

const card = (page: Page, key: string) => page.locator(`[data-search-card="${t(key)}"]`);
const dialog = (page: Page) => page.getByRole("dialog").last();

async function scrollTo(target: Locator, block: ScrollLogicalPosition = "start"): Promise<void> {
  await target.evaluate((el, at) => el.scrollIntoView({ block: at }), block);
}

// Overview and first start

async function overview(page: Page): Promise<void> {
  await page.goto("/dashboard");
  await page.getByTestId("protection-row").first().waitFor();
  await page.getByTestId("target-row").first().waitFor();
}

picture("overview/home", DESKTOP, overview);

const FIRST_START: Omit<Demo, "theme"> = {
  items: freshItems(),
  runs: [],
  findings: [],
  locations: [FRESH_LOCAL],
  nextAt: night(-1, -60),
  domains: FRESH_DOMAINS,
  settings: {
    filesEnabled: false,
    zfsEnabled: false,
    configEnabled: false,
    everythingSchedule: "off",
    vmsSchedule: "off",
    zfsOffsiteSchedule: "off",
    recoveryKitAck: false,
  },
};

picture(
  "getting-started/welcome",
  DESKTOP,
  async (page) => {
    await page.goto("/");
    await expect(page).toHaveURL(/\/welcome$/);
    await expect(page.getByTestId("welcome-found")).toBeVisible();
  },
  FIRST_START,
);

picture(
  "getting-started/first-steps",
  DESKTOP,
  async (page) => {
    await page.goto("/dashboard");
    const steps = page.getByTestId("first-steps").locator("li[data-state]");
    await expect(steps.nth(1)).toHaveAttribute("data-state", "next");
  },
  {
    ...FIRST_START,
    items: freshItems(true),
    settings: { ...FIRST_START.settings, containersSchedule: "daily 02:00" },
    stored: { "bombvault.firstSteps.sectionsChosen": "1", "bombvault.firstSteps.included": "container" },
  },
);

picture("getting-started/status-ring", DESKTOP, async (page) => {
  await progress(page, [{ key: "container:immich", phase: "backup", percent: 42, active: true, step: "read" }]);
  await overview(page);
  const head = page.getByTestId("status-head");
  await expect(head.getByRole("img").first()).toBeVisible();
  return { tight: head };
});

picture("getting-started/settings-rail", DESKTOP, async (page) => {
  await page.goto("/settings/general");
  await expect(page.locator("[data-settings-content]")).toBeVisible();
});

picture("getting-started/phone-more", PHONE, async (page) => {
  await overview(page);
  await page.getByTestId("bottom-nav").getByRole("button", { name: t("nav.more") }).click();
  await expect(page.getByTestId("more-sheet")).toBeInViewport({ ratio: 1 });
});

picture("overview/customize", DESKTOP, async (page) => {
  await overview(page);
  await page.getByTestId("page-actions").getByRole("button", { name: t("dashboard.customize") }).click();
  await expect(page.getByTestId("hidden-cards")).toBeVisible();
});

picture("overview/protection-window", DESKTOP, async (page) => {
  await overview(page);
  await page.locator("[data-testid='protection-row'][data-domain='containers'] button").first().click();
  await expect(dialog(page)).toBeVisible();
});

// Backups

const rows = (page: Page) => page.getByTestId("backup-row");

async function backups(page: Page): Promise<void> {
  await page.goto("/backups");
  await expect(rows(page).first()).toBeVisible();
}

picture("backups/list", DESKTOP, backups);
picture("backups/list-phone", PHONE, backups);

picture("backups/bulk", DESKTOP, async (page) => {
  await backups(page);
  await rows(page).filter({ hasText: "nextcloud" }).click({ modifiers: ["ControlOrMeta"] });
  await rows(page).filter({ hasText: "mariadb" }).click({ modifiers: ["ControlOrMeta"] });
  await expect(page.getByTestId("bulk-bar")).toBeVisible();
});

async function entry(page: Page, path: string): Promise<void> {
  await page.goto(path);
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  await page.waitForTimeout(800);
}

picture("backups/entry-backups", DESKTOP, async (page) => {
  await entry(page, "/backups/container/immich");
  await expect(page.getByTestId("entry-backup").first()).toBeVisible();
});

picture("backups/entry-settings", DESKTOP, async (page) => {
  await entry(page, "/backups/container/plex?tab=settings");
});

picture("backups/entry-verify", DESKTOP, async (page) => {
  await entry(page, "/backups/container/immich?tab=verify");
  const sizes = card(page, "breakdown.title");
  await expect(sizes.getByRole("button").first()).toBeVisible();
  await scrollTo(sizes);
});

picture("backups/app-busy", DESKTOP, async (page) => {
  await entry(page, "/backups/container/plex?tab=settings");
  return card(page, "deferral.title");
});

// Restore

const restoreRows = (page: Page) => page.getByTestId("rail-row");
// While entries are being picked, each tile is a checkbox.
const entryTile = (page: Page, name: string, role: "button" | "checkbox" = "button") =>
  page.getByTestId("restore-entries").getByRole(role, { name: new RegExp(`^${name} `) });

async function restorePage(page: Page): Promise<void> {
  await page.goto("/restore");
  await expect(restoreRows(page).first()).toBeVisible();
}

// The Containers tile and two backups on the rail leave room for the Stacks
// card under them.
picture(
  "restore/page",
  DESKTOP,
  async (page) => {
    await restorePage(page);
    await page.getByRole("button", { name: new RegExp(`^${t("nav.containers")}`) }).first().click();
    await expect(page.getByTestId("stacks")).toBeVisible();
  },
  { backupsShown: 2 },
);
picture("restore/page-phone", PHONE, restorePage);

picture("restore/window-what", DESKTOP, async (page) => {
  await restorePage(page);
  await entryTile(page, "Documents").click();
  await restoreRows(page).first().getByRole("button", { name: t("snapshots.restore") }).click();
  const win = dialog(page);
  await win.getByRole("tab", { name: t("restore.mode.files") }).click();
  for (const folder of ["/documents/taxes", "/documents/insurance"]) {
    await win.getByTitle(folder, { exact: true }).locator("..").getByRole("button").first().click();
  }
  for (const file of ["/documents/taxes/2025", "/documents/insurance/home-policy.pdf"]) {
    await win.getByRole("checkbox", { name: file, exact: true }).click();
  }
});

picture("restore/window-replace", DESKTOP, async (page) => {
  await restorePage(page);
  await entryTile(page, "immich").click();
  await restoreRows(page).first().getByRole("button", { name: t("snapshots.restore") }).click();
  const win = dialog(page);
  await win.getByRole("radio", { name: new RegExp(`^${t("restoreWindow.replace")}`) }).click();
  await scrollTo(win.getByText(t("restoreWindow.happens"), { exact: true }));
});

picture("restore/several", DESKTOP, async (page) => {
  await stageRestores(page);
  await restorePage(page);
  await page.getByRole("button", { name: t("backups.select") }).click();
  for (const name of ["plex", "immich", "nextcloud", "vaultwarden"]) await entryTile(page, name, "checkbox").click();
  await page.getByTestId("bulk-bar").getByRole("button", { name: t("snapshots.restore") }).click();
  const win = dialog(page);
  await progress(page, [{ key: "container:nextcloud", phase: "restore", percent: 58, active: true }]);
  await win.getByRole("button", { name: new RegExp(`^${t("snapshots.restore")} · `) }).click();
  await expect(win.getByText(t("restoreMany.done"), { exact: true })).toBeVisible({ timeout: 15_000 });
  await expect(win.getByText(t("restoreMany.running"), { exact: true })).toBeVisible();
});

// Storage locations

const location = (id: string) => `/storage/${encodeURIComponent(id)}`;

picture("storage/locations", DESKTOP, async (page) => {
  await page.goto("/storage");
  await expect(card(page, "storage.title").getByRole("link").first()).toBeVisible();
});

async function addLocation(page: Page): Promise<Locator> {
  await page.goto("/storage");
  await card(page, "storage.title").getByRole("button", { name: t("storage.add.title") }).click();
  const win = dialog(page);
  await expect(win.locator(".glim-provider-pick button").first()).toBeVisible();
  return win;
}

picture("storage/add-providers", DESKTOP, async (page) => {
  await addLocation(page);
});

picture("storage/add-signin", DESKTOP, async (page) => {
  const win = await addLocation(page);
  await win.locator(".glim-provider-pick button").filter({ hasText: "Rest server" }).click();
  // Name, address, user name and password, in the order the form asks.
  const fields = win.locator("input:not([type=search])");
  await fields.nth(0).fill("Offsite NAS");
  await fields.nth(1).fill("https://offsite-nas.home.arpa:8000/tower");
  await fields.nth(2).fill("tower");
  await fields.nth(3).fill("correct-horse-battery-staple");
  await win.getByRole("button", { name: t("offsite.test") }).click();
  await win.getByRole("button", { name: t("storage.add.change") }).click();
});

picture("storage/location-page", DESKTOP, async (page) => {
  await page.goto(location("destination:b2"));
  await expect(card(page, "own.card")).toBeVisible();
});

picture("storage/location-page-phone", PHONE, async (page) => {
  await page.goto(location("destination:b2"));
  await expect(card(page, "own.card")).toBeVisible();
});

picture("storage/keep", DESKTOP, async (page) => {
  await page.goto(location("destination:b2"));
  return card(page, "storage.keep.title");
});

picture("storage/signins", DESKTOP, async (page) => {
  await page.goto("/settings/storage");
  const signIns = card(page, "signIns.title");
  await expect(signIns.getByText("Backblaze B2").first()).toBeVisible();
  return signIns;
});

picture("storage/move-own-folder", DESKTOP, async (page) => {
  await page.goto(location("destination:b2"));
  await card(page, "own.card").getByRole("button").last().click();
  const win = dialog(page);
  // The window offers every section with an own folder elsewhere. Flash and
  // Folders stay picked: B2 holds a copy of both, so it has the most to say.
  for (const section of ["nav.containers", "nav.vms", "nav.zfs", "nav.config"]) {
    await win.getByRole("switch", { name: t(section), exact: true }).click();
  }
  const happens = win.getByText(t("own.what"), { exact: true });
  await expect(happens).toBeVisible();
  await page.waitForTimeout(800);
  await scrollTo(happens);
});

// Settings

picture("schedule/week", DESKTOP, async (page) => {
  await page.goto("/settings/schedules");
  await expect(card(page, "schedules.plans")).toBeVisible();
});

picture("notifications/gotify", DESKTOP, async (page) => {
  await page.goto("/settings/notifications");
  const gotify = page.locator("[data-new='notify-gotify']");
  await expect(gotify).toBeVisible();
  await scrollTo(gotify, "center");
});

picture("offsite/pairing", DESKTOP, async (page) => {
  await page.goto("/settings/pairing");
  await expect(page.getByTestId("members")).toBeVisible();
});

const instance = (page: Page, key: string) => page.locator(`#bv-main [data-instance="${key}"]`);

picture("offsite/instances", DESKTOP, async (page) => {
  await page.goto("/instances");
  await expect(page.getByTestId("net-picture")).toBeVisible();
  await expect(instance(page, "self")).toBeVisible();
});

picture("offsite/emergency", DESKTOP, async (page) => {
  await page.goto("/restore?emergency=access");
  await expect(dialog(page).getByRole("radio").first()).toBeVisible();
});

picture("offsite/recovery-code", DESKTOP, async (page) => {
  await page.goto("/instances");
  await instance(page, "self").getByRole("button", { name: t("keeper.window.title"), exact: true }).click();
  await dialog(page).getByRole("button", { name: t("keeper.own.newCode") }).click();
  await dialog(page).getByRole("button", { name: t("keeper.own.newCode") }).click();
  await expect(page.getByTestId("recovery-code")).toBeVisible();
});

picture("zfs/replica", DESKTOP, async (page) => {
  await entry(page, `/backups/zfs/${ZFS_APPDATA}?tab=settings`);
  const replica = page.locator(".glim-notch-card").filter({ has: page.getByText(t("zfs.replica.title"), { exact: true }) });
  await expect(replica).toBeVisible();
  await scrollTo(replica);
});

picture(
  "anomalies/page",
  DESKTOP,
  async (page) => {
    await page.goto("/anomalies");
    await expect(page.locator("#bv-main li[id^='finding-']").first()).toBeVisible();
  },
  { findings: [CRITICAL, WARNING, INFO] },
);

