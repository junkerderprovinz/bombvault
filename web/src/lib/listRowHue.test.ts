// Where colour goes on a list: a row owns a palette position, so the controls
// inside it take that position, but the row is not washed in it. The bar of
// picked rows is the one filled accent surface, and its buttons are shades of
// it. Both are facts of which classes and props a file passes, so this reads
// the sources.
import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

const ROWS = [
  "src/components/backups/BackupRow.tsx",
  "src/pages/instances/InstanceCard.tsx",
  "src/pages/instances/ReceivedRepoCard.tsx",
];

const BAR = "src/components/backups/BulkBar.tsx";

function read(path: string): string {
  return readFileSync(path, "utf8");
}

describe("list rows are not washed in their hue", () => {
  it.each(ROWS)("%s applies no glim-tint", (file) => {
    // Only a className counts. A comment may name the class.
    expect([...read(file).matchAll(/className=[^\n]*glim-tint/g)]).toHaveLength(0);
  });

  it.each(ROWS)("%s keeps glim-hue on the row", (file) => {
    expect(read(file)).toMatch(/className=[^\n]*\bglim-hue\b/);
  });
});

describe("the bar of picked rows", () => {
  const bar = read(BAR);
  const buttons = [...bar.matchAll(/<Button\b[\s\S]*?\/>/g)].map((m) => m[0]);

  it("is filled with the accent", () => {
    expect(bar).toMatch(/data-testid="bulk-bar"\s+className="[^"]*\bbg-accent\b/);
  });

  it("gives its buttons a shade of the bar and no fill or hue of their own", () => {
    expect(buttons.length).toBeGreaterThan(0);
    for (const button of buttons) {
      expect(button).toContain('tone="subtle"');
      expect(button).toContain("className={ON_BAR}");
      expect(button).not.toContain("hueIndex");
    }
    expect(bar).not.toContain('tone="accent"');
  });
});
