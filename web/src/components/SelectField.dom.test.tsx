// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// SelectField — the three promises that make it a replacement rather than a
// button with a list attached (#3425, GlimStone rule 18).
//
//   1. The width does not move when the value does. A native select is as wide
//      as its widest option; a plain button is as wide as its current label,
//      which makes a row of converted pickers reflow on every pick.
//   2. It has an accessible name and announces as a listbox trigger. Replacing
//      a native control is exactly where an app loses both without noticing.
//   3. The wheel steps the CLOSED control, clamped, skipping what cannot be
//      picked.
// ---------------------------------------------------------------------------
import { useState } from "react";
import { afterEach, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { SelectField, type SelectOption } from "./SelectField";

const OPTIONS: SelectOption<string>[] = [
  { value: "all", label: "All domains" },
  { value: "containers", label: "Containers" },
  { value: "vms", label: "A considerably longer option label" },
  { value: "flash", label: "Flash", disabled: true },
  { value: "files", label: "Folders" },
];

function Harness({ start = "all" }: { start?: string }) {
  const [value, setValue] = useState(start);
  return (
    <>
      <SelectField value={value} onChange={setValue} options={OPTIONS} label="Domain" />
      <output data-testid="value">{value}</output>
    </>
  );
}

afterEach(cleanup);

function trigger(): HTMLElement {
  return screen.getByRole("combobox", { name: "Domain" });
}

// fireEvent, not a raw dispatchEvent: the handler changes React state, and an
// event fired outside act() leaves the re-render unflushed when the assertion
// reads the value one line later. The listener itself is a real, non-passive
// one (lib/selectScroll) and fireEvent reaches it exactly the same way.
function wheel(el: HTMLElement, deltaY: number) {
  fireEvent.wheel(el, { deltaY });
}

it("announces as a named listbox trigger, the way a select does", () => {
  render(<Harness />);
  expect(trigger().getAttribute("aria-haspopup")).toBe("listbox");
  expect(trigger().getAttribute("aria-expanded")).toBe("false");
});

it("carries every label so the width is the widest one, not the current one", () => {
  render(<Harness />);
  // All five are in the trigger; four are invisible. That is the whole
  // mechanism: the button's intrinsic width is the longest label's.
  for (const o of OPTIONS) {
    expect(trigger().textContent).toContain(o.label);
  }
  const shown = [...trigger().querySelectorAll("span")].filter(
    (s) => s.className.includes("row-start-1") && !s.className.includes("invisible"),
  );
  expect(shown).toHaveLength(1);
  expect(shown[0]?.textContent).toBe("All domains");
});

it("opens a listbox and picks an option", () => {
  render(<Harness />);
  fireEvent.click(trigger());
  expect(trigger().getAttribute("aria-expanded")).toBe("true");
  fireEvent.click(screen.getByRole("option", { name: "Containers" }));
  expect(screen.getByTestId("value").textContent).toBe("containers");
  // Picking closes it; a single-select list that stays open is a second click
  // to dismiss.
  expect(trigger().getAttribute("aria-expanded")).toBe("false");
});

it("steps with the wheel while closed, and steps OVER a disabled option", () => {
  render(<Harness start="vms" />);
  // vms -> flash is disabled, so one notch down lands on files.
  wheel(trigger(), 120);
  expect(screen.getByTestId("value").textContent).toBe("files");
  // And it clamps at the end rather than wrapping to the top.
  wheel(trigger(), 120);
  expect(screen.getByTestId("value").textContent).toBe("files");
  // Back up, over the disabled entry again.
  wheel(trigger(), -120);
  expect(screen.getByTestId("value").textContent).toBe("vms");
});

it("does not step while disabled", () => {
  render(
    <SelectField value="all" onChange={() => { throw new Error("must not fire"); }} options={OPTIONS} label="Domain" disabled />,
  );
  wheel(screen.getByRole("combobox", { name: "Domain" }), 120);
});
