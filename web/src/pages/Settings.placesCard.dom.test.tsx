// @vitest-environment jsdom
// The Storage tab opens with the places card.
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { Settings } from "../lib/api";

function baseSettings(): Settings {
  return {
    encryptionEnabled: true,
    containersEnabled: false,
    vmsEnabled: false,
    flashEnabled: false,
    filesEnabled: false,
    configEnabled: false,
    receiverEnabled: false,
    fleetEnabled: false,
    containersPath: "backups/containers",
    vmsPath: "backups/vms",
    flashPath: "backups/flash",
    filesPath: "backups/files",
    configPath: "backups/config",
    restoreFolder: "restore",
    containersSchedule: "daily 02:00",
    vmsSchedule: "off",
    flashSchedule: "off",
    filesSchedule: "off",
    configSchedule: "off",
    containersOffsite: "",
    vmsOffsite: "",
    flashOffsite: "",
    configOffsite: "",
    filesOffsite: "",
    retentionKeepLast: 5,
    retentionKeepDaily: 7,
    retentionKeepWeekly: 4,
    retentionKeepMonthly: 6,
    defaultLanguage: "en",
    registryAuths: [],
  } as unknown as Settings;
}

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getSettings: () => Promise.resolve({ ok: true, settings: baseSettings(), hostMountRoot: "/mnt", platform: "unraid" }),
    getAuth: () => Promise.resolve({ enabled: false, authed: false }),
    listContainers: () => Promise.resolve({ ok: true, containers: [] }),
    listVMs: () => Promise.resolve({ ok: true, vms: [] }),
    listFileSets: () => Promise.resolve({ ok: true, fileSets: [] }),
    getStatus: () => Promise.resolve({ ok: true }),
  };
});

vi.mock("../lib/places", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/places")>();
  return {
    ...actual,
    listPlaces: () => Promise.resolve({ ok: true, places: [], unplaced: [] }),
    getPlacesCatalog: () => Promise.resolve({ ok: true, providers: [] }),
  };
});

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

it("opens the Storage tab with the places card", async () => {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <SettingsPage />
        </ToastProvider>
      </I18nProvider>
    );
  });
  await act(async () => {
    window.location.hash = "#storage";
    window.dispatchEvent(new HashChangeEvent("hashchange"));
  });
  // The card titles are the tab's only second-level headings.
  const cards = screen.getAllByRole("heading", { level: 2 });
  expect(cards[0]!.textContent?.startsWith(en["places.title"])).toBe(true);
  expect(screen.getByText(en["places.empty"])).toBeTruthy();
});
