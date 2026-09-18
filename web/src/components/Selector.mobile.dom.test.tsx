// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// Selector — the MOBILE half of the pinning decision (below Tailwind's md
// breakpoint, 48rem).
//
// The desktop pinning suite next door (Selector.dom.test.tsx, "equalWidth")
// runs against this repo's default jsdom answer: DESKTOP — the guarded setup
// stub (src/lib/testSetup/matchMedia.ts) answers every min-width query true,
// so `pinWidth` engages there and its assertions pin the 260px/MIN_PINNED_
// WIDTH inline widths. This file is the other side of that coin: it stubs
// window.matchMedia WHOLESALE (vi.stubGlobal in beforeAll, restored in
// afterAll — a per-file stub keeps winning over the setup stub, which installs
// only when matchMedia is absent) so useIsDesktop answers FALSE, i.e. a phone.
//
// What it locks: below 48rem the equalWidth pinning measurement (items 5b/5c
// in Selector.tsx's header) is SUPPRESSED. Segments keep the pinned branch's
// own classes (the fixed well height, centring) but lose the inline width, so
// the strip renders the content-hugging presentation that already wraps per
// the file's "wraps, never scrolls" rule. The measured finding behind the
// gate: a ~303px phone column fits ONE 200px
// (MIN_PINNED_WIDTH) segment per row, so the Settings General tab stacked 12+
// pinned 200x43 blocks into ~2400px of page scroll at 390px.
//
// Scope note: jsdom applies no media queries to CSS, so the max-md: half of
// the mobile fixes is asserted nowhere in any suite; this file locks the JS
// half — the pinWidth suppression — which IS testable, keyed on the ONE
// breakpoint authority, the imported DESKTOP_QUERY constant (never a second
// literal: useMediaQuery.ts is the breakpoint's only home).
// ---------------------------------------------------------------------------
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { Selector, type SelectorItem } from "./Selector";
import { DESKTOP_QUERY } from "../lib/useMediaQuery";
import { RAINBOW_OFF, applyRainbow } from "../lib/appearance";

const ITEMS: SelectorItem[] = [
  { id: "a", label: "Alpha" },
  { id: "b", label: "Beta" },
  { id: "c", label: "Gamma" },
];

// Phone-environment matchMedia: matches:false for EVERY query — the width one
// included (DESKTOP_QUERY is what useIsDesktop actually passes; naming the
// constant here keeps this file keyed to the shared authority instead of a
// private string). No-op listeners on both APIs and onchange:null, the same
// shape the guarded setup stub uses, minus its desktop default.
const mobileMatchMedia = vi.fn(
  (query: string): MediaQueryList =>
    ({
      media: query,
      matches: false,
      onchange: null,
      addEventListener: () => {},
      removeEventListener: () => {},
      addListener: () => {},
      removeListener: () => {},
      dispatchEvent: () => false,
    }) as unknown as MediaQueryList,
);

beforeAll(() => {
  vi.stubGlobal("matchMedia", mobileMatchMedia);
});

afterAll(() => {
  vi.unstubAllGlobals();
});

beforeEach(() => {
  applyRainbow(RAINBOW_OFF);
});

afterEach(() => {
  cleanup();
});

// jsdom's getBoundingClientRect() is a zero rect by default — the same keyed
// stub Selector.dom.test.tsx's own pinning tests use (natural widths
// 220/180/260), so the measurement pipeline would have REAL numbers to pin
// with if it were ever allowed to run down here.
function stubRectWidths(widths: Record<string, number>): () => void {
  const restore = HTMLElement.prototype.getBoundingClientRect;
  HTMLElement.prototype.getBoundingClientRect = function (this: HTMLElement) {
    const id = this.getAttribute("data-sel-id");
    const w = id ? (widths[id] ?? 0) : 0;
    return { x: 0, y: 0, left: 0, top: 0, right: w, bottom: 0, width: w, height: 0, toJSON() {} } as DOMRect;
  };
  return () => {
    HTMLElement.prototype.getBoundingClientRect = restore;
  };
}

describe("Selector — pinning suppressed below 48rem (mobile)", () => {
  it("consults the shared width axis through DESKTOP_QUERY before answering", () => {
    render(<Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} equalWidth />);
    // The gate must ride the ONE breakpoint hook: the recorded matchMedia
    // calls carry the shared constant. A re-literalised width check anywhere
    // else (window.innerWidth < N, a private query string) would empty this
    // log and fail here.
    const queries = mobileMatchMedia.mock.calls.map(([q]) => q);
    expect(queries).toContain(DESKTOP_QUERY);
  });

  it("equalWidth chip strip does not pin below 48rem — no inline width, not even the 200px floor", () => {
    const restore = stubRectWidths({ a: 220, b: 180, c: 260 });
    try {
      render(<Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} equalWidth />);
      // Strictly EMPTY: no 260px widest-segment pin, no MIN_PINNED_WIDTH
      // (200px) floor, nothing. Below md the segments hug their own labels
      // inside the strip's flex-wrap.
      for (const id of ["a", "b", "c"]) {
        const btn = document.querySelector(`[data-sel-id="${id}"]`) as HTMLElement;
        expect(btn.style.width).toBe("");
      }
    } finally {
      restore();
    }
  });

  it("equalWidth well strip does not pin below 48rem — the measurement pass stays suppressed across a re-render", () => {
    const restore = stubRectWidths({ a: 220, b: 180, c: 260 });
    try {
      const jsx = (
        <Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} equalWidth variant="well" />
      );
      const { rerender } = render(jsx);
      for (const id of ["a", "b", "c"]) {
        const btn = document.querySelector(`[data-sel-id="${id}"]`) as HTMLElement;
        expect(btn.style.width).toBe("");
      }
      // Same items again: pass 1 must never have armed a stale match and
      // pass 2 must never have measured, so the re-render re-asserts the
      // suppression held (matchedWidth never set).
      rerender(jsx);
      for (const id of ["a", "b", "c"]) {
        const btn = document.querySelector(`[data-sel-id="${id}"]`) as HTMLElement;
        expect(btn.style.width).toBe("");
      }
    } finally {
      restore();
    }
  });

  it("content-hugging fallback keeps the equalWidth scale classes — only the inline width disappears", () => {
    render(
      <Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} equalWidth variant="well" />,
    );
    // The segment class branches key on the equalWidth PROP, not pinWidth, so
    // below md a segment is still the big selector's standardized BOX (fixed
    // well height, centred, flex-none) — it just hugs its own label's width
    // instead of wearing a measured one.
    const tab = screen.getByRole("tab", { name: "Alpha" });
    expect(tab.className).toContain("flex-none");
    expect(tab.className).toContain("justify-center");
    expect(tab.className).toContain("h-[var(--badge-md)]");
    expect(tab.style.width).toBe("");
  });
});
