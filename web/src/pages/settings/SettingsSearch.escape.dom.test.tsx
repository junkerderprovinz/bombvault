// @vitest-environment jsdom
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, useLocation } from "react-router-dom";
import { I18nProvider, en } from "../../lib/i18n";
import { SettingsSearch } from "./SettingsSearch";
import { SETTINGS_PAGES } from "./settingsPages";

let main: HTMLElement;

function Where() {
  const { pathname, hash } = useLocation();
  return <output data-testid="where">{pathname + hash}</output>;
}

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

function renderSearch() {
  return render(
    <I18nProvider>
      <MemoryRouter initialEntries={["/settings/general"]}>
        <SettingsSearch pages={SETTINGS_PAGES} />
        <input aria-label="another field" />
        <Where />
      </MemoryRouter>
    </I18nProvider>,
    { container: main }
  );
}

const searchField = () => screen.queryByRole("combobox", { name: en["settings.search.placeholder"] });

it("comes up with the cursor in it on a slash and on Ctrl+K", () => {
  renderSearch();
  expect(searchField()).toBeNull();

  fireEvent.keyDown(document.body, { key: "/" });
  expect(document.activeElement).toBe(searchField());

  fireEvent.keyDown(searchField() as HTMLElement, { key: "Escape" });
  expect(searchField()).toBeNull();

  fireEvent.keyDown(document.body, { key: "k", ctrlKey: true });
  expect(document.activeElement).toBe(searchField());
});

it("leaves a slash typed into a field alone, and takes Ctrl+K from there", () => {
  renderSearch();
  const other = screen.getByLabelText("another field");
  act(() => other.focus());

  expect(fireEvent.keyDown(other, { key: "/" })).toBe(true);
  expect(searchField()).toBeNull();

  expect(fireEvent.keyDown(other, { key: "k", metaKey: true })).toBe(false);
  expect(document.activeElement).toBe(searchField());
});

it("leads a name from an earlier version to where its cards went, and says so", () => {
  renderSearch();
  fireEvent.keyDown(document.body, { key: "/" });
  fireEvent.change(searchField() as HTMLElement, { target: { value: "off-site" } });

  const hit = screen.getAllByRole("option")[0];
  expect(hit.textContent).toContain(en["settings.tab.offsite"]);
  expect(hit.textContent).toContain(en["tile.search.former"].replace("{tab}", en["storage.title"]));

  fireEvent.click(hit);
  expect(screen.getByTestId("where").textContent).toBe("/settings/storage#offsite");
});

it("leaves the focus where it is when the bar comes back on a scroll up after a search", () => {
  renderSearch();
  fireEvent.keyDown(document.body, { key: "/" });
  fireEvent.change(searchField() as HTMLElement, { target: { value: "off-site" } });
  fireEvent.click(screen.getAllByRole("option")[0]);
  expect(searchField()).toBeNull();

  const other = screen.getByLabelText("another field");
  act(() => other.focus());
  act(() => void main.dispatchEvent(new WheelEvent("wheel", { deltaY: -40 })));
  expect(searchField()).not.toBeNull();
  expect(document.activeElement).toBe(other);
});
