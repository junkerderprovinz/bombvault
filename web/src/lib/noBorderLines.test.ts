// Separation comes from surface steps and spacing, never from a drawn line
// (GlimStone rules 5 and 14): no frame on a field, a button, a select or a
// window, and no rule between rows. A line looks deliberate where it is written
// and only reads as clutter on the page, so this reads every class list and
// every stylesheet rule instead of rendering a few pages.
import { dirname, join, relative, sep } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { blank, blankComments, readSource, walk } from "../components/sourceTree.testsupport";

const SRC = join(dirname(fileURLToPath(import.meta.url)), "..");

/** Source files under `dir` whose names match `ext`, without tests and without
 *  the translations, where "border" is a word and not a class. */
function sourceFiles(dir: string, ext: RegExp): string[] {
  return walk(dir).filter((full) => {
    const [name, ...dirs] = relative(dir, full).split(sep).reverse();
    return ext.test(name) && !/\.test\.|\.testsupport\./.test(name) && name !== "i18n.ts" && !dirs.includes("locales");
  });
}

const where = (file: string) => relative(SRC, file).replace(/\\/g, "/");

const lineStarts = new Map<string, number[]>();

/** The line `index` falls on. A page holds thousands of strings, so each text's
 *  line starts are found once and searched. */
function lineAt(text: string, index: number): number {
  let starts = lineStarts.get(text);
  if (!starts) {
    starts = [0];
    for (let i = text.indexOf("\n"); i !== -1; i = text.indexOf("\n", i + 1)) starts.push(i + 1);
    lineStarts.set(text, starts);
  }
  let lo = 0;
  let hi = starts.length - 1;
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1;
    if (starts[mid] <= index) lo = mid;
    else hi = mid - 1;
  }
  return lo + 1;
}

const QUOTED = /"(?:[^"\\\n]|\\.)*"|'(?:[^'\\\n]|\\.)*'/g;

type Template = { start: number; end: number; chunks: { text: string; at: number }[] };

/** Every template literal in `code`, with the static chunks of the templates
 *  nested in its `${…}` among its own. A regex would take the backtick that
 *  opens a nested template for the one that closes the outer, and skip the
 *  classes in between. */
function templates(code: string): Template[] {
  const quoted = new RegExp(QUOTED.source, "y");
  let i = 0;
  // Reads the template whose backtick is at i and leaves i past its closing one.
  const read = (chunks: Template["chunks"]) => {
    let from = ++i;
    while (i < code.length && code[i] !== "`") {
      if (code[i] === "\\") {
        i += 2;
      } else if (code.startsWith("${", i)) {
        chunks.push({ text: code.slice(from, i), at: from });
        i += 2;
        for (let depth = 0; i < code.length && (depth > 0 || code[i] !== "}"); ) {
          quoted.lastIndex = i;
          if (code[i] === "`") read(chunks);
          else if (quoted.test(code)) i = quoted.lastIndex;
          else {
            if (code[i] === "{") depth++;
            else if (code[i] === "}") depth--;
            i++;
          }
        }
        from = ++i;
      } else {
        i++;
      }
    }
    chunks.push({ text: code.slice(from, i), at: from });
    i++;
  };
  const out: Template[] = [];
  while ((i = code.indexOf("`", i)) !== -1) {
    const start = i;
    const chunks: Template["chunks"] = [];
    read(chunks);
    out.push({ start, end: i, chunks });
  }
  return out;
}

/** Every quoted string, and every static chunk of a template literal, with the
 *  line it starts on. QUOTED also finds the strings inside `${…}`. */
function literals(source: string): { text: string; line: number }[] {
  const code = blankComments(source);
  const out = [...code.matchAll(QUOTED)].map((m) => ({ text: m[0].slice(1, -1), line: lineAt(code, m.index) }));
  for (const { chunks } of templates(code)) {
    for (const { text, at } of chunks) out.push({ text, line: lineAt(code, at) });
  }
  return out;
}

/** `code` with its strings and template literals blanked, so what is left is code. */
function blankStrings(code: string): string {
  let out = "";
  let last = 0;
  for (const { start, end } of templates(code)) {
    out += code.slice(last, start) + blank(code.slice(start, end));
    last = end;
  }
  return (out + code.slice(last)).replace(QUOTED, blank);
}

