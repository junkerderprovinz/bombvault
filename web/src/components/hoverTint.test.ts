// Whatever the pointer is over lights up in the accent (GlimStone rule 21).
// Each tint keeps the OKLab lightness of the grey step it replaces and takes
// 30% of the accent's chroma, so the text on it reads as it did on the grey.
import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { DEFAULT_ACCENT_PRESETS } from "../lib/accent";
import { MIN_TEXT_CONTRAST, accentInks, contrastRatio } from "../lib/contrast";
import { INK_LIGHTNESS, darkenTo, lightenTo, oklabOf, srgbOf } from "../lib/oklab";

const SRC = join(dirname(fileURLToPath(import.meta.url)), "..");
const CSS = readFileSync(join(SRC, "index.css"), "utf8").replace(/\/\*[\s\S]*?\*\//g, "");

// A selector starts after the last at-rule statement before it, such as @import.
const rules = [...CSS.matchAll(/([^{}]+)\{([^{}]*)\}/g)].map(([, selector, body]) => ({ selector: selector.split(";").pop()!.trim(), body }));

/** The custom properties of the first rule whose selector is exactly `selector`. */
function block(selector: string): Record<string, string> {
  const rule = rules.find((r) => r.selector === selector);
  expect(rule, `no rule ${selector}`).toBeTruthy();
  return Object.fromEntries([...rule!.body.matchAll(/(--[\w-]+)\s*:\s*([^;]+);/g)].map((m) => [m[1], m[2].trim()]));
}

const tint = (l: number) => `oklab(from var(--accent) ${l} calc(a * var(--hover-chroma)) calc(b * var(--hover-chroma)))`;
const both = (light: number, dark: number) => `light-dark(${tint(light)}, ${tint(dark)})`;

// GlimStone's lightness steps: light, dark and OLED.
const STEPS = {
  "--hover": [0.907, 0.329, 0.256],
  "--hover-filled": [0.861, 0.432, 0.338],
  "--hover-raised": [0.827, 0.535, 0.432],
} as const;

describe("the hover tints", () => {
  it("are GlimStone's, declared again inside a rainbow position", () => {
    const own = block(":root,\n[data-theme],\n[data-rainbow] .glim-hue");
    expect(own["--hover-chroma"]).toBe("0.3");
    for (const [token, [light, dark]] of Object.entries(STEPS)) expect(own[token]).toBe(both(light, dark));
  });

  it("move down with OLED, with or without data-theme", () => {
    for (const selector of [
      '[data-theme="dark"][data-oled],\n[data-theme="dark"][data-oled][data-rainbow] .glim-hue',
      ':root[data-oled]:not([data-theme]),\n  :root[data-oled]:not([data-theme])[data-rainbow] .glim-hue',
    ]) {
      const oled = block(selector);
      for (const [token, [light, , dark]] of Object.entries(STEPS)) expect(oled[token], `${selector} ${token}`).toBe(both(light, dark));
    }
  });

  it("follow a rainbow position as Tailwind colours too", () => {
    for (const [utility, token] of [["hover", "hover"], ["hoverFilled", "hover-filled"], ["hoverRaised", "hover-raised"]]) {
      expect(block("@theme")[`--color-${utility}`]).toBe(`var(--${token})`);
      expect(block("[data-rainbow] .glim-hue")[`--color-${utility}`]).toBe(`var(--${token})`);
    }
  });

  it("keep the text on a hovered row at 4.5:1 for every preset in every theme", () => {
    const themes = [
      { name: "light", side: 0, text: "#161616", sub: "#525252" },
      { name: "dark", side: 1, text: "#f4f4f4", sub: "#c6c6c6" },
      { name: "OLED", side: 2, text: "#f4f4f4", sub: "#c6c6c6" },
    ];
    const misses: string[] = [];
    for (const accent of DEFAULT_ACCENT_PRESETS) {
      const [, a, b] = oklabOf(accent);
      for (const { name, side, text, sub } of themes) {
        for (const [token, steps] of Object.entries(STEPS)) {
          const ground = srgbOf([steps[side], a * 0.3, b * 0.3]);
          const inks = token === "--hover-raised" ? [text] : [text, sub];
          for (const ink of inks) {
            const r = contrastRatio(ink, ground);
            if (r < MIN_TEXT_CONTRAST) misses.push(`${accent} ${ink} on ${token} on ${name}: ${r.toFixed(2)}`);
          }
        }
      }
    }
    expect(misses).toEqual([]);
  });
});

// A ground that comes from the accent, a button's tint, a rainbow position, a
// brand tile or a state hue. A raised tile's hover is a theme token set to a
// tint below.
const LIT = /var\(--(hover|hover-filled|hover-raised|accent[\w-]*|btn-tint(?:ed)?|item-hue[\w-]*|tile|status-[\w-]+|glim-tile-raised-hover)\)/;
const SYSTEM = /^(Highlight|HighlightText|ButtonFace|ButtonText|Canvas|CanvasText|LinkText|GrayText|SelectedItem)$/;
// A chip's dismiss control takes the pill's own ink, accent or neutral.
const OWN_INK = new Set([".glim-btn-chip:hover:not(:disabled)"]);

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

  it("gives a raised tile a tint in every theme", () => {
    const values = [...CSS.matchAll(/--glim-tile-raised-hover:\s*([^;]+);/g)].map((m) => m[1].trim());
    expect(values.length).toBeGreaterThan(0);
    for (const v of values) expect(v).toMatch(/^var\(--hover(-filled|-raised)?\)$/);
  });

  it("lights its ground with the accent in a component", () => {
    const grey: string[] = [];
    const walk = (dir: string) => {
      for (const name of readdirSync(dir)) {
        const path = join(dir, name);
        if (statSync(path).isDirectory()) walk(path);
        else if (/\.tsx?$/.test(name) && !/\.test\.tsx?$/.test(name)) {
          const text = readFileSync(path, "utf8");
          // A disabled control keeps its resting fill under the pointer.
          for (const m of text.matchAll(/(?<![\w-])((?:[\w-]+:)*hover:bg-carbon-[\w-]+)/g)) {
            if (/(^|:)disabled:/.test(m[1])) continue;
            grey.push(`${path.slice(SRC.length + 1).replace(/\\/g, "/")}:${text.slice(0, m.index).split("\n").length} ${m[1]}`);
          }
        }
      }
    };
    walk(SRC);
    expect(grey).toEqual([]);
  });

  it("fills a row's glyph tile with the accent when the tile stands in the row's button", () => {
    const lit = rules.find((r) => r.selector.includes(".glim-hue:is(:hover, :focus-within, [data-picked]) > button:not(:disabled) > .glim-row-glyph"));
    expect(lit?.body).toMatch(/background-color:\s*var\(--accent\)/);
  });
});

