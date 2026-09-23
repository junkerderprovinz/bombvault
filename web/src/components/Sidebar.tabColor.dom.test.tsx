// @vitest-environment jsdom
// The rail's hue classes and positions. The tint itself comes from index.css,
// which jsdom does not apply, so it is checked in a browser.
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

describe("Sidebar nav rows carry a rainbow hue position", () => {
  it("every idle nav destination carries glim-hue and glim-hue-icon", () => {
    // "/" matches no NavLink target, so every link is idle.
    renderSidebar(["/"]);
    const dashboard = screen.getByRole("link", { name: "Dashboard" });
    const recovery = screen.getByRole("link", { name: "Recovery" });
    const settings = screen.getByRole("link", { name: "Settings" });
    for (const link of [dashboard, recovery, settings]) {
      expect(link.className).toContain("glim-hue");
      expect(link.className).toContain("glim-hue-icon");
      // glim-nav-idle belongs to the footer rows only.
      expect(link.className).not.toContain("glim-nav-idle");
    }
  });

  it("each idle nav item carries its own --item-hue, distinct by position", () => {
    renderSidebar(["/"]);
    const dashboard = screen.getByRole("link", { name: "Dashboard" });
    const recovery = screen.getByRole("link", { name: "Recovery" });
    const containers = screen.getByRole("link", { name: "Containers" });
    const dashHue = dashboard.style.getPropertyValue("--item-hue");
    const recoveryHue = recovery.style.getPropertyValue("--item-hue");
    const containersHue = containers.style.getPropertyValue("--item-hue");
    expect(dashHue).toMatch(/^var\(--rb-[0-7]\)$/);
    expect(recoveryHue).toMatch(/^var\(--rb-[0-7]\)$/);
    expect(containersHue).toMatch(/^var\(--rb-[0-7]\)$/);
    // Three consecutive positions in an 8-colour palette are pairwise distinct.
    expect(new Set([dashHue, recoveryHue, containersHue]).size).toBe(3);
  });

  it("the active destination carries glim-active and the accent fill, and keeps its --item-hue", () => {
    renderSidebar(["/dashboard"]);
    const dashboard = screen.getByRole("link", { name: "Dashboard" });
    expect(dashboard.className).toContain("glim-active");
    expect(dashboard.className).toContain("bg-accent");
    expect(dashboard.className).toContain("text-accentContrast");
    expect(dashboard.style.getPropertyValue("--item-hue")).toMatch(/^var\(--rb-[0-7]\)$/);

    const recovery = screen.getByRole("link", { name: "Recovery" });
    expect(recovery.className).not.toContain("glim-active");
    expect(recovery.className).not.toContain("bg-accent");
    expect(recovery.className).toContain("glim-hue-icon");
  });

  it("hiding VMs keeps the hues of the tabs before it and moves the ones after", () => {
    // Only the domain flags this test needs; Sidebar reads no other field.
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

    // Dashboard, Recovery and Containers come before VMs and keep their hue.
    // Flash comes after it and moves up into the slot VMs held.
    const vmsOff = { ...allOn, vmsEnabled: false } as unknown as Settings;
    renderSidebar(["/"], vmsOff);
    expect(screen.getByRole("link", { name: "Dashboard" }).style.getPropertyValue("--item-hue")).toBe(dashboardHueOn);
    expect(screen.getByRole("link", { name: "Recovery" }).style.getPropertyValue("--item-hue")).toBe(recoveryHueOn);
    expect(screen.getByRole("link", { name: "Containers" }).style.getPropertyValue("--item-hue")).toBe(containersHueOn);
    expect(screen.queryByRole("link", { name: "VMs" })).toBeNull();
    expect(screen.getByRole("link", { name: "Flash" }).style.getPropertyValue("--item-hue")).not.toBe(flashHueOn);
  });
});

// glim-nav-idle decides when the colour appears, glim-hue which colour the row
// owns, so the footer rows carry both.
describe("Sidebar footer rows carry a hue too", () => {
  it("the view toggle carries glim-hue as well as glim-nav-idle, and a real hue", () => {
    renderSidebar(["/"]);
    const toggle = screen.getByRole("button", { name: "Simple view" });
    expect(toggle.className).toContain("glim-nav-idle");
    expect(toggle.className).toContain("glim-hue");
    // The class alone is not enough: without --item-hue the accent resolves
    // to nothing.
    expect(toggle.style.getPropertyValue("--item-hue")).toMatch(/^var\(--rb-[0-7]\)$/);
  });

  it("its hue continues the rail's own sequence rather than restarting", () => {
    renderSidebar(["/"]);
    const settings = screen.getByRole("link", { name: "Settings" });
    const toggle = screen.getByRole("button", { name: "Simple view" });
    // Settings follows the toggle directly; equal hues would mean one of them
    // started a counter of its own.
    expect(toggle.style.getPropertyValue("--item-hue")).not.toBe(
      settings.style.getPropertyValue("--item-hue")
    );
  });
});
