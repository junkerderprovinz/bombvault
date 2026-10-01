// The Pairing tab, in German because its labels run longest. The layout test
// checks that the step cards, the phrase card and the relay card fit at
// 1280px and 390px. The flow tests stand in two instances with two pages and
// a faked group API each, since a harness database is in no group: one where
// both generate a phrase and are told after a minute how to merge
// (https://github.com/junkerderprovinz/bombvault/issues/270), and one where
// the second enters the phrase and pairs with the first one's words.
import { expect, test, type Page } from "@playwright/test";

const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

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
  connected: boolean;
  /** Who a join with PHRASE finds in the group. */
  joinFinds: Member[];
}

function instance(name: string, over: Partial<Instance> = {}): Instance {
  return { name, active: false, joinedAgo: 0, memberSeen: false, members: [], mode: "project", connected: true, joinFinds: [], ...over };
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
      url: "wss://relay.familie-hofer.example.at",
      projectUrl: "wss://relay.halleluja.design/relay/connect",
      connected: f.active && f.connected,
      serve: false,
      serveClients: 0,
    },
    joinedAgo: f.active ? f.joinedAgo : 0,
    memberSeen: f.memberSeen,
  };
}

async function stage(page: Page, f: Instance): Promise<void> {
  await page.route("**/api/settings", async (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    const res = await route.fetch();
    const body = await res.json();
    body.settings = { ...body.settings, receiverEnabled: true, fleetEnabled: true, pullEnabled: true };
    await route.fulfill({ response: res, json: body });
  });
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
    const patch = route.request().postDataJSON() as { mode?: string };
    if (patch.mode) f.mode = patch.mode;
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
    const controls = [...root.querySelectorAll("button, a, input, textarea, [role='switch'], [role='tab']")];
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
    await expect(page.getByText("Schwiegereltern Tower Gmunden Keller")).toBeVisible();
    await expect(phraseCard(page).getByText("Gekoppelt", { exact: true })).toBeVisible();
    await expectFits(page);

    // The three cards sit side by side on the desktop and stack on a phone.
    const tops = await page
      .getByRole("heading", { level: 2, name: /Erste Instanz|Jede weitere|Fertig/ })
      .evaluateAll((hs) => hs.map((h) => Math.round(h.getBoundingClientRect().top)));
    expect(new Set(tops).size).toBe(width === 1280 ? 1 : 3);

    await expect(page.locator("#relay-address")).toHaveCount(0);
    await page.getByRole("tab", { name: "Eigenes Relay" }).click();
    await expect(page.locator("#relay-address")).toBeVisible();
    await expect(page.getByText("Woher du ein Relay bekommst")).toBeVisible();
    await settle(page);
    await expectFits(page);
    await page.getByRole("tab", { name: "Kein Relay" }).click();
    await expect(page.locator("#relay-address")).toHaveCount(0);
    // The picture and what the relay sees open in place, and fit too.
    await page.getByRole("button", { name: "Wie funktioniert das?" }).click();
    await expect(page.getByRole("img", { name: /Instanz C in einem anderen Netz/ })).toBeVisible();
    await settle(page);
    await expectFits(page);

    // Alone, with the relay down: both hints and the paste field fit.
    Object.assign(f, { members: [], memberSeen: false, joinedAgo: 90, mode: "project", connected: false });
    await page.reload();
    await expect(phraseCard(page)).toHaveAttribute("data-stage", "alone");
    await expect(page.getByText("Relay nicht erreichbar", { exact: true })).toBeVisible();
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
  await expect(phraseCard(pageB).getByText("Gekoppelt", { exact: true })).toBeVisible();
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
  await expect(phraseCard(pageB).getByText("Gekoppelt", { exact: true })).toBeVisible();
  await expect(pageB.getByText("Tower Linz", { exact: true }).last()).toBeVisible();

  Object.assign(a, { members: [{ id: "b", name: "Büro Linz", version: "v9.2.1", direct: true, relay: false }], memberSeen: true });
  await page.clock.fastForward(11_000);
  await expect(phraseCard(page)).toHaveAttribute("data-stage", "paired");
  await expect(phraseCard(page).getByText("Gekoppelt", { exact: true })).toBeVisible();
});
