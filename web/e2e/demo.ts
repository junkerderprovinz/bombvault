// The demo server the documentation pictures show: an Unraid box that has
// been backing up for a few weeks. Its containers, VM, folders and dataset,
// their runs, the three storage locations, the schedule, the notifications,
// the group it is paired with and what anomaly detection found. Everything is
// staged at the route layer on top of the helpers the other specs use, so the
// SPA and the binary are real and only the answers are made up.
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import type { Page, Route } from "@playwright/test";

import { container, dataset, fileSet, item, stageEntries, vm, type StagedRun } from "./entries";
import { stageOverview } from "./overview";
import { PLACEMENT } from "./placement";
import { blockServerLook } from "./serverLook";

const NOW = Math.floor(Date.now() / 1000);
const MIN = 60;
const HOUR = 3600;
const DAY = 86400;
const MB = 1024 ** 2;
const GB = 1024 ** 3;
const TB = 1024 * GB;
const iso = (at: number) => new Date(at * 1000).toISOString();

/** The night the backups ran, so every run of one night stands at 03:xx. */
export function night(daysAgo: number, minute = 0): number {
  const d = new Date();
  d.setHours(3, minute, 0, 0);
  if (d.getTime() / 1000 > NOW) d.setDate(d.getDate() - 1);
  d.setDate(d.getDate() - daysAgo);
  return Math.floor(d.getTime() / 1000);
}

/**
 * words reads the app's own strings for a language, so a spec can name a
 * button by its translation key and render the pictures in any language the
 * app speaks. A key the locale lacks falls back to English, as in the app.
 */
export function words(lang: string): (key: string) => string {
  const lib = fileURLToPath(new URL("../src/lib/", import.meta.url));
  const source = readFileSync(`${lib}i18n.ts`, "utf8");
  const block = (text: string, start: string) => {
    const from = text.indexOf(start);
    return text.slice(from, text.indexOf("\n}", from));
  };
  const parse = (text: string) => {
    const table: Record<string, string> = {};
    for (const [, key, literal] of text.matchAll(/^\s*"([^"]+)":\s*("(?:[^"\\]|\\.)*")/gm)) table[key] = JSON.parse(literal);
    return table;
  };
  const en = parse(block(source, "export const en = {"));
  const own =
    lang === "en"
      ? {}
      : lang === "de"
        ? parse(block(source, "export const de: Translations = {"))
        : parse(readFileSync(`${lib}locales/${lang}.ts`, "utf8"));
  return (key) => {
    const text = own[key] ?? en[key];
    if (text === undefined) throw new Error(`no string ${key}`);
    return text;
  };
}

// Entries

const FILES_DOCS = "d0c5d0c5d0c5d0c5d0c5d0c5d0c5d0c5";
const FILES_PHOTOS = "9407059407059407059407059407a0b1";
export const ZFS_APPDATA = "2fa5a0d2fa5a0d2fa5a0d2fa5a0d2fa5";
const VM_NAME = "homeassistant";

type Json = Record<string, unknown>;

/** The fourteen nights of the run strip, newest first. */
function strip(prefix: string, statuses: Record<number, string> = {}) {
  return Array.from({ length: 14 }, (_, i) => ({
    id: `${prefix}-${i}`,
    kind: "backup",
    status: statuses[i] ?? "success",
    startedAt: night(i, 5),
  }));
}

interface Entry {
  kind: string;
  key: string;
  name: string;
  bytes: number;
  /** The minute of the night it runs at. */
  minute: number;
  over?: Json;
  statuses?: Record<number, string>;
}

const ENTRIES: Entry[] = [
  { kind: "container", key: "plex", name: "plex", bytes: 18.4 * GB, minute: 2, over: { effectiveSchedule: { kind: "own", spec: "daily 04:30" } } },
  { kind: "container", key: "immich", name: "immich", bytes: 38.6 * GB, minute: 6, over: { stack: "immich" } },
  { kind: "container", key: "immich-postgres", name: "immich-postgres", bytes: 2.1 * GB, minute: 9, over: { stack: "immich", dbTier: "curated" } },
  { kind: "container", key: "nextcloud", name: "nextcloud", bytes: 46.2 * GB, minute: 11, over: { stack: "nextcloud" } },
  { kind: "container", key: "mariadb", name: "mariadb", bytes: 1.4 * GB, minute: 18, over: { stack: "nextcloud", dbTier: "curated" } },
  { kind: "container", key: "paperless-ngx", name: "paperless-ngx", bytes: 6.3 * GB, minute: 19, statuses: { 3: "failed" } },
  { kind: "container", key: "vaultwarden", name: "vaultwarden", bytes: 120 * MB, minute: 21 },
  { kind: "vm", key: VM_NAME, name: VM_NAME, bytes: 32 * GB, minute: 25 },
  { kind: "files", key: FILES_DOCS, name: "Documents", bytes: 12.7 * GB, minute: 34 },
  { kind: "files", key: FILES_PHOTOS, name: "Photos", bytes: 418 * GB, minute: 36 },
  { kind: "zfs", key: ZFS_APPDATA, name: "cache/appdata", bytes: 96 * GB, minute: 0 },
  { kind: "flash", key: "flash", name: "flash", bytes: 1.1 * GB, minute: 44 },
  { kind: "config", key: "config", name: "config", bytes: 48 * MB, minute: 45 },
];

const APPS = new Set(["container", "vm"]);
const NIGHTLY = { kind: "everything", spec: "daily 03:00" };

const ITEMS: Json[] = [
  ...ENTRIES.map((e) =>
    item({
      kind: e.kind,
      key: e.key,
      name: e.name,
      effectiveSchedule: NIGHTLY,
      lastBackup: night(0, e.minute) + 70,
      sourceBytes: e.bytes,
      state: APPS.has(e.kind) ? "running" : undefined,
      installed: APPS.has(e.kind) ? true : undefined,
      runs: strip(e.key, e.statuses),
      ...e.over,
    }),
  ),
  item({ key: "bombvault", name: "bombvault", self: true, included: false, lastBackup: 0, lastRunStatus: "", sourceBytes: null, runs: [] }),
];

/** The same server on its first start: everything found, nothing backed up
 *  and nothing on a schedule yet. */
export function freshItems(scheduled = false): Json[] {
  return ITEMS.filter((i) => ["container", "vm", "flash"].includes(i.kind as string)).map((i) => ({
    ...i,
    included: scheduled && i.kind === "container" && !i.self,
    effectiveSchedule: scheduled && i.kind === "container" ? { kind: "domain", spec: "daily 02:00" } : { kind: "none", spec: "", reason: "schedule-off" },
    lastBackup: 0,
    lastRunStatus: "",
    sourceBytes: null,
    runs: [],
  }));
}

/** A snapshot id that looks like restic's and stays the same from run to run. */
function snapshotId(seed: string): string {
  let h = 0x811c9dc5;
  let id = "";
  for (let round = 0; round < 2; round++) {
    for (const c of `${seed}${round}`) h = Math.imul(h ^ c.charCodeAt(0), 0x01000193) >>> 0;
    id += h.toString(16).padStart(8, "0");
  }
  return id;
}

const FAIL_REASON = "restic backup failed: unable to create lock in backend: repository is already locked by PID 4211 on tower";

