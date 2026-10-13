// The Restore page. A fresh harness database has nothing to restore, so the
// entries, their timelines, one open finding and the answer of the restore
// check are staged at the route layer. On the desktop: the tiles, the search,
// the backups of the picked entry on their rail, the restore window and the
// windows of the rare ways, among them the emergency that takes the key of
// another install over from its recovery kit. On the phones: the same content
// with nothing past the edge. German, because its labels run longest.
import { expect, test, type Page } from "@playwright/test";
import { mkdirSync } from "node:fs";
import { join } from "node:path";

const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

// Set to a folder to keep pictures of the page at the widths and themes the
// design is compared at.
const SHOTS = process.env.RESTORE_SHOTS;

// A handler can still wait on the real server when its test has passed;
// what it throws then is dropped.
test.afterEach(async ({ page }) => {
  await page.unrouteAll({ behavior: "ignoreErrors" });
});

const NOW = Math.floor(Date.now() / 1000);
const HOUR = 3600;
const DAY = 86400;
const MB = 1024 ** 2;

const SET_ID = "a1b2c3d4e5f60718293a4b5c6d7e8f90";
const EVERYTHING = { kind: "everything", spec: "daily@03:00" };

const PLACEMENT = {
  segment: "local-offsite",
  repo: "",
  repoLabel: "",
  repoKind: "domain",
  repoOff: false,
  homeFollows: true,
  copiesFollow: true,
  skip: [],
  locked: false,
  lockReason: "",
  segmentLocks: {},
  paused: false,
  unreadable: false,
};

function item(over: Record<string, unknown>) {
  const key = String(over.key);
  return {
    kind: "container",
    name: key,
    included: true,
    paused: false,
    effectiveSchedule: EVERYTHING,
    lastBackup: NOW - 4 * HOUR,
    lastRunStatus: "success",
    sourceBytes: 65 * MB,
    state: "running",
    installed: true,
    runs: [],
    ...over,
  };
}

const LONG_NAME = "nextcloud-aio-datenbank-hauptserver-produktivsystem";

const ITEMS = [
  item({ key: "homeassistant" }),
  item({ key: "immich" }),
  item({ key: LONG_NAME }),
  item({ key: "gitea", lastBackup: 0, lastRunStatus: "", sourceBytes: null, included: false }),
  item({ kind: "vm", key: "win11", name: "Windows 11" }),
  item({ kind: "files", key: SET_ID, name: "Fotos", state: undefined, installed: undefined }),
  item({ kind: "flash", key: "flash", name: "flash", state: undefined, installed: undefined }),
  item({ kind: "config", key: "config", name: "config", state: undefined, installed: undefined }),
];

const CONTAINERS = ["homeassistant", "immich", LONG_NAME, "gitea"].map((name) => ({
  name,
  image: `ghcr.io/example/${name}:latest`,
  state: "running",
  status: "Up 3 days",
  ip: "192.168.20.40",
  installed: true,
  includeInSchedule: true,
  lastBackup: NOW - 4 * HOUR,
  lastBackupStarted: NOW - 4 * HOUR - 44,
  preHook: "",
  postHook: "",
  stopContainers: [],
  excludes: [],
  lastUpdateCheck: 0,
  lastUpdateResult: "",
  stack: "",
  placement: PLACEMENT,
  dbEngine: "",
  dbSuggestedEngine: "",
  dbTier: "",
  dbDumpOff: false,
  dbDumpEngine: "",
  dbDumpLabelOff: false,
  dbDumpsGlobalOff: false,
  dbDataCoverage: "",
  dbDumpHookOverlap: false,
  dumpOnly: false,
}));

// Backups today, yesterday, and three, four and thirty-five days ago.
const DAYS = [0, 1, 3, 4, 35];
const snapshotOf = (key: string, i: number) => `${key.slice(0, 6)}${i}aa11bb22cc33dd44`.slice(0, 16);

const LOCAL = { place: "local", label: "", kind: "home", remote: false, enabled: true, appendOnly: false, state: "read" };
const BOX = { place: "offsite:t1", label: "Storage Box", kind: "target", remote: true, enabled: true, appendOnly: false, state: "read" };

function timeline(key: string) {
  return {
    ok: true,
    places: [LOCAL, BOX],
    rows: DAYS.map((daysAgo, i) => ({
      key: snapshotOf(key, i),
      time: new Date((NOW - 4 * HOUR - daysAgo * DAY) * 1000).toISOString(),
      size: (65 + i) * MB,
      places: [
        { place: "local", snapshotIds: [snapshotOf(key, i)], tags: i === 3 ? ["vor dem Update"] : [] },
        ...(i < 4 ? [{ place: BOX.place, snapshotIds: [`${snapshotOf(key, i)}c`], tags: [] }] : []),
      ],
    })),
  };
}

// The newest backup of immich lost most of its files, so the one before it
// is the one to restore from.
const FINDING = {
  id: "f1",
  detector: "source",
  metric: "source_bytes_shrink",
  severity: "critical",
  state: "open",
  scopeKind: "item",
  scopeId: "tg-immich",
  targetId: "tg-immich",
  domain: "container",
  part: "",
  targetName: "",
  name: "immich",
  runId: "r0",
  lastRunId: "r0",
  lastRunAt: NOW - 4 * HOUR,
  lastGood: { runId: "r1", snapshotId: snapshotOf("immich", 1), at: NOW - 4 * HOUR - DAY },
  flaggedSnapshots: [snapshotOf("immich", 0)],
  observed: 6 * MB,
  expected: 65 * MB,
  threshold: 30 * MB,
  samples: 12,
  sensitivity: "balanced",
  details: { refBytes: 65 * MB },
  occurrences: 1,
  firstSeenAt: NOW - 4 * HOUR,
  lastSeenAt: NOW - 4 * HOUR,
  recoveredAt: 0,
  resolvedAt: 0,
  ackedAt: 0,
  ackNote: "",
  expectable: false,
  retentionHeld: false,
  stillPresent: true,
};

