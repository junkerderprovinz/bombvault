// Every tab repeats the same heading markup:
//
//   <h1 className="text-2xl font-semibold text-carbon-text">…</h1>
//   <p className="mt-1 text-sm text-carbon-textSub">…</p>
//
// Each page is correct on its own, so neither the type checker nor a unit test
// notices one drifting from its siblings. A source scan does.
import { readdirSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const PAGES = join(dirname(fileURLToPath(import.meta.url)), "..", "pages");

// Glyphs.tsx is a developer sheet reachable only by typing its route, and
// Login.tsx is the pre-authentication card with no tab and no subtitle.
const NOT_A_TAB = new Set(["Glyphs.tsx", "Login.tsx"]);

function pageFiles(): string[] {
  return readdirSync(PAGES).filter((f) => /^[A-Z].*\.tsx$/.test(f) && !/\.test\.tsx$/.test(f) && !NOT_A_TAB.has(f));
}

describe("every tab's heading", () => {
  it("finds the pages at all (so an empty scan cannot pass)", () => {
    expect(pageFiles().length).toBeGreaterThan(6);
  });

  it.each(pageFiles())("%s uses the house size and the house subtitle colour", (file) => {
    const src = readFileSync(join(PAGES, file), "utf8");
    const h1 = src.match(/<h1 className="([^"]*)"/);
    expect(h1, `${file} has no <h1 className="…">; every tab needs one heading`).not.toBeNull();
    expect(
      h1![1],
      `${file}'s heading is "${h1![1]}", want the house form "text-2xl font-semibold text-carbon-text".\n` +
        `A tab whose title is smaller than its siblings reads as less important than they are, and\n` +
        `nothing in the type system or the tests can see it - which is how Recovery kept a text-lg\n` +
        `heading until somebody looked at the two tabs side by side.`,
    ).toBe("text-2xl font-semibold text-carbon-text");

    // The subtitle under the title, where one exists. text-carbon-textMuted is
    // the dimmer token for hints inside a card, not for a page subtitle. The
    // classes are checked as a set, since their order in the markup varies.
    const after = src.slice(src.indexOf(h1![0]));
    const sub = after.match(/<p className="([^"]*)"/);
    if (sub && /text-carbon-text(Sub|Muted)/.test(sub[1])) {
      const classes = sub[1].split(/\s+/);
      expect(
        classes,
        `${file}'s subtitle is "${sub[1]}", want the house set: mt-1, text-sm and text-carbon-textSub.\n` +
          `text-carbon-textMuted is the dimmer token for hints inside a card, not for a page's own subtitle.`,
      ).toContain("text-carbon-textSub");
      expect(classes, `${file}'s subtitle is not text-sm`).toContain("text-sm");
      expect(classes, `${file}'s subtitle has no mt-1`).toContain("mt-1");
    }
  });
});
