// Two kinds of rule that cost a page with live values on every frame, as
// GlimStone's rules 36 and 37 describe them. A :has() that reads the class or
// style attribute makes Chrome restyle the whole page whenever any class or
// inline style changes, and a bar that is not contained makes Firefox lay out
// every ancestor of it again on each step. Both look fine and only show up in
// a profiler.
import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const SRC = join(dirname(fileURLToPath(import.meta.url)), "..");
const REPO = join(SRC, "..", "..");
const SHEETS = [join(SRC, "index.css"), join(SRC, "providerMarks.css"), join(REPO, "docs", "stylesheets", "extra.css")];

function sources(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return sources(path);
    return /\.tsx?$/.test(name) && !/\.test\.tsx?$/.test(name) ? [path] : [];
  });
}

/** Every `:has(...)` and every Tailwind `has-[...]` variant, with its argument
 *  matched through nested brackets. */
function hasSelectors(text: string): string[] {
  const found: string[] = [];
  for (const m of text.matchAll(/:has\(|\bhas-\[/g)) {
    const open = m[0].endsWith("(") ? "(" : "[";
    const close = open === "(" ? ")" : "]";
    let depth = 0;
    for (let i = m.index + m[0].length - 1; i < text.length; i++) {
      if (text[i] === open) depth++;
      else if (text[i] === close && --depth === 0) {
        found.push(text.slice(m.index, i + 1));
        break;
      }
    }
  }
  return found;
}

const readsClassOrStyle = (selector: string) => /\[\s*(?:class|style)\b/.test(selector);

function costly(text: string): string[] {
  return hasSelectors(text.replace(/\/\*[\s\S]*?\*\//g, "")).filter(readsClassOrStyle);
}

describe("a :has() selector", () => {
  it("is caught when it reads the class or style attribute, in a sheet or a class list", () => {
    expect(costly('.a:has([class*="open"]) { }')).toEqual([':has([class*="open"])']);
    expect(costly('.a:not(:has([style*="none"])) { }')).toEqual([':has([style*="none"])']);
    expect(costly('<div className="has-[[class~=open]]:grow" />')).toEqual(["has-[[class~=open]]"]);
    expect(costly(".a:has(.is-open) { } /* :has([class]) */")).toEqual([]);
    expect(costly('<div className="has-[[aria-expanded=true]]:opacity-100" />')).toEqual([]);
  });

  it("tests a class by its name, never the class or style attribute", () => {
    const files = [...SHEETS, ...sources(SRC)];
    const found = files.flatMap((file) => costly(readFileSync(file, "utf8")).map((hit) => `${relative(REPO, file)}: ${hit}`));
    expect(found).toEqual([]);
  });
});

describe("a bar whose value changes all the time", () => {
  const css = readFileSync(join(SRC, "index.css"), "utf8").replace(/\/\*[\s\S]*?\*\//g, "");

  it("is contained in size and layout, without paint", () => {
    const rule = /\.glim-meter\s*\{([^}]*)\}/.exec(css)?.[1] ?? "";
    expect(rule).toMatch(/contain:\s*size layout\s*;/);
    expect(rule).not.toMatch(/paint|strict|content/);
  });

  it("sets a real height wherever it wears the containment", () => {
    const lists = sources(SRC).flatMap((file) =>
      [...readFileSync(file, "utf8").matchAll(/"([^"]*\bglim-meter\b[^"]*)"/g)].map((m) => ({ file, classes: m[1] })),
    );
    expect(lists.length).toBeGreaterThan(0);
    const unsized = lists.filter(({ classes }) => !/(?:^|\s)h-(?!auto\b)[\w.[\]]+/.test(classes));
    expect(unsized.map(({ file, classes }) => `${relative(REPO, file)}: ${classes}`)).toEqual([]);
  });
});