/** The backups of one entry, newest first: one a night for two weeks and one
 *  a week before that. */
function backupsOf(e: Entry): StagedRun[] {
  const days = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 20, 27, 34];
  return days.map((daysAgo, i) => {
    const failed = e.statuses?.[daysAgo] === "failed";
    const start = night(daysAgo, e.minute);
    return {
      id: `${e.key}-run-${i}`,
      targetId: `tg-${e.key}`,
      kind: "backup",
      status: failed ? "failed" : "success",
      startedAt: start,
      finishedAt: start + 70 + (i % 4) * 9,
      snapshotId: failed ? "" : snapshotId(`${e.key}-${i}`),
      bytes: failed ? 0 : Math.round(e.bytes * (0.004 + (i % 5) * 0.002)),
      sourceBytes: Math.round(e.bytes * (1 - daysAgo * 0.002)),
      error: failed ? FAIL_REASON : "",
      acknowledged: failed,
      groupId: "",
      target: e.name,
      domain: e.kind,
    };
  });
}

const RUNS: Record<string, StagedRun[]> = Object.fromEntries(ENTRIES.map((e) => [`${e.kind}:${e.key}`, backupsOf(e)]));

/** The runs of the last few nights across every entry, newest first. */
const RECENT_RUNS: Json[] = Object.values(RUNS)
  .flatMap((runs) => runs.slice(0, 3))
  .sort((a, b) => (b.startedAt as number) - (a.startedAt as number));

const DEFERRAL = {
  retryMin: 30,
  windowFrom: "22:00",
  windowTo: "06:00",
  maxHours: 12,
  atDeadline: "back-up",
  preset: "plex",
  apiKeySet: false,
  pausedMin: 30,
  failOpen: true,
  netKiB: 0,
  netMin: 5,
};

const CONTAINERS: Json[] = [
  container({ name: "plex", image: "lscr.io/linuxserver/plex:latest", ip: "192.168.1.30", scheduleCadence: "daily 04:30", deferral: DEFERRAL, placement: PLACEMENT }),
  container({ name: "immich", image: "ghcr.io/immich-app/immich-server:release", ip: "192.168.1.31", stack: "immich", placement: PLACEMENT }),
  container({ name: "immich-postgres", image: "ghcr.io/immich-app/postgres:16-vectorchord", ip: "192.168.1.32", stack: "immich", dbEngine: "postgres", dbTier: "curated", placement: PLACEMENT }),
  container({ name: "nextcloud", image: "lscr.io/linuxserver/nextcloud:latest", ip: "192.168.1.33", stack: "nextcloud", placement: PLACEMENT }),
  container({ name: "mariadb", image: "lscr.io/linuxserver/mariadb:latest", ip: "192.168.1.34", stack: "nextcloud", dbEngine: "mariadb", dbTier: "curated", placement: PLACEMENT }),
  container({ name: "paperless-ngx", image: "ghcr.io/paperless-ngx/paperless-ngx:latest", ip: "192.168.1.35", placement: PLACEMENT }),
  container({ name: "vaultwarden", image: "vaultwarden/server:latest", ip: "192.168.1.36", placement: PLACEMENT }),
].map((c) => ({ ...c, lastBackup: night(0, 5) + 70, lastBackupStarted: night(0, 5), status: "Up 9 days" }));

const VMS: Json[] = [vm({ name: VM_NAME, libvirtName: VM_NAME, lastBackup: night(0, 25) + 400, lastBackupStarted: night(0, 25), placement: PLACEMENT })];

const FILE_SETS: Json[] = [
  fileSet({ id: FILES_DOCS, name: "Documents", path: "documents", lastBackup: night(0, 34) + 50 }),
  fileSet({ id: FILES_PHOTOS, name: "Photos", path: "photos", lastBackup: night(0, 36) + 380 }),
];

const DATASETS: Json[] = [
  dataset({
    id: ZFS_APPDATA,
    dataset: "cache/appdata",
    hostMountpoint: "/mnt/cache/appdata",
    repoEffective: "/mnt/user/backups/bombvault/zfs",
    lastBackup: night(0, 0) + 140,
    lastCheckAt: night(0, 0) + 140,
    members: ["", "plex", "immich", "nextcloud"].map((rel, i) => ({
      dataset: rel ? `cache/appdata/${rel}` : "cache/appdata",
      relPath: rel,
      hostMountpoint: rel ? `/mnt/cache/appdata/${rel}` : "/mnt/cache/appdata",
      outcome: "backed-up",
      isNew: false,
      usedByDataset: [2.4, 18.4, 38.6, 36.6][i] * GB,
      lastBackupAt: night(0, 0) + 140,
    })),
    sites: 2,
    rule321: "met",
  }),
];

// Storage locations

const KEEP_BALANCED = { keepLast: 0, keepDaily: 7, keepWeekly: 4, keepMonthly: 6, keepYearly: 2 };

const SECTIONS = ["containers", "vms", "flash", "files", "zfs", "config"];

function section(over: Json) {
  return {
    domain: "containers",
    use: "copy",
    where: "",
    enabled: true,
    immutable: false,
    retention: KEEP_BALANCED,
    compression: "auto",
    limitUpload: 0,
    limitDownload: 0,
    own: [],
    ...over,
  };
}

const B2_BUCKET = "s3:https://s3.us-west-004.backblazeb2.com/tower-bombvault";
const NAS_REPO = "rest:https://offsite-nas.home.arpa:8000/tower";

const LOCAL = {
  id: "path:7c1e5a90b3d2f461",
  object: "path",
  kind: "local",
  provider: "",
  backend: "local",
  name: "bombvault",
  where: "/mnt/user/backups/bombvault",
  enabled: true,
  offPremises: false,
  sections: SECTIONS.map((domain) =>
    section({ domain, use: "home", where: `/mnt/user/backups/bombvault/${domain === "config" ? "selfbackup" : domain}` }),
  ),
  retention: KEEP_BALANCED,
  protection: { immutable: false, testable: false },
  capacity: {
    at: NOW,
    usedBytes: 1.9 * TB,
    freeBytes: 6.1 * TB,
    totalBytes: 8 * TB,
    source: "statfs",
    storedBytes: 486 * GB,
    growthBytesPerWeek: 11 * GB,
    weeksToFull: 380,
  },
};

/** The same folder on a first start, with nothing in it yet to measure. */
export const FRESH_LOCAL = { ...LOCAL, capacity: { at: NOW, usedBytes: 1.4 * TB, freeBytes: 6.6 * TB, totalBytes: 8 * TB, source: "statfs" } };

const B2_SECTIONS = ["containers", "files", "flash", "config"];

const B2 = {
  id: "destination:b2",
  object: "destination",
  kind: "offsite",
  provider: "b2",
  mark: "IconBackblaze",
  backend: "s3",
  name: "Backblaze B2",
  where: B2_BUCKET,
  credsRef: "set-b2",
  enabled: true,
  offPremises: true,
  sections: B2_SECTIONS.map((domain) =>
    section({
      domain,
      targetId: `t-b2-${domain}`,
      where: `${B2_BUCKET}/${domain === "config" ? "selfbackup" : domain}`,
      primary: domain === "containers",
      immutable: true,
      lastCopy: { at: night(0, 50), ok: true },
    }),
  ),
  retention: KEEP_BALANCED,
  compression: "auto",
  limitUpload: 0,
  limitDownload: 0,
  protection: { immutable: true, testable: true, lastTamper: { at: NOW - 2 * DAY, protected: true } },
  capacity: { unsupported: true, storedBytes: 431 * GB, growthBytesPerWeek: 7.4 * GB },
};

