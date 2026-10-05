// @vitest-environment jsdom
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, en } from "../../lib/i18n";
import { SettingsSearch } from "./SettingsSearch";
import { SETTINGS_PAGES } from "./settingsPages";

let main: HTMLElement;

beforeEach(() => {
  window.localStorage.setItem("bv-lang", "en");
  main = document.createElement("main");
  main.id = "bv-main";
  document.body.appendChild(main);
  vi.spyOn(performance, "now").mockReturnValue(10_000);
});

afterEach(() => {
  cleanup();
  main.remove();
  vi.restoreAllMocks();
});

it("closes the list, then clears the text, then puts the bar away on each Escape", () => {
  render(
    <I18nProvider>
      <MemoryRouter initialEntries={["/settings/general"]}>
        <SettingsSearch pages={SETTINGS_PAGES} />
      </MemoryRouter>
    </I18nProvider>,
    { container: main }
  );
  act(() => void main.dispatchEvent(new WheelEvent("wheel", { deltaY: -40 })));

  const field = screen.getByRole("combobox", { name: en["settings.search.placeholder"] });
  act(() => field.focus());
  fireEvent.change(field, { target: { value: "sched" } });
  expect(screen.queryByRole("listbox")).not.toBeNull();

  fireEvent.keyDown(field, { key: "Escape" });
  expect(screen.queryByRole("listbox")).toBeNull();
  expect(field).toHaveProperty("value", "sched");

  fireEvent.keyDown(field, { key: "Escape" });
  expect(field).toHaveProperty("value", "");

  fireEvent.keyDown(field, { key: "Escape" });
  expect(screen.queryByRole("combobox")).toBeNull();
});
