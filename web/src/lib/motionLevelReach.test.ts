// Motion rules name the quiet levels, never the lively ones. A rule keyed on
// `[data-motion="wild"]` leaves out the storm above it and the page before the
// boot code sets the attribute, although bare `:root` carries the wild numbers.
// `:not([data-motion="subtle"]):not([data-motion="off"])` covers wild, storm,
// the missing attribute and any level added later.
//
// The scan reads the stylesheet rather than the DOM, because the defect is a
// level nobody rendered.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const CSS = join(dirname(fileURLToPath(import.meta.url)), "..", "index.css");

// Strip comments first: the notes next to the rules quote the wrong form.
function code(text: string): string {
  return text.replace(/\/\*[\s\S]*?\*\//g, (m) => m.replace(/[^\n]/g, " "));
}

/** The levels a rule must never single out. */
const LIVELY = ["wild", "storm"];

/**
 * gatesSomething tells a rule gated on a level (`[data-motion="wild"] .thing`)
 * from the level's own token block (`:root[data-motion="storm"] {`): a brace
 * right after the attribute is a token block.
 */
function gatesSomething(css: string, end: number): boolean {
  return !/^\s*\{/.test(css.slice(end));
}

/**
 * reduceBlockRanges finds the `@media (prefers-reduced-motion: reduce)` blocks,
 * where storm may be named. The question there is whether the OS preference
 * still holds, and only storm is exempt, so the :not() form would wrongly
 * exempt wild and bare :root too. Brace matching keeps an edit above the block
 * from widening the exemption. lib/stormOverridesOs.test.ts checks what the
 * block contains.
 */
function reduceBlockRanges(css: string): [number, number][] {
  const out: [number, number][] = [];
  const needle = "@media (prefers-reduced-motion: reduce)";
  for (let i = css.indexOf(needle); i !== -1; i = css.indexOf(needle, i + 1)) {
    const open = css.indexOf("{", i);
    if (open === -1) break;
    let depth = 0;
    for (let j = open; j < css.length; j++) {
      if (css[j] === "{") depth++;
      else if (css[j] === "}") {
        depth--;
        if (depth === 0) {
          out.push([i, j]);
          break;
        }
      }
    }
  }
  return out;
}

describe("motion selectors", () => {
  const css = code(readFileSync(CSS, "utf8"));

  it("never key an animation on a lively level by name", () => {
    const offenders: string[] = [];
    const exempt = reduceBlockRanges(css);
    const insideReduceBlock = (at: number) => exempt.some(([from, to]) => at > from && at < to);
    for (const level of LIVELY) {
      const pattern = new RegExp(`\\[data-motion=["']${level}["']\\]`, "g");
      for (const m of css.matchAll(pattern)) {
        if (!gatesSomething(css, m.index! + m[0].length)) continue;
        if (level === "storm" && insideReduceBlock(m.index!)) continue;
        const line = css.slice(0, m.index!).split("\n").length;
        offenders.push(
          `index.css:${line} - ${m[0]} names a lively level, so every level above it ` +
            `and the attribute being absent both fall out of this rule. ` +
            `Write :root:not([data-motion="subtle"]):not([data-motion="off"]) instead.`
        );
      }
    }
    expect(offenders).toEqual([]);
  });

  it("still has quiet-level rules to protect, so the scan can find something", () => {
    // Otherwise the guard passes on an empty file, a renamed attribute or a
    // stylesheet without the motion engine.
    const quiet = [...css.matchAll(/\[data-motion=["'](subtle|off)["']\]/g)];
    expect(quiet.length).toBeGreaterThanOrEqual(8);
  });

  it("declares the hidden level's own token block", () => {
    // Without its block the selector rule still passes while storm renders at
    // the default numbers.
    expect(css).toContain(':root[data-motion="storm"]');
  });
});

describe("the reduce-block exemption", () => {
  const css = code(readFileSync(CSS, "utf8"));

  it("is narrow: wild is still forbidden from gating inside it too", () => {
    // Wild obeys the operating system like the other offered levels.
    const offenders: string[] = [];
    for (const [from, to] of reduceBlockRanges(css)) {
      const body = css.slice(from, to);
      for (const m of body.matchAll(/\[data-motion=["']wild["']\]/g)) {
        if (!gatesSomething(body, m.index! + m[0].length)) continue;
        offenders.push(`reduce block offset ${m.index} - ${m[0]}`);
      }
    }
    expect(offenders).toEqual([]);
  });

  it("does not apply outside that block", () => {
    // A storm-gated animation moved out of the reduce block is rejected again.
    const exempt = reduceBlockRanges(css);
    const outside = [...css.matchAll(/\[data-motion=["']storm["']\]/g)].filter((m) => {
      if (!gatesSomething(css, m.index! + m[0].length)) return false;
      return !exempt.some(([from, to]) => m.index! > from && m.index! < to);
    });
    expect(outside).toEqual([]);
  });
});
