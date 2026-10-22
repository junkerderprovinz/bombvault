// The Instances page at phone width and on the desktop, staged at the route
// layer with every module switched on (a fresh harness database has them off
// and lists nothing). The picture and the cards stand on the page, with no
// card around them. On the phones: the 24px rhythm, and with every remove
// armed and an instance's window opened with everything in it, no pan, no
// control past the viewport or its card, no text cut short and no stray
// backtick, in the net picture, the grid, the window, its dialogs and the
// empty state. On the desktop: the 40px rhythm, cards of one size, the net
// picture in the arrangement that fits, a role asked for from its tile, a key
// deposit made with its recovery code shown once, and the addresses /receiver,
// /pull and /fleet with their hash forms.
// German, because its labels run longest.
import { expect, test, type Locator, type Page } from "@playwright/test";
import { blockServerLook } from "./serverLook";

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
  away: [],
  settling: false,
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
  reachable: true,
};

const roleRequest = (id: string, state: string, sections: string[]) => ({
  id,
  state,
  sections,
  store: "rest",
  askedAt: iso(2 * 3600),
  decidedAt: state === "asked" ? "" : iso(3600),
  decidedBy: state === "asked" ? "" : "person",
});

const NO_ROLES = { receiver: null, fetcher: null, zfs: null, keeper: null };

// The first member holds every state a tile and a row can show. The second
// runs a release from before role requests, too old to keep a key deposit.
const ROLES = [
  {
    memberId: MEMBER,
    name: MEMBER_NAME,
    reachable: true,
    answers: true,
    canKeep: true,
    keep: "yes",
    byMember: {
      receiver: roleRequest("rr1", "allowed", ["containers", "vms", "flash"]),
      fetcher: roleRequest("rr2", "asked", ["containers"]),
      zfs: { state: "allowed", items: [{ id: "z1", dataset: "cache/appdata", state: "allowed" }] },
      keeper: { state: "held", at: iso(3 * 3600) },
    },
    forMember: {
      receiver: roleRequest("rr3", "asked", ["containers", "vms", "files", "flash", "zfs", "config"]),
      fetcher: roleRequest("rr4", "allowed", ["containers", "files"]),
      zfs: {
        state: "asked",
        items: [
          { id: "q1", dataset: "cache/appdata", state: "asked" },
          { id: "q2", dataset: "cache/domains", state: "allowed" },
        ],
      },
      keeper: { state: "held", at: iso(5 * 3600), asks: 1 },
    },
    asks: 3,
  },
  { memberId: "member-2", name: "Büro Linz", reachable: true, answers: false, canKeep: false, keep: "too-old", byMember: NO_ROLES, forMember: NO_ROLES, asks: 0 },
];

const SELF_ROLES = {
  receiver: [],
  fetcher: [{ memberId: MEMBER, name: MEMBER_NAME }],
  zfs: [{ memberId: MEMBER, name: MEMBER_NAME }],
  keeper: [{ memberId: MEMBER, name: MEMBER_NAME }],
};

const CODE = "K7QF2-M9XRT-4HPLD-W3NVC";

const release = (id: string, over: Record<string, unknown>) => ({
  id,
  ownerId: "member-2",
  ownerName: "Büro Linz",
  requesterId: MEMBER,
  requesterName: MEMBER_NAME,
  state: "asked",
  askedAt: iso(900),
  decidedAt: "",
  releasedAt: "",
  knownSince: iso(200 * 86400),
  requests: 1,
  requestsLastHour: 1,
  ...over,
});

