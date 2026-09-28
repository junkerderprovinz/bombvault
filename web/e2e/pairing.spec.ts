// The Pairing tab at 1280px and at 390px, in German because its labels run
// longest: the three step cards, the phrase card and the relay card fit the
// screen, and the relay selector swaps the route and shows the address field
// for an own relay only. The group is staged at the route layer, since a fresh
// harness database is in no group.
import { expect, test, type Page } from "@playwright/test";

const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

function group(mode: string) {
  return {
    ok: true,
    active: true,
    instanceId: "self",
    name: "Tower Linz",
    passwordSet: true,
    members: [
      { id: "m1", name: "Schwiegereltern Tower Gmunden Keller", version: "v9.2.0", direct: false, relay: true },
      { id: "m2", name: "Büro Linz", version: "v9.2.0", direct: true, relay: true },
    ],
    relay: {
      mode,
      url: "wss://relay.familie-hofer.example.at",
      projectUrl: "wss://relay.halleluja.design/relay/connect",
      connected: true,
      serve: false,
      serveClients: 0,
    },
  };
}

async function stage(page: Page): Promise<void> {
  let mode = "project";
  await page.route("**/api/settings", async (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    const res = await route.fetch();
    const body = await res.json();
    body.settings = { ...body.settings, receiverEnabled: true, fleetEnabled: true, pullEnabled: true };
    await route.fulfill({ response: res, json: body });
  });
  await page.route("**/api/group", (route) => route.fulfill({ json: group(mode) }));
  await page.route("**/api/group/relay", async (route) => {
    const patch = route.request().postDataJSON() as { mode?: string };
    if (patch.mode) mode = patch.mode;
    await route.fulfill({ json: group(mode) });
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

async function open(page: Page, width: number): Promise<void> {
  await stage(page);
  await page.setViewportSize({ width, height: 900 });
  await page.goto("/instances#pairing");
  await expect(page.getByRole("tab", { name: "Kopplung" })).toHaveAttribute("aria-selected", "true");
  await expect(page.getByText("Hier ablesen")).toBeVisible();
  await settle(page);
}

for (const width of [1280, 390]) {
  test(`pairing @ ${width}px: steps, phrase and relay fit, and the route switches`, async ({ page }, testInfo) => {
    const phone = MOBILE_PROJECTS.has(testInfo.project.name);
    test.skip(phone !== (width === 390), width === 390 ? "phone width runs on the phone projects" : "desktop width runs on the desktop projects");
    test.skip(!phone && testInfo.project.use.viewport!.width < width, "the 768px project cannot show 1280px");
    await open(page, width);

    for (const title of ["Hier ablesen", "Drüben eintippen", "Fertig"]) {
      await expect(page.getByRole("heading", { level: 2, name: new RegExp(title) })).toBeVisible();
    }
    await expect(page.getByText("Schwiegereltern Tower Gmunden Keller")).toBeVisible();
    await expectFits(page);

    // The three cards sit side by side on the desktop and stack on a phone.
    const tops = await page
      .getByRole("heading", { level: 2, name: /Hier ablesen|Drüben eintippen|Fertig/ })
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
    await expect(page.getByRole("img", { name: /Instanz C in einem anderen Netz/ })).toBeVisible();
  });
}
