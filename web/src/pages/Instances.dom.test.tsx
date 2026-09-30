// @vitest-environment jsdom
// The Instances tabs, each gated on its own setting: no tab for a feature
// that is off, and a hash naming a disabled tab falls back to the first one
// that is on. Pairing lives in Settings, and a link to its old tab here
// follows it there.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { I18nProvider, en } from "../lib/i18n";

type Flags = { receiverEnabled: boolean; fleetEnabled: boolean; pullEnabled: boolean };

let flags: Flags = { receiverEnabled: true, fleetEnabled: true, pullEnabled: true };

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getSettings: () => Promise.resolve({ ok: true, settings: flags }),
  };
});

// Markers instead of the real pages, which would bring their own API calls.
vi.mock("./Receiver", () => ({ Receiver: () => <div>PANEL receiver</div> }));
vi.mock("./Fleet", () => ({ Fleet: () => <div>PANEL fleet</div> }));
vi.mock("./Pull", () => ({ Pull: () => <div>PANEL pull</div> }));

const { Instances } = await import("./Instances");

function Where() {
  const loc = useLocation();
  return <div data-testid="where">{loc.pathname + loc.hash}</div>;
}

async function renderPage() {
  await act(async () => {
    render(
      <I18nProvider>
        <MemoryRouter initialEntries={[`/instances${window.location.hash}`]}>
          <Routes>
            <Route path="/instances" element={<Instances />} />
            <Route path="/settings" element={<Where />} />
          </Routes>
        </MemoryRouter>
      </I18nProvider>,
    );
  });
}

beforeEach(() => {
  flags = { receiverEnabled: true, fleetEnabled: true, pullEnabled: true };
  window.location.hash = "";
  localStorage.clear();
});

afterEach(cleanup);

describe("instances tab strip", () => {
  it("leads with the instances of the group and shows one tab per switched-on feature", async () => {
    await renderPage();
    const tabs = screen.getAllByRole("tab").map((el) => el.textContent?.trim());
    expect(tabs).toEqual([en["instances.title"], en["receiver.title"], en["pull.title"]]);
    expect(screen.queryByText("PANEL fleet")).not.toBeNull();
    expect(screen.queryByText("PANEL receiver")).toBeNull();
  });

  it("leaves out the tab of a feature that is switched off", async () => {
    flags = { receiverEnabled: true, fleetEnabled: false, pullEnabled: true };
    await renderPage();
    expect(screen.queryByRole("tab", { name: en["instances.title"] })).toBeNull();
    expect(screen.queryByRole("tab", { name: en["pull.title"] })).not.toBeNull();
    expect(screen.queryByText("PANEL receiver")).not.toBeNull();
  });

  it("shows a single switched-on feature without a tab strip", async () => {
    flags = { receiverEnabled: true, fleetEnabled: false, pullEnabled: false };
    await renderPage();
    expect(screen.queryByRole("tablist")).toBeNull();
    expect(screen.queryByText("PANEL receiver")).not.toBeNull();
  });

  it("falls back to the first tab that is on when the hash names a disabled one", async () => {
    // A bookmark saved while pulling was still on.
    window.location.hash = "#pull";
    flags = { receiverEnabled: true, fleetEnabled: true, pullEnabled: false };
    await renderPage();
    expect(screen.queryByText("PANEL pull")).toBeNull();
    expect(screen.queryByText("PANEL fleet")).not.toBeNull();
  });

  it("opens the tab the hash names when that one is on", async () => {
    window.location.hash = "#receiver";
    await renderPage();
    expect(screen.queryByText("PANEL receiver")).not.toBeNull();
    expect(screen.queryByText("PANEL fleet")).toBeNull();
  });

  it("sends a link to the old pairing tab on to Settings", async () => {
    window.location.hash = "#pairing";
    await renderPage();
    expect(screen.getByTestId("where").textContent).toBe("/settings#pairing");
  });

  it("shows nothing but the heading when all three are off", async () => {
    flags = { receiverEnabled: false, fleetEnabled: false, pullEnabled: false };
    await renderPage();
    expect(screen.queryByRole("tablist")).toBeNull();
    expect(screen.queryByText("PANEL receiver")).toBeNull();
    expect(screen.queryByText("PANEL fleet")).toBeNull();
    expect(screen.queryByText("PANEL pull")).toBeNull();
    expect(screen.queryByText(en["instances.title"])).not.toBeNull();
  });

  it("gives every tab a different name", async () => {
    await renderPage();
    const names = screen.getAllByRole("tab").map((el) => el.textContent?.trim());
    expect(new Set(names).size).toBe(names.length);
  });
});
