// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// Selector — real DOM/keyboard behaviour (GlimStone form-engine Phase 2, Task
// 3). Covers what Selector.test.ts's pure nextFocusIndex/rovedIndex tests
// can't: actual focus movement, click wiring, roving tabindex reflected in
// the rendered DOM, and the `hue` opt-out's real class/style output. Named
// `.dom.test.tsx` per appearance.dom.test.tsx's own convention for the
// jsdom-opted-in exception (vitest.config.ts stays "node" by default).
//
// RTL note: production code reads direction via
// `getComputedStyle(strip.current).direction`, which in a real browser
// resolves through the UA stylesheet's `[dir="rtl"] { direction: rtl }` rule
// once `dir="rtl"` lands on `<html>` (lib/i18n.ts's isRtl). jsdom does not
// implement that UA-stylesheet cascade, so the RTL test below sets
// `direction: rtl` directly as an inline style on the strip — jsdom DOES
// resolve inline styles through getComputedStyle reliably, so this still
// exercises the real component code path (the same getComputedStyle read),
// just without depending on jsdom's incomplete CSS engine for the
// attribute→property mapping. The `dir`-attribute-driven path itself is only
// verified live, in a real browser, per the plan's own "verify RTL on the
// rendered page, not by reading the CSS" instruction (design-language.md,
// "Right-to-left languages").
// ---------------------------------------------------------------------------
import { useState } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { Selector, type SelectorItem } from "./Selector";
import { RAINBOW_OFF, applyRainbow } from "../lib/appearance";

const ITEMS: SelectorItem[] = [
  { id: "a", label: "Alpha" },
  { id: "b", label: "Beta" },
  { id: "c", label: "Gamma" },
];

beforeEach(() => {
  applyRainbow(RAINBOW_OFF);
});

afterEach(() => {
  cleanup();
});

// A controlled wrapper mirroring how every real call site owns `active`
// itself (Selector is stateless) — needed so keyboard navigation's onChange
// call is actually reflected back into props across a re-render, the same
// way Settings.tsx's tab === key comparison drives roving tabindex live.
function OneOfThree({ onChangeSpy, initial = "a" }: { onChangeSpy?: (id: string) => void; initial?: string }) {
  const [active, setActive] = useState(initial);
  return (
    <Selector
      items={ITEMS}
      label="Test strip"
      select="one"
      active={active}
      onChange={(id) => {
        setActive(id);
        onChangeSpy?.(id);
      }}
    />
  );
}

describe("Selector — click wiring", () => {
  it("calls onChange with the clicked item's id", () => {
    const spy = vi.fn();
    render(<OneOfThree onChangeSpy={spy} />);
    fireEvent.click(screen.getByRole("tab", { name: "Beta" }));
    expect(spy).toHaveBeenCalledWith("b");
  });

  it("never calls onChange for a disabled item's click", () => {
    const spy = vi.fn();
    render(
      <Selector
        items={[{ id: "a", label: "Alpha" }, { id: "b", label: "Beta", disabled: true }]}
        label="Test strip"
        active="a"
        onChange={spy}
      />
    );
    fireEvent.click(screen.getByRole("tab", { name: "Beta" }));
    expect(spy).not.toHaveBeenCalled();
  });
});

describe("Selector — roving tabindex", () => {
  it("only the active item is a tab stop (tabIndex 0); the rest are -1", () => {
    render(<OneOfThree initial="b" />);
    // No @testing-library/jest-dom in this repo (see package.json) — plain
    // DOM property access instead of the toHaveAttribute() matcher.
    expect((screen.getByRole("tab", { name: "Alpha" }) as HTMLElement).tabIndex).toBe(-1);
    expect((screen.getByRole("tab", { name: "Beta" }) as HTMLElement).tabIndex).toBe(0);
    expect((screen.getByRole("tab", { name: "Gamma" }) as HTMLElement).tabIndex).toBe(-1);
  });

  it("moving focus with the keyboard shifts the roving tab stop to the newly-selected item", () => {
    render(<OneOfThree />);
    const alpha = screen.getByRole("tab", { name: "Alpha" });
    alpha.focus();
    fireEvent.keyDown(screen.getByRole("tablist"), { key: "ArrowRight" });
    expect((screen.getByRole("tab", { name: "Beta" }) as HTMLElement).tabIndex).toBe(0);
    expect((screen.getByRole("tab", { name: "Alpha" }) as HTMLElement).tabIndex).toBe(-1);
  });
});

