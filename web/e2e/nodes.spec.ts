// The node pages on the demo server: a card per node on Instances with when it
// backs up and where its backups go, the window that adds one, the nodes card
// on the overview, a section per node on the Backups list with the window that
// adds and edits a node's folder sets, and the page of an entry on a node with
// its backups and a restore. Every request about a node goes to its route
// under /api/nodes/{id}/api, never to this server's own.
//
// NODE_SHOTS=<folder> also writes the pictures of these pages in the light,
// dark and rainbow look at 1280 and 375 pixels:
//
//   NODE_SHOTS=../shots npx playwright test nodes --project=desktop-1280
import { expect, test, type Browser, type Page } from "@playwright/test";
import { mkdirSync } from "node:fs";
import { join } from "node:path";

import { FILES_DOCS, stageDemo, words } from "./demo";
import { overflow, settle } from "./entries";
import { COMPOSE, NODE, NODE_CONTAINERS, NODE_FILE_SETS, NODES, stageNodes } from "./nodes";

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

test("a node's card says it is a node in a badge before its state", async ({ page }) => {
  await open(page, "/instances");
  const online = card(page, NODE.id);
  const kind = online.getByText(t("nodes.kind"), { exact: true });
  const state = online.getByText(t("instances.connected"), { exact: true });
  await expect(kind).toBeVisible();
  const [k, s] = [(await kind.boundingBox())!, (await state.boundingBox())!];
  expect(k.x + k.width).toBeLessThanOrEqual(s.x);
  expect(Math.abs(k.y - s.y)).toBeLessThan(2);
  await expect(kind).toHaveCount(1);
});

