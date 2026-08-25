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
import { Selector, MIN_PINNED_WIDTH, type SelectorItem } from "./Selector";
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

describe("Selector — equalWidth (Settings.tsx's tab strip)", () => {
  it("default (equalWidth unset) keeps content-hugging chips, flex-wrap, no flex-1", () => {
    render(<OneOfThree />);
    expect(screen.getByRole("tablist").className).toContain("flex-wrap");
    const tab = screen.getByRole("tab", { name: "Alpha" });
    expect(tab.className).not.toContain("flex-1");
  });

  it("equalWidth keeps the strip flex-wrap (fixed-width segments can genuinely overflow a narrow row, unlike well's own measured-and-pinned segments) and every segment becomes flex-none (content-width matched, not stretched), while keeping the chip's own idle bg-carbon-surface2 fill (not well's transparent/shared-track look)", () => {
    render(
      <Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} equalWidth />
    );
    const list = screen.getByRole("tablist");
    // flex-wrap, NOT flex-nowrap (caught live — a fixed-pixel-width row can
    // overflow a real viewport where "well"'s own row (always exactly the sum
    // of its segments' pinned widths) never could; see Selector.tsx's own
    // file header, item 5b, for the live overflow bug this corrects).
    expect(list.className).toContain("flex-wrap");
    expect(list.className).not.toContain("flex-nowrap");
    // No shared well track — each segment still carries its own chip fill.
    expect(list.className).not.toContain("bg-carbon-surface2");

    const idle = screen.getByRole("tab", { name: "Beta" });
    // flex-none, NOT flex-1 (jdp's correction: pinned to the widest segment's
    // own measured content width via inline style, not stretched to fill the
    // row via a flex share — see Selector.tsx's own file header, item 5b).
    expect(idle.className).toContain("flex-none");
    expect(idle.className).not.toContain("flex-1");
    expect(idle.className).toContain("justify-center");
    expect(idle.className).toContain("bg-carbon-surface2");
    expect(idle.className).not.toContain("h-[var(--badge-md)]");

    const active = screen.getByRole("tab", { name: "Alpha" });
    expect(active.className).toContain("bg-accent");
    expect(active.className).toContain("text-accentContrast");
  });

  it("equalWidth measures each segment's natural content width and pins EVERY segment to the WIDEST one's width, not a full-row stretch", () => {
    // jsdom's getBoundingClientRect() is always a zero rect by default (see
    // ColorPickerPopover.dom.test.tsx's own identical stub) — stub it here,
    // keyed by data-sel-id, so the three segments report distinct, realistic
    // natural widths for the component's own two-pass measurement effect to
    // actually exercise, rather than every segment trivially "matching" 0.
    // Widths are all comfortably ABOVE MIN_PINNED_WIDTH (round 3's own
    // standardized floor, see Selector.tsx's own doc near that constant) so
    // this test still isolates the pre-existing "widest segment wins" claim
    // — the floor's own behaviour (every segment BELOW it) gets its own test
    // right below this one.
    const restore = HTMLElement.prototype.getBoundingClientRect;
    const widths: Record<string, number> = { a: 220, b: 180, c: 260 };
    HTMLElement.prototype.getBoundingClientRect = function (this: HTMLElement) {
      const id = this.getAttribute("data-sel-id");
      const w = id ? (widths[id] ?? 0) : 0;
      return { x: 0, y: 0, left: 0, top: 0, right: w, bottom: 0, width: w, height: 0, toJSON() {} } as DOMRect;
    };
    try {
      render(
        <Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} equalWidth />
      );
      // "Gamma" (id "c") is the widest at 260px — every segment, including the
      // narrower "Alpha" (220px) and "Beta" (180px), must end up pinned to
      // that SAME 260px, not their own natural width and not a
      // container-filling stretch (there is no container width involved in
      // this test at all).
      for (const id of ["a", "b", "c"]) {
        const btn = document.querySelector(`[data-sel-id="${id}"]`) as HTMLElement;
        expect(btn.style.width).toBe("260px");
      }
    } finally {
      HTMLElement.prototype.getBoundingClientRect = restore;
    }
  });

  it("equalWidth never pins narrower than MIN_PINNED_WIDTH, even when every segment's own natural content is smaller (round 3's own standardized floor — jdp: \"Die horizontalen Selektoren bitte breiter und möglichst gleich breit\")", () => {
    const restore = HTMLElement.prototype.getBoundingClientRect;
    // All three well below the floor — the widest of these (80px) alone would
    // have been last round's own answer; the floor must win instead.
    const widths: Record<string, number> = { a: 60, b: 40, c: 80 };
    HTMLElement.prototype.getBoundingClientRect = function (this: HTMLElement) {
      const id = this.getAttribute("data-sel-id");
      const w = id ? (widths[id] ?? 0) : 0;
      return { x: 0, y: 0, left: 0, top: 0, right: w, bottom: 0, width: w, height: 0, toJSON() {} } as DOMRect;
    };
    try {
      render(
        <Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} equalWidth />
      );
      for (const id of ["a", "b", "c"]) {
        const btn = document.querySelector(`[data-sel-id="${id}"]`) as HTMLElement;
        expect(btn.style.width).toBe(`${MIN_PINNED_WIDTH}px`);
      }
    } finally {
      HTMLElement.prototype.getBoundingClientRect = restore;
    }
  });

  it("equalWidth is what turns a \"well\" strip into the BIG scale — pinned width AND the fixed --badge-md height (round 8: it is no longer ignored there)", () => {
    render(
      <Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} equalWidth variant="well" />
    );
    const tab = screen.getByRole("tab", { name: "Alpha" });
    // One flex-none + the fixed well height, not a second/duplicate class or
    // a `flex-1` share (jdp's live-review correction, task 1 follow-up: a
    // pinned strip pins to a MEASURED width via `pinWidth`, never flex-grow —
    // see Selector.tsx's own `pinWidth` doc).
    expect(tab.className).toContain("flex-none");
    expect(tab.className).not.toContain("flex-1");
    expect(tab.className).toContain("h-[var(--badge-md)]");
    expect(tab.className).toContain("justify-center");
  });

  it("keyboard navigation is unregressed under equalWidth", () => {
    const spy = vi.fn();
    render(<OneOfThree onChangeSpy={spy} />);
    // Re-render with equalWidth via a fresh controlled instance.
    cleanup();
    function EqualWidthThree() {
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
          equalWidth
        />
      );
    }
    render(<EqualWidthThree />);
    screen.getByRole("tab", { name: "Alpha" }).focus();
    fireEvent.keyDown(screen.getByRole("tablist"), { key: "ArrowRight" });
    expect(document.activeElement).toBe(screen.getByRole("tab", { name: "Beta" }));
    expect(spy).toHaveBeenCalledWith("b");
  });
});