describe("Selector — keyboard navigation, LTR", () => {
  it("ArrowRight moves focus to the next item and selects it (select=\"one\" activates on move)", () => {
    const spy = vi.fn();
    render(<OneOfThree onChangeSpy={spy} />);
    screen.getByRole("tab", { name: "Alpha" }).focus();
    fireEvent.keyDown(screen.getByRole("tablist"), { key: "ArrowRight" });
    expect(document.activeElement).toBe(screen.getByRole("tab", { name: "Beta" }));
    expect(spy).toHaveBeenCalledWith("b");
  });

  it("ArrowLeft wraps from the first item to the last", () => {
    render(<OneOfThree />);
    screen.getByRole("tab", { name: "Alpha" }).focus();
    fireEvent.keyDown(screen.getByRole("tablist"), { key: "ArrowLeft" });
    expect(document.activeElement).toBe(screen.getByRole("tab", { name: "Gamma" }));
  });

  it("End jumps straight to the last item", () => {
    render(<OneOfThree />);
    screen.getByRole("tab", { name: "Alpha" }).focus();
    fireEvent.keyDown(screen.getByRole("tablist"), { key: "End" });
    expect(document.activeElement).toBe(screen.getByRole("tab", { name: "Gamma" }));
  });

  it("Home jumps straight back to the first item", () => {
    render(<OneOfThree initial="c" />);
    screen.getByRole("tab", { name: "Gamma" }).focus();
    fireEvent.keyDown(screen.getByRole("tablist"), { key: "Home" });
    expect(document.activeElement).toBe(screen.getByRole("tab", { name: "Alpha" }));
  });

  it("skips a disabled item when stepping past it", () => {
    const spy = vi.fn();
    render(
      <Selector
        items={[
          { id: "a", label: "Alpha" },
          { id: "b", label: "Beta", disabled: true },
          { id: "c", label: "Gamma" },
        ]}
        label="Test strip"
        active="a"
        onChange={spy}
      />
    );
    screen.getByRole("tab", { name: "Alpha" }).focus();
    fireEvent.keyDown(screen.getByRole("tablist"), { key: "ArrowRight" });
    expect(document.activeElement).toBe(screen.getByRole("tab", { name: "Gamma" }));
    expect(spy).toHaveBeenCalledWith("c");
  });
});

describe("Selector — keyboard navigation, RTL", () => {
  // See the file header for why direction is set inline rather than via a
  // dir="rtl" attribute in this jsdom test.
  function renderRtl(spy?: (id: string) => void) {
    const utils = render(<OneOfThree onChangeSpy={spy} />);
    const list = screen.getByRole("tablist");
    list.style.direction = "rtl";
    return utils;
  }

  it("ArrowRight moves BACKWARD (toward the previous item) under RTL", () => {
    renderRtl();
    screen.getByRole("tab", { name: "Beta" }).focus();
    fireEvent.keyDown(screen.getByRole("tablist"), { key: "ArrowRight" });
    expect(document.activeElement).toBe(screen.getByRole("tab", { name: "Alpha" }));
  });

  it("ArrowLeft moves FORWARD (toward the next item) under RTL", () => {
    renderRtl();
    screen.getByRole("tab", { name: "Beta" }).focus();
    fireEvent.keyDown(screen.getByRole("tablist"), { key: "ArrowLeft" });
    expect(document.activeElement).toBe(screen.getByRole("tab", { name: "Gamma" }));
  });

  it("Home/End are direction-independent — still the first/last DOM item", () => {
    renderRtl();
    screen.getByRole("tab", { name: "Beta" }).focus();
    fireEvent.keyDown(screen.getByRole("tablist"), { key: "End" });
    expect(document.activeElement).toBe(screen.getByRole("tab", { name: "Gamma" }));
  });
});

describe("Selector — select=\"many\"", () => {
  function ManySelector({ onChangeSpy }: { onChangeSpy: (id: string) => void }) {
    const [active, setActive] = useState<ReadonlySet<string>>(new Set(["a"]));
    return (
      <Selector
        items={ITEMS}
        label="Toggle strip"
        select="many"
        active={active}
        onChange={(id) => {
          const next = new Set(active);
          if (next.has(id)) next.delete(id);
          else next.add(id);
          setActive(next);
          onChangeSpy(id);
        }}
      />
    );
  }

  it("renders role=\"group\", not role=\"tablist\"", () => {
    render(<ManySelector onChangeSpy={vi.fn()} />);
    expect(screen.queryByRole("tablist")).toBeNull();
    expect(screen.getByRole("group", { name: "Toggle strip" })).toBeTruthy();
  });

  it("clicking toggles membership via onChange", () => {
    const spy = vi.fn();
    render(<ManySelector onChangeSpy={spy} />);
    fireEvent.click(screen.getByRole("button", { name: "Beta" }));
    expect(spy).toHaveBeenCalledWith("b");
  });

  it("arrow-key movement moves focus WITHOUT toggling — many-select never activates on move", () => {
    const spy = vi.fn();
    render(<ManySelector onChangeSpy={spy} />);
    screen.getByRole("button", { name: "Alpha" }).focus();
    fireEvent.keyDown(screen.getByRole("group"), { key: "ArrowRight" });
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "Beta" }));
    expect(spy).not.toHaveBeenCalled();
  });
});

