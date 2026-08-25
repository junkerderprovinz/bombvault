// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// DropdownListbox — the shared portalled listbox panel. This is the
// regression guard for the live bug it was extracted to fix (jdp, Containers
// tab): the panel used to render as an `absolute` child of its trigger's
// wrapper, and ContainerRow's card is `relative overflow-hidden`, which
// hard-clips absolutely-positioned descendants no matter what z-index they
// carry — so the list stopped dead at the card's bottom edge.
//
// The first test therefore asserts the thing that actually fixes it: the
// panel is NOT a descendant of the overflow-hidden box at all. Asserting
// "the panel exists and has role=listbox" would have passed against the
// broken build too — it always existed, it was just clipped.
//
// The second half covers the behaviours the portal put at risk. The one that
// matters most is option-press: the old per-call-site dismissal handlers
// asked "is this mousedown inside the TRIGGER's wrapper?", which a portalled
// option button is not, so keeping them would have unmounted the panel on
// mousedown and the option's own click would never have fired — a list that
// closes without selecting anything.
//
// jsdom opted in explicitly (real DOM/click/keyboard behaviour needed) — see
// Selector.dom.test.tsx's own header for this repo's naming convention.
// ---------------------------------------------------------------------------
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { useRef, useState } from "react";
import { DropdownListbox } from "./DropdownListbox";

/** A miniature of the real call site: a trigger inside the exact
 *  `relative overflow-hidden` card shell ContainerRow renders (Containers.tsx
 *  line ~1465), so the clipping ancestor this component exists to escape is
 *  genuinely present in the tree under test. */
function Harness({
  onPick,
  multiselectable,
}: {
  onPick?: (value: string) => void;
  multiselectable?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  return (
    <div data-testid="card" className="relative overflow-hidden bg-carbon-surface rounded-card p-4">
      <div className="relative inline-block" ref={ref}>
        <button type="button" aria-haspopup="listbox" aria-expanded={open} onClick={() => setOpen((v) => !v)}>
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

describe("DropdownListbox — escaping the clipping ancestor", () => {
  it("renders the panel outside the overflow-hidden card, as a direct child of document.body", () => {
    render(<Harness />);
    fireEvent.click(screen.getByRole("button", { name: "Trigger" }));
    const panel = screen.getByRole("listbox");
    const card = screen.getByTestId("card");
    // THE assertion: not merely "the panel exists", but "no overflow-hidden
    // ancestor can clip it any more".
    expect(card.contains(panel)).toBe(false);
    expect(panel.parentElement).toBe(document.body);
  });

  it("positions the panel as fixed chrome, not in the card's own flow", () => {
    render(<Harness />);
    fireEvent.click(screen.getByRole("button", { name: "Trigger" }));
    const panel = screen.getByRole("listbox");
    // Class, not computed style: no stylesheet is loaded under jsdom, so the
    // Tailwind utility is the only observable form the positioning takes here.
    expect(panel.classList.contains("fixed")).toBe(true);
    // Centred on the trigger — computeBubblePosition's `left` is a CENTRE, so
    // the translate is what makes the panel line up with the button.
    expect(panel.style.transform).toBe("translateX(-50%)");
  });

  it("is not in the DOM at all while closed", () => {
    render(<Harness />);
    expect(screen.queryByRole("listbox")).toBeNull();
  });
});

describe("DropdownListbox — interaction contract kept across the portal", () => {
  it("pressing an option fires its onClick and does NOT dismiss the panel first", () => {
    const onPick = vi.fn();
    render(<Harness onPick={onPick} multiselectable />);
    fireEvent.click(screen.getByRole("button", { name: "Trigger" }));

    // mousedown lands inside the (portalled) panel — the dismissal listener
    // must exempt it, or the panel unmounts before the click arrives.
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

  it("a mousedown on the trigger itself does not run the dismissal (the trigger owns its own toggle)", () => {
    render(<Harness />);
    const trigger = screen.getByRole("button", { name: "Trigger" });
    fireEvent.click(trigger);
    fireEvent.mouseDown(trigger);
    expect(screen.queryByRole("listbox")).not.toBeNull();
  });

  it("closes when an ancestor scrolls, but NOT when the panel's own scrollable list is scrolled", () => {
    render(<Harness />);
    fireEvent.click(screen.getByRole("button", { name: "Trigger" }));
    // The panel is `max-h-60 overflow-y-auto`: scrolling the list itself
    // never de-anchors it from its trigger, so it must survive that.
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