// ---------------------------------------------------------------------------
// variant="well" — the app's ONE grooved horizontal selector, at both scales.
//
// Round 7 had shipped a SECOND grooved variant ("track") for the compact,
// repeated-per-card selectors, whose one deliberate difference was that its
// idle segments each carried their own visible fill. jdp reversed exactly
// that in round 8 ("Die kleinen Selektoren sollen so aussehen wie die
// grossen! Die nicht ausgewaehlten Optionen sollen kein Badge sein"), which
// left the two variants differing only in the pinned-width/height bundle —
// i.e. in the `equalWidth` prop that already existed. So "track" is gone and
// `equalWidth` is now the pinning knob for both variants; these tests cover
// the two scales of the one variant, and specifically the invariants that
// stop them drifting apart again.
// ---------------------------------------------------------------------------
describe("Selector — variant=\"well\", shared by both scales", () => {
  it("default variant (\"chip\") renders none of the groove's wrapper/segment classes", () => {
    render(<OneOfThree />);
    expect(screen.getByRole("tablist").className).not.toContain("bg-carbon-surface3");
    expect(screen.getByRole("tablist").className).not.toContain("w-fit");
    const tab = screen.getByRole("tab", { name: "Alpha" });
    expect(tab.className).not.toContain("flex-1");
    expect(tab.className).not.toContain("--badge-md");
    expect(tab.className).not.toContain("bg-transparent");
  });

  // The groove has to differ from BOTH parent surfaces this variant legally
  // sits on — a Card (`bg-carbon-surface`) and a CadenceBuilder well
  // (`bg-carbon-surface2`). Round 7's first cut painted it `bg-carbon-surface2`,
  // which measured literally identical to the well behind it at six of its
  // nine live call sites, so the one thing the variant exists to add could
  // not be seen at all. `bg-carbon-surface3` is the only surface token
  // distinct from both, and round 8 unified BOTH scales onto it.
  it("the strip itself becomes an enclosing groove at surface3 — never surface2, which is invisible inside a surface2 well", () => {
    for (const props of [{}, { equalWidth: true }]) {
      cleanup();
      render(<Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} variant="well" {...props} />);
      const list = screen.getByRole("tablist");
      expect(list.className).toContain("bg-carbon-surface3");
      expect(list.className).not.toContain("bg-carbon-surface2");
      expect(list.className).toContain("rounded-control");
      // The established groove ring for this role, not a re-derived
      // near-miss (round 7's own 0.15rem left a 2.4px ring, too thin to read
      // as an enclosure). Scale separation comes from the segments.
      expect(list.className).toContain("gap-[0.2rem]");
      expect(list.className).toContain("p-[0.2rem]");
    }
  });

  it("the groove hugs its own segments (w-fit max-w-full) at BOTH scales, and never forces one row", () => {
    for (const props of [{}, { equalWidth: true }]) {
      cleanup();
      render(<Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} variant="well" {...props} />);
      const list = screen.getByRole("tablist");
      expect(list.className).toContain("w-fit");
      expect(list.className).toContain("max-w-full");
      // `flex-wrap`, never `flex-nowrap`: a pinned strip's width is the sum
      // of N x max(widest label, MIN_PINNED_WIDTH), which has no guaranteed
      // relationship to the available width — the same overflow "chip"'s own
      // equalWidth already wraps for. Wrapping beats spilling off the page.
      expect(list.className).toContain("flex-wrap");
      expect(list.className).not.toContain("flex-nowrap");
      // No `self-start`: `width: fit-content` already opts the strip out of a
      // flex column's default `align-items: stretch`, and `self-start` would
      // have top-aligned it inside the row-shaped parents (NotifyCard's "on"
      // row, the weekday row, the drill-kind row) that centre a label beside
      // it. This is what let Settings.tsx drop three wrapper divs.
      expect(list.className).not.toContain("self-start");
    }
  });

  // THE round-8 fix, and the invariant that keeps the two scales one control:
  // an unselected option is not a badge. Round 7's small variant filled every
  // idle segment at `bg-carbon-surface`; that is what this asserts is gone.
  it("idle segments are transparent at BOTH scales — no per-segment badge fill — while the active segment still fills with the accent", () => {
    for (const props of [{}, { equalWidth: true }]) {
      cleanup();
      render(<Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} variant="well" {...props} />);
      const active = screen.getByRole("tab", { name: "Alpha" });
      const idle = screen.getByRole("tab", { name: "Beta" });
      expect(active.className).toContain("bg-accent");
      expect(active.className).toContain("text-accentContrast");
      expect(idle.className).toContain("bg-transparent");
      // None of the three idle fills any other treatment could hand it: the
      // round-7 key fill, the chip default, or `raised`.
      expect(idle.className).not.toContain("bg-carbon-surface ");
      expect(idle.className).not.toContain("bg-carbon-surface2");
      expect(idle.className).not.toContain("bg-carbon-surface3");
      // CORRECTED (jdp, live-review — a well segment must reshape with
      // round/soft/square too, the same as the groove itself and every "chip"
      // segment already do; see Selector.tsx's own comment on this line for
      // the live getComputedStyle proof of the regression this fixes).
      expect(active.className).toContain("rounded-control");
      expect(idle.className).toContain("rounded-control");
      // TrickWork's own crossfade-only transition, at both scales — no
      // sliding pill/thumb element, and no second element per segment.
      expect(active.className).toContain("[transition:background-color_120ms_ease]");
      expect(screen.getByRole("tablist").querySelectorAll("[data-sel-id]").length).toBe(ITEMS.length);
    }
  });

  it("the two scales differ in EXACTLY one thing: whether segments are pinned. Everything else is byte-identical", () => {
    render(<Selector items={ITEMS} label="Small" active="a" onChange={() => {}} variant="well" />);
    const smallList = screen.getByRole("tablist").className;
    const smallTab = (screen.getByRole("tab", { name: "Alpha" }) as HTMLElement).className;
    cleanup();
    render(<Selector items={ITEMS} label="Big" active="a" onChange={() => {}} variant="well" equalWidth />);
    const bigList = screen.getByRole("tablist").className;
    const bigTab = (screen.getByRole("tab", { name: "Alpha" }) as HTMLElement).className;

    // The groove itself is literally the same string at both scales — this is
    // the structural guarantee that replaced "two variants that looked alike."
    expect(bigList).toBe(smallList);

    // The segment differs only by the pinning classes.
    const PIN = ["flex-none", "justify-center", "text-center", "h-[var(--badge-md)]"];
    const strip = (s: string) => s.split(/\s+/).filter((c) => !PIN.includes(c)).join(" ");
    expect(strip(bigTab)).toBe(strip(smallTab));
    for (const c of PIN) {
      expect(bigTab.split(/\s+/)).toContain(c);
      expect(smallTab.split(/\s+/)).not.toContain(c);
    }
  });

  it("without equalWidth a \"well\" strip is content-hugging — no pinned width, no fixed height — which is what makes it cheap to repeat on every schedule card", () => {
    render(<Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} variant="well" />);
    const tab = screen.getByRole("tab", { name: "Alpha" });
    expect(tab.className).not.toContain("flex-none");
    expect(tab.className).not.toContain("h-[var(--badge-md)]");
    expect(tab.style.width).toBe("");
  });

  it("`plain` and `raised` are both ignored under variant=\"well\" — an idle segment stays transparent either way", () => {
    render(
      <Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} variant="well" plain raised />
    );
    const idle = screen.getByRole("tab", { name: "Beta" });
    expect(idle.className).toContain("bg-transparent");
    expect(idle.className).not.toContain("bg-carbon-surface3");
  });

  it("keyboard navigation is unregressed under variant=\"well\" at both scales — arrow keys still move focus and select (TrickWork's own version has no arrow-key support at all)", () => {
    for (const props of [{}, { equalWidth: true }]) {
      cleanup();
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
            {...props}
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
    }
  });

  it("RTL still mirrors under variant=\"well\" — ArrowRight moves backward, same as \"chip\" (the direction read is variant-independent)", () => {
    render(<Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} variant="well" />);
    const list = screen.getByRole("tablist");
    list.style.direction = "rtl";
    screen.getByRole("tab", { name: "Beta" }).focus();
    fireEvent.keyDown(list, { key: "ArrowRight" });
    expect(document.activeElement).toBe(screen.getByRole("tab", { name: "Alpha" }));
  });

  it("every segment still carries its own rainbow position at both scales — the active fill reads that position, so switching scale can never shift a hue", () => {
    for (const props of [{}, { equalWidth: true }]) {
      cleanup();
      render(<Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} variant="well" {...props} />);
      const tabs = Array.from(
        screen.getByRole("tablist").querySelectorAll<HTMLElement>("[data-sel-id]")
      );
      expect(tabs.length).toBe(ITEMS.length);
      tabs.forEach((tab) => {
        expect(tab.className).toContain("glim-hue");
        // hueVars() lands the position colour on the element as a custom
        // property; without it the accent fill has nothing to resolve.
        expect(tab.getAttribute("style") ?? "").toContain("--");
      });
      expect(tabs[0].className).toContain("glim-active");
    }
  });
});
describe("Selector — iconOnly/tip (PathModeSwitch's Local/Remote pair, GlimStone follow-up round)", () => {
  const ICON_ITEMS: SelectorItem[] = [
    { id: "local", label: "Local", icon: <span data-testid="icon-local" />, iconOnly: true, tip: "Local path on this host" },
    { id: "remote", label: "Remote", icon: <span data-testid="icon-remote" />, iconOnly: true, tip: "Remote restic repository" },
  ];

  it("iconOnly hides the visible label text but keeps it as the accessible name via aria-label", () => {
    render(<Selector items={ICON_ITEMS} label="Path mode" active="local" onChange={() => {}} />);
    // getByRole({name}) matches the computed accessible name, which for an
    // iconOnly segment now comes from aria-label, not visible text content —
    // finding it this way IS the assertion that the accessible name survived.
    const local = screen.getByRole("tab", { name: "Local" });
    expect(local.querySelector("span.truncate")).toBeNull();
    expect(local.getAttribute("aria-label")).toBe("Local");
    expect(local.querySelector("[data-testid='icon-local']")).not.toBeNull();
  });

  it("a non-iconOnly item is unaffected — visible label still renders, no aria-label added", () => {
    render(<OneOfThree />);
    const alpha = screen.getByRole("tab", { name: "Alpha" });
    expect(alpha.querySelector("span.truncate")).not.toBeNull();
    expect(alpha.getAttribute("aria-label")).toBeNull();
  });

  it("hovering an item with `tip` reveals a portal-rendered .glim-bubble tooltip with that text", () => {
    render(<Selector items={ICON_ITEMS} label="Path mode" active="local" onChange={() => {}} />);
    expect(document.querySelector(".glim-bubble")).toBeNull();
    fireEvent.mouseEnter(screen.getByRole("tab", { name: "Local" }));
    const bubble = document.querySelector(".glim-bubble");
    expect(bubble).not.toBeNull();
    expect(bubble?.textContent).toBe("Local path on this host");
  });

  it("moving the mouse off the item hides the tooltip again", () => {
    render(<Selector items={ICON_ITEMS} label="Path mode" active="local" onChange={() => {}} />);
    const tab = screen.getByRole("tab", { name: "Local" });
    fireEvent.mouseEnter(tab);
    expect(document.querySelector(".glim-bubble")).not.toBeNull();
    fireEvent.mouseLeave(tab);
    expect(document.querySelector(".glim-bubble")).toBeNull();
  });

  it("focusing an item with `tip` also reveals the tooltip (keyboard-accessible, same as InfoBubble)", () => {
    render(<Selector items={ICON_ITEMS} label="Path mode" active="local" onChange={() => {}} />);
    const remote = screen.getByRole("tab", { name: "Remote" });
    fireEvent.focus(remote);
    const bubble = document.querySelector(".glim-bubble");
    expect(bubble?.textContent).toBe("Remote restic repository");
    fireEvent.blur(remote);
    expect(document.querySelector(".glim-bubble")).toBeNull();
  });

  it("an item with no `tip` never renders a tooltip on hover, and stays a plain click/keyboard target", () => {
    const spy = vi.fn();
    render(<OneOfThree onChangeSpy={spy} />);
    const alpha = screen.getByRole("tab", { name: "Alpha" });
    fireEvent.mouseEnter(alpha);
    expect(document.querySelector(".glim-bubble")).toBeNull();
    fireEvent.click(screen.getByRole("tab", { name: "Beta" }));
    expect(spy).toHaveBeenCalledWith("b");
  });

  it("keyboard arrow navigation is unregressed for iconOnly/tip items — roving tabindex and selection still work", () => {
    const spy = vi.fn();
    function IconTwo() {
      const [active, setActive] = useState("local");
      return (
        <Selector
          items={ICON_ITEMS}
          label="Path mode"
          select="one"
          active={active}
          onChange={(id) => {
            setActive(id);
            spy(id);
          }}
        />
      );
    }
    render(<IconTwo />);
    screen.getByRole("tab", { name: "Local" }).focus();
    fireEvent.keyDown(screen.getByRole("tablist"), { key: "ArrowRight" });
    expect(document.activeElement).toBe(screen.getByRole("tab", { name: "Remote" }));
    expect(spy).toHaveBeenCalledWith("remote");
    expect((screen.getByRole("tab", { name: "Remote" }) as HTMLElement).tabIndex).toBe(0);
    expect((screen.getByRole("tab", { name: "Local" }) as HTMLElement).tabIndex).toBe(-1);
  });
});