// What the colour picker can hand in, GlimStone's sweep: twelve hues at low,
// mid and high chroma on four lightness steps, and a run of greys.
const SWEEP = [
  ...[0.45, 0.6, 0.75, 0.9].flatMap((l) =>
    [0.02, 0.08, 0.2].flatMap((c) => Array.from({ length: 12 }, (_, i) => srgbOf([l, c * Math.cos((i * Math.PI) / 6), c * Math.sin((i * Math.PI) / 6)]))),
  ),
  ...["#222222", "#555555", "#6B6B6B", "#777777", "#7B7B7B", "#888888", "#999999", "#BBBBBB", "#DDDDDD"],
];

/** `share` of `b` in `a`, as color-mix() in srgb mixes two opaque colours. */
function mixHex(a: string, b: string, share: number): string {
  const ch = (hex: string, i: number) => parseInt(hex.slice(1 + 2 * i, 3 + 2 * i), 16);
  return "#" + [0, 1, 2].map((i) => Math.round(ch(a, i) * (1 - share) + ch(b, i) * share).toString(16).padStart(2, "0")).join("");
}

describe("a tinted button under the pointer", () => {
  const TONES = { neutral: "--carbon-hover-raised", subtle: "--carbon-surface3" } as const;
  const tinted = (tone: string) => rules.find((r) => r.selector === `.glim-btn-${tone}`)?.body.match(/--btn-tinted:\s*([^;]+);/)?.[1].trim();
  const grounds = rules.filter((r) => /^:is\(\.glim-btn-neutral, \.glim-btn-subtle\):not\(:disabled\):(hover|focus-visible)$/.test(r.selector));

  it("mixes 22% of the tint into the step each tone hovers to", () => {
    for (const [tone, step] of Object.entries(TONES)) expect(tinted(tone)).toBe(`color-mix(in srgb, var(${step}), var(--btn-tint) 22%)`);
  });

  it("takes the same ground under the pointer and the keyboard", () => {
    expect(grounds).toHaveLength(2);
    for (const { body } of grounds) expect(body).toMatch(/background-color:\s*light-dark\(var\(--btn-tinted\), oklab\(from var\(--btn-tinted\) min\(l, [\d.]+\) a b\)\);/);
  });

  it("keeps its label at 4.5:1 for every picked accent in every theme", () => {
    // The lightness a tinted ground stops at on a dark theme, if it stops at all.
    const ceiling = Number(grounds[0]?.body.match(/min\(l, ([\d.]+)\)/)?.[1] ?? Infinity);
    const themes = [
      { name: "light", dark: false, text: "#161616", steps: { neutral: "#c6c6c6", subtle: "#d1d1d1" } },
      { name: "dark", dark: true, text: "#f4f4f4", steps: { neutral: "#6f6f6f", subtle: "#525252" } },
      { name: "OLED", dark: true, text: "#f4f4f4", steps: { neutral: "#525252", subtle: "#393939" } },
    ];
    const misses: string[] = [];
    for (const accent of [...DEFAULT_ACCENT_PRESETS, ...SWEEP]) {
      const inks = accentInks(accent);
      for (const { name, dark, text, steps } of themes) {
        const glyph = dark ? (inks.dark ?? lightenTo(accent, INK_LIGHTNESS.dark)) : (inks.light ?? darkenTo(accent, INK_LIGHTNESS.light));
        for (const [tone, step] of Object.entries(steps)) {
          const mixed = mixHex(step, glyph, 0.22);
          const [l, a, b] = oklabOf(mixed);
          const ground = dark && l > ceiling ? srgbOf([ceiling, a, b]) : mixed;
          const r = contrastRatio(text, ground);
          if (r < MIN_TEXT_CONTRAST) misses.push(`${accent} ${tone} on ${name}: ${ground} ${r.toFixed(2)}`);
        }
      }
    }
    expect(misses).toEqual([]);
  });
});
