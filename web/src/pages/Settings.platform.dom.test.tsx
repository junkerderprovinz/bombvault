// @vitest-environment jsdom
// The flash backup reads the Unraid boot drive, so the switch for it is offered
// on Unraid only. A host that already has it on keeps the switch.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { Settings } from "../lib/api";

let platformOnServer = "unraid";
let flashOn = false;

function settings(): Settings {
  return {
    encryptionEnabled: true,
    containersEnabled: true,
    vmsEnabled: false,
    flashEnabled: flashOn,
    filesEnabled: false,
    configEnabled: false,
    zfsEnabled: false,
    receiverEnabled: false,
    fleetEnabled: false,
    pullEnabled: false,
    dbDumpsEnabled: true,
    registryAuths: [],
    defaultLanguage: "en",
  } as unknown as Settings;
}

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getSettings: () =>
      Promise.resolve({ ok: true, settings: settings(), hostMountRoot: "/host/user", platform: platformOnServer }),
    getAuth: () => Promise.resolve({ enabled: false, authed: false }),
    listContainers: () => Promise.resolve({ ok: true, containers: [] }),
    listVMs: () => Promise.resolve({ ok: true, vms: [] }),
    listFileSets: () => Promise.resolve({ ok: true, fileSets: [] }),
    listZFSDatasets: () => Promise.resolve({ ok: true, datasets: [] }),
    getStatus: () => Promise.resolve({ ok: true }),
  };
});

const { SettingsPage } = await import("./Settings");

async function renderGeneralPage() {
  await act(async () => {
    render(
      <MemoryRouter initialEntries={["/settings/general"]}>
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
      </MemoryRouter>
    );
  });
}

const flashSwitch = () => screen.queryAllByRole("switch", { name: en["settings.flashEnabled"] });

beforeEach(() => {
  window.matchMedia = ((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia;
  window.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
  platformOnServer = "unraid";
  flashOn = false;
});

afterEach(cleanup);

describe("the flash backup switch", () => {
  it("is offered on Unraid", async () => {
    await renderGeneralPage();
    expect(flashSwitch().length).toBeGreaterThan(0);
  });

  it("is not offered on TrueNAS", async () => {
    platformOnServer = "truenas";
    await renderGeneralPage();
    expect(flashSwitch()).toHaveLength(0);
  });

  it("stays where it is already switched on, so it can be turned off", async () => {
    platformOnServer = "generic";
    flashOn = true;
    await renderGeneralPage();
    expect(flashSwitch().length).toBeGreaterThan(0);
  });
});
