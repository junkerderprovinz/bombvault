// @vitest-environment jsdom
// Pairing leads the Instances tabs, and each other tab is gated on its own
// setting: no tab for a feature that is off, and a hash naming a disabled tab
// falls back to pairing.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, en } from "../lib/i18n";
import { InstanceProvider } from "../lib/instanceScope";

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
vi.mock("./Pairing", () => ({ Pairing: () => <div>PANEL pairing</div> }));

const { Instances } = await import("./Instances");

async function renderPage() {
  await act(async () => {
    render(
      <MemoryRouter>
        <InstanceProvider>
          <I18nProvider>
            <Instances />
          </I18nProvider>
        </InstanceProvider>
      </MemoryRouter>,
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
  it("leads with pairing and shows one tab per switched-on feature", async () => {
    await renderPage();
    const tabs = screen.getAllByRole("tab").map((el) => el.textContent?.trim());
    expect(tabs[0]).toBe(en["pairing.title"]);
    expect(screen.queryByRole("tab", { name: en["receiver.title"] })).not.toBeNull();
    expect(screen.queryByRole("tab", { name: en["fleet.title"] })).not.toBeNull();
    expect(screen.queryByRole("tab", { name: en["pull.title"] })).not.toBeNull();
    expect(screen.queryByText("PANEL pairing")).not.toBeNull();
    expect(screen.queryByText("PANEL receiver")).toBeNull();
  });

  it("leaves out the tab of a feature that is switched off", async () => {
    flags = { receiverEnabled: true, fleetEnabled: false, pullEnabled: true };
    await renderPage();
    expect(screen.queryByRole("tab", { name: en["fleet.title"] })).toBeNull();
    expect(screen.queryByRole("tab", { name: en["pull.title"] })).not.toBeNull();
  });

  it("keeps pairing beside a single switched-on feature", async () => {
    flags = { receiverEnabled: true, fleetEnabled: false, pullEnabled: false };
    await renderPage();
    const tabs = screen.getAllByRole("tab").map((el) => el.textContent?.trim());
    expect(tabs).toEqual([en["pairing.title"], en["receiver.title"]]);
  });

  it("falls back to pairing when the hash names a disabled tab", async () => {
    // A bookmark saved while pulling was still on.
    window.location.hash = "#pull";
    flags = { receiverEnabled: true, fleetEnabled: false, pullEnabled: false };
    await renderPage();
    expect(screen.queryByText("PANEL pull")).toBeNull();
    expect(screen.queryByText("PANEL pairing")).not.toBeNull();
  });

  it("opens the tab the hash names when that one is on", async () => {
    window.location.hash = "#fleet";
    await renderPage();
    expect(screen.queryByText("PANEL fleet")).not.toBeNull();
    expect(screen.queryByText("PANEL receiver")).toBeNull();
  });

  it("shows nothing but the heading when all three are off", async () => {
    flags = { receiverEnabled: false, fleetEnabled: false, pullEnabled: false };
    await renderPage();
    expect(screen.queryByRole("tablist")).toBeNull();
    expect(screen.queryByText("PANEL receiver")).toBeNull();
    expect(screen.queryByText("PANEL fleet")).toBeNull();
    expect(screen.queryByText("PANEL pull")).toBeNull();
    expect(screen.queryByText("PANEL pairing")).toBeNull();
    expect(screen.queryByText(en["instances.title"])).not.toBeNull();
  });

  it("gives every tab a different name", async () => {
    await renderPage();
    const names = screen.getAllByRole("tab").map((el) => el.textContent?.trim());
    expect(new Set(names).size).toBe(names.length);
  });
});
