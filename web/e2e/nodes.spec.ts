// The node pages on the demo server: a card per node on Instances with when it
// backs up, the window that adds one, the nodes card on the overview, a
// section per node on the Backups list, and the page of an entry on a node
// with its backups and a restore. Every request about a node
// goes to its route under /api/nodes/{id}/api, never to this server's own.
//
// NODE_SHOTS=<folder> also writes the pictures of these pages in the light,
// dark and rainbow look at 1280 and 375 pixels:
//
//   NODE_SHOTS=../shots npx playwright test nodes --project=desktop-1280
import { expect, test, type Browser, type Page } from "@playwright/test";
import { mkdirSync } from "node:fs";
import { join } from "node:path";

import { stageDemo, words } from "./demo";
import { overflow, settle } from "./entries";
import { COMPOSE, NODE, NODE_CONTAINERS, stageNodes } from "./nodes";

const t = words("en");

test.afterEach(async ({ page }) => {
  await page.unrouteAll({ behavior: "ignoreErrors" });
});

async function open(page: Page, path: string, receiverNodeUrl = "") {
  await stageDemo(page, { theme: "dark" });
  const calls = await stageNodes(page, receiverNodeUrl);
  await page.goto(path);
  return calls;
}

const card = (page: Page, id: string) => page.locator(`[data-instance="node:${id}"]`);

test("every node has its card with its state, where its backups go and its actions", async ({ page }) => {
  await open(page, "/instances");
  const online = card(page, NODE.id);
  await expect(online).toContainText(NODE.name);
  await expect(online).toContainText(t("instances.connected"));
  await expect(online).toContainText("v10.5.0");
  await expect(online).toContainText(NODE.placementTarget ?? "");
  for (const key of ["nodes.editPlacement", "nodes.replace", "nodes.release"]) {
    await expect(online.getByRole("button", { name: t(key) })).toBeVisible();
  }
  await expect(card(page, "2b4d6f8a0c1e3a5b7c9d0e1f2a3b4c5d")).toContainText(t("instances.inactive"));
  const waiting = card(page, "9e8d7c6b5a4f3e2d1c0b9a8f7e6d5c4b");
  await expect(waiting).toContainText(t("nodes.waiting"));
  await expect(waiting.getByRole("button", { name: t("nodes.editPlacement") })).toBeDisabled();
  expect((await overflow(page)).pan).toBe(0);
});

const plan = (scope: Page | ReturnType<typeof card>, section: string) => scope.getByRole("button", { name: new RegExp(`^${t(section)}:`) });

// pickMode picks a mode in an open schedule editor: a row of tabs where they
// fit, a list that opens on a narrow phone.
async function pickMode(page: Page, key: string) {
  const editor = page.getByRole("dialog").last();
  await editor.getByRole("group").waitFor();
  if ((await editor.getByRole("combobox").count()) > 0) {
    await editor.getByRole("combobox").click();
    await page.getByRole("option", { name: t(key) }).click();
  } else {
    await editor.getByRole("tab", { name: t(key) }).click();
  }
}

test("a node's card says when each section backs up and saves a change at once", async ({ page }) => {
  const calls = await open(page, "/instances");
  const online = card(page, NODE.id);
  await expect(plan(online, "nav.containers")).toContainText("2:00");
  await expect(plan(online, "nav.files")).toBeVisible();
  await plan(online, "nav.containers").click();
  await pickMode(page, "cadence.off");
  await expect.poll(() => calls.scheduled).toEqual([{ id: NODE.id, body: { containers: "off" } }]);
  await expect(plan(online, "nav.containers")).toContainText(t("cadence.off"));
  expect((await overflow(page)).pan).toBe(0);
});

test("the overview shows each node with its last backup, its errors and its anomalies", async ({ page }) => {
  await open(page, "/dashboard");
  const nodes = page.getByTestId("overview-nodes");
  await expect(nodes.locator("[data-node]")).toHaveCount(3);
  const online = nodes.locator(`[data-node="${NODE.id}"]`);
  await expect(online).toContainText(NODE.name);
  await expect(online).toContainText(t("instances.connected"));
  await expect(online).toContainText(t("containers.lastBackup"));
  await expect(online).toContainText(t("nav.anomalies"));
  const away = nodes.locator('[data-node="2b4d6f8a0c1e3a5b7c9d0e1f2a3b4c5d"]');
  await expect(away).toContainText(t("instances.inactive"));
  await expect(away.locator(".text-statusFail")).toHaveText("2");
  await expect(nodes.locator('[data-node="9e8d7c6b5a4f3e2d1c0b9a8f7e6d5c4b"]')).toContainText(t("containers.never"));
  expect((await overflow(page)).pan).toBe(0);
});

