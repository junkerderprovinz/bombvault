// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// StickyActionBar dom tests; the chrome contract of the shared sticky-in-flow
// bar. Assert class, not computed layout; jsdom has no layout engine, so
// "sticky works" is pinned as (a) the sticky/bottom-0 classes present and (b)
// the fixed-positioning anti-pattern absent (the locked shell discipline),
// plus the chrome contract it shares with the BottomNav precedent (sidebar
// surface, safe-area bottom padding), including the absence of a top
// hairline: surfaces in this app are told apart by shade.
// ---------------------------------------------------------------------------

import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { StickyActionBar } from "./StickyActionBar";

describe("StickyActionBar", () => {
  it("renders its children", () => {
    const { getByRole, getByText } = render(
      <StickyActionBar>
        <button type="button" role="button">
          Save folders
        </button>
      </StickyActionBar>
    );
    expect(getByRole("button")).toBeTruthy();
    expect(getByText("Save folders")).toBeTruthy();
  });

  it("is a normal-flow sticky element; sticky bottom-0 classes present, position:fixed absent", () => {
    const { container } = render(
      <StickyActionBar>
        <div>row</div>
      </StickyActionBar>
    );
    const bar = container.firstElementChild as HTMLElement;
    expect(bar.className).toContain("sticky");
    expect(bar.className).toContain("bottom-0");
    // The locked discipline: never fixed chrome (fights the visualViewport
    // keyboard mechanism; restated at the component header).
    expect(bar.className).not.toMatch(/(^|\s)fixed(\s|$)/);
  });

  it("carries the chrome classes; sidebar surface, no hairline of its own, plain 12px bottom padding", () => {
    const { container } = render(
      <StickyActionBar>
        <div>row</div>
      </StickyActionBar>
    );
    const bar = container.firstElementChild as HTMLElement;
    expect(bar.className).toContain("bg-carbon-sidebar");
    // No top hairline: surfaces in this app are told apart by shade, and the
    // bar sits on the page it overlays.
    expect(bar.className).not.toContain("border-t");
    expect(bar.className).not.toContain("border-carbon-border");
    // Plain pb-3, and deliberately NO safe-area inset: the bar is sticky
    // inside main#bv-main, and BottomNav, main's flex sibling below it,
    // owns the home-indicator inset on its own host, so the bar never
    // reaches the screen edge. Reserving the inset here made the bar 22px
    // taller than every other phone chrome row on devices that have one
    // on devices that have one.
    expect(bar.className).toContain("pb-3");
    expect(bar.className).not.toContain("safe-area-bottom");
  });

  it("appends caller className after its own (callers add visibility gates)", () => {
    const { container } = render(
      <StickyActionBar className="md:hidden">
        <div>row</div>
      </StickyActionBar>
    );
    const bar = container.firstElementChild as HTMLElement;
    expect(bar.className).toContain("md:hidden");
    expect(bar.className).toContain("sticky");
  });
});
