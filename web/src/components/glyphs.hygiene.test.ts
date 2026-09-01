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

  it("crops the viewBox of any rotated glyph ([426])", () => {
    // A rotated drawing does not fill the box its own numbers say it does.
    // IconClose is IconAdd's plus turned 45 degrees: the plus reads at 72% of
    // its 14-unit box, the cross at 58%, because the axis-aligned extent of a
    // diagonal is smaller than the arms making it. Both crosses were the
    // smallest marks among 48 while their geometry claimed otherwise.
    //
    // Nothing caught it, and could not have: getBBox and getBoundingClientRect
    // BOTH report a transformed group's pre-transform extent, so every cheap
    // measurement agreed with every other cheap measurement. The contact sheet
    // rated the glyph 101% filled while it painted 58%.
    //
    // So the rule is structural rather than measured: if a glyph rotates, its
    // viewBox has to be cropped to what the rotation actually leaves, which
    // means it cannot still be the full nominal grid. That is checkable in the
    // source and cannot be fooled by the same blind spot twice.
    // Only a WHOLLY rotated glyph is affected, and the distinction is the
    // difference between a real guard and a nuisance. Six drawings rotate a
    // DETAIL inside themselves — a tick inside a circle, a strike-through
    // across an eye — while an unrotated outer shape still fills the box. Those
    // lose nothing and must not be flagged, or the rule gets switched off.
    //
    // The case that loses size is the one where every mark sits inside a single
    // rotated group: then the group IS the glyph, and turning it shrinks the
    // whole thing.
    const offenders: string[] = [];
    for (const file of walk(SRC)) {
      const src = readFileSync(file, "utf8");
      // Each <svg …> element with everything up to its closing bracket, plus
      // the body that follows, so the viewBox and the transform are compared
      // within one glyph rather than across neighbours.
      for (const m of src.matchAll(/<svg\b([^>]*)>([\s\S]*?)<\/svg>/g)) {
        const [, attrs, rawBody] = m;
        const body = rawBody.trim();
        // One top-level <g> carrying the rotation, and nothing outside it.
        const whole = /^<g\b[^>]*transform\s*=\s*"[^"]*rotate\([^>]*>([\s\S]*)<\/g>$/.exec(body);
        if (!whole || /<\/g>/.test(whole[1])) continue;
        const vb = /viewBox\s*=\s*"([^"]+)"/.exec(attrs)?.[1]?.trim();
        if (!vb) continue;
        const [minX, minY] = vb.split(/\s+/).map(Number);
        // An uncropped box starts at the origin. A cropped one cannot.
        if (minX === 0 && minY === 0) {
          const line = src.slice(0, m.index).split("\n").length;
          offenders.push(`${file.slice(SRC.length + 1)}:${line}`);
        }
      }
    }
    expect(
      offenders,
      "A rotated glyph keeps an uncropped viewBox, so it renders smaller than every glyph beside it."
    ).toEqual([]);
  });

  it("never paints text or graphics with the flat accent ([381])", () => {
    // accentText and accent are not two shades of one idea, they are opposites:
    // `accent` is a FILL, meant to have text on top of it, and `accentText` is
    // the accent mixed toward the ink so it can BE the text.
    //
    // Using the fill as text is not a taste question, it is measured: flat
    // accent gold sits at 1.61:1 on the light background, against 4.5 for body
    // copy and 3:1 for a graphic. The charts carry that measurement in their
    // own comment and four call sites were converted then.
    //
    // One was not. WhatsNewDialog's inline link kept `text-accent` and stayed
    // unreadable, which is the ordinary shape of this miss: the rule got
    // applied everywhere the sweep looked, and afterwards nobody could tell
    // which places it had not looked at. So the sweep is a test now, which is
    // the only version of it that runs again next time.
    const offenders: string[] = [];
    for (const file of walk(SRC)) {
      // Comments have to go BEFORE the split into lines, not after. Several of
      // them quote the flat name while explaining why not to use it, and the
      // longest — Recovery.tsx's, which carries the original measurement —
      // spans ten lines of a JSX {/* … */} block. A per-line strip cannot see
      // that it is inside one, so it flagged the very comment that documents
      // the rule. A guard that trips on its own rationale teaches people to
      // delete the rationale.
      //
      // Comment bodies become blank lines rather than disappearing, so the
      // reported line numbers still point at the real file.
      const src = readFileSync(file, "utf8")
        .replace(/\/\*[\s\S]*?\*\//g, (m) => "\n".repeat((m.match(/\n/g) ?? []).length))
        .replace(/\/\/.*$/gm, "");
      src.split("\n").forEach((line, i) => {
        // The exact utility only: `text-accent` with nothing appended, so
        // text-accentText and text-accentContrast (the deliberate ones) pass.
        if (/\btext-accent(?![A-Za-z-])/.test(line)) {
          offenders.push(`${file.slice(SRC.length + 1)}:${i + 1}`);
        }
      });
    }
    expect(
      offenders,
      "text-accent is a fill, not a text colour (1.61:1). Use text-accentText."
    ).toEqual([]);
  });
});
