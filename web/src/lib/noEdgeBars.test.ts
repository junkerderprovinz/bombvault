// GlimStone rule 5: no vertical marks, no leading-edge bars and no accent
// stripes. A row that is picked is filled, and a severity is said by its badge.
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
    } else if (/\.(tsx?|css)$/.test(entry) && !/\.test\.tsx?$/.test(entry)) {
      out.push(path);
    }
  }
  return out;
}

it("draws no bar on the leading edge of a row anywhere in src", () => {
  const bar = /inset[_ ]-?\d+px[_ ]0[_ ]0|border-[sl]-(\d|\[)/;
  const offenders = sources(SRC)
    .filter((path) => bar.test(readFileSync(path, "utf8")))
    .map((path) => path.slice(SRC.length + 1));
  expect(offenders, "Fill what is picked and let a badge say the severity, GlimStone rule 5.").toEqual([]);
});
