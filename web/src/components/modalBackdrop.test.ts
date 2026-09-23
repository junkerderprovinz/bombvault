import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";
import { expect, it } from "vitest";

/**
 * Every modal backdrop takes its fade and darkness from `.glim-modal-backdrop`.
 * One that forgets the class opens over an undimmed page, and nothing at its
 * call site looks wrong. A backdrop is a div with `fixed inset-0 z-50`, found
 * in string and template literals rather than lines, as in hoverRamp.test.ts.
 */

const here = dirname(fileURLToPath(import.meta.url));
const src = join(here, "..");

/** What marks a div as a modal backdrop in this codebase. */
const BACKDROP = "fixed inset-0 z-50";

function tsxFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((entry) => {
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) return entry === "node_modules" ? [] : tsxFiles(full);
    if (!full.endsWith(".tsx") || full.includes(".test.")) return [];
    return [full];
  });
}

/** Every quoted string and template chunk in a file, without its quotes. */
function literals(source: string): string[] {
  return (source.match(/"[^"\n]*"|'[^'\n]*'|`[^`]*`/g) ?? []).map((raw) => raw.slice(1, -1));
}

const backdrops = tsxFiles(src).flatMap((file) =>
  literals(readFileSync(file, "utf8"))
    .filter((text) => text.includes(BACKDROP))
    .map((text) => ({ where: relative(src, file), text }))
);

it("finds the backdrops at all", () => {
  // The checks below pass vacuously if the search stops matching.
  expect(backdrops.length).toBeGreaterThanOrEqual(9);
});

it("gives every modal backdrop the class that darkens it", () => {
  for (const { where, text } of backdrops) {
    expect(text, where).toContain("glim-modal-backdrop");
  }
});

it("paints the darkness outside any motion query", () => {
  // Darkening is not motion. Inside a prefers-reduced-motion query it would
  // vanish for anyone who asks for less motion. A rendered dialog only sees the
  // queries it matches, so this reads the stylesheet.
  const css = readFileSync(join(src, "index.css"), "utf8");

  // Everything inside a prefers-reduced-motion query, by brace counting from
  // each query's opening brace.
  const gated: string[] = [];
  const q = /@media\s*\([^)]*prefers-reduced-motion[^)]*\)\s*\{/g;
  let m: RegExpExecArray | null;
  while ((m = q.exec(css))) {
    let depth = 1;
    let i = m.index + m[0].length;
    const from = i;
    for (; i < css.length && depth > 0; i++) {
      if (css[i] === "{") depth++;
      else if (css[i] === "}") depth--;
    }
    gated.push(css.slice(from, i));
  }

  expect(
    gated.length,
    "no prefers-reduced-motion query was found in index.css, so this test is\n" +
      "checking nothing. The query's shape changed.",
  ).toBeGreaterThan(0);

  for (const block of gated) {
    const rule = /\.glim-modal-backdrop\s*\{([^}]*)\}/.exec(block);
    if (!rule) continue;
    expect(
      rule[1],
      "the backdrop's own colour is declared inside a prefers-reduced-motion\n" +
        "query. A reader who asks their system for less motion then gets NO\n" +
        "darkening, and a window floating over an undimmed page. Move the\n" +
        "background out to the unconditional rule and leave only the animation\n" +
        "in here.",
    ).not.toMatch(/background/);
  }
});

it("takes its darkness from the token, not from a literal", () => {
  // A literal has to be found wherever it was typed before it can change; a
  // token has one name.
  const css = readFileSync(join(src, "index.css"), "utf8");
  const rules = Array.from(css.matchAll(/\.glim-modal-backdrop\s*\{([^}]*)\}/g)).map((r) => r[1]);
  const painting = rules.filter((body) => /background/.test(body));

  expect(painting.length, "no .glim-modal-backdrop rule paints a background at all").toBe(1);
  expect(
    painting[0],
    "the backdrop paints a literal colour. It takes --glim-scrim, which the\n" +
      "theme blocks set to .65 dark and .55 light.",
  ).toMatch(/var\(--glim-scrim\)/);
});

it("keeps the darkness in the class, not beside it", () => {
  // A second value on the element wins or loses against the class depending on
  // which one Tailwind emits last, so two windows would darken differently
  // and only one of them would answer to index.css.
  for (const { where, text } of backdrops) {
    expect(text, where).not.toMatch(/bg-black\//);
  }
});
