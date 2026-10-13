// The storage locations a fresh harness database cannot have: one of each kind
// of object, staged at the route layer with the records their pages write
// through, the defaults that say which section is backed up where, and the
// providers a new location is set up with.
import type { Page, Route } from "@playwright/test";
import { PLACEMENT } from "./placement";

const NOW = Math.floor(Date.now() / 1000);
const DAY = 86400;
const GB = 1024 ** 3;
const TB = 1024 * GB;

const KEEP_LONG = { keepLast: 0, keepDaily: 14, keepWeekly: 8, keepMonthly: 12, keepYearly: 3 };
const KEEP_BALANCED = { keepLast: 0, keepDaily: 7, keepWeekly: 4, keepMonthly: 6, keepYearly: 1 };
const KEEP_OWN = { keepLast: 3, keepDaily: 30, keepWeekly: 0, keepMonthly: 24, keepYearly: 0 };

function section(over: Record<string, unknown>) {
  return {
    domain: "containers",
    use: "copy",
    where: "",
    enabled: true,
    immutable: false,
    retention: KEEP_LONG,
    compression: "auto",
    limitUpload: 0,
    limitDownload: 0,
    own: [],
    ...over,
  };
}

export const LOCAL = {
  id: "path:5f2c1d0a9b3e4c7f",
  object: "path",
  kind: "local",
  provider: "",
  backend: "local",
  name: "bombvault",
  where: "/mnt/user/backups/bombvault",
  enabled: true,
  offPremises: false,
  sections: [
    section({ domain: "containers", use: "home", where: "/mnt/user/backups/bombvault/containers", retention: KEEP_BALANCED }),
    section({ domain: "vms", use: "home", where: "/mnt/user/backups/bombvault/vms", retention: KEEP_OWN, own: ["retention"] }),
    section({ domain: "flash", use: "home", where: "/mnt/user/backups/bombvault/flash", retention: KEEP_BALANCED }),
    section({ domain: "files", use: "home", where: "/mnt/user/backups/bombvault/files", retention: KEEP_BALANCED }),
    section({ domain: "config", use: "home", where: "/mnt/user/backups/bombvault/selfbackup", retention: KEEP_BALANCED }),
  ],
  retention: KEEP_BALANCED,
  protection: { immutable: false, testable: false },
  capacity: {
    at: NOW,
    usedBytes: 412 * GB,
    freeBytes: 3.1 * TB,
    totalBytes: 3.5 * TB,
    source: "statfs",
    storedBytes: 380 * GB,
    growthBytesPerWeek: 95 * GB,
    weeksToFull: 33,
  },
};

export const STORAGE_BOX = {
  id: "destination:box",
  object: "destination",
  kind: "offsite",
  provider: "storagebox",
  mark: "IconHetzner",
  backend: "rclone",
  name: "Storage Box",
  where: "rclone:storagebox:bombvault",
  enabled: true,
  offPremises: true,
  sections: [
    section({
      domain: "containers",
      targetId: "t-box-containers",
      where: "rclone:storagebox:bombvault/containers",
      primary: true,
      retention: KEEP_OWN,
      own: ["retention", "compression", "limits", "enabled"],
      lastCopy: { at: NOW - 3600, ok: true },
    }),
    section({
      domain: "flash",
      targetId: "t-box-flash",
      where: "rclone:storagebox:bombvault/flash",
      retention: KEEP_BALANCED,
      compression: "max",
      limitUpload: 512,
      own: ["retention", "compression", "limits"],
      lastCopy: { at: NOW - 7200, ok: true },
    }),
    section({ domain: "config", targetId: "t-box-config", where: "rclone:storagebox:bombvault/selfbackup" }),
  ],
  retention: KEEP_LONG,
  compression: "auto",
  limitUpload: 0,
  limitDownload: 0,
  protection: { immutable: false, testable: false },
  capacity: {
    at: NOW,
    usedBytes: 268 * GB,
    freeBytes: 756 * GB,
    totalBytes: TB,
    source: "sftp",
    storedBytes: 268 * GB,
    growthBytesPerWeek: 6 * GB,
    weeksToFull: 126,
  },
};

