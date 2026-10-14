// Builds the Play and F-Droid images in fastlane/metadata/android: the phone
// screenshots at 1080x1920, the 1024x500 feature graphic and the 512x512 icon.
//
// The screens are the launcher itself, served from the app's built assets and
// fed example servers through a stand-in for the app's bridge, so every run
// shows the launcher as it is. A server opened in the app is a BombVault
// binary booted on an empty data directory, its answers about entries, runs
// and storage replaced with the example server's. Each screen sits in a drawn
// phone on the dark relief background, under a caption.
//
// Every page is rendered at twice the size and scaled down in a second page,
// which keeps the text sharp through the phone's tilt.
//
// `node android/store/render.mjs readme` builds the README's pictures instead:
// .github/assets/screenshots/android.png at 1920x1000, as wide as the other
// screenshots, and the call for Android testers, testers.png at 1920x640.
//
// Deps (global): playwright-core with its Chromium installed.
// Run: (cd web && npm run build && npm run build:launcher) && go build ./cmd/bombvault && node android/store/render.mjs [readme]

import { execSync, spawn } from "node:child_process";
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { createServer } from "node:http";
import { createServer as createNetServer } from "node:net";
import { createRequire } from "node:module";
import { tmpdir } from "node:os";
import { dirname, extname, join, normalize } from "node:path";
import { fileURLToPath } from "node:url";

const require = createRequire(import.meta.url);
const { chromium } = require(`${execSync("npm root -g").toString().trim()}/playwright-core`);

const here = dirname(fileURLToPath(import.meta.url));
const root = dirname(dirname(here));
const site = join(root, "android", "app", "src", "main", "assets", "launcher");
const metadata = join(root, "fastlane", "metadata", "android");

const CAPTIONS = {
  "de-DE": {
    lang: "de",
    sub: "Die App zu deinen BombVault-Servern",
    tagline: "Deine Backup-Server im Blick,<br>direkt vom Handy aus.",
    list: "Jedes Backup live, <em>auch vom Sofa aus</em>",
    activity: "Alle Server reden. <em>Du liest mit.</em>",
    pairing: "Zwölf Wörter rein, <em>alle Server da</em>",
    server: "Jeder Server neu gestaltet. <em>Ein Tipp bringt dich zurück.</em>",
    backups: "Alle Backups <em>in einer Liste</em>",
    servers: ["Home-NAS", "Offsite", "Büro"],
    photos: "Fotos",
    documents: "Dokumente",
  },
  "en-US": {
    lang: "en",
    sub: "The app for your BombVault servers",
    tagline: "Your backup servers at a glance,<br>right from your phone.",
    list: "Every backup live, <em>even from the couch</em>",
    activity: "Your servers talk. <em>You listen in.</em>",
    pairing: "Type twelve words. <em>Every server shows up.</em>",
    server: "Each server, redesigned. <em>One tap takes you back.</em>",
    backups: "All your backups <em>in one list</em>",
    servers: ["Home NAS", "Offsite", "Office"],
    photos: "Photos",
    documents: "Documents",
  },
};

// The order on the store page. `lift` names an element of the screen drawn
// floating in front of the phone, `page` a page of the first server's own
// interface. The activity log is drawn only for the feature graphic.
const SHOTS = [{ name: "list", lift: "section" }, { name: "server", page: "/dashboard" }, { name: "pairing" }, { name: "backups", page: "/backups" }];
const ACTIVITY = { name: "activity" };

// The release the example group and the app run.
const VERSION = "10.0.0";

// The clock in the status bar, and the time the example runs are counted from.
const NOW = new Date("2026-10-02T21:45:00");

// The phone's screen in CSS pixels, as wide as a common phone, and the status
// and gesture bars the screen draws above and below the app.
const SCREEN = { w: 405, h: 720, status: 24, nav: 16 };
const DPR = 1080 / SCREEN.w;
const STATUS_H = SCREEN.status * DPR;

async function cached(file, url) {
  const path = join(tmpdir(), `BombVault-${file}`);
  if (!existsSync(path)) {
    const res = await fetch(url);
    if (!res.ok) throw new Error(`${file}: fetch ${res.status}`);
    writeFileSync(path, Buffer.from(await res.arrayBuffer()));
  }
  return readFileSync(path);
}

