// The Instances page at phone width, every lane staged at the route layer with
// the three features switched on (a fresh harness database has them off and
// lists nothing). On the phones, per lane: the 24px rhythm on the page and in
// the tab panel, and with every card open and every remove armed, no pan, no
// control past the viewport or its card, no text cut short and no stray
// backtick, in the lists, the dialogs and the empty states. On the desktop:
// the 40px rhythm and the one-line cards and inventory table as they were.
// German, because its labels run longest.
import { expect, test, type Page } from "@playwright/test";

const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

const now = Math.floor(Date.now() / 1000);
const iso = (secondsAgo: number) => new Date((now - secondsAgo) * 1000).toISOString();

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
  memberId: "member-1",
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
  receivedRepo("r3", "Ferienhaus Wolfgangsee", "/mnt/user/empfang/ferienhaus-wolfgangsee-containers", {
    enabled: false,
    lastCheckOk: null,
    lastCheckAt: 0,
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

const peer = (id: string, name: string, url: string, over: Record<string, unknown>) => ({
  id,
  name,
  url,
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
  memberId: "member-1",
  needsPairing: false,
  direct: false,
  relay: true,
  ...over,
});

const PEERS = [
  peer("p1", "tower", "https://tower-schwiegereltern.familie-hofer.example.at:3443", {
    lastPollInstanceName: "Schwiegereltern Tower Gmunden Keller",
  }),
  peer("p2", "Büro Linz", "https://10.20.30.40:3443", {
    lastPollOk: false,
    lastPollError:
      'Get "https://10.20.30.40:3443/api/fleet/status": dial tcp 10.20.30.40:3443: connect: connection refused',
    lastPollVersion: "",
  }),
  peer("p3", "Ferienhaus Wolfgangsee", "https://ferienhaus.example.at", {
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
    from: "Schwiegereltern Tower Gmunden Keller",
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
  memberId: "member-1",
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
  members: [
    { id: "member-1", name: "Schwiegereltern Tower Gmunden Keller", version: "v9.2.0", direct: false, relay: true },
    { id: "member-2", name: "Büro Linz", version: "v9.2.0", direct: true, relay: true },
  ],
  relay: {
    mode: "project",
    url: "",
    projectUrl: "wss://relay.halleluja.design/relay/connect",
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

type Staged = { empty?: boolean };

async function stage(page: Page, { empty = false }: Staged = {}): Promise<void> {
  await page.route("**/api/settings", async (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    const res = await route.fetch();
    const body = await res.json();
    body.settings = { ...body.settings, receiverEnabled: true, fleetEnabled: true, pullEnabled: true };
    await route.fulfill({ response: res, json: body });
  });
  await page.route("**/api/receiver/repos", (route) =>
    route.fulfill({ json: { ok: true, repos: empty ? [] : RECEIVED } }),
  );
  await page.route("**/api/receiver/repos/*/inventory", (route) =>
    route.fulfill({ json: { ok: true, inventory: INVENTORY } }),
  );
  await page.route("**/api/fleet/peers", (route) => route.fulfill({ json: { ok: true, peers: empty ? [] : PEERS } }));
  await page.route("**/api/fleet/peers/*/mesh-offer", (route) =>
    route.fulfill({ json: { ok: true, snippet: SNIPPET } }),
  );
  await page.route("**/api/fleet/mesh-offers", (route) =>
    route.fulfill({ json: { ok: true, offers: empty ? [] : OFFERS } }),
  );
  await page.route("**/api/pull/sources", (route) =>
    route.fulfill({ json: { ok: true, sources: empty ? [] : PULLS } }),
  );
  await page.route("**/api/group", (route) => route.fulfill({ json: GROUP }));
  await page.route("**/api/group/members/*/repos", (route) =>
    route.fulfill({ json: { ok: true, instanceName: "Schwiegereltern Tower Gmunden Keller", repos: MEMBER_REPOS } }),
  );
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

const LANES = ["receiver", "fleet", "pull"] as const;
type Lane = (typeof LANES)[number];

// The button a lane is ready by. Fleet rows come from the pairing group, so
// that lane has nothing to add and waits for a card's poll button instead.
const ADD_LABEL: Record<Lane, string> = {
  receiver: "Empfangenes Repo hinzufügen",
  fleet: "Jetzt abfragen",
  pull: "Quelle hinzufügen",
};

const EMPTY_LABEL: Record<Lane, string> = { ...ADD_LABEL, fleet: "Zur Kopplung" };

async function openLane(page: Page, width: number, lane: Lane, staged: Staged = {}): Promise<void> {
  await stage(page, staged);
  await page.setViewportSize({ width, height: 800 });
  await page.goto(`/instances#${lane}`);
  await expect(page.getByRole("heading", { level: 1, name: "Instanzen" })).toBeVisible();
  await expect(page.getByRole("tab", { name: "Holen" })).toBeVisible();
  await expect(page.getByRole("button", { name: ADD_LABEL[lane] }).first()).toBeVisible();
  await settle(page);
}

/** Opens every card's details and arms every remove, the widest state a card
 *  reaches. */
async function openEverything(page: Page): Promise<void> {
  const details = page.locator("#bv-main").getByRole("button", { name: "Details", exact: true });
  for (let i = 0; i < (await details.count()); i++) await details.nth(i).click();
  const remove = page.locator("#bv-main").getByRole("button", { name: "Entfernen", exact: true });
  while ((await remove.count()) > 0) await remove.first().click();
  await settle(page);
}

async function gaps(page: Page): Promise<{ page: string; lane: string }> {
  return {
    page: await page
      .getByRole("heading", { level: 1 })
      .locator("xpath=..")
      .evaluate((root) => getComputedStyle(root).rowGap),
    lane: await page.locator("#bv-main .glim-tab-slide > div").evaluate((root) => getComputedStyle(root).rowGap),
  };
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
          const card = el.closest(".rounded-card")?.getBoundingClientRect();
          const r = el.getBoundingClientRect();
          return card && r.width > 0 && (r.right > card.right + 1 || r.left < card.left - 1);
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

async function expectDialogFits(page: Page, title: string): Promise<void> {
  const dialog = page.getByRole("dialog", { name: title });
  await expect(dialog).toBeVisible();
  await settle(page);
  await dialog.evaluate((el) => el.setAttribute("data-probe", ""));
  await expectFits(page, "[data-probe]");
}

async function tabFills(page: Page): Promise<{ strip: string; tabsWithoutFill: number }> {
  return page.getByRole("tablist", { name: "Instanzen" }).evaluate((strip) => ({
    strip: getComputedStyle(strip).backgroundColor,
    tabsWithoutFill: [...strip.querySelectorAll('[role="tab"]')].filter(
      (tab) => getComputedStyle(tab).backgroundColor === "rgba(0, 0, 0, 0)",
    ).length,
  }));
}

for (const width of [320, 360]) {
  for (const lane of LANES) {
    test(`${lane} @ ${width}px: every card open, nothing pans, clips or is cut short`, async ({ page }, testInfo) => {
      test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
      await openLane(page, width, lane);
      await openEverything(page);
      if (lane === "receiver") await expect(page.getByText("Gesamt", { exact: true })).toHaveCount(RECEIVED.length);

      expect(await gaps(page)).toEqual({ page: "24px", lane: "24px" });
      await expectFits(page, "#bv-main");
      expect(await tabFills(page), "each tab is its own badge, with no groove behind the strip").toEqual({
        strip: "rgba(0, 0, 0, 0)",
        tabsWithoutFill: 0,
      });

      if (lane !== "fleet") {
        // The last-received or last-pull block moves under the name and
        // starts where the name does, instead of squeezing the name beside it.
        const [name, meta] = lane === "receiver" ? [RECEIVED[0].name, "1284 Backups"] : [PULLS[0].name, "318 Snapshots geholt"];
        const card = page.locator("#bv-main div.rounded-card.glim-hue").first();
        // Measured on the text itself: a right-aligned line sits in a box that
        // starts at the same edge.
        const textX = (text: string) =>
          card.getByText(text, { exact: true }).evaluate((el) => {
            const range = document.createRange();
            range.selectNodeContents(el);
            return range.getBoundingClientRect().x;
          });
        expect(Math.abs((await textX(meta)) - (await textX(name))), `"${meta}" does not line up under the name`).toBeLessThan(1);
      }

      if (lane === "receiver") {
        // The inventory lists each source with its date and size in view,
        // rather than a table whose last columns scroll out of the card.
        await expect(page.locator("#bv-main table")).toHaveCount(0);
        await expect(page.locator("#bv-main").getByText("49.4 GB", { exact: true })).toHaveCount(RECEIVED.length);
      }
    });

    test(`${lane} dialogs @ ${width}px fit the screen`, async ({ page }, testInfo) => {
      test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
      await openLane(page, width, lane);

      if (lane !== "fleet") {
        await page.getByRole("button", { name: ADD_LABEL[lane] }).click();
        await expectDialogFits(page, ADD_LABEL[lane]);
        await page.getByRole("button", { name: "Abbrechen" }).click();

        await page.locator("#bv-main").getByRole("button", { name: "Bearbeiten", exact: true }).first().click();
        const editTitle = { receiver: "Empfangenes Repo bearbeiten", pull: "Bearbeiten" }[lane];
        await expectDialogFits(page, editTitle);
        await page.getByRole("button", { name: "Abbrechen" }).click();
      }

      if (lane === "fleet") {
        await page.getByRole("button", { name: "Speicher anbieten" }).first().click();
        await expectDialogFits(page, "Off-site-Speicher anbieten");
        await page.getByRole("dialog").getByRole("textbox").fill("http://192.168.178.20:8000");
        await page.getByRole("button", { name: "Angebot senden" }).click();
        await expect(page.getByRole("button", { name: "Kopieren" })).toHaveCount(2);
        await expectDialogFits(page, "Off-site-Speicher anbieten");
      }
    });
  }

  test(`empty lanes @ ${width}px: nothing pans or clips`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
    await openLane(page, width, "receiver", { empty: true });
    for (const [lane, tab] of [["receiver", "Empfänger"], ["fleet", "Flotte"], ["pull", "Holen"]] as const) {
      await page.getByRole("tab", { name: tab }).click();
      await expect(page.getByRole("button", { name: EMPTY_LABEL[lane] })).toBeVisible();
      await settle(page);
      expect(await gaps(page)).toEqual({ page: "24px", lane: "24px" });
      await expectFits(page, "#bv-main");
    }
  });
}

for (const lane of LANES) {
  test(`${lane} on the desktop keeps the 40px rhythm and one-line cards`, async ({ page }, testInfo) => {
    test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "desktop-only");
    const width = testInfo.project.use.viewport!.width;
    await openLane(page, width, lane);
    await openEverything(page);

    expect(await gaps(page)).toEqual({ page: "40px", lane: "40px" });
    await expectFits(page, "#bv-main", { desktop: true });
    if (lane === "pull") return;

    // Names and addresses stay one truncated line, and the receiver's
    // last-received block stays beside the name.
    const card = page.locator("#bv-main div.rounded-card.glim-hue").first();
    // A fleet card of a paired member shows no address, only its name.
    const oneLine = [card.locator("span.font-semibold").first()];
    if (lane !== "fleet") oneLine.push(card.locator("p.font-mono").first());
    for (const el of oneLine) {
      expect(await el.evaluate((node) => getComputedStyle(node).whiteSpace)).toBe("nowrap");
    }
    if (lane === "receiver") {
      const nameBox = (await card.locator("span.font-semibold").first().boundingBox())!;
      const receivedBox = (await card.getByText("Zuletzt empfangen", { exact: true }).first().boundingBox())!;
      expect(receivedBox.y, "the last-received block left the name's row").toBeLessThan(nameBox.y + nameBox.height);
      await expect(page.locator("#bv-main table")).toHaveCount(RECEIVED.length);
    }
    // At 768px the armed confirm needs a second row; at 1280px there is room.
    if (width >= 1280) {
      const tops = await card
        .getByRole("button", { name: /^(Details|Bearbeiten|Entfernen bestätigen)$/ })
        .evaluateAll((buttons) => [...new Set(buttons.map((b) => Math.round(b.getBoundingClientRect().top)))]);
      expect(tops, "the card actions wrapped").toHaveLength(1);
    }
  });
}
