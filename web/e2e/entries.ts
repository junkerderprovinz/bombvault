// Entries of every kind for the specs of the Backups list and the entry page.
// A fresh harness database has none and the harness has no Docker, libvirt or
// ZFS to list any, so they are staged at the route layer. The SPA, its
// requests and the binary are real, only the answers are staged.
import type { Page } from "@playwright/test";

export const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

export const NOW = Math.floor(Date.now() / 1000);
const HOUR = 3600;
const DAY = 86400;
const MB = 1024 ** 2;

export const SET_ID = "a1b2c3d4e5f60718293a4b5c6d7e8f90";
export const ZFS_ID = "0f1e2d3c4b5a69788796a5b4c3d2e1f0";

const EVERYTHING = { kind: "everything", spec: "daily@03:00", alsoSpec: "" };

const PLACEMENT = {
  segment: "local-offsite",
  repo: "",
  repoLabel: "",
  repoKind: "domain",
  repoOff: false,
  homeFollows: true,
  copiesFollow: true,
  skip: [],
  locked: true,
  lockReason: "first-backup",
  segmentLocks: {},
  paused: false,
  unreadable: false,
};

type Json = Record<string, unknown>;

export interface StagedRun extends Json {
  id: string;
  snapshotId: string;
  finishedAt: number;
}

/** One successful backup per day given, counted back from four hours ago. */
export function runsOf(prefix: string, domain: string, target: string, daysAgo: number[] = [0, 1, 3, 9]): StagedRun[] {
  return daysAgo.map((days, i) => ({
    id: `${prefix}-r${i}`,
    targetId: `tg-${prefix}`,
    kind: "backup",
    status: "success",
    startedAt: NOW - 4 * HOUR - days * DAY - 44,
    finishedAt: NOW - 4 * HOUR - days * DAY,
    snapshotId: `${prefix}${i}aa11bb22cc33dd44`.slice(0, 16),
    bytes: (i + 1) * MB,
    sourceBytes: 65 * MB,
    error: "",
    acknowledged: false,
    target,
    domain,
  }));
}

