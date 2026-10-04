// The placement row in the Settings defaults card, with a target and a
// destination staged at the route layer. Lokal, the target and the
// destination share one row at every width, every label whole, and the row
// hugs its buttons. A screen reader hears a group of toggle buttons, and the
// keyboard moves along it without choosing until Space. The location field
// under it and the apply button keep their words whole.
// German, because its labels run longest.
import { expect, test, type Locator, type Page } from "@playwright/test";

const DOMAINS = ["containers", "vms", "files"] as const;

function defaultRow(domain: string) {
  return {
    domain,
    exists: true,
    home: "",
    homeKind: "domain",
    homeOff: false,
    skip: [],
    paused: false,
    confirmedAt: 1_758_170_400,
    counts: { follow: 3, own: 0, open: 0, chosenNoRun: 0 },
    unreadable: false,
  };
}

function options(domain: string) {
  return {
    domain,
    unreadable: false,
    paused: false,
    homes: [
      { id: "", name: "", location: `backups/${domain}`, kind: "domain", scheme: "" },
      { id: "r-nas", name: "NAS Keller", location: "/mnt/remotes/nas/bombvault", kind: "local", scheme: "" },
    ],
    targets: [{ id: "t-b2", name: "B2", enabled: true, primary: true, appendOnly: false, hint: "" }],
    sendTo: [{ kind: "direct", repoId: "", targetId: "t-b2", name: "B2", location: "" }],
    segmentLocks: {},
    default: defaultRow(domain),
  };
}

const WASABI = {
  id: "d-wasabi",
  name: "Wasabi",
  provider: "wasabi",
  repo: "s3:https://s3.wasabisys.com/bombvault",
  credsRef: "",
  storageClass: "",
  immutable: false,
  createdAt: 1_758_170_400,
  domains: [],
};

/** Stages the defaults and returns the default changes the page asked for. */
async function stage(page: Page): Promise<string[]> {
  const asked: string[] = [];
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.addInitScript(() => window.localStorage.setItem("bv-lang", "de"));
  await page.route("**/api/placement/defaults", (route) =>
    route.fulfill({ json: { ok: true, defaults: DOMAINS.map(defaultRow) } }),
  );
  await page.route("**/api/placement/options*", (route) => {
    const domain = new URL(route.request().url()).searchParams.get("domain") ?? "containers";
    return route.fulfill({ json: { ok: true, options: options(domain) } });
  });
  await page.route("**/api/offsite/destinations", (route) => route.fulfill({ json: { ok: true, destinations: [WASABI] } }));
  await page.route("**/api/placement/default/**", (route) => {
    asked.push(route.request().url());
    return route.fulfill({ json: { ok: false, error: "staged" } });
  });
  return asked;
}

async function containersRow(page: Page): Promise<Locator> {
  await page.goto("/settings#storage");
  const row = page.getByRole("group", { name: "Ablage" }).first();
  await expect(row).toBeVisible();
  await expect(row.getByRole("button")).toHaveCount(3);
  return row;
}

test("the placement row keeps its buttons on one row, every label whole and inside the window", async ({ page }) => {
  await stage(page);
  const row = await containersRow(page);

  const tops = await row.getByRole("button").evaluateAll((els) => els.map((el) => Math.round(el.getBoundingClientRect().top)));
  expect(new Set(tops).size, JSON.stringify(tops)).toBe(1);

  const cut = await row.locator("[data-sel-label]").evaluateAll((els) =>
    els.filter((el) => el.scrollWidth > el.clientWidth + 1).map((el) => el.textContent),
  );
  expect(cut, "labels cut off").toEqual([]);

  const vw = page.viewportSize()!.width;
  const past = await row.getByRole("button").evaluateAll(
    (els, width) => els.filter((el) => el.getBoundingClientRect().right > width + 1).map((el) => el.textContent),
    vw,
  );
  expect(past, "buttons past the window").toEqual([]);
});

test("the keyboard moves along the placement row and chooses on Space", async ({ page }) => {
  const asked = await stage(page);
  const row = await containersRow(page);
  const local = row.getByRole("button", { name: "Lokal", exact: true });
  const b2 = row.getByRole("button", { name: "B2", exact: true });
  await expect(local).toHaveAttribute("aria-pressed", "true");
  await expect(b2).toHaveAttribute("aria-pressed", "true");

  await local.focus();
  await page.keyboard.press("ArrowRight");
  await expect(b2).toBeFocused();
  await expect(b2).toHaveAttribute("aria-pressed", "true");
  expect(asked).toEqual([]);

  await page.keyboard.press("Space");
  await expect.poll(() => asked.length).toBeGreaterThan(0);
});

test("the row offers the destination unpressed beside the target and hugs its buttons", async ({ page }) => {
  await stage(page);
  const row = await containersRow(page);
  const wasabi = row.getByRole("button", { name: "Wasabi", exact: true });
  await expect(wasabi).toHaveAttribute("aria-pressed", "false");
  const [rowBox, b2Box, wasabiBox] = [
    await row.boundingBox(),
    await row.getByRole("button", { name: "B2", exact: true }).boundingBox(),
    await wasabi.boundingBox(),
  ];
  expect(wasabiBox!.x, "the destination comes after the target").toBeGreaterThan(b2Box!.x + b2Box!.width - 1);
  const buttons = await row.getByRole("button").evaluateAll((els) => els.reduce((sum, el) => sum + el.getBoundingClientRect().width, 0));
  expect(rowBox!.width, "the track hugs its buttons").toBeLessThan(buttons + 20);
});

test("the default's location and its apply button keep their words whole", async ({ page }) => {
  await stage(page);
  await containersRow(page);
  const card = page.locator("section").filter({ has: page.getByRole("group", { name: "Ablage" }) }).first();
  await expect(card.getByRole("combobox", { name: "Gespeichert auf" })).toBeVisible();
  const cut = await card.evaluate((root) =>
    [...root.querySelectorAll("[role='combobox'] span:not(.invisible), .glim-btn-label")]
      .filter((el) => el.getBoundingClientRect().width > 0 && (el.scrollWidth > el.clientWidth + 1 || el.scrollHeight > el.clientHeight + 1))
      .map((el) => el.textContent),
  );
  expect(cut, "words cut short").toEqual([]);
  const vw = page.viewportSize()!.width;
  const past = await card.evaluate(
    (root, width) => [...root.querySelectorAll("button, [role='combobox']")].filter((el) => el.getBoundingClientRect().right > width + 1).length,
    vw,
  );
  expect(past, "controls past the window").toBe(0);
});