// Every state a keeper's line and a kept deposit can show, a request from an
// instance this server knows and one from an instance it has never seen.
const DEPOSITS = {
  codeSince: iso(40 * 86400),
  sent: [
    { memberId: MEMBER, name: MEMBER_NAME, state: "held", sentAt: iso(3 * 3600) },
    { memberId: "member-5", name: "Nachbarschafts-Tower Familie Oberhuber", state: "stale", sentAt: iso(9 * 86400) },
    { memberId: "member-6", name: "Ferienhaus Wolfgangsee", state: "declined", sentAt: iso(70 * 86400) },
  ],
  kept: [
    { ownerId: MEMBER, ownerName: MEMBER_NAME, state: "held", createdAt: iso(30 * 86400), updatedAt: iso(5 * 3600) },
    { ownerId: "member-2", ownerName: "Büro Linz", state: "held", createdAt: iso(12 * 86400), updatedAt: iso(26 * 3600) },
    { ownerId: "member-6", ownerName: "Ferienhaus Wolfgangsee", state: "declined", createdAt: iso(80 * 86400), updatedAt: iso(70 * 86400) },
  ],
  releases: [
    release("rel1", {}),
    release("rel2", {
      requesterId: "member-9",
      requesterName: "buero-linz-neuinstallation-2026",
      knownSince: "",
      requests: 7,
      requestsLastHour: 3,
    }),
    release("rel3", { state: "released", releasedAt: iso(86400) }),
  ],
};

const NO_DEPOSITS = { codeSince: "", sent: [], kept: [], releases: [] };

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
  sectionPlace: true,
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
  await json("**/api/group/roles", {
    ok: true,
    members: empty ? [] : ROLES,
    self: empty ? { receiver: [], fetcher: [], zfs: [], keeper: [] } : SELF_ROLES,
  });
  await json("**/api/group/deposits", { ok: true, deposits: empty ? NO_DEPOSITS : DEPOSITS });
  await json("**/api/group/members/*/repos", { ok: true, instanceName: MEMBER_NAME, repos: MEMBER_REPOS });
  await json("**/api/zfs/receive/requests", empty ? [] : ZFS_REQUESTS);
  await json("**/api/zfs/replica/local-pools", [{ name: "sicherung", sizeBytes: 8 * 1024 ** 4, freeBytes: 3.1 * 1024 ** 4 }]);
  await json("**/api/zfs/replica/servers", empty ? [] : [ZFS_SERVER]);
  await json("**/api/zfs", { ok: true, datasets: empty ? [] : [{ id: "z1", dataset: "cache/appdata" }] });
  await json("**/api/zfs/datasets/*/replica", REPLICA);
  await json("**/api/offsite/group-receivers", { ok: true, receivers: [] });
  // Same seeding as narrow-viewport.spec.ts: the stored locale is the look,
  // and the server's display prefs must not overwrite it mid-boot.
  await blockServerLook(page);
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

async function expectDialogFits(page: Page, title: string | Locator, options?: { desktop?: boolean }): Promise<void> {
  const dialog = typeof title === "string" ? page.getByRole("dialog", { name: title }) : title;
  await expect(dialog).toBeVisible();
  await settle(page);
  await dialog.evaluate((el) => el.setAttribute("data-probe", ""));
  await expectFits(page, "[data-probe]", options);
  await dialog.evaluate((el) => el.removeAttribute("data-probe"));
}

/** The cards and the picture above them stand on the page: none has a card
 *  around it, and each is as wide as the page's own column lets it be. */
