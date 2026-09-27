// @vitest-environment jsdom
// Selector behaviour that needs a DOM: focus movement, clicks, the roving
// tabindex and the hue classes. The pure navigation math is tested in
// Selector.test.ts.
//
// The component reads direction with getComputedStyle. jsdom does not apply
// the UA rule that maps dir="rtl" to direction: rtl, so the RTL tests set
// direction inline on the strip; the dir attribute path is only checked in a
// browser.
import { useState } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { Selector, MAX_PINNED_WIDTH, MIN_PINNED_WIDTH, type SelectorItem } from "./Selector";
import { RAINBOW_OFF, applyRainbow } from "../lib/appearance";
import { setLabelMode } from "../lib/controls";

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

// Selector is stateless, so this wrapper owns active the way a real call site
// does.
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

describe("Selector click wiring", () => {
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

describe("Selector roving tabindex", () => {
  it("only the active item is a tab stop (tabIndex 0); the rest are -1", () => {
    render(<OneOfThree initial="b" />);
    // jest-dom is not installed, so read the property directly.
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

describe("Selector keyboard navigation, LTR", () => {
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

describe("Selector keyboard navigation, RTL", () => {
  // See the file header for why direction is set inline.
  function renderRtl(spy?: (id: string) => void) {
    const utils = render(<OneOfThree onChangeSpy={spy} />);
    const list = screen.getByRole("tablist");
    list.style.direction = "rtl";
    return utils;
  }

  it("ArrowRight moves backward to the previous item under RTL", () => {
    renderRtl();
    screen.getByRole("tab", { name: "Beta" }).focus();
    fireEvent.keyDown(screen.getByRole("tablist"), { key: "ArrowRight" });
    expect(document.activeElement).toBe(screen.getByRole("tab", { name: "Alpha" }));
  });

  it("ArrowLeft moves forward to the next item under RTL", () => {
    renderRtl();
    screen.getByRole("tab", { name: "Beta" }).focus();
    fireEvent.keyDown(screen.getByRole("tablist"), { key: "ArrowLeft" });
    expect(document.activeElement).toBe(screen.getByRole("tab", { name: "Gamma" }));
  });

  it("End still jumps to the last DOM item under RTL", () => {
    renderRtl();
    screen.getByRole("tab", { name: "Beta" }).focus();
    fireEvent.keyDown(screen.getByRole("tablist"), { key: "End" });
    expect(document.activeElement).toBe(screen.getByRole("tab", { name: "Gamma" }));
  });
});

describe("Selector select=\"many\"", () => {
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

  it("arrow keys move focus without toggling", () => {
    const spy = vi.fn();
    render(<ManySelector onChangeSpy={spy} />);
    screen.getByRole("button", { name: "Alpha" }).focus();
    fireEvent.keyDown(screen.getByRole("group"), { key: "ArrowRight" });
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "Beta" }));
    expect(spy).not.toHaveBeenCalled();
  });
});

describe("Selector hue opt-out", () => {
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

describe("Selector under a coarse pointer", () => {
  it("grows a text segment to the button height, which a glyph segment has already", () => {
    render(
      <Selector
        items={[ITEMS[0], { id: "g", label: "Glyph", icon: <svg /> }]}
        label="Test strip"
        active="a"
        onChange={() => {}}
      />
    );
    expect(screen.getByRole("tab", { name: "Alpha" }).className).toContain("pointer-coarse:min-h-(--btn-h)");
    const glyph = screen.getByRole("tab", { name: "Glyph" }).className;
    expect(glyph).toContain("glim-seg-btn");
    expect(glyph).not.toContain("pointer-coarse:min-h-(--btn-h)");
  });
});

// jsdom lays nothing out, so the room and each segment's widths are stubbed:
// the track reports `room` as its clientWidth, and a segment measured at
// max-content or min-content reports its entry in `widths`.
function layOut(room: number, widths: Record<string, { oneLine: number; narrowest: number }>) {
  const clientWidth = Object.getOwnPropertyDescriptor(Element.prototype, "clientWidth")!;
  Object.defineProperty(Element.prototype, "clientWidth", {
    configurable: true,
    get(this: Element) {
      return this.getAttribute("aria-orientation") === "horizontal" ? room : 0;
    },
  });
  const computed = window.getComputedStyle;
  const spy = vi.spyOn(window, "getComputedStyle").mockImplementation((el: Element) => {
    const style = computed(el);
    const id = el.getAttribute("data-sel-id") ?? el.querySelector("[data-sel-id]")?.getAttribute("data-sel-id");
    const w = id ? widths[id] : undefined;
    const set = (el as HTMLElement).style.width;
    const overrides: Record<string, string> = {
      paddingLeft: "0px",
      paddingRight: "0px",
      columnGap: "3.2px",
    };
    if (w && set === "max-content") overrides.width = `${w.oneLine}px`;
    if (w && set === "min-content") overrides.width = `${w.narrowest}px`;
    return new Proxy(style, {
      get: (target, key: string) => (key in overrides ? overrides[key] : Reflect.get(target, key)),
    });
  });
  return () => {
    Object.defineProperty(Element.prototype, "clientWidth", clientWidth);
    spy.mockRestore();
  };
}

const oneLine = (...ws: number[]) =>
  Object.fromEntries(ws.map((w, i) => [ITEMS[i]?.id ?? `x${i}`, { oneLine: w, narrowest: w }]));

const seg = (id: string) => document.querySelector(`[data-sel-id="${id}"]`) as HTMLElement;

const classes = (el: Element) => el.className.split(/\s+/);

// How many segments share a row, read back from the flex basis
// `calc((100% - n * gap) / n)`, which jsdom serialises as `calc(1/n*(...))`.
const perRow = (el: HTMLElement) => {
  const share = /^1 0 calc\(([\d.]+)\*/.exec(el.style.flex);
  return share ? Math.round(1 / parseFloat(share[1])) : null;
};

describe("Selector width", () => {
  it("spans its box at both scales and in both stylings", () => {
    for (const props of [{}, { equalWidth: true }, { variant: "chip" as const }]) {
      cleanup();
      render(<Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} {...props} />);
      const list = screen.getByRole("tablist");
      expect(classes(list)).toContain("w-full");
      expect(classes(list)).not.toContain("w-fit");
    }
  });

  it("hugs its segments with `inline`, for a toolbar row", () => {
    render(<Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} inline />);
    const list = screen.getByRole("tablist");
    expect(classes(list)).toContain("w-fit");
    expect(classes(list)).toContain("max-w-full");
    expect(classes(list)).not.toContain("w-full");
  });

  it("wraps rather than forcing one row, and never needs self-start", () => {
    render(<Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} equalWidth />);
    const list = screen.getByRole("tablist");
    expect(list.className).toContain("flex-wrap");
    expect(list.className).not.toContain("self-start");
  });

  it("lets an unpinned segment grow from its label into the row", () => {
    render(<OneOfThree />);
    expect(screen.getByRole("tab", { name: "Alpha" }).style.flex).toBe("1 0 auto");
  });
});

describe("Selector equalWidth", () => {
  it("pins every segment to the widest label and grows it into its share of the row", () => {
    const restore = layOut(900, oneLine(220, 180, 260));
    try {
      render(<Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} equalWidth />);
      for (const id of ["a", "b", "c"]) {
        expect(seg(id).style.minWidth).toBe("260px");
        expect(perRow(seg(id))).toBe(3);
      }
    } finally {
      restore();
    }
  });

  it("never pins narrower than MIN_PINNED_WIDTH while the row has room for it", () => {
    const restore = layOut(900, oneLine(60, 40, 80));
    try {
      render(<Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} equalWidth />);
      for (const id of ["a", "b", "c"]) expect(seg(id).style.minWidth).toBe(`${MIN_PINNED_WIDTH}px`);
    } finally {
      restore();
    }
  });

  it("lets the floor give way to an even share of a narrow row, keeping the options side by side", () => {
    const restore = layOut(320, oneLine(90, 97, 80));
    try {
      render(<Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} equalWidth />);
      expect(seg("a").style.minWidth).toBe(`${Math.floor((320 - 2 * 3.2) / 3)}px`);
      expect(perRow(seg("a"))).toBe(3);
    } finally {
      restore();
    }
  });

  it("splits the segments into even rows once they do not fit on one", () => {
    const six = ["a", "b", "c", "d", "e", "f"].map((id) => ({ id, label: id.toUpperCase() }));
    const widths = Object.fromEntries(six.map((i) => [i.id, { oneLine: 150, narrowest: 150 }]));
    const restore = layOut(700, widths);
    try {
      render(<Selector items={six} label="Test strip" active="a" onChange={() => {}} equalWidth />);
      // Four fit to a row, so six go three and three rather than four and two.
      expect(perRow(seg("a"))).toBe(3);
      expect(seg("a").style.minWidth).toBe("150px");
    } finally {
      restore();
    }
  });

  it("caps the pinned width, so one long label does not widen every segment", () => {
    const restore = layOut(700, oneLine(500, 90, 90));
    try {
      render(<Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} equalWidth />);
      expect(seg("b").style.minWidth).toBe(`${MAX_PINNED_WIDTH}px`);
      expect(screen.getByRole("tablist").style.flexWrap).toBe("");
    } finally {
      restore();
    }
  });

  it("lays the row out by content where a capped label would wrap and the options fit side by side", () => {
    const restore = layOut(700, { a: { oneLine: 420, narrowest: 180 }, b: { oneLine: 90, narrowest: 90 }, c: { oneLine: 90, narrowest: 90 } });
    try {
      render(<Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} equalWidth />);
      const list = screen.getByRole("tablist");
      expect(list.style.flexWrap).toBe("nowrap");
      expect(list.style.width).toBe("100%");
      for (const id of ["a", "b", "c"]) {
        expect(seg(id).style.flex).toBe("1 0 auto");
        expect(seg(id).style.minWidth).toBe("");
      }
    } finally {
      restore();
    }
  });

  it("lets a pinned label wrap, where an unpinned one truncates", () => {
    render(<Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} equalWidth />);
    expect(seg("a").querySelector("[data-sel-label]")!.className).not.toContain("truncate");
    cleanup();
    render(<OneOfThree />);
    expect(seg("a").querySelector("[data-sel-label]")!.className).toContain("truncate");
  });

  it("puts the row's flex on the span that wraps a disabled segment with a tip", () => {
    const restore = layOut(900, oneLine(220, 180, 260));
    try {
      render(
        <Selector
          items={[ITEMS[0], { ...ITEMS[1], disabled: true, title: "Pick a folder first" }, ITEMS[2]]}
          label="Test strip"
          active="a"
          onChange={() => {}}
          equalWidth
        />
      );
      const wrapper = seg("b").parentElement!;
      expect(wrapper.tagName).toBe("SPAN");
      expect(wrapper.style.minWidth).toBe("260px");
    } finally {
      restore();
    }
  });

  it("gives a \"well\" strip the fixed --badge-md height", () => {
    render(<Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} equalWidth />);
    const tab = screen.getByRole("tab", { name: "Alpha" });
    expect(tab.className).toContain("h-[var(--badge-md)]");
    expect(tab.className).toContain("justify-center");
  });

  it("keyboard navigation still works with equalWidth", () => {
    const spy = vi.fn();
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

// The well is the default styling, at two scales: with and without
// equalWidth. These tests keep the two scales from drifting apart.
describe("Selector well at both scales", () => {
  it("the chip styling renders none of the groove's classes", () => {
    render(<Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} variant="chip" />);
    expect(screen.getByRole("tablist").className).not.toContain("bg-carbon-surface3");
    const tab = screen.getByRole("tab", { name: "Alpha" });
    expect(tab.className).not.toContain("--badge-md");
    expect(screen.getByRole("tab", { name: "Beta" }).className).toContain("bg-carbon-surface2");
  });

  // The groove sits on a Card (surface) or a CadenceBuilder well (surface2),
  // and surface3 is the only token distinct from both.
  it("the strip is a groove at surface3, not surface2, which would vanish inside a surface2 well", () => {
    for (const props of [{}, { equalWidth: true }]) {
      cleanup();
      render(<Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} {...props} />);
      const list = screen.getByRole("tablist");
      expect(list.className).toContain("bg-carbon-surface3");
      expect(list.className).not.toContain("bg-carbon-surface2");
      expect(list.className).toContain("rounded-pill");
      // The standard groove ring; a thinner one does not read as an enclosure.
      expect(list.className).toContain("gap-[0.2rem]");
      expect(list.className).toContain("p-[0.2rem]");
    }
  });

  // An unselected option is not a badge, so idle segments have no fill.
  it("idle segments are transparent at both scales while the active segment fills with the accent", () => {
    for (const props of [{}, { equalWidth: true }]) {
      cleanup();
      render(<Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} {...props} />);
      const active = screen.getByRole("tab", { name: "Alpha" });
      const idle = screen.getByRole("tab", { name: "Beta" });
      expect(active.className).toContain("bg-accent");
      expect(active.className).toContain("text-accentContrast");
      expect(idle.className).toContain("bg-transparent");
      expect(idle.className).not.toContain("bg-carbon-surface ");
      expect(idle.className).not.toContain("bg-carbon-surface2");
      expect(idle.className).not.toContain("bg-carbon-surface3");
      // In round the groove is a pill with a pill inside it.
      expect(active.className).toContain("rounded-pill");
      expect(idle.className).toContain("rounded-pill");
      // A background crossfade, with no sliding thumb element.
      expect(active.className).toContain("[transition:background-color_120ms_ease]");
      expect(screen.getByRole("tablist").querySelectorAll("[data-sel-id]").length).toBe(ITEMS.length);
    }
  });

  it("the two scales differ only in pinning", () => {
    render(<Selector items={ITEMS} label="Small" active="a" onChange={() => {}} />);
    const smallList = screen.getByRole("tablist").className;
    const smallTab = (screen.getByRole("tab", { name: "Alpha" }) as HTMLElement).className;
    cleanup();
    render(<Selector items={ITEMS} label="Big" active="a" onChange={() => {}} equalWidth />);
    const bigList = screen.getByRole("tablist").className;
    const bigTab = (screen.getByRole("tab", { name: "Alpha" }) as HTMLElement).className;

    expect(bigList).toBe(smallList);

    // The segment differs by two sanctioned riders: the pinning classes the
    // big scale carries, and the touch height of the small one, which the big
    // scale's fixed height already clears. They are stripped before the
    // byte-identity compare, so the guard keeps catching any third divergence.
    const PIN = ["text-center", "h-[var(--badge-md)]"];
    const TOUCH = "pointer-coarse:min-h-(--btn-h)";
    const strip = (s: string) =>
      s
        .split(/\s+/)
        .filter((c) => !PIN.includes(c) && c !== TOUCH)
        .join(" ");
    expect(strip(bigTab)).toBe(strip(smallTab));
    for (const c of PIN) {
      expect(bigTab.split(/\s+/)).toContain(c);
      expect(smallTab.split(/\s+/)).not.toContain(c);
    }
    expect(smallTab.split(/\s+/)).toContain(TOUCH);
    expect(bigTab.split(/\s+/)).not.toContain(TOUCH);
  });

  it("without equalWidth a strip has no pinned width or fixed height", () => {
    render(<Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} />);
    const tab = screen.getByRole("tab", { name: "Alpha" });
    expect(tab.className).not.toContain("h-[var(--badge-md)]");
    expect(tab.style.minWidth).toBe("");
  });

  it("arrow keys still move focus and select at both scales", () => {
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

  it("ArrowRight still moves backward under RTL in the well", () => {
    render(<Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} />);
    const list = screen.getByRole("tablist");
    list.style.direction = "rtl";
    screen.getByRole("tab", { name: "Beta" }).focus();
    fireEvent.keyDown(list, { key: "ArrowRight" });
    expect(document.activeElement).toBe(screen.getByRole("tab", { name: "Alpha" }));
  });

  it("every segment carries its own rainbow position at both scales", () => {
    for (const props of [{}, { equalWidth: true }]) {
      cleanup();
      render(<Selector items={ITEMS} label="Test strip" active="a" onChange={() => {}} {...props} />);
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

describe("Selector iconOnly and tip", () => {
  const ICON_ITEMS: SelectorItem[] = [
    { id: "local", label: "Local", icon: <span data-testid="icon-local" />, iconOnly: true, tip: "Local path on this host" },
    { id: "remote", label: "Remote", icon: <span data-testid="icon-remote" />, iconOnly: true, tip: "Remote restic repository" },
  ];

  it("iconOnly hides the visible label text but keeps it as the accessible name via aria-label", () => {
    render(<Selector items={ICON_ITEMS} label="Path mode" active="local" onChange={() => {}} />);
    // Finding it by name checks that aria-label supplies the accessible name.
    const local = screen.getByRole("tab", { name: "Local" });
    expect(local.querySelector("span.truncate")).toBeNull();
    expect(local.getAttribute("aria-label")).toBe("Local");
    expect(local.querySelector("[data-testid='icon-local']")).not.toBeNull();
  });

  it("a non-iconOnly item keeps its visible label and gets no aria-label", () => {
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

  it("focusing an item with `tip` also reveals the tooltip", () => {
    render(<Selector items={ICON_ITEMS} label="Path mode" active="local" onChange={() => {}} />);
    const remote = screen.getByRole("tab", { name: "Remote" });
    fireEvent.keyDown(document.body, { key: "Tab" });
    act(() => remote.focus());
    const bubble = document.querySelector(".glim-bubble");
    expect(bubble?.textContent).toBe("Remote restic repository");
    act(() => remote.blur());
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

  it("arrow keys still move the roving tabindex and select with iconOnly and tip items", () => {
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

// When the label engine hides a segment's text, the segment names itself in
// the .glim-bubble tooltip, and title is never a native attribute.
describe("Selector glyph mode names its segments", () => {
  const PLAIN: SelectorItem[] = [
    { id: "a", label: "Alpha", icon: <span data-testid="icon-a" /> },
    { id: "b", label: "Beta", icon: <span data-testid="icon-b" /> },
  ];
  const WITH_TIPS: SelectorItem[] = [
    { id: "local", label: "Local", icon: <span />, tip: "Local path on this host" },
    { id: "remote", label: "Remote", icon: <span />, tip: "Remote restic repository" },
  ];

  afterEach(() => {
    // Both axes are persisted and would leak into later files.
    setLabelMode("tabs", "textGlyph");
    setLabelMode("buttons", "textGlyph");
    localStorage.clear();
  });

  it("falls back to the label when a hidden segment carries no tip of its own", () => {
    setLabelMode("buttons", "glyph");
    render(<Selector items={PLAIN} label="Test strip" active="a" onChange={() => {}} />);
    const alpha = screen.getByRole("tab", { name: "Alpha" });
    expect(alpha.querySelector("span.truncate")).toBeNull();
    fireEvent.mouseEnter(alpha);
    expect(document.querySelector(".glim-bubble")?.textContent).toBe("Alpha");
  });

  it("lets an explicit tip win outright rather than joining it to the label", () => {
    setLabelMode("buttons", "glyph");
    render(<Selector items={WITH_TIPS} label="Path mode" active="local" onChange={() => {}} />);
    fireEvent.mouseEnter(screen.getByRole("tab", { name: "Local" }));
    // These tips are full sentences that replace the label, not add to it.
    expect(document.querySelector(".glim-bubble")?.textContent).toBe("Local path on this host");
  });

  it("says nothing on hover while a segment still shows its own words", () => {
    render(<Selector items={PLAIN} label="Test strip" active="a" onChange={() => {}} />);
    fireEvent.mouseEnter(screen.getByRole("tab", { name: "Alpha" }));
    expect(document.querySelector(".glim-bubble")).toBeNull();
  });

  it("moves `title` out of the native attribute and into the bubble, reachable while disabled", () => {
    const items: SelectorItem[] = [
      { id: "a", label: "Alpha", disabled: true, title: "Pick a target folder first" },
      { id: "b", label: "Beta" },
    ];
    const { container } = render(
      <Selector items={items} label="Test strip" active="b" onChange={() => {}} />
    );
    const alpha = screen.getByRole("tab", { name: "Alpha" });
    expect(alpha.getAttribute("title")).toBeNull();
    // A disabled button emits no mouse events, so the hover lands on the
    // tooltip's wrapper instead.
    const wrapper = container.querySelector("span.inline-flex") as HTMLElement;
    expect(wrapper.contains(alpha)).toBe(true);
    fireEvent.mouseEnter(wrapper);
    expect(document.querySelector(".glim-bubble")?.textContent).toBe("Pick a target folder first");
  });

  // A strip follows the axis its size implies: lg is a page-level tab strip
  // and follows "tabs", any other size sits in a form row and follows
  // "buttons". Both directions are checked, since one alone would pass on a
  // component that reads the wrong axis everywhere.
  it("reads the buttons axis for a form-row strip, and only that one", () => {
    setLabelMode("tabs", "glyph");
    render(<Selector items={PLAIN} label="Test strip" active="a" onChange={() => {}} />);
    // Tabs is hiding labels and this strip does not care: its words stay.
    expect(screen.getByRole("tab", { name: "Alpha" }).querySelector("span.truncate")).not.toBeNull();
  });

  it("reads the tabs axis for a page-level strip, and only that one", () => {
    setLabelMode("buttons", "glyph");
    render(
      <Selector items={PLAIN} label="Test strip" size="lg" active="a" onChange={() => {}} />
    );
    // Buttons is hiding labels and the tab strip does not care.
    expect(screen.getByRole("tab", { name: "Alpha" }).querySelector("span.truncate")).not.toBeNull();
  });

  it("joins `title` to the name once the label is hidden, keeping both", () => {
    setLabelMode("buttons", "glyph");
    render(
      <Selector
        items={[{ id: "a", label: "Alpha", icon: <span />, title: "Busy right now" }]}
        label="Test strip"
        active="a"
        onChange={() => {}}
      />
    );
    fireEvent.mouseEnter(screen.getByRole("tab", { name: "Alpha" }));
    expect(document.querySelector(".glim-bubble")?.textContent).toBe("Alpha — Busy right now");
  });

  // The Settings tab strip passes title: label so a truncated label can still
  // be read in full; joining it to the name would repeat it.
  it("collapses a `title` that only repeats the name", () => {
    for (const mode of ["glyph", "textGlyph"] as const) {
      cleanup();
      setLabelMode("tabs", mode);
      render(
        <Selector
          items={[{ id: "general", label: "Allgemein", icon: <span />, title: "Allgemein" }]}
          label="Settings sections"
          active="general"
          onChange={() => {}}
        />
      );
      fireEvent.mouseEnter(screen.getByRole("tab", { name: "Allgemein" }));
      // In text mode the title stands alone.
      expect(document.querySelector(".glim-bubble")?.textContent).toBe("Allgemein");
    }
  });
});

// A segment's colour comes from its position, so selectors stacked with the
// same number of segments would repeat each column's colour down the page.
// hueOffset shifts where a selector starts in the palette.
describe("Selector hueOffset", () => {
  const styleOf = (name: string) =>
    screen.getByRole("tab", { name }).getAttribute("style") ?? "";

  it("gives the same position a different colour than an unshifted selector", () => {
    applyRainbow({ on: true });
    const { unmount } = render(
      <Selector items={ITEMS} label="First" active={ITEMS[0].id} onChange={() => {}} />,
    );
    const plain = ITEMS.map((it) => styleOf(it.label));
    unmount();

    render(
      <Selector items={ITEMS} label="Second" active={ITEMS[0].id} onChange={() => {}} hueOffset={1} />,
    );
    const shifted = ITEMS.map((it) => styleOf(it.label));

    expect(shifted).not.toEqual(plain);
    // Shifted by exactly one: position i now wears what position i+1 wore.
    for (let i = 0; i < ITEMS.length - 1; i++) {
      expect(shifted[i]).toBe(plain[i + 1]);
    }
  });

  it("defaults to no shift, so existing call sites are untouched", () => {
    applyRainbow({ on: true });
    const { unmount } = render(
      <Selector items={ITEMS} label="Implicit" active={ITEMS[0].id} onChange={() => {}} />,
    );
    const implicit = ITEMS.map((it) => styleOf(it.label));
    unmount();

    render(
      <Selector items={ITEMS} label="Explicit" active={ITEMS[0].id} onChange={() => {}} hueOffset={0} />,
    );
    expect(ITEMS.map((it) => styleOf(it.label))).toEqual(implicit);
  });

  it("is ignored when the selector is not hued at all", () => {
    applyRainbow({ on: true });
    render(
      <Selector items={ITEMS} label="Flat" active={ITEMS[0].id} onChange={() => {}} hue={false} hueOffset={3} />,
    );
    for (const it of ITEMS) {
      expect(styleOf(it.label)).not.toContain("--item-hue");
    }
  });
});
