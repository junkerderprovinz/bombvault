// Settings at phone width, with a filled instance staged at the route layer: a
// fresh harness database has every domain off and nothing configured, which
// hides most of the page. On the phones, per page: the 24px card rhythm, the
// rail of pages as glyphs beside the content, nothing panning or reaching past
// the viewport or its card, no stray backtick, and every control big enough to
// hit. Then the arrangements that change on a phone: target and credential
// rows put their actions under the name, repository and passkey rows wrap
// instead of cutting, the every-N-days field keeps three digits clear of its
// steppers, and the MCP card's confirmations come up as a sheet. On the
// desktop: the 40px rhythm and the rail with its names once there is room.
// German, because its labels run longest; advanced mode on, so every expert
// control is there too.
import { expect, test, type Page } from "@playwright/test";

const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

// A test can end while a route handler still waits on the real server. Closing
// the context then disposes the response it is about to read, and Playwright
// fails a test that already passed, so what the handler throws is dropped.
test.afterEach(async ({ page }) => {
  await page.unrouteAll({ behavior: "ignoreErrors" });
});

const NOW = Math.floor(Date.now() / 1000);

const SETTINGS = {
  containersEnabled: true,
  vmsEnabled: true,
  flashEnabled: true,
  configEnabled: true,
  filesEnabled: true,
  zfsEnabled: true,
  receiverEnabled: true,
  fleetEnabled: true,
  pullEnabled: true,
  dbDumpsEnabled: true,
  anomalyEnabled: true,
  anomalySensitivity: "strict",
  anomalyNotifyMin: "warning",
  anomalyRetentionHold: true,
  // Three digits in the every-N-days field, the most a phone has to fit.
  containersSchedule: "everyN 120 03:00",
  vmsSchedule: "weekly Sun 04:30",
  flashSchedule: "daily 02:15",
  configSchedule: "daily 02:30",
  filesSchedule: "15 5 */3 * *",
  zfsSchedule: "daily 01:45",
  everythingSchedule: "weekly Sat 23:00",
  everythingPreHookSet: true,
  everythingPostHookSet: true,
  containersOffsiteSchedule: "daily 06:00",
  vmsOffsiteSchedule: "weekly Sun 07:00",
  retentionKeepLast: 7,
  retentionKeepDaily: 14,
  retentionKeepWeekly: 8,
  retentionKeepMonthly: 12,
  offsiteLimitUpload: 20000,
  offsiteLimitDownload: 50000,
  backupCores: 4,
  flashZipExportEnabled: true,
  flashZipExportPath: "user/sicherungen/flash-export/unraid-usb-stick",
  flashZipExportKeep: 5,
  exportEncryptEnabled: true,
  exportAgeRecipients: "age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p",
  metricsEnabled: true,
  metricsTokenSet: true,
  widgetTokenSet: true,
  instanceName: "Tower im Keller (Hauptserver)",
  drillsEnabled: true,
  drillsSchedule: "weekly Sun 05:00",
  drillsSubsetPct: 10,
  containersOffsiteImmutable: true,
  offsiteGrowthBudgetGB: 250,
  pruneImageAfterUpdate: true,
  digestEnabled: true,
  digestSchedule: "weekly Mon 08:00",
  perItemSchedules: true,
  registryAuths: [
    { host: "ghcr.io", username: "junkerderprovinz", token: "", tokenSet: true },
    { host: "registry.gitlab.example-company.internal:5050", username: "deploy-bot-readonly", token: "", tokenSet: true },
  ],
};

const repo = (id: string, name: string, url: string, extra: Record<string, unknown> = {}) => ({
  id,
  name,
  repo: url,
  credsRef: "",
  storageClass: "",
  limitUpload: 0,
  limitDownload: 0,
  immutable: false,
  enabled: true,
  inUse: 2,
  ...extra,
});

const REPOS = [
  repo("r1", "Wasabi Frankfurt (Langzeitarchiv)", "s3:https://s3.eu-central-1.wasabisys.com/bombvault-offsite-langzeitarchiv/container", {
    credsRef: "c1",
    immutable: true,
  }),
  repo("r2", "Hetzner Storage Box", "sftp:u123456@u123456.your-storagebox.de:/home/bombvault/vms"),
  repo("r3", "Zweitserver bei den Eltern", "rest:https://backup.eltern-zuhause.example.net:8000/bombvault/", {
    enabled: false,
    inUse: 0,
  }),
];

