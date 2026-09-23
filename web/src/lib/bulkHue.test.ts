// Bars coloured, rows not washed: both are call-site facts, which classes and
// props a page passes, so this reads the page sources instead of mounting whole
// pages. It isolates the `<Button …/>` block carrying a given labelKey and
// asserts on that block alone, so a hueIndex on a neighbouring button cannot
// satisfy it the way it would a line-wise regex.
import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

import { BULK_HUE } from "./bulkHue";

const PAGES = [
  "src/pages/Containers.tsx",
  "src/pages/VMs.tsx",
  "src/pages/Files.tsx",
  "src/pages/Fleet.tsx",
  "src/pages/Receiver.tsx",
];

function read(path: string): string {
  return readFileSync(path, "utf8");
}

/** The `<Button …/>` block carrying this labelKey, or null. */
function buttonBlock(source: string, labelKey: string): string | null {
  for (const m of source.matchAll(/<Button\b[\s\S]*?\/>/g)) {
    if (m[0].includes(`labelKey="${labelKey}"`)) return m[0];
  }
  return null;
}

describe("list rows are not washed in their hue", () => {
  it.each(PAGES)("%s applies no glim-tint", (page) => {
    const src = read(page);
    // Only a className counts; a comment may still name the class.
    const applied = [...src.matchAll(/className=[^\n]*glim-tint/g)];
    expect(applied).toHaveLength(0);
  });

  it("keeps glim-hue on the rows, so controls inside them still take a position", () => {
    // Rows still own a palette position; they are only not washed in it.
    for (const page of PAGES) {
      expect(read(page)).toMatch(/className=[^\n]*glim-hue glim-stagger-row/);
    }
  });
});

describe("the bulk-action bars carry a palette position", () => {
  const CASES: Array<[string, string, keyof typeof BULK_HUE]> = [
    ["src/pages/Containers.tsx", "schedule.includeAll", "include"],
    ["src/pages/Containers.tsx", "containers.discover", "discover"],
    ["src/pages/Containers.tsx", "containers.backupSelected", "backup"],
    ["src/pages/Containers.tsx", "containers.restoreSelected", "restore"],
    ["src/pages/VMs.tsx", "schedule.includeAll", "include"],
    ["src/pages/VMs.tsx", "containers.discover", "discover"],
    ["src/pages/VMs.tsx", "vms.backupSelected", "backup"],
    ["src/pages/VMs.tsx", "vms.restoreSelected", "restore"],
    ["src/pages/Files.tsx", "files.backupAll", "backup"],
  ];

  it.each(CASES)("%s: %s takes BULK_HUE.%s", (page, labelKey, hue) => {
    const block = buttonBlock(read(page), labelKey);
    expect(block).not.toBeNull();
    expect(block).toContain(`hueIndex={BULK_HUE.${hue}}`);
  });

  it("gives the same action the same colour on every page", () => {
    // Positions are keyed to the action, not counted per page, so "Back up
    // selected" has one colour everywhere.
    const containers = buttonBlock(read("src/pages/Containers.tsx"), "containers.backupSelected");
    const vms = buttonBlock(read("src/pages/VMs.tsx"), "vms.backupSelected");
    const files = buttonBlock(read("src/pages/Files.tsx"), "files.backupAll");
    for (const block of [containers, vms, files]) {
      expect(block).toContain("hueIndex={BULK_HUE.backup}");
    }
  });

  it("keeps the four positions distinct, or two actions would share a colour", () => {
    const values = Object.values(BULK_HUE);
    expect(new Set(values).size).toBe(values.length);
  });
});
