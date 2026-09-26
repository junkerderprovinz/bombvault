// @vitest-environment jsdom
// The off-site settings live on the Storage tab, and an #offsite link lands
// there with the address rewritten.
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, within } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { Settings } from "../lib/api";

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getSettings: () =>
      Promise.resolve({
        ok: true,
        settings: { restoreFolder: "restore", registryAuths: [], defaultLanguage: "en" } as unknown as Settings,
        hostMountRoot: "/mnt",
        platform: "unraid",
      }),
    getAuth: () => Promise.resolve({ enabled: false, authed: false }),
    listContainers: () => Promise.resolve({ ok: true, containers: [] }),
    listVMs: () => Promise.resolve({ ok: true, vms: [] }),
    listFileSets: () => Promise.resolve({ ok: true, fileSets: [] }),
    getStatus: () => Promise.resolve({ ok: true }),
  };
});

vi.mock("../lib/places", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/places")>()),
  listPlaces: () => Promise.resolve({ ok: true, places: [], unplaced: [] }),
  getPlacesCatalog: () => Promise.resolve({ ok: true, providers: [] }),
  getStorageDomains: () => Promise.resolve({ ok: true, domains: [] }),
}));

const { SettingsPage } = await import("./Settings");

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
});
afterEach(() => {
  cleanup();
  window.location.hash = "";
});

async function renderPage() {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <SettingsPage />
        </ToastProvider>
      </I18nProvider>
    );
  });
}

it("has no Off-site tab", async () => {
  await renderPage();
  const strip = screen.getByRole("tablist", { name: en["settings.title"] });
  const tabs = within(strip).getAllByRole("tab").map((tab) => tab.textContent);
  expect(tabs).toHaveLength(6);
  expect(tabs).not.toContain("Off-site");
});

it("is where an #offsite link lands, and the address says so", async () => {
  window.location.hash = "#offsite";
  await renderPage();
  const strip = screen.getByRole("tablist", { name: en["settings.title"] });
  expect(within(strip).getByRole("tab", { name: en["settings.tab.storage"] }).getAttribute("aria-selected")).toBe("true");
  expect(window.location.hash).toBe("#storage");
});
