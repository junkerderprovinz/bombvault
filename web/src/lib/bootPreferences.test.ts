// ---------------------------------------------------------------------------
// Every stored appearance preference is applied before the first paint.
//
// Found live, not by reading code: the running instance had no
// `data-labels-sidebar` attribute on <html> at all. `controls.ts` exports
// `applyStoredLabelModes`, its own doc says "called at boot in main.tsx before
// first render", and nothing called it — the label engine shipped with that
// half wired to nowhere for the whole round.
//
// Why it looked fine anyway, which is exactly why a test has to hold it: the
// components read the mode through `useLabelMode` (localStorage, read
// synchronously), so the rendering was right in every mode. And `setLabelMode`
// DOES set the attribute, so changing the setting made it appear — until the
// next reload dropped it again. Same app, same setting, attribute sometimes
// there and sometimes not, with nothing on screen to give it away.
//
// Today no CSS keys off `data-labels-*`, so nothing was visibly broken. That is
// the whole hazard: the first rule someone writes against it will work while
// they are testing (they just changed the setting) and break on reload.
//
// The test is deliberately generic rather than a line about label modes: every
// `applyStored*` helper in lib/ is a boot-time preference by construction, so a
// future eighth one is covered the day it is written, not the day someone
// notices it never ran.
// ---------------------------------------------------------------------------
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
    // Seven today: theme, language, accent, rainbow, shape, motion, labels.
    expect(bootHelpers().length).toBeGreaterThanOrEqual(7);
  });

  it("main.tsx calls every applyStored* helper lib/ exports", () => {
    const main = readFileSync(MAIN, "utf8");
    const missing = bootHelpers().filter((n) => !new RegExp(`\\b${n}\\s*\\(`).test(main));
    // Named in the failure, so the message says WHICH preference never runs
    // rather than only that the count is wrong.
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
