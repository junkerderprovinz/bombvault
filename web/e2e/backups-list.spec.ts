// The Backups page: one list for everything that is backed up. A fresh harness
// database has no entries of any kind, so the list, the settings switches and
// the anomaly figures are staged at the route layer. On the desktop: tiles,
// the bar above the list, rows that open their kind's page, and the page
// actions in their corner. On the phones: tiles as chips, no run strip, the
// actions as glyphs above the bottom bar, and nothing past the edge. German,
// because its labels run longest.
import { expect, test, type Page } from "@playwright/test";
import { mkdirSync } from "node:fs";
import { join } from "node:path";

const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

// Set to a folder to keep pictures of the page at the widths and themes the
// design is compared at.
const SHOTS = process.env.BACKUPS_SHOTS;

// A handler can still wait on the real server when its test has passed;
// what it throws then is dropped.
test.afterEach(async ({ page }) => {
  await page.unrouteAll({ behavior: "ignoreErrors" });
});

const NOW = Math.floor(Date.now() / 1000);
const HOUR = 3600;
const DAY = 86400;
const GB = 1024 ** 3;

const DAILY = { kind: "domain", spec: "daily@03:30", alsoSpec: "" };
const EVERYTHING = { kind: "everything", spec: "daily@03:00", alsoSpec: "" };

function runs(statuses: string[], prefix: string) {
  return statuses.map((status, i) => ({
    id: `${prefix}-${i}`,
    kind: "backup",
    status,
    startedAt: NOW - 4 * HOUR - i * DAY,
  }));
}

const OK14 = Array<string>(14).fill("success");

function entry(over: Record<string, unknown>) {
  const key = String(over.key);
  return {
    kind: "container",
    name: key,
    included: true,
    paused: false,
    effectiveSchedule: EVERYTHING,
    lastBackup: NOW - 4 * HOUR,
    lastRunStatus: "success",
    sourceBytes: 2 * GB,
    state: "running",
    installed: true,
    runs: runs(OK14, key),
    ...over,
  };
}

const LONG_NAME = "nextcloud-aio-datenbank-hauptserver-produktivsystem";

const ITEMS = [
  entry({ kind: "config", key: "config", name: "config", state: undefined, installed: undefined }),
  entry({ kind: "vm", key: "debian", name: "Debian Docker-Host", state: "shut off", effectiveSchedule: { kind: "own", spec: "weekly@0@04:00", alsoSpec: "" }, lastBackup: NOW - 3 * DAY, runs: runs(OK14.slice(0, 8), "debian") }),
  entry({ kind: "files", key: "a1", name: "Dokumente", state: undefined, installed: undefined, effectiveSchedule: DAILY, runs: runs(OK14.slice(0, 7), "dokumente") }),
  entry({ kind: "files", key: "a2", name: "Fotos", state: undefined, installed: undefined, sourceBytes: 180 * GB }),
  entry({ key: "gitea", lastBackup: 0, lastRunStatus: "", sourceBytes: null, included: false, effectiveSchedule: { kind: "none", spec: "", alsoSpec: "", reason: "excluded" }, runs: [] }),
  entry({ key: "immich", sourceBytes: 40 * GB }),
  entry({ key: "jellyfin", runs: runs(["success", "success", "success", "cancelled", ...OK14.slice(0, 10)], "jellyfin") }),
  entry({ key: LONG_NAME }),
  entry({ key: "paperless-ngx", state: "exited", lastRunStatus: "failed", lastBackup: NOW - DAY - 4 * HOUR, runs: runs(["failed", ...OK14.slice(0, 5)], "paperless") }),
  entry({ key: "uptime-kuma", lastBackup: 0, lastRunStatus: "", sourceBytes: null, runs: [] }),
  entry({ kind: "zfs", key: "z1", name: "tank/appdata", state: undefined, installed: undefined }),
  entry({ kind: "flash", key: "flash", name: "flash", state: undefined, installed: undefined }),
  entry({ kind: "vm", key: "win11", name: "Windows 11" }),
  entry({ key: "bombvault", self: true, lastBackup: 0, lastRunStatus: "", sourceBytes: null, runs: [] }),
  entry({ key: "pihole", installed: false, state: undefined, included: false, lastBackup: NOW - 28 * DAY }),
  entry({ kind: "vm", key: "win10", name: "Windows 10", installed: false, state: undefined, included: false, lastBackup: NOW - 60 * DAY }),
];