const dataUrl = (buf, type) => `data:${type};base64,${buf.toString("base64")}`;
const bree = dataUrl(
  await cached("BreeSerif-Regular.ttf", "https://github.com/google/fonts/raw/main/ofl/breeserif/BreeSerif-Regular.ttf"),
  "font/ttf",
);
const lato = dataUrl(await cached("Lato-Regular.ttf", "https://github.com/google/fonts/raw/main/ofl/lato/Lato-Regular.ttf"), "font/ttf");
const roboto = dataUrl(
  await cached("Roboto.ttf", "https://github.com/google/fonts/raw/main/ofl/roboto/Roboto%5Bwdth%2Cwght%5D.ttf"),
  "font/ttf",
);

// The app icon, and the vault alone without its tile for the caption and the relief.
const iconSvg = readFileSync(join(here, "icon.svg"), "utf8");
const vault = dataUrl(Buffer.from(iconSvg.replace(/<g id="tile">[\s\S]*?<\/g>/, "")), "image/svg+xml");

/** The example group: three servers, what ran on each, and what runs now. */
function group(text) {
  const now = Math.floor(NOW.getTime() / 1000);
  const [nas, offsite, office] = text.servers;
  const servers = [
    { id: "s1", name: nas, url: "https://192.168.1.20:3443/", member: true },
    { id: "s2", name: offsite, url: "https://10.8.0.2:3443/", member: true },
    { id: "s3", name: office, url: "", member: true },
  ];
  const run = (id, target, domain, startedAgo, took, mb = 0) => ({
    id,
    targetId: target,
    kind: "backup",
    status: took === null ? "running" : "success",
    startedAt: now - startedAgo,
    finishedAt: took === null ? null : now - startedAgo + took,
    snapshotId: "",
    bytes: mb * 1e6,
    error: "",
    acknowledged: false,
    target,
    domain,
  });
  const next = (h, job = "backup") => [{ job, domain: "", next: new Date((now + h * 3600) * 1000).toISOString() }];
  const activity = {
    s1: {
      runs: [
        run("a1", "nextcloud", "container", 95, null),
        run("a2", "immich", "container", 1500, 240, 1840),
        run("a3", "paperless", "container", 1900, 70, 96),
      ],
      progress: [{ key: "container:nextcloud", phase: "backup", percent: 42, active: true, startedAt: now - 95 }],
      next: next(3),
    },
    s2: {
      runs: [run("b1", "homeassistant", "vm", 3400, 410, 2310), run("b2", text.photos, "files", 2300, 380, 5120)],
      progress: [],
      next: next(5, "offsite"),
    },
    s3: { runs: [run("c1", text.documents, "files", 1200, 55, 212)], progress: [], next: next(2) },
  };
  const members = servers.map((s) => ({ id: s.id, name: s.name, version: `v${VERSION}` }));
  return { servers, activity, members };
}

/** The page's stand-in for the object the app injects as `bombvaultApp`. */
function fakeApp({ state, activity, look }) {
  const listeners = new Set();
  const post = (msg) => {
    const data = JSON.stringify(msg);
    for (const fn of listeners) fn({ data });
  };
  window.bombvaultApp = {
    postMessage(raw) {
      const req = JSON.parse(raw);
      setTimeout(() => {
        if (req.op === "state") post({ op: "state", ...state });
        else if (req.op === "activity")
          post({ op: "activity", ticket: req.ticket, status: 200, body: JSON.stringify({ ok: true, ...activity[req.id] }) });
        else if (req.op === "look")
          post({ op: "look", ticket: req.ticket, status: 200, body: JSON.stringify({ ok: true, prefs: look, stored: true }) });
      }, 0);
    },
    addEventListener: (_, fn) => listeners.add(fn),
    removeEventListener: (_, fn) => listeners.delete(fn),
  };
}

/** Serves the built launcher the way the app's asset loader does. */
function serve() {
  const types = {
    ".html": "text/html",
    ".js": "text/javascript",
    ".css": "text/css",
    ".svg": "image/svg+xml",
    ".png": "image/png",
    ".json": "application/json",
    ".woff2": "font/woff2",
    ".txt": "text/plain",
  };
  const server = createServer((req, res) => {
    const path = new URL(req.url, "http://x").pathname;
    const file = join(site, normalize(path === "/" ? "launcher.html" : decodeURIComponent(path)));
    if (!file.startsWith(site) || !existsSync(file)) {
      res.writeHead(404).end();
      return;
    }
    res.writeHead(200, { "content-type": types[extname(file)] ?? "application/octet-stream" }).end(readFileSync(file));
  });
  return new Promise((resolve) => server.listen(0, "127.0.0.1", () => resolve(server)));
}