const checks: Record<string, unknown>[] = [];

async function stage(page: Page, theme: "dark" | "light" = "dark"): Promise<void> {
  checks.length = 0;
  const json = (path: string, body: unknown) =>
    page.route(
      (url) => url.pathname === path,
      (route) => (route.request().method() === "GET" ? route.fulfill({ json: body }) : route.continue()),
    );

  await json("/api/items", { ok: true, items: ITEMS, unlisted: [] });
  await page.route(
    (url) => /^\/api\/items\/[^/]+\/[^/]+$/.test(url.pathname),
    (route) => {
      if (route.request().method() !== "GET") return route.continue();
      const [kind, key] = new URL(route.request().url()).pathname.split("/").slice(3).map(decodeURIComponent);
      const found = ITEMS.find((i: Record<string, unknown>) => i.kind === kind && i.key === key);
      if (found) return route.fulfill({ json: { ok: true, item: found } });
      return route.fulfill({ status: 404, json: { ok: false, error: "no such item", code: "not-found" } });
    },
  );
  await json("/api/containers", { ok: true, containers: CONTAINERS });
  await json("/api/vms", {
    ok: true,
    vms: [{ name: "Windows 11", libvirtName: "win11", state: "running", method: "graceful", includeInSchedule: true, lastBackup: NOW, lastBackupStarted: NOW, placement: PLACEMENT }],
  });
  await json("/api/files", {
    ok: true,
    fileSets: [{ id: SET_ID, name: "Fotos", path: "fotos", excludes: [], enabled: true, lastBackup: NOW, effectiveSchedule: EVERYTHING, pathExists: true, placement: PLACEMENT }],
  });
  await json("/api/anomalies/items", { ok: true, items: [] });
  await json("/api/anomalies", { ok: true, anomalies: [FINDING], nextCursor: "" });
  await json("/api/anomalies/summary", {
    ok: true,
    summary: {
      enabled: true,
      ready: true,
      generation: 3,
      open: { critical: 1, warning: 0, info: 0 },
      recoveredCritical: 0,
      learningItems: 0,
      retentionHeld: 0,
      evalErrors: 0,
      notifyMuted: false,
      backfill: { slots: 0, done: 0, failed: 0, filled: 0, withoutSummary: 0 },
      unmeasuredVolumes: [],
    },
  });
  await page.route(
    (url) => url.pathname === "/api/runs",
    (route) => route.fulfill({ json: { ok: true, runs: [] } }),
  );
  await page.route(
    (url) => /^\/api\/items\/[^/]+\/[^/]+\/timeline$/.test(url.pathname),
    (route) => {
      const key = decodeURIComponent(new URL(route.request().url()).pathname.split("/")[4]);
      return route.fulfill({ json: timeline(key) });
    },
  );
  await page.route("**/api/restore/check", (route) => {
    const req = route.request().postDataJSON() as Record<string, unknown>;
    checks.push(req);
    const beside = req.target === "beside";
    return route.fulfill({
      json: {
        ok: true,
        ready: true,
        checks: [
          { id: "repository", status: "ok", detail: "/mnt/user/backups/bombvault/containers" },
          { id: "key", status: "ok" },
          { id: "snapshot", status: "ok", detail: String(req.snapshotId).slice(0, 8) },
          { id: "space", status: "ok", need: 65 * MB, free: 1200 * 1024 * MB },
        ],
        plan: {
          inPlace: !beside,
          added: beside ? 1218 : 2,
          changed: beside ? 0 : 12,
          unchanged: beside ? 0 : 1204,
          extra: beside ? 0 : 3,
          files: [],
          listCapped: false,
          partial: false,
          missing: false,
          definition: [],
          shared: [],
          ...(beside
            ? {
                beside: {
                  name: `${String(req.name)}-restored`,
                  folders: [{ original: `/mnt/user/appdata/${String(req.name)}`, copy: `/mnt/user/appdata/${String(req.name)}-restored` }],
                  container: { ports: [{ container: "2283/tcp", original: "2283", copy: "2284" }], shared: [] },
                },
              }
            : { backupFirst: { status: "ok", seconds: 41, bytes: 2 * MB } }),
        },
      },
    });
  });
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
          configEnabled: true,
          anomalyEnabled: true,
        },
      },
    });
  });
  // The stored look is the one under test, so the server's must not replace it.
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.addInitScript((mode) => {
    window.localStorage.setItem("bv-lang", "de");
    window.localStorage.setItem("bv-theme", mode);
  }, theme);
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

async function openPage(page: Page, path = "/restore"): Promise<void> {
  await page.goto(path);
  await expect(page.getByRole("heading", { level: 1, name: "Wiederherstellung" })).toBeAttached();
  await expect(page.getByTestId("rail-row").first()).toBeVisible();
  await settle(page);
}

const entries = (page: Page) => page.getByTestId("restore-entries");
const rows = (page: Page) => page.getByTestId("rail-row");
const rail = (page: Page) => page.getByRole("listbox", { name: "Sicherung" });
const wizard = (page: Page) => page.getByRole("dialog", { name: "Notfall-Assistent" });

// A recovery kit sealed with age, as far as the staged server looks at it.
const SEALED_KIT = "-----BEGIN AGE ENCRYPTED FILE-----\nYWdlLWVuY3J5cHRpb24ub3JnL3YxCg==\n-----END AGE ENCRYPTED FILE-----\n";
const IDENTITY = `AGE-SECRET-KEY-1${"Q".repeat(58)}`;

const KIT_PLACES = [
  { domain: "containers", kind: "local", location: "/mnt/user/backups/bombvault/containers", folder: "backups/bombvault/containers", state: "readable" },
  { domain: "config", kind: "local", location: "/mnt/user/backups/bombvault/config", folder: "backups/bombvault/config", state: "readable" },
  { domain: "containers", kind: "offsite", location: "rclone:storagebox:bombvault/containers", state: "sign-in" },
  { domain: "files", kind: "local", location: "/mnt/disks/usb/bombvault/files", state: "missing" },
];

