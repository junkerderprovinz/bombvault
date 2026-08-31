// ---------------------------------------------------------------------------
// Two guards over the glyph set and the buttons that wear it ([331], [332]).
//
// Both exist because the failures they catch are SILENT. Nothing throws, no
// test goes red, the page renders — it just renders slightly wrong, and the
// only detection method so far has been jdp noticing across five rounds of
// live review. A guard that fires in CI is a cheaper reviewer.
//
// These read source text rather than rendering. That is deliberate for the
// second one: the question is not "what colour is this button now" (a rendered
// test answers that for one state on one page) but "does any call site carry a
// literal that could beat its tone" — a property of the code, checkable in one
// pass over the tree.
// ---------------------------------------------------------------------------
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

const SRC = join(__dirname, "..");

function walk(dir: string, out: string[] = []): string[] {
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) walk(full, out);
    else if (/\.tsx$/.test(entry) && !/\.test\.tsx$/.test(entry)) out.push(full);
  }
  return out;
}

/** Every `<Button …>` opening tag in the tree, with its file and line. */
function buttonTags(): { file: string; line: number; props: string }[] {
  const out: { file: string; line: number; props: string }[] = [];
  for (const file of walk(SRC)) {
    const s = readFileSync(file, "utf8");
    const re = /<Button\b/g;
    let m: RegExpExecArray | null;
    while ((m = re.exec(s))) {
      // Scan to the '>' that closes the opening tag, ignoring any inside the
      // braces of a prop value — a naive indexOf('>') stops at the first arrow
      // function and reports a fraction of the props.
      let depth = 0;
      let i = re.lastIndex;
      let end = -1;
      for (; i < s.length; i++) {
        const c = s[i];
        if (c === "{") depth++;
        else if (c === "}") depth--;
        else if (c === ">" && depth === 0) {
          end = i;
          break;
        }
      }
      if (end < 0) continue;
      out.push({
        file: file.slice(SRC.length + 1).replace(/\\/g, "/"),
        line: s.slice(0, m.index).split("\n").length,
        props: s.slice(re.lastIndex, end),
      });
    }
  }
  return out;
}

describe("glyph sizing", () => {
  // [331]: the crop test next door proves the viewBox is computed correctly
  // FROM the declared ink. It cannot prove the declared ink is right — a
  // mis-measured import produces a perfectly consistent, perfectly wrong
  // result. What catches that is the consequence: a glyph whose declared ink
  // is too small crops to a box larger than its drawing and renders visibly
  // smaller than everything beside it.
  //
  // Read out of the generator rather than the emitted file, because that is
  // where a human types the number a human measured.
  const gen = readFileSync(join(SRC, "..", "..", "scripts", "gen_glyphs.py"), "utf8");
  const entries = [...gen.matchAll(
    /imported\(\s*"(\w+)"[^)]*?\(([\d.]+),\s*([\d.]+),\s*([\d.]+),\s*([\d.]+)\)/gs,
  )];

  it("declares an ink box for every imported glyph", () => {
    expect(entries.length).toBeGreaterThan(0);
  });

  it.each(entries.map((e) => [e[1], Number(e[4]), Number(e[5])] as const))(
    "%s declares a plausible ink box",
    (name, w, h) => {
      // Zero or negative is a typo; the crop would divide by it.
      expect(w, `${name} has no declared width`).toBeGreaterThan(0);
      expect(h, `${name} has no declared height`).toBeGreaterThan(0);
      // Nothing in this set is more than 4:1. A ratio past that is a
      // transposed or half-copied measurement, which crops to a box four
      // times too big in one direction and renders a quarter of the size.
      const ratio = Math.max(w / h, h / w);
      expect(ratio, `${name} is ${ratio.toFixed(1)}:1 — measurement transposed?`).toBeLessThan(4);
    },
  );
});

describe("buttons", () => {
  // [332]: `tone` sets the background; a `bg-*` class in the call site's own
  // className sets it too. Which one wins is decided by their order in the
  // COMPILED stylesheet, not by the order they appear in the attribute — so
  // the class list reads correctly and the button paints wrong. It cost a
  // round: [326] was set to accent, shipped grey, and only measuring the
  // deployed page found it.
  //
  // The three call sites that restated a neutral surface were harmless right
  // up until somebody set a tone on one of them, which is the definition of a
  // trap rather than a style issue.
  it("never lets a call site paint its own background", () => {
    const offenders = buttonTags()
      .filter((b) => /className/.test(b.props))
      .filter((b) => /\bbg-(?!accent\b)[\w-]+/.test(b.props))
      .map((b) => `${b.file}:${b.line}`);
    expect(
      offenders,
      "These Buttons set a background in className, which silently overrides `tone`. " +
        "Use tone, or extend the tone table if a genuinely new surface is needed.",
    ).toEqual([]);
  });

  it("gives every button a labelKey, even if the answer is null", () => {
    // TypeScript already requires the prop ([335]); this catches the other
    // half — a call site that satisfies the compiler by writing `null` where a
    // real key exists. It cannot know the intent, so it only checks the prop
    // is present, and stands as the record of why it is required at all.
    const missing = buttonTags()
      .filter((b) => !/\blabelKey\b/.test(b.props))
      .map((b) => `${b.file}:${b.line}`);
    expect(missing, "Buttons without labelKey cannot pick a glyph.").toEqual([]);
  });
});
