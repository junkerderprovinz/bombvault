// A selector that sets a value takes its colour from its place on the page,
// which lib/selectorHue.ts counts. A start set by hand would give two selectors
// one colour again, so only the tab strips may set one: they keep a colour per
// tab. The PayPal window's two selectors keep theirs too, since the giving
// windows stay as they are.
//
// This reads source; it does not render.
import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const SRC = join(dirname(fileURLToPath(import.meta.url)), "..");

const TAB_STRIPS = ["pages/BackupEntry.tsx", "components/PaypalDialog.tsx"];

function sources(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return name === "locales" ? [] : sources(path);
    return /\.tsx$/.test(name) && !/\.test\.tsx$/.test(name) ? [path] : [];
  });
}

/** Each `<Selector … />` open tag in a file, as its own text. */
function selectorTags(text: string): string[] {
  const out: string[] = [];
  for (let i = text.indexOf("<Selector"); i !== -1; i = text.indexOf("<Selector", i + 1)) {
    if (!/<Selector[\s>]/.test(text.slice(i, i + 10))) continue;
    let depth = 0;
    for (let j = i; j < text.length; j++) {
      const c = text[j];
      if (c === "{" || c === "(") depth++;
      else if (c === "}" || c === ")") depth--;
      else if (depth === 0 && c === ">") {
        out.push(text.slice(i, j + 1));
        break;
      }
    }
  }
  return out;
}

describe("selector colours", () => {
  it("finds the selectors at all", () => {
    const tags = sources(SRC).flatMap((path) => selectorTags(readFileSync(path, "utf8")));
    expect(tags.length).toBeGreaterThan(40);
  });

  it("lets only a tab strip set where its colours start", () => {
    const found: string[] = [];
    for (const path of sources(SRC)) {
      const name = path.slice(SRC.length + 1).replace(/\\/g, "/");
      if (TAB_STRIPS.includes(name) || name === "components/Selector.tsx") continue;
      for (const tag of selectorTags(readFileSync(path, "utf8"))) {
        if (/\bhueOffset=/.test(tag)) found.push(`${name}: ${tag.match(/label=\{?[^\n]*/)?.[0] ?? tag.slice(0, 40)}`);
      }
    }
    expect(found).toEqual([]);
  });

  it("gives a segment a colour of its own only in a tab strip", () => {
    const text = readFileSync(join(SRC, "components/Selector.tsx"), "utf8");
    const segmentStyles = text.match(/style=\{[^\n]*hueVars\([^\n]*/g) ?? [];
    expect(segmentStyles).toHaveLength(1);
    expect(segmentStyles[0]).toMatch(/^style=\{perTab \?/);
    expect(text).toMatch(/countSelector\(strip\.current\)/);
  });
});