const STATUS_ICONS = `<svg viewBox="0 0 42 14" width="42" height="14" fill="#e8e8e8">
  <path d="M7 13 0 4.4a11 11 0 0 1 14 0Z"/>
  <path d="M17 13h11V2Z"/>
  <rect x="33" y="2" width="8" height="11" rx="1.4"/><rect x="35.5" y=".8" width="3" height="1.8" rx=".5"/>
</svg>`;

/** A port that was free a moment ago. */
function freePort() {
  return new Promise((resolve) => {
    const probe = createNetServer().listen(0, "127.0.0.1", () => {
      const { port } = probe.address();
      probe.close(() => resolve(port));
    });
  });
}

/** Starts the BombVault binary at the repo root on an empty data directory. */
async function boot() {
  const binary = join(root, process.platform === "win32" ? "bombvault.exe" : "bombvault");
  if (!existsSync(binary)) throw new Error(`${binary} is missing: go build ./cmd/bombvault`);
  const data = mkdtempSync(join(tmpdir(), "BombVault-store-"));
  const port = await freePort();
  const home = join(data, "home");
  mkdirSync(home);
  const child = spawn(binary, [], {
    env: {
      ...process.env,
      APP_KEY: "0".repeat(64),
      DATA_DIR: data,
      BIND_HOST: "127.0.0.1",
      HOME: home,
      USERPROFILE: home,
      HTTP_ONLY: "true",
      PORT: String(port),
    },
    stdio: "ignore",
  });
  const url = `http://127.0.0.1:${port}`;
  for (let i = 0; ; i++) {
    if (await fetch(`${url}/api/health`).then((res) => res.ok, () => false)) break;
    if (i > 150) throw new Error("bombvault did not start");
    await new Promise((r) => setTimeout(r, 200));
  }
  return {
    url,
    stop() {
      child.kill();
      // Windows holds the database open until the process is gone.
      setTimeout(() => rmSync(data, { recursive: true, force: true, maxRetries: 5 }), 500);
    },
  };
}

/** What the first server of the group has backed up, and where it keeps it. */
function homeServer(text) {
  const now = Math.floor(NOW.getTime() / 1000);
  const H = 3600;
  const D = 86400;
  const GB = 1024 ** 3;
  const TB = 1024 * GB;
  const run = (target, domain, ago, took, gb) => ({
    id: `run-${target}`,
    targetId: target,
    kind: "backup",
    status: "success",
    startedAt: now - ago,
    finishedAt: now - ago + took,
    snapshotId: "5f3a9c1e7b2d4a60",
    bytes: gb * GB,
    error: "",
    acknowledged: false,
    groupId: "",
    target,
    domain,
  });
  const runs = [
    run("immich", "container", 25 * 60, 240, 1.8),
    run("paperless", "container", 32 * 60, 70, 0.1),
    run("nextcloud", "container", 18 * H, 95, 0.4),
    run("Home Assistant", "vm", 19 * H, 410, 2.3),
    run(text.photos, "files", 20 * H, 380, 5.1),
    run("flash", "flash", 21 * H, 20, 0.2),
  ];
  const item = (kind, key, ago, extra = {}) => ({
    kind,
    key,
    name: key,
    included: true,
    paused: false,
    effectiveSchedule: { kind: "domain", spec: "daily 03:00" },
    lastBackup: now - ago,
    lastRunStatus: "success",
    sourceBytes: 4 * GB,
    runs: [{ id: `run-${key}`, kind: "backup", status: "success", startedAt: now - ago }],
    ...extra,
  });
  const app = { state: "running", installed: true };
  const items = [
    item("container", "immich", 25 * 60, app),
    item("container", "paperless", 32 * 60, app),
    item("container", "nextcloud", 18 * H, app),
    item("vm", "Home Assistant", 19 * H, { state: "running" }),
    item("files", text.photos, 20 * H),
    item("flash", "flash", 21 * H),
  ];
  const domain = (name, ago) => ({
    domain: name,
    enabled: true,
    schedule: "daily 03:00",
    coveredBy: "",
    lastSuccess: now - ago,
    periodSeconds: D,
    status: "ok",
    lastVerified: now - D,
    lastVerifiedOK: true,
    verifiedDetail: "",
    drillDetail: "",
    offsiteConfigured: true,
    offPremisesCovered: true,
    offsiteImmutable: true,
    lastTamperAt: now - 3 * D,
    lastTamperOK: true,
    lastReplicationAt: now - 3 * H,
    lastReplicationOK: true,
    lastDrDrillAt: now - 5 * D,
    lastDrDrillOK: true,
    lastOffsiteSubsetAt: now - 2 * D,
    lastOffsiteSubsetOK: true,
    offsiteDrillScheduled: true,
    protection: "green",
    tamperState: "ok",
    replicationState: "ok",
    drillState: "ok",
    encryptionOn: true,
    pruneStrategySet: true,
  });
  const keep = { keepLast: 0, keepDaily: 7, keepWeekly: 4, keepMonthly: 6, keepYearly: 1 };
  const section = (name, where, use, lastCopy) => ({
    domain: name,
    use,
    where,
    enabled: true,
    immutable: false,
    retention: keep,
    compression: "auto",
    limitUpload: 0,
    limitDownload: 0,
    own: [],
    ...(lastCopy ? { lastCopy: { at: now - lastCopy, ok: true } } : {}),
  });
  const kinds = ["containers", "vms", "flash", "files"];
  const local = {
    id: "path:local",
    object: "path",
    kind: "local",
    provider: "",
    backend: "local",
    name: "bombvault",
    where: "/mnt/user/backups/bombvault",
    enabled: true,
    offPremises: false,
    sections: kinds.map((k) => section(k, `/mnt/user/backups/bombvault/${k}`, "home")),
    retention: keep,
    protection: { immutable: false, testable: false },
    capacity: { at: now, usedBytes: 412 * GB, freeBytes: 3.1 * TB, totalBytes: 3.5 * TB, source: "statfs", storedBytes: 380 * GB, growthBytesPerWeek: 9 * GB, weeksToFull: 300 },
  };
  const box = {
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
    sections: kinds.map((k) => section(k, `rclone:storagebox:bombvault/${k}`, "copy", 3 * H)),
    retention: keep,
    compression: "auto",
    limitUpload: 0,
    limitDownload: 0,
    protection: { immutable: true, testable: false },
    capacity: { at: now, usedBytes: 268 * GB, freeBytes: 756 * GB, totalBytes: TB, source: "sftp", storedBytes: 268 * GB, growthBytesPerWeek: 6 * GB, weeksToFull: 126 },
  };
  return {
    runs,
    items,
    domains: kinds.map((k, i) => domain(k, [25 * 60, 19 * H, 21 * H, 20 * H][i])),
    locations: [local, box],
    // Repository size and restored size in GB, and the snapshot count, per kind.
    stats: { containers: [38, 92, 168], vms: [61, 74, 28], flash: [0.4, 0.9, 30], files: [118, 121, 30] },
    next: [{ job: "backup", domain: "containers", next: new Date((now + 5 * H + 15 * 60) * 1000).toISOString() }],
  };
}