describe("Selector — hue opt-out (Dashboard's heatmap toggle)", () => {
  it("hue=true (default) carries .glim-hue/.glim-hue-icon and an --item-hue inline style", () => {
    render(<OneOfThree />);
    const tab = screen.getByRole("tab", { name: "Alpha" });
    expect(tab.className).toContain("glim-hue");
    expect(tab.className).toContain("glim-hue-icon");
    expect(tab.style.getPropertyValue("--item-hue")).not.toBe("");
  });

  it("hue=false carries neither class, even while rainbow is globally on", () => {
    applyRainbow({ on: true });
    render(
      <Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} hue={false} />
    );
    const tab = screen.getByRole("tab", { name: "Alpha" });
    expect(tab.className).not.toContain("glim-hue");
    expect(tab.style.getPropertyValue("--item-hue")).toBe("");
  });
});

describe("Selector — variant=\"well\" (TrickWork-styled track, Settings.tsx's shape picker)", () => {
  it("default variant (\"chip\") renders none of the well track's wrapper/segment classes", () => {
    render(<OneOfThree />);
    expect(screen.getByRole("tablist").className).not.toContain("bg-carbon-surface2");
    const tab = screen.getByRole("tab", { name: "Alpha" });
    expect(tab.className).not.toContain("flex-1");
    expect(tab.className).not.toContain("--badge-md");
  });

  it("the strip itself becomes the padded well track: shared background, well gap/padding, no flex-wrap", () => {
    render(<Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} variant="well" />);
    const list = screen.getByRole("tablist");
    expect(list.className).toContain("bg-carbon-surface2");
    expect(list.className).toContain("rounded-control");
    expect(list.className).toContain("gap-[0.2rem]");
    expect(list.className).toContain("p-[0.2rem]");
    expect(list.className).not.toContain("flex-wrap");
  });

  it("idle segments are transparent and flush (no radius, no idle chip background); the active segment still fills with the accent", () => {
    render(<Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} variant="well" />);
    const active = screen.getByRole("tab", { name: "Alpha" });
    const idle = screen.getByRole("tab", { name: "Beta" });
    expect(active.className).toContain("bg-accent");
    expect(active.className).toContain("text-accentContrast");
    expect(idle.className).toContain("bg-transparent");
    expect(idle.className).not.toContain("bg-carbon-surface2");
    expect(idle.className).not.toContain("rounded-control");
  });

  it("segments are equal-width and centered, with the crossfade-only transition and --badge-md height from the exact TrickWork spec — no sliding pill element", () => {
    render(<Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} variant="well" />);
    const tab = screen.getByRole("tab", { name: "Alpha" });
    expect(tab.className).toContain("flex-1");
    expect(tab.className).toContain("justify-center");
    expect(tab.className).toContain("h-[var(--badge-md)]");
    expect(tab.className).toContain("[transition:background-color_120ms_ease]");
    // No pill/thumb: every "on" state lives on the segment's OWN background
    // colour, there is no extra sibling element carrying position/transform.
    expect(screen.getByRole("tablist").querySelectorAll("[data-sel-id]").length).toBe(ITEMS.length);
  });

  it("keyboard navigation is unregressed under variant=\"well\" — arrow keys still move focus and select (TrickWork's own version has no arrow-key support at all)", () => {
    const spy = vi.fn();
    function WellThree() {
      const [active, setActive] = useState("a");
      return (
        <Selector
          items={ITEMS}
          label="Test strip"
          select="one"
          active={active}
          onChange={(id) => {
            setActive(id);
            spy(id);
          }}
          variant="well"
        />
      );
    }
    render(<WellThree />);
    screen.getByRole("tab", { name: "Alpha" }).focus();
    fireEvent.keyDown(screen.getByRole("tablist"), { key: "ArrowRight" });
    expect(document.activeElement).toBe(screen.getByRole("tab", { name: "Beta" }));
    expect(spy).toHaveBeenCalledWith("b");
    expect((screen.getByRole("tab", { name: "Beta" }) as HTMLElement).tabIndex).toBe(0);
    expect((screen.getByRole("tab", { name: "Alpha" }) as HTMLElement).tabIndex).toBe(-1);
  });

  it("RTL still mirrors under variant=\"well\" — ArrowRight moves backward, same as \"chip\" (the direction read is variant-independent)", () => {
    render(<Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} variant="well" />);
    const list = screen.getByRole("tablist");
    list.style.direction = "rtl";
    screen.getByRole("tab", { name: "Beta" }).focus();
    fireEvent.keyDown(list, { key: "ArrowRight" });
    expect(document.activeElement).toBe(screen.getByRole("tab", { name: "Alpha" }));
  });
});