export const B2 = {
  id: "destination:b2",
  object: "destination",
  kind: "offsite",
  provider: "b2",
  mark: "IconBackblaze",
  backend: "s3",
  name: "Backblaze B2",
  where: "s3:https://s3.eu-central-003.backblazeb2.com/bv-fotos",
  credsRef: "set-b2",
  enabled: true,
  offPremises: true,
  sections: [
    section({
      domain: "files",
      targetId: "t-b2-files",
      where: "s3:https://s3.eu-central-003.backblazeb2.com/bv-fotos/files",
      immutable: true,
    }),
  ],
  retention: KEEP_LONG,
  compression: "auto",
  limitUpload: 0,
  limitDownload: 0,
  protection: { immutable: true, testable: false },
  capacity: { unsupported: true, storedBytes: 1.8 * GB, growthBytesPerWeek: 1.1 * GB },
};

export const REST = {
  id: "target:t-rest",
  object: "target",
  kind: "offsite",
  provider: "",
  backend: "rest",
  name: "NAS bei den Eltern mit einem ziemlich langen Namen",
  where: "rest:https://tower-2.local:8000/tower/containers",
  enabled: false,
  offPremises: true,
  sections: [
    section({
      domain: "vms",
      targetId: "t-rest",
      where: "rest:https://tower-2.local:8000/tower/containers",
      enabled: false,
      immutable: true,
      lastTamper: { domain: "vms", at: NOW - 3 * DAY, protected: true, detail: "" },
    }),
  ],
  retention: KEEP_LONG,
  compression: "auto",
  limitUpload: 0,
  limitDownload: 0,
  protection: { immutable: true, testable: true, lastTamper: { at: NOW - 3 * DAY, protected: true } },
  capacity: {},
};

export const ARCHIVE = {
  id: "repo:archive",
  object: "repo",
  kind: "local",
  provider: "",
  backend: "local",
  name: "Archiv",
  where: "/mnt/disks/archiv/bombvault",
  enabled: true,
  offPremises: false,
  sections: [],
  retention: KEEP_BALANCED,
  compression: "max",
  limitUpload: 0,
  limitDownload: 0,
  protection: { immutable: false, testable: false },
  capacity: { at: NOW, usedBytes: 7.6 * TB, freeBytes: 0.4 * TB, totalBytes: 8 * TB, source: "statfs", weeksToFull: 3 },
};

export const LOCATIONS = [LOCAL, ARCHIVE, STORAGE_BOX, B2, REST];

function target(id: string, domain: string, destinationId: string, over: Record<string, unknown> = {}) {
  return {
    id,
    domain,
    name: id,
    repo: `rclone:storagebox:bombvault/${domain}`,
    credsRef: "",
    storageClass: "",
    immutable: false,
    schedule: "",
    retentionKeepLast: 0,
    retentionKeepDaily: 14,
    retentionKeepWeekly: 8,
    retentionKeepMonthly: 12,
    retentionKeepYearly: 3,
    compression: "auto",
    limitUpload: 0,
    limitDownload: 0,
    growthBudgetGb: 0,
    enabled: true,
    createdAt: NOW - 30 * DAY,
    sortOrder: 1,
    destinationId,
    own: [],
    ...over,
  };
}

const TARGETS = [
  target("t-box-containers", "containers", "box", { sortOrder: 0, own: ["retention", "compression", "limits", "enabled"] }),
  target("t-box-flash", "flash", "box", { compression: "max", limitUpload: 512, own: ["retention", "compression", "limits"] }),
  target("t-box-config", "config", "box"),
  target("t-b2-files", "files", "b2", { repo: "s3:https://s3.eu-central-003.backblazeb2.com/bv-fotos/files", immutable: true }),
  target("t-rest", "vms", "", { repo: "rest:https://tower-2.local:8000/tower/containers", immutable: true, enabled: false }),
];

function destination(location: typeof STORAGE_BOX | typeof B2, id: string) {
  return {
    id,
    name: location.name,
    provider: location.provider,
    mark: location.mark,
    repo: location.where,
    credsRef: "",
    storageClass: "",
    immutable: location.protection.immutable,
    createdAt: NOW - 30 * DAY,
    domains: location.sections.map((s) => s.domain),
    retention: location.retention,
    compression: location.compression,
    limitUpload: 0,
    limitDownload: 0,
    enabled: true,
    offPremises: true,
  };
}

function defaultRow(domain: string, skip: string[]) {
  return {
    domain,
    exists: true,
    home: "",
    homeKind: "domain",
    homeOff: false,
    skip,
    paused: false,
    confirmedAt: NOW - 30 * DAY,
    counts: { follow: 3, own: 1, open: 0, chosenNoRun: 0 },
    unreadable: false,
  };
}

// Containers go to the Storage Box, folders to B2, and VMs stay local: their
// only target is switched off.
const DEFAULTS = [defaultRow("containers", []), defaultRow("vms", ["*"]), defaultRow("files", [])];

