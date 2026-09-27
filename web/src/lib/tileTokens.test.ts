// A tile set straight on a card takes its tones from the --glim-tile-* tokens.
// A colour mode that misses one falls back to the dark value, and a dark
// chip on the light theme's grey tile is what that looks like.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const CSS = readFileSync(join(dirname(fileURLToPath(import.meta.url)), "..", "index.css"), "utf8").replace(
  /\/\*[\s\S]*?\*\//g,
  ""
);

function tokensOf(selector: string): string[] {
  const at = CSS.indexOf(selector + " {");
  expect(at, selector).toBeGreaterThanOrEqual(0);
  const body = CSS.slice(at, CSS.indexOf("\n}", at));
  return [...body.matchAll(/(--glim-tile-[a-z-]+)\s*:/g)].map((m) => m[1]).sort();
}

describe("the tile tokens", () => {
  const dark = tokensOf(`:root,\n[data-theme="dark"]`);

  it("are set on the dark default", () => {
    expect(dark).toEqual([
      "--glim-tile-bg",
      "--glim-tile-raised",
      "--glim-tile-raised-hover",
      "--glim-tile-switch-off",
      "--glim-tile-well",
    ]);
  });

  it("are set again for the light theme, with and without JavaScript", () => {
    expect(tokensOf(`[data-theme="light"]`)).toEqual(dark);
    expect(tokensOf(`  :root:not([data-theme])`)).toEqual(dark);
  });
});
