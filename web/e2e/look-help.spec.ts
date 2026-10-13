// The theme choice System and the four buttons of the Help card. System has to
// follow the device in a real engine, where the media query and its change
// event exist, and the marks are measured here because their boxes are what
// the eye compares from one button to the next.
import { expect, test, type Locator, type Page } from "@playwright/test";
import { MOBILE_PROJECTS } from "./entries";
import { blockServerLook } from "./serverLook";

const GROUND = { dark: "#161616", light: "#f4f4f4" } as const;
type Scheme = keyof typeof GROUND;

async function stage(page: Page, device: Scheme, stored?: string): Promise<void> {
  // The harness database may hold a look from another spec, and adopting it
  // would overwrite the one under test.
  await blockServerLook(page);
  await page.emulateMedia({ colorScheme: device, reducedMotion: "reduce" });
  await page.addInitScript((theme) => {
    window.localStorage.setItem("bv-lang", "en");
    if (theme) window.localStorage.setItem("bv-theme", theme);
  }, stored);
}

async function expectTheme(page: Page, scheme: Scheme): Promise<void> {
  await expect(page.locator("html")).toHaveAttribute("data-theme", scheme);
  await expect(page.locator('meta[name="theme-color"]')).toHaveAttribute("content", GROUND[scheme]);
}

const themeTab = (page: Page, name: string) =>
  page.getByRole("tablist", { name: "Theme" }).getByRole("tab", { name, exact: true });

for (const device of ["dark", "light"] as const) {
  test(`a browser that never chose starts on System and wears a ${device} device's theme`, async ({ page }) => {
    await stage(page, device);
    await page.goto("/settings/look");
    await expectTheme(page, device);
    await expect(themeTab(page, "System")).toHaveAttribute("aria-selected", "true");
    expect(await page.evaluate(() => window.localStorage.getItem("bv-theme"))).toBeNull();
  });
}

test("the theme tabs read Dark, Light, System", async ({ page }) => {
  await stage(page, "dark");
  await page.goto("/settings/look");
  await expect(page.getByRole("tablist", { name: "Theme" }).getByRole("tab")).toHaveText(["Dark", "Light", "System"]);
});

test("System follows the device while the page stays open", async ({ page }) => {
  await stage(page, "dark");
  await page.goto("/settings/look");
  await expectTheme(page, "dark");
  await page.emulateMedia({ colorScheme: "light" });
  await expectTheme(page, "light");
  await page.emulateMedia({ colorScheme: "dark" });
  await expectTheme(page, "dark");
});

test("a chosen theme holds against the device, and System hands it back", async ({ page }) => {
  await stage(page, "light");
  await page.goto("/settings/look");
  await themeTab(page, "Dark").click();
  await expectTheme(page, "dark");
  await page.emulateMedia({ colorScheme: "dark" });
  await page.emulateMedia({ colorScheme: "light" });
  await expectTheme(page, "dark");

  await themeTab(page, "System").click();
  await expectTheme(page, "light");
  expect(await page.evaluate(() => window.localStorage.getItem("bv-theme"))).toBe("system");

  await page.reload();
  await expectTheme(page, "light");
  await expect(themeTab(page, "System")).toHaveAttribute("aria-selected", "true");
});

for (const [stored, device] of [
  ["dark", "light"],
  ["light", "dark"],
] as const) {
  test(`an install that chose ${stored} keeps it on a ${device} device`, async ({ page }) => {
    await stage(page, device, stored);
    await page.goto("/settings/look");
    await expectTheme(page, stored);
    await expect(themeTab(page, stored === "dark" ? "Dark" : "Light")).toHaveAttribute("aria-selected", "true");
  });
}

interface Placed {
  name: string;
  /** The mark's box, the centre of its ink and the ink's longer side, from the
   *  button's own corner. */
  mark: { x: number; y: number; w: number; h: number };
  ink: { cx: number; cy: number; long: number };
  /** Where the words start and the middle of the lines on show. A touch screen
   *  shows the second line at rest, a pointer only brings it in. */
  text: { x: number; cy: number };
  viewBox: string;
}