function targets(domain: string) {
  const target = (id: string, name: string, url: string, extra: Record<string, unknown>) => ({
    id: `${domain}-${id}`,
    domain,
    name,
    repo: url,
    credsRef: "",
    storageClass: "",
    immutable: false,
    schedule: "",
    retentionKeepLast: 3,
    retentionKeepDaily: 7,
    retentionKeepWeekly: 4,
    retentionKeepMonthly: 6,
    limitUpload: 0,
    limitDownload: 0,
    growthBudgetGb: 0,
    enabled: true,
    createdAt: NOW - 86400 * 30,
    ...extra,
  });
  return [
    target("t1", "Wasabi Frankfurt (Langzeitarchiv)", `s3:https://s3.eu-central-1.wasabisys.com/bombvault-offsite-langzeitarchiv/${domain}`, {
      credsRef: "c1",
      immutable: true,
      sortOrder: 1,
    }),
    target("t2", "Hetzner Storage Box", `sftp:u123456@u123456.your-storagebox.de:/home/bombvault/${domain}`, { sortOrder: 2 }),
  ];
}

const CRED_SETS = [
  {
    id: "c1",
    name: "Wasabi Schlüssel (nur Schreiben)",
    s3KeyId: "BEISPIEL-SCHLUESSEL-ID",
    s3Region: "eu-central-1",
    restUser: "",
    s3StorageClass: "STANDARD_IA",
    s3SecretSet: true,
    restPasswordSet: false,
  },
  {
    id: "c2",
    name: "rest-server Eltern",
    s3KeyId: "",
    s3Region: "",
    restUser: "bombvault-tower",
    s3StorageClass: "",
    s3SecretSet: false,
    restPasswordSet: true,
  },
];

const NOTIFY = {
  on: "failure",
  webhookEnabled: true,
  webhookUrl: "https://discord.com/api/webhooks/123456789012345678/abcdefghijklmnopqrstuvwxyz",
  webhookFormat: "discord",
  matrixEnabled: true,
  matrixHomeserver: "https://matrix.example-homeserver.org",
  matrixToken: "",
  matrixRoom: "!aBcDeFgHiJkLmNoP:example-homeserver.org",
  healthchecksUrl: "https://hc-ping.com/5f1c3d2e-8a7b-4c6d-9e0f-1a2b3c4d5e6f",
  healthchecksByDomain: { containers: "https://hc-ping.com/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" },
  unraid: true,
  smtpEnabled: true,
  smtpHost: "smtp.mailbox-provider.example.com",
  smtpPort: 587,
  smtpUsername: "bombvault@example.com",
  smtpPassword: "",
  smtpFrom: "BombVault <bombvault@example.com>",
  smtpTo: "admin@example.com, zweitadmin@example.com",
  smtpTls: "starttls",
  appriseEnabled: true,
  appriseUrl: "http://apprise.lan:8000/notify/bombvault",
  appriseTags: "backup,unraid",
  scheduledSummary: true,
  notifyOnUpdate: true,
};

const mcpKey = (id: string, label: string, extra: Record<string, unknown> = {}) => ({
  id,
  viaOAuth: false,
  label,
  hint: "f3a9",
  client: "",
  canStartBackups: true,
  createdAt: NOW - 86400 * 20,
  rotatedAt: 0,
  lastUsedAt: NOW - 3600,
  lastUsedFrom: "192.168.178.24",
  revokedAt: 0,
  revokedReason: "",
  inUse: true,
  unusable: "",
  callsToday: 17,
  ...extra,
});

const MCP = {
  ok: true,
  endpointPath: "/mcp",
  limit: 10,
  authEnabled: true,
  hostAllowsKeys: true,
  startsPerHour: 12,
  cooldownMinutes: 15,
  itemStartsPerDay: 4,
  certificate: null,
  keys: [
    mcpKey("k1", "Claude Code auf der Workstation im Arbeitszimmer", { client: "claude-code" }),
    mcpKey("k2", "Laptop", { canStartBackups: false, client: "cursor", callsToday: 0 }),
    mcpKey("k3", "claude.ai Konnektor", {
      viaOAuth: true,
      hint: "",
      client: "claudeai",
      lastUsedFrom: "2a02:8108:1234:5678:9abc:def0:1234:5678",
    }),
  ],
  revoked: [mcpKey("k4", "Alter Tablet-Schlüssel", { revokedAt: NOW - 86400 * 3, revokedReason: "user", inUse: false })],
  oauth: { enabled: true, issuer: "https://bombvault.meine-domain.example.org", active: true, grantLimit: 10, connectorPath: "/mcp" },
};