/** Answers the routes the server's pages read with `homeServer`. */
async function stageServer(page, text) {
  const s = homeServer(text);
  const json = (body) => (route) => route.fulfill({ json: { ok: true, ...body } });
  const at = (path) => (url) => url.pathname === path;
  // The look is the one set in the browser, not the empty server's.
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.route("**/api/settings", async (route) => {
    if (route.request().method() !== "GET") return route.continue();
    const body = await (await route.fetch()).json();
    const settings = { ...body.settings, anomalyEnabled: true, encryptionEnabled: true, recoveryKitAck: true, instanceName: text.servers[0] };
    await route.fulfill({ json: { ...body, settings } });
  });
  await page.route(at("/api/status"), json({ domains: s.domains }));
  await page.route(at("/api/coverage"), json({ coverage: { total: s.items.length, protected: s.items.length, domains: [] } }));
  await page.route(at("/api/items"), json({ items: s.items, unlisted: [] }));
  await page.route(at("/api/runs"), json({ runs: s.runs }));
  await page.route(at("/api/schedule/next"), json({ runs: s.next }));
  await page.route(at("/api/storage/locations"), json({ locations: s.locations }));
  await page.route(at("/api/stats"), (route) => {
    const domain = new URL(route.request().url()).searchParams.get("domain");
    const [raw, restore, snapshots] = s.stats[domain] ?? [];
    const latest = raw ? { domain, source: "local", at: NOW.getTime() / 1000, rawSize: raw * 1024 ** 3, restoreSize: restore * 1024 ** 3, snapshots } : null;
    return route.fulfill({ json: { ok: true, stats: latest ? [latest] : [], latest, forecast: null } });
  });
  const quiet = { critical: 0, warning: 0, info: 0 };
  const backfill = { slots: 0, done: 0, failed: 0, filled: 0, withoutSummary: 0 };
  const summary = { enabled: true, ready: true, generation: 7, open: quiet, recoveredCritical: 0, learningItems: 0, retentionHeld: 0 };
  await page.route(at("/api/anomalies/summary"), json({ summary: { ...summary, evalErrors: 0, notifyMuted: false, backfill, unmeasuredVolumes: [] } }));
  await page.route(at("/api/anomalies/items"), json({ items: [] }));
  await page.route(at("/api/anomalies"), json({ anomalies: [], nextCursor: "" }));
}

