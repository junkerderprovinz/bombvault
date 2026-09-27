// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { useState } from "react";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { TapPopover, type TapPopoverProps } from "./TapPopover";

// Behavioral proof for the tap popover: tap-open, three dismissal paths
// (consuming backdrop, Escape, re-tap), light focus — in on open, restored to
// the trigger on every path — and explicitly NO focus trap.
//
// Class-token assertions (fixed/inset-0, -translate-x-1/2, w-max, glim-fade)
// follow the same documented exception as BottomSheet.dom.test.tsx and
// ConfirmSheet.dom.test.tsx: these are STYLING/geometry contracts and jsdom
// computes no geometry, so the observable form of the contract IS the token.
// Presence assertions, never whole-class snapshots.
//
// Plain DOM assertions only (no jest-dom), matching the repo convention.

function Fixture(props: Partial<TapPopoverProps> = {}) {
  return (
    <div>
      <button type="button" onClick={() => {}}>
        Background
      </button>
      <TapPopover label="Filter options" trigger={<span>Filters</span>} {...props}>
        <button type="button">Panel control</button>
      </TapPopover>
    </div>
  );
}

// The backdrop is a portal child that unmounts with the popover — a captured
// reference goes stale across a close/reopen cycle (the element is detached,
// and a dispatched click on it never reaches React's root listener). Always
// query it fresh for the open under test.
function backdropEl(): HTMLElement {
  return document.body.querySelector<HTMLElement>("div[aria-hidden='true']")!;
}

function openFixture(props: Partial<TapPopoverProps> = {}) {
  render(<Fixture {...props} />);
  const trigger = screen.getByRole("button", { name: "Filters" });
  // A real tap focuses the trigger first (mousedown → focus → click); jsdom's
  // fireEvent.click does not, so the focus tests set it explicitly.
  trigger.focus();
  fireEvent.click(trigger);
  return {
    trigger,
    panel: screen.getByRole("dialog", { name: "Filter options" }),
    backdrop: backdropEl(),
  };
}

describe("TapPopover", () => {
  afterEach(cleanup);

  it("opens on trigger tap and reports the aria-expanded + aria-haspopup=dialog pair", () => {
    render(<Fixture />);
    const trigger = screen.getByRole("button", { name: "Filters" });
    expect(trigger.getAttribute("aria-expanded")).toBe("false");
    expect(trigger.getAttribute("aria-haspopup")).toBe("dialog");

    trigger.focus();
    fireEvent.click(trigger);

    expect(screen.getByRole("dialog", { name: "Filter options" })).toBeTruthy();
    expect(trigger.getAttribute("aria-expanded")).toBe("true");
  });

  it("dismisses on an outside tap that lands on the transparent consuming backdrop — never on the content beneath", () => {
    const onBackgroundClick = vi.fn();
    render(
      <div>
        <button type="button" onClick={onBackgroundClick}>
          Background
        </button>
        <TapPopover label="Filter options" trigger={<span>Filters</span>}>
          <button type="button">Panel control</button>
        </TapPopover>
      </div>,
    );
    const trigger = screen.getByRole("button", { name: "Filters" });
    fireEvent.click(trigger);
    const backdrop = document.body.querySelector<HTMLElement>("div[aria-hidden='true']")!;

    // The layer is a real full-viewport fixed element (a real
    // hit target, so outside taps die here) and visually transparent (a
    // popover is not a modal — no scrim tint).
    expect(backdrop.className).toContain("fixed");
    expect(backdrop.className).toContain("inset-0");
    expect(backdrop.className).not.toContain("bg-");

    // A tap anywhere outside the panel hits THIS element (it covers the
    // viewport). The click closes the popover and does not reach the
    // background control behind it.
    fireEvent.click(backdrop);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(onBackgroundClick).not.toHaveBeenCalled();
  });

  it("dismisses on Escape (document-level, regardless of focus)", () => {
    const { panel } = openFixture();
    fireEvent.keyDown(panel, { key: "Escape" });
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("re-tapping the trigger toggles it closed", () => {
    const { trigger } = openFixture();
    fireEvent.click(trigger);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(trigger.getAttribute("aria-expanded")).toBe("false");
  });

  it("moves focus into the panel on open and restores it to the trigger on EVERY dismissal path", () => {
    // Path 1: Escape.
    const { trigger, panel } = openFixture();
    expect(document.activeElement).toBe(panel);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(document.activeElement).toBe(trigger);

    // Path 2: outside tap on the backdrop (fresh query — the reopen mounted
    // a NEW backdrop element; the captured one is detached).
    trigger.focus();
    fireEvent.click(trigger);
    fireEvent.click(backdropEl());
    expect(document.activeElement).toBe(trigger);

    // Path 3: re-tap (direct dispatch on the trigger).
    trigger.focus();
    fireEvent.click(trigger);
    fireEvent.click(trigger);
    expect(document.activeElement).toBe(trigger);
  });

  it("does NOT trap Tab — focus can leave the panel to outside content and Tab is never preventDefault()ed", () => {
    const { panel } = openFixture();
    expect(document.activeElement).toBe(panel);

    // No Tab containment cycling: the popover's listeners must leave a Tab
    // keydown cancelable and unhandled.
    const tab = new KeyboardEvent("keydown", { key: "Tab", bubbles: true, cancelable: true });
    document.dispatchEvent(tab);
    expect(tab.defaultPrevented).toBe(false);

    // And focus itself moves out freely.
    const background = screen.getByRole("button", { name: "Background" });
    background.focus();
    expect(document.activeElement).toBe(background);
    expect(screen.getByRole("dialog", { name: "Filter options" })).toBeTruthy();
  });

  it("positions the panel off the trigger rect via computeBubblePosition before paint (centred-X pairing)", () => {
    const { panel } = openFixture();
    // computeBubblePosition's clamped center X, applied before first paint.
    expect(panel.style.left).toMatch(/px$/);
    expect(panel.style.top).toMatch(/px$/);
    // The centre-pairing contract: left is a CENTER, so the panel must carry
    // w-max + -translate-x-1/2 (the .glim-bubble engine pairing) — dropping
    // either puts the panel's left EDGE at the trigger's center.
    expect(panel.className).toContain("w-max");
    expect(panel.className).toContain("-translate-x-1/2");
    // Same entrance engine as every other floating surface.
    expect(panel.className).toContain("glim-fade");
  });

  it("controlled mode: the owner's state decides, every dismissal reports through onOpenChange", () => {
    function Controlled() {
      const [open, setOpen] = useState(true);
      return (
        <TapPopover
          label="Filter options"
          open={open}
          onOpenChange={setOpen}
          trigger={<span>Filters</span>}
        >
          <button type="button">Panel control</button>
        </TapPopover>
      );
    }
    render(<Controlled />);
    const backdrop = document.body.querySelector<HTMLElement>("div[aria-hidden='true']")!;

    fireEvent.click(backdrop);
    // onOpenChange(false) flowed to the owner, whose state flip unmounted the
    // panel — the controlled loop, end to end (ColorPickerPopover's shape).
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("a disabled trigger does not open", () => {
    render(<Fixture disabled />);
    const trigger = screen.getByRole("button", { name: "Filters" });
    expect((trigger as HTMLButtonElement).disabled).toBe(true);
    fireEvent.click(trigger);
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});