export function item(over: Json): Json {
  return {
    kind: "container",
    name: String(over.key),
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

export function container(over: Json = {}): Json {
  return {
    name: "immich",
    image: "ghcr.io/immich-app/immich-server:release",
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
    ...over,
  };
}

export function vm(over: Json = {}): Json {
  return {
    name: "Windows 11",
    libvirtName: "win11",
    state: "running",
    method: "graceful",
    includeInSchedule: true,
    lastBackup: NOW - 4 * HOUR,
    lastBackupStarted: NOW - 5 * HOUR,
    placement: PLACEMENT,
    ...over,
  };
}

export function fileSet(over: Json = {}): Json {
  return {
    id: SET_ID,
    name: "Fotos",
    path: "fotos",
    excludes: [],
    enabled: true,
    lastBackup: NOW - 4 * HOUR,
    effectiveSchedule: EVERYTHING,
    pathExists: true,
    placement: PLACEMENT,
    ...over,
  };
}

export function dataset(over: Json = {}): Json {
  return {
    id: ZFS_ID,
    dataset: "tank/appdata",
    enabled: true,
    excludes: [],
    scheduleCadence: "",
    repo: "",
    repoEffective: "/mnt/user/backups/bombvault/zfs",
    stopContainers: [],
    restartPending: [],
    excludedChildren: [],
    hookContainer: "",
    preSnapshot: "",
    postSnapshot: "",
    hostMountpoint: "/mnt/tank/appdata",
    lastBackup: NOW - 4 * HOUR,
    lastRunStatus: "success",
    lastCheckCode: "ok",
    lastCheckDetail: "",
    lastCheckAt: NOW - 4 * HOUR,
    leftoverCount: 0,
    safetyCount: 0,
    safetyOldestAt: 0,
    members: [
      {
        dataset: "tank/appdata",
        relPath: "",
        hostMountpoint: "/mnt/tank/appdata",
        outcome: "backed-up",
        isNew: false,
        usedByDataset: 1024 ** 3,
        lastBackupAt: NOW - 4 * HOUR,
      },
    ],
    effectiveSchedule: EVERYTHING,
    sites: 1,
    rule321: "one-copy",
    ...over,
  };
}

export const BOX = {
  place: "offsite:t1",
  label: "Storage Box",
  kind: "target",
  remote: true,
  enabled: true,
  appendOnly: false,
  state: "read",
};

const LOCAL = { place: "local", label: "", kind: "home", remote: false, enabled: true, appendOnly: false, state: "read" };

export interface Staged {
  items: Json[];
  containers?: Json[];
  vms?: Json[];
  fileSets?: Json[];
  datasets?: Json[];
  /** The runs of an entry, under "<kind>:<key>". Each successful one is a
   *  backup at the local place and at the storage box. */
  runs?: Record<string, StagedRun[]>;
  lang?: string;
}

/** One entry of every kind, each with a few backups. */
export const EVERY_KIND: Staged = {
  items: [
    item({ key: "immich" }),
    item({ kind: "vm", key: "win11", name: "Windows 11" }),
    item({ kind: "files", key: SET_ID, name: "Fotos", state: undefined, installed: undefined }),
    item({ kind: "zfs", key: ZFS_ID, name: "tank/appdata", state: undefined, installed: undefined }),
    item({ kind: "flash", key: "flash", name: "flash", state: undefined, installed: undefined }),
    item({ kind: "config", key: "config", name: "config", state: undefined, installed: undefined }),
  ],
  containers: [container()],
  vms: [vm()],
  fileSets: [fileSet()],
  datasets: [dataset()],
  runs: {
    "container:immich": runsOf("immich", "container", "immich"),
    "vm:win11": runsOf("win11", "vm", "win11"),
    [`files:${SET_ID}`]: runsOf("fotos", "files", "Fotos"),
    [`zfs:${ZFS_ID}`]: runsOf("tank", "zfs", "tank/appdata"),
    "flash:flash": runsOf("flash", "flash", "Unraid flash"),
    "config:config": runsOf("config", "config", "config"),
  },
};

const TIMELINE_KIND: Record<string, string> = {
  containers: "container",
  vms: "vm",
  files: "files",
  flash: "flash",
  config: "config",
};

const KINDS_ON = {
  containersEnabled: true,
  vmsEnabled: true,
  flashEnabled: true,
  filesEnabled: true,
  zfsEnabled: true,
  configEnabled: true,
};

/** stageEntries answers what the list and the entry page read. */
export async function stageEntries(page: Page, staged: Staged): Promise<void> {
  const runs = staged.runs ?? {};
  const get = (path: string, body: unknown) =>
    page.route(
      (url) => url.pathname === path,
      (route) => (route.request().method() === "GET" ? route.fulfill({ json: body }) : route.continue()),
    );

  await get("/api/items", { ok: true, items: staged.items, unlisted: [] });
  await page.route(
    (url) => /^\/api\/items\/[^/]+\/[^/]+$/.test(url.pathname),
    (route) => {
      if (route.request().method() !== "GET") return route.continue();
      const [kind, key] = new URL(route.request().url()).pathname.split("/").slice(3).map(decodeURIComponent);
      const item = staged.items.find((i) => i.kind === kind && i.key === key);
      if (item) return route.fulfill({ json: { ok: true, item } });
      return route.fulfill({ status: 404, json: { ok: false, error: "no such item", code: "not-found" } });
    },
  );
  await get("/api/containers", { ok: true, containers: staged.containers ?? [] });
  await get("/api/vms", { ok: true, vms: staged.vms ?? [] });
  await get("/api/files", { ok: true, fileSets: staged.fileSets ?? [] });
  await get("/api/zfs", { ok: true, datasets: staged.datasets ?? [] });
  await get("/api/checks/items", { ok: true, items: [] });
  await get("/api/breakdown", {
    ok: true,
    breakdown: { state: "none", path: "", size: 0, files: 0, added: 0, children: [] },
  });

  await page.route(
    (url) => url.pathname === "/api/runs",
    (route) => {
      const q = new URL(route.request().url()).searchParams;
      if (!q.has("itemKind")) return route.continue();
      return route.fulfill({ json: { ok: true, runs: runs[`${q.get("itemKind")}:${q.get("itemKey")}`] ?? [] } });
    },
  );
  await page.route(
    (url) => /^\/api\/items\/[^/]+\/[^/]+\/timeline$/.test(url.pathname),
    (route) => {
      const url = new URL(route.request().url());
      const [, , , domain, key] = url.pathname.split("/");
      if (url.searchParams.has("place")) return route.fulfill({ json: { ok: true, place: BOX, rows: [] } });
      const rows = (runs[`${TIMELINE_KIND[domain]}:${decodeURIComponent(key)}`] ?? []).map((run) => ({
        key: run.snapshotId,
        time: new Date(run.finishedAt * 1000).toISOString(),
        places: [
          { place: "local", snapshotIds: [run.snapshotId], tags: [] },
          { place: BOX.place, snapshotIds: [`${run.snapshotId}-box`], tags: [] },
        ],
      }));
      return route.fulfill({ json: { ok: true, places: [LOCAL, BOX], rows } });
    },
  );
  await page.route(
    (url) => url.pathname === "/api/placement/options",
    (route) => {
      const domain = new URL(route.request().url()).searchParams.get("domain") ?? "";
      return route.fulfill({
        json: {
          ok: true,
          options: {
            domain,
            unreadable: false,
            paused: false,
            homes: [{ id: "", name: "", location: `/mnt/user/backups/${domain}`, kind: "domain", scheme: "" }],
            targets: [{ id: "t1", name: "Storage Box", enabled: true, primary: false, appendOnly: false, hint: "" }],
            sendTo: [],
            segmentLocks: {},
            default: {
              domain,
              exists: true,
              home: "",
              homeKind: "domain",
              homeOff: false,
              skip: [],
              paused: false,
              confirmedAt: NOW,
              counts: {},
              unreadable: false,
            },
          },
        },
      });
    },
  );
  await page.route("**/api/settings", async (route) => {
    if (route.request().method() !== "GET") return route.continue();
    const res = await route.fetch();
    const body = await res.json();
    await route.fulfill({ json: { ...body, settings: { ...body.settings, ...KINDS_ON } } });
  });
  // The stored language is the one under test, so the server's look must not replace it.
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.addInitScript((lang) => window.localStorage.setItem("bv-lang", lang), staged.lang ?? "en");
}

/** settle waits for every animation that ends, so a measure reads the page at rest. */
export async function settle(page: Page): Promise<void> {
  await page.evaluate(() =>
    Promise.all(
      document
        .getAnimations()
        .filter((a) => a.effect?.getTiming().iterations !== Infinity)
        .map((a) => a.finished.catch(() => {})),
    ),
  );
}

/** How far the page pans sideways, and the controls that reach past its edge. */
export async function overflow(page: Page): Promise<{ pan: number; past: string[] }> {
  return page.evaluate(() => {
    const main = document.querySelector("#bv-main")!;
    const vw = window.innerWidth;
    const name = (el: Element) => (el.getAttribute("aria-label") ?? el.textContent ?? "").trim().slice(0, 40);
    return {
      pan: Math.max(document.documentElement.scrollWidth - vw, main.scrollWidth - main.clientWidth),
      past: [...document.querySelectorAll("#bv-main button, #bv-main a, #bv-main input, #bv-main [role='switch']")]
        .filter((el) => {
          const r = el.getBoundingClientRect();
          return r.width > 0 && (r.right > vw + 1 || r.left < -1);
        })
        .map(name),
    };
  });
}
