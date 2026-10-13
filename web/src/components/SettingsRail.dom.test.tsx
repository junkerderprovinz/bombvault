// @vitest-environment jsdom
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, useLocation } from "react-router-dom";
import { SettingsRail } from "./SettingsRail";

const ITEMS = ["general", "look", "storage"].map((id) => ({ id, label: id, icon: null, to: `/settings/${id}` }));

function Where() {
  return <output data-testid="where">{useLocation().pathname}</output>;
}

afterEach(cleanup);

it("is one tab stop on the open page, and the arrow keys move the focus without opening a page", () => {
  render(
    <MemoryRouter initialEntries={["/settings/look"]}>
      <SettingsRail items={ITEMS} active="look" label="Settings pages" form="column" onReorder={() => {}} />
      <Where />
    </MemoryRouter>
  );
  const tile = (name: string) => screen.getByRole("link", { name });
  expect(ITEMS.map((i) => tile(i.label).tabIndex)).toEqual([-1, 0, -1]);

  act(() => tile("look").focus());
  fireEvent.keyDown(tile("look"), { key: "ArrowDown" });
  expect(document.activeElement).toBe(tile("storage"));
  fireEvent.keyDown(tile("storage"), { key: "Home" });
  expect(document.activeElement).toBe(tile("general"));
  expect(screen.getByTestId("where").textContent).toBe("/settings/look");
});

it("moves the focused tile with Alt and an arrow, and stops at the ends", () => {
  const orders: string[][] = [];
  render(
    <MemoryRouter initialEntries={["/settings/look"]}>
      <SettingsRail items={ITEMS} active="look" label="Settings pages" form="column" onReorder={(ids) => orders.push(ids)} />
    </MemoryRouter>
  );
  const tile = (name: string) => screen.getByRole("link", { name });

  act(() => tile("look").focus());
  fireEvent.keyDown(tile("look"), { key: "ArrowDown", altKey: true });
  expect(orders).toEqual([["general", "storage", "look"]]);
  fireEvent.keyDown(tile("look"), { key: "ArrowUp", altKey: true });
  expect(orders[1]).toEqual(["look", "general", "storage"]);

  act(() => tile("general").focus());
  fireEvent.keyDown(tile("general"), { key: "ArrowUp", altKey: true });
  expect(orders).toHaveLength(2);
});

// In a strip each tile is 80px wide with 4px between, and the strip shows the
// first 200px of them.
function layOutStrip() {
  const at = (el: HTMLElement) => ITEMS.findIndex((i) => i.id === el.getAttribute("data-rail-id")) * 84;
  const spies = [
    vi.spyOn(HTMLElement.prototype, "offsetLeft", "get").mockImplementation(function (this: HTMLElement) {
      return at(this);
    }),
    vi.spyOn(HTMLElement.prototype, "offsetWidth", "get").mockReturnValue(80),
    vi.spyOn(Element.prototype, "clientWidth", "get").mockReturnValue(200),
  ];
  return { mockRestore: () => spies.forEach((spy) => spy.mockRestore()) };
}

it("scrolls the strip to the open page's tile, at once on the first draw and smoothly after", () => {
  const scrollBy = vi.fn();
  HTMLElement.prototype.scrollBy = scrollBy;
  const layout = layOutStrip();
  try {
    const { rerender } = render(
      <MemoryRouter>
        <SettingsRail items={ITEMS} active="storage" label="Settings pages" form="strip" onReorder={() => {}} />
      </MemoryRouter>
    );
    // Storage ends at 248; 48px of room after it puts its edge at 152.
    expect(scrollBy).toHaveBeenCalledTimes(1);
    expect(scrollBy).toHaveBeenLastCalledWith({ left: 96, behavior: "instant" });

    rerender(
      <MemoryRouter>
        <SettingsRail items={ITEMS} active="look" label="Settings pages" form="strip" onReorder={() => {}} />
      </MemoryRouter>
    );
    // Look ends at 164, 12px into the room kept before the right edge.
    expect(scrollBy).toHaveBeenLastCalledWith({ left: 12, behavior: "smooth" });
  } finally {
    layout.mockRestore();
    delete (HTMLElement.prototype as Partial<HTMLElement>).scrollBy;
  }
});

it("walks a strip with the left and right arrows and brings the focused tile into view", () => {
  const scrollBy = vi.fn();
  HTMLElement.prototype.scrollBy = scrollBy;
  const layout = layOutStrip();
  try {
    render(
      <MemoryRouter initialEntries={["/settings/general"]}>
        <SettingsRail items={ITEMS} active="general" label="Settings pages" form="strip" onReorder={() => {}} />
        <Where />
      </MemoryRouter>
    );
    const tile = (name: string) => screen.getByRole("link", { name });

    act(() => tile("general").focus());
    fireEvent.keyDown(tile("general"), { key: "ArrowDown" });
    expect(document.activeElement).toBe(tile("general"));

    fireEvent.keyDown(tile("general"), { key: "ArrowRight" });
    expect(document.activeElement).toBe(tile("look"));
    fireEvent.keyDown(tile("look"), { key: "ArrowRight" });
    expect(document.activeElement).toBe(tile("storage"));
    expect(scrollBy).toHaveBeenLastCalledWith({ left: 96, behavior: "smooth" });
    fireEvent.keyDown(tile("storage"), { key: "ArrowLeft" });
    expect(document.activeElement).toBe(tile("look"));
    expect(screen.getByTestId("where").textContent).toBe("/settings/general");
  } finally {
    layout.mockRestore();
    delete (HTMLElement.prototype as Partial<HTMLElement>).scrollBy;
  }
});
