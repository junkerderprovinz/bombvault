// The Instances page at phone width and on the desktop, staged at the route
// layer with every module switched on (a fresh harness database has them off
// and lists nothing). On the phones: the 24px rhythm, and with every remove
// armed and an instance's window opened with everything in it, no pan, no
// control past the viewport or its card, no text cut short and no stray
// backtick, in the grid, the window, its dialogs and the empty state. On the
// desktop: the 40px rhythm, cards of one size, and the addresses /receiver,
// /pull and /fleet with their hash forms.
// German, because its labels run longest.
import { expect, test, type Page } from "@playwright/test";

const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

// A test can end while a route handler still waits on the real server. Closing
// the context then disposes the response it is about to read, and Playwright
// fails a test that already passed, so what the handler throws is dropped.
test.afterEach(async ({ page }) => {
  await page.unrouteAll({ behavior: "ignoreErrors" });
});

const now = Math.floor(Date.now() / 1000);
const iso = (secondsAgo: number) => new Date((now - secondsAgo) * 1000).toISOString();

const MEMBER = "member-1";
const MEMBER_NAME = "Schwiegereltern Tower Gmunden Keller";

const receivedRepo = (id: string, name: string, repo: string, over: Record<string, unknown>) => ({
  id,
  name,
  repo,
  deadManHours: 26,
  checkCadence: "",
  readDataPercent: 5,
  lastCheckAt: now - 3 * 3600,
  lastCheckOk: true,
  lastCheckError: "",
  lastCheckReadData: false,
  enabled: true,
  createdAt: now - 90 * 86400,
  sortOrder: 0,
  memberId: MEMBER,
  needsPairing: false,
  lastReceived: iso(5 * 3600),
  snapshotCount: 1284,
  reachable: true,
  ...over,
});

const RECEIVED = [
  receivedRepo(
    "r1",
    "Schwiegereltern Tower Offsite Gmunden",
    "rest:https://backup-empfang.familie-hofer.example.at:8443/schwiegereltern-tower-containers",
    {},
  ),
  receivedRepo(
    "r2",
    "Büro Linz Nextcloud und Datenbanken",
    "s3:https://s3.eu-central-2.wasabisys.com/buero-linz-bombvault-offsite-append-only/vms",
    {
      reachable: false,
      lastCheckOk: false,
      lastCheckError:
        "Fatal: unable to open config file: Stat: The specified key does not exist. Is there a repository at the following location?",
      snapshotCount: 0,
      lastReceived: "",
    },
  ),
  // Set up before pairing, so no instance's window can take it.
  receivedRepo("r3", "Ferienhaus Wolfgangsee", "/mnt/user/empfang/ferienhaus-wolfgangsee-containers", {
    enabled: false,
    lastCheckOk: null,
    lastCheckAt: 0,
    memberId: "",
    needsPairing: true,
  }),
];

const INVENTORY = {
  sources: [
    { host: "schwiegereltern-tower", item: "nextcloud-aio-mastercontainer", snapshotCount: 412, lastReceived: iso(5 * 3600), totalSize: 48_300_000_000 },
    { host: "schwiegereltern-tower", item: "dbdump:immich_postgres", snapshotCount: 1204, lastReceived: iso(6 * 3600), totalSize: 3_900_000_000 },
    { host: "tower", item: "homeassistant", snapshotCount: 96, lastReceived: iso(30 * 3600), totalSize: 812_000_000 },
  ],
  snapshotCount: 1712,
  lastReceived: iso(5 * 3600),
  totalSize: 53_012_000_000,
};

const domain = (name: string, protection: string, lastSuccess: number) => ({
  domain: name,
  enabled: true,
  schedule: "daily 03:00",
  coveredBy: "",
  lastSuccess,
  periodSeconds: 86400,
  status: "ok",
  protection,
});

const peer = (id: string, memberId: string, name: string, over: Record<string, unknown>) => ({
  id,
  memberId,
  name,
  url: "",
  enabled: true,
  lastPollAt: now - 2 * 3600,
  lastPollOk: true,
  lastPollError: "",
  lastPollInstanceName: "",
  lastPollVersion: "v9.0.0",
  lastPollDomains: [
    domain("containers", "green", now - 4 * 3600),
    domain("vms", "amber", now - 50 * 3600),
    domain("files", "red", now - 12 * 86400),
  ],
  createdAt: now - 60 * 86400,
  sortOrder: 0,
  needsPairing: false,
  direct: false,
  relay: true,
  ...over,
});