interface StagedEmergency {
  backupKey: "own" | "adopted" | "locked";
  source?: string;
  adoptedAt?: number;
  undoable: boolean;
  session?: Record<string, unknown>;
}

/** The instance of the group that keeps the deposit, and what its person has
 *  answered the request with. */
const keeper = { allows: false };

const SESSION = "sess-e2e";

const KIT_SESSION = {
  id: SESSION,
  kit: { server: "tower", writtenAt: NOW - 2 * DAY },
  encrypted: true,
  places: KIT_PLACES,
  expiresAt: NOW + 1800,
  cloudSignIn: true,
  rcloneConfig: true,
  replaces: { cloudSignIn: false, rcloneConfig: false },
};

/**
 * Stages the emergency of another install: the key the server backs up under
 * and the session that tried a shown key. Returns what the page asked of it.
 */
async function stageEmergency(page: Page, start: StagedEmergency = { backupKey: "own", undoable: false }) {
  let state = start;
  const asked: string[] = [];
  keeper.allows = false;
  await page.route(
    (url) => url.pathname.startsWith("/api/emergency"),
    (route) => {
      const request = route.request();
      const step = `${request.method()} ${new URL(request.url()).pathname.replace("/api/emergency", "") || "/"}`;
      if (request.method() !== "GET") asked.push(step);
      const refuse = (code: string) => route.fulfill({ json: { ok: false, code, error: code } });
      // Every step after the read names the emergency it acts on.
      const named = request.method() === "DELETE" ? new URL(request.url()).searchParams.get("session") : request.postDataJSON()?.session;
      if (["POST /adopt", "POST /sign-ins", "POST /finish", "DELETE /"].includes(step) && named !== SESSION) return refuse("emergency-ended");
      switch (step) {
        case "POST /read": {
          const body = request.postDataJSON() as { kit?: string; identity?: string };
          if (body.kit === SEALED_KIT && !body.identity) return refuse("kit-sealed");
          if (body.kit === SEALED_KIT && body.identity !== IDENTITY) return refuse("kit-not-for-identity");
          state = { ...state, session: { source: "kit", ...KIT_SESSION } };
          break;
        }
        case "GET /deposits":
          return route.fulfill({
            json: {
              ok: true,
              deposits: [{ memberId: "m-nas", memberName: "nas", ownerId: "o-tower", ownerName: "tower", updatedAt: new Date((NOW - DAY) * 1000).toISOString(), newest: true }],
              silent: ["dachboden"],
            },
          });
        case "POST /group": {
          const body = request.postDataJSON() as { code: string; ask: boolean };
          asked[asked.length - 1] = `group ${body.ask ? "ask" : "look"} ${body.code}`;
          if (!keeper.allows) return route.fulfill({ json: { ok: true, release: "asked" } });
          state = { ...state, session: { source: "group", ...KIT_SESSION } };
          return route.fulfill({ json: { ok: true, release: "allowed", emergency: state } });
        }
        case "POST /adopt":
          state = { backupKey: "adopted", source: "kit", adoptedAt: NOW, undoable: true, session: { ...state.session, refusal: "adopt-same-key" } };
          break;
        case "POST /sign-ins": {
          // The places are tried again with the kit's sign-ins.
          const tried = KIT_PLACES.map((place) => (place.state === "sign-in" ? { ...place, state: "readable" } : place));
          state = { ...state, session: { ...state.session, places: tried } };
          break;
        }
        case "POST /finish":
        case "DELETE /":
          state = { ...state, session: undefined };
          break;
        case "POST /undo":
          state = { backupKey: "own", undoable: false };
          break;
      }
      return route.fulfill({ json: { ok: true, emergency: state } });
    },
  );
  return asked;
}

/** Puts this install into a group, as far as the page asks about it. */
async function stageGroup(page: Page): Promise<void> {
  await page.route(
    (url) => url.pathname === "/api/group",
    (route) =>
      route.fulfill({
        json: {
          ok: true,
          active: true,
          instanceId: "new",
          name: "neu",
          passwordSet: true,
          members: [{ id: "m-nas", name: "nas", version: "10.0.0", direct: true, relay: false, address: "" }],
          relay: { mode: "off", url: "", projectUrl: "", connected: false, serve: false, serveClients: 0 },
          joinedAgo: 120,
          memberSeen: true,
          selfAddress: "",
          selfAddressManual: false,
        },
      }),
  );
}

const RECOVERY_CODE = "ABCDE-FGHJK-MNPQR-STVWX";

/** Picks the deposit the group keeps and types the recovery code for it. */
async function pickDeposit(page: Page): Promise<void> {
  await wizard(page).getByRole("radio", { name: /^Über die Gruppe/ }).click();
  await page.getByTestId("emergency-deposit").getByRole("button", { name: "Diese nehmen" }).click();
  await wizard(page).getByLabel("Wiederherstellungscode").fill("abcde fghjk mnpqr stvwx");
  await expect(wizard(page).getByLabel("Wiederherstellungscode")).toHaveValue(RECOVERY_CODE);
}

/** Shows the wizard a sealed kit and opens it with its identity. */
async function showKit(page: Page): Promise<void> {
  await wizard(page).getByRole("radio", { name: /^Recovery-Kit hochladen/ }).click();
  await page.getByTestId("kit-file").setInputFiles({ name: "bombvault-recovery-kit.md.age", mimeType: "text/plain", buffer: Buffer.from(SEALED_KIT) });
  const identity = wizard(page).getByLabel("age-Identität");
  await expect(identity).toHaveAttribute("type", "password");
  await identity.fill(IDENTITY);
  await wizard(page).getByRole("button", { name: "Kit öffnen" }).click();
  await expect(page.getByTestId("emergency-place")).toHaveCount(KIT_PLACES.length);
}

