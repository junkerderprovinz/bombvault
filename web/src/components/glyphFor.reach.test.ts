// Every button can reach a glyph, or says at its own call site that it has none.
// A button whose key matches nothing prints a word in a row of symbols. There is
// no allow-list, because from outside a documented exception and a forgotten
// button look the same; a call site that wants no glyph writes `glyph=` where
// its reader sees it.
//
// Every branch of a ternary labelKey has to resolve, since either may be on
// screen. A key held in a variable is reported rather than skipped.
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { buttonTags } from "./buttonTags.testsupport";
import { glyphFor } from "./glyphFor";

const SRC = join(__dirname, "..");
const TAGS = buttonTags(SRC);

/**
 * The keys a single call site can produce, and whether it names any key at
 * all. `null` contributes nothing and is not a gap; the hygiene test checks
 * that the prop is present.
 */
function keysOf(props: string): { keys: string[]; hasExpression: boolean } {
  const m = /\blabelKey\s*=\s*(?:"([^"]*)"|'([^']*)'|\{([\s\S]*?)\})/.exec(props);
  if (!m) return { keys: [], hasExpression: false };
  const literal = m[1] ?? m[2];
  if (literal !== undefined) return { keys: [literal], hasExpression: false };

  const expr = m[3] ?? "";
  if (/^\s*null\s*$/.test(expr)) return { keys: [], hasExpression: false };

  // A comparison inside the expression carries string literals that are not
  // keys: `labelKey={mode === "dr" ? a : b}` would otherwise contribute "dr".
  // Strip the comparisons first, then harvest what is left.
  const body = expr.replace(/[=!]==?\s*["'][^"']*["']/g, "");
  const keys = Array.from(body.matchAll(/["']([^"']+)["']/g)).map((k) => k[1]);
  return { keys, hasExpression: keys.length === 0 };
}

type Gap = { where: string; detail: string };

const gaps: Gap[] = [];
for (const tag of TAGS) {
  // An explicit glyph is the exemption, stated at the call site. A chip never
  // shows a glyph in any mode.
  if (/\bglyph\s*=/.test(tag.props)) continue;
  if (/\bvariant\s*=\s*["']chip["']/.test(tag.props)) continue;

  const { keys, hasExpression } = keysOf(tag.props);
  const where = `${tag.file}:${tag.line}`;

  if (hasExpression) {
    gaps.push({ where, detail: "labelKey is an expression with no literal in it, so nothing here can say whether it resolves" });
    continue;
  }
  if (keys.length === 0) continue; // null, or no labelKey: the hygiene test's business

  const unreachable = keys.filter((k) => glyphFor(k) === undefined);
  if (unreachable.length > 0) {
    gaps.push({ where, detail: unreachable.join(", ") });
  }
}

describe("glyph reach", () => {
  // A broken parser would report perfect coverage of nothing.
  it("finds the buttons and their keys", () => {
    expect(
      TAGS.length,
      "the <Button> scan found almost nothing, so it is measuring its own regex\n" +
        "rather than the app. Compare against: grep -rc '<Button' over web/src.",
    ).toBeGreaterThan(150);
    const distinct = new Set(TAGS.flatMap((t) => keysOf(t.props).keys));
    expect(
      distinct.size,
      "almost no distinct labelKeys came out of the tags, so the key extraction\n" +
        "is broken even though the tag scan worked.",
    ).toBeGreaterThan(100);
  });

  it("every button reaches a glyph, or says so at its own call site", () => {
    const report = gaps.map((g) => `  ${g.where}  ${g.detail}`).join("\n");
    expect(
      gaps,
      `These buttons print a word where their neighbours print a symbol:\n\n${report}\n\n` +
        `Two ways out, and an allow-list is not one of them:\n` +
        `  - add a rule to RULES in glyphFor.tsx, so the verb gets its mark everywhere;\n` +
        `  - or write glyph={...} at the call site, which states the exception where\n` +
        `    somebody reading that button will see it.\n` +
        `A key held in a variable has to be resolved by hand or given an explicit\n` +
        `glyph; this guard cannot follow it, and saying so beats skipping it.`,
    ).toEqual([]);
  });
});
