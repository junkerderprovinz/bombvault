// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// ColorPickerSwatch — real DOM behaviour Colorpicker.test.ts's pure hex<->HSV
// math tests can't cover: the popover actually opening/closing, and the hex
// field round-tripping into onChange. Named `.dom.test.tsx` per
// Selector.dom.test.tsx's own convention for the jsdom-opted-in exception
// (vitest.config.ts stays "node" by default). No @testing-library/jest-dom
// in this repo — plain DOM property/attribute access instead of the
// toBeInTheDocument()/toHaveAttribute() matchers, matching Selector's own
// tests.
// ---------------------------------------------------------------------------
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { ColorPickerSwatch } from "./ColorPickerPopover";

afterEach(() => {
  cleanup();
});

describe("ColorPickerSwatch — trigger + popover", () => {
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

  it("scrolling closes the open popover (a fixed popover would otherwise de-anchor from its trigger)", () => {
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

  it("only one popover is ever open at once — opening a second closes the first", () => {
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

describe("ColorPickerSwatch — hex field round-trip", () => {
  it("typing a valid 6-digit hex calls onChange with the normalized value", () => {
    const spy = vi.fn();
    render(<ColorPickerSwatch value="#2f6feb" onChange={spy} label="Accent colour" />);
    fireEvent.click(screen.getByRole("button", { name: "Accent colour" }));
    const hexField = screen.getByLabelText("Hex") as HTMLInputElement;
    fireEvent.change(hexField, { target: { value: "#FF00AA" } });
    expect(spy).toHaveBeenCalledWith("#ff00aa");
  });

  it("keeps the field's own typed text as-is (no forced re-casing while typing)", () => {
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

  it("dragging the SV square updates the hex field to match (bidirectional sync)", () => {
    const spy = vi.fn();
    render(<ColorPickerSwatch value="#ff0000" onChange={spy} label="Accent colour" />);
    fireEvent.click(screen.getByRole("button", { name: "Accent colour" }));
    const dialog = screen.getByRole("dialog", { name: "Accent colour" });
    const sv = dialog.querySelector(".glim-picker-sv") as HTMLElement;
    // jsdom's getBoundingClientRect() is always a zero rect by default,
    // which would divide-by-zero the drag math below — stub it with a
    // realistic box so (clientX, clientY) maps to a real, predictable
    // fraction of the square instead of NaN.
    sv.getBoundingClientRect = () =>
      ({ x: 0, y: 0, left: 0, top: 0, right: 220, bottom: 112, width: 220, height: 112, toJSON() {} }) as DOMRect;
    // Top-left corner of the SV square is saturation=0, value=1 — white,
    // regardless of the starting hue (#ff0000 -> h=0).
    fireEvent.mouseDown(sv, { clientX: 0, clientY: 0 });
    expect(spy).toHaveBeenCalledWith("#ffffff");
    const hexField = screen.getByLabelText("Hex") as HTMLInputElement;
    expect(hexField.value).toBe("#ffffff");
  });
});
