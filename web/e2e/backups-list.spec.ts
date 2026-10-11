// The Backups page: one list for everything that is backed up. A fresh harness
// database has no entries of any kind, so the list, the settings switches and
// the anomaly figures are staged at the route layer. On the desktop: tiles,
// the bar above the list, rows that open their entry's page and are picked
// like files, the menus, the windows and the page actions in their corner. On
// the phones: tiles as chips, no run strip, the actions as glyphs above the
// bottom bar, a long press to pick, and nothing past the edge. German, because
// its labels run longest.
import { expect, test, type Locator, type Page, type Route } from "@playwright/test";
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

const DAILY = { kind: "domain", spec: "daily@03:30" };
const EVERYTHING = { kind: "everything", spec: "daily@03:00" };

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
  entry({ kind: "vm", key: "debian", name: "Debian Docker-Host", state: "shut off", effectiveSchedule: { kind: "own", spec: "weekly@0@04:00" }, lastBackup: NOW - 3 * DAY, runs: runs(OK14.slice(0, 8), "debian") }),
  entry({ kind: "files", key: "a1", name: "Dokumente", state: undefined, installed: undefined, effectiveSchedule: DAILY, runs: runs(OK14.slice(0, 7), "dokumente") }),
  entry({ kind: "files", key: "a2", name: "Fotos", state: undefined, installed: undefined, sourceBytes: 180 * GB }),
  entry({ key: "gitea", lastBackup: 0, lastRunStatus: "", sourceBytes: null, included: false, effectiveSchedule: { kind: "none", spec: "", reason: "excluded" }, runs: [], renameFrom: "pihole", renameReason: "appdata-bind" }),
  entry({ key: "immich", sourceBytes: 40 * GB, stack: "immich", ip: "192.168.20.40", image: "ghcr.io/immich-app/immich-server:release" }),
  entry({ key: "immich_postgres", stack: "immich", ip: "192.168.20.41", image: "postgres:16", dbTier: "curated", dbDataCoverage: "stopped" }),
  entry({
    key: "jellyfin",
    ip: "192.168.20.35",
    image: "linuxserver/jellyfin:10.10.3",
    changedSinceBackup: [{ field: "image", change: "changed", now: "linuxserver/jellyfin:10.10.3", backup: "linuxserver/jellyfin:10.9.11" }],
    runs: runs(["success", "success", "success", "cancelled", ...OK14.slice(0, 10)], "jellyfin"),
  }),
  entry({ key: LONG_NAME }),
  entry({ key: "paperless-ngx", state: "exited", lastRunStatus: "failed", lastRunError: "restic: the repository is locked", lastBackup: NOW - DAY - 4 * HOUR, runs: runs(["failed", ...OK14.slice(0, 5)], "paperless") }),
  entry({ key: "uptime-kuma", lastBackup: 0, lastRunStatus: "", sourceBytes: null, runs: [], formerNames: ["bitwarden"] }),
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
  /** What the ZFS connection test answers, "ok" unless said otherwise. */
  zfsCode?: string;
}

