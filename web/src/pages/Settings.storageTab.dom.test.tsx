// @vitest-environment jsdom
// The Storage tab opens with the places, the domains and the restore folder,
// and holds no card for what a place already covers.
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { Settings } from "../lib/api";

const puts: Settings[] = [];

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
    putSettings: (s: Settings) => {
      puts.push(s);
      return Promise.resolve({ ok: true });
    },
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
    getStorageDomains: () => Promise.resolve({ ok: true, domains: [] }),
  };
});

const { SettingsPage } = await import("./Settings");

beforeEach(() => {
  puts.length = 0;
  vi.useFakeTimers({ shouldAdvanceTime: true });
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
  vi.useRealTimers();
  window.location.hash = "";
});

async function storageTab() {
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
}

// The card titles are the tab's only second-level headings.
const cardTitles = () => screen.getAllByRole("heading", { level: 2 }).map((h) => h.textContent ?? "");

it("holds the places, the domains and the restore folder, then the other cards", async () => {
  await storageTab();
  const titles = cardTitles();
  expect(titles[0]!.startsWith(en["places.title"])).toBe(true);
  expect(titles[1]!.startsWith(en["storageDomains.title"])).toBe(true);
  expect(titles[2]!.startsWith(en["settings.restoreFolder"])).toBe(true);
  expect(titles[3]!.startsWith(en["settings.imageMaintenanceTitle"])).toBe(true);
});

it("has no card for repositories, placement defaults, backup paths or local retention", async () => {
  await storageTab();
  const titles = cardTitles();
  for (const gone of ["Repositories", "Placement defaults", "Backup Paths", "Snapshot retention"]) {
    expect(titles.some((title) => title.startsWith(gone)), gone).toBe(false);
  }
  expect(screen.queryByDisplayValue("backups/containers")).toBeNull();
});

it("saves the restore folder 800 ms after the last key", async () => {
  await storageTab();
  fireEvent.change(screen.getByRole("textbox", { name: en["settings.restoreFolder"] }), { target: { value: "restore/here" } });
  expect(puts).toHaveLength(0);
  await act(async () => {
    vi.advanceTimersByTime(900);
  });
  await waitFor(() => expect(puts).toHaveLength(1));
  expect(puts[0]!.restoreFolder).toBe("restore/here");
});
