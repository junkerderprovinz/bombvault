// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { en, I18nProvider } from "../../lib/i18n";
import type { MenuAction } from "./ActionMenu";
import { MoreMenu } from "./MoreMenu";

afterEach(cleanup);

const more = () => screen.queryByRole("button", { name: en["nav.more"] });

function action(id: string, onSelect = vi.fn()): MenuAction {
  return { id, label: `Do ${id}`, glyph: <i />, onSelect };
}

function renderMenu(actions: MenuAction[]) {
  render(
    <I18nProvider>
      <MoreMenu label="Row" actions={actions} newId="menu" />
    </I18nProvider>,
  );
}

describe("MoreMenu", () => {
  it("shows a single action as a button of its own", () => {
    const only = action("one");
    renderMenu([only]);

    expect(more()).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Do one" }));
    expect(only.onSelect).toHaveBeenCalledOnce();
    expect(screen.queryByRole("menu")).toBeNull();
  });

  it("opens a menu of two actions behind the More button", () => {
    const second = action("two");
    renderMenu([action("one"), second]);

    expect(screen.queryByRole("button", { name: "Do one" })).toBeNull();
    fireEvent.click(more()!);
    expect(screen.getAllByRole("menuitem").map((item) => item.textContent)).toEqual(["Do one", "Do two"]);
    fireEvent.click(screen.getByRole("menuitem", { name: "Do two" }));
    expect(second.onSelect).toHaveBeenCalledOnce();
  });

  it("keeps the mark of what it stands in for either way", () => {
    renderMenu([action("one")]);
    expect(document.querySelector('[data-new="menu"]')).not.toBeNull();
    cleanup();
    renderMenu([action("one"), action("two")]);
    expect(document.querySelector('[data-new="menu"]')).not.toBeNull();
  });

  it("holds the first row unlit when the pointer opened the menu, until a key is pressed", () => {
    renderMenu([action("one"), action("two")]);
    fireEvent.pointerDown(more()!);
    fireEvent.click(more()!);
    const menu = screen.getByRole("menu");
    expect(menu.hasAttribute("data-pointer")).toBe(true);
    expect(document.activeElement).toBe(screen.getByRole("menuitem", { name: "Do one" }));

    fireEvent.keyDown(menu, { key: "ArrowDown" });
    expect(menu.hasAttribute("data-pointer")).toBe(false);
  });

  it("shows the focus at once when the keyboard opened the menu", () => {
    renderMenu([action("one"), action("two")]);
    fireEvent.keyDown(more()!, { key: "Enter" });
    fireEvent.click(more()!);
    expect(screen.getByRole("menu").hasAttribute("data-pointer")).toBe(false);
  });
});
