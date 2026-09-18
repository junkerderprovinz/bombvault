// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// StickyActionBar dom tests — the chrome contract of the shared sticky-in-flow
// bar. Assert CLASS, not computed layout — jsdom has no layout engine, so
// "sticky works" is pinned as (a) the sticky/bottom-0 classes present and (b)
// the fixed-positioning anti-pattern ABSENT (the locked shell discipline),
// plus the chrome trio (sidebar surface, top hairline, safe-area bottom
// padding) that the BottomNav precedent shares.
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

  it("is a normal-flow sticky element — sticky bottom-0 classes present, position:fixed absent", () => {
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

  it("carries the chrome classes — sidebar surface, top hairline, safe-area bottom padding", () => {
    const { container } = render(
      <StickyActionBar>
        <div>row</div>
      </StickyActionBar>
    );
    const bar = container.firstElementChild as HTMLElement;
    expect(bar.className).toContain("bg-carbon-sidebar");
    expect(bar.className).toContain("border-t");
    expect(bar.className).toContain("border-carbon-border");
    // Safe area: the BottomNav padding recipe, clamped to a 12px floor so a
    // browser reporting no inset still leaves real breathing room.
    expect(bar.className).toContain("pb-[max(0.75rem,var(--safe-area-bottom))]");
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
