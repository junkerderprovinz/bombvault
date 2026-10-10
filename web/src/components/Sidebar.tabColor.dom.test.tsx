// @vitest-environment jsdom
// The rail's hue classes and positions. The tint itself comes from index.css,
// which jsdom does not apply, so it is checked in a browser.
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { Sidebar } from "./Sidebar";
import { I18nProvider } from "../lib/i18n";
import type { Settings } from "../lib/api";

const PINS_KEY = "bombvault.navPins";

function renderSidebar(initialEntries: string[] = ["/"], settings: Settings | null = null, authEnabled = false) {
  return render(
    <MemoryRouter initialEntries={initialEntries}>
      <I18nProvider>
        <Sidebar settings={settings} authEnabled={authEnabled} />
      </I18nProvider>
    </MemoryRouter>
  );
}

beforeEach(() => {
  localStorage.clear();
});

afterEach(() => {
  cleanup();
});

describe("Sidebar nav rows carry a rainbow hue position", () => {
  it("every idle nav destination carries glim-hue and glim-hue-icon", () => {
    // "/" matches no NavLink target, so every link is idle.
    renderSidebar(["/"]);
    const overview = screen.getByRole("link", { name: "Overview" });
    const restore = screen.getByRole("link", { name: "Restore" });
    const settings = screen.getByRole("link", { name: "Settings" });
    for (const link of [overview, restore, settings]) {
      expect(link.className).toContain("glim-hue");
      expect(link.className).toContain("glim-hue-icon");
      // glim-nav-idle belongs to the footer rows only.
      expect(link.className).not.toContain("glim-nav-idle");
    }
  });

  it("each idle nav item carries its own --item-hue, distinct by position", () => {
    renderSidebar(["/"]);
    const hues = ["Overview", "Backups", "Restore"].map((name) =>
      screen.getByRole("link", { name }).style.getPropertyValue("--item-hue")
    );
    for (const hue of hues) expect(hue).toMatch(/^var\(--rb-[0-7]\)$/);
    // Three consecutive positions in an 8-colour palette are pairwise distinct.
    expect(new Set(hues).size).toBe(3);
  });

  it("the active destination carries glim-active and the accent fill, and keeps its --item-hue", () => {
    renderSidebar(["/dashboard"]);
    const overview = screen.getByRole("link", { name: "Overview" });
    expect(overview.className).toContain("glim-active");
    expect(overview.className).toContain("bg-accent");
    expect(overview.className).toContain("text-accentContrast");
    expect(overview.style.getPropertyValue("--item-hue")).toMatch(/^var\(--rb-[0-7]\)$/);

    const restore = screen.getByRole("link", { name: "Restore" });
    expect(restore.className).not.toContain("glim-active");
    expect(restore.className).not.toContain("bg-accent");
    expect(restore.className).toContain("glim-hue-icon");
  });

  it("keeps an entry current on the pages below it", () => {
    renderSidebar(["/storage/local"]);
    expect(screen.getByRole("link", { name: "Storage locations" }).className).toContain("glim-active");
    expect(screen.getByRole("link", { name: "Settings" }).className).not.toContain("glim-active");
  });

  it("shows Instances only while pinned, with a hue of its own", () => {
    const { unmount } = renderSidebar(["/"]);
    expect(screen.queryByRole("link", { name: "Instances" })).toBeNull();
    unmount();

    localStorage.setItem(PINS_KEY, JSON.stringify({ instances: true }));
    renderSidebar(["/"]);
    const instances = screen.getByRole("link", { name: "Instances" });
    expect(instances.getAttribute("href")).toBe("/instances");
    expect(instances.style.getPropertyValue("--item-hue")).toMatch(/^var\(--rb-[0-7]\)$/);
    expect(instances.style.getPropertyValue("--item-hue")).not.toBe(
      screen.getByRole("link", { name: "Storage locations" }).style.getPropertyValue("--item-hue")
    );
  });

  it("hiding Anomalies keeps the hue of the row before it and moves the ones after", () => {
    const hueOf = (name: string) => screen.getByRole("link", { name }).style.getPropertyValue("--item-hue");
    const { unmount } = renderSidebar(["/"], { anomalyEnabled: true } as unknown as Settings);
    const shown = { overview: hueOf("Overview"), backups: hueOf("Backups"), restore: hueOf("Restore") };
    unmount();

    renderSidebar(["/"], { anomalyEnabled: false } as unknown as Settings);
    expect(screen.queryByRole("link", { name: "Anomalies" })).toBeNull();
    expect(hueOf("Overview")).toBe(shown.overview);
    expect(hueOf("Backups")).not.toBe(shown.backups);
    expect(hueOf("Restore")).not.toBe(shown.restore);
  });

  it("unpinning the storage locations moves Settings up one position", () => {
    const hueOf = (name: string) => screen.getByRole("link", { name }).style.getPropertyValue("--item-hue");
    const { unmount } = renderSidebar(["/"]);
    const pinned = { restore: hueOf("Restore"), settings: hueOf("Settings") };
    unmount();

    localStorage.setItem(PINS_KEY, JSON.stringify({ storage: false }));
    renderSidebar(["/"]);
    expect(screen.queryByRole("link", { name: "Storage locations" })).toBeNull();
    expect(hueOf("Restore")).toBe(pinned.restore);
    expect(hueOf("Settings")).not.toBe(pinned.settings);
  });
});

// glim-nav-idle decides when the colour appears, glim-hue which colour the row
// owns, so the sign-out row carries both.
describe("Sidebar footer rows carry a hue too", () => {
  it("sign-out carries glim-hue as well as glim-nav-idle, and a real hue", () => {
    renderSidebar(["/"], null, true);
    const signOut = screen.getByRole("button", { name: "Sign out" });
    expect(signOut.className).toContain("glim-nav-idle");
    expect(signOut.className).toContain("glim-hue");
    // The class alone is not enough: without --item-hue the accent resolves
    // to nothing.
    expect(signOut.style.getPropertyValue("--item-hue")).toMatch(/^var\(--rb-[0-7]\)$/);
  });

  it("its hue continues the rail's own sequence rather than restarting", () => {
    renderSidebar(["/"], null, true);
    const settings = screen.getByRole("link", { name: "Settings" });
    const signOut = screen.getByRole("button", { name: "Sign out" });
    // Settings follows sign-out directly; equal hues would mean one of them
    // started a counter of its own.
    expect(signOut.style.getPropertyValue("--item-hue")).not.toBe(
      settings.style.getPropertyValue("--item-hue")
    );
  });
});