const PEERS = [
  peer("p1", MEMBER, "tower", { lastPollInstanceName: MEMBER_NAME }),
  peer("p2", "member-2", "Büro Linz", {
    lastPollOk: false,
    lastPollError:
      'Get "https://10.20.30.40:3443/api/fleet/status": dial tcp 10.20.30.40:3443: connect: connection refused',
    lastPollVersion: "",
  }),
  peer("p3", "", "Ferienhaus Wolfgangsee", {
    url: "https://ferienhaus.example.at",
    needsPairing: true,
    enabled: false,
    lastPollAt: 0,
    lastPollOk: null,
    lastPollVersion: "",
    lastPollDomains: [],
  }),
];

const OFFERS = [
  {
    id: "o1",
    from: MEMBER_NAME,
    suggestedDomain: "containers",
    repo: "rest:http://bombvault-mesh@192.168.178.50:8000/schwiegereltern-tower-containers-mesh-offsite",
    restUser: "bombvault-mesh",
    status: "pending",
    receivedAt: now - 20 * 60,
  },
  {
    id: "o2",
    from: "",
    suggestedDomain: "vms",
    repo: "rest:http://10.20.30.40:8000/buero-linz-vms",
    restUser: "mesh",
    status: "pending",
    receivedAt: now - 3 * 86400,
  },
];

const pullSource = (id: string, name: string, repo: string, over: Record<string, unknown>) => ({
  id,
  name,
  repo,
  credsRef: "",
  domain: "containers",
  cadence: "",
  limitDownload: 0,
  limitUpload: 0,
  lastPullAt: now - 7 * 3600,
  lastPullOk: true,
  lastPullError: "",
  snapshotsPulled: 318,
  enabled: true,
  createdAt: now - 30 * 86400,
  sortOrder: 0,
  memberId: MEMBER,
  needsPairing: false,
  ...over,
});

const PULLS = [
  pullSource(
    "s1",
    "Nachbarschafts-Tower Familie Oberhuber",
    "rest:https://pull.oberhuber-familie.example.at:8000/oberhuber-tower-containers-append-only",
    {},
  ),
  pullSource("s2", "Büro Linz Archiv", "s3:https://s3.eu-central-2.wasabisys.com/buero-linz-archiv/files", {
    domain: "files",
    lastPullOk: false,
    lastPullError:
      "Fatal: repository does not exist: unable to open config file: Stat: 403 Forbidden, the credentials for this bucket were rejected",
  }),
  pullSource("s3", "Ferienhaus Wolfgangsee", "/mnt/remotes/ferienhaus/bombvault/zfs", {
    domain: "zfs",
    enabled: false,
    lastPullOk: null,
    lastPullAt: 0,
    snapshotsPulled: 0,
    memberId: "",
    needsPairing: true,
  }),
];

const SNIPPET = {
  user: "bombvault-mesh",
  password: "beispiel-passwort",
  htpasswd: "bombvault-mesh:$2y$05$abcdefghijklmnopqrstuv",
  dockerRun:
    "docker run -d --name bombvault-mesh-rest -p 8000:8000 -v /mnt/user/appdata/bombvault-mesh:/data -e OPTIONS='--append-only --private-repos' restic/rest-server:0.13.0",
  compose:
    "services:\n  rest-server:\n    image: restic/rest-server:0.13.0\n    ports: [\"8000:8000\"]\n    volumes: [\"/mnt/user/appdata/bombvault-mesh:/data\"]\n    environment:\n      OPTIONS: \"--append-only --private-repos\"",
  unraid: "",
  repo: "rest:http://bombvault-mesh@192.168.178.20:8000/tower-containers",
};

const GROUP = {
  ok: true,
  active: true,
  instanceId: "self",
  name: "Tower Linz",
  passwordSet: false,
  selfAddress: "https://tower-linz.familie-hofer.example.at:3443",
  members: [
    { id: MEMBER, name: MEMBER_NAME, version: "v9.2.0", direct: false, relay: true, address: "https://tower-schwiegereltern.familie-hofer.example.at:3443" },
    { id: "member-2", name: "Büro Linz", version: "v9.2.0", direct: true, relay: true, address: "https://10.20.30.40:3443" },
  ],
  relay: {
    mode: "project",
    url: "",
    projectUrl: "wss://parleyport.halleluja.design/relay/connect",
    connected: true,
    serve: false,
    serveClients: 0,
  },
};

