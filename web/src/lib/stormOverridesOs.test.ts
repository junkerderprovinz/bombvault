// The storm motion level outranks the operating system.
//
// The three offered levels sit inside `@media (prefers-reduced-motion:
// no-preference)`, so a browser reporting reduced motion never evaluates their
// data-motion selectors. "storm" is a hidden level reached by clicking the
// same option five times, so choosing it is a statement of intent rather than
// an inherited default.
//
// Its gate lives in the `reduce` block. That block swaps in gentler substitutes
// (a fade instead of a shake, a fade instead of the logo shattering), and those
// are exactly the effects that make the storm, so each substitute excludes
// storm and storm gets the full animation back in the same block. The
// micro-interactions (press, lift, spinner) stay OS-gated.
//
// Node environment: this reads the stylesheet, it does not render.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const HERE = dirname(fileURLToPath(import.meta.url));
const css = readFileSync(join(HERE, "..", "index.css"), "utf8");

/** The body of the `@media (prefers-reduced-motion: reduce)` block that
 *  carries the substitutes, found by brace matching rather than by a line
 *  number so an edit above it cannot silently point this test at nothing. */
function reduceBlock(): string {
  const needle = "@media (prefers-reduced-motion: reduce)";
  let from = -1;
  // The file has several such blocks; the substitutes live in the last one.
  for (let i = css.indexOf(needle); i !== -1; i = css.indexOf(needle, i + 1)) from = i;
  expect(from).toBeGreaterThan(-1);
  const open = css.indexOf("{", from);
  let depth = 0;
  for (let i = open; i < css.length; i++) {
    if (css[i] === "{") depth++;
    else if (css[i] === "}") {
      depth--;
      if (depth === 0) return css.slice(open + 1, i);
    }
  }
  throw new Error("unbalanced reduce block");
}