const NAS_SECTIONS = ["containers", "vms", "zfs"];

const NAS = {
  id: "target:t-nas-containers",
  object: "target",
  kind: "offsite",
  provider: "",
  backend: "rest",
  name: "Offsite NAS",
  where: NAS_REPO,
  enabled: true,
  offPremises: true,
  sections: NAS_SECTIONS.map((domain) =>
    section({
      domain,
      targetId: `t-nas-${domain}`,
      where: `${NAS_REPO}/${domain}`,
      immutable: true,
      lastCopy: { at: night(0, 55), ok: true },
      lastTamper: { domain, at: NOW - 3 * DAY, protected: true, detail: "" },
    }),
  ),
  retention: KEEP_BALANCED,
  compression: "auto",
  limitUpload: 0,
  limitDownload: 0,
  protection: { immutable: true, testable: true, lastTamper: { at: NOW - 3 * DAY, protected: true } },
  capacity: { at: NOW, usedBytes: 2.2 * TB, freeBytes: 1.8 * TB, totalBytes: 4 * TB, source: "rest", storedBytes: 212 * GB, growthBytesPerWeek: 4.1 * GB, weeksToFull: 440 },
};

const LOCATIONS = [LOCAL, B2, NAS];

/** The storage locations after a rebuild, where the folder sets found backups
 *  of their own at B2 and their copies wait for the decision. */
export const AWAITING_DECISION = [
  LOCAL,
  { ...B2, sections: B2.sections.map((s) => (s.domain === "files" ? { ...s, awaitsDecision: true, lastCopy: undefined } : s)) },
  NAS,
];

function target(id: string, domain: string, destinationId: string, repo: string, over: Json = {}) {
  return {
    id,
    domain,
    name: id,
    repo,
    credsRef: destinationId === "b2" ? "set-b2" : "",
    storageClass: "",
    immutable: true,
    schedule: "",
    retentionKeepLast: 0,
    retentionKeepDaily: 7,
    retentionKeepWeekly: 4,
    retentionKeepMonthly: 6,
    retentionKeepYearly: 2,
    compression: "auto",
    limitUpload: 0,
    limitDownload: 0,
    growthBudgetGb: 0,
    enabled: true,
    createdAt: NOW - 60 * DAY,
    sortOrder: 1,
    destinationId,
    own: [],
    ...over,
  };
}

const TARGETS = [
  ...B2_SECTIONS.map((d, i) => target(`t-b2-${d}`, d, "b2", `${B2_BUCKET}/${d}`, { sortOrder: i })),
  ...NAS_SECTIONS.map((d, i) => target(`t-nas-${d}`, d, "", `${NAS_REPO}/${d}`, { sortOrder: 10 + i })),
];

const B2_DESTINATION = {
  id: "b2",
  name: B2.name,
  provider: "b2",
  mark: "IconBackblaze",
  repo: B2_BUCKET,
  credsRef: "set-b2",
  storageClass: "",
  immutable: true,
  createdAt: NOW - 60 * DAY,
  domains: B2_SECTIONS,
  retention: KEEP_BALANCED,
  compression: "auto",
  limitUpload: 0,
  limitDownload: 0,
  enabled: true,
  offPremises: true,
};

const COPIES: Record<string, { id: string; name: string }[]> = {
  containers: [
    { id: "t-b2-containers", name: B2.name },
    { id: "t-nas-containers", name: NAS.name },
  ],
  vms: [{ id: "t-nas-vms", name: NAS.name }],
  files: [{ id: "t-b2-files", name: B2.name }],
  flash: [{ id: "t-b2-flash", name: B2.name }],
  config: [{ id: "t-b2-config", name: B2.name }],
  zfs: [{ id: "t-nas-zfs", name: NAS.name }],
};

function defaultRow(domain: string) {
  return {
    domain,
    exists: true,
    home: "",
    homeKind: "domain",
    homeOff: false,
    skip: [],
    paused: false,
    confirmedAt: NOW - 50 * DAY,
    counts: { follow: 4, own: 0, open: 0, chosenNoRun: 0 },
    unreadable: false,
  };
}

function placementOptions(domain: string) {
  const copies = COPIES[domain] ?? [];
  return {
    domain,
    unreadable: false,
    paused: false,
    homes: [{ id: "", name: "", location: `/mnt/user/backups/bombvault/${domain}`, kind: "domain", scheme: "" }],
    targets: copies.map((t) => ({ ...t, enabled: true, primary: false, appendOnly: true, hint: "" })),
    sendTo: copies.map((t) => ({ kind: "direct", repoId: "", targetId: t.id, name: t.name, location: "" })),
    segmentLocks: {},
    default: defaultRow(domain),
  };
}

function provider(id: string, name: string, group: string, route: string, over: Json = {}) {
  return { id, name, backend: route === "rclone" ? id : "", group, route, auth: "login", ...over };
}

const PROVIDERS = [
  provider("path", "Mounted path", "server", "path", { mark: "IconFolder", auth: "none" }),
  provider("rest", "Rest server", "server", "rest", { mark: "IconShield", lock: true }),
  provider("storagebox", "Hetzner Storage Box", "server", "rclone", { mark: "IconHetzner", auth: "sshkey" }),
  provider("sftp", "SFTP", "server", "rclone", { auth: "login" }),
  provider("minio", "MinIO", "selfs3", "s3", { lock: true }),
  provider("b2", "Backblaze B2", "storage", "s3", { mark: "IconBackblaze", lock: true }),
  provider("wasabi", "Wasabi", "storage", "s3", { lock: true }),
  provider("s3", "Amazon S3", "storage", "s3", { lock: true }),
  provider("gdrive", "Google Drive", "cloud", "rclone", { auth: "token" }),
  provider("onedrive", "OneDrive", "cloud", "rclone", { auth: "token" }),
  provider("dropbox", "Dropbox", "cloud", "rclone", { auth: "token" }),
];

const CRED_SETS = [
  {
    id: "set-b2",
    name: "Backblaze B2",
    s3KeyId: "004a1f9c3e7d2b80000000003",
    s3Region: "us-west-004",
    restUser: "",
    s3StorageClass: "",
    s3SecretSet: true,
    restPasswordSet: false,
    default: true,
    usedBy: [{ id: B2.id, name: B2.name }],
  },
  {
    id: "set-nas",
    name: "Offsite NAS",
    s3KeyId: "",
    s3Region: "",
    restUser: "tower",
    s3StorageClass: "",
    s3SecretSet: false,
    restPasswordSet: true,
    usedBy: [{ id: NAS.id, name: NAS.name }],
  },
  { id: "set-wasabi", name: "Wasabi archive", s3KeyId: "WAS3K7Q2ZP0DEMO", s3Region: "eu-central-2", restUser: "", s3StorageClass: "", s3SecretSet: true, restPasswordSet: false, usedBy: [] },
];

