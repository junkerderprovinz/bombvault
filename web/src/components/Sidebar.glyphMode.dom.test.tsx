// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// Sidebar — what the rail does once the "sidebar" axis is set to glyphs, which
// is where jdp's two standing decisions for this round land:
//
//   1. THE RAIL KEEPS ITS WIDTH and the glyphs centre in it ("Glyphen
//      zentriert in voller Breite lassen"). The narrow-rail alternative was
//      measured live and needs its own smaller logo — a separate job. What
//      this file guards is the half that IS decided: every row that has lost
//      its text centres, and no row is left hard-left in a 224px column.
//
//   2. A ROW WITH NO VISIBLE TEXT EXPLAINS ITSELF IN THE REAL BUBBLE, never
//      in a native `title=` balloon (design-language's anti-pattern, the one
//      lint-rules/icon-badge-needs-tooltip.js fails the build over). Eleven
//      unnamed pictures would otherwise be the whole rail.
//
// Both are class/DOM contracts, which is what jsdom can actually hold: the
// real centring is CSS, verified live in a browser, exactly the split
// Sidebar.tabColor.dom.test.tsx's own header describes.
// ---------------------------------------------------------------------------
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
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

/** Every row of the rail: the nav destinations, Settings, and the
 *  Simple/Advanced toggle — which is the one that has historically been
 *  forgotten (it was the single row not following this axis at all). */
function railRows(): HTMLElement[] {
  return [
    ...document.querySelectorAll<HTMLElement>("aside .bv-nav-row"),
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
    // Dashboard, Recovery, Containers, the view toggle, Settings — the rail
    // with no optional domain enabled. If this ever reads 0 the selector is
    // wrong and the assertion below would pass vacuously.
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

describe("glyph mode names the rows in a real bubble", () => {
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

    // Keyboard reachable, which the native balloon never was.
    fireEvent.focus(dashboard);
    expect(document.querySelector(".glim-bubble")?.textContent).toBe("Dashboard");
  });

  it("gives the view toggle the same bubble, carrying the view it will show", () => {
    setLabelMode("sidebar", "glyph");
    renderSidebar();
    // Simple is the default view, so that is the name currently on the row.
    const toggle = screen.getByRole("button", { name: "Simple view" });
    expect(toggle.getAttribute("title")).toBeNull();
    fireEvent.mouseEnter(toggle);
    expect(document.querySelector(".glim-bubble")?.textContent).toBe("Simple view");
  });

  it("drops the wordmark and centres the mark, so the header sits on the glyph column's axis", () => {
    setLabelMode("sidebar", "glyph");
    renderSidebar();
    const logo = screen.getByRole("button", { name: "Dashboard" });
    expect(logo.className).toContain("justify-center");
    expect(screen.queryByText("BombVault")).toBeNull();
    // Removed, not `sr-only`: the button's own aria-label is the name, so a
    // hidden copy would only make a screen reader say "Dashboard BombVault".
    expect(logo.querySelector(".sr-only")).toBeNull();
  });

  it("keeps the wordmark in both text modes", () => {
    for (const mode of ["text", "textGlyph"] as const) {
      cleanup();
      setLabelMode("sidebar", mode);
      renderSidebar();
      expect(screen.getByText("BombVault")).toBeTruthy();
      expect(screen.getByRole("button", { name: "Dashboard" }).className).not.toContain(
        "justify-center",
      );
    }
  });

  it("says nothing on hover while the rows still show their own text", () => {
    setLabelMode("sidebar", "textGlyph");
    renderSidebar();
    // Including the view toggle, which carried its name in a native `title`
    // in EVERY mode before this round — the balloon on a row whose words are
    // printed right next to it.
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