/** Takes one screen of the app as the phone shows it, 1080x1920. */
async function capture(browser, urls, text, shot) {
  const { servers, activity, members } = group(text);
  const paired = shot.name !== "pairing";
  const state = {
    servers: paired ? servers : [],
    found: [],
    group: paired ? { paired: true, connected: true } : { paired: false, connected: true, joining: { members, connected: true } },
    camera: true,
    app: { version: VERSION, versionCode: 100000, android: "36", model: "Pixel 8", deviceName: "" },
  };
  const look = { "bv-theme": "dark", "bv-accent": "#FCC419" };

  const page = await browser.newPage({
    viewport: { width: SCREEN.w, height: SCREEN.h - SCREEN.status - SCREEN.nav },
    deviceScaleFactor: DPR,
    colorScheme: "dark",
    hasTouch: true,
    isMobile: true,
  });
  await page.clock.setFixedTime(NOW);
  await page.addInitScript(({ lang }) => {
    localStorage.setItem("bv-lang", lang);
    localStorage.setItem("bv-theme", "dark");
  }, text);
  await page.addInitScript(fakeApp, { state, activity, look });
  if (shot.page) {
    await stageServer(page, text);
    await page.goto(`${urls.server}${shot.page}`);
    await page.waitForSelector("#bv-main [data-testid], #bv-main section");
  } else {
    await page.goto(`${urls.launcher}/launcher.html`);
    await page.waitForSelector("main");
  }
  await page.evaluate(() => document.fonts.ready);

  const header = page.locator("header button");
  if (shot.name === "activity") await page.locator("section [role=button]").click();
  if (shot.name === "pairing") await header.nth(0).click();
  await page.waitForTimeout(shot.page ? 1500 : 600);

  // The card's label sits above its edge, so the lifted part spans everything
  // drawn in it but the lines scrolled out of its log.
  let lift;
  if (shot.lift) {
    const box = await page
      .locator(shot.lift)
      .first()
      .evaluate((el) => {
        const shown = [el, ...el.querySelectorAll("*")].filter((e) => !e.parentElement.closest(".overflow-y-auto"));
        const rects = shown.map((e) => e.getBoundingClientRect());
        const top = Math.min(...rects.map((r) => r.top));
        const r = el.getBoundingClientRect();
        return { x: r.left, y: top, width: r.width, height: r.bottom - top };
      });
    lift = { x: box.x * DPR, y: box.y * DPR + STATUS_H, w: box.width * DPR, h: box.height * DPR };
  }
  const bg = await page.evaluate(() => getComputedStyle(document.body).backgroundColor);
  const app = await page.screenshot();
  await page.close();

  const frame = await browser.newPage({ viewport: { width: SCREEN.w, height: SCREEN.h }, deviceScaleFactor: DPR });
  await frame.setContent(`<!doctype html><html><head><style>
@font-face { font-family: Roboto; src: url(${roboto}); font-weight: 100 900; }
* { margin: 0; } body { width: ${SCREEN.w}px; height: ${SCREEN.h}px; background: ${bg}; position: relative; overflow: hidden; }
.status { position: absolute; left: 0; right: 0; top: 0; height: ${SCREEN.status}px; display: flex; align-items: center; justify-content: space-between; padding: 0 14px 0 16px;
  font: 500 13px/1 Roboto, sans-serif; color: #e8e8e8; }
.app { position: absolute; left: 0; top: ${SCREEN.status}px; width: ${SCREEN.w}px; }
.pill { position: absolute; left: 50%; bottom: 6px; width: 108px; height: 4px; margin-left: -54px; border-radius: 2px; background: #e8e8e8; opacity: .55; }
</style></head><body>
<div class="status"><span>${NOW.toTimeString().slice(0, 5)}</span>${STATUS_ICONS}</div>
<img class="app" src="${dataUrl(app, "image/png")}">
<div class="pill"></div>
</body></html>`);
  await frame.locator("img").evaluate((img) => img.decode());
  const png = await frame.screenshot();
  await frame.close();
  return { png: dataUrl(png, "image/png"), lift, bg };
}