const MCP_ACTIVITY = {
  ok: true,
  events: [
    { at: NOW - 60, tool: "get_backup_status_for_every_domain_and_repository", outcome: "ok", runId: "" },
    { at: NOW - 120, tool: "start_backup", outcome: "ok", runId: "run-1" },
    { at: NOW - 300, tool: "", outcome: "rate_limited", runId: "" },
  ],
  runs: [],
};

const PASSKEYS = {
  ok: true,
  supported: true,
  rpId: "bombvault.meine-domain.example.org",
  total: 2,
  here: 1,
  passkeys: [
    {
      id: "p1",
      name: "iPhone von Johannes (iCloud-Schlüsselbund)",
      rpId: "bombvault.meine-domain.example.org",
      usableHere: true,
      backedUp: true,
      createdAt: NOW - 86400 * 40,
      lastUsedAt: NOW - 7200,
      transports: "internal,hybrid",
    },
    {
      id: "p2",
      name: "YubiKey 5C NFC",
      rpId: "tower.local",
      usableHere: false,
      backedUp: false,
      createdAt: NOW - 86400 * 90,
      lastUsedAt: 0,
      transports: "usb,nfc",
    },
  ],
};

const container = (name: string, scheduleCadence = "") => ({
  name,
  image: `lscr.io/linuxserver/${name}:latest`,
  state: "running",
  status: "Up 2 hours",
  ip: "172.18.0.5",
  installed: true,
  includeInSchedule: true,
  lastBackup: null,
  lastBackupStarted: null,
  preHook: "",
  postHook: "",
  stopContainers: [],
  excludes: [],
  lastUpdateCheck: 0,
  lastUpdateResult: "",
  stack: "",
  scheduleCadence,
});

const vm = (name: string, libvirtName: string, scheduleCadence = "") => ({
  name,
  libvirtName,
  state: "running",
  method: "graceful",
  includeInSchedule: true,
  lastBackup: null,
  lastBackupStarted: null,
  scheduleCadence,
});

const PAGES = [
  ["general", "Allgemein"],
  ["look", "Aussehen"],
  ["storage", "Speicher"],
  ["retention", "Aufbewahrung"],
  ["schedules", "Zeitpläne"],
  ["containers", "Container"],
  ["offsite", "Off-site"],
  ["cloud", "Cloud-Zugänge"],
  ["notifications", "Benachrichtigungen"],
  ["integrity", "Integrität"],
  ["security", "Sicherheit"],
  ["pairing", "Kopplung"],
  ["integrations", "Anbindungen"],
  ["system", "System"],
] as const;

