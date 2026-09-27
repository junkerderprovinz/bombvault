// A glyph beside a button's words is the words' size, and a glyph alone in a
// square is half the square. The sizes live only in index.css, so this reads
// the stylesheet.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const CSS = readFileSync(join(dirname(fileURLToPath(import.meta.url)), "..", "index.css"), "utf8").replace(
  /\/\*[\s\S]*?\*\//g,
  ""
);

/** The width and height one exact selector sets, from its own rule. */
function sizeOf(selector: string): { width?: string; height?: string } {
  const at = CSS.indexOf("\n" + selector + " {");
  expect(at, selector).toBeGreaterThanOrEqual(0);
  const body = CSS.slice(at, CSS.indexOf("}", at));
  return {
    width: /\swidth:\s*([^;]+);/.exec(body)?.[1],
    height: /\sheight:\s*([^;]+);/.exec(body)?.[1],
  };
}

describe("button glyphs", () => {
  it("stand as tall as the 14px words beside them", () => {
    expect(sizeOf(".glim-btn-glyph > svg")).toEqual({ width: "0.875rem", height: "0.875rem" });
  });

  it("take 16px in the taller key control", () => {
    expect(sizeOf(".glim-btn-key .glim-btn-glyph > svg")).toEqual({ width: "1rem", height: "1rem" });
  });

  it("fill half the square when they stand alone", () => {
    expect(sizeOf(".glim-btn-icon .glim-btn-glyph > svg")).toEqual({ width: "1rem", height: "1rem" });
    expect(sizeOf(".glim-btn-icon.glim-btn-key .glim-btn-glyph > svg")).toEqual({
      width: "1.25rem",
      height: "1.25rem",
    });
  });
});
