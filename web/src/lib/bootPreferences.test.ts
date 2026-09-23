// Every applyStored* helper in lib/ restores a preference before first paint,
// so main.tsx has to call each one. A missing call does not show: components
// read their mode from localStorage and render correctly, and changing the
// setting applies the attribute, so only CSS keyed off the attribute breaks,
// and only after a reload. Finding the helpers by name covers the next one the
// day it is written.
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

const LIB = join(__dirname);
const MAIN = join(__dirname, "..", "main.tsx");

function bootHelpers(): string[] {
  const names: string[] = [];
  for (const file of readdirSync(LIB)) {
    if (!file.endsWith(".ts") || file.includes(".test.")) continue;
    const src = readFileSync(join(LIB, file), "utf8");
    for (const m of src.matchAll(/export function (applyStored\w+)\s*\(/g)) {
      names.push(m[1]);
    }
  }
  return names.sort();
}

describe("boot-time preferences", () => {
  it("finds the helpers (the scan is not silently empty)", () => {
    // theme, language, accent, rainbow, shape, motion and labels
    expect(bootHelpers().length).toBeGreaterThanOrEqual(7);
  });

  it("main.tsx calls every applyStored* helper lib/ exports", () => {
    const main = readFileSync(MAIN, "utf8");
    const missing = bootHelpers().filter((n) => !new RegExp(`\\b${n}\\s*\\(`).test(main));
    // A list rather than a count, so the failure names the missing helper.
    expect(missing).toEqual([]);
  });

  it("imports each of them too, so the call cannot be a stale identifier", () => {
    const main = readFileSync(MAIN, "utf8");
    const unimported = bootHelpers().filter(
      (n) => !new RegExp(`import\\s*\\{[^}]*\\b${n}\\b[^}]*\\}`, "s").test(main),
    );
    expect(unimported).toEqual([]);
  });
});
