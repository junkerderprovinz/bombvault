// A filled control answers the pointer by a colour step (rule 21): the accent
// and status fills read --accent-hover and --status-*-hover, a surface steps up
// its ramp (hoverRamp.test.ts). Opacity would let the card show through and
// brightness() moves light and dark themes the same way, so neither may come
// back as a hover.
//
// Raising an element from hidden to shown, `hover:opacity-100`, is a reveal and
// not a step, so it stays allowed.
import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const SRC = join(dirname(fileURLToPath(import.meta.url)), "..");
const CSS = readFileSync(join(SRC, "index.css"), "utf8");

/** The rail's logo is a picture with no fill to step, so it dims instead. */
const ALLOWED = ["components/Sidebar.tsx hover:opacity-90"];

const DIMMING = /\b(?:hover|group-hover|peer-hover):(?:opacity-(?!100\b)\d+|brightness-\d+|saturate-\d+|\[filter:[^\]]*\])/g;

function sources(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return name === "locales" ? [] : sources(path);
    return /\.tsx?$/.test(name) && !/\.test\.tsx?$/.test(name) ? [path] : [];
  });
}

describe("hover by a colour step", () => {
  it("declares the steps for every filled tone", () => {
    for (const token of ["--accent-hover", "--status-ok-hover", "--status-fail-hover", "--status-warn-hover"]) {
      expect(CSS).toMatch(new RegExp(`${token}:\\s*color-mix\\(in srgb, var\\([^)]+\\) 85%, var\\(--carbon-text\\)\\)`));
    }
  });

  it("dims nothing under the pointer in a component", () => {
    const files = sources(SRC);
    expect(files.length).toBeGreaterThan(100);
    const found: string[] = [];
    for (const path of files) {
      const name = path.slice(SRC.length + 1).replace(/\\/g, "/");
      const text = readFileSync(path, "utf8");
      for (const m of text.matchAll(DIMMING)) {
        if (ALLOWED.includes(`${name} ${m[0]}`)) continue;
        found.push(`${name}:${text.slice(0, m.index).split("\n").length} ${m[0]}`);
      }
    }
    expect(found).toEqual([]);
  });

  it("brightens nothing under the pointer in the stylesheet", () => {
    const found: string[] = [];
    for (const rule of CSS.replace(/\/\*[\s\S]*?\*\//g, "").matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
      const [, selector, body] = rule;
      if (!/:hover|:is\([^)]*:hover/.test(selector)) continue;
      // Turning a mark white on a brand's fill is a colour, not a step.
      if (/brightness\((?!0\)\s*invert)/.test(body)) found.push(selector.trim());
    }
    expect(found).toEqual([]);
  });
});