/** What moving the own folders of some sections to B2 would do. */
function ownFolderSteps(domains: string[]) {
  return domains.map((domain) => {
    const copy = B2_SECTIONS.includes(domain);
    const folder = domain === "config" ? "selfbackup" : domain;
    return {
      domain,
      from: `/mnt/user/backups/bombvault/${folder}`,
      to: `${B2_BUCKET}/${folder}`,
      same: false,
      ...(copy ? { copyAt: `${B2_BUCKET}/${folder}` } : {}),
      copies: COPIES[domain]?.map((c) => c.name) ?? [],
      backups: 96,
      pending: copy ? 2 : 96,
      key: `own:${domain}`,
      keep: KEEP_BALANCED,
      keepThere: KEEP_BALANCED,
    };
  });
}

/** The answer of the check before a restore, for one entry. */
function restoreCheck(req: Json) {
  if (req.kind === "stack") {
    const passed = [{ id: "repository", status: "ok" }, { id: "key", status: "ok" }];
    return { ok: true, ready: true, members: ["immich", "immich-postgres"].map((name) => ({ name, ready: true, checks: passed })) };
  }
  const beside = req.target === "beside";
  const name = String(req.name);
  return {
    ok: true,
    ready: true,
    checks: [
      { id: "repository", status: "ok", detail: "/mnt/user/backups/bombvault/containers" },
      { id: "key", status: "ok" },
      { id: "snapshot", status: "ok", detail: String(req.snapshotId ?? "").slice(0, 8) },
      { id: "space", status: "ok", need: 2.1 * GB, free: 6.1 * TB },
    ],
    plan: {
      inPlace: !beside,
      added: beside ? 4_212 : 14,
      changed: beside ? 0 : 88,
      unchanged: beside ? 0 : 4_110,
      extra: beside ? 0 : 6,
      files: [],
      listCapped: false,
      partial: false,
      missing: false,
      definition: [],
      shared: [],
      ...(beside
        ? {
            beside: {
              name: `${name}-restored`,
              folders: [{ original: `/mnt/user/appdata/${name}`, copy: `/mnt/user/appdata/${name}-restored` }],
              container: { ports: [{ container: "2283/tcp", original: "2283", copy: "2284" }], shared: [] },
            },
          }
        : { backupFirst: { status: "ok", seconds: 74, bytes: 160 * MB } }),
    },
  };
}

/** The files of a backup of the Documents folder. */
const DOCUMENT_FILES = [
  "taxes",
  "taxes/2024",
  "taxes/2024/return-2024.pdf",
  "taxes/2024/receipts.zip",
  "taxes/2025",
  "taxes/2025/return-2025.pdf",
  "taxes/2025/donations.pdf",
  "insurance",
  "insurance/home-policy.pdf",
  "insurance/car-policy.pdf",
  "contracts",
  "contracts/lease.pdf",
  "contracts/internet.pdf",
  "manuals",
  "manuals/dishwasher.pdf",
].map((path) => ({ path: `/documents/${path}`, type: path.endsWith(".pdf") || path.endsWith(".zip") ? "file" : "dir", size: 240_000 }));

// The state of each section, as the overview and the protection window read it.

function domainState(over: Json) {
  return {
    domain: "containers",
    enabled: true,
    schedule: "daily 03:00",
    coveredBy: "",
    lastSuccess: night(0, 20),
    periodSeconds: DAY,
    status: "ok",
    lastVerified: NOW - DAY,
    lastVerifiedOK: true,
    verifiedDetail: "",
    drillDetail: "",
    offsiteConfigured: true,
    offPremisesCovered: true,
    offsiteImmutable: true,
    lastTamperAt: NOW - 3 * DAY,
    lastTamperOK: true,
    lastReplicationAt: night(0, 55),
    lastReplicationOK: true,
    lastDrDrillAt: NOW - 5 * DAY,
    lastDrDrillOK: true,
    lastOffsiteSubsetAt: NOW - 2 * DAY,
    lastOffsiteSubsetOK: true,
    offsiteDrillScheduled: true,
    protection: "green",
    tamperState: "ok",
    replicationState: "ok",
    drillState: "ok",
    encryptionOn: true,
    pruneStrategySet: true,
    lastStartTest: { container: "vaultwarden", ok: true, at: NOW - 2 * DAY, detail: "" },
    ...over,
  };
}

const DOMAINS = [
  domainState({}),
  domainState({ domain: "vms", schedule: "weekly sun 04:00", lastSuccess: night(0, 25) + 400, lastStartTest: undefined }),
  domainState({ domain: "flash", lastSuccess: night(0, 44) + 30, lastStartTest: undefined }),
  domainState({ domain: "files", lastSuccess: night(0, 36) + 380, lastStartTest: undefined }),
  domainState({ domain: "zfs", schedule: "daily 03:00", lastSuccess: night(0, 0) + 140, lastStartTest: undefined }),
  domainState({ domain: "config", lastSuccess: night(0, 45) + 10, lastStartTest: undefined }),
];

/** The sections of a first start: the three switched on, nothing backed up. */
export const FRESH_DOMAINS = SECTIONS.map((domain) =>
  ["containers", "vms", "flash"].includes(domain)
    ? domainState({
        domain,
        schedule: domain === "containers" ? "daily 02:00" : "",
        lastSuccess: 0,
        status: "never",
        lastVerified: 0,
        offsiteConfigured: false,
        offPremisesCovered: false,
        offsiteImmutable: false,
        lastTamperAt: 0,
        lastReplicationAt: 0,
        lastDrDrillAt: 0,
        lastOffsiteSubsetAt: 0,
        offsiteDrillScheduled: false,
        protection: "red",
        tamperState: "",
        replicationState: "",
        drillState: "",
        lastStartTest: undefined,
      })
    : domainState({ domain, enabled: false, status: "off", schedule: "", protection: "", lastSuccess: 0 }),
);

const COVERAGE = {
  total: 13,
  protected: 13,
  domains: SECTIONS.map((domain) => {
    const total = ENTRIES.filter((e) => `${e.kind}s` === domain || e.kind === domain).length;
    return { domain, enabled: true, total, protected: total, unprotected: [] };
  }),
};

// Anomaly detection

let findingSeq = 0;
function finding(over: Json) {
  findingSeq += 1;
  return {
    id: `an-${findingSeq}`,
    detector: "new_data",
    metric: "new_data",
    severity: "warning",
    state: "open",
    scopeKind: "item",
    scopeId: "tg-nextcloud",
    targetId: "tg-nextcloud",
    domain: "container",
    part: "",
    targetName: "",
    name: "nextcloud",
    runId: "nextcloud-run-0",
    lastRunId: "nextcloud-run-0",
    lastRunAt: night(0, 11) + 70,
    observed: 4.8 * GB,
    expected: 140 * MB,
    threshold: 600 * MB,
    samples: 14,
    sensitivity: "balanced",
    details: { median: 140 * MB, mad: 40 * MB, z: 8.1, refBytes: 600 * MB },
    occurrences: 1,
    firstSeenAt: night(0, 11) + 70,
    lastSeenAt: night(0, 11) + 70,
    recoveredAt: 0,
    resolvedAt: 0,
    ackedAt: 0,
    clearedAt: 0,
    ackNote: "",
    notifiedAt: night(0, 11) + 75,
    expectable: true,
    retentionHeld: false,
    stillPresent: true,
    ...over,
  };
}

