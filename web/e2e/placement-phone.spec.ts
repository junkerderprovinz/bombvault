// The placement row in the Settings defaults card, with a target staged at the
// route layer. Below 600px its three segments sit in two rows, two over one,
// every label whole; wider, they share one row. A screen reader hears a radio
// group, and the keyboard moves along it without choosing until Space. The
// copy chips under it keep to their segments beside their caption, and the
// location field and the apply button keep their words whole.
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
  await page.route("**/api/placement/default/**", (route) => {
    asked.push(route.request().url());
    return route.fulfill({ json: { ok: false, error: "staged" } });
  });
  return asked;
}

async function containersRow(page: Page): Promise<Locator> {
  await page.goto("/settings#storage");
  const row = page.getByRole("radiogroup", { name: "Ablage" }).first();
  await expect(row).toBeVisible();
  await expect(row.getByRole("radio")).toHaveCount(3);
  return row;
}

test("the placement row takes two rows below 600px and one above, every label whole", async ({ page }) => {
  await stage(page);
  const row = await containersRow(page);
  const narrow = page.viewportSize()!.width < 600;

  const tops = await row.getByRole("radio").evaluateAll((els) => els.map((el) => Math.round(el.getBoundingClientRect().top)));
  expect(new Set(tops).size, JSON.stringify(tops)).toBe(narrow ? 2 : 1);
  if (narrow) expect(tops[0], "Lokal and Lokal + Off-site share the first row").toBe(tops[1]);

  const cut = await row.locator("[data-sel-label]").evaluateAll((els) =>
    els.filter((el) => el.scrollWidth > el.clientWidth + 1).map((el) => el.textContent),
  );
  expect(cut, "labels cut off").toEqual([]);
});

test("the keyboard moves along the placement row and chooses on Space", async ({ page }) => {
  const asked = await stage(page);
  const row = await containersRow(page);
  const local = row.getByRole("radio", { name: "Lokal", exact: true });
  const both = row.getByRole("radio", { name: "Lokal + Off-site" });
  await expect(both).toHaveAttribute("aria-checked", "true");

  await both.focus();
  await page.keyboard.press("ArrowLeft");
  await expect(local).toBeFocused();
  await expect(local).toHaveAttribute("aria-checked", "false");
  expect(asked).toEqual([]);

  await page.keyboard.press("Space");
  await expect.poll(() => asked.length).toBeGreaterThan(0);
});

test("the copy chips keep to their segments beside their caption", async ({ page }) => {
  await stage(page);
  await containersRow(page);
  const chips = page.getByRole("group", { name: "Kopie nach" }).first();
  await expect(chips).toBeVisible();
  const caption = page.getByText("Kopie nach", { exact: true }).first();
  const [chipBox, captionBox] = [await chips.boundingBox(), await caption.boundingBox()];
  expect(chipBox!.x, "the chips start after the caption").toBeGreaterThan(captionBox!.x + captionBox!.width);
  const segments = await chips.getByRole("button").evaluateAll((els) => els.reduce((sum, el) => sum + el.getBoundingClientRect().width, 0));
  expect(chipBox!.width, "the track hugs its chip").toBeLessThan(segments + 20);
});

test("the default's location and its apply button keep their words whole", async ({ page }) => {
  await stage(page);
  await containersRow(page);
  const card = page.locator("section").filter({ has: page.getByRole("radiogroup", { name: "Ablage" }) }).first();
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
