// The page column ends where the rail ends: bottom padding inside the scroll
// container can never be scrolled away and leaves the last card short of the
// rail. This reads Layout.tsx, because jsdom does no layout and a render test
// could only compare the same class string.
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const layout = readFileSync(join(here, "Layout.tsx"), "utf8");

// The per-route wrapper inside <main>, the only element with glim-page-enter.
const pageWrapper = /className="glim-page-enter([^"]*)"/.exec(layout);

describe("the scrolling page column", () => {
  it("has a page wrapper", () => {
    expect(pageWrapper).not.toBeNull();
  });

  it("carries no bottom padding, so the last card ends level with the rail", () => {
    const cls = pageWrapper![1];
    expect(cls).toContain("pb-0");
    // `p-6` covers the other three sides; no pb-N or py-N may override it.
    expect(/\bpb-(?!0\b)\d/.test(cls)).toBe(false);
    expect(/\bpy-\d/.test(cls)).toBe(false);
  });
});