const ENTRY_COUNT = ITEMS.length - 1;

function watched(targetId: string, domain: string, name: string, warning: number) {
  return {
    targetId,
    domain,
    name,
    scheduled: true,
    sensitivity: "",
    effective: "balanced",
    notifyMin: "",
    effectiveNotifyMin: "warning",
    learning: { samples: 10, needed: 10, newData: 10, source: 10, duration: 10, noData: false },
    typical: { sourceBytes: GB, newDataBytes: 1024 ** 2, resticMs: 30000 },
    dump: null,
    datasets: [],
    open: { critical: 0, warning, info: 0 },
    retentionHeld: false,
    selectionSince: 0,
    expectations: [],
  };
}

const WATCHED = [watched("tg-immich", "container", "immich", 1), watched("tg-fotos", "files", "Fotos", 0)];

interface Stage {
  items?: typeof ITEMS;
  unlisted?: string[];
  theme?: "dark" | "light";
}

async function stage(page: Page, s: Stage = {}): Promise<{ posts: string[] }> {
  const posts: string[] = [];
  await page.route("**/api/items", (route) =>
    route.fulfill({ json: { ok: true, items: s.items ?? ITEMS, unlisted: s.unlisted ?? [] } }),
  );
  await page.route("**/api/settings", async (route) => {
    if (route.request().method() !== "GET") return route.continue();
    const res = await route.fetch();
    const body = await res.json();
    await route.fulfill({
      json: {
        ...body,
        settings: {
          ...body.settings,
          containersEnabled: true,
          vmsEnabled: true,
          flashEnabled: true,
          filesEnabled: true,
          zfsEnabled: true,
          configEnabled: true,
        },
      },
    });
  });
  await page.route("**/api/anomalies/summary", (route) =>
    route.fulfill({
      json: {
        ok: true,
        summary: {
          enabled: true,
          ready: true,
          generation: 3,
          open: { critical: 0, warning: 1, info: 0 },
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
  await page.route("**/api/anomalies/items", (route) => route.fulfill({ json: { ok: true, items: WATCHED } }));
  await page.route(
    (url) => url.pathname === "/api/anomalies",
    (route) => route.fulfill({ json: { ok: true, anomalies: [], nextCursor: "" } }),
  );
  for (const path of ["**/api/discover", "**/api/vms/discover", "**/api/files/discover", "**/api/zfs/discover"]) {
    await page.route(path, (route) => {
      posts.push(new URL(route.request().url()).pathname);
      return route.fulfill({ json: { ok: true, discovered: 0 } });
    });
  }
  await page.route("**/api/backup-everything", (route) => {
    posts.push("/api/backup-everything");
    return route.fulfill({ json: { ok: true, started: true } });
  });
  await page.route("**/api/containers/backup-all", (route) => {
    posts.push(`/api/containers/backup-all ${route.request().postData() ?? ""}`);
    return route.fulfill({ json: { ok: true, started: 1 } });
  });
  // The stored look is the one under test, so the server's must not replace it.
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.addInitScript((theme) => {
    window.localStorage.setItem("bv-lang", "de");
    window.localStorage.setItem("bv-theme", theme);
  }, s.theme ?? "dark");
  return { posts };
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

async function openList(page: Page, url = "/backups"): Promise<void> {
  await page.goto(url);
  await expect(page.getByRole("heading", { level: 1, name: "Sicherungen" })).toBeAttached();
  await expect(page.getByTestId("backup-row").first()).toBeVisible();
  await settle(page);
}

const rows = (page: Page) => page.getByTestId("backup-row");
const tile = (page: Page, name: RegExp) =>
  page.getByRole("group", { name: "Was die Liste zeigt" }).getByRole("button", { name });

test("the list shows every kind with its count, its last runs and one status", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop list");
  await stage(page);
  await openList(page);

  await expect(tile(page, /^Alle/)).toContainText(`${ENTRY_COUNT} Einträge`);
  await expect(tile(page, /^Container/)).toContainText("7 Einträge");
  await expect(tile(page, /^Flash/)).toContainText("1 Eintrag");
  await expect(tile(page, /^Nicht installiert/)).toContainText("2 Einträge");
  await expect(rows(page)).toHaveCount(ENTRY_COUNT);

  const immich = rows(page).filter({ hasText: "immich" });
  await expect(immich.getByRole("link", { name: "Anomalie" })).toBeVisible();
  await expect(rows(page).filter({ hasText: "paperless-ngx" })).toContainText("Fehlgeschlagen");
  await expect(rows(page).filter({ hasText: "gitea" })).toContainText("Nicht geschützt");
  await expect(rows(page).filter({ hasText: "uptime-kuma" })).toContainText("Wartet auf die erste Sicherung");
  await expect(page.getByTestId("backup-self-row")).toContainText("Sichert sich nicht selbst");
  // The strip needs room beside the name, which the column beside the rail
  // at 768px does not have.
  const strip = rows(page).filter({ hasText: "paperless-ngx" }).getByRole("img");
  if (testInfo.project.name === "desktop-1280") {
    await expect(strip).toHaveAttribute("aria-label", "Letzte Läufe: 5 ok, 1 fehlgeschlagen");
  } else {
    await expect(strip).toBeHidden();
  }

  // What is gone from the host stands last, under its own heading.
  await expect(page.getByRole("heading", { name: "Nicht installiert (nur Backups)" })).toBeVisible();
  await expect(rows(page).last()).toContainText(/pihole|Windows 10/);
});

test("a tile narrows the list and stays in the address", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop list");
  await stage(page);
  await openList(page);

  await tile(page, /^VMs/).click();
  await expect(page).toHaveURL(/\/backups\?kind=vm$/);
  await expect(rows(page)).toHaveCount(3);
  await expect(tile(page, /^VMs/)).toHaveAttribute("aria-pressed", "true");
  // The tiles stay where they are: choosing one hides nothing but rows.
  await expect(tile(page, /^Container/)).toBeVisible();

  await page.reload();
  await expect(rows(page)).toHaveCount(3);
  await tile(page, /^Alle/).click();
  await expect(page).toHaveURL(/\/backups$/);
  await expect(rows(page)).toHaveCount(ENTRY_COUNT);
});

test("filter, sort and search work on the list", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop list");
  await stage(page);
  await openList(page);

  await page.getByRole("button", { name: "Filter" }).click();
  const filters = page.getByRole("dialog", { name: "Filter" });
  await filters.getByRole("tab", { name: "Nie gesichert" }).click();
  await expect(rows(page)).toHaveCount(2);
  await filters.getByRole("tab", { name: "Alle" }).nth(1).click();
  await page.keyboard.press("Escape");
  await expect(rows(page)).toHaveCount(ENTRY_COUNT);

  await page.getByRole("button", { name: "Name A bis Z" }).click();
  await page.getByRole("option", { name: "Größe, größte zuerst" }).click();
  await expect(rows(page).first()).toContainText("Fotos");
  await page.getByRole("button", { name: "Größe, größte zuerst" }).click();
  await page.getByRole("option", { name: "Name A bis Z" }).click();

  await page.getByRole("button", { name: "Suchen" }).click();
  const search = page.getByRole("searchbox", { name: "Suchen" });
  await expect(search).toBeFocused();
  await search.fill("win");
  await expect(rows(page)).toHaveCount(2);
  await search.fill("zzz");
  await expect(page.getByText("Keine Einträge entsprechen den aktuellen Filtern.")).toBeVisible();
  await search.press("Escape");
  await expect(rows(page)).toHaveCount(ENTRY_COUNT);
});

test("a row opens the page of its kind", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop list");
  await stage(page);
  await openList(page);
  // On the glyph and not on the name, to show the whole row is the link.
  await rows(page).filter({ hasText: "Dokumente" }).locator("svg").first().click();
  await expect(page).toHaveURL(/\/files$/);
});

test("the page actions rest in the content's corner and cover no row", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop corner");
  const { posts } = await stage(page);
  await openList(page);

  const actions = page.getByTestId("page-actions");
  const add = actions.getByRole("button", { name: "Hinzufügen" });
  const card = page.getByRole("heading", { level: 2, name: "Alle Einträge" }).locator("xpath=..");
  const box = async (l: ReturnType<Page["locator"]>) => (await l.boundingBox())!;

  expect(await actions.evaluate((el) => getComputedStyle(el).position)).toBe("sticky");
  expect(Math.abs((await box(add)).x + (await box(add)).width - ((await box(card)).x + (await box(card)).width))).toBeLessThanOrEqual(1);

  // While rows are left below, the corner floats over them at the window's
  // bottom. At the end it stands below the last row.
  const viewport = page.viewportSize()!;
  expect((await box(add)).y + (await box(add)).height).toBeLessThanOrEqual(viewport.height);
  await page.locator("#bv-main").evaluate((main) => main.scrollTo(0, main.scrollHeight));
  const last = await box(rows(page).last());
  expect((await box(add)).y).toBeGreaterThanOrEqual(last.y + last.height);

  await actions.getByRole("button", { name: "Jetzt alles sichern" }).click();
  await expect.poll(() => posts).toContain("/api/backup-everything");
  // The toast that answers stands above the corner, not on it.
  const toast = await box(page.getByRole("status").filter({ hasText: "Gestartet" }));
  expect(toast.y + toast.height).toBeLessThanOrEqual((await box(add)).y);
  await actions.getByRole("button", { name: "Backups entdecken" }).click();
  await expect.poll(() => posts).toContain("/api/zfs/discover");

  await tile(page, /^Container/).click();
  await actions.getByRole("button", { name: "Alle jetzt sichern" }).click();
  // What is paused, gone or BombVault itself is left out.
  await expect
    .poll(() => posts.find((p) => p.startsWith("/api/containers/backup-all")))
    .toBe(`/api/containers/backup-all ${JSON.stringify({ names: ["immich", "jellyfin", LONG_NAME, "paperless-ngx", "uptime-kuma"] })}`);

  await tile(page, /^Ordner/).click();
  await actions.getByRole("button", { name: "Ordner hinzufügen" }).click();
  await expect(page).toHaveURL(/\/files$/);
});

test("a kind the host did not answer for is named above the list", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop list");
  await stage(page, { unlisted: ["container", "vm"] });
  await openList(page);
  await expect(page.getByText("Container und VMs ließen sich auf diesem Server nicht auslesen.", { exact: false })).toBeVisible();
});