const STYLE = `
@font-face { font-family: "Bree Serif"; src: url(${bree}); }
@font-face { font-family: Lato; src: url(${lato}); }
* { box-sizing: border-box; margin: 0; }
body { position: relative; overflow: hidden; font-family: Lato, sans-serif; background: #0c0c0b; }
.backdrop, .backdrop * { position: absolute; }
.backdrop { inset: 0; overflow: hidden; }
.wall { inset: 0; background: radial-gradient(55% 60% at 62% 35%, #26231d, #0f0e0c 72%); }
.mark { transform: rotate(-10deg); opacity: .32;
  filter: grayscale(1) brightness(.36) contrast(1.2) drop-shadow(-2px -2px 0 rgba(255,255,255,.16)) drop-shadow(12px 18px 26px rgba(0,0,0,.85)); }
.vignette { inset: 0; box-shadow: inset 0 0 200px rgba(0,0,0,.6); }
h1 { font-family: "Bree Serif", serif; font-weight: 400; color: #f4f4f4; text-wrap: balance; }
h1 em { font-style: normal; color: #FCC419; }
.sub { color: #9d9481; }

.stage { position: absolute; perspective: 2400px; }
.floor { position: absolute; left: 12%; right: 12%; bottom: -4%; height: 8%; border-radius: 50%; background: rgba(0,0,0,.8); filter: blur(30px); }
.phone { position: absolute; inset: 0; transform-style: preserve-3d; transform: rotateY(-7deg) rotateX(3deg);
  background: linear-gradient(135deg, #9a9a9a 0%, #4a4a4a 14%, #2c2c2c 45%, #3a3a3a 70%, #6e6e6e 100%);
  box-shadow: -1px 1px 0 #262626, -2px 2px 0 #222, -3px 3px 0 #1e1e1e, -4px 4px 0 #1a1a1a, -5px 5px 0 #161616,
    0 2px 4px rgba(0,0,0,.35), 0 18px 36px rgba(0,0,0,.45), 0 52px 100px rgba(0,0,0,.55); }
.key { position: absolute; right: -4px; width: 5px; border-radius: 0 3px 3px 0; background: linear-gradient(90deg, #2a2a2a, #6a6a6a); }
.glass { position: absolute; background: #050505; box-shadow: inset 0 0 0 1px rgba(255,255,255,.07); }
.screen { position: absolute; overflow: hidden; background: #161616; }
.screen > img { position: absolute; left: 0; top: 0; width: 100%; }
.status { position: absolute; left: 0; right: 0; top: 0; }
.status i { position: absolute; top: 0; height: 100%; background-repeat: no-repeat; }
.cam { position: absolute; left: 50%; border-radius: 50%; background: radial-gradient(circle at 35% 35%, #2d3440, #050505 60%); box-shadow: 0 0 0 2px #0b0b0b; }
.glare { position: absolute; inset: 0; pointer-events: none;
  background: linear-gradient(118deg, rgba(255,255,255,.09) 0%, rgba(255,255,255,.03) 28%, rgba(255,255,255,0) 42%); }
.lift { position: absolute; overflow: hidden; transform: translateZ(70px); background-repeat: no-repeat;
  box-shadow: 0 0 0 1px #474747, 0 4px 8px rgba(0,0,0,.35), 0 22px 44px rgba(0,0,0,.5), 0 50px 90px rgba(0,0,0,.45); }
`;

const backdrop = (w, left, top) =>
  `<div class="backdrop"><div class="wall"></div><img class="mark" src="${vault}" style="width:${w};left:${left};top:${top}"><div class="vignette"></div></div>`;

