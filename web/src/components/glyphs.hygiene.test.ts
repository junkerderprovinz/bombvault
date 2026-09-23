// Guards over the glyph set and the buttons that wear it. What they catch is
// silent: the page renders, just slightly wrong. They read source text rather
// than rendering, because the question is whether any call site could go
// wrong, not how one page looks.
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { buttonTags, walkTsx } from "./buttonTags.testsupport";

const SRC = join(__dirname, "..");

describe("glyph sizing", () => {
  // navGlyphs.fit.dom.test.tsx proves the viewBox follows from the declared
  // ink, not that the ink was measured right. The numbers are read from the
  // generator, where they are typed.
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
      // Nothing in this set is beyond 4:1 either way, so a ratio past that is a
      // transposed or half-copied measurement.
      const ratio = Math.max(w / h, h / w);
      expect(ratio, `${name} is ${ratio.toFixed(1)}:1, measurement transposed?`).toBeLessThan(4);
    },
  );
});

describe("buttons", () => {
  // `tone` sets a button's background, and so does a bg-* class in its
  // className. Which one wins depends on their order in the compiled
  // stylesheet, not in the attribute, so the class list reads right and the
  // button paints wrong.
  it("never lets a call site paint its own background", () => {
    const offenders = buttonTags(SRC)
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
    // TypeScript requires the prop as well. This cannot tell whether a `null`
    // hides a real key, only that the prop is there.
    const missing = buttonTags(SRC)
      .filter((b) => !/\blabelKey\b/.test(b.props))
      .map((b) => `${b.file}:${b.line}`);
    expect(missing, "Buttons without labelKey cannot pick a glyph.").toEqual([]);
  });

  it("crops the viewBox of a wholly rotated glyph", () => {
    // A rotated drawing does not fill the box its numbers claim: IconClose, the
    // plus turned 45 degrees, fills 58% of its box against the plus's 72%.
    // getBBox and getBoundingClientRect report the extent before the transform,
    // so the rule is structural: a glyph drawn as one rotated group needs a
    // cropped viewBox. A detail rotated inside an unrotated shape (a tick in a
    // circle) still fills its box and is not flagged.
    const offenders: string[] = [];
    for (const file of walkTsx(SRC)) {
      const src = readFileSync(file, "utf8");
      // One svg at a time, so the viewBox and the transform belong to the same
      // glyph.
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

  it("crops a rotated glyph to its reach", () => {
    // The eye compares reach, the distance from the centre to the furthest
    // ink. A cross cropped to its extent puts its tips on the box corners and
    // reads sqrt(2) larger than a round glyph, so an arm tip must land at half
    // the viewBox from the centre. Too wide a box makes the mark small, too
    // narrow makes it large and clips the tips.
    const offenders: string[] = [];
    for (const file of walkTsx(SRC)) {
      const src = readFileSync(file, "utf8");
      for (const m of src.matchAll(/<svg\b([^>]*)>([\s\S]*?)<\/svg>/g)) {
        const [, attrs, rawBody] = m;
        const body = rawBody.trim();
        const whole = /^<g\b[^>]*transform\s*=\s*"([^"]*)"[^>]*>([\s\S]*)<\/g>$/.exec(body);
        if (!whole || /<\/g>/.test(whole[2])) continue;
        const spin = /rotate\(\s*[-\d.]+\s+([-\d.]+)\s+([-\d.]+)\s*\)/.exec(whole[1]);
        if (!spin) continue;
        const [cx, cy] = [Number(spin[1]), Number(spin[2])];
        const vb = /viewBox\s*=\s*"([^"]+)"/.exec(attrs)?.[1]?.trim();
        if (!vb) continue;
        const half = Number(vb.split(/\s+/)[2]) / 2;

        // The tip of an arm is the midpoint of its short end; a bar's corner is
        // not where the arm points.
        let reach = 0;
        for (const r of whole[2].matchAll(/<rect\b([^>]*)>/g)) {
          const at = (k: string) =>
            Number(new RegExp(`\\b${k}\\s*=\\s*"([-\\d.]+)"`).exec(r[1])?.[1] ?? NaN);
          const [x, y, w, h] = [at("x"), at("y"), at("width"), at("height")];
          if ([x, y, w, h].some(Number.isNaN)) continue;
          const tips: [number, number][] =
            w >= h
              ? [[x, y + h / 2], [x + w, y + h / 2]]
              : [[x + w / 2, y], [x + w / 2, y + h]];
          for (const [tx, ty] of tips) reach = Math.max(reach, Math.hypot(tx - cx, ty - cy));
        }
        if (!reach) continue;
        // A tenth of a unit is a fifth of a pixel at the 20px these render at.
        if (Math.abs(reach - half) > 0.1) {
          const line = src.slice(0, m.index).split("\n").length;
          offenders.push(
            `${file.slice(SRC.length + 1).replace(/\\/g, "/")}:${line} reach ${reach.toFixed(2)} vs half-box ${half.toFixed(2)}`
          );
        }
      }
    }
    expect(
      offenders,
      "A rotated glyph's arm tips must land at half its viewBox: nearer reads small, further reads oversized and clips."
    ).toEqual([]);
  });

  it("gives a dialog heading outside its padded box the box's own inset", () => {
    // The heading notch takes its inset from the padding around it (it is
    // absolutely positioned with left and right auto). A dialog whose <h2> sits
    // outside its scrolling box, which would clip the notch, has to repeat the
    // box's horizontal padding, or the badge sits flush with the card edge.
    //
    // JSX comments are blanked, keeping the line count, so a long comment
    // between the shell and the heading cannot hide the shell.
    const blankComments = (s: string) =>
      s.replace(/\{\/\*[\s\S]*?\*\/\}/g, (m) => "\n".repeat(m.split("\n").length - 1));
    const offenders: string[] = [];
    for (const file of walkTsx(SRC)) {
      const lines = blankComments(readFileSync(file, "utf8")).split("\n");
      for (let i = 0; i < lines.length; i++) {
        if (!/<h2\b/.test(lines[i])) continue;
        const head = lines.slice(i, i + 4).join("\n");
        if (!/tone="heading" size="heading"/.test(head)) continue;
        // Is this <h2> a sibling of the padded box rather than a child? The
        // shell is the `relative … max-w-*` wrapper among the previous
        // non-blank lines; the box is the `p-N rounded-card` below.
        const above: string[] = [];
        for (let j = i - 1; j >= 0 && above.length < 3; j--) {
          if (lines[j].trim()) above.unshift(lines[j]);
        }
        const joined = above.join("\n");
        const shell = /className=[`"]([^`"]*\brelative\b[^`"]*\bmax-w-[^`"]*)/.exec(joined);
        if (!shell || /\bp[xl]?-\d/.test(shell[1])) continue;
        // A wrapper that already insets the heading makes this an ordinary
        // header row, as in ConfirmDialog, whose padded card below would
        // otherwise be taken for the heading's box.
        if (/\bpx-\d/.test(joined.slice(shell.index + shell[0].length))) continue;
        const box = /\bp-(\d+)\b/.exec(
          lines.slice(i, i + 16).filter((l) => /rounded-card|bg-carbon-surface/.test(l)).join("\n")
        );
        if (!box) continue;
        const inset = new RegExp(`\\bpx-${box[1]}\\b`).test(lines[i]);
        if (!inset) {
          offenders.push(
            `${file.slice(SRC.length + 1).replace(/\\/g, "/")}:${i + 1} needs px-${box[1]} to match its box's p-${box[1]}`
          );
        }
      }
    }
    expect(
      offenders,
      "A dialog heading rendered outside its padded box must repeat that box's horizontal padding, or its notch sits flush with the card edge."
    ).toEqual([]);
  });

  it("never paints text or graphics with the flat accent", () => {
    // `accent` is a fill meant to carry text, and `accentText` is the accent
    // mixed toward the ink so it can be the text. Flat accent gold measures
    // 1.61:1 on the light background, against 4.5:1 for body text and 3:1 for
    // a graphic.
    const offenders: string[] = [];
    for (const file of walkTsx(SRC)) {
      // Comments quote the flat name to warn against it, some across several
      // lines, so they are blanked first, keeping the line numbers.
      const src = readFileSync(file, "utf8")
        .replace(/\/\*[\s\S]*?\*\//g, (m) => "\n".repeat((m.match(/\n/g) ?? []).length))
        .replace(/\/\/.*$/gm, "");
      src.split("\n").forEach((line, i) => {
        // Only `text-accent` itself, so text-accentText and text-accentContrast
        // pass.
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