const OPTION_TARGETS: Record<string, { id: string; name: string; enabled: boolean }[]> = {
  containers: [{ id: "t-box-containers", name: STORAGE_BOX.name, enabled: true }],
  vms: [{ id: "t-rest", name: REST.name, enabled: false }],
  files: [{ id: "t-b2-files", name: B2.name, enabled: true }],
};

function options(domain: string) {
  return {
    domain,
    unreadable: false,
    paused: false,
    homes: [
      { id: "", name: "", location: `/mnt/user/backups/bombvault/${domain}`, kind: "domain", scheme: "" },
      { id: "archive", name: ARCHIVE.name, location: ARCHIVE.where, kind: "local", scheme: "" },
    ],
    targets: OPTION_TARGETS[domain].map((t) => ({ ...t, primary: false, appendOnly: false, hint: "" })),
    sendTo: OPTION_TARGETS[domain]
      .filter((t) => t.enabled)
      .map((t) => ({ kind: "direct", repoId: "", targetId: t.id, name: t.name, location: "" })),
    segmentLocks: {},
    default: DEFAULTS.find((row) => row.domain === domain),
  };
}

/** What switching a target on or off for a section means, as the server counts it. */
export const IMPACT = {
  dropped: [],
  added: [{ targetId: "t-new", name: B2.name, items: 7, snapshots: 32, unknown: false, uncheckable: [] }],
  openTakeHome: 0,
  home: "",
  skip: [],
};

/** A container that copies by a rule of its own, so the page counts one entry
 *  that differs. */
export const OWN_ENTRY = "jellyfin";

const CONTAINERS = [
  { name: OWN_ENTRY, placement: { ...PLACEMENT, homeFollows: false, copiesFollow: false, skip: ["*"], segment: "local" } },
  { name: "gitea", placement: { ...PLACEMENT, homeFollows: false } },
];

function provider(id: string, name: string, group: string, route: string, over: Record<string, unknown> = {}) {
  return { id, name, backend: route === "rclone" ? id : "", group, route, auth: "login", ...over };
}

const PROVIDERS = [
  provider("path", "Mounted path", "server", "path", { mark: "IconFolder", auth: "none" }),
  provider("rest", "Rest server", "server", "rest", { mark: "IconShield", lock: true }),
  provider("storagebox", "Hetzner Storage Box", "server", "rclone", { mark: "IconHetzner", auth: "sshkey" }),
  provider("minio", "MinIO", "selfs3", "s3", { lock: true }),
  provider("b2", "Backblaze B2", "storage", "s3", { mark: "IconBackblaze", lock: true }),
  provider("gdrive", "Google Drive", "cloud", "rclone", { auth: "token" }),
];

function field(name: string, over: Record<string, unknown> = {}) {
  return { name, help: "", required: false, secret: false, password: false, advanced: false, essential: true, default: "", ...over };
}

// The form rclone describes for the Storage Box, as far as a test fills it.
const BACKENDS = {
  storagebox: {
    name: "sftp",
    description: "",
    options: [field("host", { required: true }), field("user"), field("pass", { password: true }), field("port", { essential: false })],
  },
};

const RCLONE_SECTIONS = [
  {
    name: "storagebox",
    type: "sftp",
    options: [
      { name: "host", value: "u123456.your-storagebox.de" },
      { name: "user", value: "u123456" },
      { name: "pass", hidden: true },
    ],
  },
  { name: "nas", type: "smb", options: [{ name: "host", value: "nas.local" }] },
];

/** What removing the Storage Box detaches: one part per section that copies to it. */
function part(domain: string, entries: number, backups: number, primary = false) {
  return {
    domain,
    targetId: `t-box-${domain}`,
    repo: `rclone:storagebox:bombvault/${domain === "config" ? "selfbackup" : domain}`,
    primary,
    appendOnly: false,
    entries,
    backups,
    listedAt: NOW - 3600,
    lastCopyAt: NOW - 3600,
  };
}

export const REMOVAL = { sections: [part("containers", 7, 62, true), part("flash", 1, 14), part("config", 1, 9)] };

function emptied(domain: string, entries: number, backups: number, kept?: string) {
  const { targetId, repo } = part(domain, entries, backups);
  return { domain, targetId, repo, entries, backups, ...(kept ? { kept } : {}) };
}

/** What deleting the backups at the Storage Box takes away: flash is
 *  append-only there and stays, and one container has no other backup. */