/** A phone showing `capture` with its screen `sw` wide, its top left corner at (x, y). */
function phone({ png: capture, bg: fill }, { x, y, sw, lift }) {
  const k = sw / 1080;
  const sh = Math.round(1920 * k);
  const rim = Math.max(2, Math.round(sw * 0.01));
  const bezel = Math.round(sw * 0.024);
  const r = Math.round(sw * 0.1);
  const w = sw + 2 * (rim + bezel);
  const h = sh + 2 * (rim + bezel);
  const bar = STATUS_H * k;
  // The clock and the icons move inwards by this much, clear of the corners.
  const inset = Math.round(sw * 0.045);
  const bg = `url(${capture})`;
  const size = `${sw}px ${sh}px`;
  const cam = Math.round(sw * 0.028);

  // The lifted card is drawn a tenth larger and reaches past the phone's left edge.
  const z = k * 1.1;
  const floating = lift
    ? `<div class="lift" style="left:${rim + bezel + lift.x * k - lift.w * k * 0.09}px;top:${rim + bezel + lift.y * k}px;width:${lift.w * z}px;height:${lift.h * z}px;border-radius:${30 * z}px;
        background-image:${bg};background-size:${1080 * z}px ${1920 * z}px;background-position:${-lift.x * z}px ${-lift.y * z}px"></div>`
    : "";

  return `<div class="stage" style="left:${x}px;top:${y}px;width:${w}px;height:${h}px">
  <div class="floor"></div>
  <div class="phone" style="border-radius:${r + rim + bezel}px">
    <i class="key" style="top:${h * 0.18}px;height:${h * 0.07}px"></i>
    <i class="key" style="top:${h * 0.28}px;height:${h * 0.12}px"></i>
    <div class="glass" style="inset:${rim}px;border-radius:${r + bezel}px">
      <div class="screen" style="inset:${bezel}px;border-radius:${r}px">
        <img src="${capture}">
        <div class="status" style="height:${bar}px;background:${fill}">
          <i style="left:${inset}px;width:${sw * 0.2}px;background-image:${bg};background-size:${size};background-position:0 0"></i>
          <i style="right:${inset}px;width:${sw * 0.2}px;background-image:${bg};background-size:${size};background-position:${-sw * 0.8}px 0"></i>
        </div>
        <div class="cam" style="top:${(bar - cam) / 2}px;width:${cam}px;height:${cam}px;margin-left:${-cam / 2}px"></div>
        <div class="glare"></div>
      </div>
    </div>
    ${floating}
  </div>
</div>`;
}

function screenshot(shot, caption, sub) {
  return `<!doctype html><html><head><meta charset="utf-8"><style>${STYLE}
body { width: 1080px; height: 1920px; }
.copy { position: absolute; left: 88px; right: 88px; top: 110px; display: flex; flex-direction: column; gap: 30px; }
.copy img { width: 92px; }
h1 { font-size: 78px; line-height: 1.12; }
.sub { font-size: 34px; }
</style></head><body>
${backdrop("78%", "-16%", "-5%")}
<div class="copy"><img src="${vault}"><h1>${caption}</h1><p class="sub">${sub}</p></div>
${phone(shot, { x: shot.lift ? 232 : 196, y: 612, sw: 640, lift: shot.lift })}
</body></html>`;
}

function featureGraphic(back, front, tagline) {
  return `<!doctype html><html><head><meta charset="utf-8"><style>${STYLE}
body { width: 1024px; height: 500px; }
.copy { position: absolute; left: 70px; top: 0; bottom: 0; width: 470px; display: flex; flex-direction: column; justify-content: center; gap: 18px; }
.copy img { width: 84px; }
.name { font: 400 66px/1 "Bree Serif", serif; color: #f4f4f4; }
.sub { font-size: 25px; line-height: 1.35; }
</style></head><body>
${backdrop("46%", "-9%", "-4%")}
<div class="copy"><img src="${vault}"><div class="name">BombVault</div><p class="sub">${tagline}</p></div>
${phone(back, { x: 590, y: 76, sw: 196 })}
${phone(front, { x: 758, y: 40, sw: 220 })}
</body></html>`;
}

/** The README's one picture of the app, three phones as wide as the screenshots above it. */
function androidShot(left, middle, right, text) {
  return `<!doctype html><html><head><meta charset="utf-8"><style>${STYLE}
body { width: 1920px; height: 1000px; }
.copy { position: absolute; left: 84px; top: 0; bottom: 0; width: 520px; display: flex; flex-direction: column; justify-content: center; gap: 28px; }
.copy img { width: 92px; }
h1 { font-size: 64px; line-height: 1.12; }
.sub { font-size: 28px; line-height: 1.35; }
</style></head><body>
${backdrop("50%", "-9%", "-6%")}
<div class="copy"><img src="${vault}"><h1>${text.list}</h1><p class="sub">${text.tagline}</p></div>
${phone(left, { x: 660, y: 230, sw: 330 })}
${phone(right, { x: 1500, y: 230, sw: 330 })}
${phone(middle, { x: 1060, y: 150, sw: 380, lift: middle.lift })}
</body></html>`;
}

