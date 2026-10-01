// @vitest-environment jsdom
// The MCP card is the only way to mint a key, so it sits on the Integrations
// page in plain sight rather than behind Advanced.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { Settings } from "../lib/api";

// The card's own tests run it switched on; the last case switches it off.
const mcp = vi.hoisted(() => ({ shipped: true }));
vi.mock("../lib/mcpSwitch", () => ({
  get mcpShipped() {
    return mcp.shipped;
  },
}));

const settingsOnServer = {
  encryptionEnabled: true,
  containersEnabled: true,
  registryAuths: [],
} as unknown as Settings;

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getSettings: () =>
      Promise.resolve({ ok: true, settings: settingsOnServer, hostMountRoot: "/host/user", platform: "unraid" }),
    putSettings: () => Promise.resolve({ ok: true }),
    getAuth: () => Promise.resolve({ enabled: true, authed: true }),
    listContainers: () => Promise.resolve({ ok: true, containers: [] }),
    listVMs: () => Promise.resolve({ ok: true, vms: [] }),
    listFileSets: () => Promise.resolve({ ok: true, fileSets: [] }),
    getStatus: () => Promise.resolve({ ok: true }),
    listMcpKeys: () =>
      Promise.resolve({
        ok: true,
        endpointPath: "/mcp",
        limit: 10,
        authEnabled: true,
        hostAllowsKeys: true,
        startsPerHour: 12,
        cooldownMinutes: 15,
        itemStartsPerDay: 5,
        certificate: null,
        keys: [],
        revoked: [],
        oauth: { enabled: false, issuer: "", active: false, grantLimit: 10, connectorPath: "/mcp" },
      }),
  };
});

const { SettingsPage } = await import("./Settings");

function stubBrowser() {
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
}

async function renderPage(page: string) {
  await act(async () => {
    render(
      <MemoryRouter initialEntries={[`/settings/${page}`]}>
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

beforeEach(stubBrowser);
afterEach(() => {
  cleanup();
  mcp.shipped = true;
});

describe("the MCP card on the settings page", () => {
  it("renders on the Integrations page", async () => {
    await renderPage("integrations");

    expect(screen.getByText(en["mcp.title"])).toBeTruthy();
    expect(screen.getByText(en["mcp.statusOff"])).toBeTruthy();
  });

  it("stays off the other pages", async () => {
    await renderPage("general");

    expect(screen.queryByText(en["mcp.title"])).toBeNull();
  });

  it("is nowhere while the server is switched off", async () => {
    mcp.shipped = false;
    await renderPage("integrations");

    expect(screen.queryByText(en["mcp.title"])).toBeNull();
    expect(screen.queryByText(en["mcp.statusOff"])).toBeNull();
  });
});
