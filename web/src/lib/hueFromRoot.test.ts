// Components take their hue through hueVars(), which points at the root's
// --rb-* properties. Nothing re-renders when the palette moves, so a component
// that read a colour itself would keep the first one it saw.
import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";
import { expect, it } from "vitest";

const SRC = join(dirname(fileURLToPath(import.meta.url)), "..");

function components(dir: string, out: string[] = []): string[] {
  for (const name of readdirSync(dir)) {
    const full = join(dir, name);
    if (statSync(full).isDirectory()) components(full, out);
    else if (name.endsWith(".tsx") && !/\.test\.tsx$/.test(name)) out.push(full);
  }
  return out;
}

it("leaves reading palette colours to the root", () => {
  const readers = components(SRC)
    .filter((f) => /\brainbow(Color)?At\s*\(/.test(readFileSync(f, "utf8")))
    .map((f) => relative(SRC, f));
  expect(readers).toEqual([]);
});