export const ERASURE = {
  removal: REMOVAL,
  sections: [emptied("containers", 7, 62), emptied("flash", 1, 14, "append-only"), emptied("config", 1, 9)],
  backups: 71,
  entries: 8,
  backupsOnlyThere: 12,
  lastCopies: [{ domain: "containers", identity: "container:paperless", backups: 12 }],
  key: "erase:box",
};

/** What moving the flash copy from the Storage Box to B2 copies. */
export const MOVE = {
  domain: "flash",
  targetId: "t-box-flash",
  from: "rclone:storagebox:bombvault/flash",
  to: "s3:https://s3.eu-central-003.backblazeb2.com/bv-fotos/flash",
  backups: 14,
  pending: 14,
  key: "move:t-box-flash",
};

const CRED_SETS = [
  { id: "set-b2", name: "B2 Fotos", s3KeyId: "0034f9e8d7c6", s3Region: "eu-central-003", restUser: "", s3StorageClass: "", s3SecretSet: true, restPasswordSet: false },
];

export interface Staged {
  /** Every write the page sent, in order. */
  writes: { method: string; path: string; body: unknown }[];
  /** What the server answers to a changed connection. A test replaces it to
   *  stage a refusal. */
  connection: Record<string, unknown>;
  /** What the server answers to the question what a move would copy. */
  move: Record<string, unknown>;
}

/**
 * stageStorage answers the storage routes with the fixtures above and keeps
 * what the page writes. A write is accepted without changing the fixtures, so
 * a test asserts on the request and not on a second read.
 */