/** The one thing out of the ordinary: nextcloud took in far more than usual. */
export const WARNING = finding({});

/** paperless-ngx lost most of its documents since the night before. */
export const CRITICAL = finding({
  detector: "source",
  metric: "source_bytes_shrink",
  severity: "critical",
  scopeId: "tg-paperless-ngx",
  targetId: "tg-paperless-ngx",
  name: "paperless-ngx",
  runId: "paperless-ngx-run-0",
  lastRunId: "paperless-ngx-run-0",
  lastRunAt: night(0, 19) + 70,
  observed: 0.4 * GB,
  expected: 6.3 * GB,
  threshold: 3.1 * GB,
  details: { median: 6.3 * GB, mad: 0.1 * GB, z: 9.6 },
  lastGood: { runId: "paperless-ngx-run-1", snapshotId: snapshotId("paperless-ngx-1"), at: night(1, 19) + 70 },
  flaggedSnapshots: [snapshotId("paperless-ngx-0")],
  expectable: false,
  retentionHeld: true,
  firstSeenAt: night(0, 19) + 70,
  lastSeenAt: night(0, 19) + 70,
});

export const INFO = finding({
  detector: "duration",
  metric: "duration_slower",
  severity: "info",
  scopeId: "tg-plex",
  targetId: "tg-plex",
  name: "plex",
  observed: 412_000,
  expected: 96_000,
  threshold: 180_000,
  details: { median: 96_000, mad: 12_000, z: 5.2 },
  stillPresent: false,
});

function watched(e: Entry, open: { critical: number; warning: number; info: number }) {
  return {
    targetId: `tg-${e.key}`,
    domain: e.kind,
    name: e.name,
    scheduled: true,
    sensitivity: "",
    effective: "balanced",
    notifyMin: "",
    effectiveNotifyMin: "warning",
    learning: { samples: 14, needed: 10, newData: 14, source: 14, duration: 14, noData: false },
    typical: { sourceBytes: e.bytes, newDataBytes: e.bytes * 0.006, resticMs: 80_000 },
    dump: null,
    datasets: [],
    open,
    retentionHeld: false,
    selectionSince: 0,
    expectations: [],
  };
}

function watchedItems(findings: Json[]) {
  return ENTRIES.filter((e) => e.kind !== "config").map((e) => {
    const mine = findings.filter((f) => f.targetId === `tg-${e.key}`);
    const count = (s: string) => mine.filter((f) => f.severity === s).length;
    return watched(e, { critical: count("critical"), warning: count("warning"), info: count("info") });
  });
}

function summary(findings: Json[]) {
  const count = (s: string) => findings.filter((f) => f.severity === s).length;
  return {
    enabled: true,
    ready: true,
    generation: 21,
    open: { critical: count("critical"), warning: count("warning"), info: count("info") },
    recoveredCritical: 0,
    learningItems: 0,
    retentionHeld: findings.filter((f) => f.retentionHeld).length,
    evalErrors: 0,
    notifyMuted: false,
    backfill: { slots: 0, done: 0, failed: 0, filled: 0, withoutSummary: 0 },
    unmeasuredVolumes: [],
  };
}

/** What a run measured night by night, with the newest night off the band for a finding. */
function series(targetId: string) {
  const e = ENTRIES.find((x) => `tg-${x.key}` === targetId)!;
  const shrank = targetId === CRITICAL.targetId;
  return [
    {
      scopeKind: "item",
      part: "",
      failed: Object.keys(e.statuses ?? {}).map((d) => ({ runId: `${e.key}-run-${d}`, at: night(Number(d), e.minute) })),
      quantities: [
        {
          quantity: "sourceBytes",
          points: Array.from({ length: 28 }, (_, i) => ({
            runId: i === 27 ? `${e.key}-run-0` : `${e.key}-old-${i}`,
            at: night(27 - i, e.minute),
            value: i === 27 && shrank ? 0.4 * GB : e.bytes * (0.97 + 0.03 * Math.sin(i / 3)),
          })),
          learning: false,
          samples: 14,
          needed: 10,
          band: shrank ? { low: { metric: "source_bytes_shrink", expected: e.bytes, threshold: e.bytes / 2, samples: 14 }, high: null } : null,
        },
      ],
    },
  ];
}

const CHANGES = {
  state: "ready",
  fromAt: night(1, 19) + 70,
  toAt: night(0, 19) + 70,
  summary: {
    total: { path: "", removedBytes: 5.9 * GB, removedFiles: 4_812, addedBytes: 0, addedFiles: 0, changedBytes: 12 * MB, changedFiles: 3 },
    folders: [
      { path: "data/media/documents/originals", removedBytes: 5.1 * GB, removedFiles: 2_406, addedBytes: 0, addedFiles: 0, changedBytes: 0, changedFiles: 0 },
      { path: "data/media/documents/archive", removedBytes: 0.8 * GB, removedFiles: 2_406, addedBytes: 0, addedFiles: 0, changedBytes: 0, changedFiles: 0 },
    ],
    other: { path: "", removedBytes: 0, removedFiles: 0, addedBytes: 0, addedFiles: 0, changedBytes: 12 * MB, changedFiles: 3 },
    focus: "data/media/documents/originals",
  },
};

// The size by folder of a backup, for the Verify tab.

const FOLDERS: Record<string, { name: string; size: number; files: number; added: number }[]> = {
  immich: [
    { name: "library", size: 24.1 * GB, files: 31_208, added: 412 * MB },
    { name: "thumbs", size: 6.8 * GB, files: 62_410, added: 38 * MB },
    { name: "encoded-video", size: 4.9 * GB, files: 1_204, added: 96 * MB },
    { name: "upload", size: 1.9 * GB, files: 2_311, added: 61 * MB },
    { name: "profile", size: 12 * MB, files: 6, added: 0 },
    { name: "backups", size: 880 * MB, files: 14, added: 63 * MB },
  ],
};

function breakdown(key: string) {
  const rows = FOLDERS[key];
  if (!rows) return { state: "none", path: "", size: 0, files: 0, added: 0, children: [] };
  const sum = (field: "size" | "files" | "added") => rows.reduce((n, r) => n + r[field], 0);
  return {
    state: "ready",
    snapshot: RUNS[`container:${key}`]?.[0]?.snapshotId,
    time: iso(night(0, 6) + 70),
    first: false,
    partial: false,
    root: `/mnt/user/appdata/${key}`,
    path: "",
    size: sum("size"),
    files: sum("files"),
    added: sum("added"),
    children: rows.map((r) => ({ ...r, dir: true, open: true })),
  };
}

// Settings, schedules and notifications

const SETTINGS = {
  containersEnabled: true,
  vmsEnabled: true,
  flashEnabled: true,
  filesEnabled: true,
  zfsEnabled: true,
  configEnabled: true,
  anomalyEnabled: true,
  encryptionEnabled: true,
  recoveryKitAck: true,
  instanceName: "tower",
  receiverEnabled: true,
  fleetEnabled: true,
  pullEnabled: true,
  everythingSchedule: "daily 03:00",
  containersSchedule: "off",
  vmsSchedule: "weekly Sun 04:00",
  flashSchedule: "off",
  filesSchedule: "off",
  zfsSchedule: "off",
  configSchedule: "off",
  zfsOffsiteSchedule: "daily 05:00",
};