async function expectOnPage(page: Page): Promise<void> {
  const found = await page.evaluate(() => {
    const title = document.querySelector("#bv-main h1")!.getBoundingClientRect();
    const cards = [...document.querySelectorAll<HTMLElement>("#bv-main [data-instance]")];
    const picture = document.querySelector<HTMLElement>('#bv-main [data-testid="net-picture"]')?.closest<HTMLElement>(".rounded-card");
    const lefts = cards.map((el) => Math.round(el.getBoundingClientRect().left));
    return {
      framed: cards.filter((el) => el.parentElement?.closest(".rounded-card")).length,
      pictureFramed: Boolean(picture?.parentElement?.closest(".rounded-card")),
      inset: Math.round(Math.min(...lefts) - title.left),
      pictureInset: picture ? Math.round(picture.getBoundingClientRect().left - title.left) : 0,
    };
  });
  expect(found, "a card stands around the cards or the picture").toMatchObject({ framed: 0, pictureFramed: false });
  expect(Math.abs(found.inset), "the cards are set in from the page's edge").toBeLessThanOrEqual(1);
  expect(Math.abs(found.pictureInset), "the picture is set in from the page's edge").toBeLessThanOrEqual(1);
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
    // Two role requests, a request to hand out a key deposit and a storage
    // offer wait. The count is a readout, and the Details button beside it is
    // the way in.
    await expect(card(page, MEMBER).getByText("4 Anfragen", { exact: true })).toBeVisible();
    await expect(card(page, MEMBER).getByRole("button", { name: "4 Anfragen" })).toHaveCount(0);
    await expect(card(page, "zfs:nas")).toBeVisible();
    await expect(card(page, "zfs:nas").getByText("Verbunden", { exact: true })).toBeVisible();
    // A phone has room for one side of the net picture only.
    await expect(page.locator("#bv-main .glim-net-narrow")).toBeVisible();
    await expect(page.locator("#bv-main .glim-net-wide")).toBeHidden();
    await expect(page.locator("#bv-main .glim-net-narrow").getByRole("button", { name: MEMBER_NAME })).toBeVisible();
    await expect(page.getByText("Ohne Instanz", { exact: true })).toBeVisible();
    // The inventory opens in a window, and lists each source with its date
    // and size in view, not as a table whose last columns scroll away.
    await page.locator("#bv-main").getByRole("button", { name: "Details", exact: true }).last().click();
    const inventory = page.getByRole("dialog", { name: "Bestand nach Quelle" });
    await expect(inventory.getByText("Gesamt", { exact: true })).toBeVisible();
    await expect(inventory.locator("table")).toHaveCount(0);
    await expect(inventory.getByText("49,4 GB", { exact: true })).toHaveCount(1);
    await expectDialogFits(page, inventory);
    await inventory.getByRole("button", { name: "Fertig" }).click();
    await armRemoves(page, "#bv-main");

    expect(await pageGap(page)).toBe("24px");
    await expectFits(page, "#bv-main");
    await expectOnPage(page);
    // Four tiles stand two by two, each pair sharing the card's width however
    // long the names of the roles.
    for (const key of ["self", MEMBER]) {
      const tiles = await card(page, key)
        .locator("[data-role]")
        .evaluateAll((els) => els.map((el) => Math.round(el.getBoundingClientRect().top)));
      expect(tiles, `the role tiles of ${key} left their rows`).toEqual([tiles[0], tiles[0], tiles[2], tiles[2]]);
      expect(tiles[2]).toBeGreaterThan(tiles[0]);
    }
    const states = await card(page, MEMBER)
      .locator("button[data-role]")
      .evaluateAll((els) => els.map((el) => `${el.getAttribute("data-role")}:${el.getAttribute("data-state")}`));
    expect(states).toEqual(["receiver:on", "fetcher:waiting", "zfs:on", "keeper:on"]);
    await expect(card(page, MEMBER).locator('[data-role="keeper"]')).toHaveText("SchlüsselhüterAktiv");
  });

  test(`an instance's window @ ${width}px: everything open, nothing pans, clips or is cut short`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
    await openPage(page, width);
    const win = await openDetails(page);
    for (const name of ["Was Schwiegereltern Tower Gmunden Keller für diesen Server tut", "Was dieser Server für Schwiegereltern Tower Gmunden Keller tut", "Schutz je Bereich", "Überwachung"]) {
      await expect(win.getByRole("region", { name })).toBeAttached();
    }
    // Each received repository opens its inventory in a window of its own.
    const details = win.getByRole("button", { name: "Details", exact: true });
    await expect(details).toHaveCount(2);
    for (let i = 0; i < 2; i++) {
      await details.nth(i).click();
      const inventory = page.getByRole("dialog", { name: "Bestand nach Quelle" });
      await expect(inventory.getByText("Gesamt", { exact: true })).toBeVisible();
      await expectDialogFits(page, inventory);
      await inventory.getByRole("button", { name: "Fertig" }).click();
      await expect(inventory).toHaveCount(0);
    }
    // Each role has its switch, and what the instance asks for stands with
    // the ZFS request in one list.
    await expect(win.getByRole("region", { name: /^Was Schwiegereltern/ }).getByRole("switch")).toHaveCount(4);
    await expect(win.getByText("möchte seine Off-site-Kopien hier ablegen")).toBeVisible();
    // Its request to hand out a key deposit waits in the same list, and the
    // deposit this server keeps for it stands with what was allowed.
    await expect(win.locator("[data-release]")).toHaveCount(1);
    await expect(win.locator('[data-release="rel1"]')).toContainText("fragt nach der Hinterlegung von Büro Linz");
    await expect(win.locator("[data-kept]")).toHaveCount(1);
    await expect(win.locator('[data-request="fetcher"]').getByRole("button", { name: "Freigabe entziehen" })).toBeVisible();
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

    await win.getByRole("button", { name: "Repository hinzufügen", exact: true }).click();
    await expectDialogFits(page, "Empfangenes Repository hinzufügen");
    await page.getByRole("dialog", { name: "Empfangenes Repository hinzufügen" }).getByRole("button", { name: "Abbrechen" }).click();

    await win.getByRole("button", { name: "Bearbeiten", exact: true }).first().click();
    await expectDialogFits(page, "Empfangenes Repository bearbeiten");
    await page.getByRole("dialog", { name: "Empfangenes Repository bearbeiten" }).getByRole("button", { name: "Abbrechen" }).click();

    await win.getByRole("button", { name: "Quelle hinzufügen" }).click();
    await expectDialogFits(page, "Quelle hinzufügen");
    await page.getByRole("dialog", { name: "Quelle hinzufügen" }).getByRole("button", { name: "Abbrechen" }).click();

    await win.getByRole("region", { name: /^Was Schwiegereltern/ }).getByRole("button", { name: "Ändern" }).first().click();
    await expectDialogFits(page, `${MEMBER_NAME} als Empfänger`);
    await page.getByRole("dialog", { name: `${MEMBER_NAME} als Empfänger` }).getByRole("button", { name: "Abbrechen" }).click();

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

  test(`the key deposits @ ${width}px fit the screen, the recovery code included`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
    await page.route("**/api/group/deposit/code", (route) => route.fulfill({ json: { ok: true, recoveryCode: CODE } }));
    await openPage(page, width);
    await card(page, "self").getByRole("button", { name: "Hinterlegung", exact: true }).click();
    const win = page.getByRole("dialog", { name: /^Hinterlegung/ });
    await expect(win.locator("[data-keeper]")).toHaveCount(3);
    await expect(win.locator("[data-kept]")).toHaveCount(3);
    await expect(win.locator("[data-release]")).toHaveCount(2);
    await expect(win.locator('[data-release="rel2"]').getByText("Hier noch nie gesehen")).toBeVisible();
    await expectDialogFits(page, win);

    await win.getByRole("button", { name: "Neuen Code erstellen" }).click();
    await expectDialogFits(page, "Bestätigen");
    await page.getByRole("dialog", { name: "Bestätigen" }).getByRole("button", { name: "Neuen Code erstellen" }).click();
    // On a phone the sheet's name carries its hint, which speaks of the code
    // too, so only the start of the name tells the windows apart.
    const code = page.getByRole("dialog", { name: /^Wiederherstellungscode/ });
    await expect(code.getByTestId("recovery-code").locator("li")).toHaveText(CODE.split("-"));
    await expectDialogFits(page, code);
    await expect(code.getByRole("button", { name: "Fertig" })).toBeDisabled();
    await code.getByRole("switch", { name: "Ich habe den Code aufgeschrieben" }).click();
    await code.getByRole("button", { name: "Fertig" }).click();
    await expect(code).toHaveCount(0);
  });

  test(`the key keeper's dialogs @ ${width}px fit the screen`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
    await openPage(page, width);
    // An instance too old to keep a deposit says so in place of the dialog.
    await card(page, "member-2").locator('button[data-role="keeper"]').click();
    const old = page.getByRole("dialog", { name: "Büro Linz als Schlüsselhüter" });
    await expect(old).toContainText("Aktualisiere Büro Linz zuerst.");
    await expectDialogFits(page, "Büro Linz als Schlüsselhüter");
    await old.getByRole("button", { name: "Schließen" }).click();

    // Once it is current, the dialog says what the deposit is and what the
    // day it is used takes.
    await page.route("**/api/group/roles", (route) =>
      route.fulfill({ json: { ok: true, members: [ROLES[0], { ...ROLES[1], answers: true, canKeep: true, keep: "yes" }], self: SELF_ROLES } }),
    );
    await page.reload();
    await card(page, "member-2").locator('button[data-role="keeper"]').click();
    const dialog = page.getByRole("dialog", { name: "Büro Linz als Schlüsselhüter" });
    await expect(dialog.getByTestId("keeper-needs").locator("li")).toHaveCount(3);
    await expectDialogFits(page, "Büro Linz als Schlüsselhüter");
    await dialog.getByRole("button", { name: "Abbrechen" }).click();

    // Taking a deposit back says what that means before anything is sent.
    await card(page, MEMBER).locator('button[data-role="keeper"]').click();
    await expect(page.getByRole("dialog", { name: "Bestätigen" })).toContainText("die Hinterlegung zu löschen");
    await expectDialogFits(page, "Bestätigen");
  });

  test(`an empty group @ ${width}px: nothing pans or clips`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
    await openPage(page, width, { empty: true });
    await expect(card(page, "none").getByText("Noch keine Instanz verbunden")).toBeVisible();
    await expect(page.getByRole("button", { name: "Kopplung", exact: true })).toHaveCount(1);
    expect(await pageGap(page)).toBe("24px");
    await expectFits(page, "#bv-main");
    await expectOnPage(page);
    await page.getByRole("button", { name: "Kopplung", exact: true }).click();
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
  await expectOnPage(page);

  const boxes = await page.locator("#bv-main [data-instance]").evaluateAll((els) =>
    els.map((el) => {
      const r = el.getBoundingClientRect();
      return { top: Math.round(r.top), width: Math.round(r.width), height: Math.round(r.height) };
    }),
  );
  expect(boxes).toHaveLength(5);
  expect(new Set(boxes.map((b) => b.width)).size, "cards of different widths").toBe(1);
  expect(new Set(boxes.map((b) => b.height)).size, "cards of different heights").toBe(1);
  // Where the window leaves room beside the sidebar, a card is a wide
  // rectangle; on a narrower one it fills the column and its content sets
  // the height.
  if (width >= 1024) {
    for (const b of boxes) expect(b.width, "a card is clearly wider than high").toBeGreaterThanOrEqual(b.height * 1.5);
  }
  // A name is never cut short; a long one wraps.
  const name = card(page, MEMBER).locator("span.font-semibold").first();
  expect(await name.evaluate((node) => getComputedStyle(node).textOverflow)).not.toBe("ellipsis");
  expect(await name.evaluate((node) => getComputedStyle(node).whiteSpace)).toBe("normal");

  // Pairing is the page's own action, in the corner.
  const pair = page.getByTestId("page-actions").getByRole("button", { name: "Kopplung", exact: true });
  await expect(pair).toBeVisible();
  await expect(page.getByRole("button", { name: "Kopplung", exact: true })).toHaveCount(1);
  await pair.click();
  await expect(page).toHaveURL(/\/settings\/pairing$/);
});