const MEMBER_REPOS = [
  {
    domain: "containers",
    name: "Offsite Gmunden",
    location: "rest:https://backup-empfang.familie-hofer.example.at:8443/schwiegereltern-tower-containers",
  },
];

const KEEP = { preset: "own", own: [0, 7, 3, 0, 0] };

const zfsRequest = (id: string, item: string, over: Record<string, unknown>) => ({
  id,
  peer: MEMBER,
  peerName: MEMBER_NAME,
  sourceServer: "schwiegereltern-tower-gmunden",
  item,
  members: [item, `${item}/nextcloud-aio-mastercontainer-volumes`],
  proposedKeep: KEEP,
  state: "asked",
  pool: "",
  root: "",
  keep: KEEP,
  askedAt: iso(1800),
  decidedAt: "",
  lastReceived: "",
  bytes: 0,
  ...over,
});

const ZFS_REQUESTS = [
  zfsRequest("q1", "cache/appdata", {}),
  zfsRequest("q2", "cache/domains", {
    state: "allowed",
    pool: "sicherung",
    root: "sicherung/bombvault-replica",
    decidedAt: iso(9 * 86400),
    lastReceived: iso(2 * 3600),
    bytes: 118 * 1024 * 1024,
  }),
];

const ZFS_SERVER = {
  id: "nas",
  name: "Backup-NAS im Keller",
  host: "backup-nas.familie-hofer.example.at",
  user: "bombvault-replica",
  port: 22,
  pool: "sicherung",
  root: "sicherung/bombvault-replica",
  enabled: true,
  freeBytes: 3.1 * 1024 ** 4,
  sizeBytes: 8 * 1024 ** 4,
  usedBy: ["z1"],
  folder: "tower-linz",
};

const REPLICA = {
  target: { kind: "peer", id: MEMBER },
  afterBackup: true,
  cadence: "",
  keep: KEEP,
  state: "ok",
  code: "",
  lastRun: iso(2 * 3600),
  lastBytes: 31 * 1024 * 1024,
  lastSeconds: 9,
  snapshots: [],
  members: [],
  peerState: "allowed",
};

const RECEIVER_SERVER = {
  containerName: "bombvault-rest-server",
  folder: "user/restic",
  hostPath: "/mnt/user/restic/empfang-fuer-die-ganze-familie",
  port: 8000,
  user: "tower-linz-ausserhalb",
  host: "",
  url: "http://tower-linz.familie-hofer.example.at:8000",
  createdAt: now - 40 * 86400,
  present: true,
  check: "protected",
  checkDetail: "",
  checkedAt: now - 3600,
  logins: [{ memberId: MEMBER, name: MEMBER_NAME, user: "schwiegereltern-tower-gmunden-keller", createdAt: now - 30 * 86400 }],
};

type Staged = { empty?: boolean };

