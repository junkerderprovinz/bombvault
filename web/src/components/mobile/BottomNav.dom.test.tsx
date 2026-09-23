// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// BottomNav's dom contract; the bar-card and the More trigger.
//
// The geometry half of the bar-card (insets, radius, safe areas) is CSS and
// jsdom computes no layout, so the observable form here is the class token:
// the host paints nothing (transparent, no border anywhere on the bar) and
// the card carries the sidebar surface token and the card radius; the same
// targeted-token discipline BottomSheet.dom.test.tsx states for its own
// styling contracts. What is behavioural here:
//   - the More trigger is a disclosure: aria-haspopup="dialog" and
//     aria-expanded that tracks the sheet's open state;
//   - the More trigger reads as active (filled accent) exactly while the
//     current route lives on the More side of the registry (Settings, the
//     gated tabs) and never while a bar destination is current;
//   - every slot carries the colour engine (glim-hue + its own --item-hue)
//     with the active/filled slot additionally carrying glim-active;
//   - in glyph mode the slot's name lives in aria-label and opens the app's
//     tip bubble (lib/useTipBubble), the sidebar's mechanism; the native
//     title attribute is retired.
// ---------------------------------------------------------------------------
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { BottomNav } from "./BottomNav";
import { I18nProvider } from "../../lib/i18n";
import { hueVars } from "../../lib/appearance";
import type { Settings } from "../../lib/api";

function draw(path: string, settings: Settings | null = null) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <I18nProvider>
        <BottomNav settings={settings} authEnabled={false} scrollMainToTop={() => undefined} />
      </I18nProvider>
    </MemoryRouter>,
  );
}

function bar() {
  return screen.getByTestId("bottom-nav");
}

afterEach(cleanup);

describe("BottomNav bar-card", () => {
  it("the host paints nothing and carries no line: transparent ground, zero border tokens", () => {
    // Separation by shade, never by a line; any border/ring class on the
    // host would reintroduce the strip-with-a-line-on-top the bar-card
    // language replaced.
    draw("/dashboard");
    const host = bar();
    expect(host.className).toContain("bg-transparent");
    expect(host.className).not.toMatch(/border-\S+/);
    expect(host.className).not.toMatch(/ring-\S+/);
    // The bottom safe area belongs to the host (the card floats above it).
    expect(host.className).toContain("pb-[var(--safe-area-bottom)]");
  });

  it("the card carries the sidebar surface token and the card radius, still with no line", () => {
    draw("/dashboard");
    const card = bar().querySelector("div.rounded-card");
    expect(card).not.toBeNull();
    expect(card!.className).toContain("bg-carbon-sidebar");
    expect(card!.className).not.toMatch(/border-\S+/);
  });

  it("fresh-DB fixture renders the three enabled bar destinations plus the More trigger", () => {
    draw("/dashboard");
    const slots = bar().querySelectorAll("div.flex.h-14 > *");
    expect(slots).toHaveLength(4);
    expect(within(bar()).getByRole("link", { name: "Dashboard" })).toBeTruthy();
    expect(within(bar()).getByRole("link", { name: "Recovery" })).toBeTruthy();
    expect(within(bar()).getByRole("link", { name: "Containers" })).toBeTruthy();
    expect(within(bar()).getByRole("button", { name: "More" })).toBeTruthy();
  });
});

describe("BottomNav colour engine", () => {
  it("every slot carries glim-hue and its own --item-hue, pairwise distinct", () => {
    draw("/dashboard");
    const slots = Array.from(bar().querySelectorAll("div.flex.h-14 > *")) as HTMLElement[];
    const hues = slots.map((s) => s.style.getPropertyValue("--item-hue"));
    for (const hue of hues) {
      expect(hue).toMatch(/^var\(--rb-[0-7]\)$/);
    }
    expect(new Set(hues).size).toBe(hues.length);
    for (const slot of slots) {
      expect(slot.className).toContain("glim-hue");
    }
  });

  it("the active destination slot fills with the accent and carries glim-active; resting slots do not", () => {
    draw("/dashboard");
    const active = within(bar()).getByRole("link", { name: "Dashboard" });
    expect(active.className).toContain("bg-accent");
    expect(active.className).toContain("text-accentContrast");
    expect(active.className).toContain("glim-active");
    const resting = within(bar()).getByRole("link", { name: "Containers" });
    expect(resting.className).not.toContain("bg-accent");
    expect(resting.className).not.toContain("glim-active");
  });
});