test("a Receiver is asked where it keeps the copies", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop-1280", "the flow runs once, on the desktop");
  const sent: unknown[] = [];
  await page.route("**/api/group/members/*/roles/*", async (route) => {
    sent.push(route.request().postDataJSON());
    await route.fulfill({ json: { ok: true, request: roleRequest("rr8", "asked", ["containers"]) } });
  });
  await openPage(page, testInfo.project.use.viewport!.width);
  // The second member answers requests here, so it is asked like a current one.
  await page.route("**/api/group/roles", (route) =>
    route.fulfill({ json: { ok: true, members: [ROLES[0], { ...ROLES[1], answers: true }], self: SELF_ROLES } }),
  );
  await page.reload();
  await expect(card(page, "member-2")).toBeVisible();
  await card(page, "member-2").locator('button[data-role="receiver"]').click();
  const dialog = page.getByRole("dialog", { name: "Büro Linz als Empfänger" });
  const where = dialog.getByRole("radiogroup", { name: /^Wo auf / });
  await expect(where.getByRole("radio")).toHaveText([/^Empfangsserver von /, "Freigabe"]);
  await expect(where.getByRole("radio").first()).toBeChecked();
  await expect(dialog.getByText("rest-server im Append-only-Modus")).toBeVisible();
  await where.getByRole("radio", { name: "Freigabe" }).click();
  await expect(dialog.getByText("rest-server im Append-only-Modus")).toHaveCount(0);
  await dialog.getByRole("button", { name: "Anfrage senden" }).click();
  await expect(dialog).toHaveCount(0);
  expect(sent).toEqual([expect.objectContaining({ store: "share" })]);
});