async function stage(page: Page, { empty = false }: Staged = {}): Promise<void> {
  await page.route("**/api/settings", async (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    const res = await route.fetch();
    const body = await res.json();
    body.settings = { ...body.settings, receiverEnabled: true, fleetEnabled: true, pullEnabled: true, zfsEnabled: true };
    await route.fulfill({ response: res, json: body });
  });
  const json = (url: string, body: unknown) => page.route(url, (route) => route.fulfill({ json: body }));
  await json("**/api/receiver/repos", { ok: true, repos: empty ? [] : RECEIVED });
  await json("**/api/receiver/repos/*/inventory", { ok: true, inventory: INVENTORY });
  await json("**/api/receiver/server", { ok: true, server: empty ? null : RECEIVER_SERVER, defaultPort: 8000, hostMountRoot: "/host/user" });
  await json("**/api/fleet/peers", { ok: true, peers: empty ? [] : PEERS });
  await json("**/api/fleet/peers/*/mesh-offer", { ok: true, snippet: SNIPPET });
  await json("**/api/fleet/mesh-offers", { ok: true, offers: empty ? [] : OFFERS });
  await json("**/api/pull/sources", { ok: true, sources: empty ? [] : PULLS });
  await json("**/api/group", empty ? { ...GROUP, active: false, members: [] } : GROUP);
  await json("**/api/group/members/*/repos", { ok: true, instanceName: MEMBER_NAME, repos: MEMBER_REPOS });
  await json("**/api/zfs/receive/requests", empty ? [] : ZFS_REQUESTS);
  await json("**/api/zfs/replica/local-pools", [{ name: "sicherung", sizeBytes: 8 * 1024 ** 4, freeBytes: 3.1 * 1024 ** 4 }]);
  await json("**/api/zfs/replica/servers", empty ? [] : [ZFS_SERVER]);
  await json("**/api/zfs", { ok: true, datasets: empty ? [] : [{ id: "z1", dataset: "cache/appdata" }] });
  await json("**/api/zfs/datasets/*/replica", REPLICA);
  await json("**/api/offsite/group-receivers", { ok: true, receivers: [] });
  // Same seeding as narrow-viewport.spec.ts: the stored locale is the look,
  // and the server's display prefs must not overwrite it mid-boot.
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.addInitScript(() => window.localStorage.setItem("bv-lang", "de"));
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

const card = (page: Page, key: string) => page.locator(`#bv-main [data-instance="${key}"]`);

async function openPage(page: Page, width: number, staged: Staged = {}, path = "/instances"): Promise<void> {
  await stage(page, staged);
  await page.setViewportSize({ width, height: 800 });
  await page.goto(path);
  await expect(page.getByRole("heading", { level: 1, name: "Instanzen" })).toBeVisible();
  await expect(card(page, "self")).toBeVisible();
  await settle(page);
}

/** Arms every remove inside `scope`, the widest state its buttons reach. */
async function armRemoves(page: Page, scope: string): Promise<void> {
  const remove = page.locator(scope).getByRole("button", { name: "Entfernen", exact: true });
  while ((await remove.count()) > 0) await remove.first().click();
  await settle(page);
}

async function openDetails(page: Page) {
  await card(page, MEMBER).getByRole("button", { name: "Details", exact: true }).click();
  const win = page.getByRole("dialog", { name: MEMBER_NAME });
  await expect(win).toBeVisible();
  await expect(win.getByText("möchte cache/appdata hierher replizieren")).toBeVisible();
  await settle(page);
  return win;
}

/** What must not show inside `scope`: a pan, a control past the viewport or
 *  past its own card, and on a phone, text cut short by an ellipsis. */
async function expectFits(page: Page, scope: string, { desktop = false } = {}): Promise<void> {
  const found = await page.evaluate((selector) => {
    const root = document.querySelector(selector);
    if (!root) return null;
    const vw = window.innerWidth;
    const name = (el: Element) => (el.getAttribute("aria-label") ?? el.textContent ?? "").trim().slice(0, 40);
    const controls = [...root.querySelectorAll("button, a, input, select, textarea, [role='switch'], [role='combobox']")];
    return {
      docPan: document.documentElement.scrollWidth - vw,
      scopePan: root.scrollWidth - root.clientWidth,
      pastViewport: controls
        .filter((el) => {
          const r = el.getBoundingClientRect();
          return r.width > 0 && (r.right > vw + 1 || r.left < -1);
        })
        .map(name),
      pastCard: controls
        .filter((el) => {
          const box = el.closest(".rounded-card")?.getBoundingClientRect();
          const r = el.getBoundingClientRect();
          return box && r.width > 0 && (r.right > box.right + 1 || r.left < box.left - 1);
        })
        .map(name),
      // A picker's current value may end in an ellipsis: its open list shows
      // every option in full.
      cutShort: [...root.querySelectorAll("*")]
        .filter((el) => !el.closest("[role='combobox']"))
        .filter((el) => getComputedStyle(el).textOverflow === "ellipsis" && el.scrollWidth > el.clientWidth + 1)
        .map(name),
      // The failed-check help quotes two restic commands in backticks; any
      // other backtick is JSX that leaked into the page.
      backtick: (root.textContent ?? "").replace(/`restic repair (index|snapshots)`/g, "").includes("`"),
    };
  }, scope);
  expect(found, `${scope} is missing`).not.toBeNull();
  expect(found!.docPan, "the document scrolls horizontally").toBeLessThanOrEqual(1);
  expect(found!.scopePan, `${scope} scrolls horizontally`).toBeLessThanOrEqual(1);
  expect(found!.pastViewport, "controls clip the viewport edge").toEqual([]);
  expect(found!.pastCard, "controls stick out of their card").toEqual([]);
  if (!desktop) expect(found!.cutShort, "text is cut short").toEqual([]);
  expect(found!.backtick, "a stray backtick renders on the page").toBe(false);
}

async function expectDialogFits(page: Page, title: string, options?: { desktop?: boolean }): Promise<void> {
  const dialog = page.getByRole("dialog", { name: title });
  await expect(dialog).toBeVisible();
  await settle(page);
  await dialog.evaluate((el) => el.setAttribute("data-probe", ""));
  await expectFits(page, "[data-probe]", options);
  await dialog.evaluate((el) => el.removeAttribute("data-probe"));
}

const pageGap = (page: Page) =>
  page
    .getByRole("heading", { level: 1 })
    .locator("xpath=..")
    .evaluate((root) => getComputedStyle(root).rowGap);

for (const width of [320, 360]) {
  test(`grid @ ${width}px: every remove armed, nothing pans, clips or is cut short`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
    await openPage(page, width);
    await expect(card(page, MEMBER).getByRole("button", { name: "2 Anfragen" })).toBeVisible();
    await expect(card(page, "zfs:nas")).toBeVisible();
    await expect(page.getByText("Ohne Instanz", { exact: true })).toBeVisible();
    await page.locator("#bv-main").getByRole("button", { name: "Details", exact: true }).last().click();
    await expect(page.locator("#bv-main").getByText("Gesamt", { exact: true })).toBeVisible();
    await armRemoves(page, "#bv-main");

    expect(await pageGap(page)).toBe("24px");
    await expectFits(page, "#bv-main");
    // The inventory lists each source with its date and size in view, and not
    // as a table whose last columns scroll out of the card.
    await expect(page.locator("#bv-main table")).toHaveCount(0);
    await expect(page.locator("#bv-main").getByText("49.4 GB", { exact: true })).toHaveCount(1);
    // Three tiles share a card's width, however long the names of the roles.
    const tiles = await card(page, "self")
      .locator("[data-role]")
      .evaluateAll((els) => els.map((el) => Math.round(el.getBoundingClientRect().top)));
    expect(new Set(tiles).size, "the role tiles left their row").toBe(1);
  });

  test(`an instance's window @ ${width}px: everything open, nothing pans, clips or is cut short`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
    await openPage(page, width);
    const win = await openDetails(page);
    for (const name of ["Was Schwiegereltern Tower Gmunden Keller für diesen Server tut", "Was dieser Server für Schwiegereltern Tower Gmunden Keller tut", "Schutz je Bereich", "Überwachung"]) {
      await expect(win.getByRole("region", { name })).toBeAttached();
    }
    const details = win.getByRole("button", { name: "Details", exact: true });
    for (let i = 0; i < (await details.count()); i++) await details.nth(i).click();
    await expect(win.getByText("Gesamt", { exact: true })).toHaveCount(2);
    await win.evaluate((el) => el.setAttribute("data-probe", ""));
    await armRemoves(page, "[data-probe]");
    await expectFits(page, "[data-probe]");
    await win.getByRole("button", { name: "Fertig" }).click();
    await expect(win).toHaveCount(0);
  });

  test(`the dialogs of an instance's window @ ${width}px fit the screen`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
    await openPage(page, width);
    const win = await openDetails(page);

    await win.getByRole("button", { name: "Empfangenes Repo hinzufügen" }).click();
    await expectDialogFits(page, "Empfangenes Repo hinzufügen");
    await page.getByRole("dialog", { name: "Empfangenes Repo hinzufügen" }).getByRole("button", { name: "Abbrechen" }).click();

    await win.getByRole("button", { name: "Bearbeiten", exact: true }).first().click();
    await expectDialogFits(page, "Empfangenes Repo bearbeiten");
    await page.getByRole("dialog", { name: "Empfangenes Repo bearbeiten" }).getByRole("button", { name: "Abbrechen" }).click();

    await win.getByRole("button", { name: "Quelle hinzufügen" }).click();
    await expectDialogFits(page, "Quelle hinzufügen");
    await page.getByRole("dialog", { name: "Quelle hinzufügen" }).getByRole("button", { name: "Abbrechen" }).click();

    await win.getByRole("button", { name: "Speicher anbieten" }).click();
    const offer = page.getByRole("dialog", { name: "Off-site-Speicher anbieten" });
    await expectDialogFits(page, "Off-site-Speicher anbieten");
    await offer.getByRole("textbox").fill("http://192.168.178.20:8000");
    await offer.getByRole("button", { name: "Angebot senden" }).click();
    await expect(offer.getByRole("button", { name: "Kopieren" })).toHaveCount(2);
    await expectDialogFits(page, "Off-site-Speicher anbieten");
  });

  test(`the receiving server @ ${width}px fits the screen`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
    await openPage(page, width);
    await card(page, "self").getByRole("button", { name: "Empfangsserver" }).click();
    await expectDialogFits(page, "Empfangsserver");
    await expect(page.getByRole("dialog", { name: "Empfangsserver" }).getByText("schwiegereltern-tower-gmunden-keller")).toBeVisible();
  });

  test(`an empty group @ ${width}px: nothing pans or clips`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
    await openPage(page, width, { empty: true });
    await expect(card(page, "none").getByText("Noch keine Instanz verbunden")).toBeVisible();
    await expect(page.getByRole("button", { name: "Instanz koppeln" })).toHaveCount(1);
    expect(await pageGap(page)).toBe("24px");
    await expectFits(page, "#bv-main");
    await page.getByRole("button", { name: "Instanz koppeln" }).click();
    await expect(page).toHaveURL(/\/settings\/pairing$/);
  });
}

