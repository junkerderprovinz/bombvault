// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// Sidebar — language-switcher AND theme-toggle removal (GlimStone follow-up
// pass, live-review points 9 and a later round). Both the language picker (a
// button + role="listbox" dropdown) and the dark/light theme toggle that
// used to live in SidebarControls moved into their own Cards in Settings'
// General tab (see Settings.languageCard.dom.test.tsx and
// Settings.themeCard.dom.test.tsx, the other half of each pair) and were
// DELETED here, not duplicated — jdp's request was a move each time
// ("verschieb den Sprachschalter", then the same ask for the theme toggle),
// so the sidebar must never again render a second copy of either. This is a
// regression guard against exactly that: it fails the moment anyone re-adds
// a listbox/flag picker or a theme button to the sidebar footer, while
// confirming the one control that was explicitly meant to STAY (the
// Simple/Advanced view toggle) is still there.
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
});

afterEach(() => {
  cleanup();
});

describe("Sidebar — language picker and theme toggle both moved out (no longer duplicated here)", () => {
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

  it("renders no theme toggle button (moved into Settings' ThemeCard, not duplicated here)", () => {
    renderSidebar();
    expect(screen.queryByTitle("Toggle theme")).toBeNull();
  });

  it("still renders the Simple/Advanced view toggle (explicitly meant to stay)", () => {
    renderSidebar();
    // Default state is "Simple view" (AdvancedProvider defaults to off).
    expect(screen.getByRole("button", { name: "Simple view" })).toBeTruthy();
  });
});