function dataset(name: string, over: Record<string, unknown> = {}) {
  return {
    dataset: name,
    type: "filesystem",
    hostMountpoint: `/mnt/${name}`,
    referenced: GB,
    used: 18 * GB,
    usedByDataset: 18 * GB,
    mounted: true,
    encrypted: false,
    keyLoaded: false,
    visible: true,
    writable: true,
    memberCode: "",
    managedId: "",
    coveredBy: "",
    vmDisk: false,
    system: false,
    vmVolume: false,
    blockers: [],
    ...over,
  };
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
  for (const kind of ["containers", "vms", "files", "zfs"]) {
    await page.route(`**/api/${kind}/backup-all`, (route) => {
      posts.push(`/api/${kind}/backup-all ${route.request().postData() ?? ""}`);
      return route.fulfill({ json: { ok: true, started: 1 } });
    });
  }
  for (const kind of ["flash", "config"]) {
    await page.route(`**/api/${kind}/backup`, (route) => route.fulfill({ json: { ok: true, started: true } }));
  }
  const patched = (route: Route) => {
    const req = route.request();
    if (req.method() !== "PATCH") return route.fallback();
    posts.push(`PATCH ${new URL(req.url()).pathname} ${req.postData() ?? ""}`);
    return route.fulfill({ json: { ok: true } });
  };
  await page.route("**/api/containers/*", patched);
  await page.route("**/api/vms/*", patched);
  await page.route("**/api/zfs/datasets/*", patched);
  await page.route("**/api/zfs/connection", (route) =>
    route.fulfill({
      json: {
        ok: true,
        code: s.zfsCode ?? "ok",
        target: "root@192.168.1.20",
        uriTarget: "",
        version: "2.2.6",
        detail: "",
        zfsBinary: "/usr/sbin/zfs",
        propagation: "",
        unpropagated: [],
      },
    }),
  );
  await page.route("**/api/zfs/host", (route) =>
    route.fulfill({
      json: {
        ok: true,
        available: true,
        code: "ok",
        target: "root@192.168.1.20",
        datasets: [dataset("tank"), dataset("tank/appdata", { managedId: "z1" }), dataset("tank/media"), dataset("tank/isos")],
        hiddenLegacy: 0,
        unusedZvols: 0,
        notInItem: 4,
        truncated: false,
        maxNameLength: 200,
      },
    }),
  );
  await page.route("**/api/browse*", (route) => {
    const path = new URL(route.request().url()).searchParams.get("path") ?? "";
    const dirs = path === "user" ? ["dokumente", "filme", "media"].map((name) => ({ name, path: `user/${name}` })) : [];
    return route.fulfill({ json: { ok: true, status: "ok", truncated: false, path, dirs } });
  });
  await page.route("**/api/files", (route) =>
    route.fulfill({
      json: {
        ok: true,
        fileSets: [{ id: "a1", name: "Dokumente", path: "user/dokumente", excludes: [], enabled: true, lastBackup: NOW - 4 * HOUR }],
      },
    }),
  );
  await page.route("**/api/files/sets/preset", (route) =>
    route.fulfill({ json: { ok: true, offered: false, name: "", path: "", excludes: [] } }),
  );
  await page.route("**/api/files/sets", (route) => {
    posts.push(`POST /api/files/sets ${route.request().postData() ?? ""}`);
    return route.fulfill({ json: { ok: true, id: "a9" } });
  });
  await page.route("**/api/restore/check", (route) =>
    route.fulfill({ json: { ok: true, ready: true, checks: [], members: [] } }),
  );
  await page.route("**/api/stacks/*/restore*", (route) => {
    posts.push(`${new URL(route.request().url()).pathname} ${route.request().postData() ?? ""}`);
    return route.fulfill({ json: { ok: true, started: true } });
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

// A menu at the pointer closes when the page scrolls under it, and a click
// that first has to bring its row into view scrolls. So the row is brought
// into view before the click.
async function rightClick(page: Page, row: Locator, position?: { x: number; y: number }): Promise<void> {
  await row.scrollIntoViewIfNeeded();
  await page.evaluate(() => new Promise((done) => requestAnimationFrame(() => requestAnimationFrame(done))));
  await row.click({ button: "right", position });
}
const tile = (page: Page, name: RegExp) =>
  page.getByRole("group", { name: "Was die Liste zeigt" }).getByRole("button", { name });

test("the list shows every kind with its count, its last runs and one status", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop list");
  await stage(page);
  await openList(page);

  await expect(tile(page, /^Alle/)).toContainText(`${ENTRY_COUNT} Einträge`);
  await expect(tile(page, /^Container/)).toContainText("8 Einträge");
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
  await expect(page.getByRole("heading", { name: "Nicht installiert (nur Sicherungen)" })).toBeVisible();
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

test("every status keeps to one line and the run strips line up, in the longest languages too", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop-1280", "the list with its run strips");
  for (const lang of ["de", "ru"]) {
    await stage(page);
    await page.addInitScript((code) => window.localStorage.setItem("bv-lang", code), lang);
    await page.goto("/backups");
    await expect(rows(page).first()).toBeVisible();
    await settle(page);
    const cells = await rows(page).evaluateAll((all) =>
      all.map((row) => {
        const status = row.lastElementChild!.getBoundingClientRect();
        const strip = row.querySelector("[role=img]")!.getBoundingClientRect();
        return { height: Math.round(status.height), width: Math.round(status.width), strip: Math.round(strip.right) };
      }),
    );
    expect(new Set(cells.map((c) => c.height)).size, `a status wraps in ${lang}`).toBe(1);
    expect(new Set(cells.map((c) => c.width)).size, `the statuses differ in width in ${lang}`).toBe(1);
    expect(new Set(cells.map((c) => c.strip)).size, `the strips do not line up in ${lang}`).toBe(1);
    await page.unrouteAll({ behavior: "ignoreErrors" });
  }
});

test("the mark that picks a row is a disc in every shape and lies on the row it picks", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop-1280", "a mark under the pointer needs a pointer");
  await stage(page);
  for (const shape of ["soft", "square", "round"]) {
    await page.addInitScript((value) => window.localStorage.setItem("bv-shape", value), shape);
    await openList(page);
    const row = rows(page).filter({ hasText: "Dokumente" });
    await row.hover();
    const mark = row.getByRole("checkbox", { name: "Dokumente auswählen" });
    await expect(mark).toBeVisible();
    const disc = async () =>
      mark.evaluate((el) => {
        const shown = el.firstElementChild!;
        const box = shown.getBoundingClientRect();
        return { round: parseFloat(getComputedStyle(shown).borderTopLeftRadius) >= box.width / 2, width: box.width, height: box.height };
      });
    expect(await disc(), `the mark in the ${shape} shape`).toEqual({ round: true, width: 18, height: 18 });

    const [at, around, name] = [(await mark.boundingBox())!, (await row.boundingBox())!, (await row.getByRole("link").boundingBox())!];
    expect(at.x, "the mark hangs out of its row").toBeGreaterThanOrEqual(around.x);
    expect(at.y).toBeGreaterThanOrEqual(around.y);
    expect(at.x + at.width, "the mark is not at the start of the row").toBeLessThan(name.x);

    await mark.click();
    await expect(page.getByTestId("bulk-bar")).toBeVisible();
    await expect(page).toHaveURL(/\/backups$/);
    expect(await disc(), `the mark of a picked row in the ${shape} shape`).toEqual({ round: true, width: 18, height: 18 });
    const picked = (await mark.boundingBox())!;
    const glyph = (await row.locator(".glim-row-glyph").boundingBox())!;
    expect(picked.x + picked.width, "the mark of a picked row stands before its glyph").toBeLessThanOrEqual(glyph.x);
  }
});

test("a row opens the page of its entry", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop list");
  await stage(page);
  await openList(page);
  // On the glyph and not on the name, to show the whole row is the link.
  await rows(page).filter({ hasText: "Dokumente" }).locator(".glim-row-glyph").click();
  await expect(page).toHaveURL(/\/backups\/files\/a1$/);
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
  await tile(page, /^Container/).click();
  await actions.getByRole("button", { name: "Alle jetzt sichern" }).click();
  // What is paused, gone or BombVault itself is left out.
  await expect
    .poll(() => posts.find((p) => p.startsWith("/api/containers/backup-all")))
    .toBe(
      `/api/containers/backup-all ${JSON.stringify({ names: ["immich", "immich_postgres", "jellyfin", LONG_NAME, "paperless-ngx", "uptime-kuma"] })}`,
    );

  // VMs have a batch of their own like every other kind.
  await tile(page, /^VMs/).click();
  await actions.getByRole("button", { name: "Alle jetzt sichern" }).click();
  await expect
    .poll(() => posts.find((p) => p.startsWith("/api/vms/backup-all")))
    .toBe(`/api/vms/backup-all ${JSON.stringify({ names: ["debian", "win11"] })}`);
});

test("a kind the host did not answer for is named above the list", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop list");
  await stage(page, { unlisted: ["container", "vm"] });
  await openList(page);
  await expect(page.getByText("Container und VMs ließen sich auf diesem Server nicht auslesen.", { exact: false })).toBeVisible();
});