async function edges(page: Page) {
  return page.evaluate(() => {
    const main = document.querySelector("#bv-main");
    const vw = window.innerWidth;
    // What stands in a box that scrolls sideways is in reach past the edge.
    const scrolls = (el: Element | null): boolean =>
      el !== null && (/auto|scroll/.test(getComputedStyle(el).overflowX) || scrolls(el.parentElement));
    const clipped = [...document.querySelectorAll("#bv-main button, #bv-main input, [role='dialog'] button, [role='dialog'] input, [role='switch']")]
      .filter((el) => {
        const r = el.getBoundingClientRect();
        return r.width > 0 && (r.right > vw + 1 || r.left < -1) && !scrolls(el.parentElement);
      })
      .map((el) => (el.getAttribute("aria-label") ?? el.textContent ?? "").trim().slice(0, 40));
    return {
      docPan: document.documentElement.scrollWidth - vw,
      mainPan: main ? main.scrollWidth - main.clientWidth : null,
      clipped,
    };
  });
}

test("the page opens on the entry that needs a restore, with its backups on the rail", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop layout");
  await stage(page);
  await openPage(page);

  await expect(page.getByRole("button", { name: /^Container 3 mit Sicherungen/ })).toBeVisible();
  await expect(entries(page).getByRole("button")).toHaveCount(8);
  await expect(entries(page).getByRole("button", { name: /^gitea/ })).toBeDisabled();
  await expect(entries(page).getByRole("button", { name: /^immich/ })).toHaveAttribute("aria-pressed", "true");

  await expect(rows(page)).toHaveCount(5);
  await expect(rows(page).nth(1)).toHaveAttribute("aria-selected", "true");
  await expect(rows(page).nth(1)).toContainText("Empfohlen");
  await expect(rows(page).nth(0)).toContainText("Nach der Anomalie");
  await expect(rows(page).nth(1)).toContainText("Storage Box");
  await expect(rows(page).nth(1)).toContainText("66,0 MB");
  await expect(rows(page).nth(3)).toContainText("vor dem Update");
});

test("the tiles and the search narrow the entries", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop layout");
  await stage(page);
  await openPage(page);

  await page.getByRole("button", { name: /^VMs/ }).click();
  await expect(entries(page).getByRole("button")).toHaveCount(1);
  await page.getByRole("button", { name: /^VMs/ }).click();
  await page.getByRole("searchbox", { name: "Was willst du zurückholen?" }).fill("foto");
  await expect(entries(page).getByRole("button")).toHaveCount(1);
  await entries(page).getByRole("button", { name: /^Fotos/ }).click();
  await expect(page.getByRole("heading", { level: 2, name: "Fotos" })).toBeAttached();
  await page.getByRole("searchbox").fill("gibt es nicht");
  await expect(page.getByText("Kein Eintrag passt.")).toBeVisible();
});

test("a backup is picked with the keys and by dragging the handle", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop layout");
  await stage(page);
  await openPage(page);

  await rail(page).focus();
  await page.keyboard.press("ArrowDown");
  await expect(rows(page).nth(2)).toHaveAttribute("aria-selected", "true");
  await page.keyboard.press("Home");
  await expect(rows(page).nth(0)).toHaveAttribute("aria-selected", "true");

  const handle = page.getByTestId("rail-handle");
  await handle.scrollIntoViewIfNeeded();
  const from = (await handle.boundingBox())!;
  const to = (await rows(page).nth(3).boundingBox())!;
  await page.mouse.move(from.x + from.width / 2, from.y + from.height / 2);
  await page.mouse.down();
  await page.mouse.move(from.x + from.width / 2, to.y + to.height / 2, { steps: 8 });
  await expect(rows(page).nth(3)).toHaveAttribute("aria-selected", "true");
  await page.mouse.up();
  await expect(rows(page).nth(3)).toHaveAttribute("aria-selected", "true");

  // The handle rests on the dot of the backup it was let go over.
  await settle(page);
  const rested = (await handle.boundingBox())!;
  const row = (await rows(page).nth(3).boundingBox())!;
  expect(Math.abs(rested.y + rested.height / 2 - (row.y + row.height / 2))).toBeLessThan(2);
});

test("the restore window starts on the copy next to the original and asks for the name before it replaces", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop layout");
  await stage(page);
  await openPage(page);

  await rows(page).nth(1).getByRole("button", { name: "Wiederherstellen" }).click();
  const win = page.getByRole("dialog", { name: "immich wiederherstellen" });
  await expect(win.getByRole("radio", { name: /^Neben das Original/ })).toHaveAttribute("aria-checked", "true");
  await expect(win.getByRole("radio", { name: /^Neben das Original/ })).toContainText("immich-restored");
  await expect(win).toContainText("Die Kopie bekommt 2284, das Original behält 2283");
  await expect(win.getByTestId("target-scene-beside")).toBeVisible();
  const beside = win.getByRole("button", { name: "immich neben das Original wiederherstellen" });
  await expect(beside).toBeEnabled();
  expect(checks[checks.length - 1]).toMatchObject({ kind: "container", name: "immich", snapshotId: snapshotOf("immich", 1), target: "beside" });

  await win.getByRole("radio", { name: /^Original ersetzen/ }).click();
  await expect(win.getByRole("switch", { name: "Vorher den heutigen Zustand sichern" })).toHaveAttribute("aria-checked", "true");
  const replace = win.getByRole("button", { name: "immich ersetzen" });
  await expect(replace).toBeEnabled();
  expect(checks[checks.length - 1]).toMatchObject({ backupFirst: true });
  await replace.click();
  const ask = page.getByRole("dialog").last();
  await expect(ask.getByRole("button", { name: "immich ersetzen" })).toBeDisabled();
  await ask.getByRole("textbox").fill("immich");
  await expect(ask.getByRole("button", { name: "immich ersetzen" })).toBeEnabled();
  await ask.getByRole("button", { name: "Abbrechen" }).click();

  await win.getByRole("button", { name: "Abbrechen" }).click();
  await expect(win).toBeHidden();
});

