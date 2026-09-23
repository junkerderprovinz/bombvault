// @vitest-environment jsdom
// DropdownListbox portals its panel into document.body, because a card with
// `overflow-hidden` clips an absolutely positioned child whatever its z-index.
// The second half checks the interactions the portal puts at risk, above all
// that pressing an option does not count as an outside click.
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { useRef, useState } from "react";
import { DropdownListbox } from "./DropdownListbox";

/** A trigger inside a `relative overflow-hidden` card like ContainerRow's, so
 *  the clipping ancestor is present. */
function Harness({
  onPick,
  multiselectable,
}: {
  onPick?: (value: string) => void;
  multiselectable?: boolean;
}) {
  const [open, setOpen] = useState(false);
  // On the button, as the real call sites attach it; see the width test.
  const ref = useRef<HTMLButtonElement>(null);
  return (
    <div data-testid="card" className="relative overflow-hidden bg-carbon-surface rounded-card p-4">
      <div className="inline-block">
        <button ref={ref} type="button" aria-haspopup="listbox" aria-expanded={open} onClick={() => setOpen((v) => !v)}>
          Trigger
        </button>
        <DropdownListbox
          open={open}
          onClose={() => setOpen(false)}
          triggerRef={ref}
          label="Picker"
          multiselectable={multiselectable}
        >
          {["alpha", "beta"].map((v) => (
            <button key={v} type="button" role="option" aria-selected={false} onClick={() => onPick?.(v)}>
              {v}
            </button>
          ))}
        </DropdownListbox>
      </div>
      <div data-testid="outside">outside</div>
    </div>
  );
}

afterEach(() => {
  cleanup();
});

describe("DropdownListbox escaping the clipping ancestor", () => {
  it("renders the panel outside the overflow-hidden card, as a direct child of document.body", () => {
    render(<Harness />);
    fireEvent.click(screen.getByRole("button", { name: "Trigger" }));
    const panel = screen.getByRole("listbox");
    const card = screen.getByTestId("card");
    expect(card.contains(panel)).toBe(false);
    expect(panel.parentElement).toBe(document.body);
  });

  it("positions the panel as fixed chrome, not in the card's own flow", () => {
    render(<Harness />);
    fireEvent.click(screen.getByRole("button", { name: "Trigger" }));
    const panel = screen.getByRole("listbox");
    // jsdom loads no stylesheet, so the class is what can be checked.
    expect(panel.classList.contains("fixed")).toBe(true);
    // computeBubblePosition's `left` is the centre, so the panel is shifted
    // back by half its width.
    expect(panel.style.transform).toBe("translateX(-50%)");
  });

  it("is not in the DOM at all while closed", () => {
    render(<Harness />);
    expect(screen.queryByRole("listbox")).toBeNull();
  });

  it("takes the trigger's width as a minimum and lets its content decide the rest", () => {
    // The trigger's width is only a floor, since a panel pinned to it would
    // truncate options wider than a compact trigger. jsdom reports 0 for every
    // layout read, so the rect is stubbed.
    render(<Harness />);
    const trigger = screen.getByRole("button", { name: "Trigger" });
    trigger.getBoundingClientRect = () =>
      ({ left: 100, right: 356, top: 40, bottom: 68, width: 256, height: 28, x: 100, y: 40, toJSON: () => ({}) }) as DOMRect;
    fireEvent.click(trigger);
    const panel = screen.getByRole("listbox");
    expect(panel.style.minWidth).toBe("256px");
    expect(panel.style.width).toBe("max-content");
  });
});

describe("DropdownListbox interaction across the portal", () => {
  it("pressing an option fires its onClick without dismissing the panel first", () => {
    const onPick = vi.fn();
    render(<Harness onPick={onPick} multiselectable />);
    fireEvent.click(screen.getByRole("button", { name: "Trigger" }));

    // The mousedown lands in the portalled panel, which the dismissal listener
    // has to exempt, or the panel unmounts before the click arrives.
    const option = screen.getByRole("option", { name: "alpha" });
    fireEvent.mouseDown(option);
    expect(screen.queryByRole("listbox")).not.toBeNull();
    fireEvent.click(option);
    expect(onPick).toHaveBeenCalledWith("alpha");
    // Multi-select: still open afterwards, so a second pick needs no reopen.
    expect(screen.queryByRole("listbox")).not.toBeNull();
  });

  it("closes on an outside mousedown", () => {
    render(<Harness />);
    fireEvent.click(screen.getByRole("button", { name: "Trigger" }));
    expect(screen.queryByRole("listbox")).not.toBeNull();
    fireEvent.mouseDown(screen.getByTestId("outside"));
    expect(screen.queryByRole("listbox")).toBeNull();
  });

  it("closes on Escape", () => {
    render(<Harness />);
    fireEvent.click(screen.getByRole("button", { name: "Trigger" }));
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("listbox")).toBeNull();
  });

  it("leaves a mousedown on the trigger to the trigger's own toggle", () => {
    render(<Harness />);
    const trigger = screen.getByRole("button", { name: "Trigger" });
    fireEvent.click(trigger);
    fireEvent.mouseDown(trigger);
    expect(screen.queryByRole("listbox")).not.toBeNull();
  });

  it("closes when an ancestor scrolls, but not when its own list scrolls", () => {
    render(<Harness />);
    fireEvent.click(screen.getByRole("button", { name: "Trigger" }));
    // Scrolling the list itself does not move it away from its trigger.
    fireEvent.scroll(screen.getByRole("listbox"));
    expect(screen.queryByRole("listbox")).not.toBeNull();
    fireEvent.scroll(window);
    expect(screen.queryByRole("listbox")).toBeNull();
  });

  it("carries aria-multiselectable only when asked for it", () => {
    const { unmount } = render(<Harness multiselectable />);
    fireEvent.click(screen.getByRole("button", { name: "Trigger" }));
    expect(screen.getByRole("listbox").getAttribute("aria-multiselectable")).toBe("true");
    unmount();

    render(<Harness />);
    fireEvent.click(screen.getByRole("button", { name: "Trigger" }));
    expect(screen.getByRole("listbox").getAttribute("aria-multiselectable")).toBeNull();
  });
});