async function stage(page: Page): Promise<void> {
  const json = (body: unknown) => (route: import("@playwright/test").Route) => route.fulfill({ json: body });
  // Same seeding as narrow-viewport.spec.ts: the stored look is the look, and
  // the server's display prefs must not overwrite it mid-boot.
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.addInitScript(() => {
    window.localStorage.setItem("bv-lang", "de");
    window.localStorage.setItem("bombvault.advanced", "1");
  });
  // The real answer with the staged fields laid over it, so every field the
  // page reads exists. Writes still go through, and nothing here makes one.
  await page.route("**/api/settings", async (route) => {
    if (route.request().method() !== "GET") return route.continue();
    const response = await route.fetch();
    const body = await response.json();
    await route.fulfill({ response, json: { ...body, platform: "unraid", settings: { ...body.settings, ...SETTINGS } } });
  });
  await page.route("**/api/auth", json({ ok: true, enabled: true, authed: true, totp: true, recoveryCodesLeft: 7, minPasswordLen: 12 }));
  await page.route("**/api/auth/passkeys", json(PASSKEYS));
  await page.route("**/api/repos", json({ ok: true, repos: REPOS }));
  await page.route("**/api/offsite/targets*", (route) => {
    const domain = new URL(route.request().url()).searchParams.get("domain") ?? "containers";
    return route.fulfill({ json: { ok: true, targets: targets(domain) } });
  });
  await page.route(
    "**/api/cloud",
    json({
      ok: true,
      s3KeyId: "BEISPIEL-SCHLUESSEL-ID",
      s3Region: "eu-central-1",
      restUser: "bombvault",
      s3SecretSet: true,
      restPasswordSet: true,
      s3StorageClass: "STANDARD_IA",
    }),
  );
  await page.route("**/api/cloud/creds-sets", json({ ok: true, sets: CRED_SETS }));
  await page.route("**/api/rclone", json({ ok: true, remotes: ["gdrive-familienarchiv", "onedrive-business-backup"] }));
  await page.route("**/api/notify", json({ ok: true, notify: NOTIFY, matrixTokenSet: true, smtpPasswordSet: true }));
  await page.route("**/api/mcp/keys?*", json(MCP));
  await page.route("**/api/mcp/keys/*/activity", json(MCP_ACTIVITY));
  await page.route("**/api/dashboard-plugin", json({ ok: true, sshConfigured: true, installed: true, version: "2026.09.20" }));
  await page.route(
    "**/api/containers",
    json({ ok: true, containers: [container("nextcloud-aio-mastercontainer", "daily 01:00"), container("paperless-ngx"), container("vaultwarden")] }),
  );
  await page.route("**/api/vms", json({ ok: true, vms: [vm("Windows 11 Arbeitsplatz", "win11", "weekly Sat 02:00"), vm("Home Assistant OS", "haos")] }));
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

// Scrolling up reveals the search bar, which moves the cards a frame later, so a
// click aimed right after a scroll can land beside its target.
async function scrollRest(page: Page): Promise<void> {
  await page.evaluate(
    () =>
      new Promise<void>((done) => {
        const main = document.getElementById("bv-main")!;
        let last = -1;
        let still = 0;
        const tick = () => {
          still = main.scrollTop === last ? still + 1 : 0;
          last = main.scrollTop;
          if (still >= 5) done();
          else requestAnimationFrame(tick);
        };
        requestAnimationFrame(tick);
      }),
  );
}

function rail(page: Page) {
  return page.getByRole("navigation", { name: "Einstellungsseiten" });
}

async function openPage(page: Page, width: number, id: string, name: string): Promise<void> {
  await stage(page);
  await page.setViewportSize({ width, height: 800 });
  await page.goto(`/settings/${id}`);
  await expect(rail(page).getByRole("link", { name, exact: true })).toHaveAttribute("aria-current", "page");
  // Each page loads its cards' data on mount; the last staged answer to land
  // differs per page, so the page is given its requests and its animations.
  await page.waitForLoadState("networkidle");
  await settle(page);
}

// The content column holds the sr-only heading, the search and the page
// panel, which holds the cards. Column and panel both carry the rhythm.
async function gaps(page: Page): Promise<string[]> {
  return page
    .getByRole("heading", { level: 1 })
    .locator("xpath=..")
    .evaluate((root) => [
      getComputedStyle(root).rowGap,
      getComputedStyle(root.querySelector(":scope > [data-settings-page]")!).rowGap,
    ]);
}

/** The rail's tiles: how many, how many end below the window, the smallest side. */
async function railTiles(page: Page): Promise<{ tiles: number; belowWindow: number; smallest: number }> {
  return rail(page).evaluate((nav) => {
    const boxes = [...nav.querySelectorAll("a")].map((a) => a.getBoundingClientRect());
    return {
      tiles: boxes.length,
      belowWindow: boxes.filter((b) => b.bottom > window.innerHeight + 1).length,
      smallest: Math.round(Math.min(...boxes.flatMap((b) => [b.width, b.height]))),
    };
  });
}

/** How many tiles show their name, and which shown names do not fit. */
async function railNames(page: Page): Promise<{ shown: number; cut: string[] }> {
  return rail(page).evaluate((nav) => {
    const labels = [...nav.querySelectorAll("a > span")] as HTMLElement[];
    const shown = labels.filter((l) => l.getBoundingClientRect().width > 1);
    return {
      shown: shown.length,
      cut: shown
        .filter((l) => l.scrollHeight > l.clientHeight + 1 || l.scrollWidth > l.clientWidth + 1)
        .map((l) => l.textContent ?? ""),
    };
  });
}

for (const width of [320, 360]) {
  for (const [id, name] of PAGES) {
    test(`settings ${id} @ ${width}px: nothing pans, clips or is too small to tap`, async ({ page }, testInfo) => {
      test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
      await openPage(page, width, id, name);

      // Soft, so one run reports every check a page fails rather than the first.
      expect.soft(await gaps(page), "the heading and card gaps").toEqual(["24px", "24px"]);
      const tiles = await railTiles(page);
      expect.soft(tiles.tiles, "a tile for every page").toBe(PAGES.length);
      expect.soft(tiles.smallest, "the smallest side of a tile").toBeGreaterThanOrEqual(24);

      const layout = await page.evaluate(() => {
        const main = document.querySelector("#bv-main");
        if (!main) return null;
        const vw = window.innerWidth;
        const describe = (el: Element) =>
          (el.getAttribute("aria-label") ?? el.textContent ?? el.tagName).trim().replace(/\s+/g, " ").slice(0, 40);
        const controls = [...main.querySelectorAll("button, a, input, select, textarea, [role='switch'], [role='tab']")].filter(
          (el) => {
            const box = el.getBoundingClientRect();
            return box.width > 1 && box.height > 1;
          },
        );
        const card = (el: Element) => el.parentElement?.closest(".rounded-card, .glim-card, .glim-tile") ?? null;
        return {
          docPan: document.documentElement.scrollWidth - vw,
          mainPan: main.scrollWidth - main.clientWidth,
          pastViewport: controls
            .filter((el) => {
              const box = el.getBoundingClientRect();
              return box.right > vw + 1 || box.left < -1;
            })
            .map(describe),
          // Text is counted too: a word or an address with no break
          // opportunity runs past its card while the document stays put.
          pastCard: [...main.querySelectorAll("*")]
            .filter((el) => {
              const box = el.getBoundingClientRect();
              const around = card(el);
              if (!around || box.width === 0 || el.children.length > 0) return false;
              for (let p = el.parentElement; p && p !== around; p = p.parentElement) {
                if (getComputedStyle(p).overflowX !== "visible") return false;
              }
              const edge = around.getBoundingClientRect();
              return box.right > edge.right + 1 || box.left < edge.left - 1;
            })
            .map(describe),
          // WCAG's 24px minimum, except for a link inside a run of text,
          // which the line height decides.
          tooSmall: controls
            .filter((el) => !(el.tagName === "A" && getComputedStyle(el).display === "inline"))
            .filter((el) => {
              const box = el.getBoundingClientRect();
              return box.width < 23.5 || box.height < 23.5;
            })
            .map((el) => `${describe(el)} ${Math.round(el.getBoundingClientRect().width)}x${Math.round(el.getBoundingClientRect().height)}`),
        };
      });
      expect(layout, "#bv-main is missing").not.toBeNull();
      expect.soft(layout!.docPan, "the document scrolls horizontally").toBeLessThanOrEqual(1);
      expect.soft(layout!.mainPan, "#bv-main scrolls horizontally").toBeLessThanOrEqual(1);
      expect.soft(layout!.pastViewport, "controls past the viewport edge").toEqual([]);
      expect.soft(layout!.pastCard, "content past its card's edge").toEqual([]);
      expect.soft(layout!.tooSmall, "controls under 24px").toEqual([]);
      // A backtick in JSX text compiles fine and renders on the page.
      await expect.soft(page.locator("#bv-main")).not.toContainText("`");
    });
  }
}

test("settings on a phone: the rail shows its glyphs alone and keeps their names", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only");
  await openPage(page, 360, "general", "Allgemein");
  // The names stay the links' accessible names; shown, they would squeeze the
  // content column.
  expect((await railNames(page)).shown).toBe(0);
  for (const [, name] of PAGES) await expect(rail(page).getByRole("link", { name, exact: true })).toBeVisible();
  expect((await railTiles(page)).belowWindow, "tiles below the window").toBe(0);
});