test("several entries are picked like rows of the list and restored one after the other next to their originals", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop layout");
  await stage(page);
  const started: { path: string; body: Record<string, unknown> }[] = [];
  await page.route(
    (url) => /^\/api\/(containers|vms|files\/sets)\/[^/]+\/restore$/.test(url.pathname),
    (route) => {
      started.push({ path: new URL(route.request().url()).pathname, body: route.request().postDataJSON() as Record<string, unknown> });
      return route.fulfill({ json: { ok: true, started: true } });
    },
  );
  // Each restore has finished by the time its run is read.
  const TARGET: Record<string, [string, string]> = {
    containers: ["container", "homeassistant"],
    vms: ["vm", "win11"],
    files: ["files", "Fotos"],
  };
  await page.route(
    (url) => url.pathname === "/api/runs",
    (route) =>
      route.fulfill({
        json: {
          ok: true,
          runs: started.map(({ path }, i) => {
            const [domain, target] = TARGET[path.split("/")[2]];
            return { id: `restore-${i}`, kind: "restore", status: "success", domain, target, startedAt: NOW, error: "" };
          }),
        },
      }),
  );
  await openPage(page);

  const entry = (name: RegExp) => entries(page).getByRole("button", { name });
  await entry(/^homeassistant/).click({ modifiers: ["ControlOrMeta"] });
  await entry(/^Windows 11/).click({ modifiers: ["ControlOrMeta"] });
  await entry(/^Fotos/).click({ modifiers: ["ControlOrMeta"] });
  const bar = page.getByTestId("bulk-bar");
  await expect(bar).toContainText("3 ausgewählt");
  // The configuration comes back through the emergency wizard, and says so.
  await expect(page.getByLabel("Die Konfiguration stellst du für sich allein wieder her, über den Notfall-Assistenten.")).toBeVisible();
  await expect(rows(page)).toHaveCount(0);

  await bar.getByRole("button", { name: "Wiederherstellen" }).click();
  const win = page.getByRole("dialog", { name: "3 Einträge wiederherstellen" });
  await expect(win.getByRole("radio", { name: /^Neben das Original/ })).toHaveAttribute("aria-checked", "true");
  await expect(win.getByTestId("restore-many-entry")).toHaveCount(3);
  await win.getByRole("button", { name: "Wiederherstellen · 3" }).click();
  await expect(win.locator('[data-outcome="done"]')).toHaveCount(3, { timeout: 20_000 });
  expect(started.map(({ path }) => path)).toEqual([
    "/api/containers/homeassistant/restore",
    "/api/vms/win11/restore",
    `/api/files/sets/${SET_ID}/restore`,
  ]);
  for (const { body } of started) expect(body).toMatchObject({ snapshotId: expect.any(String), target: "beside" });
  await expect(win).toContainText("3 wiederhergestellt, 0 fehlgeschlagen.");
  await win.getByRole("button", { name: "Schließen" }).click();
  await expect(win).toBeHidden();
});

test("the old address leads to the page and keeps what the link asked for", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop layout");
  await stage(page);
  await page.goto("/recovery?emergency=settings");
  await expect(page).toHaveURL(/\/restore\?emergency=settings$/);
  await expect(wizard(page).getByRole("radio", { name: /^Einstellungen zurückholen/ })).toBeVisible();
});

test("the emergency and the other sources open their windows", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop layout");
  await stage(page);
  await openPage(page);

  await page.getByTestId("page-actions").getByRole("button", { name: "Notfall-Assistent starten" }).click();
  await expect(wizard(page).getByRole("radio", { name: "Zugang" })).toHaveAttribute("aria-checked", "true");
  await wizard(page).getByRole("button", { name: "Weiter" }).click();
  await expect(wizard(page).getByRole("radio", { name: "Einstellungen", exact: true })).toHaveAttribute("aria-checked", "true");
  await wizard(page).getByRole("button", { name: "Zurück", exact: true }).click();
  await wizard(page).getByRole("button", { name: "Abbrechen" }).click();
  await expect(wizard(page)).toBeHidden();
  await expect(page).toHaveURL(/\/restore$/);

  await page.getByRole("button", { name: /^Aus einem alten Sicherungsordner/ }).click();
  const oldFolder = page.getByRole("dialog", { name: "Aus einem alten Sicherungsordner" });
  await expect(oldFolder.getByRole("button", { name: "Verbinden" })).toBeDisabled();
  await oldFolder.getByRole("button", { name: "Abbrechen" }).click();

  await page.getByRole("button", { name: /^Aus dem Appdata.Backup-Plugin übernehmen/ }).click();
  await expect(page.getByRole("dialog", { name: "Aus dem Appdata.Backup-Plugin übernehmen" })).toBeVisible();
});

test("the emergency stays shut on Back once Cancel or Back closed it", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop layout");
  await stage(page);
  await page.goto("/dashboard");
  await openPage(page);
  const start = page.getByTestId("page-actions").getByRole("button", { name: "Notfall-Assistent starten" });

  await start.click();
  await expect(wizard(page)).toBeVisible();
  await wizard(page).getByRole("button", { name: "Abbrechen" }).click();
  await expect(wizard(page)).toBeHidden();
  await expect(page).toHaveURL(/\/restore$/);
  await page.goBack();
  await expect(page).toHaveURL(/\/dashboard$/);
  await expect(wizard(page)).toHaveCount(0);

  await openPage(page);
  await start.click();
  await expect(wizard(page)).toBeVisible();
  await page.goBack();
  await expect(wizard(page)).toBeHidden();
  await expect(page).toHaveURL(/\/restore$/);
  await page.goBack();
  await expect(page).toHaveURL(/\/dashboard$/);
});

