// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// Sidebar — the tab-strip 3-state colour rule's nav-rail half (GlimStone
// follow-up round, live-review, jdp: "Die Tabs (Einstellungstabs und
// Hauptabs in der Sidebar) sollen nicht ausgewählt farblos sein. Beim
// Mouseover soll das Icon eingefärbt werden und wenn man sie anklickt soll
// der Badge eingefärbt werden und das Icon wieder in schwarz oder weiß
// angezeigt werden.").
//
// REWRITTEN (GlimStone follow-up round, rainbow REVERSAL — jdp: "Die ganzen
// Tabs in der Sidebar sind wieder nicht im Regenbogenmodus bzw in der
// Farbengine."): this file used to guard the `bv-nav-idle` marker on EVERY
// NavItem (the flat-accent-only mechanism the nav rail used before this
// round). NavItem no longer carries that marker at all — it now carries
// `.glim-hue`/`.glim-hue-icon` (plus `.glim-active` once selected), the
// EXACT classes any other hue-enabled Selector segment carries — so this
// file's class-contract assertions are updated to match. `bv-nav-idle`
// itself is NOT gone from the app: SidebarControls' own Simple/Advanced
// toggle (a genuine set-of-one, never hued) still carries it, and this
// file's own last `describe` block still guards that.
//
// index.css's generic `.glim-hue-icon` rule (mode-aware: always coloured in
// Normal/Rainbow, `[data-rainbow="reactive"] .glim-hue-icon:hover svg`/
// `:focus-within svg` for the Reactive-only hover reveal) is what actually
// paints the tint — not exercisable from jsdom (no stylesheet is loaded by
// this test's render, and jsdom does not run a real hover pseudo-class/paint
// pipeline anyway, matching this repo's own established "verify the real CSS
// live in a browser, unit-test the class contract" split — see
// Selector.dom.test.tsx's own "hue opt-out" describe block for the identical
// pattern one file over). What IS verifiable here, and what would actually
// break if the marker regressed, is the CLASS CONTRACT the CSS rule depends
// on, and the render-order `hueIndex` assignment `nextHue()` produces.
// ---------------------------------------------------------------------------
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { Sidebar } from "./Sidebar";
import { I18nProvider } from "../lib/i18n";
import { AdvancedProvider } from "../lib/advanced";
import type { Settings } from "../lib/api";

