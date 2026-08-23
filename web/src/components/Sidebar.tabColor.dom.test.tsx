// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// Sidebar — the tab-strip 3-state colour rule's nav-rail half (GlimStone
// follow-up round, live-review, jdp: "Die Tabs (Einstellungstabs und
// Hauptabs in der Sidebar) sollen nicht ausgewählt farblos sein. Beim
// Mouseover soll das Icon eingefärbt werden und wenn man sie anklickt soll
// der Badge eingefärbt werden und das Icon wieder in schwarz oder weiß
// angezeigt werden.").
//
// index.css's `.bv-nav-idle svg` rule (mode-aware: always coloured in
// Normal/Rainbow, `[data-rainbow="reactive"] .bv-nav-idle:hover svg`/
// `:focus-within svg` for the Reactive-only hover reveal) is what actually
// paints the tint — not exercisable from jsdom (no stylesheet
// is loaded by this test's render, and jsdom does not run a real hover
// pseudo-class/paint pipeline anyway, matching this repo's own established
// "verify the real CSS live in a browser, unit-test the class contract"
// split — see Selector.dom.test.tsx's own "hue opt-out" describe block for
// the identical pattern one file over). What IS verifiable here, and what
// would actually break if the marker regressed, is the CLASS CONTRACT the
// CSS rule depends on: every not-yet-selected nav destination (and the
// Simple/Advanced toggle, which shares navBase/navInactive and never has an
// active state of its own) carries `bv-nav-idle`; the active destination
// does not (it swaps to navActive's `bg-accent text-accentContrast` instead,
// already covered by this file's own existing regression guard elsewhere).
// ---------------------------------------------------------------------------
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { Sidebar } from "./Sidebar";
import { I18nProvider } from "../lib/i18n";
import { AdvancedProvider } from "../lib/advanced";

function renderSidebar(initialEntries: string[] = ["/"]) {
  return render(
    <MemoryRouter initialEntries={initialEntries}>
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

describe("Sidebar — bv-nav-idle marker (hover/focus icon-tint hook)", () => {
  it("every nav destination carries bv-nav-idle when nothing matches the current route", () => {
    // "/" matches none of the NavLink `to` targets (all are "/dashboard",
    // "/recovery", ...), so every rendered link is in its inactive state.
    renderSidebar(["/"]);
    const dashboard = screen.getByRole("link", { name: "Dashboard" });
    const recovery = screen.getByRole("link", { name: "Recovery" });
    const settings = screen.getByRole("link", { name: "Settings" });
    expect(dashboard.className).toContain("bv-nav-idle");
    expect(recovery.className).toContain("bv-nav-idle");
    expect(settings.className).toContain("bv-nav-idle");
  });

  it("the currently-active destination does NOT carry bv-nav-idle, and fills the badge with the accent instead", () => {
    renderSidebar(["/dashboard"]);
    const dashboard = screen.getByRole("link", { name: "Dashboard" });
    expect(dashboard.className).not.toContain("bv-nav-idle");
    expect(dashboard.className).toContain("bg-accent");
    expect(dashboard.className).toContain("text-accentContrast");

    // A sibling destination that is NOT the active route still idles normally.
    const recovery = screen.getByRole("link", { name: "Recovery" });
    expect(recovery.className).toContain("bv-nav-idle");
    expect(recovery.className).not.toContain("bg-accent");
  });

  it("the Simple/Advanced view toggle (never has an active state of its own) also carries bv-nav-idle", () => {
    renderSidebar(["/"]);
    const toggle = screen.getByRole("button", { name: "Simple view" });
    expect(toggle.className).toContain("bv-nav-idle");
  });
});