/** The README's call for Android testers, in English like the README. */
function testersShot(back, front) {
  return `<!doctype html><html><head><meta charset="utf-8"><style>${STYLE}
body { width: 1920px; height: 640px; }
.copy { position: absolute; left: 96px; top: 0; bottom: 0; width: 1060px; display: flex; flex-direction: column; justify-content: center; gap: 26px; }
.head { display: flex; align-items: center; gap: 22px; }
.head img { width: 64px; }
.label { padding: 8px 18px; border-radius: 8px; background: #FCC419; color: #141414; font: 700 22px/1 Lato, sans-serif; letter-spacing: .12em; text-transform: uppercase; }
h1 { font-size: 92px; line-height: 1.05; }
.sub { font-size: 34px; line-height: 1.35; }
.go { align-self: flex-start; margin-top: 8px; padding: 22px 40px; border-radius: 18px; background: #FCC419; color: #141414; font: 700 34px/1 Lato, sans-serif;
  box-shadow: 0 0 0 6px rgba(252,196,25,.18), 0 18px 40px rgba(0,0,0,.5); }
</style></head><body>
${backdrop("40%", "58%", "-30%")}
<div class="copy">
  <div class="head"><img src="${vault}"><span class="label">Google Play closed test</span></div>
  <h1>Android testers <em>wanted</em></h1>
  <p class="sub">Keep BombVault installed for 14 days and help it go public.</p>
  <span class="go">Become a tester &rarr;</span>
</div>
${phone(back, { x: 1290, y: 92, sw: 250 })}
${phone(front, { x: 1530, y: 58, sw: 282 })}
</body></html>`;
}

/** Play masks the icon itself, so the tile goes in square and without its rounded corners. */
function storeIcon() {
  const square = iconSvg.replace(/ rx="68" ry="68"/, "").replace(/(<path class="cls-2" d=")[^"]*"/, '$1M500,0H1000V1000H500Z"');
  return `<!doctype html><html><body style="margin:0"><img style="display:block;width:512px;height:512px" src="${dataUrl(Buffer.from(square), "image/svg+xml")}"></body></html>`;
}

/** Renders `html` at twice `width` x `height` and writes it scaled down to `file`. */
async function render(browser, html, width, height, file) {
  const page = await browser.newPage({ viewport: { width, height }, deviceScaleFactor: 2 });
  await page.setContent(html);
  await page.evaluate(() => document.fonts.ready);
  await page.evaluate(() => Promise.all([...document.images].map((img) => img.decode())));
  const big = await page.screenshot();
  await page.close();

  const small = await browser.newPage({ viewport: { width, height } });
  await small.setContent(
    `<body style="margin:0"><img src="${dataUrl(big, "image/png")}" style="display:block;width:${width}px;height:${height}px">`,
  );
  await small.locator("img").evaluate((img) => img.decode());
  await small.screenshot({ path: file });
  await small.close();
}

async function readme(browser, urls) {
  const text = CAPTIONS["en-US"];
  const shot = (name) => capture(browser, urls, text, [...SHOTS, ACTIVITY].find((s) => s.name === name));
  const out = join(root, ".github", "assets", "screenshots");
  const [list, activity, pairing] = [await shot("list"), await shot("activity"), await shot("pairing")];
  await render(browser, androidShot(activity, list, pairing, text), 1920, 1000, join(out, "android.png"));
  console.log("wrote .github/assets/screenshots/android.png");
  await render(browser, testersShot(list, activity), 1920, 640, join(out, "testers.png"));
  console.log("wrote .github/assets/screenshots/testers.png");
}

async function store(browser, urls) {
  const bombvault = await boot();
  try {
    for (const [locale, text] of Object.entries(CAPTIONS)) {
      const images = join(metadata, locale, "images");
      mkdirSync(join(images, "phoneScreenshots"), { recursive: true });
      const shots = {};
      for (const [i, shot] of SHOTS.entries()) {
        shots[shot.name] = await capture(browser, { ...urls, server: bombvault.url }, text, shot);
        await render(
          browser,
          screenshot(shots[shot.name], text[shot.name], text.sub),
          1080,
          1920,
          join(images, "phoneScreenshots", `${i + 1}.png`),
        );
        console.log(`wrote ${locale}/images/phoneScreenshots/${i + 1}.png`);
      }
      const activity = await capture(browser, urls, text, ACTIVITY);
      await render(browser, featureGraphic(activity, shots.list, text.tagline), 1024, 500, join(images, "featureGraphic.png"));
      console.log(`wrote ${locale}/images/featureGraphic.png`);
      await render(browser, storeIcon(), 512, 512, join(images, "icon.png"));
      console.log(`wrote ${locale}/images/icon.png`);
    }
  } finally {
    bombvault.stop();
  }
}

const server = await serve();
const launcher = `http://127.0.0.1:${server.address().port}`;
const browser = await chromium.launch();
try {
  await (process.argv[2] === "readme" ? readme : store)(browser, { launcher });
} finally {
  await browser.close();
  server.close();
}