test("the emergency reads a sealed recovery kit, takes its key over when told to and undoes that on the way out", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop layout");
  await stage(page);
  const asked = await stageEmergency(page);
  await page.goto("/restore?emergency=access");

  await expect(wizard(page).getByLabel("age-Identität")).toHaveCount(0);
  await showKit(page);
  await expect(wizard(page).getByLabel("age-Identität")).toHaveCount(0);
  await expect(wizard(page).getByText("Sein Schlüssel öffnet 2 von 4 Speicherorten.")).toBeVisible();
  await expect(wizard(page).getByText(/Noch ist nichts gespeichert\. Der Schlüssel bleibt noch 30 Minuten/)).toBeVisible();
  await expect(page.getByTestId("emergency-place").nth(2)).toContainText("Zugang fehlt");
  await expect(wizard(page).getByTestId("emergency-kit")).toHaveText(/^Recovery-Kit von tower, geschrieben /);
  await expect(wizard(page).getByRole("button", { name: "Weiter" })).toHaveCount(0);
  expect(asked).toEqual(["POST /read", "POST /read"]);

  await wizard(page).getByRole("button", { name: "Schlüssel übernehmen" }).click();
  const question = page.getByRole("dialog").last();
  await expect(question).toContainText("Bis zur ersten Sicherung kannst du das rückgängig machen.");
  await question.getByRole("button", { name: "Schlüssel übernehmen" }).click();
  await expect(wizard(page).getByText(/^Schlüssel übernommen\./)).toBeVisible();
  await wizard(page).getByRole("button", { name: "Zugangsdaten speichern" }).click();
  await expect(wizard(page).getByText("Zugangsdaten gespeichert, die Speicherorte wurden neu geprüft.")).toBeVisible();
  await expect(page.getByTestId("emergency-place").nth(2)).toContainText("lesbar");

  await wizard(page).getByRole("button", { name: "Weiter" }).click();
  await expect(wizard(page).getByText(/Die Anmeldung dieses Servers bleibt/)).toBeVisible();
  await expect(wizard(page).locator("input").first()).toHaveValue("backups/bombvault/config");

  await wizard(page).getByRole("button", { name: "Zurück", exact: true }).click();
  await wizard(page).getByRole("button", { name: "Abbrechen" }).click();
  await expect(wizard(page).getByText(/Dieser Server hat den Schlüssel übernommen\./)).toBeVisible();
  await wizard(page).getByRole("button", { name: "Übernahme rückgängig machen" }).click();
  await expect(wizard(page)).toBeHidden();
  expect(asked).toEqual(["POST /read", "POST /read", "POST /adopt", "POST /sign-ins", "POST /undo"]);
  await expect(page.getByTestId("key-adopted")).toHaveCount(0);
});

test("the emergency through the group waits for the instance that keeps the key and opens its deposit", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop layout");
  await stage(page);
  await stageGroup(page);
  const asked = await stageEmergency(page);
  await page.goto("/restore?emergency=access");

  await expect(wizard(page).getByRole("radio", { name: /^Über die Gruppe/ })).toContainText("die zwölf Wörter der Gruppe, deinen Wiederherstellungscode und jemanden an dieser Instanz");
  await wizard(page).getByRole("radio", { name: /^Über die Gruppe/ }).click();
  await expect(page.getByTestId("emergency-deposit")).toContainText("tower");
  await expect(page.getByTestId("emergency-deposit")).toContainText("verwahrt von nas");
  await expect(wizard(page).getByText(/^Keine Antwort von dachboden\./)).toBeVisible();

  await page.getByTestId("emergency-deposit").getByRole("button", { name: "Diese nehmen" }).click();
  await expect(wizard(page).getByRole("button", { name: "Hinterlegung anfordern" })).toBeDisabled();
  await wizard(page).getByLabel("Wiederherstellungscode").fill("abcde fghjk mnpqr stvwx");
  await expect(wizard(page).getByLabel("Wiederherstellungscode")).toHaveValue(RECOVERY_CODE);
  await wizard(page).getByRole("button", { name: "Hinterlegung anfordern" }).click();
  await expect(wizard(page).getByRole("status")).toContainText("Wartet auf nas.");
  expect(asked).toEqual([`group ask ${RECOVERY_CODE}`]);

  // The request stays with the keeper while the window is gone. The code does not.
  await page.goto("/restore?emergency=access");
  await expect(wizard(page).getByRole("radio", { name: /^Über die Gruppe/ })).toContainText("Bei nas wartet eine Anfrage.");
  await expect(wizard(page).getByText(/^Deine Anfrage wartet bei nas\./)).toBeVisible();
  await expect(wizard(page).getByLabel("Wiederherstellungscode")).toHaveValue("");
  await wizard(page).getByLabel("Wiederherstellungscode").fill(RECOVERY_CODE);
  await wizard(page).getByRole("button", { name: "Antwort abholen" }).click();
  await expect(wizard(page).getByRole("status")).toContainText("Wartet auf nas.");
  expect(asked[asked.length - 1]).toBe(`group look ${RECOVERY_CODE}`);

  keeper.allows = true;
  await expect(page.getByTestId("emergency-place")).toHaveCount(KIT_PLACES.length, { timeout: 15_000 });
  await expect(wizard(page).getByText("Hinterlegung geöffnet. Ihr Schlüssel öffnet 2 von 4 Speicherorten.")).toBeVisible();
  await expect(wizard(page).getByRole("button", { name: "Schlüssel übernehmen" })).toBeEnabled();
  expect(asked.every((step) => !step.startsWith("group ask") || step === asked[0])).toBe(true);
});