for (const [id, name, rows] of [
  ["offsite", "Off-site", () => targets("containers").map((t) => t.name)],
  ["cloud", "Cloud-Zugänge", () => CRED_SETS.map((c) => c.name)],
] as const) {
  test(`settings ${id} on a phone: each row puts its actions under the name`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only");
    await openPage(page, 320, id, name);
    for (const rowName of rows()) {
      const edit = page.getByRole("button", { name: "Bearbeiten" });
      const row = page.locator(".rounded-card").filter({ hasText: rowName }).filter({ has: edit }).last();
      const nameBox = (await row.getByText(rowName, { exact: true }).first().boundingBox())!;
      const actionBox = (await row.getByRole("button", { name: "Bearbeiten" }).first().boundingBox())!;
      expect(actionBox.y, `the actions of "${rowName}" share the name's row`).toBeGreaterThanOrEqual(nameBox.y + nameBox.height);
      expect(nameBox.width, `"${rowName}" is squeezed beside its actions`).toBeGreaterThan(100);
    }
  });
}

test("settings storage on a phone: a repository shows its whole name and address, each switch labelled once", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only");
  await openPage(page, 320, "storage", "Speicher");

  for (const r of REPOS) {
    for (const text of [r.name, r.repo]) {
      const el = page.getByText(text, { exact: true });
      await expect(el).toBeVisible();
      const cut = await el.evaluate((node) => node.scrollWidth > node.clientWidth + 1);
      expect(cut, `"${text}" is cut off`).toBe(false);
    }
  }
  const row = page.locator("div.rounded-card").filter({ hasText: REPOS[0].name }).last();
  await expect(row.getByText("Nur anhängen", { exact: true })).toHaveCount(1);
});

