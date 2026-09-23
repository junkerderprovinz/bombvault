// The page entrance translates and does not scale or rotate: a flatter
// entrance at the top of the motion range, and one spatial token instead of
// three. It is not the fix for the #228 flash; that is opacityNeverSprings.
//
// Node environment: this reads the stylesheet, it does not render.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const RAW = readFileSync(join(dirname(fileURLToPath(import.meta.url)), "..", "index.css"), "utf8");

/* Comments are blanked before searching because the token comments mention
   this keyframe by name. Blanked rather than deleted so reported offsets still
   match the file. */
const CSS = RAW.replace(/\/\*[\s\S]*?\*\//g, (m) => m.replace(/[^\n]/g, " "));

/** The body of one @keyframes block. */
function keyframe(name: string): string {
  const at = CSS.search(new RegExp(`@keyframes\\s+${name}\\s*\\{`));
  expect(at).toBeGreaterThan(-1);
  const open = CSS.indexOf("{", at);
  let depth = 0;
  for (let i = open; i < CSS.length; i++) {
    if (CSS[i] === "{") depth++;
    else if (CSS[i] === "}") {
      depth--;
      if (depth === 0) return CSS.slice(open + 1, i);
    }
  }
  throw new Error(`unbalanced @keyframes ${name}`);
}

describe("the page entrance", () => {
  const body = keyframe("glim-page-in");

  it("still moves, so this is a flatter entrance and not a dropped one", () => {
    // Otherwise a keyframe that lost its transform altogether would pass.
    expect(body).toMatch(/translateY\(/);
    expect(body).toMatch(/--motion-page-travel/);
  });

  it("does not scale", () => {
    expect(body).not.toMatch(/\bscale\(/);
  });

  it("does not rotate", () => {
    expect(body).not.toMatch(/\brotate\(/);
  });

  it("leaves no dead tokens behind for either of them", () => {
    // A token declared per level but read by nothing invites tuning that has
    // no effect.
    expect(CSS).not.toMatch(/--motion-page-scale/);
    expect(CSS).not.toMatch(/--motion-page-tilt/);
  });
});