test("an install in no group is led to the pairing and back to the group way", async ({ page }, testInfo) => {
  // At 768px the headings of the three pairing steps wrap letter by letter
  // and reach over the card above them, which is where the way back stands.
  test.skip(testInfo.project.name !== "desktop-1280", "the pairing page at its full width");
  await stage(page);
  await stageEmergency(page);
  await page.goto("/restore?emergency=access");

  await wizard(page).getByRole("radio", { name: /^Über die Gruppe/ }).click();
  await expect(wizard(page).getByText(/^Diese Installation ist noch in keiner Gruppe\./)).toBeVisible();
  await wizard(page).getByRole("button", { name: "Zur Kopplung" }).click();
  await expect(page).toHaveURL(/\/settings\/pairing\?for=emergency$/);
  await expect(page.getByText(/^Du kommst vom Notfall-Assistenten\./)).toBeVisible();

  await page.getByRole("button", { name: "Zurück zum Notfall-Assistenten" }).click();
  await expect(page).toHaveURL(/\/restore\?emergency=access&way=group$/);
  await expect(wizard(page).getByRole("radio", { name: /^Über die Gruppe/ })).toHaveAttribute("aria-checked", "true");
});

test("a key that was taken over is named on the page with the day it came", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop layout");
  await stage(page);
  await stageEmergency(page, { backupKey: "adopted", source: "kit", adoptedAt: Date.UTC(2026, 8, 14, 12) / 1000, undoable: false });
  await openPage(page);
  const line = page.getByTestId("key-adopted");
  await expect(line).toContainText("aus einem Recovery-Kit übernommen");
  await expect(line).toContainText("14.09.2026");
  await expect(line).toContainText("Sein eigenes Recovery-Kit nennt diesen Schlüssel.");
  await expect(line.getByRole("button")).toHaveCount(0);
});

test("a key that no longer opens stands first under what needs the person and leads to the emergency", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "the desktop layout");
  await stage(page);
  await stageEmergency(page, { backupKey: "locked", source: "kit", adoptedAt: NOW, undoable: true });
  await page.goto("/dashboard");
  const need = page.getByTestId("need-key-locked");
  await expect(need).toContainText("Der Schlüssel lässt sich nicht mehr öffnen");
  await need.getByRole("button", { name: "Notfall-Assistent starten" }).click();
  await expect(page).toHaveURL(/\/restore\?emergency=access$/);
  await expect(wizard(page).getByText(/Der übernommene Schlüssel lässt sich nicht mehr öffnen/)).toBeVisible();
});

for (const width of [320, 360]) {
  test(`the emergency with a kit @ ${width}px keeps inside the edge`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "the phone layout");
    await stage(page);
    await stageEmergency(page);
    await page.setViewportSize({ width, height: 800 });
    await page.goto("/restore?emergency=access");

    await showKit(page);
    await settle(page);
    const read = await edges(page);
    expect(read.docPan, "the document scrolls horizontally").toBeLessThanOrEqual(1);
    expect(read.clipped, "the places clip the edge").toEqual([]);

    await wizard(page).getByRole("button", { name: "Schlüssel übernehmen" }).click();
    await page.getByRole("dialog").last().getByRole("button", { name: "Schlüssel übernehmen" }).click();
    await expect(wizard(page).getByRole("button", { name: "Zugangsdaten speichern" })).toBeVisible();
    await settle(page);
    expect((await edges(page)).clipped, "the taken-over key clips the edge").toEqual([]);

    await wizard(page).getByRole("button", { name: "Abbrechen" }).click();
    await expect(wizard(page).getByRole("button", { name: "Schlüssel behalten und schließen" })).toBeVisible();
    await settle(page);
    expect((await edges(page)).clipped, "the way out clips the edge").toEqual([]);
  });
}

for (const width of [320, 360]) {
  test(`the emergency through the group @ ${width}px keeps inside the edge`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "the phone layout");
    await stage(page);
    await stageGroup(page);
    await stageEmergency(page);
    await page.setViewportSize({ width, height: 800 });
    await page.goto("/restore?emergency=access");

    await wizard(page).getByRole("radio", { name: /^Über die Gruppe/ }).click();
    await expect(page.getByTestId("emergency-deposit")).toBeVisible();
    await settle(page);
    const listed = await edges(page);
    expect(listed.docPan, "the document scrolls horizontally").toBeLessThanOrEqual(1);
    expect(listed.clipped, "the deposits clip the edge").toEqual([]);

    await page.getByTestId("emergency-deposit").getByRole("button", { name: "Diese nehmen" }).click();
    await wizard(page).getByLabel("Wiederherstellungscode").fill(RECOVERY_CODE);
    await settle(page);
    expect((await edges(page)).clipped, "the recovery code clips the edge").toEqual([]);

    await wizard(page).getByRole("button", { name: "Hinterlegung anfordern" }).click();
    await expect(wizard(page).getByRole("status")).toBeVisible();
    await settle(page);
    expect((await edges(page)).clipped, "the waiting request clips the edge").toEqual([]);
  });
}

for (const width of [320, 360]) {
  test(`restore @ ${width}px: the page and its windows keep inside the edge`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "the phone layout");
    await stage(page);
    await page.setViewportSize({ width, height: 800 });
    await openPage(page);

    const onPage = await edges(page);
    expect(onPage.docPan, "the document scrolls horizontally").toBeLessThanOrEqual(1);
    expect(onPage.mainPan!, "#bv-main scrolls horizontally").toBeLessThanOrEqual(1);
    expect(onPage.clipped, "controls clip the viewport edge").toEqual([]);

    // The tiles stay tiles with their count in words, two to a row.
    const tiles = page.getByRole("group", { name: "Bereiche" }).getByRole("button");
    await expect(tiles.first()).toContainText("mit Sicherungen");
    const [a, b, c] = await Promise.all([0, 1, 2].map(async (i) => (await tiles.nth(i).boundingBox())!));
    expect(Math.abs(a.y - b.y), "the first two tiles share a row").toBeLessThan(1);
    expect(c.y, "the third tile starts the next row").toBeGreaterThan(a.y + a.height - 1);

    // A phone has no hover, so every backup shows its restore button.
    await rows(page).nth(2).getByRole("button", { name: "Wiederherstellen" }).click();
    const win = page.getByRole("dialog", { name: "immich wiederherstellen" });
    await expect(win.getByRole("radio", { name: /^Neben das Original/ })).toBeVisible();
    await settle(page);
    expect((await edges(page)).clipped, "the restore window clips the edge").toEqual([]);
    await win.getByRole("button", { name: "Abbrechen" }).click();

    await page.goto("/restore?emergency=access");
    await wizard(page).getByRole("radio", { name: /^Schlüssel eintippen/ }).click();
    await expect(wizard(page).getByRole("button", { name: "Verbinden und prüfen" })).toBeVisible();
    await settle(page);
    const inWizard = await edges(page);
    expect(inWizard.docPan, "the document scrolls horizontally").toBeLessThanOrEqual(1);
    expect(inWizard.clipped, "the emergency window clips the edge").toEqual([]);
  });
}

