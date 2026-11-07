import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

// OLED keeps the dark theme's shadow. It is lost on black, where a card's
// #161616 is edge enough, and a ring in its place reads as a frame round every
// card. Over a card the shadow still lifts a bubble or a menu off it.

const css = readFileSync(join(dirname(fileURLToPath(import.meta.url)), "..", "index.css"), "utf8");

/** The declarations of every rule whose selector names data-oled. */
function oledBlocks(): string[] {
  return [...css.matchAll(/([^{}]*\[data-oled\][^{}]*)\{([^{}]*)\}/g)].map((m) => m[2]);
}

describe("OLED", () => {
  it("has its blocks found", () => {
    expect(oledBlocks().length).toBeGreaterThanOrEqual(2);
  });

  it("leaves the card's shadow as the dark theme sets it", () => {
    for (const block of oledBlocks()) expect(block).not.toMatch(/--elevation\s*:/);
  });
});