function placed(card: Locator): Promise<Placed[]> {
  return card.locator(".glim-readme-btn-unit").evaluateAll((units) =>
    units.map((unit) => {
      const u = unit.getBoundingClientRect();
      const mark = unit.querySelector(".glim-readme-btn-mark")!;
      const m = mark.getBoundingClientRect();
      const svg = [...mark.querySelectorAll("svg")].find((s) => getComputedStyle(s).display !== "none")!;
      const shapes = [...svg.querySelectorAll("path, rect, circle")].map((s) => s.getBoundingClientRect());
      const left = Math.min(...shapes.map((r) => r.left));
      const right = Math.max(...shapes.map((r) => r.right));
      const top = Math.min(...shapes.map((r) => r.top));
      const bottom = Math.max(...shapes.map((r) => r.bottom));
      const name = unit.querySelector(".glim-readme-btn-name")!;
      const n = name.getBoundingClientRect();
      const sub = unit.querySelector(".glim-readme-btn-sub");
      const last = sub && getComputedStyle(sub).opacity !== "0" ? sub.getBoundingClientRect() : n;
      return {
        name: name.textContent ?? "",
        mark: { x: m.x - u.x, y: m.y - u.y, w: m.width, h: m.height },
        ink: { cx: (left + right) / 2 - u.x, cy: (top + bottom) / 2 - u.y, long: Math.max(right - left, bottom - top) },
        text: { x: n.x - u.x, cy: (n.top + last.bottom) / 2 - u.y },
        viewBox: svg.getAttribute("viewBox") ?? "",
      };
    }),
  );
}

async function openHelp(page: Page, theme: Scheme): Promise<Locator> {
  await stage(page, theme, theme);
  await page.goto("/settings/info");
  const card = page.locator('[data-search-card="Help"]');
  await card.scrollIntoViewIfNeeded();
  await expect(card.locator(".glim-readme-btn-unit")).toHaveCount(4);
  return card;
}

const CLOSED_BOOK = "0 0 14 14";
const OPEN_BOOK = "0 0.125 14 14";

for (const theme of ["dark", "light"] as const) {
  test(`the Help card's four marks share one box, one size and one place, ${theme}`, async ({ page }) => {
    const card = await openHelp(page, theme);
    const four = await placed(card);
    expect(four.map((b) => b.name)).toEqual(["GitHub", "Email", "Manual", "Bug report"]);

    const [first, ...rest] = four;
    for (const b of rest) {
      const where = `${b.name} against ${first.name}: ${JSON.stringify([b, first])}`;
      for (const side of ["x", "y", "w", "h"] as const) expect(b.mark[side], where).toBeCloseTo(first.mark[side], 1);
      expect(b.ink.cx, where).toBeCloseTo(first.ink.cx, 1);
      expect(b.ink.cy, where).toBeCloseTo(first.ink.cy, 1);
      expect(b.ink.long, where).toBeCloseTo(first.ink.long, 1);
      expect(b.text.x, where).toBeCloseTo(first.text.x, 1);
      expect(b.text.cy, where).toBeCloseTo(first.text.cy, 1);
    }
  });
}

test("the manual's book is closed at rest and open on the focus, in the same place", async ({ page }) => {
  const card = await openHelp(page, "dark");
  const manual = card.getByRole("button", { name: /Manual/ });
  const rest = (await placed(card))[2];
  expect(rest.viewBox).toBe(CLOSED_BOOK);

  await card.getByRole("button", { name: /Email/ }).focus();
  await page.keyboard.press("Tab");
  await expect(manual).toBeFocused();
  const lit = (await placed(card))[2];
  expect(lit.viewBox).toBe(OPEN_BOOK);
  expect(lit.ink.cx).toBeCloseTo(rest.ink.cx, 1);
  expect(lit.ink.cy).toBeCloseTo(rest.ink.cy, 1);
  expect(lit.ink.long).toBeCloseTo(rest.ink.long, 1);
});

test("the manual's book opens under the pointer", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "desktop-only: nothing hovers on a touch screen");
  const card = await openHelp(page, "light");
  await card.getByRole("button", { name: /Manual/ }).hover();
  expect((await placed(card))[2].viewBox).toBe(OPEN_BOOK);
  await page.mouse.move(0, 0);
  expect((await placed(card))[2].viewBox).toBe(CLOSED_BOOK);
});

test("the manual's book opens under a finger", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "touch-only: the press answers where nothing hovers");
  const card = await openHelp(page, "dark");
  const box = (await card.getByRole("button", { name: /Manual/ }).boundingBox())!;
  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
  await page.mouse.down();
  expect((await placed(card))[2].viewBox).toBe(OPEN_BOOK);
  // Let go off the button, so the press opens no window.
  await page.mouse.move(0, 0);
  await page.mouse.up();
  expect((await placed(card))[2].viewBox).toBe(CLOSED_BOOK);
});
