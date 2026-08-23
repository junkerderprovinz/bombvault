// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// TimePicker — real DOM/keyboard/popover behaviour TimePicker.test.ts's pure
// parseTime/formatTime/minutesFor/nearestStep tests can't cover: the trigger
// rendering the current value, the popover actually opening/closing, hour/
// minute selection updating the value, keyboard navigation, and outside-
// click/Escape/scroll dismissal. Named `.dom.test.tsx` per
// ColorPickerPopover.dom.test.tsx's own convention for the jsdom-opted-in
// exception (vitest.config.ts stays "node" by default). No
// @testing-library/jest-dom in this repo — plain DOM property/attribute
// access instead of the toBeInTheDocument()/toHaveAttribute() matchers,
// matching ColorPickerPopover's/Selector's own tests.
// ---------------------------------------------------------------------------
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { TimePicker } from "./TimePicker";

afterEach(() => {
  cleanup();
});

describe("TimePicker — trigger", () => {
  it("shows the current value as its label, with no dialog until activated", () => {
    render(<TimePicker value="14:30" onChange={vi.fn()} label="Time" />);
    const trigger = screen.getByRole("button", { name: "Time: 14:30" });
    expect(trigger.textContent).toContain("14:30");
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("is a real button, never a typeable text field", () => {
    render(<TimePicker value="14:30" onChange={vi.fn()} label="Time" />);
    const trigger = screen.getByRole("button", { name: "Time: 14:30" });
    expect(trigger.tagName).toBe("BUTTON");
    expect(trigger.getAttribute("type")).toBe("button");
  });

  it("forces dir=ltr on the trigger regardless of page direction", () => {
    render(<TimePicker value="14:30" onChange={vi.fn()} label="Time" />);
    expect(screen.getByRole("button", { name: "Time: 14:30" }).getAttribute("dir")).toBe("ltr");
  });

  it("disabled renders a disabled trigger that never opens the popover", () => {
    render(<TimePicker value="14:30" onChange={vi.fn()} label="Time" disabled />);
    const trigger = screen.getByRole("button", { name: "Time: 14:30" }) as HTMLButtonElement;
    expect(trigger.disabled).toBe(true);
    fireEvent.click(trigger);
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});

describe("TimePicker — popover open/selection", () => {
  it("clicking the trigger opens a dialog with hour and minute listboxes", () => {
    render(<TimePicker value="14:30" onChange={vi.fn()} label="Time" />);
    fireEvent.click(screen.getByRole("button", { name: "Time: 14:30" }));
    const dialog = screen.getByRole("dialog", { name: "Time" });
    expect(dialog).toBeTruthy();
    expect(screen.getByRole("listbox", { name: "Hour" })).toBeTruthy();
    expect(screen.getByRole("listbox", { name: "Minute" })).toBeTruthy();
  });

  it("marks the current hour/minute as the selected option", () => {
    render(<TimePicker value="14:30" onChange={vi.fn()} label="Time" />);
    fireEvent.click(screen.getByRole("button", { name: "Time: 14:30" }));
    const hourBox = screen.getByRole("listbox", { name: "Hour" });
    const minuteBox = screen.getByRole("listbox", { name: "Minute" });
    expect(within(hourBox).getByRole("option", { name: "14" }).getAttribute("aria-selected")).toBe("true");
    expect(within(minuteBox).getByRole("option", { name: "30" }).getAttribute("aria-selected")).toBe("true");
  });

  it("highlights the nearest 5-minute step for an off-grid stored value, without rewriting it", () => {
    const spy = vi.fn();
    render(<TimePicker value="14:32" onChange={spy} label="Time" />);
    fireEvent.click(screen.getByRole("button", { name: "Time: 14:32" }));
    const minuteBox = screen.getByRole("listbox", { name: "Minute" });
    expect(within(minuteBox).getByRole("option", { name: "30" }).getAttribute("aria-selected")).toBe("true");
    // Merely opening the popover must not call onChange — no silent rewrite.
    expect(spy).not.toHaveBeenCalled();
  });

  it("clicking an hour option commits the new value, keeping the minute", () => {
    const spy = vi.fn();
    render(<TimePicker value="14:30" onChange={spy} label="Time" />);
    fireEvent.click(screen.getByRole("button", { name: "Time: 14:30" }));
    const hourBox = screen.getByRole("listbox", { name: "Hour" });
    fireEvent.click(within(hourBox).getByRole("option", { name: "09" }));
    expect(spy).toHaveBeenCalledWith("09:30");
  });

  it("clicking a minute option commits the new value, keeping the hour", () => {
    const spy = vi.fn();
    render(<TimePicker value="14:30" onChange={spy} label="Time" />);
    fireEvent.click(screen.getByRole("button", { name: "Time: 14:30" }));
    const minuteBox = screen.getByRole("listbox", { name: "Minute" });
    fireEvent.click(within(minuteBox).getByRole("option", { name: "05" }));
    expect(spy).toHaveBeenCalledWith("14:05");
  });

  it("picking a value does not close the popover — both hour and minute can be set in one visit", () => {
    render(<TimePicker value="14:30" onChange={vi.fn()} label="Time" />);
    fireEvent.click(screen.getByRole("button", { name: "Time: 14:30" }));
    const hourBox = screen.getByRole("listbox", { name: "Hour" });
    fireEvent.click(within(hourBox).getByRole("option", { name: "09" }));
    expect(screen.queryByRole("dialog")).not.toBeNull();
  });
});

describe("TimePicker — keyboard navigation", () => {
  it("ArrowDown on the hour listbox steps to the next hour and commits it", () => {
    const spy = vi.fn();
    render(<TimePicker value="14:30" onChange={spy} label="Time" />);
    fireEvent.click(screen.getByRole("button", { name: "Time: 14:30" }));
    fireEvent.keyDown(screen.getByRole("listbox", { name: "Hour" }), { key: "ArrowDown" });
    expect(spy).toHaveBeenCalledWith("15:30");
  });

  it("ArrowUp on the hour listbox wraps from 0 to 23", () => {
    const spy = vi.fn();
    render(<TimePicker value="00:30" onChange={spy} label="Time" />);
    fireEvent.click(screen.getByRole("button", { name: "Time: 00:30" }));
    fireEvent.keyDown(screen.getByRole("listbox", { name: "Hour" }), { key: "ArrowUp" });
    expect(spy).toHaveBeenCalledWith("23:30");
  });

  it("ArrowDown on the minute listbox steps by the configured step and wraps", () => {
    const spy = vi.fn();
    render(<TimePicker value="14:55" onChange={spy} label="Time" />);
    fireEvent.click(screen.getByRole("button", { name: "Time: 14:55" }));
    fireEvent.keyDown(screen.getByRole("listbox", { name: "Minute" }), { key: "ArrowDown" });
    expect(spy).toHaveBeenCalledWith("14:00");
  });

  it("Home/End jump to the first/last hour", () => {
    const spy = vi.fn();
    render(<TimePicker value="14:30" onChange={spy} label="Time" />);
    fireEvent.click(screen.getByRole("button", { name: "Time: 14:30" }));
    const hourBox = screen.getByRole("listbox", { name: "Hour" });
    fireEvent.keyDown(hourBox, { key: "End" });
    expect(spy).toHaveBeenLastCalledWith("23:30");
    fireEvent.keyDown(hourBox, { key: "Home" });
    expect(spy).toHaveBeenLastCalledWith("00:30");
  });

  it("ArrowRight/ArrowLeft move focus between the hour and minute columns", () => {
    render(<TimePicker value="14:30" onChange={vi.fn()} label="Time" />);
    fireEvent.click(screen.getByRole("button", { name: "Time: 14:30" }));
    const hourBox = screen.getByRole("listbox", { name: "Hour" });
    const minuteBox = screen.getByRole("listbox", { name: "Minute" });
    const selectedMinuteOption = within(minuteBox).getByRole("option", { name: "30" });
    fireEvent.keyDown(hourBox, { key: "ArrowRight" });
    expect(document.activeElement).toBe(selectedMinuteOption);
    const selectedHourOption = within(hourBox).getByRole("option", { name: "14" });
    fireEvent.keyDown(minuteBox, { key: "ArrowLeft" });
    expect(document.activeElement).toBe(selectedHourOption);
  });

  it("only the currently selected option in each column is a Tab stop (roving tabindex)", () => {
    render(<TimePicker value="14:30" onChange={vi.fn()} label="Time" />);
    fireEvent.click(screen.getByRole("button", { name: "Time: 14:30" }));
    const hourBox = screen.getByRole("listbox", { name: "Hour" });
    const options = within(hourBox).getAllByRole("option");
    const tabbable = options.filter((o) => o.getAttribute("tabindex") === "0");
    expect(tabbable).toHaveLength(1);
    expect(tabbable[0].textContent).toBe("14");
  });
});

describe("TimePicker — dismissal", () => {
  it("Escape closes the open popover", () => {
    render(<TimePicker value="14:30" onChange={vi.fn()} label="Time" />);
    fireEvent.click(screen.getByRole("button", { name: "Time: 14:30" }));
    expect(screen.queryByRole("dialog")).not.toBeNull();
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("an outside click closes the open popover", () => {
    render(
      <div>
        <TimePicker value="14:30" onChange={vi.fn()} label="Time" />
        <button>elsewhere</button>
      </div>
    );
    fireEvent.click(screen.getByRole("button", { name: "Time: 14:30" }));
    expect(screen.queryByRole("dialog")).not.toBeNull();
    fireEvent.mouseDown(screen.getByRole("button", { name: "elsewhere" }));
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("a click inside the panel itself does not close it", () => {
    render(<TimePicker value="14:30" onChange={vi.fn()} label="Time" />);
    fireEvent.click(screen.getByRole("button", { name: "Time: 14:30" }));
    fireEvent.mouseDown(screen.getByRole("dialog", { name: "Time" }));
    expect(screen.queryByRole("dialog")).not.toBeNull();
  });

  it("scrolling the page closes the open popover (a fixed popover would otherwise de-anchor from its trigger)", () => {
    render(<TimePicker value="14:30" onChange={vi.fn()} label="Time" />);
    fireEvent.click(screen.getByRole("button", { name: "Time: 14:30" }));
    expect(screen.queryByRole("dialog")).not.toBeNull();
    fireEvent.scroll(window);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("scrolling INSIDE one of the popover's own listbox columns does not close it", () => {
    // Regression test for a live bug: the dismissal scroll listener is
    // capture-phase on window, so it also receives scroll events from the
    // popover's own two internal `.glim-time-col` columns — and this
    // component's own scrollIntoView calls (bringing the current hour/minute
    // into view) fire exactly such an event. Confirmed live against the real
    // container: the popover opened and closed again ~2ms later on every
    // single open, before this guard existed. A scroll INSIDE the popover
    // never de-anchors it from its trigger, unlike a page/ancestor scroll.
    render(<TimePicker value="14:30" onChange={vi.fn()} label="Time" />);
    fireEvent.click(screen.getByRole("button", { name: "Time: 14:30" }));
    const hourBox = screen.getByRole("listbox", { name: "Hour" });
    fireEvent.scroll(hourBox);
    expect(screen.queryByRole("dialog")).not.toBeNull();
  });

  it("only one TimePicker popover is ever open at once — opening a second closes the first", () => {
    render(
      <div>
        <TimePicker value="09:00" onChange={vi.fn()} label="First" />
        <TimePicker value="18:00" onChange={vi.fn()} label="Second" />
      </div>
    );
    fireEvent.click(screen.getByRole("button", { name: "First: 09:00" }));
    expect(screen.queryByRole("dialog", { name: "First" })).not.toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Second: 18:00" }));
    expect(screen.queryByRole("dialog", { name: "First" })).toBeNull();
    expect(screen.queryByRole("dialog", { name: "Second" })).not.toBeNull();
  });
});

describe("TimePicker — positioning", () => {
  it("clamps the popover into the viewport using its own real measured size (computeBubblePosition)", () => {
    // jsdom's getBoundingClientRect()/offsetWidth/offsetHeight default to
    // zero — stub them with a realistic trigger rect pinned to the right
    // edge, matching bubblePosition.test.ts's own "Wiederherstellungskit"
    // reproduction, so the horizontal clamp math actually has something to
    // clamp against.
    Object.defineProperty(window, "innerWidth", { configurable: true, value: 1024 });
    Object.defineProperty(window, "innerHeight", { configurable: true, value: 768 });
    render(<TimePicker value="14:30" onChange={vi.fn()} label="Time" />);
    const trigger = screen.getByRole("button", { name: "Time: 14:30" });
    trigger.getBoundingClientRect = () =>
      ({ x: 1000, y: 100, left: 1000, top: 100, right: 1020, bottom: 120, width: 20, height: 20, toJSON() {} }) as DOMRect;
    fireEvent.click(trigger);
    const dialog = screen.getByRole("dialog", { name: "Time" }) as HTMLElement;
    const left = parseFloat(dialog.style.left);
    // The popover is translateX(-50%)'d around `left` (matching
    // computeBubblePosition's centred-trigger contract) — its right edge is
    // left + halfWidth, which must stay within the 8px viewport margin.
    expect(left + dialog.offsetWidth / 2).toBeLessThanOrEqual(1024 - 8 + 0.001);
  });
});
