// The window entrance, the reactive label, the shape morph, the confirm swell
// and the reorder wiggle read the level's dials, so a level is a set of
// numbers and never a rule of its own. A fixed number in one of these rules
// plays every level at one speed, and nothing on screen says which.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const CSS = readFileSync(join(dirname(fileURLToPath(import.meta.url)), "..", "index.css"), "utf8").replace(
  /\/\*[\s\S]*?\*\//g,
  ""
);

/** The body of the first rule for an exact selector at or after `from`. */
function ruleBody(selector: string, from = 0): string {
  const at = CSS.indexOf(selector + " {", from);
  expect(at, selector).toBeGreaterThanOrEqual(0);
  return CSS.slice(at, CSS.indexOf("}", at));
}

/** The body of a keyframes block, both braces deep. */
function keyframes(name: string): string {
  const at = CSS.indexOf(`@keyframes ${name} {`);
  expect(at, name).toBeGreaterThanOrEqual(0);
  return CSS.slice(at, CSS.indexOf("\n}", at));
}

// The gate that holds the app's animations, found by its first rule.
const gate = CSS.indexOf("  .glim-btn { transition:");

describe("the motion dials", () => {
  it("are read inside the gate this test searches", () => {
    expect(gate).toBeGreaterThan(0);
  });

  it("time a window's entrance and end it without a transform", () => {
    expect(ruleBody(".glim-modal-card", gate)).toMatch(/glim-modal-in var\(--motion-modal-dur\)/);
    expect(ruleBody(".glim-modal-backdrop", gate)).toMatch(/glim-fade-in var\(--motion-modal-dur\)/);
    const frames = keyframes("glim-modal-in");
    expect(frames).toMatch(/translateY\(var\(--motion-modal-travel/);
    expect(frames).not.toMatch(/scale/);
    expect(frames).toMatch(/to\s*\{[^}]*transform:\s*none/);
  });

  it("time the reactive label's reveal at every level", () => {
    const body = ruleBody(".glim-label-reactive", CSS.indexOf("@media (prefers-reduced-motion: no-preference) {\n  .glim-label-reactive"));
    expect(body).toMatch(/max-width var\(--motion-label-dur\) var\(--motion-label-ease\)/);
    expect(body).toMatch(/translateX\(var\(--motion-label-shift\)\) scaleX\(var\(--motion-label-squash\)\)/);
    expect(CSS).not.toMatch(/\[data-motion="subtle"\] \.glim-label-reactive/);
  });

  it("morph the shape through the registered radius tokens on the root", () => {
    for (const token of ["--radius-card", "--radius-control", "--radius-pill"]) {
      expect(CSS).toContain(`@property ${token} { syntax: '<length>'; inherits: true;`);
    }
    const body = ruleBody(":root.glim-shape-transitions", gate);
    expect(body).toMatch(/--radius-pill var\(--motion-shape-dur\) var\(--motion-shape-ease\)/);
  });

  it("swell a success through the confirm dials", () => {
    expect(ruleBody(".glim-confirm", gate)).toMatch(/glim-confirm var\(--motion-confirm-dur\)/);
    expect(keyframes("glim-confirm")).toMatch(/var\(--motion-confirm-scale/);
  });

  it("rock every item of a list being reordered but the one in the hand", () => {
    expect(ruleBody(".glim-drag-armed > :not(.glim-drag-lift)", gate)).toMatch(
      /glim-wiggle var\(--motion-wiggle-dur\)/
    );
    expect(keyframes("glim-wiggle")).toMatch(/rotate:\s*var\(--motion-wiggle-angle/);
  });
});