test("a key deposit is made from its tile, and the recovery code stays until it is written down", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop-1280", "the flow runs once, on the desktop");
  const sent: string[] = [];
  let deposits: unknown = NO_DEPOSITS;
  let members: unknown[] = [{ ...ROLES[0], byMember: { ...ROLES[0].byMember, keeper: null } }, ROLES[1]];
  await page.route("**/api/group/members/*/roles/keeper", async (route) => {
    sent.push(`${route.request().method()} ${new URL(route.request().url()).pathname}`);
    const keeper = { state: "held", at: iso(0) };
    deposits = { ...NO_DEPOSITS, codeSince: iso(0), sent: [{ memberId: MEMBER, name: MEMBER_NAME, state: "held", sentAt: iso(0) }] };
    members = [{ ...ROLES[0], byMember: { ...ROLES[0].byMember, keeper } }, ROLES[1]];
    await route.fulfill({ json: { ok: true, keeper, recoveryCode: CODE } });
  });
  await openPage(page, testInfo.project.use.viewport!.width);
  await page.route("**/api/group/roles", (route) => route.fulfill({ json: { ok: true, members, self: SELF_ROLES } }));
  await page.route("**/api/group/deposits", (route) => route.fulfill({ json: { ok: true, deposits } }));
  await page.reload();
  const tile = card(page, MEMBER).locator('button[data-role="keeper"]');
  await expect(tile).toHaveAttribute("data-state", "off");
  // With no deposit on either side the window has nothing to show yet.
  await expect(card(page, "self").getByRole("button", { name: "Hinterlegung", exact: true })).toHaveCount(0);

  // The tile is one of four, in the form of the others.
  const sizes = await card(page, MEMBER)
    .locator("button[data-role]")
    .evaluateAll((els) => els.map((el) => `${Math.round(el.getBoundingClientRect().width)}x${Math.round(el.getBoundingClientRect().height)}`));
  expect(sizes).toHaveLength(4);
  expect(new Set(sizes).size, "the role tiles differ in size").toBe(1);

  await tile.click();
  const dialog = page.getByRole("dialog", { name: `${MEMBER_NAME} als Schlüsselhüter` });
  await expect(dialog.getByTestId("keeper-needs").locator("li")).toHaveCount(3);
  await expectDialogFits(page, `${MEMBER_NAME} als Schlüsselhüter`, { desktop: true });
  await dialog.getByRole("button", { name: "Hinterlegen" }).click();
  expect(sent).toEqual([`PUT /api/group/members/${MEMBER}/roles/keeper`]);

  const code = page.getByRole("dialog", { name: "Wiederherstellungscode" });
  await expect(code.getByTestId("recovery-code").locator("li")).toHaveText(CODE.split("-"));
  await expectDialogFits(page, "Wiederherstellungscode", { desktop: true });
  // The four groups stand in one row and are the largest text of the window.
  const groups = await code
    .getByTestId("recovery-code")
    .locator("li")
    .evaluateAll((els) => els.map((el) => ({ top: Math.round(el.getBoundingClientRect().top), size: parseFloat(getComputedStyle(el).fontSize) })));
  expect(new Set(groups.map((g) => g.top)).size).toBe(1);
  expect(groups[0].size).toBeGreaterThanOrEqual(20);
  const done = code.getByRole("button", { name: "Fertig" });
  await expect(done).toBeDisabled();
  await page.keyboard.press("Escape");
  await page.mouse.click(5, 5);
  await expect(code).toBeVisible();
  await code.getByRole("switch", { name: "Ich habe den Code aufgeschrieben" }).click();
  await done.click();
  await expect(code).toHaveCount(0);
  await expect(page.getByText("K7QF2")).toHaveCount(0);
  await expect(tile).toHaveAttribute("data-state", "on");
  await expect(card(page, "self").getByRole("button", { name: "Hinterlegung", exact: true })).toBeVisible();
});

