// @vitest-environment jsdom
// DOM behaviour of ColorPickerSwatch: the popover opening and closing, and the
// hex field feeding onChange.
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { ColorPickerSwatch } from "./ColorPickerPopover";

afterEach(() => {
  cleanup();
});

describe("ColorPickerSwatch trigger and popover", () => {
  it("renders a swatch showing the current value as its background, no dialog until clicked", () => {
    render(<ColorPickerSwatch value="#2f6feb" onChange={vi.fn()} label="Accent colour" />);
    const trigger = screen.getByRole("button", { name: "Accent colour" });
    expect(trigger.style.backgroundColor).toBe("rgb(47, 111, 235)");
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("clicking the trigger opens the popover, pre-synced to the current value", () => {
    render(<ColorPickerSwatch value="#2f6feb" onChange={vi.fn()} label="Accent colour" />);
    fireEvent.click(screen.getByRole("button", { name: "Accent colour" }));
    const dialog = screen.getByRole("dialog", { name: "Accent colour" });
    expect(dialog).toBeTruthy();
    const hexField = screen.getByLabelText("Hex") as HTMLInputElement;
    expect(hexField.value).toBe("#2f6feb");
  });

  it("Escape closes the open popover", () => {
    render(<ColorPickerSwatch value="#2f6feb" onChange={vi.fn()} label="Accent colour" />);
    fireEvent.click(screen.getByRole("button", { name: "Accent colour" }));
    expect(screen.queryByRole("dialog")).not.toBeNull();
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("an outside click closes the open popover", () => {
    render(
      <div>
        <ColorPickerSwatch value="#2f6feb" onChange={vi.fn()} label="Accent colour" />
        <button>elsewhere</button>
      </div>
    );
    fireEvent.click(screen.getByRole("button", { name: "Accent colour" }));
    expect(screen.queryByRole("dialog")).not.toBeNull();
    fireEvent.mouseDown(screen.getByRole("button", { name: "elsewhere" }));
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("a click on the panel itself does not close it", () => {
    render(<ColorPickerSwatch value="#2f6feb" onChange={vi.fn()} label="Accent colour" />);
    fireEvent.click(screen.getByRole("button", { name: "Accent colour" }));
    fireEvent.mouseDown(screen.getByRole("dialog", { name: "Accent colour" }));
    expect(screen.queryByRole("dialog")).not.toBeNull();
  });

  it("scrolling closes the open popover", () => {
    render(<ColorPickerSwatch value="#2f6feb" onChange={vi.fn()} label="Accent colour" />);
    fireEvent.click(screen.getByRole("button", { name: "Accent colour" }));
    expect(screen.queryByRole("dialog")).not.toBeNull();
    fireEvent.scroll(window);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("disabled=true renders a disabled trigger that never opens the popover", () => {
    render(<ColorPickerSwatch value="#2f6feb" onChange={vi.fn()} label="Accent colour" disabled />);
    const trigger = screen.getByRole("button", { name: "Accent colour" }) as HTMLButtonElement;
    expect(trigger.disabled).toBe(true);
    fireEvent.click(trigger);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("opening a second popover closes the first", () => {
    render(
      <div>
        <ColorPickerSwatch value="#2f6feb" onChange={vi.fn()} label="First" />
        <ColorPickerSwatch value="#ff0000" onChange={vi.fn()} label="Second" />
      </div>
    );
    fireEvent.click(screen.getByRole("button", { name: "First" }));
    expect(screen.queryByRole("dialog", { name: "First" })).not.toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Second" }));
    expect(screen.queryByRole("dialog", { name: "First" })).toBeNull();
    expect(screen.queryByRole("dialog", { name: "Second" })).not.toBeNull();
  });
});

describe("ColorPickerSwatch hex field", () => {
  it("typing a valid 6-digit hex calls onChange with the normalized value", () => {
    const spy = vi.fn();
    render(<ColorPickerSwatch value="#2f6feb" onChange={spy} label="Accent colour" />);
    fireEvent.click(screen.getByRole("button", { name: "Accent colour" }));
    const hexField = screen.getByLabelText("Hex") as HTMLInputElement;
    fireEvent.change(hexField, { target: { value: "#FF00AA" } });
    expect(spy).toHaveBeenCalledWith("#ff00aa");
  });

  it("keeps the typed casing in the field", () => {
    const spy = vi.fn();
    render(<ColorPickerSwatch value="#2f6feb" onChange={spy} label="Accent colour" />);
    fireEvent.click(screen.getByRole("button", { name: "Accent colour" }));
    const hexField = screen.getByLabelText("Hex") as HTMLInputElement;
    fireEvent.change(hexField, { target: { value: "#FF00AA" } });
    expect(hexField.value).toBe("#FF00AA");
  });

  it("an incomplete/invalid typed value does not call onChange", () => {
    const spy = vi.fn();
    render(<ColorPickerSwatch value="#2f6feb" onChange={spy} label="Accent colour" />);
    fireEvent.click(screen.getByRole("button", { name: "Accent colour" }));
    const hexField = screen.getByLabelText("Hex") as HTMLInputElement;
    fireEvent.change(hexField, { target: { value: "#ff00a" } });
    expect(spy).not.toHaveBeenCalled();
  });

  it("dragging the SV square updates the hex field", () => {
    const spy = vi.fn();
    render(<ColorPickerSwatch value="#ff0000" onChange={spy} label="Accent colour" />);
    fireEvent.click(screen.getByRole("button", { name: "Accent colour" }));
    const dialog = screen.getByRole("dialog", { name: "Accent colour" });
    const sv = dialog.querySelector(".glim-picker-sv") as HTMLElement;
    // jsdom's rects are all zero, which would divide the drag math by zero.
    sv.getBoundingClientRect = () =>
      ({ x: 0, y: 0, left: 0, top: 0, right: 220, bottom: 112, width: 220, height: 112, toJSON() {} }) as DOMRect;
    // The top-left corner is saturation 0, value 1: white, whatever the hue.
    fireEvent.mouseDown(sv, { clientX: 0, clientY: 0 });
    expect(spy).toHaveBeenCalledWith("#ffffff");
    const hexField = screen.getByLabelText("Hex") as HTMLInputElement;
    expect(hexField.value).toBe("#ffffff");
  });
});
