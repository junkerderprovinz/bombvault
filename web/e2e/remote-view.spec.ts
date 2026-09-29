// Remote view: the Fleet card's Open button scopes the whole app to a paired
// member (?instance= in the URL), the bar it shows names that member and
// leaving it drops back to this instance, internal navigation between remote
// view's own pages keeps the scope instead of silently bouncing back to this
// instance, and Recovery, Instances and Settings are off the rail while it is
// open. The peer itself is faked at the route layer, since a harness database
// pairs with nobody; /api/instances/{id}/... calls that reach the real server
// unmocked answer "not a member" and are ignored by the assertions that do
// not depend on them.
import { expect, test, type Page } from "@playwright/test";

const now = Math.floor(Date.now() / 1000);

const PEER = {
  id: "row-1",
  memberId: "member-attic",
  name: "attic",
  url: "",
  enabled: true,
  needsPairing: false,
  direct: true,
  relay: false,
  lastPollAt: now - 60,
  lastPollOk: true,
  lastPollError: "",
  lastPollInstanceName: "attic",
  lastPollVersion: "v9.2.1",
  lastPollDomains: [],
  remoteViewEnabled: true,
  createdAt: now - 86400,
  sortOrder: 0,
};

async function stage(page: Page, over: Partial<typeof PEER> = {}): Promise<void> {
  await page.route("**/api/settings", async (route) => {
    // Once remote view is open, a forwarded call lands on /api/instances/{id}/
    // settings, which this same glob also catches; that one is meant to be
    // refused by the real server, not patched into a fake success here.
    if (route.request().method() !== "GET" || new URL(route.request().url()).pathname !== "/api/settings") {
      return route.fallback();
    }
    const res = await route.fetch();
    const body = await res.json();
    body.settings = { ...body.settings, fleetEnabled: true };
    await route.fulfill({ response: res, json: body });
  });
  await page.route("**/api/fleet/peers", (route) =>
    route.fulfill({ json: { ok: true, peers: [{ ...PEER, ...over }] } }),
  );
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.addInitScript(() => window.localStorage.setItem("bv-lang", "en"));
}

async function openFleet(page: Page): Promise<void> {
  await stage(page);
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto("/instances#fleet");
  await expect(page.getByRole("heading", { level: 1, name: "Instances" })).toBeVisible();
}

test("Open scopes the app to the peer, the bar names it, and Back leaves it", async ({ page }) => {
  await openFleet(page);

  const openBtn = page.getByRole("button", { name: "Open" });
  await expect(openBtn).toBeEnabled();
  await openBtn.click();

  await expect(page).toHaveURL(/\/dashboard\?.*instance=member-attic/);
  await expect(page.getByText("Viewing attic")).toBeVisible();

  // Local-only surfaces are off the rail while a peer's instance is open.
  // Scoped to the sidebar's own landmark (its footer, Settings included, sits
  // outside the <nav> the destination list itself is): the fresh-install
  // Dashboard also links to Recovery from its own empty-state banner, an
  // unrelated control this check must not trip over.
  const rail = page.getByRole("complementary");
  await expect(rail.getByRole("link", { name: "Settings", exact: true })).toHaveCount(0);
  await expect(rail.getByRole("link", { name: "Recovery", exact: true })).toHaveCount(0);
  await expect(rail.getByRole("link", { name: "Instances", exact: true })).toHaveCount(0);

  // Browsing to another of remote view's own pages keeps the scope; it must
  // not silently drop back to this instance.
  await rail.getByRole("link", { name: "Containers", exact: true }).click();
  await expect(page).toHaveURL(/\/containers\?.*instance=member-attic/);
  await expect(page.getByText("Viewing attic")).toBeVisible();

  await page.getByRole("button", { name: "Back to this instance" }).click();
  await expect(page).toHaveURL(/\/containers(?!\?.*instance=)/);
  await expect(page.getByText("Viewing attic")).toHaveCount(0);
  await expect(rail.getByRole("link", { name: "Settings", exact: true })).toBeVisible();
});

test("a typed URL into a local-only page redirects back to the dashboard while remote", async ({ page }) => {
  await openFleet(page);
  await page.getByRole("button", { name: "Open" }).click();
  await expect(page).toHaveURL(/instance=member-attic/);

  await page.goto("/settings?instance=member-attic&instanceName=attic");
  await expect(page).toHaveURL(/\/dashboard\?.*instance=member-attic/);
});

test("Open is disabled when the peer's own switch is off, with no card left unexplained", async ({ page }) => {
  await openFleet(page);
  await stage(page, { remoteViewEnabled: false });
  await page.reload();
  await expect(page.getByRole("heading", { level: 1, name: "Instances" })).toBeVisible();

  const openBtn = page.getByRole("button", { name: "Open" });
  await expect(openBtn).toBeDisabled();
  // The explanation is an InfoBubble (an (i) icon whose accessible name
  // carries the text, not a paragraph), the same convention as every other
  // control dimmed by a decision made elsewhere.
  await expect(page.getByLabel("Remote view is off on this instance")).toBeVisible();
});

test("the remote-view switch lives in settings, on by default", async ({ page }) => {
  await page.addInitScript(() => window.localStorage.setItem("bv-lang", "en"));
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto("/settings");
  // The switch lives on the System tab, alongside instance name and the
  // dashboard widget, not on the domain-toggle General tab settings opens to.
  await page.getByRole("tab", { name: "System" }).click();

  const toggle = page.getByRole("switch", { name: "Other instances may view this one and start backups" });
  await expect(toggle).toBeVisible();
  await expect(toggle).toHaveAttribute("aria-checked", "true");
});