test("rows are picked like files and acted on together", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop-1280", "a mouse and a keyboard");
  const { posts } = await stage(page);
  await openList(page);
  const bar = page.getByTestId("bulk-bar");
  const picked = () => page.locator('[data-testid="backup-row"][data-picked]');

  // A plain click still opens the entry; the modifier picks.
  await rows(page).filter({ hasText: "immich" }).first().click({ modifiers: ["ControlOrMeta"] });
  await expect(page).toHaveURL(/\/backups$/);
  await expect(bar).toContainText("1 ausgewählt");
  await rows(page).filter({ hasText: "paperless-ngx" }).click({ modifiers: ["Shift"] });
  // Everything between the two in the order of the list: immich, its
  // database, jellyfin, the long name and paperless.
  await expect(picked()).toHaveCount(5);
  await expect(bar).toContainText("5 ausgewählt");

  await bar.getByRole("button", { name: "Zeitplan pausieren" }).click();
  await expect.poll(() => posts.filter((p) => p.startsWith("PATCH /api/containers/")).length).toBe(5);
  await expect(page.getByRole("status").filter({ hasText: "Zeitplan für 5 Einträge pausiert" })).toBeVisible();

  await page.keyboard.press("ControlOrMeta+a");
  await expect(bar).toContainText(`${ENTRY_COUNT} ausgewählt`);
  await bar.getByRole("button", { name: "Auswahl sichern" }).click();
  // One batch per kind, and what is gone is left out of them.
  await expect.poll(() => posts.some((p) => p.startsWith("/api/containers/backup-all"))).toBe(true);
  expect(posts.find((p) => p.startsWith("/api/containers/backup-all"))).not.toContain("pihole");

  await page.keyboard.press("Escape");
  await expect(bar).toBeHidden();
  await expect(picked()).toHaveCount(0);
});