test("the address and the certificate of the placement stand without a line between them", async ({ page }) => {
  await open(page, "/instances", "https://backup.tower.example.org");
  await card(page, NODE.id).getByRole("button", { name: t("nodes.editPlacement") }).click();
  const row = (label: string) => page.getByLabel(t(label)).locator("xpath=ancestor::*[contains(@class, 'glim-form-row')][1]");
  const border = (el: Element, side: string) => getComputedStyle(el).getPropertyValue(side);
  expect(await row("nodes.address").evaluate(border, "border-bottom-width")).toBe("0px");
  expect(await row("nodes.certificate").evaluate(border, "border-top-width")).toBe("0px");
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
  await expect(box.getByRole("button", { name: `${t("nodes.startAt")}: 03:00` })).toBeVisible();
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
  expect(calls.scheduled).toEqual([{ id: "4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d", body: { containers: "daily 03:00", files: "daily 03:00" } }]);
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

test("a node's folder sets are added and edited in its section through the node", async ({ page }) => {
  const calls = await open(page, "/backups?kind=files");
  const section = page.locator(`[data-node-section="${NODE.id}"]`);
  const uploads = section.getByRole("link", { name: "Website uploads" });
  await uploads.waitFor();

  await section.getByRole("button", { name: t("files.addSet") }).click();
  const add = page.getByRole("dialog", { name: t("nodes.addSet").replace("{name}", NODE.name) });
  await expect(add.getByRole("group", { name: t("storage.title") })).toHaveCount(0);
  await add.getByPlaceholder("documents").fill("Photos");
  await add.getByRole("button", { name: t("folder.browseTitle") }).click();
  await add.getByRole("button", { name: "srv", exact: true }).click();
  await add.getByRole("button", { name: "photos", exact: true }).click();
  await expect(add.getByRole("button", { name: t("folder.newFolder") })).toHaveCount(0);
  // The window's own Cancel and Save stick to its bottom, so on a phone the
  // picker's button has to clear them without scrolling.
  const use = add.getByRole("button", { name: t("folder.use") });
  expect(await use.evaluate((el) => {
    const r = el.getBoundingClientRect();
    const hit = document.elementFromPoint(r.left + r.width / 2, r.bottom - 2);
    return hit !== null && el.contains(hit);
  })).toBe(true);
  await add.getByRole("button", { name: t("folder.use") }).click();
  await expect(add.getByPlaceholder("user/appdata")).toHaveValue("srv/photos");
  expect((await overflow(page)).pan).toBe(0);
  await add.getByRole("button", { name: t("settings.save") }).click();
  await expect(add).toHaveCount(0);

  await uploads.click({ button: "right" });
  await page.getByRole("menuitem", { name: t("common.edit") }).click();
  const edit = page.getByRole("dialog", { name: t("nodes.editSet").replace("{name}", NODE.name) });
  await expect(edit.getByPlaceholder("documents")).toBeDisabled();
  await edit.locator("textarea").fill("cache/");
  await edit.getByRole("button", { name: t("settings.save") }).click();
  await expect(edit).toHaveCount(0);

  expect(calls.sets).toEqual([
    { method: "POST", path: "/api/files/sets", body: { name: "Photos", path: "srv/photos", excludes: [], enabled: true } },
    { method: "PATCH", path: `/api/files/sets/${NODE_FILE_SETS[0].id}`, body: { name: "Website uploads", path: "srv/www/uploads", excludes: ["cache/"], enabled: true } },
  ]);
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

// keepOnNode opens the placement step of a new node and picks keeping the
// backups on the node itself.
async function keepOnNode(page: Page) {
  await page.getByRole("button", { name: t("nodes.add") }).first().click();
  await page.getByRole("dialog", { name: t("nodes.add") }).getByRole("textbox", { name: t("nodes.step.name") }).fill("vps-helsinki");
  await page.getByRole("button", { name: t("nodes.next") }).click();
  await page.getByRole("radio", { name: new RegExp(`^${t("nodes.toNode")}`) }).click();
}

// onNode stages the online node as one that keeps its own backups.
async function onNode(page: Page) {
  await page.route(
    (url) => url.pathname === "/api/nodes",
    (route) => route.fulfill({ json: { ok: true, nodes: [{ ...NODE, placement: "node", placementTarget: KEPT_AT }, ...NODES.slice(1)] } }),
  );
}

const KEPT_AT = "https://203.0.113.7:8443";

test("a node keeps its own backups at an address checked as it is typed", async ({ page }) => {
  const calls = await open(page, "/instances");
  await keepOnNode(page);
  const box = page.getByRole("dialog", { name: t("nodes.add") });
  await expect(box).toContainText(t("nodes.portNote"));
  const address = page.getByLabel(t("nodes.nodeAddress"));
  await address.fill("http://203.0.113.7:8000");
  await expect(page.getByRole("alert")).toHaveText(t("nodes.fetchPlain"));
  await expect(page.getByRole("button", { name: t("nodes.next") })).toBeDisabled();
  await address.fill(KEPT_AT + "/backups");
  await expect(page.getByRole("alert")).toHaveText(t("nodes.addressForm"));
  await address.fill(KEPT_AT);
  await expect(page.getByRole("alert")).toHaveCount(0);
  expect((await overflow(page)).pan).toBe(0);
  await page.getByRole("button", { name: t("nodes.next") }).click();
  await expect(page.getByTestId("node-adoption")).toContainText(t("nodes.placedOnNode").replace("{target}", KEPT_AT), { timeout: 15_000 });
  expect(calls.placed).toEqual([{ id: "4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d", body: { kind: "node", address: KEPT_AT } }]);
  expect(calls.addresses).toEqual([]);
});

test("a node that keeps its own backups shows where on its card and keeps its address when placed again", async ({ page }) => {
  await stageDemo(page, { theme: "dark" });
  const calls = await stageNodes(page);
  await onNode(page);
  await page.goto("/instances");
  const online = card(page, NODE.id);
  await expect(online).toContainText(t("nodes.fig.onNode"));
  await expect(online).toContainText(KEPT_AT);
  await online.getByRole("button", { name: t("nodes.editPlacement") }).click();
  const box = page.getByRole("dialog", { name: t("nodes.editPlacement") });
  await expect(box.getByRole("radio", { name: new RegExp(`^${t("nodes.toNode")}`) })).toHaveAttribute("aria-checked", "true");
  await expect(page.getByLabel(t("nodes.nodeAddress"))).toHaveValue(KEPT_AT);
  await box.getByRole("button", { name: t("settings.save") }).click();
  await expect.poll(() => calls.placed).toEqual([{ id: NODE.id, body: { kind: "node", address: KEPT_AT } }]);
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
      const startAt = page.getByRole("button", { name: `${t("nodes.startAt")}: 03:00` });
      await startAt.click();
      await page.getByRole("listbox", { name: t("timePicker.hour") }).waitFor();
      await shoot(page, `add-node-2-start-time-${at}`);
      // A second tap on the trigger lands on the picker's backdrop and closes it.
      await startAt.click({ force: true });
      await page.getByRole("listbox", { name: t("timePicker.hour") }).waitFor({ state: "detached" });
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

test("pictures of a node's folder set window", async ({ browser }, testInfo) => {
  test.skip(!SHOTS || testInfo.project.name !== "desktop-1280", "only when a folder for the pictures is named");
  test.setTimeout(300_000);
  mkdirSync(SHOTS!, { recursive: true });
  for (const theme of LOOKS) {
    for (const width of WIDTHS) {
      const at = `${theme}-${width}`;
      const page = await look(browser, width, theme);
      await page.goto("/backups?kind=files");
      const section = page.locator(`[data-node-section="${NODE.id}"]`);
      await section.getByRole("link", { name: "Website uploads" }).waitFor();
      await section.getByRole("button", { name: t("files.addSet") }).click();
      const add = page.getByRole("dialog", { name: t("nodes.addSet").replace("{name}", NODE.name) });
      await add.getByPlaceholder("documents").fill("Photos");
      await add.getByRole("button", { name: t("folder.browseTitle") }).click();
      await add.getByRole("button", { name: "srv", exact: true }).click();
      await add.getByRole("button", { name: "photos", exact: true }).waitFor();
      await shoot(page, `node-folder-set-browse-${at}`);
      await add.getByRole("button", { name: "photos", exact: true }).click();
      await add.getByRole("button", { name: t("folder.use") }).click();
      await shoot(page, `node-folder-set-add-${at}`);
      await add.getByRole("button", { name: t("files.cancel") }).click();
      await section.getByRole("link", { name: "Website uploads" }).click({ button: "right" });
      await page.getByRole("menuitem", { name: t("common.edit") }).click();
      await page.getByRole("dialog", { name: t("nodes.editSet").replace("{name}", NODE.name) }).waitFor();
      await shoot(page, `node-folder-set-edit-${at}`);
      await page.context().close();
    }
  }
});

// openFolderWindow opens the folder set window on home or on the node and
// browses into a folder, the way the pictures above do.
async function openFolderWindow(page: Page, scope: "home" | "node") {
  if (scope === "home") {
    await page.route(
      (url) => url.pathname === "/api/browse",
      (route) => {
        const at = new URL(route.request().url()).searchParams.get("path") ?? "";
        const dirs = ["appdata", "photos", "www"].map((name) => ({ name, path: at ? `${at}/${name}` : name }));
        return route.fulfill({ json: { ok: true, root: "/mnt/user", path: at, dirs, status: "ok", truncated: false } });
      },
    );
    await page.goto(`/backups/files/${FILES_DOCS}?tab=settings`);
    await page.getByRole("button", { name: t("common.edit") }).click();
    const edit = page.getByRole("dialog", { name: t("files.editSet") });
    await edit.getByRole("button", { name: t("folder.browseTitle") }).click();
    await edit.getByRole("button", { name: "photos", exact: true }).click();
    await edit.getByRole("button", { name: "www", exact: true }).waitFor();
    return edit;
  }
  await page.goto("/backups?kind=files");
  const section = page.locator(`[data-node-section="${NODE.id}"]`);
  await section.getByRole("link", { name: "Website uploads" }).waitFor();
  await section.getByRole("button", { name: t("files.addSet") }).click();
  const add = page.getByRole("dialog", { name: t("nodes.addSet").replace("{name}", NODE.name) });
  await add.getByRole("button", { name: t("folder.browseTitle") }).click();
  await add.getByRole("button", { name: "srv", exact: true }).click();
  await add.getByRole("button", { name: "photos", exact: true }).waitFor();
  return add;
}

for (const scope of ["home", "node"] as const) {
  test(`the folder window on ${scope} keeps "Use this folder" clear of the window's foot on a phone`, async ({ browser }, testInfo) => {
    test.skip(testInfo.project.name !== "desktop-1280", "opens its own phone-sized pages");
    for (const theme of ["light", "dark"] as const) {
      const page = await look(browser, 375, theme);
      const dialog = await openFolderWindow(page, scope);
      const use = dialog.getByRole("button", { name: t("folder.use") });
      await expect(use).toBeVisible();
      // The window's body keeps 1rem below its last row, so the button rests at
      // least that far above the foot that holds Cancel and Save.
      await expect
        .poll(async () => {
          const button = (await use.boundingBox())!;
          const foot = (await dialog.locator("[data-dialog-actions]").boundingBox())!;
          return button.y >= 0 && button.y + button.height <= foot.y - 16 && foot.y + foot.height <= 812;
        })
        .toBe(true);
      const sideways = await dialog.evaluate((el) => {
        const vw = window.innerWidth;
        const scrollers = [el, ...el.querySelectorAll("*")].filter((node) => ["auto", "scroll"].includes(getComputedStyle(node).overflowX));
        return {
          pan: Math.max(0, ...scrollers.map((node) => node.scrollWidth - node.clientWidth)),
          past: [...el.querySelectorAll("button, input")].filter((node) => {
            const r = node.getBoundingClientRect();
            return r.width > 0 && (r.right > vw + 1 || r.left < -1);
          }).length,
        };
      });
      expect(sideways).toEqual({ pan: 0, past: 0 });
      expect((await overflow(page)).pan).toBe(0);
      if (SHOTS) {
        mkdirSync(SHOTS, { recursive: true });
        await page.mouse.move(0, 0);
        await settle(page);
        await page.screenshot({ path: join(SHOTS, `folder-window-footer-${scope}-${theme}-375.png`), animations: "disabled" });
      }
      await page.context().close();
    }
  });
}

test("pictures of a node that keeps its own backups", async ({ browser }, testInfo) => {
  test.skip(!SHOTS || testInfo.project.name !== "desktop-1280", "only when a folder for the pictures is named");
  test.setTimeout(300_000);
  mkdirSync(SHOTS!, { recursive: true });
  for (const theme of LOOKS) {
    for (const width of WIDTHS) {
      const at = `${theme}-${width}`;
      const page = await look(browser, width, theme);
      await page.goto("/instances");
      await keepOnNode(page);
      await page.getByLabel(t("nodes.nodeAddress")).fill(KEPT_AT);
      await shoot(page, `keep-on-node-add-step-2-${at}`);
      await page.getByLabel(t("nodes.nodeAddress")).fill("http://203.0.113.7:8000");
      await shoot(page, `keep-on-node-add-step-2-refused-${at}`);
      await page.context().close();

      const placed = await look(browser, width, theme);
      await onNode(placed);
      await placed.goto("/instances");
      await card(placed, NODE.id).scrollIntoViewIfNeeded();
      await shoot(placed, `keep-on-node-card-${at}`);
      await card(placed, NODE.id).getByRole("button", { name: t("nodes.editPlacement") }).click();
      await placed.getByLabel(t("nodes.nodeAddress")).waitFor();
      await shoot(placed, `keep-on-node-edit-placement-${at}`);
      await placed.context().close();
    }
  }
});
