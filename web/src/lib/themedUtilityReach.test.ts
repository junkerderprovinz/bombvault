// Every themed utility a component names has to exist in the theme.
//
// Tailwind v4 builds a utility from a `--color-<name>` custom property in the
// `@theme` block. Write `bg-statusWarnBgSoft` without declaring
// `--color-statusWarnBgSoft` and nothing complains: Tailwind emits no rule,
// TypeScript never sees class strings, the lint rules read code rather than
// CSS, and the element renders transparent while keeping its radius and
// padding, so it still looks intended.
//
// This is a source scan rather than a DOM test because a DOM test sees only the
// components it renders, and the wrong class is usually one no test renders.
import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const SRC = join(dirname(fileURLToPath(import.meta.url)), "..");
const CSS = join(SRC, "index.css");

// The themed families. `carbon`, `accent` and `status` are the three prefixes
// this app's @theme block defines; anything else in a class name is either a
// stock Tailwind colour or not a colour at all, and neither is ours to check.
const FAMILY = "(?:carbon|accent|status)";
// A utility is <property>-<Family><Rest>, and the property list is the set of
// Tailwind utilities that resolve against --color-*. `divide` and `outline` are
// in it because both take a colour and both are used here.
const PROPERTY = "(?:bg|text|border|ring|fill|stroke|divide|outline|from|via|to|shadow|caret|accent)";
// The name may carry hyphens of its own (`carbon-textSub`), so the character
// class allows them, and a trailing hyphen is trimmed off afterwards so a typo
// like `bg-carbon-surface-` does not read as part of the name.
const USE = new RegExp(`\\b${PROPERTY}-(${FAMILY}[A-Za-z0-9-]*)`, "g");

function sources(dir: string): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir)) {
    if (entry === "node_modules" || entry === "dist" || entry === "locales") continue;
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) {
      out.push(...sources(full));
      continue;
    }
    if (/\.tsx?$/.test(entry) && !/\.test\.tsx?$/.test(entry)) out.push(full);
  }
  return out;
}

// Comments are stripped before the scan because they may quote class names
// that are not in the theme. A line comment is stripped only when its slashes
// are not part of a URL, hence the check on the preceding character.
function code(text: string): string {
  return text
    .replace(/\/\*[\s\S]*?\*\//g, " ")
    .replace(/(^|[^:\w])\/\/[^\n]*/g, "$1");
}

const css = readFileSync(CSS, "utf8");
// The @theme declarations, as a set of bare names: --color-statusWarnBg -> statusWarnBg.
const declared = new Set(
  Array.from(css.matchAll(/--color-([A-Za-z0-9-]+)\s*:/g)).map((m) => m[1]),
);

type Use = { name: string; file: string; line: number };

const uses: Use[] = [];
for (const file of sources(SRC)) {
  const text = code(readFileSync(file, "utf8"));
  text.split("\n").forEach((line, i) => {
    for (const m of line.matchAll(USE)) {
      const name = m[1].replace(/-+$/, "");
      uses.push({ name, file: file.slice(SRC.length + 1).replace(/\\/g, "/"), line: i + 1 });
    }
  });
}

describe("themed utilities", () => {
  // A scanner that reads nothing passes silently, so check that it read
  // something.
  it("the scan actually reached the source tree", () => {
    expect(
      uses.length,
      "the themed-utility scan found almost nothing, so it is measuring its own\n" +
        "regex rather than the app. Check SRC, the file filter and USE before\n" +
        "trusting a green run below.",
    ).toBeGreaterThan(300);
    expect(
      declared.size,
      "no --color-* declarations were read out of index.css, so every utility\n" +
        "below would be reported missing. The @theme block's shape changed.",
    ).toBeGreaterThan(30);
  });

  it("every one of them is declared in the theme", () => {
    const missing = uses.filter((u) => !declared.has(u.name));
    const report = missing
      .map((u) => `  ${u.file}:${u.line}  ${u.name}`)
      .join("\n");
    expect(
      missing,
      `A themed utility names a colour the @theme block does not declare:\n\n${report}\n\n` +
        `Tailwind emits no rule for an undeclared name, so the element renders with\n` +
        `NO colour and keeps its radius and padding - it looks deliberate and is\n` +
        `transparent. Add --color-<name> to the @theme block in web/src/index.css\n` +
        `and give it a value in BOTH theme blocks, or use a name that exists.\n` +
        `Declared names starting the same way: ${[...declared]
          .filter((d) => missing.some((m) => d.slice(0, 9) === m.name.slice(0, 9)))
          .join(", ") || "none"}`,
    ).toEqual([]);
  });
});
