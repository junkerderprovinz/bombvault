// @vitest-environment jsdom
// The rail in glyph mode. jsdom checks the classes and the DOM; the centring
// itself is CSS and needs a browser.
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { Sidebar } from "./Sidebar";
import { I18nProvider } from "../lib/i18n";
import { AdvancedProvider } from "../lib/advanced";
import { setLabelMode } from "../lib/controls";

function renderSidebar() {
  return render(
    <MemoryRouter initialEntries={["/"]}>
      <I18nProvider>
        <AdvancedProvider>
          <Sidebar settings={null} />
        </AdvancedProvider>
      </I18nProvider>
    </MemoryRouter>
  );
}

/** Every row of the rail, the view toggle included. */
function railRows(): HTMLElement[] {
  return [
    ...document.querySelectorAll<HTMLElement>("aside .glim-nav-row"),
  ];
}

beforeEach(() => {
  localStorage.clear();
});

afterEach(() => {
  cleanup();
  localStorage.clear();
});

describe("glyph mode centres the rail", () => {
  it("centres every row, the view toggle included", () => {
    setLabelMode("sidebar", "glyph");
    renderSidebar();
    const rows = railRows();
    // Dashboard, Recovery, Containers, the view toggle and Settings. The count
    // keeps the loop below from passing on an empty list.
    expect(rows.length).toBe(5);
    for (const row of rows) expect(row.className).toContain("justify-center");
  });

  it("leaves the rows left-aligned in both text modes", () => {
    for (const mode of ["text", "textGlyph"] as const) {
      cleanup();
      setLabelMode("sidebar", mode);
      renderSidebar();
      for (const row of railRows()) expect(row.className).not.toContain("justify-center");
    }
  });
});

describe("glyph mode is the only mode that narrows the rail", () => {
  function rail(): HTMLElement {
    const el = document.querySelector<HTMLElement>("aside");
    if (!el) throw new Error("no rail rendered");
    return el;
  }

  it("drops the rail to the house width and the mark to 44px in glyph mode", () => {
    setLabelMode("sidebar", "glyph");
    renderSidebar();
    // The shared token, not a width of this app's own.
    expect(rail().className).toContain("w-(--rail-narrow)");
    expect(rail().className).not.toContain("w-56");
    // The mark's box and the size the shatter tiles read have to change
    // together, or the tiles slice an image scaled to the wrong box.
    const mark = document.querySelector<HTMLElement>(".glim-logo-mark");
    expect(mark?.className).toContain("h-11 w-11");
    expect(mark?.parentElement?.style.getPropertyValue("--egg-mark")).toBe("44px");
  });

  it("keeps 224px and the 104px mark in the other three modes, reactive included", () => {
    for (const mode of ["text", "textGlyph", "reactive"] as const) {
      cleanup();
      setLabelMode("sidebar", mode);
      renderSidebar();
      expect(rail().className, mode).toContain("w-56");
      expect(rail().className, mode).not.toContain("w-(--rail-narrow)");
      const mark = document.querySelector<HTMLElement>(".glim-logo-mark");
      expect(mark?.className, mode).toContain("h-26 w-26");
      expect(mark?.parentElement?.style.getPropertyValue("--egg-mark"), mode).toBe("104px");
    }
  });
});

describe("glyph mode names the rows in a tooltip bubble", () => {
  it("reveals a nav row's name on hover and on focus, with no native title", () => {
    setLabelMode("sidebar", "glyph");
    renderSidebar();
    const dashboard = screen.getByRole("link", { name: "Dashboard" });
    expect(dashboard.getAttribute("title")).toBeNull();
    expect(document.querySelector(".glim-bubble")).toBeNull();

    fireEvent.mouseEnter(dashboard);
    expect(document.querySelector(".glim-bubble")?.textContent).toBe("Dashboard");
    fireEvent.mouseLeave(dashboard);
    expect(document.querySelector(".glim-bubble")).toBeNull();

    // Keyboard focus shows it too, which a native title never does.
    fireEvent.keyDown(document.body, { key: "Tab" });
    act(() => dashboard.focus());
    expect(document.querySelector(".glim-bubble")?.textContent).toBe("Dashboard");
  });

  it("gives the view toggle the same bubble, naming the current view", () => {
    setLabelMode("sidebar", "glyph");
    renderSidebar();
    // Simple is the default view, so that is the name currently on the row.
    const toggle = screen.getByRole("button", { name: "Simple view" });
    expect(toggle.getAttribute("title")).toBeNull();
    fireEvent.mouseEnter(toggle);
    expect(document.querySelector(".glim-bubble")?.textContent).toBe("Simple view");
  });

  it("drops the wordmark and keeps the mark centred", () => {
    setLabelMode("sidebar", "glyph");
    renderSidebar();
    const logo = screen.getByRole("button", { name: "Dashboard" });
    expect(logo.className).toContain("items-center");
    expect(screen.queryByText("BombVault")).toBeNull();
    // Removed, not `sr-only`: the button's own aria-label is the name, so a
    // hidden copy would only make a screen reader say "Dashboard BombVault".
    expect(logo.querySelector(".sr-only")).toBeNull();
  });

  it("stands the wordmark under the mark in both text modes", () => {
    for (const mode of ["text", "textGlyph"] as const) {
      cleanup();
      setLabelMode("sidebar", mode);
      renderSidebar();
      const logo = screen.getByRole("button", { name: "Dashboard" });
      expect(logo.className, mode).toContain("flex-col");
      expect(logo.lastElementChild?.textContent, mode).toBe("BombVault");
    }
  });

  it("says nothing on hover while the rows still show their own text", () => {
    setLabelMode("sidebar", "textGlyph");
    renderSidebar();
    for (const el of [
      screen.getByRole("link", { name: "Dashboard" }),
      screen.getByRole("button", { name: "Simple view" }),
    ]) {
      expect(el.getAttribute("title")).toBeNull();
      fireEvent.mouseEnter(el);
      expect(document.querySelector(".glim-bubble")).toBeNull();
    }
  });
});