export async function stageStorage(page: Page, width: number, lang = "de"): Promise<Staged> {
  const staged: Staged = { writes: [], connection: { ok: true, warnings: [] }, move: { ok: true, move: MOVE } };
  const record = (route: Route, answer: unknown) => {
    const request = route.request();
    staged.writes.push({ method: request.method(), path: new URL(request.url()).pathname, body: request.postDataJSON() });
    return route.fulfill({ json: answer });
  };

  await page.route(
    (url) => url.pathname === "/api/storage/locations",
    (route) => route.fulfill({ json: { ok: true, locations: LOCATIONS } }),
  );
  await page.route(
    (url) => url.pathname.startsWith("/api/storage/locations/"),
    (route) => {
      const id = decodeURIComponent(new URL(route.request().url()).pathname.slice("/api/storage/locations/".length));
      const location = LOCATIONS.find((l) => l.id === id);
      if (!location) return route.fulfill({ status: 404, json: { ok: false, error: "no such storage location" } });
      return route.fulfill({ json: { ok: true, location } });
    },
  );
  await page.route(
    (url) => url.pathname === "/api/offsite/targets",
    (route) => route.fulfill({ json: { ok: true, targets: TARGETS } }),
  );
  await page.route(
    (url) => url.pathname.startsWith("/api/offsite/targets/"),
    (route) => {
      const method = route.request().method();
      if (new URL(route.request().url()).pathname.endsWith("/move")) {
        return method === "POST" ? record(route, { ok: true, started: true, move: MOVE }) : route.fulfill({ json: staged.move });
      }
      if (method === "PUT") return record(route, { ok: true, warnings: [] });
      return record(route, { ok: true, reachable: true, initialized: true, testable: true, protected: true });
    },
  );
  await page.route(
    (url) => url.pathname.startsWith("/api/offsite/destinations"),
    (route) => {
      const { pathname } = new URL(route.request().url());
      if (pathname.endsWith("/connection")) return record(route, staged.connection);
      if (pathname.endsWith("/erasure")) {
        return route.request().method() === "POST"
          ? record(route, { ok: true, started: true, erasure: ERASURE })
          : route.fulfill({ json: { ok: true, erasure: ERASURE } });
      }
      if (pathname.endsWith("/removal")) {
        return route.request().method() === "POST" ? record(route, { ok: true }) : route.fulfill({ json: { ok: true, removal: REMOVAL } });
      }
      if (route.request().method() === "PUT") return record(route, { ok: true, warnings: [] });
      if (pathname.includes("/domains/")) return record(route, { ok: true, created: true, target: target("t-new", pathname.split("/").pop() ?? "", "b2") });
      if (route.request().method() === "POST") return record(route, { ok: true, destination: { ...destination(B2, "new"), name: "Rest server" } });
      return route.fulfill({ json: { ok: true, destinations: [destination(STORAGE_BOX, "box"), destination(B2, "b2")] } });
    },
  );
  await page.route("**/api/cloud/creds-sets", (route) => {
    if (route.request().method() === "POST") return record(route, { ok: true });
    return route.fulfill({ json: { ok: true, sets: CRED_SETS } });
  });
  await page.route("**/api/rclone", (route) =>
    route.fulfill({ json: { ok: true, remotes: ["storagebox", "nas"], sections: RCLONE_SECTIONS } }),
  );
  // A path that was really saved would move the harness's own repositories.
  await page.route("**/api/settings", (route) => (route.request().method() === "PUT" ? record(route, { ok: true }) : route.fallback()));
  // Without Docker the harness has no media servers to offer.
  await page.route("**/api/settings/streaming", (route) =>
    route.fulfill({
      json: {
        ok: true,
        settings: { enabled: true, mediaServers: ["jellyfin"], mediaServersAuto: false, thresholdMbit: 2, limitKiB: 512, holdMin: 5 },
        candidates: [{ name: "jellyfin", image: "jellyfin/jellyfin", hostNetwork: false }],
        streaming: "",
      },
    }),
  );
  await page.route("**/api/repos/*", (route) => record(route, { ok: true }));
  await page.route("**/api/placement/defaults", (route) => route.fulfill({ json: { ok: true, defaults: DEFAULTS } }));
  await page.route(
    (url) => url.pathname === "/api/placement/options",
    (route) => {
      const domain = new URL(route.request().url()).searchParams.get("domain") ?? "";
      return route.fulfill({ json: { ok: true, options: options(domain) } });
    },
  );
  await page.route(
    (url) => url.pathname.startsWith("/api/placement/default/"),
    (route) => record(route, { ok: true, impact: IMPACT }),
  );
  await page.route(
    (url) => url.pathname === "/api/placement/new-target-preview",
    (route) =>
      route.fulfill({
        json: {
          ok: true,
          preview: { items: 7, formerlyExcluded: [{ identity: `container:${OWN_ENTRY}`, skip: ["*"] }], defaultExcludes: false, snapshots: 32, bytes: null, unreadable: [] },
        },
      }),
  );
  await page.route("**/api/containers", (route) => route.fulfill({ json: { ok: true, containers: CONTAINERS } }));
  await page.route("**/api/containers/*", (route) => record(route, { ok: true, placement: PLACEMENT }));
  await page.route(
    (url) => url.pathname.startsWith("/api/items/") && url.pathname.endsWith("/placement/preview"),
    (route) =>
      route.fulfill({
        json: { ok: true, added: [{ targetId: "t-box-containers", name: STORAGE_BOX.name, snapshots: 12, uncheckable: [] }], dropped: [] },
      }),
  );
  await page.route("**/api/offsite/providers", (route) => route.fulfill({ json: { ok: true, providers: PROVIDERS, backends: BACKENDS } }));
  await page.route("**/api/offsite/group-receivers", (route) =>
    route.fulfill({ json: { ok: true, receivers: [{ memberId: "m-tower", name: "tower-2", url: "https://tower-2.local:8000", needsAddress: false }] } }),
  );
  await page.route("**/api/offsite/drafts/check", (route) => route.fulfill({ json: { ok: true, reachable: true, initialized: false } }));
  // The stored locale is the look, and the server's display prefs must not
  // overwrite it while the page boots.
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.addInitScript((code) => window.localStorage.setItem("bv-lang", code), lang);
  await page.setViewportSize({ width, height: 900 });
  return staged;
}

export interface Layout {
  docPan: number;
  mainPan: number | null;
  clipped: string[] | null;
  overflowing: string[] | null;
}

/** What runs past the edge of the page: the scrollers, controls and text. */
export function measureLayout(page: Page): Promise<Layout> {
  return page.evaluate(() => {
    const main = document.querySelector("#bv-main");
    const vw = window.innerWidth;
    const past = (el: Element) => {
      const r = el.getBoundingClientRect();
      return r.width > 0 && (r.right > vw + 1 || r.left < -1);
    };
    const name = (el: Element) => (el.getAttribute("aria-label") ?? el.textContent ?? "").trim().slice(0, 40);
    return {
      docPan: document.documentElement.scrollWidth - vw,
      mainPan: main ? main.scrollWidth - main.clientWidth : null,
      clipped: main
        ? [...main.querySelectorAll("button, a, input, select, textarea, [role='switch']")].filter(past).map(name)
        : null,
      overflowing: main ? [...main.querySelectorAll("span, p, code, li, label")].filter(past).map(name) : null,
    };
  });
}

/** Waits for every animation that ends; the pictures on these pages loop. */
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
