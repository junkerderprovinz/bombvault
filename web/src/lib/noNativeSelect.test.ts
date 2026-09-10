// ---------------------------------------------------------------------------
// GlimStone rule 18, enforced instead of remembered: no native <select> in the
// app (#3425).
//
// "A native control gets replaced, not persuaded." An open <select> is drawn by
// the operating system, so no rule in this house reaches inside it — not the
// shape engine, not the palette, not the type scale. The app carried twenty-one
// of them for as long as replacing one meant hand-rolling a trigger, a panel
// and a wheel handler; with SelectField that is a five-line call, so there is
// no longer a reason for a new one to appear.
//
// A rule nobody can see broken is a rule that comes back. This test is the
// place it gets caught, in the second it is written, rather than in a live
// review a month later.
// ---------------------------------------------------------------------------
import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { expect, it } from "vitest";

const SRC = join(dirname(fileURLToPath(import.meta.url)), "..");

function sources(dir: string): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir)) {
    const path = join(dir, entry);
    if (statSync(path).isDirectory()) {
      out.push(...sources(path));
    } else if (/\.tsx?$/.test(entry) && !/\.test\.tsx?$/.test(entry)) {
      out.push(path);
    }
  }
  return out;
}

/** Comments talk ABOUT the native control all over this codebase, which is the
 *  point: they explain why it is gone. Only JSX counts, so line and block
 *  comments come out before the search. */
function code(text: string): string {
  return text.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");
}

it("has no native <select> left anywhere in src", () => {
  const offenders: string[] = [];
  for (const path of sources(SRC)) {
    const body = code(readFileSync(path, "utf8"));
    if (/<select[\s/>]/.test(body)) offenders.push(path.slice(SRC.length + 1));
  }
  expect(
    offenders,
    "Use SelectField (components/SelectField.tsx) instead — it is the app's own " +
      "replacement, and GlimStone rule 18 says a native control gets replaced, not persuaded.",
  ).toEqual([]);
});

it("finds the files at all, so an empty pass cannot mean an empty scan", () => {
  const files = sources(SRC);
  expect(files.length).toBeGreaterThan(100);
  // And the comment stripper does not eat real markup on the way past.
  expect(code("// <select>\n<select value={x} />")).toContain("<select value=");
});