test("the key deposits on the desktop: every line fits, and a request is allowed", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "desktop-only");
  const sent: unknown[] = [];
  await page.route("**/api/group/deposits/releases/*", async (route) => {
    sent.push([new URL(route.request().url()).pathname, route.request().postDataJSON()]);
    await route.fulfill({ json: { ok: true, state: "allowed" } });
  });
  await openPage(page, testInfo.project.use.viewport!.width);
  await card(page, "self").getByRole("button", { name: "Hinterlegung", exact: true }).click();
  const win = page.getByRole("dialog", { name: /^Hinterlegung/ });
  await expect(win.getByTestId("code-since")).toContainText("Wiederherstellungscode erstellt am");
  await expect(win.locator("[data-keeper]")).toHaveText([/Aktiv/, /Veraltet.*Jetzt senden/, /Dort gelöscht.*Erneut anfragen/]);
  // An answered request is not listed, and the one from an unknown instance says so.
  await expect(win.locator("[data-release]")).toHaveCount(2);
  const unknown = win.locator('[data-release="rel2"]');
  await expect(unknown).toContainText("Hier noch nie gesehen");
  await expect(unknown).toContainText("7 Anfragen von dort bisher, 3 in der letzten Stunde");
  await expect(win.locator('[data-release="rel1"]')).toContainText("Hier zuerst gesehen am");
  await expectDialogFits(page, win, { desktop: true });

  await unknown.getByRole("button", { name: "Erlauben" }).click();
  expect(sent).toEqual([["/api/group/deposits/releases/rel2", { decision: "allow" }]]);
  await expect(page.getByText("kann die Hinterlegung innerhalb von 30 Minuten abholen")).toBeVisible();

  // Deleting a kept deposit asks first, and the question is no red one.
  await win.locator('[data-kept="member-2"]').getByRole("button", { name: "Löschen" }).click();
  const question = page.getByRole("dialog", { name: "Bestätigen" });
  await expect(question).toContainText("Hinterlegung von Büro Linz löschen?");
  await expectDialogFits(page, "Bestätigen", { desktop: true });
  await question.getByRole("button", { name: "Abbrechen" }).click();
});

