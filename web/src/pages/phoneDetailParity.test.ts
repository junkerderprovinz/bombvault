// A phone detail replaces the desktop row below the breakpoint, so a control
// the row gains and the detail lacks is a feature the phone cannot reach.
// This reads source; it does not render.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

const HERE = dirname(fileURLToPath(import.meta.url));

// The source of one top-level function, from its declaration to the next one.
function functionSource(file: string, name: string): string {
  const text = readFileSync(join(HERE, file), "utf8");
  const start = text.search(new RegExp(`^(export )?function ${name}[({]`, "m"));
  expect(start, `${file} has no function ${name}`).toBeGreaterThanOrEqual(0);
  const rest = text.slice(start + 1);
  const next = rest.search(/^(export )?function /m);
  return next < 0 ? rest : rest.slice(0, next);
}

function jsxComponents(source: string): Set<string> {
  return new Set([...source.matchAll(/(?<![\w.])<([A-Z]\w*)/g)].map((m) => m[1]));
}

describe("a phone detail carries every control of its desktop row", () => {
  it.each([
    // The container chips and their editors come in through ContainerSectionChips.
    ["Containers.tsx", "ContainerRow", "MobileContainerDetail", ["Selector"]],
    ["VMs.tsx", "VMRow", "MobileVMDetail", []],
  ])("%s: %s against %s", (file, row, detail, elsewhere) => {
    const desktop = jsxComponents(functionSource(file, row));
    const phone = jsxComponents(functionSource(file, detail));
    const missing = [...desktop].filter((c) => !phone.has(c) && !elsewhere.includes(c));
    expect(missing, "on the desktop row but not in the phone detail").toEqual([]);
  });
});