test("the grid on the desktop keeps the 40px rhythm and cards of one size", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "desktop-only");
  const width = testInfo.project.use.viewport!.width;
  await openPage(page, width);
  await armRemoves(page, "#bv-main");

  expect(await pageGap(page)).toBe("40px");
  await expectFits(page, "#bv-main", { desktop: true });

  const boxes = await page.locator("#bv-main [data-instance]").evaluateAll((els) =>
    els.map((el) => {
      const r = el.getBoundingClientRect();
      return { top: Math.round(r.top), width: Math.round(r.width), height: Math.round(r.height) };
    }),
  );
  expect(boxes).toHaveLength(5);
  expect(new Set(boxes.map((b) => b.width)).size, "cards of different widths").toBe(1);
  for (const top of new Set(boxes.map((b) => b.top))) {
    const row = boxes.filter((b) => b.top === top);
    expect(new Set(row.map((b) => b.height)).size, "cards of one row differ in height").toBe(1);
  }
  // A name stays one line on the desktop, cut with an ellipsis if it must be.
  const name = card(page, MEMBER).locator("span.font-semibold").first();
  expect(await name.evaluate((node) => getComputedStyle(node).whiteSpace)).toBe("nowrap");
});

test("an instance's window on the desktop holds its cards and fits", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "desktop-only");
  await openPage(page, testInfo.project.use.viewport!.width);
  const win = await openDetails(page);
  const details = win.getByRole("button", { name: "Details", exact: true });
  for (let i = 0; i < (await details.count()); i++) await details.nth(i).click();
  // The inventory is a table where there is room for its four columns.
  await expect(win.locator("table")).toHaveCount(2);
  await expectDialogFits(page, MEMBER_NAME, { desktop: true });
  await page.keyboard.press("Escape");
  await expect(win).toHaveCount(0);
});