for (const width of [320, 390]) {
  test(`the list @ ${width}px: chips, no strip, glyph actions, nothing past the edge`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "the phone layout lives below 48rem");
    await stage(page);
    await page.setViewportSize({ width, height: 800 });
    await openList(page);

    const tiles = page.getByRole("group", { name: "Was die Liste zeigt" });
    const tops = await tiles.getByRole("button").evaluateAll((all) => all.map((el) => Math.round(el.getBoundingClientRect().top)));
    expect(new Set(tops).size, "the chips stand in one row").toBe(1);

    const first = rows(page).first();
    await expect(first.getByRole("img")).toBeHidden();
    await expect(rows(page).filter({ hasText: "paperless-ngx" })).toContainText("Fehlgeschlagen");

    const actions = page.getByTestId("page-actions");
    const add = actions.getByRole("button", { name: "Hinzufügen" });
    const box = (await add.boundingBox())!;
    expect(box.width).toBe(box.height);
    const nav = (await page.getByRole("navigation", { name: "Mobile Navigation" }).boundingBox())!;
    expect(box.y + box.height).toBeLessThanOrEqual(nav.y);

    const pan = await page.evaluate(() => {
      const main = document.querySelector("#bv-main")!;
      return Math.max(document.documentElement.scrollWidth - window.innerWidth, main.scrollWidth - main.clientWidth);
    });
    expect(pan, "the page scrolls sideways").toBeLessThanOrEqual(1);
    const past = await rows(page).evaluateAll((all) =>
      all.filter((el) => el.getBoundingClientRect().right > window.innerWidth + 1).map((el) => el.textContent),
    );
    expect(past, "rows run past the edge").toEqual([]);
  });
}

test("pictures of the page", async ({ page }, testInfo) => {
  test.skip(!SHOTS || testInfo.project.name !== "desktop-1280", "only when a folder for them is named");
  mkdirSync(SHOTS!, { recursive: true });
  for (const [name, width, theme] of [
    ["1440-dark", 1440, "dark"],
    ["1440-light", 1440, "light"],
    ["390", 390, "dark"],
  ] as const) {
    await page.unrouteAll({ behavior: "ignoreErrors" });
    await stage(page, { theme });
    await page.setViewportSize({ width, height: width < 500 ? 844 : 1000 });
    await openList(page);
    await page.screenshot({ path: join(SHOTS!, `all-viewport-${name}.png`) });
    await page.locator("#bv-main").evaluate((main) => main.scrollTo(0, main.scrollHeight));
    await page.screenshot({ path: join(SHOTS!, `all-end-${name}.png`) });
  }
});
