// @vitest-environment jsdom
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

// fireEvent wraps the event in act(), so the state update is flushed before
// the next assertion reads it.
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
  // All five labels are in the trigger and four are invisible, so the width
  // is the longest label's.
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
  // Picking closes the list.
  expect(trigger().getAttribute("aria-expanded")).toBe("false");
});

it("steps with the wheel while closed and skips a disabled option", () => {
  render(<Harness start="vms" />);
  // flash is disabled, so one notch down from vms lands on files.
  wheel(trigger(), 120);
  expect(screen.getByTestId("value").textContent).toBe("files");
  // Clamps at the end instead of wrapping.
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
