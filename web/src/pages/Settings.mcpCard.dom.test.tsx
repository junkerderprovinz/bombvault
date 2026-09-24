// @vitest-environment jsdom
// The MCP card is the only way to mint a key, so it sits on the System tab in
// plain sight rather than behind Advanced.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { Settings } from "../lib/api";

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

async function renderTab(tab: string) {
  await act(async () => {
    window.location.hash = "#" + tab;
    render(
      <I18nProvider>
        <ToastProvider>
          <SettingsPage />
        </ToastProvider>
      </I18nProvider>
    );
  });
  await act(async () => {
    window.dispatchEvent(new HashChangeEvent("hashchange"));
  });
}

beforeEach(stubBrowser);
afterEach(cleanup);

describe("the MCP card on the settings page", () => {
  it("renders on the System tab", async () => {
    await renderTab("system");

    expect(screen.getByText(en["mcp.title"])).toBeTruthy();
    expect(screen.getByText(en["mcp.statusOff"])).toBeTruthy();
  });

  it("stays off the other tabs", async () => {
    await renderTab("general");

    expect(screen.queryByText(en["mcp.title"])).toBeNull();
  });
});
