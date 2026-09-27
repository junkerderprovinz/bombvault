// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { Link, MemoryRouter, Route, Routes, useNavigate } from "react-router-dom";

vi.mock("../lib/api", () => ({
  getAuth: async () => ({ ok: true, enabled: false, authed: true }),
  getSettings: async () => ({ ok: true, settings: null }),
  getHealth: async () => ({ ok: true, version: "dev" }),
  getAnomalySummary: async () => ({ ok: false }),
  getAnomalies: async () => ({ ok: true, anomalies: [], nextCursor: "" }),
}));
vi.mock("../lib/displayPrefs", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/displayPrefs")>()),
  sync: async () => {},
}));
vi.mock("../components/Sidebar", () => ({
  Sidebar: () => (
    <nav aria-label="rail">
      <Link to="/files">Folders</Link>
    </nav>
  ),
}));
vi.mock("../components/WhatsNewDialog", () => ({ WhatsNewDialog: () => null }));

import { Layout } from "./Layout";

afterEach(cleanup);

function Folders() {
  const navigate = useNavigate();
  return (
    <>
      <p>folders</p>
      <Link to="/dashboard">to the dashboard</Link>
      <button type="button" onClick={() => navigate(-1)}>
        back
      </button>
    </>
  );
}

function renderLayout() {
  render(
    <MemoryRouter initialEntries={["/dashboard"]}>
      <Routes>
        <Route element={<Layout />}>
          <Route path="/dashboard" element={<p>dashboard</p>} />
          <Route path="/files" element={<Folders />} />
        </Route>
      </Routes>
    </MemoryRouter>
  );
}

// jsdom applies no media queries, so these read the classes that hide the rail
// below the sm breakpoint and show the button that opens it there.
describe("the rail on a narrow window", () => {
  it("hides behind a menu button that opens it over the page", async () => {
    renderLayout();
    const menu = await screen.findByRole("button", { name: "Menu" });
    expect((menu.parentElement as HTMLElement).className).toContain("sm:hidden");
    expect(menu.getAttribute("aria-expanded")).toBe("false");
    const rail = screen.getByRole("navigation", { name: "rail" }).parentElement as HTMLElement;
    expect(rail.className).toContain("max-sm:hidden");

    fireEvent.click(menu);
    expect(menu.getAttribute("aria-expanded")).toBe("true");
    expect(rail.className).not.toContain("max-sm:hidden");
    expect(rail.className).toContain("max-sm:fixed");
  });

  it("closes again once a page is chosen", async () => {
    renderLayout();
    fireEvent.click(await screen.findByRole("button", { name: "Menu" }));
    fireEvent.click(screen.getByRole("link", { name: "Folders" }));
    expect(await screen.findByText("folders")).toBeTruthy();
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Menu" }).getAttribute("aria-expanded")).toBe("false")
    );
  });

  it.each(["to the dashboard", "back"])("stays closed on the page it was opened on after %s", async (way) => {
    renderLayout();
    fireEvent.click(await screen.findByRole("button", { name: "Menu" }));
    fireEvent.click(screen.getByRole("link", { name: "Folders" }));
    fireEvent.click(await screen.findByText(way));
    expect(await screen.findByText("dashboard")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Menu" }).getAttribute("aria-expanded")).toBe("false");
    expect(screen.queryByTestId("rail-backdrop")).toBeNull();
  });

  it("closes on a click beside it", async () => {
    renderLayout();
    fireEvent.click(await screen.findByRole("button", { name: "Menu" }));
    fireEvent.click(screen.getByTestId("rail-backdrop"));
    expect(screen.getByRole("button", { name: "Menu" }).getAttribute("aria-expanded")).toBe("false");
  });
});
