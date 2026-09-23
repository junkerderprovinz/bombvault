// @vitest-environment jsdom
// Each Instances tab is gated on its own setting: no tab for a feature that is
// off, no strip for a single choice, and a hash naming a disabled tab falls
// back to the first enabled one.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
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

async function renderPage() {
  await act(async () => {
    render(
      <I18nProvider>
        <Instances />
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
  it("shows one tab per switched-on feature, and the first one's panel", async () => {
    await renderPage();
    expect(screen.queryByRole("tab", { name: en["receiver.title"] })).not.toBeNull();
    expect(screen.queryByRole("tab", { name: en["fleet.title"] })).not.toBeNull();
    expect(screen.queryByRole("tab", { name: en["pull.title"] })).not.toBeNull();
    expect(screen.queryByText("PANEL receiver")).not.toBeNull();
    expect(screen.queryByText("PANEL fleet")).toBeNull();
  });

  it("leaves out the tab of a feature that is switched off", async () => {
    flags = { receiverEnabled: true, fleetEnabled: false, pullEnabled: true };
    await renderPage();
    expect(screen.queryByRole("tab", { name: en["fleet.title"] })).toBeNull();
    expect(screen.queryByRole("tab", { name: en["pull.title"] })).not.toBeNull();
  });

  it("renders no strip at all when only one feature is on", async () => {
    flags = { receiverEnabled: true, fleetEnabled: false, pullEnabled: false };
    await renderPage();
    expect(screen.queryByRole("tablist")).toBeNull();
    expect(screen.queryByText("PANEL receiver")).not.toBeNull();
  });

  it("falls back to an enabled tab when the hash names a disabled one", async () => {
    // A bookmark saved while pulling was still on.
    window.location.hash = "#pull";
    flags = { receiverEnabled: true, fleetEnabled: false, pullEnabled: false };
    await renderPage();
    expect(screen.queryByText("PANEL pull")).toBeNull();
    expect(screen.queryByText("PANEL receiver")).not.toBeNull();
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
    expect(screen.queryByText(en["instances.title"])).not.toBeNull();
  });

  it("gives the three tabs three different names", async () => {
    await renderPage();
    const names = screen.getAllByRole("tab").map((el) => el.textContent?.trim());
    expect(new Set(names).size).toBe(names.length);
  });
});