describe("BottomNav More trigger", () => {
  it("announces a dialog and tracks the sheet's open state in aria-expanded", () => {
    draw("/dashboard");
    const trigger = within(bar()).getByRole("button", { name: "More" });
    expect(trigger.getAttribute("aria-haspopup")).toBe("dialog");
    expect(trigger.getAttribute("aria-expanded")).toBe("false");
    fireEvent.click(trigger);
    expect(trigger.getAttribute("aria-expanded")).toBe("true");
  });

  it("is filled while a More-side route is current (Settings), not while a bar destination is", () => {
    draw("/settings");
    const onMoreRoute = within(bar()).getByRole("button", { name: "More" });
    expect(onMoreRoute.className).toContain("bg-accent");
    expect(onMoreRoute.className).toContain("glim-active");
    // The state is announced as well as painted, so it survives a screen
    // reader and forced colours; the destination slots get the same from
    // NavLink.
    expect(onMoreRoute.getAttribute("aria-current")).toBe("true");
    cleanup();
    draw("/dashboard");
    const onBarRoute = within(bar()).getByRole("button", { name: "More" });
    expect(onBarRoute.className).not.toContain("bg-accent");
    expect(onBarRoute.className).not.toContain("glim-active");
    expect(onBarRoute.getAttribute("aria-current")).toBeNull();
  });

  it("is filled on a gated More-side route exactly while its gate is on: the registry lookup decides, never the path", () => {
    // /vms has no literal in the trigger's active logic (the retired
    // special-case disjunct did): its More-side membership is purely the
    // enabled flag in the registry, so the gate flip is what this assertion
    // rides.
    draw("/vms", { vmsEnabled: true } as Settings);
    const gateOn = within(bar()).getByRole("button", { name: "More" });
    expect(gateOn.className).toContain("bg-accent");
    expect(gateOn.getAttribute("aria-current")).toBe("true");
    cleanup();
    draw("/vms");
    const gateOff = within(bar()).getByRole("button", { name: "More" });
    expect(gateOff.className).not.toContain("bg-accent");
    expect(gateOff.getAttribute("aria-current")).toBeNull();
  });

  it("hands the sheet a rotation position that continues the bar's, not a restart", () => {
    draw("/dashboard");
    const slots = Array.from(bar().querySelectorAll("div.flex.h-14 > *")) as HTMLElement[];
    // The DOM row ends with the More trigger; the destination slots are
    // everything before it, so the trigger's own position is the count of
    // destinations and the sheet's first row is the one after the trigger.
    const triggerHue = slots[slots.length - 1].style.getPropertyValue("--item-hue");
    fireEvent.click(within(bar()).getByRole("button", { name: "More" }));
    const firstRow = screen.getByTestId("more-sheet").querySelector("a") as HTMLElement;
    const firstRowHue = firstRow.style.getPropertyValue("--item-hue");
    // Restarting at the palette's first colour would repeat the bar's first
    // slot's colour on the same screen.
    expect(firstRowHue).not.toBe(triggerHue);
    expect(firstRowHue).toBe(String(hueVars(slots.length)["--item-hue"]));
  });
});

