// @vitest-environment jsdom
// The language picker and the theme toggle live in Settings' General tab; the
// sidebar footer keeps only the view toggle.
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { Sidebar } from "./Sidebar";
import { I18nProvider } from "../lib/i18n";
import { AdvancedProvider } from "../lib/advanced";

function renderSidebar() {
  return render(
    <MemoryRouter>
      <I18nProvider>
        <AdvancedProvider>
          <Sidebar settings={null} />
        </AdvancedProvider>
      </I18nProvider>
    </MemoryRouter>
  );
}

beforeEach(() => {
  localStorage.removeItem("bv-lang");
  localStorage.removeItem("bombvault.advanced");
});

afterEach(() => {
  cleanup();
});

describe("Sidebar footer without language picker and theme toggle", () => {
  it("renders no listbox", () => {
    renderSidebar();
    expect(screen.queryByRole("listbox")).toBeNull();
  });

  it("renders no listbox-opening trigger (aria-haspopup=\"listbox\")", () => {
    renderSidebar();
    const triggers = document.querySelectorAll('[aria-haspopup="listbox"]');
    expect(triggers.length).toBe(0);
  });

  it("renders no flag glyph (fi-* class) in the footer", () => {
    renderSidebar();
    expect(document.querySelector('[class*="fi-"]')).toBeNull();
  });

  it("renders no theme toggle button", () => {
    renderSidebar();
    expect(screen.queryByTitle("Toggle theme")).toBeNull();
  });

  it("renders the Simple/Advanced view toggle", () => {
    renderSidebar();
    // AdvancedProvider starts in the simple view.
    expect(screen.getByRole("button", { name: "Simple view" })).toBeTruthy();
  });
});