test("a right click opens a row's actions at the pointer", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop-1280", "a mouse");
  const { posts } = await stage(page);
  await openList(page);

  const jellyfin = rows(page).filter({ hasText: "jellyfin" });
  const at = (await jellyfin.boundingBox())!;
  await rightClick(page, jellyfin, { x: 300, y: at.height / 2 });
  const menu = page.getByRole("menu", { name: "jellyfin" });
  await expect(menu).toBeVisible();
  const box = (await menu.boundingBox())!;
  expect(Math.abs(box.x - (at.x + 300))).toBeLessThanOrEqual(2);
  await expect(menu.getByRole("menuitem")).toHaveText([
    "Jetzt sichern",
    "Zeitplan pausieren",
    "Neueste Sicherung wiederherstellen",
    "Öffnen",
    "Auswählen",
  ]);

  await menu.getByRole("menuitem", { name: "Zeitplan pausieren" }).click();
  await expect(menu).toBeHidden();
  await expect.poll(() => posts).toContain('PATCH /api/containers/jellyfin {"includeInSchedule":false}');

  // On a picked row the menu is about the whole selection.
  await jellyfin.click({ modifiers: ["ControlOrMeta"] });
  await rows(page).filter({ hasText: "Fotos" }).click({ modifiers: ["ControlOrMeta"] });
  await rightClick(page, jellyfin);
  const many = page.getByRole("menu");
  await expect(many).toContainText("2 ausgewählt");
  await many.getByRole("menuitem", { name: "Auswahl aufheben" }).click();
  await expect(page.getByTestId("bulk-bar")).toBeHidden();
});

test("the rare actions wait under More", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop-1280", "the desktop corner");
  const { posts } = await stage(page);
  await openList(page);
  const more = page.getByTestId("page-actions").getByRole("button", { name: "Mehr" });

  await more.click();
  const menu = page.getByRole("menu", { name: "Mehr" });
  await expect(menu.getByRole("menuitem")).toHaveText([
    "Auswählen",
    "Sicherungen entdecken",
    "Stack wiederherstellen",
    "ZFS-Verbindung zu diesem Server",
  ]);
  // It opens above its button, clear of the corner.
  const corner = (await more.boundingBox())!;
  const box = (await menu.boundingBox())!;
  expect(box.y + box.height).toBeLessThanOrEqual(corner.y);

  await menu.getByRole("menuitem", { name: "Sicherungen entdecken" }).click();
  await expect.poll(() => posts).toContain("/api/zfs/discover");

  await more.click();
  await page.getByRole("menuitem", { name: "Auswählen" }).click();
  await expect(page.getByTestId("bulk-bar")).toContainText("Auswählen: anklicken, Strg+Klick oder Umschalt+Klick");
  await page.getByTestId("bulk-bar").getByRole("button", { name: "Fertig" }).click();

  await more.click();
  await page.getByRole("menuitem", { name: "ZFS-Verbindung zu diesem Server" }).click();
  const zfs = page.getByRole("dialog", { name: "Verbindung zu diesem Server" });
  await expect(zfs).toContainText("Verbunden als root@192.168.1.20");
  await zfs.getByRole("button", { name: "Schließen" }).click();
});