// ---------------------------------------------------------------------------
// The bar's own label axis ("bottombar", lib/controls.ts); behaviour, not
// just the source asserts mobileShellSource.test.ts carries. The four modes
// change what a slot's caption is: visible text, or sr-only words behind an
// aria-label, or the reactive at-rest reveal. The contract that matters most:
// a hiding mode never leaves a slot unnamed; the caption's words move into
// aria-label (and open the tip bubble), so a glyph bar is never eleven
// unnamed pictures. jsdom computes no layout, and nothing here needs it: the
// modes' observable surface in a dom environment is exactly classes and
// attributes. The axis preference arrives the way the real app delivers it;
// the localStorage key getLabelMode reads; never by poking the hook.
// ---------------------------------------------------------------------------
describe("BottomNav label axis (bottombar)", () => {
  const AXIS_KEY = "bv-labels-bottombar";

  afterEach(() => {
    localStorage.removeItem(AXIS_KEY);
  });

  function drawInMode(mode: string) {
    localStorage.setItem(AXIS_KEY, mode);
    return draw("/dashboard");
  }

  function allSlots(): HTMLElement[] {
    return Array.from(bar().querySelectorAll("div.flex.h-14 > *")) as HTMLElement[];
  }

  it("text mode shows every caption and names slots through their text alone", () => {
    drawInMode("text");
    const slots = allSlots();
    expect(slots).toHaveLength(4);
    for (const slot of slots) {
      expect(slot.querySelector("span.sr-only")).toBeNull();
      expect(slot.querySelector("span.glim-label-reactive")).toBeNull();
      expect(slot.getAttribute("aria-label")).toBeNull();
      expect(slot.getAttribute("title")).toBeNull();
      expect(slot.style.getPropertyValue("--reactive-chars")).toBe("");
    }
    expect(within(bar()).getByRole("link", { name: "Dashboard" })).toBeTruthy();
  });

  it("glyph mode hides the captions and keeps every slot named (aria-label plus the tip bubble)", () => {
    drawInMode("glyph");
    for (const slot of allSlots()) {
      expect(slot.querySelector("span.sr-only")).not.toBeNull();
      expect(slot.getAttribute("aria-label")).toBeTruthy();
      // The native title is retired here the way the sidebar retired it
      // (lib/useTipBubble): the slot opens the app's own bubble on hover and
      // focus, and the bubble says what the aria-label says.
      expect(slot.getAttribute("title")).toBeNull();
      fireEvent.mouseEnter(slot);
      const bubble = document.body.querySelector(".glim-bubble");
      expect(bubble).not.toBeNull();
      expect(bubble!.textContent).toBe(slot.getAttribute("aria-label"));
      fireEvent.mouseLeave(slot);
      expect(document.body.querySelector(".glim-bubble")).toBeNull();
    }
    // The accessible names survive the hiding: role queries resolve through
    // aria-label exactly as assistive tech reads them.
    expect(within(bar()).getByRole("link", { name: "Dashboard" })).toBeTruthy();
    expect(within(bar()).getByRole("link", { name: "Recovery" })).toBeTruthy();
    expect(within(bar()).getByRole("link", { name: "Containers" })).toBeTruthy();
    expect(within(bar()).getByRole("button", { name: "More" })).toBeTruthy();
  });

  it("reactive mode arms the coarse at-rest reveal: glim-reactive slots, reactive captions, sized ceilings", () => {
    drawInMode("reactive");
    for (const slot of allSlots()) {
      // Hiding mode, so the naming contract from glyph mode still holds.
      expect(slot.getAttribute("aria-label")).toBeTruthy();
      expect(slot.className).toContain("glim-reactive");
      // The caption is neither shown flat nor sr-only: it is the reactive
      // label the index.css coarse block reveals at rest.
      const caption = slot.querySelector("span.glim-label-reactive");
      expect(caption).not.toBeNull();
      expect(slot.querySelector("span.sr-only")).toBeNull();
      // The reveal's ceiling comes from the label's own length; the slot
      // carries the per-label width var the CSS formula consumes.
      expect(slot.style.getPropertyValue("--reactive-chars")).not.toBe("");
    }
  });
});