test("a node is added in three steps and turns green once it answers", async ({ page }, testInfo) => {
  const calls = await open(page, "/instances");
  await page.getByRole("button", { name: t("nodes.add") }).first().click();
  const box = page.getByRole("dialog", { name: t("nodes.add") });
  const desktop = !testInfo.project.name.startsWith("mobile");
  const widths: number[] = [];
  const measure = async () => {
    if (desktop) widths.push(Math.round((await box.boundingBox())!.width));
  };

  await expect(box).toBeVisible();
  await measure();
  await box.getByRole("textbox", { name: t("nodes.step.name") }).fill("vps-helsinki");
  await page.getByRole("button", { name: t("nodes.next") }).click();

  const address = page.getByLabel(t("nodes.address"));
  await address.fill("http://203.0.113.7:8000");
  await expect(page.getByRole("alert")).toHaveText(t("nodes.addressPlain"));
  await expect(page.getByRole("button", { name: t("nodes.next") })).toBeDisabled();
  await address.fill("https://backup.tower.example.org");
  await expect(page.getByRole("alert")).toHaveCount(0);
  await expect(plan(box, "nav.containers")).toContainText("2:00");
  await expect(plan(box, "nav.files")).toContainText("2:30");
  await measure();
  await page.getByRole("button", { name: t("nodes.next") }).click();

  const compose = page.getByTestId("node-compose");
  await expect(compose).toContainText(`APP_KEY: "${COMPOSE.APP_KEY}"`);
  await expect(compose).toContainText('NODE_NAME: "vps-helsinki"');
  await expect(compose).toContainText("image: junkerderprovinz/bombvault:latest");
  await measure();
  await expect(page.getByTestId("node-adoption")).toContainText(t("nodes.online").replace("{name}", "vps-helsinki"), { timeout: 15_000 });
  await expect(page.getByTestId("node-adoption")).toContainText(t("nodes.placed").replace("{target}", "https://backup.tower.example.org"));
  expect(calls.addresses).toEqual([{ url: "https://backup.tower.example.org", caPem: "" }]);
  expect(calls.placed).toEqual([{ id: "4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d", body: { kind: "receiver" } }]);
  expect(calls.scheduled).toEqual([{ id: "4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d", body: { containers: "daily 02:00", files: "daily 02:30" } }]);
  // Every step of the window keeps the one width the other windows have.
  if (desktop) expect(new Set(widths).size).toBe(1);
  if (desktop) expect(widths[0]).toBeLessThanOrEqual(768);
});

test("a node's containers stand in a section of their own and lead to its pages", async ({ page }) => {
  await open(page, "/backups?kind=container");
  const section = page.locator(`[data-node-section="${NODE.id}"]`);
  await expect(section).toContainText(NODE.name);
  for (const c of NODE_CONTAINERS.filter((c) => !c.self)) await expect(section.getByRole("link", { name: c.name, exact: true })).toBeVisible();
  await expect(section.getByRole("link", { name: "bombvault-node" })).toHaveCount(0);
  await expect(page.locator("[data-node-section]")).toHaveCount(2);
  expect((await overflow(page)).pan).toBe(0);

  await section.getByRole("link", { name: "nextcloud", exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`/nodes/${NODE.id}/backups/container/nextcloud$`));
});

test("an entry on a node lists its backups and restores one through the node", async ({ page }) => {
  const calls = await open(page, `/nodes/${NODE.id}/backups/container/nextcloud`);
  await expect(page.getByText(t("nodes.onNode").replace("{name}", NODE.name))).toBeVisible();
  const restores = page.getByRole("button", { name: t("snapshots.restore") });
  await expect(restores).toHaveCount(5);
  await restores.first().click();
  await page.getByRole("dialog").getByRole("button", { name: t("snapshots.restore") }).click();
  await expect(page.getByText(t("nodes.restoreStarted").replace("{name}", "nextcloud"))).toBeVisible();
  expect(calls.restored).toHaveLength(1);
  expect(calls.restored[0].path).toBe("/api/containers/nextcloud/restore");
  expect(calls.restored[0].body).toMatchObject({ confirm: true });
  expect((await overflow(page)).pan).toBe(0);
});