function renderSidebar(initialEntries: string[] = ["/"], settings: Settings | null = null) {
  return render(
    <MemoryRouter initialEntries={initialEntries}>
      <I18nProvider>
        <AdvancedProvider>
          <Sidebar settings={settings} />
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

describe("Sidebar — NavItem carries a rainbow hue position (glim-hue/glim-hue-icon)", () => {
  it("every nav destination carries glim-hue and glim-hue-icon when nothing matches the current route", () => {
    // "/" matches none of the NavLink `to` targets (all are "/dashboard",
    // "/recovery", ...), so every rendered link is in its inactive state.
    renderSidebar(["/"]);
    const dashboard = screen.getByRole("link", { name: "Dashboard" });
    const recovery = screen.getByRole("link", { name: "Recovery" });
    const settings = screen.getByRole("link", { name: "Settings" });
    for (const link of [dashboard, recovery, settings]) {
      expect(link.className).toContain("glim-hue");
      expect(link.className).toContain("glim-hue-icon");
      // Idle items must NOT carry the old flat-accent marker any more.
      expect(link.className).not.toContain("bv-nav-idle");
    }
  });

  it("each idle nav item carries its OWN --item-hue inline custom property, distinct by position", () => {
    renderSidebar(["/"]);
    const dashboard = screen.getByRole("link", { name: "Dashboard" });
    const recovery = screen.getByRole("link", { name: "Recovery" });
    const containers = screen.getByRole("link", { name: "Containers" });
    const dashHue = dashboard.style.getPropertyValue("--item-hue");
    const recoveryHue = recovery.style.getPropertyValue("--item-hue");
    const containersHue = containers.style.getPropertyValue("--item-hue");
    expect(dashHue).toMatch(/^#[0-9a-fA-F]{6}$/);
    expect(recoveryHue).toMatch(/^#[0-9a-fA-F]{6}$/);
    expect(containersHue).toMatch(/^#[0-9a-fA-F]{6}$/);
    // Three consecutive positions in an 8-colour palette are pairwise distinct.
    expect(new Set([dashHue, recoveryHue, containersHue]).size).toBe(3);
  });

  it("the currently-active destination carries glim-active and fills the badge with the accent, still keeping its own --item-hue", () => {
    renderSidebar(["/dashboard"]);
    const dashboard = screen.getByRole("link", { name: "Dashboard" });
    expect(dashboard.className).toContain("glim-active");
    expect(dashboard.className).toContain("bg-accent");
    expect(dashboard.className).toContain("text-accentContrast");
    expect(dashboard.style.getPropertyValue("--item-hue")).toMatch(/^#[0-9a-fA-F]{6}$/);

    // A sibling destination that is NOT the active route still idles normally.
    const recovery = screen.getByRole("link", { name: "Recovery" });
    expect(recovery.className).not.toContain("glim-active");
    expect(recovery.className).not.toContain("bg-accent");
    expect(recovery.className).toContain("glim-hue-icon");
  });

  it("hiding a conditional domain tab (VMs) does not disturb the hue positions of the tabs around it", () => {
    // `as unknown as Settings` matches this repo's own established stub
    // convention for a partial Settings fixture (see PathModeSwitch.dom.test.tsx's
    // STUB_SETTINGS) — only the three domain flags this test actually exercises
    // are set; every other field is unused by Sidebar itself.
    const allOn = {
      vmsEnabled: true,
      flashEnabled: true,
      filesEnabled: true,
    } as unknown as Settings;
    const { unmount } = renderSidebar(["/"], allOn);
    const dashboardHueOn = screen.getByRole("link", { name: "Dashboard" }).style.getPropertyValue("--item-hue");
    const recoveryHueOn = screen.getByRole("link", { name: "Recovery" }).style.getPropertyValue("--item-hue");
    const containersHueOn = screen.getByRole("link", { name: "Containers" }).style.getPropertyValue("--item-hue");
    const flashHueOn = screen.getByRole("link", { name: "Flash" }).style.getPropertyValue("--item-hue");
    unmount();

    // Turning VMs off removes exactly one NavItem BEFORE Flash in the list —
    // Dashboard/Recovery/Containers (all ahead of the conditional block) must
    // keep their exact same hue; Flash (which comes after the now-hidden VMs
    // slot) shifts to the position VMs used to hold.
    const vmsOff = { ...allOn, vmsEnabled: false } as unknown as Settings;
    renderSidebar(["/"], vmsOff);
    expect(screen.getByRole("link", { name: "Dashboard" }).style.getPropertyValue("--item-hue")).toBe(dashboardHueOn);
    expect(screen.getByRole("link", { name: "Recovery" }).style.getPropertyValue("--item-hue")).toBe(recoveryHueOn);
    expect(screen.getByRole("link", { name: "Containers" }).style.getPropertyValue("--item-hue")).toBe(containersHueOn);
    expect(screen.queryByRole("link", { name: "VMs" })).toBeNull();
    // Flash moved up one slot, so it now wears the hue VMs used to have —
    // never the same colour it had while VMs was still visible.
    expect(screen.getByRole("link", { name: "Flash" }).style.getPropertyValue("--item-hue")).not.toBe(flashHueOn);
  });
});

describe("Sidebar — bv-nav-idle marker (SidebarControls' own flat-accent set-of-one)", () => {
  it("the Simple/Advanced view toggle (never has an active state, never hued) carries bv-nav-idle, not glim-hue", () => {
    renderSidebar(["/"]);
    const toggle = screen.getByRole("button", { name: "Simple view" });
    expect(toggle.className).toContain("bv-nav-idle");
    expect(toggle.className).not.toContain("glim-hue");
  });
});
