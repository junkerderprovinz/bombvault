// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { TimePicker } from "./TimePicker";

afterEach(() => {
  cleanup();
});

describe("TimePicker trigger", () => {
  it("shows the current value and no dialog until activated", () => {
    render(<TimePicker value="14:30" onChange={vi.fn()} label="Time" />);
    const trigger = screen.getByRole("button", { name: "Time: 14:30" });
    expect(trigger.textContent).toContain("14:30");
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("is a button, not a text field", () => {
    render(<TimePicker value="14:30" onChange={vi.fn()} label="Time" />);
    const trigger = screen.getByRole("button", { name: "Time: 14:30" });
    expect(trigger.tagName).toBe("BUTTON");
    expect(trigger.getAttribute("type")).toBe("button");
  });

  it("forces dir=ltr on the trigger", () => {
    render(<TimePicker value="14:30" onChange={vi.fn()} label="Time" />);
    expect(screen.getByRole("button", { name: "Time: 14:30" }).getAttribute("dir")).toBe("ltr");
  });

  it("does not open when disabled", () => {
    render(<TimePicker value="14:30" onChange={vi.fn()} label="Time" disabled />);
    const trigger = screen.getByRole("button", { name: "Time: 14:30" }) as HTMLButtonElement;
    expect(trigger.disabled).toBe(true);
    fireEvent.click(trigger);
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});

describe("TimePicker popover", () => {
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

  it("highlights the nearest step for an off-grid value without rewriting it", () => {
    const spy = vi.fn();
    render(<TimePicker value="14:32" onChange={spy} label="Time" />);
    fireEvent.click(screen.getByRole("button", { name: "Time: 14:32" }));
    const minuteBox = screen.getByRole("listbox", { name: "Minute" });
    expect(within(minuteBox).getByRole("option", { name: "30" }).getAttribute("aria-selected")).toBe("true");
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

  it("stays open after a pick so hour and minute can be set together", () => {
    render(<TimePicker value="14:30" onChange={vi.fn()} label="Time" />);
    fireEvent.click(screen.getByRole("button", { name: "Time: 14:30" }));
    const hourBox = screen.getByRole("listbox", { name: "Hour" });
    fireEvent.click(within(hourBox).getByRole("option", { name: "09" }));
    expect(screen.queryByRole("dialog")).not.toBeNull();
  });
});

describe("TimePicker keyboard navigation", () => {
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

  it("makes only the selected option a Tab stop", () => {
    render(<TimePicker value="14:30" onChange={vi.fn()} label="Time" />);
    fireEvent.click(screen.getByRole("button", { name: "Time: 14:30" }));
    const hourBox = screen.getByRole("listbox", { name: "Hour" });
    const options = within(hourBox).getAllByRole("option");
    const tabbable = options.filter((o) => o.getAttribute("tabindex") === "0");
    expect(tabbable).toHaveLength(1);
    expect(tabbable[0].textContent).toBe("14");
  });
});

describe("TimePicker dismissal", () => {
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

  it("scrolling the page closes the open popover", () => {
    render(<TimePicker value="14:30" onChange={vi.fn()} label="Time" />);
    fireEvent.click(screen.getByRole("button", { name: "Time: 14:30" }));
    expect(screen.queryByRole("dialog")).not.toBeNull();
    fireEvent.scroll(window);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("scrolling a listbox column does not close it", () => {
    render(<TimePicker value="14:30" onChange={vi.fn()} label="Time" />);
    fireEvent.click(screen.getByRole("button", { name: "Time: 14:30" }));
    const hourBox = screen.getByRole("listbox", { name: "Hour" });
    fireEvent.scroll(hourBox);
    expect(screen.queryByRole("dialog")).not.toBeNull();
  });

  it("opening a second picker closes the first", () => {
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

describe("TimePicker positioning", () => {
  it("clamps the popover into the viewport", () => {
    // jsdom measures everything as zero, so the trigger gets a rect near the
    // right edge.
    Object.defineProperty(window, "innerWidth", { configurable: true, value: 1024 });
    Object.defineProperty(window, "innerHeight", { configurable: true, value: 768 });
    render(<TimePicker value="14:30" onChange={vi.fn()} label="Time" />);
    const trigger = screen.getByRole("button", { name: "Time: 14:30" });
    trigger.getBoundingClientRect = () =>
      ({ x: 1000, y: 100, left: 1000, top: 100, right: 1020, bottom: 120, width: 20, height: 20, toJSON() {} }) as DOMRect;
    fireEvent.click(trigger);
    const dialog = screen.getByRole("dialog", { name: "Time" }) as HTMLElement;
    const left = parseFloat(dialog.style.left);
    // The popover is centred on `left`, so its right edge is left plus half
    // its width, which must stay inside the 8px margin.
    expect(left + dialog.offsetWidth / 2).toBeLessThanOrEqual(1024 - 8 + 0.001);
  });
});
