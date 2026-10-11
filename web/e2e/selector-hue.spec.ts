// Under rainbow every selector that sets a value wears a colour of its own: the
// selectors of a page count through the palette in the order they stand, the
// chosen segment is filled with its selector's colour and carries the ink that
// fill needs, and the segments not chosen stay without colour. A tab strip
// keeps a colour per tab. Measured in the browser, at rest, under the pointer
// and under focus, in both themes and in the reactive mode.
import { expect, test, type Page } from "@playwright/test";

import { EVERY_KIND, settle, stageEntries } from "./entries";

async function look(page: Page, theme: "dark" | "light", reactive = false): Promise<void> {
  // The stored look is the one under test, so the server's must not replace it.
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.addInitScript(
    ([theme, reactive]) => {
      window.localStorage.setItem("bv-theme", theme as string);
      window.localStorage.setItem("bv-rainbow", JSON.stringify({ on: true, reactive }));
    },
    [theme, reactive] as const,
  );
}

interface Strip {
  hue: string;
  chosen: { bg: string; ink: string }[];
  idle: { bg: string; ink: string; glyph: string | null }[];
}

/** Every counted selector on the page, with the colours of its segments. */
function strips(page: Page): Promise<Strip[]> {
  return page.evaluate(() =>
    Array.from(document.querySelectorAll<HTMLElement>("#bv-main [data-hue]")).map((track) => {
      const segs = Array.from(track.querySelectorAll<HTMLElement>("[data-sel-id]"));
      const paint = (el: HTMLElement) => {
        const style = getComputedStyle(el);
        const glyph = el.querySelector("svg");
        return { bg: style.backgroundColor, ink: style.color, glyph: glyph ? getComputedStyle(glyph).color : null };
      };
      return {
        hue: track.dataset.hue!,
        chosen: segs.filter((s) => s.classList.contains("glim-active")).map(paint),
        idle: segs.filter((s) => !s.classList.contains("glim-active")).map(paint),
      };
    }),
  );
}

function luminance(rgb: string): number {
  const [r, g, b] = (rgb.match(/[\d.]+/g) ?? []).slice(0, 3).map((v) => {
    const c = Number(v) / 255;
    return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function contrast(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}

const TRANSPARENT = "rgba(0, 0, 0, 0)";

for (const theme of ["dark", "light"] as const) {
  test(`${theme}: the selectors of a page count through the palette, chosen in their colour and idle without`, async ({ page }, testInfo) => {
    test.skip(testInfo.project.name !== "desktop-1280", "the colours are the same at every width");
    await look(page, theme);
    await page.goto("/settings/look");
    await expect(page.locator("#bv-main [data-hue]").nth(6)).toBeAttached();
    await settle(page);

    const found = await strips(page);
    expect(found.map((s) => s.hue)).toEqual(found.map((_, i) => String(i)));
    const fills = found.map((s) => s.chosen[0].bg);
    // Eight colours in the palette, so no two of the first eight repeat.
    expect(new Set(fills.slice(0, 8)).size).toBe(Math.min(found.length, 8));
    for (const s of found) {
      for (const seg of s.chosen) {
        expect(seg.bg).not.toBe(TRANSPARENT);
        expect(contrast(seg.bg, seg.ink), `ink on ${seg.bg}`).toBeGreaterThanOrEqual(4.5);
      }
      for (const seg of s.idle) {
        expect(seg.bg).toBe(TRANSPARENT);
        if (seg.glyph) expect(seg.glyph).toBe(seg.ink);
      }
    }

    // Under the pointer an idle segment takes the plain hover, not a colour.
    const idle = page.locator("#bv-main [data-hue] [data-sel-id]:not(.glim-active)").first();
    const before = await idle.evaluate((el) => getComputedStyle(el.closest("[data-hue]")!).getPropertyValue("--item-hue"));
    await idle.hover();
    await expect
      .poll(() => idle.evaluate((el) => getComputedStyle(el).backgroundColor))
      .toBe(await page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue("--carbon-hover").trim()).then((hex) => {
        const n = parseInt(hex.slice(1), 16);
        return `rgb(${(n >> 16) & 255}, ${(n >> 8) & 255}, ${n & 255})`;
      }));
    expect(before).not.toBe("");

    // The focus ring is the selector's colour, not the single accent's.
    await page.mouse.move(0, 0);
    const first = page.locator("#bv-main [data-hue]").nth(1).locator("[data-sel-id].glim-active");
    await first.focus();
    await page.keyboard.press("Shift+Tab");
    await page.keyboard.press("Tab");
    const ring = await first.evaluate((el) => getComputedStyle(el).outlineColor);
    const flat = await page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue("--focus-ring-flat").trim());
    expect(ring).not.toBe(flat);
  });
}

test("reactive: a chosen segment keeps its colour at rest, and an idle one gets none", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop-1280", "the colours are the same at every width");
  await look(page, "dark", true);
  await page.goto("/settings/look");
  await expect(page.locator("#bv-main [data-hue]").nth(3)).toBeAttached();
  await settle(page);
  const found = await strips(page);
  expect(new Set(found.slice(0, 4).map((s) => s.chosen[0].bg)).size).toBe(4);
  for (const s of found) {
    for (const seg of s.chosen) expect(contrast(seg.bg, seg.ink)).toBeGreaterThanOrEqual(4.5);
    for (const seg of s.idle) expect(seg.bg).toBe(TRANSPARENT);
  }
});

test("a tab strip keeps a colour per tab", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop-1280", "the colours are the same at every width");
  await stageEntries(page, EVERY_KIND);
  await look(page, "dark");
  await page.goto("/backups/container/immich");
  const tabs = page.getByRole("tablist", { name: "immich" }).getByRole("tab");
  await expect(tabs).toHaveCount(3);
  const glyphs = await tabs.evaluateAll((els) => els.map((el) => getComputedStyle(el.querySelector("svg")!).color));
  expect(new Set(glyphs).size).toBe(3);
  await expect(page.getByRole("tablist", { name: "immich" })).not.toHaveAttribute("data-hue");
});