const SHOTS = process.env.NODE_SHOTS;
const LOOKS = ["light", "dark", "rainbow"] as const;
const WIDTHS = [1280, 375];

async function look(browser: Browser, width: number, theme: (typeof LOOKS)[number]): Promise<Page> {
  const context = await browser.newContext({
    viewport: { width, height: width > 600 ? 900 : 812 },
    colorScheme: theme === "light" ? "light" : "dark",
    reducedMotion: "reduce",
  });
  const page = await context.newPage();
  await stageDemo(page, {
    theme: theme === "light" ? "light" : "dark",
    stored: theme === "rainbow" ? { "bv-rainbow": JSON.stringify({ on: true, reactive: false }) } : {},
  });
  await stageNodes(page);
  return page;
}

async function shoot(page: Page, name: string) {
  await page.mouse.move(0, 0);
  await settle(page);
  await page.waitForTimeout(400);
  await page.screenshot({ path: join(SHOTS!, `${name}.png`), fullPage: true, animations: "disabled" });
}

test("pictures of the node pages", async ({ browser }, testInfo) => {
  test.skip(!SHOTS || testInfo.project.name !== "desktop-1280", "only when a folder for the pictures is named");
  test.setTimeout(600_000);
  mkdirSync(SHOTS!, { recursive: true });
  for (const theme of LOOKS) {
    for (const width of WIDTHS) {
      const at = `${theme}-${width}`;
      const page = await look(browser, width, theme);
      await page.goto("/instances");
      await card(page, NODE.id).scrollIntoViewIfNeeded();
      await shoot(page, `instances-node-cards-${at}`);
      await plan(card(page, NODE.id), "nav.containers").click();
      await page.getByRole("dialog", { name: t("nav.containers") }).waitFor();
      await shoot(page, `node-schedule-open-${at}`);
      await page.keyboard.press("Escape");

      await page.getByRole("button", { name: t("nodes.add") }).first().click();
      await page.getByRole("dialog", { name: t("nodes.add") }).getByRole("textbox", { name: t("nodes.step.name") }).fill("vps-helsinki");
      await shoot(page, `add-node-1-name-${at}`);
      await page.getByRole("button", { name: t("nodes.next") }).click();
      await page.getByLabel(t("nodes.address")).fill("http://203.0.113.7:8000");
      await shoot(page, `add-node-2-placement-${at}`);
      await page.getByLabel(t("nodes.address")).fill("https://backup.tower.example.org");
      await page.getByRole("button", { name: t("nodes.next") }).click();
      await shoot(page, `add-node-3-compose-${at}`);
      await expect(page.getByTestId("node-adoption")).toContainText(t("nodes.placed").replace("{target}", "https://backup.tower.example.org"), { timeout: 15_000 });
      await page.getByTestId("node-adoption").scrollIntoViewIfNeeded();
      await shoot(page, `add-node-3-online-${at}`);

      await page.goto("/dashboard");
      const nodes = page.getByTestId("overview-nodes");
      await nodes.locator("[data-node]").first().waitFor();
      await shoot(page, `overview-${at}`);
      const nodesCard = nodes.locator("xpath=ancestor::*[@data-grid-block][1]");
      await nodesCard.evaluate((el) => el.scrollIntoView({ block: "center" }));
      await settle(page);
      // The heading badge stands above the card's edge, so the picture reaches
      // a little higher than the card.
      const box = (await nodesCard.boundingBox())!;
      await page.screenshot({
        path: join(SHOTS!, `overview-nodes-card-${at}.png`),
        clip: { x: box.x, y: box.y - 16, width: box.width, height: box.height + 16 },
        animations: "disabled",
      });

      await page.goto("/backups?kind=container");
      await page.locator(`[data-node-section="${NODE.id}"]`).getByRole("link", { name: "nextcloud", exact: true }).waitFor();
      await shoot(page, `containers-node-section-${at}`);
      await page.goto("/backups?kind=files");
      await page.locator(`[data-node-section="${NODE.id}"]`).getByRole("link", { name: "Website uploads" }).waitFor();
      await shoot(page, `files-node-section-${at}`);
      await page.goto(`/nodes/${NODE.id}/backups/container/nextcloud`);
      await page.getByRole("button", { name: t("snapshots.restore") }).first().waitFor();
      await shoot(page, `node-entry-${at}`);
      await page.context().close();
    }
  }
});
