// Opacity never rides a spring. A spring curve overshoots its target, which
// suits a distance but not opacity: the value clamps at 1, so the fade ends
// early and then sits flat. In a software rasteriser an alpha driven past 1
// also flashes (#228). Movement keeps the spring and opacity gets its own
// animation on a monotonic curve, so these checks look at the pairing of
// keyframe and curve rather than at the keyframe bodies alone.
//
// Node environment: this reads the stylesheet, it does not render.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const RAW = readFileSync(join(dirname(fileURLToPath(import.meta.url)), "..", "index.css"), "utf8");
/* Comments blanked, not deleted, so reported offsets still match the file. */
const CSS = RAW.replace(/\/\*[\s\S]*?\*\//g, (m) => m.replace(/[^\n]/g, " "));

/** The body of one @keyframes block, found by a real declaration rather than
 *  by a prose mention of its name. */
function keyframe(name: string): string {
  const at = CSS.search(new RegExp(`@keyframes\\s+${name}\\s*\\{`));
  expect(at, `@keyframes ${name} not found`).toBeGreaterThan(-1);
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

/** Every `animation:` shorthand in the file that names this keyframe. */
function usages(keyframeName: string): string[] {
  const out: string[] = [];
  for (const m of CSS.matchAll(/animation:\s*([^;]+);/g)) {
    if (new RegExp(`\\b${keyframeName}\\b`).test(m[1])) out.push(m[1].replace(/\s+/g, " ").trim());
  }
  return out;
}

/** The four keyframes driven by a per-level curve that move something. */
const MOVERS = ["glim-page-in", "glim-tab-slide", "glim-card-in", "glim-row-in"];

/** Whichever comma-separated part of an `animation:` shorthand runs `name`. */
function part(shorthand: string, name: string): string {
  return shorthand.split(",").find((p) => new RegExp(`\\b${name}\\b`).test(p)) ?? "";
}

describe("nothing fades on a spring", () => {
  // Two routes satisfy this and both are in use: a transform-only keyframe
  // paired with glim-fade-in on ease-out (the page, the tab panel, the notch
  // card), or a keyframe that fades on a curve that does not spring (the
  // staggered row, which renders in the dozens and cannot afford the parse
  // cost of a two-animation shorthand). Accepting only one route would force
  // the expensive one everywhere.
  for (const name of MOVERS) {
    it(`${name} never has opacity on --motion-page-ease`, () => {
      const fades = /opacity:/.test(keyframe(name));
      const uses = usages(name);
      expect(uses.length, `${name} is declared nowhere`).toBeGreaterThan(0);

      for (const use of uses) {
        const mine = part(use, name);
        if (fades) {
          expect(mine, `${name} fades, so its own curve must not spring: ${use}`).not.toMatch(
            /--motion-page-ease/
          );
          continue;
        }
        // Transform-only: something still has to bring the opacity, and that
        // something must not spring either.
        expect(use, `${name} moves without a paired fade: ${use}`).toMatch(/glim-fade-in/);
        expect(part(use, "glim-fade-in"), `the fade beside ${name} springs: ${use}`).not.toMatch(
          /--motion-page-ease/
        );
      }
    });
  }

  it("still moves something at every one of them", () => {
    // Otherwise four keyframes that lost their transforms would pass.
    for (const name of MOVERS) expect(keyframe(name), `${name} stopped moving`).toMatch(/transform:/);
  });
});

describe("the fade keyframe itself", () => {
  it("is declared exactly once", () => {
    // A second declaration would silently override the first.
    const count = [...CSS.matchAll(/@keyframes\s+glim-fade-in\s*\{/g)].length;
    expect(count).toBe(1);
  });

  it("fades and nothing else", () => {
    const body = keyframe("glim-fade-in");
    expect(body).toMatch(/opacity:/);
    expect(body).not.toMatch(/transform:/);
  });
});
