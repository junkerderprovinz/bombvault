// Every text size is one of GlimStone's five: 22, 16, 14, 13 and 12px, read
// from the tokens in index.css. A size written as a number, an arbitrary
// Tailwind value or one of Tailwind's steps off the scale is the drift the
// scale exists to stop, so this scans the source for each of them.
//
// Two kinds of text stand outside the scale on purpose. A wordmark is the
// product's name drawn as a piece of chrome, and the text inside a picture is
// drawn in the picture's own units and scales with it.
import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const SRC = join(dirname(fileURLToPath(import.meta.url)), "..");
const CSS = readFileSync(join(SRC, "index.css"), "utf8");

/** Files whose sizes belong to a picture, drawn inside an SVG viewBox. */
const PICTURES = ["components/anomalies/SeriesCurve.tsx", "components/entry/DumpScene.tsx"];

/** The wordmarks, each the one off-scale size its file may use. */
const WORDMARKS: Record<string, string> = {
  "components/Sidebar.tsx": "text-xl",
  "pages/Dashboard.tsx": "text-lg",
  "pages/overview/InstanceSwitch.tsx": "text-lg",
  "launcher/Launcher.tsx": "text-xl",
};

/** Stylesheet rules that size the text of a picture, by selector prefix, and
 *  the README button's second line, which belongs to the giving buttons that
 *  stay as they are. */
const CSS_PICTURES = [
  ".glim-target-word",
  ".glim-sandbox-",
  ".glim-crypt-kit-word",
  ".glim-net-",
  ".glim-route-",
  ".glim-step-",
  ".glim-flow",
  ".glim-readme-btn-sub",
];

const OFF_SCALE = /\btext-(?:lg|[2-9]?xl|\[[\d.]+(?:px|rem|em|%)?\])(?![\w-])/g;

function sources(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return name === "locales" ? [] : sources(path);
    return /\.tsx?$/.test(name) && !/\.test\.tsx?$/.test(name) ? [path] : [];
  });
}

const rel = (path: string) => path.slice(SRC.length + 1).replace(/\\/g, "/");

describe("the type scale", () => {
  it("holds the five sizes", () => {
    const size = (name: string) => CSS.match(new RegExp(`--${name}:\\s*([\\d.]+)rem`))?.[1];
    expect([size("text-status"), size("text-title"), size("text-body"), size("text-subline"), size("text-meta")]).toEqual([
      "1.375",
      "1",
      "0.875",
      "0.8125",
      "0.75",
    ]);
  });

  it("gives Tailwind's own steps the sizes of the scale", () => {
    expect(CSS).toMatch(/--text-xs:\s*var\(--text-meta\)/);
    expect(CSS).toMatch(/--text-sm:\s*var\(--text-body\)/);
    expect(CSS).toMatch(/--text-base:\s*var\(--text-title\)/);
  });

  it("writes no size off the scale in a component", () => {
    const files = sources(SRC);
    expect(files.length).toBeGreaterThan(100);
    const found: string[] = [];
    for (const path of files) {
      const name = rel(path);
      if (PICTURES.includes(name)) continue;
      const text = readFileSync(path, "utf8");
      for (const m of text.matchAll(OFF_SCALE)) {
        if (WORDMARKS[name] === m[0]) continue;
        found.push(`${name}:${text.slice(0, m.index).split("\n").length} ${m[0]}`);
      }
    }
    expect(found).toEqual([]);
  });

  it("writes every font size in the stylesheet as a token", () => {
    const found: string[] = [];
    for (const m of CSS.matchAll(/font-size:\s*([^;}]+)/g)) {
      const value = m[1].trim();
      if (/^var\(--text-[a-z]+\)$/.test(value) || value === "inherit") continue;
      const open = CSS.lastIndexOf("{", m.index);
      const selector = CSS.slice(CSS.lastIndexOf("}", open) + 1, open).replace(/\/\*[\s\S]*?\*\//g, "").trim();
      if (CSS_PICTURES.some((prefix) => selector.split(",").every((s) => s.trim().includes(prefix)))) continue;
      found.push(`${selector} ${value}`);
    }
    expect(found).toEqual([]);
  });
});