test("the database dumps are explained once", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop list");
  await stage(page);
  await openList(page);
  const note = page.getByTestId("dump-intro");
  await expect(note).toContainText("immich_postgres");
  await note.getByRole("button", { name: "Alles klar" }).click();
  await expect(note).toBeHidden();
  await page.reload();
  await expect(rows(page).first()).toBeVisible();
  await expect(note).toBeHidden();
});

test("a stack comes back through its window", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop-1280", "the desktop list");
  const { posts } = await stage(page);
  await openList(page, "/backups?kind=container");

  const stacks = page.getByTestId("stacks");
  await expect(stacks).toContainText("2 Container · immich, immich_postgres");
  await stacks.getByRole("button", { name: "Stack wiederherstellen" }).click();
  const win = page.getByRole("dialog", { name: "Stack wiederherstellen" });
  await expect(win.getByRole("radio", { name: /immich/ })).toHaveAttribute("aria-checked", "true");
  await win.getByRole("button", { name: "Weiter" }).click();
  await expect(win).toContainText("Alle Container dieses Stacks wiederherstellen?");
  await win.getByRole("button", { name: "Stack wiederherstellen" }).click();
  await expect.poll(() => posts.find((p) => p.startsWith("/api/stacks/immich/restore"))).toBeTruthy();
  await expect(win).toContainText("immich_postgres");
  await win.getByRole("button", { name: "Schließen" }).click();
  await expect(win).toBeHidden();
});

test("Add opens one window for a folder or a ZFS dataset", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop-1280", "the desktop window");
  const { posts } = await stage(page);
  await openList(page);

  await page.getByTestId("page-actions").getByRole("button", { name: "Hinzufügen" }).click();
  const win = page.getByRole("dialog", { name: "Was soll BombVault zusätzlich sichern?" });
  await expect(win.getByRole("button", { name: /Einen Ordner/ })).toContainText("3 Freigaben gefunden, 1 davon sicherst du schon");
  await expect(win.getByRole("button", { name: /Ein ZFS-Dataset/ })).toContainText("4 Datasets auf dem Host sind in keinem Eintrag");

  await win.getByRole("button", { name: /Einen Ordner/ }).click();
  const found = win.getByTestId("found-folders");
  await expect(found.getByRole("button", { name: /dokumente/ })).toBeDisabled();
  await found.getByRole("button", { name: /media/ }).click();
  await expect(win.getByPlaceholder("z. B. Dokumente")).toHaveValue("Media");
  await win.getByRole("button", { name: "Anlegen" }).click();
  await expect
    .poll(() => posts.find((p) => p.startsWith("POST /api/files/sets")))
    .toBe(`POST /api/files/sets ${JSON.stringify({ name: "Media", path: "user/media", excludes: [], enabled: true })}`);
  await expect(win).toBeHidden();

  await tile(page, /^ZFS/).click();
  await page.getByTestId("page-actions").getByRole("button", { name: "Dataset hinzufügen" }).click();
  const zfs = page.getByRole("dialog", { name: "ZFS-Datasets hinzufügen" });
  await expect(zfs.getByRole("tree")).toContainText("tank/media");
  await zfs.getByRole("button", { name: "Andere Auswahl" }).click();
  await expect(page.getByRole("dialog", { name: "Was soll BombVault zusätzlich sichern?" })).toBeVisible();
});

test("a ZFS connection that is down stands above the ZFS entries", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop list");
  await stage(page, { zfsCode: "ssh-auth" });
  await openList(page, "/backups?kind=zfs");
  const note = page.getByTestId("zfs-down");
  await expect(note).toBeVisible();
  await note.getByRole("button", { name: "Verbindung ansehen" }).click();
  await expect(page.getByRole("dialog", { name: "Verbindung zu diesem Server" })).toBeVisible();
});

