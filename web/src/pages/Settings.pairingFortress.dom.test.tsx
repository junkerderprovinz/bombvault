// @vitest-environment jsdom
// Settings > Pairing carries the fortress whether or not a section that pairs
// over the group is on, since the fortress joins every app and not only
// BombVault's own instances.
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { Settings } from "../lib/api";

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getSettings: () => Promise.resolve({ ok: true, settings: {} as Settings, hostMountRoot: "/host/user", platform: "unraid" }),
    getAuth: () => Promise.resolve({ enabled: true, authed: true, totp: false, minPasswordLen: 12 }),
    getFortress: () =>
      Promise.resolve({ ok: true, joined: false, phrase: false, device: "Tower", defaultDevice: "Tower", passwordSet: true, members: [], permissions: [], pairedWith: [], removed: [], removedHere: false }),
    listContainers: () => Promise.resolve({ ok: true, containers: [] }),
    listVMs: () => Promise.resolve({ ok: true, vms: [] }),
    getStatus: () => Promise.resolve({ ok: true }),
  };
});

const { SettingsPage } = await import("./Settings");

afterEach(cleanup);

describe("Settings > Pairing", () => {
  it("offers to join the fortress while no section pairs over the group", async () => {
    localStorage.setItem("bv-lang", "en");
    await act(async () => {
      render(
        <MemoryRouter initialEntries={["/settings/pairing"]}>
          <Routes>
            <Route
              path="/settings/:page"
              element={
                <I18nProvider>
                  <ToastProvider>
                    <SettingsPage />
                  </ToastProvider>
                </I18nProvider>
              }
            />
          </Routes>
        </MemoryRouter>,
      );
    });
    expect(await screen.findByText(en["settings.pairingNeedsDomain"])).toBeTruthy();
    expect(await screen.findByText("Not in a fortress yet")).toBeTruthy();
    expect(document.querySelector('[data-fortress="open-join"]')).toBeTruthy();
  });
});