test("settings schedules on a phone: the every-N-days field shows three digits beside its steppers", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the steppers sit side by side under a coarse pointer");
  await openPage(page, 320, "schedules", "Zeitpläne");

  const room = await page.locator("input.glim-num").evaluateAll((inputs) => {
    const input = inputs.find((el) => (el as HTMLInputElement).value === "120") as HTMLInputElement | undefined;
    if (!input) return null;
    const style = getComputedStyle(input);
    const ctx = document.createElement("canvas").getContext("2d")!;
    ctx.font = `${style.fontStyle} ${style.fontWeight} ${style.fontSize} ${style.fontFamily}`;
    return {
      content: input.clientWidth - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight),
      digits: ctx.measureText("000").width,
    };
  });
  expect(room, "no every-N-days field holds 120").not.toBeNull();
  expect(room!.content, "the digits' room beside the steppers").toBeGreaterThanOrEqual(room!.digits);
});

test("settings security on a phone: passkeys wrap", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only");
  await openPage(page, 320, "security", "Sicherheit");

  for (const key of PASSKEYS.passkeys) {
    const name = page.getByText(key.name, { exact: true });
    const cut = await name.evaluate((node) => node.scrollWidth > node.clientWidth + 1);
    expect(cut, `"${key.name}" is cut off`).toBe(false);
  }
});

test("settings integrations on a phone: the MCP confirmations come up as a sheet", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only");
  await openPage(page, 320, "integrations", "Anbindungen");

  await page.getByRole("button", { name: "Schlüssel ersetzen" }).first().click();
  const sheet = page.getByRole("dialog");
  await expect(sheet).toBeVisible();
  await settle(page);
  for (const answer of ["Abbrechen", "Schlüssel ersetzen"]) {
    const box = (await sheet.getByRole("button", { name: answer }).boundingBox())!;
    expect(box.x, `${answer} starts past the left edge`).toBeGreaterThanOrEqual(0);
    expect(box.x + box.width, `${answer} ends past the right edge`).toBeLessThanOrEqual(320);
  }
  await sheet.getByRole("button", { name: "Abbrechen" }).click();
  await expect(sheet).toBeHidden();

  // The client dialog stays a dialog: it fits, and it scrolls inside.
  const client = page.getByRole("button", { name: /Claude Code/ }).first();
  await client.scrollIntoViewIfNeeded();
  await scrollRest(page);
  await client.click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await settle(page);
  const box = (await dialog.boundingBox())!;
  expect(box.x).toBeGreaterThanOrEqual(0);
  expect(box.x + box.width).toBeLessThanOrEqual(320);
  expect(box.y + box.height).toBeLessThanOrEqual(800);
});

test("settings on the desktop keeps the 40px rhythm, with the rail's names once there is room", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "desktop-only");
  const width = testInfo.project.use.viewport!.width;
  await openPage(page, width, "general", "Allgemein");

  expect(await gaps(page)).toEqual(["40px", "40px"]);
  const tiles = await railTiles(page);
  expect(tiles.tiles).toBe(PAGES.length);
  expect(tiles.belowWindow, "tiles below the window").toBe(0);
  // Below 64rem the content column needs the room, so the rail keeps its glyphs.
  const names = await railNames(page);
  expect(names.shown).toBe(width >= 1024 ? PAGES.length : 0);
  expect(names.cut, "names cut off").toEqual([]);
});

test("settings on a wide screen shows every page's name whole", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "desktop-only");
  await openPage(page, 1920, "general", "Allgemein");

  const names = await railNames(page);
  expect(names.shown).toBe(PAGES.length);
  expect(names.cut, "names cut off").toEqual([]);
});
