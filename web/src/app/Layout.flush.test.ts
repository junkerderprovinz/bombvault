// The page column starts and ends where the chrome does: its first row stands
// on the rail's top line, and bottom padding inside the scroll container can
// never be scrolled away and leaves the last card short of it. On the desktop that chrome is the rail, on a phone the bottom bar
// with the sticky action bar above it, so both scrollers are checked. This
// reads Layout.tsx, because jsdom does no layout and a render test could only
// compare the same class string.
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const layout = readFileSync(join(here, "Layout.tsx"), "utf8");

// The per-route wrapper inside <main>, the only element with glim-page-enter.
const pageWrapper = /className="glim-page-enter([^"]*)"/.exec(layout);

// The phone scroller: the one ternary branch whose <main> carries the 16px
// gutter class itself (the desktop main stays gutter-free; the frame owns it
// there). `\s+` between the attributes tolerates the phone branch's wrapped
// JSX (its safe-area classes make the line long enough to be split); only
// The attribute order is fixed, so the match cannot land on the
// desktop main.
const mobileMain = /<main\s+id="bv-main"\s+className="([^"]*p-4[^"]*)"/.exec(layout);

describe("the scrolling page column", () => {
  it("has a page wrapper", () => {
    expect(pageWrapper).not.toBeNull();
  });

  it("carries no bottom padding, so the last card ends level with the rail", () => {
    const cls = pageWrapper![1];
    expect(cls).toContain("pb-0");
    expect(/\bpb-(?!0\b)\d/.test(cls)).toBe(false);
    expect(/\bpy-\d/.test(cls)).toBe(false);
  });

  it("carries no top padding, so the first row starts on the rail's top line", () => {
    const cls = pageWrapper![1];
    expect(cls).toContain("pt-0");
    expect(/\b(p|pt)-(?!0\b)\d/.test(cls)).toBe(false);
    expect(cls).toContain("px-6");
  });

  it("has a phone scroller", () => {
    expect(mobileMain).not.toBeNull();
  });

  it("carries no bottom padding on the phone scroller, so the sticky action bar reaches the bottom bar", () => {
    const cls = mobileMain![1];
    expect(cls).toContain("p-4");
    expect(cls).toContain("pb-0");
    expect(/\bpb-(?!0\b)\d/.test(cls)).toBe(false);
    expect(/\bpy-\d/.test(cls)).toBe(false);
  });
});

describe("a page that opens with a card", () => {
  it("starts at the card's badge, half the 22px badge above the card", () => {
    const css = readFileSync(join(here, "..", "index.css"), "utf8");
    const rule =
      /\.glim-page-enter > \.glim-notch-card:first-child,\s*\.glim-page-enter > :first-child > \.glim-notch-card:first-child,\s*\.glim-page-enter > :first-child > \.glim-page-title:first-child \+ \.glim-notch-card\s*\{([^}]*)\}/.exec(
        css,
      );
    expect(rule?.[1]).toMatch(/margin-top:\s*11px/);
  });

  it("still finds that badge when the sr-only page heading comes first", () => {
    const css = readFileSync(join(here, "..", "index.css"), "utf8");
    expect(css).toContain(".glim-page-title:first-child + .glim-notch-card");
  });
});