test("the net picture on the desktop shows the group with an arrow per role", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "desktop-only");
  await openPage(page, testInfo.project.use.viewport!.width);
  // Beside the sidebar both windows leave room for the narrow picture only.
  // At 1280 its legend stands beside it, at 768 below.
  const picture = page.locator("#bv-main .glim-net-narrow");
  const wide = page.locator("#bv-main .glim-net-wide");
  const key = page.locator("#bv-main .glim-net-key");
  await expect(picture).toBeVisible();
  await expect(wide).toBeHidden();
  const beside = async (pic: typeof picture) => {
    const [p, k] = [(await pic.boundingBox())!, (await key.boundingBox())!];
    return k.x >= p.x + p.width && k.y < p.y + p.height;
  };
  expect(await beside(picture)).toBe(testInfo.project.name === "desktop-1280");
  for (const name of ["Tower Linz", MEMBER_NAME, "Büro Linz", "Backup-NAS im Keller"]) {
    await expect(picture.getByRole("button", { name })).toBeVisible();
  }
  const lanes = await picture.locator(`[data-net="${MEMBER}"] [data-lane]`).evaluateAll((els) => els.map((el) => el.getAttribute("data-lane")));
  // The key deposit both instances hold for each other is no arrow.
  expect(lanes).toEqual(["out:receiver:on", "out:fetcher:waiting", "out:zfs:on", "in:receiver:waiting", "in:fetcher:on", "in:zfs:waiting"]);
  await expect(page.locator("#bv-main .glim-net-key")).toContainText("Wartet auf Freigabe");
  // An arrow that waits is dashed, one that is on is solid.
  const dashes = await picture
    .locator(`[data-net="${MEMBER}"] .glim-net-line`)
    .evaluateAll((els) => els.map((el) => getComputedStyle(el).strokeDasharray !== "none"));
  expect(dashes).toEqual([false, true, false, true, false, true]);
  // Pointing at a node lights up its card.
  await picture.getByRole("button", { name: MEMBER_NAME }).hover();
  await expect(card(page, MEMBER)).toHaveClass(/glim-net-entry-hot/);
  // A dot travels along every arrow that is on.
  const travelling = () =>
    page.evaluate(() => document.getAnimations().filter((a) => (a as CSSAnimation).animationName === "glim-net-travel").length);
  expect(await travelling()).toBeGreaterThan(0);
  await page.emulateMedia({ reducedMotion: "reduce" });
  expect(await travelling(), "the picture moves under reduced motion").toBe(0);

  // A wider window has room for the wide picture, still with the legend beside it.
  await page.setViewportSize({ width: 1600, height: 900 });
  await expect(wide).toBeVisible();
  await expect(picture).toBeHidden();
  expect(await beside(wide)).toBe(true);
});

