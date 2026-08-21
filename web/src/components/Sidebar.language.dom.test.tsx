// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// Sidebar — language-switcher removal (GlimStone follow-up pass, live-review
// point 9). The language picker that used to live in SidebarControls (a
// button + role="listbox" dropdown) moved into its own Card in Settings'
// General tab (see Settings.languageCard.dom.test.tsx, the other half of
// this pair) and was DELETED here, not duplicated — jdp's request was a
// move ("verschieb den Sprachschalter"), so the sidebar must never again
// render a second copy of it. This is a regression guard against exactly
// that: it fails the moment anyone re-adds a listbox/flag picker to the
// sidebar footer, while confirming the two controls that were explicitly
// meant to STAY (theme toggle, Simple/Advanced view toggle) are still there.
// ---------------------------------------------------------------------------
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
  // jsdom doesn't implement matchMedia (throws "not a function") — lib/
  // theme.ts's getResolvedTheme()/onSystemThemeChange() (read by
  // SidebarControls at mount, unrelated to what this file actually tests)
  // both call it, so mounting Sidebar in jsdom at all needs this minimal
  // stub first.
  window.matchMedia = ((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia;
});

afterEach(() => {
  cleanup();
});

describe("Sidebar — language picker moved out (no longer duplicated here)", () => {
  it("renders no listbox (the language dropdown is gone)", () => {
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

  it("still renders the theme toggle button (explicitly meant to stay)", () => {
    renderSidebar();
    expect(screen.getByTitle("Toggle theme")).toBeTruthy();
  });

  it("still renders the Simple/Advanced view toggle (explicitly meant to stay)", () => {
    renderSidebar();
    // Default state is "Simple view" (AdvancedProvider defaults to off).
    expect(screen.getByRole("button", { name: "Simple view" })).toBeTruthy();
  });
});
