// The stylesheet does not nest style rules. A rule that opens inside another
// one is a rule that lost its closing brace, which a browser reads as nesting:
// everything after it quietly applies to the wrong elements, and nothing fails.
//
// Node environment: this reads the stylesheet, it does not render.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const RAW = readFileSync(join(dirname(fileURLToPath(import.meta.url)), "..", "index.css"), "utf8");

/* Comments and strings are blanked so a brace inside them is not counted.
   Blanked rather than deleted so reported lines still match the file. */
const CSS = RAW.replace(/\/\*[\s\S]*?\*\/|"(?:[^"\\\n]|\\.)*"|'(?:[^'\\\n]|\\.)*'/g, (m) => m.replace(/[^\n]/g, " "));

type Block = "at" | "keyframes" | "step" | "rule";

/** Walks the braces and returns where a style rule opens inside another one,
 *  and how many blocks are still open at the end. */
function shape(css: string): { nested: string[]; open: number } {
  const stack: Block[] = [];
  const nested: string[] = [];
  let header = 0;
  for (let i = 0; i < css.length; i++) {
    const c = css[i];
    if (c === ";") header = i + 1;
    if (c === "}") {
      stack.pop();
      header = i + 1;
    }
    if (c !== "{") continue;
    const head = css.slice(header, i).trim();
    const parent = stack[stack.length - 1];
    const kind: Block = head.startsWith("@")
      ? /^@(-webkit-)?keyframes\b/.test(head)
        ? "keyframes"
        : "at"
      : parent === "keyframes"
        ? "step"
        : "rule";
    if (kind === "rule" && stack.includes("rule")) {
      nested.push(`line ${css.slice(0, i).split("\n").length}: ${head.split("\n").pop()?.trim()}`);
    }
    stack.push(kind);
    header = i + 1;
  }
  return { nested, open: stack.length };
}

describe("the stylesheet's shape", () => {
  it("finds a rule that lost its closing brace", () => {
    const broken = ".a {\n  color: red;\n\n.b {\n  color: blue;\n}\n";
    expect(shape(broken)).toEqual({ nested: ["line 4: .b"], open: 1 });
    expect(shape("@media print {\n  @keyframes x {\n    from { opacity: 0; }\n  }\n  .a { color: red; }\n}\n")).toEqual({ nested: [], open: 0 });
  });

  it("closes every block and nests no style rule in another", () => {
    const found = shape(CSS);
    expect(found.nested, `style rules opened inside another: ${found.nested.join("; ")}`).toEqual([]);
    expect(found.open).toBe(0);
  });
});