test("a role is asked for from its tile, and an older instance takes it at once", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop-1280", "the flow runs once, on the desktop");
  const sent: { url: string; body: unknown }[] = [];
  await page.route("**/api/group/members/*/roles/*", async (route) => {
    sent.push({ url: new URL(route.request().url()).pathname, body: route.request().postDataJSON() });
    await route.fulfill({ json: { ok: true, request: roleRequest("rr9", "allowed", ["containers"]) } });
  });
  await openPage(page, testInfo.project.use.viewport!.width);
  await card(page, "member-2").locator('button[data-role="receiver"]').click();
  const dialog = page.getByRole("dialog", { name: "Büro Linz als Empfänger" });
  await expect(dialog).toContainText("läuft ein älteres BombVault");
  await expectDialogFits(page, "Büro Linz als Empfänger", { desktop: true });
  await dialog.getByRole("button", { name: "Einschalten" }).click();
  await expect(dialog).toHaveCount(0);
  expect(sent).toHaveLength(1);
  expect(sent[0].url).toBe("/api/group/members/member-2/roles/receiver");
  await expect(page.getByText("Büro Linz ist jetzt dein Empfänger.")).toBeVisible();

  // Switching one off says what stops before anything is sent.
  await card(page, MEMBER).locator('button[data-role="receiver"]').click();
  const question = page.getByRole("dialog", { name: "Bestätigen" });
  await expect(question).toContainText("schickt keine Off-site-Kopien mehr an");
  await question.getByRole("button", { name: "Abbrechen" }).click();
  expect(sent).toHaveLength(1);
});

test("an instance's window on the desktop holds its cards and fits", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "desktop-only");
  await openPage(page, testInfo.project.use.viewport!.width);
  const win = await openDetails(page);
  await expectDialogFits(page, MEMBER_NAME, { desktop: true });
  // The inventory is a table where there is room for its four columns.
  const details = win.getByRole("button", { name: "Details", exact: true });
  await expect(details).toHaveCount(2);
  for (let i = 0; i < 2; i++) {
    await details.nth(i).click();
    const inventory = page.getByRole("dialog", { name: "Bestand nach Quelle" });
    await expect(inventory.locator("table")).toHaveCount(1);
    await inventory.getByRole("button", { name: "Fertig" }).click();
    await expect(inventory).toHaveCount(0);
  }
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
