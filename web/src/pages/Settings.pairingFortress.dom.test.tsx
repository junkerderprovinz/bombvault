// @vitest-environment jsdom
// Settings > Pairing offers the one phrase whether or not a section that pairs
// over the group is on, since the same words bring the other apps of the
// fortress in, not only BombVault's own instances.
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
      Promise.resolve({ ok: true, joined: false, phrase: false, defaultDevice: "Tower", passwordSet: true, members: [], permissions: [], pairedWith: [] }),
    getGroup: () =>
      Promise.resolve({
        ok: true,
        active: false,
        instanceId: "self",
        name: "Tower",
        passwordSet: true,
        members: [],
        away: [],
        settling: false,
        relay: { mode: "project", url: "", projectUrl: "wss://parleyport.halleluja.design/relay/connect", connected: false, serve: false, serveClients: 0 },
        joinedAgo: 0,
        memberSeen: false,
        waiting: [],
        selfAddress: "",
        ownAddresses: [],
      }),
    listContainers: () => Promise.resolve({ ok: true, containers: [] }),
    listVMs: () => Promise.resolve({ ok: true, vms: [] }),
    getStatus: () => Promise.resolve({ ok: true }),
  };
});

const { SettingsPage } = await import("./Settings");

afterEach(cleanup);

describe("Settings > Pairing", () => {
  it("offers to make or enter the phrase while no section pairs over the group", async () => {
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
    expect(await screen.findByRole("button", { name: new RegExp(en["pairing.create"]) })).toBeTruthy();
    expect(screen.getByRole("button", { name: new RegExp(en["pairing.enter"]) })).toBeTruthy();
    expect(document.querySelector("[data-fortress]")).toBeNull();
  });
});
