// The storage locations a fresh harness database cannot have: one of each kind
// of object, staged at the route layer with the records their pages write
// through.
import type { Page, Route } from "@playwright/test";

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

const CRED_SETS = [
  { id: "set-b2", name: "B2 Fotos", s3KeyId: "0034f9e8d7c6", s3Region: "eu-central-003", restUser: "", s3StorageClass: "", s3SecretSet: true, restPasswordSet: false },
];

export interface Staged {
  /** Every write the page sent, in order. */
  writes: { method: string; path: string; body: unknown }[];
}

/**
 * stageStorage answers the storage routes with the fixtures above and keeps
 * what the page writes. A write is accepted without changing the fixtures, so
 * a test asserts on the request and not on a second read.
 */
export async function stageStorage(page: Page, width: number, lang = "de"): Promise<Staged> {
  const staged: Staged = { writes: [] };
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
      if (route.request().method() === "PUT") return record(route, { ok: true, warnings: [] });
      return record(route, { ok: true, reachable: true, initialized: true, testable: true, protected: true });
    },
  );
  await page.route(
    (url) => url.pathname.startsWith("/api/offsite/destinations"),
    (route) => {
      if (route.request().method() === "PUT") return record(route, { ok: true, warnings: [] });
      return route.fulfill({ json: { ok: true, destinations: [destination(STORAGE_BOX, "box"), destination(B2, "b2")] } });
    },
  );
  await page.route("**/api/cloud/creds-sets", (route) => {
    if (route.request().method() === "POST") return record(route, { ok: true });
    return route.fulfill({ json: { ok: true, sets: CRED_SETS } });
  });
  await page.route("**/api/rclone", (route) => route.fulfill({ json: { ok: true, remotes: ["storagebox", "nas"] } }));
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
