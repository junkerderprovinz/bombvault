// Three nodes for the specs of the node pages: one online that backs up to the
// receiving server, one away at a storage location, and one that has not
// answered yet. A node added from the window answers on the second look.
import type { Page, Route } from "@playwright/test";

const NOW = Math.floor(Date.now() / 1000);
const HOUR = 3600;
const DAY = 86400;
const iso = (at: number) => new Date(at * 1000).toISOString();

interface StagedNode {
  id: string;
  name: string;
  createdAt: number;
  adoptedAt: number;
  placement?: string;
  placementTarget?: string;
  online: boolean;
  lastSeen?: number;
  version?: string;
}

export const ROUTES = [
  "GET /api/status",
  "GET /api/progress",
  "GET /api/runs",
  "GET /api/containers",
  "GET /api/containers/{name}",
  "GET /api/containers/{name}/mounts",
  "GET /api/containers/{name}/snapshots",
  "PATCH /api/containers/{name}",
  "POST /api/containers/{name}/backup",
  "POST /api/containers/{name}/restore",
  "GET /api/files",
  "GET /api/files/sets/{id}/snapshots",
  "POST /api/files/sets",
  "PATCH /api/files/sets/{id}",
  "POST /api/files/sets/{id}/backup",
  "POST /api/files/sets/{id}/restore",
  "POST /api/backup/cancel",
];

export const NODES: StagedNode[] = [
  {
    id: "7c1f0e2a9b3d4c5e6f708192a3b4c5d6",
    name: "vps-falkenstein",
    createdAt: NOW - 30 * DAY,
    adoptedAt: NOW - 30 * DAY + 300,
    placement: "receiver",
    placementTarget: "https://backup.tower.example.org",
    online: true,
    version: "v10.5.0",
  },
  {
    id: "2b4d6f8a0c1e3a5b7c9d0e1f2a3b4c5d",
    name: "Hetzner Nürnberg Nextcloud Produktivsystem",
    createdAt: NOW - 12 * DAY,
    adoptedAt: NOW - 12 * DAY + 600,
    placement: "location",
    placementTarget: "Wasabi Frankfurt",
    online: false,
    lastSeen: NOW - 5 * HOUR,
    version: "v10.4.2",
  },
  { id: "9e8d7c6b5a4f3e2d1c0b9a8f7e6d5c4b", name: "pi-garage", createdAt: NOW - HOUR, adoptedAt: 0, online: false },
];

export const NODE = NODES[0];

export const COMPOSE = {
  MODE: "node",
  APP_KEY: "3f9c2b7e8d1a4c6f0b5e9d2a7c4f1e8b6d3a0c9f2e5b8d1a4c7f0e3b6d9a2c5f",
  NODE_HOME: "bvhome1.eyJpZCI6IjVhOWYyYzRlIiwia2V5IjoiTUNvd0JRWURLMlZ3QXlFQSIsInJlbGF5Ijoid3NzOi8vcmVsYXkuZXhhbXBsZS5vcmcifQ",
  NODE_NAME: "",
};

const container = (name: string, image: string, lastBackup: number | null, over: { stack?: string; includeInSchedule?: boolean; self?: boolean } = {}) => ({
  name,
  image,
  state: "running",
  status: "Up 3 days",
  ip: "172.18.0.4",
  installed: true,
  includeInSchedule: true,
  lastBackup,
  stack: "",
  self: false,
  ...over,
});

export const NODE_CONTAINERS = [
  container("nextcloud", "nextcloud:29-apache", NOW - 7 * HOUR, { stack: "cloud" }),
  container("nextcloud-db", "postgres:16", NOW - 7 * HOUR, { stack: "cloud" }),
  container("caddy", "caddy:2", NOW - 31 * HOUR),
  container("uptime-kuma", "louislam/uptime-kuma:1", null, { includeInSchedule: false }),
  container("bombvault-node", "junkerderprovinz/bombvault:latest", null, { self: true }),
];

export const NODE_FILE_SETS = [
  {
    id: "f0e1d2c3b4a5968778695a4b3c2d1e0f",
    name: "Website uploads",
    path: "srv/www/uploads",
    excludes: [],
    enabled: true,
    lastBackup: NOW - 8 * HOUR,
    pathExists: true,
    effectiveSchedule: { kind: "domain", spec: "" },
  },
];

const SNAPSHOTS = Array.from({ length: 5 }, (_, i) => ({
  id: `${(0x5a3c91e0 + i * 977).toString(16)}d2b4f6a8c0e1f3a5b7c9d0e2f4a6b8`,
  time: iso(NOW - (7 + 24 * i) * HOUR),
  paths: ["/host/user/appdata/nextcloud"],
  tags: [],
  hostname: "bombvault-node",
}));

