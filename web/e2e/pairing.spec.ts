// The Pairing page, in German because its labels run longest. The layout test
// checks that the step cards with their scene, the phrase card, the list of
// instances and the relay card in its four forms fit at 1280px and 390px. The
// flow tests stand in two instances with two pages and
// a faked group API each, since a harness database is in no group: one where
// both generate a phrase and are told after a minute how to merge
// (https://github.com/junkerderprovinz/bombvault/issues/270), and one where
// the second enters the phrase and pairs with the first one's words.
import { expect, test, type Page } from "@playwright/test";

const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

// A test can end while a route handler still waits on the real server. Closing
// the context then disposes the response it is about to read, and Playwright
// fails a test that already passed, so what the handler throws is dropped.
test.afterEach(async ({ page }) => {
  await page.unrouteAll({ behavior: "ignoreErrors" });
});

// The BIP39 test vector: twelve listed words that check out.
const PHRASE = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about";

interface Member {
  id: string;
  name: string;
  version: string;
  direct: boolean;
  relay: boolean;
}

/** One faked instance: what its GET /api/group answers, changed by the
 *  routes the page calls and by the test. */
interface Instance {
  name: string;
  active: boolean;
  joinedAgo: number;
  memberSeen: boolean;
  members: Member[];
  mode: string;
  url: string;
  connected: boolean;
  /** Who a join with PHRASE finds in the group. */
  joinFinds: Member[];
}

function instance(name: string, over: Partial<Instance> = {}): Instance {
  return { name, active: false, joinedAgo: 0, memberSeen: false, members: [], mode: "project", url: "", connected: true, joinFinds: [], ...over };
}

function view(f: Instance) {
  return {
    ok: true,
    active: f.active,
    instanceId: f.name,
    name: f.name,
    passwordSet: true,
    members: f.members,
    relay: {
      mode: f.mode,
      url: f.url,
      projectUrl: "wss://parleyport.halleluja.design/relay/connect",
      connected: f.active && f.connected,
      serve: false,
      serveClients: 0,
    },
    joinedAgo: f.active ? f.joinedAgo : 0,
    memberSeen: f.memberSeen,
    selfAddress: "https://tower-linz.familie-hofer.example.at:3443",
    selfAddressManual: false,
    ownAddresses: ["192.168.178.20", "10.20.30.41", "tower-linz-keller-serverschrank-2.local"],
  };
}

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
  usedBy: [],
  folder: "tower-linz",
  reachable: true,
};