/** Every substitute in that block, as selector plus declaration. */
function substitutes(): { selector: string; body: string }[] {
  const out: { selector: string; body: string }[] = [];
  const body = reduceBlock().replace(/\/\*[\s\S]*?\*\//g, "");
  for (const m of body.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    const selector = m[1].trim();
    if (selector) out.push({ selector, body: m[2].trim() });
  }
  return out;
}

/** The body of the `@media (prefers-reduced-motion: no-preference)` block that
 *  holds the full animations, so the tests below can ask what "restored"
 *  actually means instead of hard-coding a list that drifts. */
function noPreferenceBlock(): string {
  const needle = "@media (prefers-reduced-motion: no-preference)";
  for (let i = css.indexOf(needle); i !== -1; i = css.indexOf(needle, i + 1)) {
    const open = css.indexOf("{", i);
    let depth = 0;
    for (let j = open; j < css.length; j++) {
      if (css[j] === "{") depth++;
      else if (css[j] === "}") {
        depth--;
        if (depth === 0) {
          const body = css.slice(open + 1, j);
          if (body.includes(".glim-egg-boom")) return body;
          i = j;
          break;
        }
      }
    }
  }
  throw new Error("no-preference block holding the egg not found");
}

/** Every animation name a block applies to the shatter egg's debris layers:
 *  the fragment tiles, the flash/shockwave pseudo elements, the cloud puffs
 *  and the sparks.
 *
 *  Names alone would also accept the five rules with their gates inverted (the
 *  full explosion for the reduced-motion user and nothing for the storm), since
 *  the set of names would be the same. `gate` says which selectors may
 *  contribute. */
function boomAnimations(body: string, gate?: RegExp): Set<string> {
  const out = new Set<string>();
  const clean = body.replace(/\/\*[\s\S]*?\*\//g, "");
  for (const m of clean.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    const selector = m[1];
    if (!/\.glim-(frag|boom-fx|cloud|particle)\b/.test(selector)) continue;
    if (gate && !gate.test(selector)) continue;
    const anim = /animation:\s*([\w-]+)/.exec(m[2]);
    if (anim?.[1]) out.add(anim[1]);
  }
  return out;
}

describe("the reduce block", () => {
  it("still carries substitutes rather than switching motion off", () => {
    // If this block ever becomes `animation: none` everywhere, the storm
    // exemption below needs rethinking.
    const subs = substitutes();
    expect(subs.length).toBeGreaterThan(4);
    expect(subs.some((s) => /animation:/.test(s.body))).toBe(true);
  });

  it("exempts the storm from every gentler substitute", () => {
    const offenders = substitutes()
      .filter((s) => /animation:|display:|opacity:/.test(s.body))
      .filter((s) => !/:root\[data-motion="storm"\]/.test(s.selector))
      .filter((s) => !/:root:not\(\[data-motion="storm"\]\)/.test(s.selector))
      .map((s) => s.selector);
    expect(offenders).toEqual([]);
  });

  it("gives the storm the full animation back, not just an exemption", () => {
    // Exempting without restoring would leave a storm user with no animation
    // on these elements under reduced motion, worse than the gentle
    // substitute.
    const subs = substitutes();
    const storm = subs.filter((s) => /:root\[data-motion="storm"\]/.test(s.selector));
    expect(storm.length).toBeGreaterThan(3);

    /** The element a gated selector targets, without its :root prefix. */
    const target = (sel: string) => sel.replace(/:root(:not\()?\[data-motion="storm"\]\)?\s*/g, "").trim();
    const gentle = new Map(
      subs
        .filter((s) => /:root:not\(\[data-motion="storm"\]\)/.test(s.selector))
        .map((s) => [target(s.selector), s.body]),
    );

    // Not "contains no fade": the modal backdrop fades at full motion too,
    // just for longer, so banning the keyword would fail on correct code.
    // What has to hold is that storm does not get the same declaration the
    // reduced-motion substitute uses.
    let compared = 0;
    for (const rule of storm) {
      const sub = gentle.get(target(rule.selector));
      if (sub === undefined) continue;
      compared += 1;
      expect(rule.body).not.toBe(sub);
    }
    expect(compared).toBeGreaterThan(3);
  });

  it("animates every layer it stops hiding, because their resting state is invisible", () => {
    // The test above only compares selectors present on both sides, so it
    // misses a target that is exempted and never restored. The shatter egg's
    // layers (`.glim-frag`, `.glim-cloud`, `.glim-particle` and both
    // `.glim-boom-fx` pseudo elements) rest at `opacity: 0` and appear only
    // through their animation. Lifting `display: none` alone would make the
    // logo vanish for the 1.4s of Sidebar.tsx's timer with no debris in its
    // place.
    const hidden = substitutes().filter((s) => /display:\s*none/.test(s.body));
    expect(hidden.length).toBeGreaterThan(0);

    // Derived from the no-preference block rather than listed here, so a new
    // boom layer added there has to be answered here too.
    const full = boomAnimations(noPreferenceBlock());
    expect(full.size).toBeGreaterThan(2);

    // Storm-gated selectors only, so inverted gates fail here.
    const restored = boomAnimations(reduceBlock(), /:root\[data-motion="storm"\]/);
    const missing = [...full].filter((name) => !restored.has(name));
    expect(missing).toEqual([]);

    // And the other direction: no debris layer may be animated for everybody
    // in here, which is what an inverted gate looks like. The reduced-motion
    // user's substitute for the whole egg is the logo's quiet fade-pulse.
    const ungated = boomAnimations(reduceBlock(), /^(?!.*\[data-motion=)/s);
    expect([...ungated]).toEqual([]);
  });
});

describe("the three offered levels stay OS-gated", () => {
  it("keeps the main motion rules inside the no-preference block", () => {
    const needle = "@media (prefers-reduced-motion: no-preference)";
    let found = false;
    for (let i = css.indexOf(needle); i !== -1; i = css.indexOf(needle, i + 1)) {
      const open = css.indexOf("{", i);
      let depth = 0;
      for (let j = open; j < css.length; j++) {
        if (css[j] === "{") depth++;
        else if (css[j] === "}") {
          depth--;
          if (depth === 0) {
            const body = css.slice(open + 1, j);
            if (body.includes(".glim-btn:active") && body.includes(".glim-stagger-row")) found = true;
            i = j;
            break;
          }
        }
      }
    }
    expect(found).toBe(true);
  });
});