/** The utility a class names once its variants (`hover:`, `md:`) and the
 *  important mark are taken off. A colon inside brackets is part of the value. */
function utility(token: string): string {
  let depth = 0;
  let cut = 0;
  for (let i = 0; i < token.length; i++) {
    if (token[i] === "[") depth++;
    else if (token[i] === "]") depth--;
    else if (token[i] === ":" && depth === 0) cut = i + 1;
  }
  return token.slice(cut).replace(/^!|!$/g, "");
}

// A width is what makes a border visible: Tailwind's preflight sets every
// border to zero, so a colour or a style class alone draws nothing.
const WIDTH = String.raw`(?:[1-9]\d*|\[[^\]]+\])`;
const LINE = new RegExp(
  String.raw`^(?:border(?:-(?:x|y|t|r|b|l|s|e|bs|be))?(?:-${WIDTH})?|divide-[xy](?:-${WIDTH})?)$`,
);

function drawsLine(classList: string): boolean {
  return classList.split(/\s+/).some((token) => LINE.test(utility(token)));
}

/** Class lists that draw a line on purpose. Each says why the line is a glyph
 *  or a state rather than a frame. */
const ALLOWED_CLASS_LISTS: { match: RegExp; reason: string }[] = [
  {
    match: /\banimate-spin\b/,
    reason: "a busy spinner: the ring is the glyph, a border with one side cleared",
  },
];

/** Stylesheet rules that draw a line on purpose, by selector. */
const ALLOWED_RULES: Record<string, string> = {
  ".glim-picker-dot": "the colour picker's position marker: a white ring is the one mark that shows on every colour under it",
  ".glim-boom-fx::after": "the shockwave of the logo's easter egg, an effect ring and not a frame",
};

type Line = { file: string; line: number; text: string };

// A style object draws a line through the same properties; the colour keys
// (`borderColor`, `borderTopColor`) draw nothing on their own.
const STYLE_KEY = /\bborder(?:Top|Right|Bottom|Left|Inline|Block)?(?:Start|End)?(?:Width|Style)?\s*:/g;

function sourceLines(): Line[] {
  const out: Line[] = [];
  for (const file of sourceFiles(SRC, /\.tsx?$/)) {
    const text = readSource(file);
    for (const { text: list, line } of literals(text)) {
      if (!drawsLine(list) || ALLOWED_CLASS_LISTS.some((a) => a.match.test(list))) continue;
      out.push({ file: where(file), line, text: list.trim() });
    }
    const code = blankStrings(blankComments(text));
    for (const m of code.matchAll(STYLE_KEY)) out.push({ file: where(file), line: lineAt(code, m.index), text: m[0] });
    // An <hr> is a line by default; it keeps its meaning only as spacing.
    for (const m of blankComments(text).matchAll(/<hr\b[^>]*>/g)) {
      if (!/\bborder-(?:0|none)\b/.test(m[0])) out.push({ file: where(file), line: lineAt(text, m.index), text: m[0] });
    }
  }
  return out;
}

// Only the properties that give a border its width or style; border-radius,
// border-color and border-collapse draw nothing.
const CSS_LINE =
  /(?:^|[;{\s])(border(?:-(?:top|right|bottom|left|inline|block)(?:-(?:start|end))?)?(?:-(?:width|style))?)\s*:\s*([^;}]*)/g;