const NOTIFY = {
  channels: [
    { id: "unraid", kind: "unraid", enabled: true, on: "failure" },
    {
      id: "gotify",
      kind: "gotify",
      enabled: true,
      on: "failure",
      gotify: { url: "https://gotify.home.arpa", token: "********", priorityFailure: 8, prioritySuccess: 3 },
    },
  ],
  scheduledSummary: true,
  notifyOnUpdate: true,
};

// The group: this server, the NAS that keeps its copies, the office box and
// a lab server that has been away for a day.

const GROUP_ID = { nas: "m-offsite-nas", office: "m-office", lab: "m-lab" };

const GROUP = {
  ok: true,
  active: true,
  instanceId: "self",
  name: "tower",
  passwordSet: true,
  selfAddress: "https://tower.home.arpa:3443",
  waiting: [],
  ownAddresses: ["192.168.1.10", "tower.home.arpa"],
  members: [
    { id: GROUP_ID.nas, name: "Offsite NAS", version: "v10.4.2", direct: true, relay: true, address: "https://offsite-nas.home.arpa:3443" },
    { id: GROUP_ID.office, name: "office", version: "v10.4.2", direct: false, relay: true, address: "https://office.example.net:3443" },
  ],
  away: [{ id: GROUP_ID.lab, name: "lab", lastSeen: NOW - 26 * HOUR, address: "https://192.168.1.40:3443" }],
  settling: false,
  relay: {
    mode: "project",
    url: "",
    projectUrl: "wss://parleyport.halleluja.design/relay/connect",
    connected: true,
    serve: false,
    fortress: false,
    serveClients: 0,
  },
  joinedAgo: 40 * DAY,
  memberSeen: true,
};

const roleRequest = (id: string, state: string, sections: string[]) => ({
  id,
  state,
  sections,
  store: "rest",
  askedAt: iso(NOW - 40 * DAY),
  decidedAt: state === "asked" ? "" : iso(NOW - 40 * DAY + HOUR),
  decidedBy: state === "asked" ? "" : "person",
});

const NO_ROLES = { receiver: null, fetcher: null, zfs: null, keeper: null };

const ROLES = [
  {
    memberId: GROUP_ID.nas,
    name: "Offsite NAS",
    reachable: true,
    canKeep: true,
    keep: "yes",
    byMember: {
      receiver: roleRequest("rr1", "allowed", ["containers", "vms", "zfs"]),
      fetcher: null,
      zfs: { state: "allowed", items: [{ id: ZFS_APPDATA, dataset: "cache/appdata", state: "allowed" }] },
      keeper: { state: "held", at: iso(NOW - 6 * HOUR) },
    },
    forMember: NO_ROLES,
    asks: 0,
  },
  {
    memberId: GROUP_ID.office,
    name: "office",
    reachable: true,
    canKeep: true,
    keep: "yes",
    byMember: { ...NO_ROLES, fetcher: roleRequest("rr2", "asked", ["files"]) },
    forMember: { ...NO_ROLES, receiver: roleRequest("rr3", "allowed", ["files"]) },
    asks: 1,
  },
];

const SELF_ROLES = {
  receiver: [{ memberId: GROUP_ID.office, name: "office" }],
  fetcher: [],
  zfs: [],
  keeper: [{ memberId: GROUP_ID.nas, name: "Offsite NAS" }],
};

const statusOf = (domain: string, protection: string, lastSuccess: number) => ({
  domain,
  enabled: true,
  schedule: "daily 03:00",
  coveredBy: "",
  lastSuccess,
  periodSeconds: DAY,
  status: "ok",
  protection,
});

const peer = (id: string, memberId: string, name: string, over: Json) => ({
  id,
  memberId,
  name,
  url: "",
  enabled: true,
  lastPollAt: NOW - 20 * MIN,
  lastPollOk: true,
  lastPollError: "",
  lastPollInstanceName: name,
  lastPollVersion: "v10.4.2",
  lastPollDomains: [statusOf("containers", "green", night(0, 10)), statusOf("files", "green", night(0, 30))],
  createdAt: NOW - 40 * DAY,
  sortOrder: 0,
  needsPairing: false,
  direct: true,
  relay: true,
  ...over,
});

const PEERS = [
  peer("p1", GROUP_ID.nas, "Offsite NAS", { lastPollDomains: [statusOf("containers", "green", night(0, 12)), statusOf("vms", "green", night(1, 30))] }),
  peer("p2", GROUP_ID.office, "office", { direct: false }),
];

const RECEIVER_SERVER = {
  containerName: "bombvault-rest-server",
  folder: "user/restic",
  hostPath: "/mnt/user/restic",
  port: 8000,
  user: "tower",
  host: "",
  url: "http://tower.home.arpa:8000",
  createdAt: NOW - 40 * DAY,
  present: true,
  check: "protected",
  checkDetail: "",
  checkedAt: NOW - HOUR,
  logins: [{ memberId: GROUP_ID.office, name: "office", user: "office", createdAt: NOW - 30 * DAY }],
};

const RECEIVED = [
  {
    id: "r1",
    name: "office: Folders",
    repo: "/mnt/user/restic/office/files",
    deadManHours: 26,
    checkCadence: "",
    readDataPercent: 5,
    lastCheckAt: NOW - 3 * HOUR,
    lastCheckOk: true,
    lastCheckError: "",
    lastCheckReadData: false,
    enabled: true,
    createdAt: NOW - 30 * DAY,
    sortOrder: 0,
    memberId: GROUP_ID.office,
    needsPairing: false,
    lastReceived: iso(night(0, 40)),
    snapshotCount: 214,
    reachable: true,
  },
];

const DEPOSITS = {
  codeSince: iso(NOW - 40 * DAY),
  sent: [{ memberId: GROUP_ID.nas, name: "Offsite NAS", state: "held", sentAt: iso(NOW - 6 * HOUR) }],
  kept: [{ ownerId: GROUP_ID.office, ownerName: "office", state: "held", createdAt: iso(NOW - 30 * DAY), updatedAt: iso(NOW - DAY) }],
  releases: [],
};

/** The recovery code a new key deposit is sealed with, shown once. */
const RECOVERY_CODE = "R4TW8-KQ2MX-7HNPD-V9CJL";

const ZFS_SERVER = {
  id: "zfs-backup",
  name: "zfs-backup",
  host: "zfs-backup.home.arpa",
  user: "bombvault-replica",
  port: 22,
  pool: "backup",
  root: "backup/replica",
  enabled: true,
  freeBytes: 5.2 * TB,
  sizeBytes: 12 * TB,
  usedBy: [ZFS_APPDATA],
  folder: "tower",
  reachable: true,
};

const KEEP = { preset: "balanced", own: [0, 7, 4, 6, 2] };

