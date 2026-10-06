// @vitest-environment jsdom
import { afterEach, expect, it } from "vitest";
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
      <SettingsRail items={ITEMS} active="look" label="Settings pages" onReorder={() => {}} />
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