test("restore on a phone: the eye of a secret field is big enough to tap", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "the larger targets follow the coarse pointer");
  await stage(page);
  await page.setViewportSize({ width: 360, height: 800 });
  await page.goto("/restore?emergency=access");
  await wizard(page).getByRole("radio", { name: /^Schlüssel eintippen/ }).click();

  const eye = wizard(page).getByRole("button", { name: "Wert anzeigen" }).first();
  await eye.scrollIntoViewIfNeeded();
  await settle(page);
  const eyeBox = (await eye.boundingBox())!;
  const fieldBox = (await eye.locator("xpath=preceding-sibling::input").boundingBox())!;
  expect(eyeBox.width, "the eye is narrower than a fingertip").toBeGreaterThanOrEqual(43.5);
  expect(eyeBox.height, "the eye is shorter than its field").toBeGreaterThanOrEqual(fieldBox.height - 0.5);
});

test("pictures of the page", async ({ page }, testInfo) => {
  test.skip(!SHOTS || testInfo.project.name !== "desktop-1280", "only when a folder for them is named");
  // Three looks of the page and of each of its windows.
  test.setTimeout(120_000);
  mkdirSync(SHOTS!, { recursive: true });
  for (const [name, width, theme] of [
    ["1440-dark", 1440, "dark"],
    ["1440-light", 1440, "light"],
    ["390", 390, "dark"],
  ] as const) {
    await page.unrouteAll({ behavior: "ignoreErrors" });
    await stage(page, theme);
    await page.setViewportSize({ width, height: width < 500 ? 2600 : 1700 });
    await openPage(page);
    await page.screenshot({ path: join(SHOTS!, `page-${name}.png`) });

    await page.setViewportSize({ width, height: width < 500 ? 844 : 1000 });
    await rows(page).nth(1).getByRole("button", { name: "Wiederherstellen" }).click();
    const win = page.getByRole("dialog", { name: "immich wiederherstellen" });
    await expect(win.getByRole("button", { name: "immich neben das Original wiederherstellen" })).toBeEnabled();
    await page.screenshot({ path: join(SHOTS!, `window-${name}.png`) });
    await win.getByRole("radio", { name: /^Original ersetzen/ }).click();
    await expect(win.getByRole("button", { name: "immich ersetzen" })).toBeEnabled();
    await win.getByRole("button", { name: "immich ersetzen" }).scrollIntoViewIfNeeded();
    await page.screenshot({ path: join(SHOTS!, `window-replace-${name}.png`) });
    await win.getByRole("button", { name: "Abbrechen" }).click();

    await page.unrouteAll({ behavior: "ignoreErrors" });
    await stage(page, theme);
    await stageEmergency(page);
    await stageGroup(page);
    await page.goto("/restore?emergency=access");
    await pickDeposit(page);
    await page.screenshot({ path: join(SHOTS!, `emergency-group-${name}.png`) });
    await wizard(page).getByRole("button", { name: "Hinterlegung anfordern" }).click();
    await expect(wizard(page).getByRole("status")).toBeVisible();
    await page.screenshot({ path: join(SHOTS!, `emergency-group-waiting-${name}.png`) });
    await wizard(page).getByRole("button", { name: "Andere Hinterlegung wählen" }).click();
    await expect(page.getByTestId("emergency-deposit")).toBeVisible();
    await page.screenshot({ path: join(SHOTS!, `emergency-group-list-${name}.png`) });
    await wizard(page).getByRole("radio", { name: /^Recovery-Kit hochladen/ }).click();
    await page.screenshot({ path: join(SHOTS!, `emergency-ways-${name}.png`) });
    await showKit(page);
    await page.screenshot({ path: join(SHOTS!, `emergency-read-${name}.png`) });
    await wizard(page).getByRole("button", { name: "Schlüssel übernehmen" }).click();
    await page.getByRole("dialog").last().getByRole("button", { name: "Schlüssel übernehmen" }).click();
    await expect(wizard(page).getByRole("button", { name: "Zugangsdaten speichern" })).toBeVisible();
    await page.screenshot({ path: join(SHOTS!, `emergency-taken-${name}.png`) });
    await page.goto("/restore");
    await expect(page.getByTestId("key-adopted")).toBeVisible();
    await page.getByTestId("key-adopted").scrollIntoViewIfNeeded();
    await page.screenshot({ path: join(SHOTS!, `page-adopted-${name}.png`) });

    for (const step of ["access", "settings", "selection"]) {
      await page.goto(`/restore?emergency=${step}`);
      await expect(wizard(page)).toBeVisible();
      await page.waitForTimeout(600);
      await page.screenshot({ path: join(SHOTS!, `emergency-${step}-${name}.png`) });
    }
    await page.goto("/restore");
    await page.getByRole("button", { name: /^Aus einem alten Sicherungsordner/ }).click();
    await settle(page);
    await page.screenshot({ path: join(SHOTS!, `old-folder-${name}.png`) });
    await page.getByRole("dialog").getByRole("button", { name: "Abbrechen" }).click();
    await page.getByRole("button", { name: /^Aus dem Appdata.Backup-Plugin übernehmen/ }).click();
    await settle(page);
    await page.screenshot({ path: join(SHOTS!, `appdata-${name}.png`) });
  }
});
