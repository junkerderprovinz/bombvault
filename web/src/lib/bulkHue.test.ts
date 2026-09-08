// The two promises this round made about colour, checked against the source
// rather than trusted (jdp, 2026-09-08: "ja auch einfärben aber die zeilen
// sollen nicht eingefärbt werden, das ist dann zu farbig").
//
// Both are call-site facts — which classes and props a page passes — so there
// is no rendered artefact to inspect short of mounting three whole pages with
// their API layer stubbed. This reads the files instead, and it parses rather
// than greps: it isolates the one `<Button …/>` block that carries a given
// labelKey and asks about THAT block, so a hueIndex on a neighbouring button
// cannot satisfy the assertion. A line-wise regex would have passed on exactly
// that mistake.
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

describe("the list rows are no longer washed in their hue", () => {
  it.each(PAGES)("%s applies no glim-tint", (page) => {
    const src = read(page);
    // The class may still be MENTIONED in a comment explaining why it went;
    // what must not survive is an application of it inside a className.
    const applied = [...src.matchAll(/className=[^\n]*glim-tint/g)];
    expect(applied).toHaveLength(0);
  });

  it("keeps glim-hue on the rows, so controls inside them still take a position", () => {
    // Dropping both would have been the easy over-correction: the wash is what
    // was too much colour, not the row owning a position.
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
    // The point of keying the position to the ACTION rather than to a per-page
    // counter: "Back up selected" must not be one colour on Containers and
    // another on VMs.
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
