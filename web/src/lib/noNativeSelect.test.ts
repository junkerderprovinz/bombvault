// GlimStone rule 18: no native <select>. The operating system draws an open
// <select>, so none of the app's shape, palette or type rules reach inside it.
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

/** code strips comments, which mention <select> freely. */
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
    "Use SelectField (components/SelectField.tsx) instead; it is the app's own " +
      "replacement, and GlimStone rule 18 says a native control gets replaced, not persuaded.",
  ).toEqual([]);
});

it("scans the source tree and keeps real markup when stripping comments", () => {
  const files = sources(SRC);
  expect(files.length).toBeGreaterThan(100);
  expect(code("// <select>\n<select value={x} />")).toContain("<select value=");
});
