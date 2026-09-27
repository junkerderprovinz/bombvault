// @vitest-environment jsdom
// The axes the person owns (language, theme, shape, motion, labels, colours)
// stand on a Look tab of their own, second after General, with the language
// first. General keeps what concerns the app as a whole.
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { Settings } from "../lib/api";

function baseSettings(over: Partial<Settings> = {}): Settings {
  return {
    encryptionEnabled: true,
    containersEnabled: true,
    vmsEnabled: false,
    flashEnabled: false,
    filesEnabled: false,
    configEnabled: false,
    receiverEnabled: false,
    fleetEnabled: false,
    pullEnabled: false,
    dbDumpsEnabled: true,
    containersSchedule: "off",
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
    drDrillTarget: "",
    drDrillTargetVm: "",
    anomalyEnabled: true,
    anomalySensitivity: "balanced",
    anomalyNotifyMin: "critical",
    anomalyRetentionHold: true,
    ...over,
  } as unknown as Settings;
}


vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getSettings: () =>
      Promise.resolve({ ok: true, settings: baseSettings(), hostMountRoot: "/host/user", platform: "unraid" }),
    putSettings: () => Promise.resolve({ ok: true }),
    getAuth: () => Promise.resolve({ enabled: false, authed: false }),
    listContainers: () => Promise.resolve({ ok: true, containers: [] }),
    listVMs: () => Promise.resolve({ ok: true, vms: [] }),
    listFileSets: () => Promise.resolve({ ok: true, fileSets: [] }),
    getStatus: () => Promise.resolve({ ok: true }),
    getDrills: () => Promise.resolve({ ok: true, drills: [], latest: null }),
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

async function renderAt(hash: string) {
  await act(async () => {
    window.location.hash = hash;
    render(
      <MemoryRouter>
        <I18nProvider>
          <ToastProvider>
            <SettingsPage />
          </ToastProvider>
        </I18nProvider>
      </MemoryRouter>
    );
  });
}

/** The card headings of the open tab, top to bottom. */
function headings(): string[] {
  return screen.getAllByRole("heading", { level: 2 }).map((h) => h.textContent?.trim() ?? "");
}

beforeEach(() => {
  stubBrowser();
  localStorage.setItem("bv-lang", "en");
});

afterEach(() => {
  cleanup();
  window.location.hash = "";
  localStorage.removeItem("bv-lang");
});

it("puts the Look tab second, after General", async () => {
  await renderAt("#general");
  const tabs = within(screen.getByRole("tablist", { name: en["settings.title"] })).getAllByRole("tab");
  expect(tabs.slice(0, 2).map((tab) => tab.textContent?.trim())).toEqual([
    en["settings.tab.general"],
    en["settings.tab.look"],
  ]);
});

it("opens the Look tab on the language, then theme and shape", async () => {
  await renderAt("#look");
  await screen.findByRole("button", { name: /English/ });
  expect(headings().slice(0, 3)).toEqual([en["settings.language"], en["settings.theme"], en["settings.shape"]]);
});

it("keeps the look off the General tab", async () => {
  await renderAt("#general");
  await screen.findByText(en["about.title"]);
  const general = headings();
  for (const key of ["settings.language", "settings.theme", "settings.shape", "settings.motion", "settings.colors"] as const) {
    expect(general, key).not.toContain(en[key]);
  }
});
