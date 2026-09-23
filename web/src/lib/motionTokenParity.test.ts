// Every motion level answers every motion token. A level that omits one
// inherits the bare :root value, which is the lively one, so a quiet level
// would keep a lively number and the storm would keep the default's. Allowed
// omissions are listed with their reason.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const RAW = readFileSync(join(dirname(fileURLToPath(import.meta.url)), "..", "index.css"), "utf8");

// Comments are blanked, keeping offsets, because the notes in the bare root
// block name token families ("--motion-shape-*") that would count as
// declarations.
const CSS = RAW.replace(/\/\*[\s\S]*?\*\//g, (m) => m.replace(/[^\n]/g, " "));

/** The tokens declared in the block that starts at `at`. */
function tokensAt(at: number): Set<string> {
  if (at < 0) return new Set();
  const body = CSS.slice(at, CSS.indexOf("\n}", at));
  return new Set([...body.matchAll(/(--motion-[a-z-]+)\s*:/g)].map((m) => m[1]));
}

/** The tokens declared in one `:root...{ }` block, by selector. */
function tokensOf(selector: string): Set<string> {
  return tokensAt(CSS.indexOf(selector + " {"));
}

/**
 * The tokens of the bare `:root` block that holds the lively defaults. The file
 * has several bare `:root` blocks, and the first declares no motion token.
 */
function livelyDefaults(): Set<string> {
  const marker = CSS.indexOf("--motion-page-travel:");
  return tokensAt(CSS.lastIndexOf(":root {", marker));
}

/**
 * QUIET_MAY_OMIT lists the curves: a quieter level changes duration, not
 * easing. A livelier level gets no such exemption, so storm must declare them.
 */
const QUIET_MAY_OMIT = new Set(["--motion-shape-ease", "--motion-wipe-ease", "--motion-toast-ease"]);

describe("motion tokens", () => {
  const lively = livelyDefaults();

  it("declares the lively default on the bare root", () => {
    // With only a handful, the comparisons below would check nothing.
    expect(lively.size).toBeGreaterThanOrEqual(20);
  });

  for (const level of ["subtle", "off"]) {
    it(`${level} answers every dial the default sets`, () => {
      const mine = tokensOf(`:root[data-motion="${level}"]`);
      const missing = [...lively].filter((t) => !mine.has(t) && !QUIET_MAY_OMIT.has(t));
      expect(missing).toEqual([]);
    });
  }

  it("storm answers every dial, curves included", () => {
    // A livelier level on the default's gentler curve reads as sluggish rather
    // than wilder.
    const mine = tokensOf(':root[data-motion="storm"]');
    const missing = [...lively].filter((t) => !mine.has(t));
    expect(missing).toEqual([]);
  });

  // Travel is the page's only spatial dial; lib/pageEnterFlat.test.ts checks
  // that tilt and scale do not exist.
  it("keeps the page's one spatial dial answered at every level", () => {
    for (const level of ["subtle", "off", "storm"]) {
      const at = CSS.indexOf(`:root[data-motion="${level}"] {`);
      const body = CSS.slice(at, CSS.indexOf("\n}", at));
      expect(body).toMatch(/--motion-page-travel:/);
    }
  });
});