function stylesheetLines(): Line[] {
  const out: Line[] = [];
  for (const file of sourceFiles(SRC, /\.css$/)) {
    const css = readSource(file).replace(/\/\*[\s\S]*?\*\//g, blank);
    for (const rule of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
      const selector = rule[1].trim();
      if (selector in ALLOWED_RULES) continue;
      for (const decl of rule[2].matchAll(CSS_LINE)) {
        const value = decl[2].trim();
        if (/^(?:none|0(?:px)?)$/.test(value)) continue;
        out.push({ file: where(file), line: lineAt(css, rule.index + rule[1].length), text: `${selector} { ${decl[1]}: ${value} }` });
      }
    }
  }
  return out;
}

const report = (lines: Line[]) => lines.map((l) => `  ${l.file}:${l.line}  ${l.text}`).join("\n");

describe("border lines", () => {
  it("recognises every way a class draws a line, and nothing else", () => {
    for (const cls of ["border", "border-t", "border-b-2", "md:border-x", "hover:border-2", "focus:border", "divide-y", "border-[3px]", "border-t!"]) {
      expect(drawsLine(cls), cls).toBe(true);
    }
    for (const cls of ["border-0", "last:border-0", "border-none", "border-carbon-border", "border-t-transparent", "border-dashed", "box-border", "divide-carbon-border", "border-collapse", "rounded-card"]) {
      expect(drawsLine(cls), cls).toBe(false);
    }
  });

  it("reads class lists where they are, not in comments", () => {
    const found = literals(
      'const a = "border-t";\n// "border-b"\n{/* `divide-y` */}\nconst u = "https://x"; const b = `p-2 ${on ? "border-2" : ""}`;\n' +
        'const c = `p-2 ${on ? `border-x` : ""} divide-y`;\n' +
        'const d = `${a ? `p-1 ${b ? `border-y` : ""} border-l` : ""}`;',
    );
    expect(found.filter((l) => drawsLine(l.text)).map((l) => [l.text.trim(), l.line])).toEqual([
      ["border-t", 1],
      ["border-2", 4],
      ["border-x", 5],
      ["divide-y", 5],
      ["border-y", 6],
      ["border-l", 6],
    ]);
  });

  it("reaches the source tree", () => {
    // An empty scan would pass everything below.
    expect(sourceFiles(SRC, /\.tsx?$/).length).toBeGreaterThan(100);
    expect(sourceFiles(SRC, /\.css$/).length).toBeGreaterThan(0);
  });

  it("draws no line on a field, a button, a select, a window or between rows", () => {
    const lines = sourceLines();
    expect(
      lines,
      `These draw a line:\n\n${report(lines)}\n\n` +
        "Take the line out and let a surface step (bg-carbon-surface2 on a card, the\n" +
        "window's own surface) or spacing (gap, padding) carry the separation.",
    ).toEqual([]);
  });

  it("draws no line in the stylesheet", () => {
    const lines = stylesheetLines();
    expect(lines, `These rules draw a line:\n\n${report(lines)}`).toEqual([]);
  });
});

describe("focus", () => {
  // Rules 14 and 21: a field shows focus as a brightness step, everything else
  // keeps the ring. `outline-none` is how the ring gets lost.
  const FIELD_FOCUS = /\bglim-field-focus(?:-well)?\b/;
  const ALLOWED_OUTLINE_NONE: Record<string, string> = {
    "components/NumberField.tsx": "the stepper arrows are never focused (tabIndex -1); the field around them is",
    "components/Toggle.tsx": "the switch grows to the touch floor on a coarse pointer, so its ring sits on the visible pill instead",
  };

  it("takes the ring off nothing but a field", () => {
    const lost: string[] = [];
    for (const file of sourceFiles(SRC, /\.tsx?$/)) {
      if (where(file) in ALLOWED_OUTLINE_NONE) continue;
      for (const { text, line } of literals(readSource(file))) {
        const noRing = text.split(/\s+/).some((t) => utility(t) === "outline-none");
        if (noRing && !FIELD_FOCUS.test(text)) lost.push(`${where(file)}:${line}`);
      }
    }
    expect(lost, "These drop the focus ring outside a field (GlimStone rule 21).").toEqual([]);
  });

  it("drops the ring in the stylesheet only where a field steps its fill", () => {
    const css = readSource(join(SRC, "index.css")).replace(/\/\*[\s\S]*?\*\//g, "");
    const lost: string[] = [];
    for (const rule of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
      if (!/(?:^|[;\s])outline\s*:\s*(?:none|0)\s*(?:;|$)/.test(rule[2])) continue;
      if (/:focus/.test(rule[1]) && /background-color\s*:/.test(rule[2])) continue;
      lost.push(rule[1].trim());
    }
    expect(lost, "These rules drop the focus ring outside a field (GlimStone rule 21).").toEqual([]);
  });

  it("steps a focused field's fill", () => {
    const css = readSource(join(SRC, "index.css"));
    for (const selector of [".glim-field-focus:focus", ".glim-field-focus-well:focus"]) {
      const body = new RegExp(`${selector.replace(/[.:-]/g, "\\$&")}\\s*\\{([^}]*)\\}`).exec(css)?.[1];
      expect(body, `${selector} is missing`).toBeDefined();
      expect(body).toMatch(/background-color\s*:/);
    }
  });
});