const REPLICA = {
  target: { kind: "server", id: ZFS_SERVER.id },
  afterBackup: true,
  cadence: "",
  keep: KEEP,
  keepOnSource: 2,
  state: "ok",
  code: "",
  lastRun: iso(night(0, 3)),
  lastBytes: 1.6 * GB,
  lastSeconds: 48,
  snapshots: [0, 1, 2, 3].map((d) => ({ name: `bombvault-replica-${new Date((night(d, 3)) * 1000).toISOString().slice(0, 10)}`, created: iso(night(d, 3)) })),
  members: ["", "/plex", "/immich", "/nextcloud"].map((rel, i) => ({
    dataset: `cache/appdata${rel}`,
    volume: false,
    targetPath: `backup/replica/tower/cache/appdata${rel}`,
    state: "ok",
    code: "",
    lastBytes: [12, 410, 960, 220][i] * MB,
    mount: "",
  })),
  peerState: "",
};

// The timeline of an entry: its backups and where each one is kept.

const KIND_OF_SECTION: Record<string, string> = { containers: "container", vms: "vm", files: "files", zfs: "zfs", flash: "flash", config: "config" };

/** Every backup is local. The first copy holds the newest seven, the second the newest four. */
function timeline(domain: string, key: string) {
  const copies = COPIES[domain] ?? [];
  const runs = (RUNS[`${KIND_OF_SECTION[domain]}:${key}`] ?? []).filter((r) => r.status === "success");
  return {
    ok: true,
    places: [
      { place: "local", label: "", kind: "home", remote: false, enabled: true, appendOnly: false, state: "read" },
      ...copies.map((c) => ({ place: `offsite:${c.id}`, label: c.name, kind: "target", remote: true, enabled: true, appendOnly: true, state: "read" })),
    ],
    rows: runs.map((run, i) => ({
      key: run.snapshotId,
      time: iso(run.finishedAt),
      size: run.sourceBytes,
      places: [
        { place: "local", snapshotIds: [run.snapshotId], tags: i === 2 ? ["before update"] : [] },
        ...copies.filter((_, n) => i < [7, 4][n]).map((c) => ({ place: `offsite:${c.id}`, snapshotIds: [snapshotId(`${run.snapshotId}${c.id}`)], tags: [] })),
      ],
    })),
  };
}

// Staging

export interface Demo {
  theme: "dark" | "light";
  lang?: string;
  /** The open findings, the warning alone unless a picture needs more. */
  findings?: Json[];
  /** The entries the server lists, all of the demo unless a picture needs a first start. */
  items?: Json[];
  runs?: Json[];
  /** What localStorage holds before the page loads, beside language and theme. */
  stored?: Record<string, string>;
  locations?: Json[];
  settings?: Json;
  /** When the next backup is due, the coming night's full backup unless said otherwise. */
  nextAt?: number;
  /** The state of each section, the demo's unless a picture needs a first start. */
  domains?: Json[];
  /** How many of an entry's backups the timeline lists, so a page can show
   *  what stands below them. */
  backupsShown?: number;
}

/**
 * stageDemo answers every route the pictures read with the demo server. The
 * overview and the entries come from their own helpers, the rest is staged
 * here, and the routes staged last answer first.
 */
