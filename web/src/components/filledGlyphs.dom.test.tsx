// @vitest-environment jsdom
// Every glyph is a filled shape. A stroked outline sits at a different weight
// than the filled badges and switches around it, and a set that mixes the two
// reads as two icon libraries (GlimStone, "Icon glyphs"). The source is read,
// the generated sets included, and the folder picker is rendered, since its
// glyphs come from three files.
import { dirname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import { blankComments, readSource, walkTsx } from "./sourceTree.testsupport";

vi.mock("../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/api")>()),
  browse: async () => ({ ok: true, dirs: [{ name: "appdata", path: "user/appdata" }] }),
}));

const { FolderBrowser } = await import("./FolderBrowser");

const SRC = join(dirname(fileURLToPath(import.meta.url)), "..");

/** Drawings that are stroked on purpose, by file, with how many each holds. */
const STROKED: Record<string, { count: number; reason: string }> = {
  "components/InfoBubble.tsx": {
    count: 1,
    reason: "the (i) of the info bubble, explanatory furniture rather than a control, and the one stroked glyph",
  },
  "pages/Dashboard.tsx": {
    count: 1,
    reason: "the storage trend line, a chart drawn at a hairline weight (GlimStone, Charts), not a glyph",
  },
};

/** An attribute's value as written: `a="x"`, `a={"x"}` or the expression in `a={x}`. */
function attr(attrs: string, name: string): string | undefined {
  const m = new RegExp(String.raw`(?:^|\s)${name}\s*=\s*(?:"([^"]*)"|\{\s*"([^"]*)"\s*\}|\{([^}]*)\})`).exec(attrs);
  return m ? (m[1] ?? m[2] ?? m[3]) : undefined;
}

const SHAPE = /<(path|circle|rect|ellipse|polygon|polyline|line)\b([^>]*?)\/?>/g;

/** Whether a drawing paints any shape as an outline: stroked, and with no fill
 *  of its own or from its <svg>. A shape without a fill anywhere is filled
 *  black by SVG itself, so only an explicit "none" makes an outline. */
function strokes(svgAttrs: string, body: string): boolean {
  const svgFill = attr(svgAttrs, "fill");
  const svgStroke = attr(svgAttrs, "stroke");
  for (const [, tag, attrs] of body.matchAll(SHAPE)) {
    const stroke = attr(attrs, "stroke") ?? svgStroke;
    const fill = attr(attrs, "fill") ?? svgFill;
    const stroked = (stroke !== undefined && stroke !== "none") || /\bstroke-(?:current|\[)/.test(attr(attrs, "className") ?? "");
    const outline = tag === "line" || tag === "polyline" || fill === "none" || /\bfill-none\b/.test(attr(attrs, "className") ?? "");
    if (stroked && outline) return true;
  }
  return false;
}

/** Every drawing in the source: each <svg>, and in the generated sets each
 *  <G> block, which takes the fill of the one <svg> inside G. */
function drawings(): { file: string; line: number; stroked: boolean }[] {
  const out: { file: string; line: number; stroked: boolean }[] = [];
  for (const path of walkTsx(SRC)) {
    const file = relative(SRC, path).replace(/\\/g, "/");
    const src = blankComments(readSource(path));
    const line = (i: number) => src.slice(0, i).split("\n").length;
    for (const m of src.matchAll(/<svg\b([^>]*)>([\s\S]*?)<\/svg>/g)) {
      out.push({ file, line: line(m.index), stroked: strokes(m[1], m[2]) });
    }
    const wrapper = /function G\([\s\S]*?<svg\b([^>]*)>/.exec(src);
    if (!wrapper) continue;
    for (const m of src.matchAll(/<G\b[^>]*>([\s\S]*?)<\/G>/g)) {
      out.push({ file, line: line(m.index), stroked: strokes(wrapper[1], m[1]) });
    }
  }
  return out;
}

afterEach(cleanup);

describe("glyphs", () => {
  it("tells an outline from a filled shape", () => {
    expect(strokes('fill="none" stroke="currentColor"', '<path d="M1 1L2 2" />')).toBe(true);
    expect(strokes('fill="none"', '<circle r="7" stroke="currentColor" />')).toBe(true);
    expect(strokes("", '<polyline points="0,0 1,1" stroke="currentColor" />')).toBe(true);
    // The disclosure chevron: an unfilled <svg> whose one shape fills itself.
    expect(strokes('fill="none"', '<path fill="currentColor" d="M4 1.3 8.5 6 4 10.7Z" />')).toBe(false);
    // A filled triangle whose corners a same-colour stroke rounds.
    expect(strokes("", '<path fill="currentColor" stroke="currentColor" strokeWidth="1.4" d="M5 1Z" />')).toBe(false);
    expect(strokes('fill="currentColor"', '<path fillRule="evenodd" strokeWidth="1" d="M0 0Z" />')).toBe(false);
  });

  it("reads the generated sets and every inline drawing", () => {
    const all = drawings();
    for (const set of ["components/glyphs.tsx", "components/navGlyphs.tsx"]) {
      expect(all.filter((d) => d.file === set).length, set).toBeGreaterThan(20);
    }
    expect(all.length).toBeGreaterThan(90);
  });

  it("draws every glyph as a filled shape", () => {
    const stroked = new Map<string, number[]>();
    for (const d of drawings().filter((d) => d.stroked)) stroked.set(d.file, [...(stroked.get(d.file) ?? []), d.line]);
    const wrong = [...stroked]
      .filter(([file, lines]) => STROKED[file]?.count !== lines.length)
      .map(([file, lines]) => `${file}:${lines.join(",")}`);
    expect(wrong, "Draw these as filled shapes: a line glyph needs real geometry, not a stroke.").toEqual([]);
    const gone = Object.keys(STROKED).filter((file) => !stroked.has(file));
    expect(gone, "no longer stroked, so take them off STROKED").toEqual([]);
  });

  it("gives the folder picker filled glyphs only", async () => {
    render(
      <I18nProvider>
        <FolderBrowser label="Restore folder" value="user" hostMountRoot="/mnt" onChange={() => {}} />
      </I18nProvider>,
    );
    await act(async () => screen.getByRole("button", { name: en["folder.browseTitle"] }).click());
    const dialog = screen.getByRole("dialog");
    const svgs = [...dialog.querySelectorAll("svg")];
    expect(svgs.length).toBeGreaterThan(0);
    for (const svg of svgs) {
      const own = [...svg.attributes].map((a) => `${a.name}="${a.value}"`).join(" ");
      expect(strokes(own, svg.innerHTML), svg.outerHTML.slice(0, 120)).toBe(false);
    }
  });
});
