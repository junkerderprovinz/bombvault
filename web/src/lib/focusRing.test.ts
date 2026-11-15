// The focus ring is the accent's ink, opaque, so it follows a picked accent and
// a rainbow position. It stands on the ground around a control: the page, the
// rail, a card, a well or a row's hover. Surface3 is a field's fill, and a field
// shows its focus with a ring and a fill of its own.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { DEFAULT_ACCENT_PRESETS } from "./accent";
import { hueVars, RAINBOW } from "./appearance";
import { accentInks, contrastRatio, MIN_TEXT_CONTRAST } from "./contrast";
import { darkenTo, darkInkCss, inkCss, INK_LIGHTNESS, lightenTo, lightInkCss } from "./oklab";

const CSS = readFileSync(join(dirname(fileURLToPath(import.meta.url)), "..", "index.css"), "utf8");
const MIN_GRAPHIC_CONTRAST = 3;
const RING_GROUNDS = ["carbon-bg", "carbon-sidebar", "carbon-surface", "carbon-surface2", "carbon-hover"];
const PICKED = ["#FFFF00", "#0000FF", "#777777", "#FAFAFA", "#050505"];

/** The body of the first rule whose selector is exactly `selector`, with the
 *  space after each property name evened out. */
function block(selector: string): string {
  const start = CSS.indexOf(`\n${selector} {\n`);
  if (start < 0) throw new Error(`no block ${selector}`);
  return CSS.slice(start, CSS.indexOf("\n}\n", start)).replace(/(--[\w-]+):\s+/g, "$1: ");
}

function tokens(body: string): Record<string, string> {
  return Object.fromEntries([...body.matchAll(/--([\w-]+):\s*(#[0-9a-fA-F]{6});/g)].map((m) => [m[1], m[2]]));
}

const DARK_BLOCK = ':root,\n[data-theme="dark"]';
const LIGHT_BLOCK = '[data-theme="light"]';
const darkInk = (hex: string) => accentInks(hex).dark ?? lightenTo(hex, INK_LIGHTNESS.dark);
const lightInk = (hex: string) => accentInks(hex).light ?? darkenTo(hex, INK_LIGHTNESS.light);
const THEMES = {
  dark: { grounds: tokens(block(DARK_BLOCK)), ink: darkInk, step: (hue: string) => lightenTo(hue, INK_LIGHTNESS.dark) },
  OLED: {
    grounds: { ...tokens(block(DARK_BLOCK)), ...tokens(block('[data-theme="dark"][data-oled]')) },
    ink: darkInk,
    step: (hue: string) => lightenTo(hue, INK_LIGHTNESS.dark),
  },
  light: { grounds: tokens(block(LIGHT_BLOCK)), ink: lightInk, step: (hue: string) => darkenTo(hue, INK_LIGHTNESS.light) },
};

describe("the focus ring", () => {
  it("is the accent's ink in every theme block, which derives it on the OKLab steps", () => {
    const flat = [...CSS.matchAll(/--focus-ring(?:-flat)?:\s*([^;]+);/g)]
      .map((m) => m[1].trim())
      .filter((value) => !value.includes("--item-hue-ring"));
    expect(flat.length).toBeGreaterThan(0);
    for (const value of flat) expect(value).toBe("var(--accent-text)");
    expect(block(DARK_BLOCK)).toContain(`--accent-text: var(--accent-ink-dark, ${darkInkCss("var(--accent)")});`);
    expect(block(LIGHT_BLOCK)).toContain(`--accent-text: var(--accent-ink-light, ${lightInkCss("var(--accent)")});`);
    expect(CSS).not.toContain("--ink-mix");
  });

  for (const [theme, { grounds, ink, step }] of Object.entries(THEMES)) {
    it(`reaches 3:1 around a control for every preset and picked accent on ${theme}`, () => {
      const misses = [...DEFAULT_ACCENT_PRESETS, ...PICKED].flatMap((hex) =>
        RING_GROUNDS.map((g) => [g, contrastRatio(ink(hex), grounds[g])] as const)
          .filter(([, r]) => r < MIN_GRAPHIC_CONTRAST)
          .map(([g, r]) => `${hex} on ${g}: ${r.toFixed(2)}:1`),
      );
      expect(misses).toEqual([]);
    });

    it(`gives every rainbow position a ring at 3:1 and a glyph at 4.5:1 on ${theme}`, () => {
      const misses = RAINBOW.flatMap((hue) => [
        ...RING_GROUNDS.filter((g) => contrastRatio(step(hue), grounds[g]) < MIN_GRAPHIC_CONTRAST).map((g) => `${hue} ring on ${g}`),
        ...(contrastRatio(step(hue), grounds["carbon-surface"]) < MIN_TEXT_CONTRAST ? [`${hue} glyph on the card`] : []),
      ]);
      expect(misses).toEqual([]);
    });
  }

  it("takes a rainbow position's ink for its ring", () => {
    expect(hueVars(5)["--item-hue-glyph"]).toBe(inkCss("var(--rb-5)"));
    expect(hueVars(5)["--item-hue-ring"]).toBe("var(--item-hue-glyph)");
  });
});