test("a ZFS server's card opens its own page", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "desktop-only");
  await openPage(page, testInfo.project.use.viewport!.width);
  await card(page, "zfs:nas").getByRole("button", { name: "Öffnen" }).click();
  await expect(page).toHaveURL(/\/instances\/zfs\/nas$/);
  await expect(page.getByRole("heading", { level: 1, name: "ZFS-Server" })).toBeVisible();
  await expect(page.locator("#bv-main").getByText("bombvault-replica@backup-nas.familie-hofer.example.at:22")).toBeVisible();
  await page.locator("#bv-main").getByRole("button", { name: "Zurück" }).click();
  await expect(page).toHaveURL(/\/instances$/);
});

for (const [path, lands] of [
  ["/receiver", "receiver"],
  ["/instances#receiver", "receiver"],
  ["/pull", "grid"],
  ["/instances#pull", "grid"],
  ["/fleet", "grid"],
  ["/instances#fleet", "grid"],
] as const) {
  test(`${path} lands on the instances page`, async ({ page }, testInfo) => {
    test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "desktop-only");
    await openPage(page, testInfo.project.use.viewport!.width, {}, path);
    await expect(page).toHaveURL(/\/instances$/);
    await expect(card(page, MEMBER)).toBeVisible();
    await expect(page.getByRole("dialog", { name: "Empfangsserver" })).toHaveCount(lands === "receiver" ? 1 : 0);
  });
}
