// Whatever the pointer is over lights up in one colour, the accent's hue on
// surface3's lightness (GlimStone rule 21): a button, a row, a menu entry and a
// tile alike.
import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { DEFAULT_ACCENT_PRESETS } from "../lib/accent";
import { MIN_TEXT_CONTRAST, contrastRatio } from "../lib/contrast";
import { oklabOf, srgbOf } from "../lib/oklab";

const SRC = join(dirname(fileURLToPath(import.meta.url)), "..");
const stylesheet = (path: string) => readFileSync(path, "utf8").replace(/\/\*[\s\S]*?\*\//g, "");

// A selector starts after the last at-rule statement before it, such as @import.
const rulesOf = (css: string) =>
  [...css.matchAll(/([^{}]+)\{([^{}]*)\}/g)].map(([, selector, body]) => ({ selector: selector.split(";").pop()!.trim(), body }));
const rules = rulesOf(stylesheet(join(SRC, "index.css")));
// BombVault ports GlimStone's stylesheet instead of loading it, so the port is
// held to the release it names.
const gsRules = rulesOf(stylesheet(join(SRC, "..", "node_modules", "@glimstone", "core", "tokens.css")));

/** The custom properties of the first rule whose selector is exactly `selector`. */
function block(selector: string, from = rules): Record<string, string> {
  const rule = from.find((r) => r.selector === selector);
  expect(rule, `no rule ${selector}`).toBeTruthy();
  return Object.fromEntries([...rule!.body.matchAll(/(--[\w-]+)\s*:\s*([^;]+);/g)].map((m) => [m[1], m[2].trim()]));
}

const GS_HOVER = block(":root,\n[data-theme],\n[data-rainbow] .glim-hue", gsRules);
const GS_OLED_HOVER = block("[data-oled],\n[data-oled][data-rainbow] .glim-hue", gsRules)["--hover"];
const lightness = (hover: string) => [...hover.matchAll(/oklab\(from var\(--accent\) ([\d.]+) /g)].map((m) => Number(m[1]));

// GlimStone's lightness for the hover: light, dark and OLED.
const STEP = [...lightness(GS_HOVER["--hover"]), lightness(GS_OLED_HOVER)[1]];
const CHROMA = Number(GS_HOVER["--hover-chroma"]);

// What the colour picker can hand in, GlimStone's sweep: twelve hues at low,
// mid and high chroma on four lightness steps, and a run of greys.
const SWEEP = [
  ...[0.45, 0.6, 0.75, 0.9].flatMap((l) =>
    [0.02, 0.08, 0.2].flatMap((c) => Array.from({ length: 12 }, (_, i) => srgbOf([l, c * Math.cos((i * Math.PI) / 6), c * Math.sin((i * Math.PI) / 6)]))),
  ),
  ...["#222222", "#555555", "#6B6B6B", "#777777", "#7B7B7B", "#888888", "#999999", "#BBBBBB", "#DDDDDD"],
];

describe("the hover", () => {
  it("is GlimStone's one colour, declared again inside a rainbow position", () => {
    const own = block(":root,\n[data-theme],\n[data-rainbow] .glim-hue");
    expect(own["--hover-chroma"]).toBe(GS_HOVER["--hover-chroma"]);
    expect(own["--hover"]).toBe(GS_HOVER["--hover"]);
    expect(STEP).toHaveLength(3);
    expect(STEP.every((l) => l > 0 && l < 1) && CHROMA > 0).toBe(true);
    expect(Object.keys(own).filter((k) => k.startsWith("--hover"))).toEqual(["--hover-chroma", "--hover"]);
  });

  it("moves down with OLED, with or without data-theme", () => {
    for (const selector of [
      '[data-theme="dark"][data-oled],\n[data-theme="dark"][data-oled][data-rainbow] .glim-hue',
      ':root[data-oled]:not([data-theme]),\n  :root[data-oled]:not([data-theme])[data-rainbow] .glim-hue',
    ]) {
      expect(block(selector)).toEqual({ "--hover": GS_OLED_HOVER });
    }
  });

  it("follows a rainbow position as a Tailwind colour too", () => {
    for (const selector of ["@theme", "[data-rainbow] .glim-hue"]) {
      const own = block(selector);
      expect(own["--color-hover"]).toBe("var(--hover)");
      expect(Object.keys(own).filter((k) => k.startsWith("--color-hover"))).toEqual(["--color-hover"]);
    }
  });

  it("keeps the text and the secondary text at 4.5:1 for every picked accent in every theme", () => {
    const themes = [
      { name: "light", side: 0, text: "#161616", sub: "#525252" },
      { name: "dark", side: 1, text: "#f4f4f4", sub: "#c6c6c6" },
      { name: "OLED", side: 2, text: "#f4f4f4", sub: "#c6c6c6" },
    ];
    const misses: string[] = [];
    for (const accent of [...DEFAULT_ACCENT_PRESETS, ...SWEEP]) {
      const [, a, b] = oklabOf(accent);
      for (const { name, side, text, sub } of themes) {
        const ground = srgbOf([STEP[side], a * CHROMA, b * CHROMA]);
        for (const ink of [text, sub]) {
          const r = contrastRatio(ink, ground);
          if (r < MIN_TEXT_CONTRAST) misses.push(`${accent} ${ink} on ${name}: ${r.toFixed(2)}`);
        }
      }
    }
    expect(misses).toEqual([]);
  });
});

// The one hover ground, and the grounds that come from the accent, a rainbow
// position, a brand tile or a state hue.
const LIT = /var\(--(hover|accent[\w-]*|item-hue[\w-]*|tile|status-[\w-]+)\)/;
const SYSTEM = /^(Highlight|HighlightText|ButtonFace|ButtonText|Canvas|CanvasText|LinkText|GrayText|SelectedItem)$/;
// A chip's dismiss control takes the pill's own ink, accent or neutral.
const OWN_INK = new Set([".glim-btn-chip:hover:not(:disabled)"]);

/** Every class in the components that names a hover ground, with where it stands. */
function hoverGrounds(): { at: string; cls: string }[] {
  const found: { at: string; cls: string }[] = [];
  const walk = (dir: string) => {
    for (const name of readdirSync(dir)) {
      const path = join(dir, name);
      if (statSync(path).isDirectory()) walk(path);
      else if (/\.tsx?$/.test(name) && !/\.test\.tsx?$/.test(name)) {
        const text = readFileSync(path, "utf8");
        for (const m of text.matchAll(/(?<![\w-])((?:[\w-]+:)*hover:bg-(?:\[[^\]\s]+\]|[\w-]+))/g)) {
          found.push({ at: `${path.slice(SRC.length + 1).replace(/\\/g, "/")}:${text.slice(0, m.index).split("\n").length}`, cls: m[1] });
        }
      }
    }
  };
  walk(SRC);
  return found;
}

describe("every hover", () => {
  it("lights its ground with the accent in the stylesheet", () => {
    const hovered = rules.filter(({ selector }) => /:hover|\[data-highlighted\]/.test(selector.replace(/:not\([^()]*\)/g, "")));
    expect(hovered.length).toBeGreaterThan(20);
    const grey = hovered.flatMap(({ selector, body }) =>
      [...body.matchAll(/(?<![\w-])background(?:-color)?\s*:\s*([^;]+);/g)]
        .map((m) => m[1].trim())
        .filter((v) => v !== "transparent" && !SYSTEM.test(v) && !LIT.test(v) && !OWN_INK.has(selector.replace(/\s+/g, " ")))
        .map((v) => `${selector.replace(/\s+/g, " ")}: ${v}`),
    );
    expect(grey).toEqual([]);
  });

  it("gives a control without a ground of its own the accent ink in the stylesheet", () => {
    const grey = rules
      .filter(({ selector, body }) => /:hover/.test(selector.replace(/:not\([^()]*\)/g, "")) && !/background/.test(body))
      .flatMap(({ selector, body }) =>
        [...body.matchAll(/(?<![\w-])color\s*:\s*([^;]+);/g)].filter((m) => /var\(--carbon-/.test(m[1])).map((m) => `${selector.replace(/\s+/g, " ")}: ${m[1].trim()}`),
      );
    expect(grey).toEqual([]);
  });

  it("lights its ground with the one hover colour in a component", () => {
    const found = hoverGrounds();
    expect(found.length).toBeGreaterThan(20);
    // A disabled control keeps its resting fill under the pointer, the accent
    // and state fills and a state badge step along their own colour, and the
    // bulk bar's buttons stand on the accent, so they darken it instead.
    const other = found.filter(
      ({ at, cls }) =>
        !/(^|:)disabled:/.test(cls) &&
        !/hover:bg-(hover|accent[\w-]*|status\w*Hover|\[color-mix\(in_srgb,var\(--(status|accent)-[^\]]+\])$/.test(cls) &&
        !at.startsWith("components/backups/BulkBar.tsx:"),
    );
    expect(other.map(({ at, cls }) => `${at} ${cls}`)).toEqual([]);
  });

  it("fills a row's glyph tile with the accent when the tile stands in the row's button", () => {
    const lit = rules.find((r) => r.selector.includes(".glim-hue:is(:hover, :focus-within, [data-picked]) > button:not(:disabled) > .glim-row-glyph"));
    expect(lit?.body).toMatch(/background-color:\s*var\(--accent\)/);
  });
});

describe("a button and a row under the pointer", () => {
  const source = (file: string) => readFileSync(join(SRC, file), "utf8");

  it("take the same colour", () => {
    const grounds = rules
      .filter((r) => /^:is\(\.glim-btn-neutral, \.glim-btn-subtle\):not\(:disabled\):(hover|focus-visible)$/.test(r.selector))
      .map((r) => r.body.match(/background-color:\s*([^;]+);/)?.[1].trim());
    expect(grounds).toEqual(["var(--hover)", "var(--hover)"]);
    const tones = source("components/Button.tsx").match(/neutral: "([^"]+)",\n\s*subtle: "([^"]+)"/);
    expect(tones?.slice(1).map((c) => c.match(/hover:bg-[\w-]+/)?.[0])).toEqual(["hover:bg-hover", "hover:bg-hover"]);
    const row = source("pages/Restore.tsx").match(/data-new=\{newId\}\s*className="([^"]+)"/)?.[1];
    expect(row?.match(/hover:bg-[\w-]+/)?.[0]).toBe("hover:bg-hover");
  });
});
