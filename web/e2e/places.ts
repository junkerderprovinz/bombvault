// Storage places and domain rows for the specs that show the Storage tab or
// Recovery, with the longest names and addresses a phone has to fit. A fresh
// harness database has no place, so these are staged at the route layer.
import type { Page } from "@playwright/test";

const NOW = Math.floor(Date.now() / 1000);
const ALL = ["containers", "vms", "flash", "config", "files", "zfs"];
const FOLDERS = { containers: "container", vms: "vms", flash: "flash", config: "config", files: "files", zfs: "zfs" };

const place = (id: string, name: string, provider: string, kind: string, base: string, extra: Record<string, unknown>) => ({
  id,
  name,
  provider,
  kind,
  base,
  folders: FOLDERS,
  offPremises: false,
  storageClass: "",
  immutable: false,
  retentionKeepLast: 7,
  retentionKeepDaily: 14,
  retentionKeepWeekly: 8,
  retentionKeepMonthly: 12,
  limitUpload: 0,
  limitDownload: 0,
  growthBudgetGb: 0,
  enabled: true,
  sortOrder: 0,
  credsRef: "",
  usage: { homeDomains: [], defaults: [], copyDomains: [], items: 0, copies: 0, repositories: 0 },
  locked: {},
  repository: false,
  creds: { shared: false, fields: {}, set: [] },
  ...extra,
});

export const PLACES = [
  place("p1", "Unraid-Array (Freigabe backups)", "unraid-folder", "local", "user/backups/bombvault-tower-im-keller", {
    usage: {
      homeDomains: ["containers", "vms", "flash", "config", "files", "zfs"],
      defaults: ["containers", "vms", "files"],
      copyDomains: [],
      items: 3,
      copies: 0,
      repositories: 6,
    },
    locked: { containers: true, vms: true },
    lastTest: { at: NOW - 3600, ok: true, source: "run" },
  }),
  place("p2", "Wasabi Frankfurt (Langzeitarchiv, nur Schreiben)", "wasabi", "s3", "s3:https://s3.eu-central-1.wasabisys.com/bombvault-offsite-langzeitarchiv-tower", {
    offPremises: true,
    immutable: true,
    storageClass: "STANDARD_IA",
    sortOrder: 1,
    credsRef: "c1",
    usage: { homeDomains: [], defaults: [], copyDomains: ALL, items: 2, copies: 1284, repositories: 6 },
    lastTest: { at: NOW - 7200, ok: false, source: "test", error: "403 Forbidden" },
    creds: { shared: false, fields: { keyId: "BEISPIEL-SCHLUESSEL-ID-FUER-WASABI", region: "eu-central-1" }, set: ["secret"] },
  }),
  place("p3", "rest-server bei den Eltern", "rest-server", "rest", "rest:https://backup.eltern-zuhause.example.net:8000/bombvault", {
    offPremises: true,
    immutable: true,
    sortOrder: 2,
    usage: { homeDomains: [], defaults: [], copyDomains: ["containers"], items: 0, copies: 318, repositories: 1 },
    creds: { shared: false, fields: { user: "bombvault-tower" }, set: ["password"] },
  }),
  place("p4", "Hetzner Storage Box", "storagebox", "sftp", "sftp:u123456@u123456.your-storagebox.de:/home/bombvault", {
    offPremises: true,
    enabled: false,
    sortOrder: 3,
  }),
];

export const UNPLACED = [
  {
    rowId: "t9",
    domain: "vms",
    role: "target",
    name: "Altes NAS im Büro (vor dem Umzug)",
    repo: "sftp:backup@nas-buero.example.internal:/volume1/bombvault/vms-langzeitarchiv",
    immutable: false,
    protectable: true,
    items: 2,
  },
  {
    rowId: "r3",
    domain: "",
    role: "repository",
    name: "Zweitserver bei den Eltern",
    repo: "rest:https://zweitserver.eltern-zuhause.example.net:8000/bombvault-sammelrepository/",
    immutable: false,
    protectable: true,
    items: 0,
  },
];

const chip = (placeId: string, on: boolean, extra: Record<string, unknown> = {}) => ({
  placeId,
  targetId: on ? `${placeId}-target` : undefined,
  on,
  disabled: false,
  ...extra,
});

const row = (domain: string, extra: Record<string, unknown> = {}) => ({
  domain,
  homePlace: "p1",
  storedIn: "p1",
  chips: [chip("p2", true), chip("p3", domain === "containers"), chip("p4", false, { disabled: true, reason: "off" })],
  exceptions: [],
  paused: false,
  schedule: "daily 03:00",
  unreadable: false,
  ...extra,
});

export const DOMAINS = [
  row("containers", {
    paused: true,
    schedule: "everyN 120 03:00",
    exceptions: [
      { identity: "container:nextcloud-aio-mastercontainer", name: "nextcloud-aio-mastercontainer", link: "/containers" },
      { identity: "container:paperless-ngx", name: "paperless-ngx", link: "/containers" },
    ],
  }),
  row("vms", { schedule: "weekly Sun 04:30" }),
  row("flash"),
  row("config"),
  row("files", { schedule: "15 5 */3 * *" }),
  row("zfs", { unreadable: true }),
];

/** The Self-Backup's copies, which Recovery offers to restore the settings from. */
export const CONFIG_TARGETS = [
  {
    id: "p2-target",
    domain: "config",
    name: "Wasabi Frankfurt (Langzeitarchiv, nur Schreiben)",
    repo: "s3:https://s3.eu-central-1.wasabisys.com/bombvault-offsite-langzeitarchiv-tower/config",
    placeId: "p2",
    credsRef: "c1",
    storageClass: "",
    immutable: true,
    schedule: "",
    retentionKeepLast: 7,
    retentionKeepDaily: 14,
    retentionKeepWeekly: 8,
    retentionKeepMonthly: 12,
    limitUpload: 0,
    limitDownload: 0,
    growthBudgetGb: 0,
    enabled: true,
    sortOrder: 1,
    createdAt: NOW - 86400 * 30,
  },
];

/** stagePlaces answers the place and domain-row reads; the catalog comes from
 *  the server, so the add window shows every real provider. */
export async function stagePlaces(page: Page): Promise<void> {
  await page.route("**/api/places", (route) =>
    route.request().method() === "GET"
      ? route.fulfill({ json: { ok: true, places: PLACES, unplaced: UNPLACED } })
      : route.fallback(),
  );
  await page.route("**/api/storage/domains", (route) => route.fulfill({ json: { ok: true, domains: DOMAINS } }));
  await page.route("**/api/offsite/targets*", (route) => {
    const domain = new URL(route.request().url()).searchParams.get("domain");
    return route.fulfill({ json: { ok: true, targets: domain === "config" ? CONFIG_TARGETS : [] } });
  });
}
