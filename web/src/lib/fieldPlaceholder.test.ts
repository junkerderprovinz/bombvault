// A placeholder is the only hint some fields give, so it is measured against
// the fills a field rests on. Muted text reads 2.35:1 on surface3, the sub
// step 4.5:1 or more on all of them.
import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { contrastRatio, MIN_TEXT_CONTRAST } from "./contrast";

const SRC = join(dirname(fileURLToPath(import.meta.url)), "..");
const CSS = readFileSync(join(SRC, "index.css"), "utf8");

/** The body of the first rule whose selector line is exactly `selector`. */
function block(selector: string): string {
  const start = CSS.indexOf(`\n${selector} {\n`);
  if (start < 0) throw new Error(`no block ${selector}`);
  return CSS.slice(start, CSS.indexOf("\n}\n", start));
}

function token(body: string, name: string): string {
  const m = body.match(new RegExp(`--${name}:\\s*(#[0-9a-fA-F]{6})`));
  if (!m) throw new Error(`no --${name}`);
  return m[1];
}

function sources(dir: string): string[] {
  return readdirSync(dir).flatMap((entry) => {
    const path = join(dir, entry);
    if (statSync(path).isDirectory()) return sources(path);
    return /\.tsx?$/.test(entry) && !/\.test\.tsx?$/.test(entry) ? [path] : [];
  });
}

describe("a field's placeholder", () => {
  for (const [theme, selector] of [
    ["dark", ':root,\n[data-theme="dark"]'],
    ["light", '[data-theme="light"]'],
  ] as const) {
    it(`reads at 4.5:1 on every field fill of the ${theme} theme`, () => {
      const body = block(selector);
      const placeholder = token(body, "field-placeholder");
      for (const fill of ["carbon-bg", "carbon-surface", "carbon-surface2", "carbon-surface3"]) {
        expect(contrastRatio(placeholder, token(body, fill)), fill).toBeGreaterThanOrEqual(MIN_TEXT_CONTRAST);
      }
    });
  }

  it("is what every placeholder in the stylesheet and the markup is painted with", () => {
    const rules = [...CSS.matchAll(/::placeholder\s*\{([^}]*)\}/g)].map((m) => m[1]);
    for (const rule of rules.filter((r) => /\bcolor:/.test(r))) {
      expect(rule).toContain("var(--field-placeholder)");
    }
    const utilities = sources(SRC).flatMap((path) =>
      [...readFileSync(path, "utf8").matchAll(/placeholder:text-(?!(?:xs|sm|base|lg|\d?xl)\b)([\w-]+)/g)].map(
        (m) => `${path}: ${m[1]}`,
      ),
    );
    expect(utilities.length).toBeGreaterThan(0);
    for (const u of utilities) expect(u).toMatch(/: fieldPlaceholder$/);
  });
});