export async function stageDemo(page: Page, demo: Demo): Promise<void> {
  const lang = demo.lang ?? "en";
  const findings = demo.findings ?? [WARNING];
  const items = demo.items ?? ITEMS;
  const runs = demo.runs ?? RECENT_RUNS;

  await stageOverview(page, undefined, { items, runs, findings, lang });
  await stageEntries(page, { items, containers: CONTAINERS, vms: VMS, fileSets: FILE_SETS, datasets: DATASETS, runs: RUNS, lang });

  const json = (path: string | RegExp, body: unknown) =>
    page.route(
      (url) => (typeof path === "string" ? url.pathname === path : path.test(url.pathname)),
      (route) => (route.request().method() === "GET" ? route.fulfill({ json: body }) : route.fulfill({ json: { ok: true } })),
    );
  const ok = (path: string | RegExp, body: unknown = { ok: true }) =>
    page.route(
      (url) => (typeof path === "string" ? url.pathname === path : path.test(url.pathname)),
      (route) => route.fulfill({ json: body }),
    );

  await json("/api/status", { ok: true, domains: demo.domains ?? DOMAINS });
  await json("/api/coverage", { ok: true, coverage: COVERAGE });
  await json("/api/schedule/next", {
    ok: true,
    runs: [{ job: "backup", domain: "everything", next: iso(demo.nextAt ?? night(-1, 0)) }],
  });
  await page.route(
    (url) => url.pathname === "/api/stats",
    (route) => {
      const domain = new URL(route.request().url()).searchParams.get("domain") ?? "";
      const sizes: Record<string, [number, number, number]> = {
        containers: [61, 113, 238],
        vms: [24, 32, 30],
        flash: [0.6, 1.1, 30],
        files: [392, 431, 30],
        zfs: [88, 96, 30],
        config: [0.03, 0.05, 30],
      };
      const [raw, restore, snapshots] = sizes[domain] ?? [];
      const latest = raw ? { domain, source: "local", at: NOW, rawSize: raw * GB, restoreSize: restore * GB, snapshots } : null;
      return route.fulfill({ json: { ok: true, stats: latest ? [latest] : [], latest, forecast: null } });
    },
  );

  await json("/api/anomalies", { ok: true, anomalies: findings, nextCursor: "" });
  await json("/api/anomalies/summary", { ok: true, summary: summary(findings) });
  await json("/api/anomalies/items", { ok: true, items: watchedItems(findings) });
  await page.route(
    (url) => /^\/api\/anomalies\/items\/[^/]+\/series$/.test(url.pathname),
    (route) => route.fulfill({ json: { ok: true, series: series(decodeURIComponent(new URL(route.request().url()).pathname.split("/")[4])) } }),
  );
  await json(/^\/api\/anomalies\/[^/]+\/changes$/, { ok: true, changes: CHANGES });
  await json("/api/checks/items", {
    ok: true,
    items: ENTRIES.map((e) => ({
      targetId: `tg-${e.key}`,
      domain: e.kind,
      name: e.name,
      probe: { targetId: `tg-${e.key}`, domain: e.kind, at: NOW - 2 * DAY, ok: true, detail: "", snapshotId: "s", files: 1284, bytes: e.bytes },
    })),
  });

  await page.route(
    (url) => url.pathname === "/api/runs",
    (route) => {
      const q = new URL(route.request().url()).searchParams;
      const own = q.has("itemKind") ? (RUNS[`${q.get("itemKind")}:${q.get("itemKey")}`] ?? []) : runs;
      return route.fulfill({ json: { ok: true, runs: own } });
    },
  );
  await page.route(
    (url) => /^\/api\/items\/[^/]+\/[^/]+\/timeline$/.test(url.pathname),
    (route) => {
      const url = new URL(route.request().url());
      const [, , , domain, key] = url.pathname.split("/");
      const kept = timeline(domain, decodeURIComponent(key));
      if (url.searchParams.has("place")) return route.fulfill({ json: { ok: true, place: kept.places[1], rows: [] } });
      return route.fulfill({ json: { ...kept, rows: kept.rows.slice(0, demo.backupsShown) } });
    },
  );
  await page.route(
    (url) => url.pathname === "/api/breakdown",
    (route) => {
      const q = new URL(route.request().url()).searchParams;
      return route.fulfill({ json: { ok: true, breakdown: breakdown(q.get("item") ?? "") } });
    },
  );
  await json(/^\/api\/containers\/[^/]+\/mounts$/, {
    ok: true,
    mounts: [
      { source: "/mnt/user/appdata/plex", destination: "/config", mode: "rw", included: true },
      { source: "/mnt/user/media", destination: "/media", mode: "ro", included: false },
    ],
    custom: [],
    hostMountRoot: "/host/user",
  });

  await page.route("**/api/restore/check", (route) => route.fulfill({ json: restoreCheck(route.request().postDataJSON() ?? {}) }));
  await json(/^\/api\/files\/sets\/[^/]+\/files$/, { ok: true, files: DOCUMENT_FILES });
  await json(/^\/api\/containers\/[^/]+\/files$/, { ok: true, files: DOCUMENT_FILES });

  // The emergency of a lost install, as the wizard asks about it.
  await json("/api/emergency", { ok: true, emergency: { backupKey: "own", undoable: false } });
  await json("/api/emergency/deposits", {
    ok: true,
    deposits: [{ memberId: GROUP_ID.nas, memberName: "Offsite NAS", ownerId: "self", ownerName: "tower", updatedAt: iso(NOW - 6 * HOUR), newest: true }],
    silent: [],
  });

  // Storage locations and where each section goes.
  await json("/api/storage/locations", { ok: true, locations: demo.locations ?? LOCATIONS });
  await page.route(
    (url) => url.pathname.startsWith("/api/storage/own-folders"),
    (route) => route.fulfill({ json: { ok: true, steps: ownFolderSteps((route.request().postDataJSON()?.domains as string[]) ?? []) } }),
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
  await json("/api/offsite/targets", { ok: true, targets: TARGETS });
  await page.route(
    (url) => url.pathname.startsWith("/api/offsite/targets/"),
    (route) => {
      if (new URL(route.request().url()).pathname.endsWith("/move")) {
        return route.fulfill({ json: { ok: true, move: { domain: "vms", targetId: "t-nas-vms", from: `${NAS_REPO}/vms`, to: `${B2_BUCKET}/vms`, backups: 9, pending: 9, key: "move:t-nas-vms" } } });
      }
      return route.fulfill({ json: { ok: true, reachable: true, initialized: true, testable: true, protected: true, warnings: [] } });
    },
  );
  await page.route(
    (url) => url.pathname.startsWith("/api/offsite/destinations"),
    (route) => {
      if (route.request().method() !== "GET") return route.fulfill({ json: { ok: true, warnings: [] } });
      return route.fulfill({ json: { ok: true, destinations: [B2_DESTINATION] } });
    },
  );
  await ok("/api/offsite/providers", { ok: true, providers: PROVIDERS, backends: {} });
  await ok("/api/offsite/group-receivers", {
    ok: true,
    receivers: [{ memberId: GROUP_ID.nas, name: "Offsite NAS", url: "https://offsite-nas.home.arpa:3443", needsAddress: false }],
  });
  await ok("/api/offsite/drafts/check", { ok: true, reachable: true, initialized: false, testable: true, protected: true });
  await json("/api/cloud/creds-sets", { ok: true, sets: CRED_SETS });
  await json("/api/rclone", { ok: true, remotes: [], sections: [] });
  await json("/api/placement/defaults", { ok: true, defaults: SECTIONS.map(defaultRow) });
  await page.route(
    (url) => url.pathname === "/api/placement/options",
    (route) => route.fulfill({ json: { ok: true, options: placementOptions(new URL(route.request().url()).searchParams.get("domain") ?? "") } }),
  );
  await ok(/^\/api\/storage\/sections\/[^/]+\/local$/);

  // Notifications, the group and ZFS.
  await json("/api/notify", { ok: true, notify: NOTIFY });
  await json("/api/group", GROUP);
  await json("/api/group/roles", { ok: true, members: ROLES, self: SELF_ROLES });
  await json("/api/group/deposits", { ok: true, deposits: DEPOSITS });
  await ok("/api/group/deposit/code", { ok: true, recoveryCode: RECOVERY_CODE });
  await json("/api/fleet/peers", { ok: true, peers: PEERS });
  await json("/api/fleet/mesh-offers", { ok: true, offers: [] });
  await json("/api/pull/sources", { ok: true, sources: [] });
  await json("/api/receiver/repos", { ok: true, repos: RECEIVED });
  await json("/api/receiver/server", { ok: true, server: RECEIVER_SERVER, defaultPort: 8000, hostMountRoot: "/host/user" });
  await json("/api/zfs/receive/requests", []);
  await json("/api/zfs/replica/servers", [ZFS_SERVER]);
  await json("/api/zfs/replica/key", { publicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIDemoReplicaKeyOfTheTowerServer bombvault-replica" });
  await json(/^\/api\/zfs\/datasets\/[^/]+\/replica$/, REPLICA);

  // A release, so the pages name a version where the harness build has none.
  await page.route("**/api/health", async (route) => {
    const res = await route.fetch();
    await route.fulfill({ json: { ...(await res.json()), version: "v10.4.2" } });
  });
  await page.route("**/api/settings", async (route: Route) => {
    if (route.request().method() !== "GET") return route.fulfill({ json: { ok: true } });
    const res = await route.fetch();
    const body = await res.json();
    await route.fulfill({ json: { ...body, platform: "unraid", settings: { ...body.settings, ...SETTINGS, ...demo.settings } } });
  });

  await blockServerLook(page);
  await page.addInitScript(
    ([language, theme, stored]) => {
      window.localStorage.setItem("bv-lang", language);
      window.localStorage.setItem("bv-theme", theme);
      window.localStorage.setItem("bv-dbdump-intro-seen", "1");
      for (const [key, value] of Object.entries(stored)) window.localStorage.setItem(key, value);
    },
    [lang, demo.theme, demo.stored ?? {}] as const,
  );
}

/**
 * stageRestores takes the restores of several containers one after the
 * other: the first is done as soon as it starts, the second keeps running.
 */
export async function stageRestores(page: Page): Promise<void> {
  const started: string[] = [];
  await page.route(
    (url) => /^\/api\/containers\/[^/]+\/restore$/.test(url.pathname),
    (route) => {
      started.push(decodeURIComponent(new URL(route.request().url()).pathname.split("/")[3]));
      return route.fulfill({ json: { ok: true, started: true } });
    },
  );
  await page.route(
    (url) => url.pathname === "/api/runs" && !url.searchParams.has("itemKind"),
    (route) =>
      route.fulfill({
        json: {
          ok: true,
          runs: started.map((name, i) => ({
            id: `restore-${name}`,
            targetId: `tg-${name}`,
            kind: "restore",
            status: i === 0 ? "success" : "running",
            startedAt: NOW - 60 + i * 20,
            finishedAt: i === 0 ? NOW - 10 : null,
            snapshotId: "",
            bytes: 0,
            error: "",
            acknowledged: false,
            groupId: "",
            target: name,
            domain: "container",
          })),
        },
      }),
  );
}

/** progress stands a backup in for the server's live progress stream. */
export async function progress(page: Page, frames: Json[]): Promise<void> {
  await page.route("**/api/progress", (route) =>
    route.fulfill({
      status: 200,
      contentType: "text/event-stream",
      body: frames.map((frame) => `data: ${JSON.stringify(frame)}\n\n`).join(""),
    }),
  );
}