async function stage(page: Page, f: Instance): Promise<void> {
  await page.route("**/api/settings", async (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    const res = await route.fetch();
    const body = await res.json();
    body.settings = { ...body.settings, receiverEnabled: true, fleetEnabled: true, pullEnabled: true, zfsEnabled: true };
    await route.fulfill({ response: res, json: body });
  });
  await page.route("**/api/group/roles", (route) =>
    route.fulfill({
      json: {
        ok: true,
        members: f.members.map((m, i) => ({
          memberId: m.id,
          name: m.name,
          reachable: true,
          answers: true,
          // The first member keeps this instance's copies, so the picture has an arrow.
          byMember: {
            receiver: i === 0 ? { id: "rr1", state: "allowed", sections: ["containers"], store: "rest", askedAt: "", decidedAt: "", decidedBy: "person" } : null,
            fetcher: null,
            zfs: null,
          },
          forMember: { receiver: null, fetcher: null, zfs: null },
          asks: 0,
        })),
        self: { receiver: [], fetcher: [], zfs: [] },
      },
    }),
  );
  await page.route("**/api/zfs/replica/servers", (route) => route.fulfill({ json: [ZFS_SERVER] }));
  await page.route("**/api/zfs/replica/key", (route) => route.fulfill({ json: { publicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIBombVaultReplicaKeyForTheHarness bombvault-replica" } }));
  await page.route("**/api/group", async (route) => {
    if (route.request().method() === "DELETE") Object.assign(f, { active: false, joinedAgo: 0, memberSeen: false, members: [] });
    await route.fulfill({ json: view(f) });
  });
  await page.route("**/api/group/phrase", async (route) => {
    Object.assign(f, { active: true, joinedAgo: 0, memberSeen: false, members: [] });
    await route.fulfill({ json: { ok: true, phrase: PHRASE, group: view(f) } });
  });
  await page.route("**/api/group/join", async (route) => {
    const { phrase } = route.request().postDataJSON() as { phrase: string };
    if (phrase !== PHRASE) return route.fulfill({ json: { ok: false, reason: "checksum" } });
    Object.assign(f, { active: true, joinedAgo: 0, members: f.joinFinds, memberSeen: f.joinFinds.length > 0 });
    await route.fulfill({ json: view(f) });
  });
  await page.route("**/api/group/relay", async (route) => {
    const patch = route.request().postDataJSON() as { mode?: string; url?: string };
    if (patch.mode) f.mode = patch.mode;
    if (patch.url !== undefined) f.url = patch.url;
    await route.fulfill({ json: view(f) });
  });
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

/** No pan, no control past the viewport or past its card. */
async function expectFits(page: Page): Promise<void> {
  const found = await page.evaluate(() => {
    const root = document.querySelector("#bv-main")!;
    const vw = window.innerWidth;
    const name = (el: Element) => (el.getAttribute("aria-label") ?? el.textContent ?? "").trim().slice(0, 40);
    // The Settings rail's strip scrolls sideways, so its tiles stand past the
    // window by design; the strip itself has to stay inside.
    const strip = "nav[data-rail-form='strip']";
    const controls = [...root.querySelectorAll(`button, a, input, textarea, [role='switch'], [role='tab'], ${strip}`)].filter(
      (el) => !el.parentElement?.closest(strip),
    );
    return {
      docPan: document.documentElement.scrollWidth - vw,
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
    };
  });
  expect(found.docPan, "the document scrolls horizontally").toBeLessThanOrEqual(1);
  expect(found.pastViewport, "controls clip the viewport edge").toEqual([]);
  expect(found.pastCard, "controls stick out of their card").toEqual([]);
}

async function open(page: Page, f: Instance, width = 1280): Promise<void> {
  await stage(page, f);
  await page.setViewportSize({ width, height: 900 });
  await page.goto("/settings/pairing");
  await expect(page.getByRole("navigation", { name: "Einstellungsseiten" }).getByRole("link", { name: "Kopplung", exact: true })).toHaveAttribute("aria-current", "page");
  await expect(page.getByText("Erste Instanz", { exact: true })).toBeVisible();
  await settle(page);
}

const phraseCard = (page: Page) => page.locator("[data-stage]");
const pairState = (page: Page) => page.getByTestId("pair-state");
const generate = (page: Page) => page.getByRole("button", { name: /Phrase generieren/ });
const enter = (page: Page) => page.getByRole("button", { name: /Phrase eingeben/ });

const attic: Member = { id: "attic", name: "Schwiegereltern Tower Gmunden Keller", version: "v9.2.1", direct: false, relay: true };
const cellar: Member = { id: "cellar", name: "Büro Linz", version: "v9.2.1", direct: true, relay: true };

for (const width of [1280, 390]) {
  test(`pairing @ ${width}px: steps, tiles, the alone hint and relay fit`, async ({ page }, testInfo) => {
    const phone = MOBILE_PROJECTS.has(testInfo.project.name);
    test.skip(phone !== (width === 390), width === 390 ? "phone width runs on the phone projects" : "desktop width runs on the desktop projects");
    test.skip(!phone && testInfo.project.use.viewport!.width < width, "the 768px project cannot show 1280px");
    const f = instance("Tower Linz", { active: true, members: [attic, cellar], memberSeen: true, joinedAgo: 600 });
    await open(page, f, width);

    for (const title of ["Erste Instanz", "Jede weitere", "Fertig"]) {
      await expect(page.getByRole("heading", { level: 2, name: new RegExp(title) })).toBeVisible();
    }
    const list = page.getByTestId("members");
    await expect(list.getByText("Schwiegereltern Tower Gmunden Keller")).toBeVisible();
    await expect(pairState(page)).toHaveText("Gekoppelt");
    // A member says in plain words that it is paired and how it is reached.
    await expect(list.getByRole("listitem").filter({ hasText: "Büro Linz" })).toContainText("Gekoppelt · Direkt");
    await expect(list.getByRole("listitem").filter({ hasText: "Schwiegereltern" })).toContainText("Gekoppelt · Über Relay");
    // Another group's phrase can be entered without leaving this one first.
    await expect(enter(page)).toBeVisible();
    // The list ends with the way in for a ZFS server that runs no BombVault.
    const zfsRow = page.getByTestId("zfs-row");
    await expect(zfsRow).toContainText("ZFS-Server ohne BombVault");
    await expect(zfsRow).toContainText("Backup-NAS im Keller");
    // What the instance found out about itself stands above the address a
    // person can correct, as lines and no fields.
    const found = page.getByTestId("own-addresses");
    await expect(found.getByRole("listitem")).toHaveText(["192.168.178.20", "10.20.30.41", "tower-linz-keller-serverschrank-2.local"]);
    await expect(found.locator("input")).toHaveCount(0);
    const known = page.getByLabel(/^Bekannte Adressen/);
    await expect(known).toHaveValue("https://tower-linz.familie-hofer.example.at:3443");
    expect((await found.boundingBox())!.y).toBeLessThan((await known.boundingBox())!.y);
    // The group as a network stands on the Instances page and not here.
    await expect(page.getByTestId("net-picture")).toHaveCount(0);
    await expectFits(page);
    await zfsRow.getByRole("button", { name: "Per SSH verbinden" }).click();
    const connect = page.getByRole("dialog", { name: "ZFS-Server hinzufügen" });
    await expect(connect).toBeVisible();
    await connect.getByRole("button", { name: "Abbrechen" }).click();
    await expect(connect).toHaveCount(0);

    // One pointer plays the scene over the three cards, and stands still
    // where motion is reduced.
    const pointing = () =>
      page.evaluate(() => document.getAnimations().filter((a) => (a as CSSAnimation).animationName === "glim-step-pointer").length);
    expect(await pointing()).toBe(1);
    await page.emulateMedia({ reducedMotion: "reduce" });
    expect(await pointing(), "the scene moves under reduced motion").toBe(0);
    expect(await page.locator(".glim-step-pic").first().evaluate((svg) => svg.getAnimations({ subtree: true }).length)).toBe(0);
    await page.emulateMedia({ reducedMotion: "no-preference" });

    // The three cards sit side by side on the desktop and stack on a phone.
    // A title that wraps makes its heading taller, so the pictures are measured.
    const tops = await page.locator("[data-step-frame]").evaluateAll((frames) => frames.map((f) => Math.round(f.getBoundingClientRect().top)));
    expect(tops).toHaveLength(3);
    expect(new Set(tops).size).toBe(width === 1280 ? 1 : 3);

    // The relay in its four forms, each with its picture.
    await expect(page.getByRole("radio")).toHaveText(["Projekt-Relay", "Eigenes Relay", "BombVault-Instanz", "Kein Relay"]);
    await expect(page.locator("#relay-address")).toHaveCount(0);
    await expect(page.getByRole("img", { name: /sprechen über das Relay/ })).toBeVisible();
    await page.getByRole("radio", { name: "Eigenes Relay" }).click();
    await expect(page.locator("#relay-address")).toBeVisible();
    await expect(page.getByTestId("own-relay")).toContainText("ParleyPort");
    await expect(page.getByText("Selbst gehostet")).toBeVisible();
    await settle(page);
    await expectFits(page);
    await page.getByRole("radio", { name: "BombVault-Instanz" }).click();
    await expect(page.getByRole("img", { name: /ist selbst das Relay/ })).toBeVisible();
    await expect(page.getByRole("switch", { name: "Als Relay dienen" })).toBeVisible();
    await expect(page.locator("#relay-address")).toBeVisible();
    await settle(page);
    await expectFits(page);
    await page.getByRole("radio", { name: "Kein Relay" }).click();
    await expect(page.locator("#relay-address")).toHaveCount(0);
    await expect(page.getByRole("img", { name: /ohne Relay nicht erreichbar/ })).toBeVisible();
    await settle(page);
    await expectFits(page);
    // What the relay sees opens in a window.
    await page.getByRole("button", { name: "Was das Relay sieht" }).click();
    await expect(page.getByRole("dialog", { name: "Was das Relay sieht" })).toContainText("nichts verlässt dein Netzwerk");
    await page.keyboard.press("Escape");
    await expect(page.getByRole("dialog")).toHaveCount(0);

    // Alone, with the relay down: both hints and the paste field fit.
    Object.assign(f, { members: [], memberSeen: false, joinedAgo: 90, mode: "project", connected: false });
    await page.reload();
    await expect(phraseCard(page)).toHaveAttribute("data-stage", "alone");
    await expect(page.getByText("Relay nicht erreichbar", { exact: true })).toBeVisible();
    await expect(page.getByTestId("relay-check")).toContainText("parleyport.halleluja.design");
    await settle(page);
    await expectFits(page);

    // Outside a group: the two tiles fit, and the paste field opens under them.
    Object.assign(f, { active: false });
    await page.reload();
    await expect(generate(page)).toBeVisible();
    await enter(page).click();
    await expect(page.getByRole("textbox", { name: "Die zwölf Wörter deiner ersten Instanz" })).toBeVisible();
    await settle(page);
    await expectFits(page);
  });
}

test("two instances that both generate a phrase are told after a minute how to become one group", async ({ page, context }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop-1280", "the flow runs once, on the desktop");
  const a = instance("Tower Linz");
  const b = instance("Büro Linz");
  const pageB = await context.newPage();
  for (const [p, f] of [
    [page, a],
    [pageB, b],
  ] as const) {
    await p.clock.install();
    await open(p, f);
    await generate(p).click();
    await expect(phraseCard(p)).toHaveAttribute("data-stage", "new");
    await expect(p.getByText("Neue Gruppe", { exact: true })).toBeVisible();
    await expect(p.getByText("Jetzt auf der anderen Instanz")).toBeVisible();
    // The words open in a window of their own; closing it leaves the card.
    await p.keyboard.press("Escape");
    await expect(p.getByRole("dialog")).toHaveCount(0);
  }

  a.joinedAgo = b.joinedAgo = 61;
  for (const p of [page, pageB]) {
    await p.clock.fastForward(11_000);
    await expect(phraseCard(p)).toHaveAttribute("data-stage", "alone");
    await expect(p.getByText("Noch allein", { exact: true })).toBeVisible();
    await expect(p.getByRole("button", { name: /Drüben auch eine Phrase generiert\?/ })).toBeVisible();
  }

  // B enters A's words in place of its own and finds A.
  b.joinFinds = [{ id: "a", name: "Tower Linz", version: "v9.2.1", direct: true, relay: false }];
  await pageB.getByRole("button", { name: /Drüben auch eine Phrase generiert\?/ }).click();
  await pageB.getByRole("textbox", { name: "Die zwölf Wörter der anderen Instanz" }).fill(PHRASE);
  await pageB.getByRole("button", { name: "Koppeln" }).click();
  await expect(phraseCard(pageB)).toHaveAttribute("data-stage", "paired");
  await expect(pairState(pageB)).toHaveText("Gekoppelt");
});

test("a relay address on a paired instance opens the page on the BombVault route", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop-1280", "the flow runs once, on the desktop");
  const f = instance("Tower Linz", {
    active: true,
    members: [cellar],
    memberSeen: true,
    joinedAgo: 600,
    mode: "own",
    url: "wss://bombvault.familie-hofer.example.at/relay/connect",
  });
  await open(page, f);
  await expect(page.getByRole("radio", { name: "BombVault-Instanz" })).toHaveAttribute("aria-checked", "true");
  await expect(page.locator("#relay-address")).toHaveValue("wss://bombvault.familie-hofer.example.at/relay/connect");
  // A letter travels in the picture, at the pace the stylesheet sets.
  const letters = await page.evaluate(() =>
    document.getAnimations().filter((a) => (a as CSSAnimation).animationName === "glim-letter-arc").length,
  );
  expect(letters).toBe(2);
});

test("the second instance enters the phrase, pastes the numbered words and both read paired", async ({ page, context }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop-1280", "the flow runs once, on the desktop");
  const a = instance("Tower Linz");
  const b = instance("Büro Linz", { joinFinds: [{ id: "a", name: "Tower Linz", version: "v9.2.1", direct: true, relay: false }] });
  await page.clock.install();
  await open(page, a);
  await generate(page).click();
  await expect(page.getByRole("list", { name: "Die zwölf Wörter" })).toContainText("about");

  const pageB = await context.newPage();
  await open(pageB, b);
  await enter(pageB).click();
  const field = pageB.getByRole("textbox", { name: "Die zwölf Wörter deiner ersten Instanz" });
  const pair = pageB.getByRole("button", { name: "Koppeln" });
  await field.fill("abandon abandon unveel ");
  await expect(pageB.getByRole("alert")).toHaveText("Wort 3 („unveel“) steht nicht auf der Wortliste.");
  await expect(pair).toBeDisabled();
  await field.fill(
    PHRASE.split(" ")
      .map((w, i) => `${i + 1}. ${w}`)
      .join("\n"),
  );
  await expect(pageB.getByText("12 von 12 Wörtern")).toBeVisible();
  await expect(pair).toBeEnabled();
  await pair.click();
  await expect(pairState(pageB)).toHaveText("Gekoppelt");
  await expect(pageB.getByTestId("members").getByText("Tower Linz", { exact: true })).toBeVisible();

  Object.assign(a, { members: [{ id: "b", name: "Büro Linz", version: "v9.2.1", direct: true, relay: false }], memberSeen: true });
  await page.clock.fastForward(11_000);
  await expect(phraseCard(page)).toHaveAttribute("data-stage", "paired");
  await expect(pairState(page)).toHaveText("Gekoppelt");
});
