// @vitest-environment jsdom
// The panel is portalled to document.body, so the next Tab from the trigger
// would never reach it. It moves focus in on open and hands it back on close.
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { useRef, useState } from "react";
import { DropdownListbox } from "./DropdownListbox";

function Harness({ selected = "b" }: { selected?: string }) {
  const triggerRef = useRef<HTMLButtonElement>(null);
  const [open, setOpen] = useState(false);
  return (
    <div>
      <button ref={triggerRef} onClick={() => setOpen((v) => !v)}>
        open
      </button>
      <button>elsewhere</button>
      <DropdownListbox open={open} triggerRef={triggerRef} onClose={() => setOpen(false)} label="letters">
        {["a", "b", "c"].map((v) => (
          <button key={v} role="option" aria-selected={v === selected} onClick={() => setOpen(false)}>
            {v}
          </button>
        ))}
      </DropdownListbox>
    </div>
  );
}

/** Opening takes a layout effect and a state update; act() lets both commit. */
async function open(selected?: string) {
  render(<Harness selected={selected} />);
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: "open" }));
  });
}

function option(name: string) {
  return screen.getByRole("option", { name });
}

beforeEach(() => {
  // jsdom lacks scrollIntoView, which the panel calls while focusing.
  Element.prototype.scrollIntoView = function () {};
});

afterEach(cleanup);

describe("DropdownListbox keyboard access", () => {
  it("moves focus onto the selected option when it opens", async () => {
    await open("b");
    expect(document.activeElement).toBe(option("b"));
  });

  it("falls back to the first option when nothing is selected", async () => {
    await open("nothing-matches");
    expect(document.activeElement).toBe(option("a"));
  });

  it("walks the options with the arrow keys, wrapping at both ends", async () => {
    await open("a");
    const panel = screen.getByRole("listbox");

    fireEvent.keyDown(panel, { key: "ArrowDown" });
    expect(document.activeElement).toBe(option("b"));

    fireEvent.keyDown(panel, { key: "ArrowUp" });
    expect(document.activeElement).toBe(option("a"));

    // Wrap backwards off the start, and forwards off the end.
    fireEvent.keyDown(panel, { key: "ArrowUp" });
    expect(document.activeElement).toBe(option("c"));
    fireEvent.keyDown(panel, { key: "ArrowDown" });
    expect(document.activeElement).toBe(option("a"));
  });

  it("jumps to the ends with Home and End", async () => {
    await open("b");
    const panel = screen.getByRole("listbox");

    fireEvent.keyDown(panel, { key: "End" });
    expect(document.activeElement).toBe(option("c"));
    fireEvent.keyDown(panel, { key: "Home" });
    expect(document.activeElement).toBe(option("a"));
  });

  it("gives focus back to the trigger when it closes from inside", async () => {
    await open("b");
    const trigger = screen.getByRole("button", { name: "open" });
    // jsdom does not focus a button on click, so focus is set explicitly.
    option("b").focus();
    await act(async () => {
      fireEvent.click(option("b"));
    });
    expect(document.activeElement).toBe(trigger);
  });

  it("leaves focus alone when it closes with focus outside the panel", async () => {
    // Someone who moved focus elsewhere has chosen where it should be. A real
    // focus move gives the panel a blur whose relatedTarget is outside it.
    await open("b");
    const trigger = screen.getByRole("button", { name: "open" });
    const elsewhere = screen.getByRole("button", { name: "elsewhere" });
    await act(async () => {
      elsewhere.focus();
    });
    await act(async () => {
      fireEvent.click(option("b"));
    });
    expect(document.activeElement).toBe(elsewhere);
    expect(document.activeElement).not.toBe(trigger);
  });
});