test("the search finds an image, an address and a former name, and the list sorts by address", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop-1280", "the desktop list");
  await stage(page);
  await openList(page);

  await page.getByRole("button", { name: "Suchen" }).click();
  const search = page.getByRole("searchbox", { name: "Suchen" });
  await search.fill("linuxserver/jelly");
  await expect(rows(page)).toHaveText([/jellyfin/]);
  await search.fill("192.168.20.4");
  await expect(rows(page)).toHaveCount(2);
  await search.fill("bitwarden");
  await expect(rows(page)).toHaveText([/uptime-kuma/]);
  await search.press("Escape");

  await page.getByRole("button", { name: "Name A bis Z" }).click();
  await page.getByRole("option", { name: "IP" }).click();
  await expect(rows(page).first()).toContainText("192.168.20.35");
  await expect(rows(page).nth(1)).toContainText("192.168.20.40");
});

test("a long press starts picking on a touch screen", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "a finger");
  await stage(page);
  await openList(page);
  const first = rows(page).first();
  const at = (await first.boundingBox())!;
  const point = { clientX: at.x + 60, clientY: at.y + at.height / 2, pointerType: "touch", isPrimary: true, bubbles: true };
  await first.dispatchEvent("pointerdown", point);
  await page.waitForTimeout(650);
  await first.dispatchEvent("pointerup", point);

  const bar = page.getByTestId("bulk-bar");
  await expect(bar).toContainText("1 ausgewählt");
  // From here a tap picks, and the page stays.
  await rows(page).nth(1).click();
  await expect(bar).toContainText("2 ausgewählt");
  await expect(page).toHaveURL(/\/backups$/);
  const past = await bar.evaluate((el) => el.getBoundingClientRect().right > window.innerWidth + 1);
  expect(past, "the bar runs past the edge").toBe(false);
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
    const shot = (view: string) => page.screenshot({ path: join(SHOTS!, `${view}-${name}.png`) });
    await shot("list");
    if (width > 500) {
      await rightClick(page, rows(page).filter({ hasText: "jellyfin" }), { x: 300, y: 20 });
      await shot("context-one");
      await page.keyboard.press("Escape");
      await rows(page).filter({ hasText: "jellyfin" }).click({ modifiers: ["ControlOrMeta"] });
      await rows(page).filter({ hasText: "immich" }).first().click({ modifiers: ["ControlOrMeta"] });
    } else {
      await page.getByTestId("page-actions").getByRole("button", { name: "Mehr" }).click();
      await page.getByRole("menuitem", { name: "Auswählen" }).click();
      await rows(page).filter({ hasText: "jellyfin" }).click();
      await rows(page).filter({ hasText: "immich" }).first().click();
    }
    await shot("picked");
    await page.getByTestId("bulk-bar").getByRole("button", { name: "Fertig" }).click();
    await page.getByTestId("page-actions").getByRole("button", { name: "Mehr" }).click();
    await shot("more");
    await page.getByRole("menuitem", { name: "Stack wiederherstellen" }).click();
    await settle(page);
    await shot("stack-pick");
    await page.getByRole("dialog").getByRole("button", { name: "Weiter" }).click();
    await shot("stack-check");
    await page.getByRole("dialog").getByRole("button", { name: "Zurück" }).click();
    await page.getByRole("dialog").getByRole("button", { name: "Abbrechen" }).click();
    await page.getByTestId("page-actions").getByRole("button", { name: "Hinzufügen" }).click();
    await settle(page);
    await shot("add-pick");
    await page.getByRole("dialog").getByRole("button", { name: /Einen Ordner/ }).click();
    await page.getByTestId("found-folders").getByRole("button", { name: /media/ }).click();
    await shot("add-folder-picked");
    await page.getByRole("dialog").getByRole("button", { name: "Andere Auswahl" }).click();
    await page.getByRole("dialog").getByRole("button", { name: /Ein ZFS-Dataset/ }).click();
    await expect(page.getByRole("tree")).toBeVisible();
    await settle(page);
    await shot("add-zfs");
    await page.getByRole("dialog").getByRole("button", { name: "Abbrechen" }).click();
    await tile(page, /^Container/).click();
    await settle(page);
    await shot("container");
    await page.locator("#bv-main").evaluate((main) => main.scrollTo(0, main.scrollHeight));
    await shot("container-end");
  }
});