const LOCATIONS = [
  {
    id: "loc-wasabi",
    name: "Wasabi Frankfurt",
    provider: "wasabi",
    repo: "s3:https://s3.eu-central-2.wasabisys.com/tower-offsite",
    credsRef: "cred-main",
    storageClass: "",
    connections: 0,
    immutable: false,
    createdAt: NOW - 90 * DAY,
    domains: ["containers"],
    retention: {},
    compression: "auto",
    limitUpload: 0,
    limitDownload: 0,
    enabled: true,
    offPremises: true,
  },
];

const SIGN_INS = [
  { id: "cred-main", name: "Wasabi tower", s3KeyId: "AKIA1", s3Region: "eu-central-2", restUser: "", s3StorageClass: "", s3SecretSet: true, restPasswordSet: false, default: true },
  { id: "cred-node", name: "Wasabi nodes (no delete)", s3KeyId: "AKIA2", s3Region: "eu-central-2", restUser: "", s3StorageClass: "", s3SecretSet: true, restPasswordSet: false },
];

const RECEIVER = {
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
  logins: [],
};

/** What the staged node calls received, for a spec to check. */
export interface NodeCalls {
  restored: { path: string; body: unknown }[];
  placed: { id: string; body: unknown }[];
  addresses: unknown[];
}

const json = (route: Route, body: unknown, status = 200) => route.fulfill({ status, json: body });

/** stageNodes answers every node route, on top of whatever else the page has staged. */
export async function stageNodes(page: Page, receiverNodeUrl = ""): Promise<NodeCalls> {
  const calls: NodeCalls = { restored: [], placed: [], addresses: [] };
  let added: StagedNode | null = null;
  let looks = 0;

  await page.route(
    (url) => url.pathname === "/api/nodes",
    (route) => {
      if (route.request().method() === "POST") {
        const name = (route.request().postDataJSON() as { name: string }).name;
        added = { id: "4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d", name, createdAt: NOW, adoptedAt: 0, online: false };
        return json(route, { ok: true, node: added, compose: { ...COMPOSE, NODE_NAME: name } });
      }
      const nodes = [...NODES];
      if (added) {
        looks++;
        nodes.push(looks > 1 ? { ...added, adoptedAt: NOW, online: true, version: "v10.5.0" } : added);
      }
      return json(route, { ok: true, nodes });
    },
  );
  await page.route(
    (url) => url.pathname === "/api/nodes/routes",
    (route) => json(route, { ok: true, routes: ROUTES }),
  );
  await page.route(
    (url) => /^\/api\/nodes\/[^/]+\/(compose|placement)$/.test(url.pathname),
    (route) => {
      const [, , , id, what] = new URL(route.request().url()).pathname.split("/");
      if (what === "compose") return json(route, { ok: true, compose: { ...COMPOSE, NODE_NAME: NODES.find((n) => n.id === id)?.name ?? "" } });
      calls.placed.push({ id, body: route.request().postDataJSON() });
      return json(route, { ok: true, reach: [{ domain: "containers", reached: true }, { domain: "files", reached: true }], warning: "" });
    },
  );
  await page.route(
    (url) => /^\/api\/nodes\/[^/]+$/.test(url.pathname) && url.pathname !== "/api/nodes/routes",
    (route) => json(route, { ok: true, told: true }),
  );
  await page.route(
    (url) => /^\/api\/nodes\/[^/]+\/api\//.test(url.pathname),
    (route) => {
      const path = "/api/" + new URL(route.request().url()).pathname.split("/api/")[2];
      const method = route.request().method();
      if (method === "POST" && path.endsWith("/restore")) {
        calls.restored.push({ path, body: route.request().postDataJSON() });
        return json(route, { ok: true, started: true });
      }
      if (method === "POST") return json(route, { ok: true, started: true });
      if (path === "/api/containers") return json(route, { ok: true, containers: NODE_CONTAINERS });
      if (path === "/api/files") return json(route, { ok: true, fileSets: NODE_FILE_SETS });
      if (path.endsWith("/snapshots")) return json(route, { ok: true, snapshots: SNAPSHOTS });
      const name = decodeURIComponent(path.split("/")[3] ?? "");
      const found = NODE_CONTAINERS.find((c) => c.name === name);
      return found ? json(route, { ok: true, container: found, hostAnswered: true }) : json(route, { ok: false, error: "not found" }, 404);
    },
  );
  await page.route(
    (url) => url.pathname === "/api/receiver/node-address",
    (route) => {
      calls.addresses.push(route.request().postDataJSON());
      return json(route, { ok: true, nodeUrl: (route.request().postDataJSON() as { url: string }).url });
    },
  );
  await page.route(
    (url) => url.pathname === "/api/receiver/server",
    (route) => json(route, { ok: true, server: { ...RECEIVER, nodeUrl: receiverNodeUrl, nodeCa: false }, defaultPort: 8000, hostMountRoot: "/host/user" }),
  );
  await page.route(
    (url) => url.pathname === "/api/offsite/destinations",
    (route) => json(route, { ok: true, destinations: LOCATIONS }),
  );
  await page.route(
    (url) => url.pathname === "/api/cloud/creds-sets",
    (route) => json(route, { ok: true, sets: SIGN_INS }),
  );
  return calls;
}
