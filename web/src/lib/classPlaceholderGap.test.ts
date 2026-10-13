// A utility class in a className template needs a space before a placeholder.
//
// Tailwind finds classes by scanning the source as text, and it reads
// `max-md:overflow-y-auto${` as one token that names no utility, so the rule is
// never emitted. Nothing else notices: the class is in the DOM and the element
// only misses the one property. The house classes (`glim-*`) live in index.css
// and are not generated, so they may run into a placeholder, and so may a
// class that the placeholder completes, such as `fi-${code}`.
import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const SRC = join(dirname(fileURLToPath(import.meta.url)), "..");

function sources(dir: string): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir)) {
    if (entry === "node_modules" || entry === "dist" || entry === "locales") continue;
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) out.push(...sources(full));
    else if (/\.tsx$/.test(entry) && !/\.test\.tsx$/.test(entry)) out.push(full);
  }
  return out;
}

/** The end of the `${...}` that opens at `at`, past any template inside it. */
function placeholderEnd(text: string, at: number): number {
  let depth = 0;
  for (let i = at + 1; i < text.length; i++) {
    const c = text[i];
    if (c === "`") i = templateEnd(text, i);
    else if (c === "{") depth++;
    else if (c === "}" && --depth === 0) return i;
  }
  return text.length;
}

/** The closing backtick of the template that opens at `at`. */
function templateEnd(text: string, at: number): number {
  for (let i = at + 1; i < text.length; i++) {
    if (text[i] === "\\") i++;
    else if (text[i] === "`") return i;
    else if (text.startsWith("${", i)) i = placeholderEnd(text, i);
  }
  return text.length;
}

function crowded(file: string): string[] {
  const text = readFileSync(file, "utf8");
  const found: string[] = [];
  for (const m of text.matchAll(/className=\{`/g)) {
    const open = m.index + m[0].length - 1;
    const end = templateEnd(text, open);
    for (let i = open + 1; i < end; i++) {
      if (!text.startsWith("${", i)) continue;
      const token = /[^\s`]*$/.exec(text.slice(open + 1, i))![0];
      if (token !== "" && !token.startsWith("glim-") && !/[}-]$/.test(token)) {
        const line = text.slice(0, i).split("\n").length;
        found.push(`${relative(SRC, file)}:${line} ${token}\${`);
      }
      i = placeholderEnd(text, i);
    }
  }
  return found;
}

describe("className templates", () => {
  it("leave a space between a utility class and a placeholder", () => {
    expect(sources(SRC).flatMap(crowded)).toEqual([]);
  });
});
