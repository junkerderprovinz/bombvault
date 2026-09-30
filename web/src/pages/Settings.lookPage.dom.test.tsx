// @vitest-environment jsdom
// The axes the person owns (theme, shape, motion, labels, colours) stand on a
// Look page of their own, second in the rail after General. Language sits on
// General instead, alongside what concerns the app as a whole.
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
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

async function renderAt(path: string) {
  await act(async () => {
    render(
      <MemoryRouter initialEntries={[path]}>
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

/** The card headings of the open page, top to bottom. */
function headings(): string[] {
  return screen.getAllByRole("heading", { level: 2 }).map((h) => h.textContent?.trim() ?? "");
}

beforeEach(() => {
  stubBrowser();
  localStorage.setItem("bv-lang", "en");
});

afterEach(() => {
  cleanup();
  localStorage.removeItem("bv-lang");
});

it("puts Look second in the settings rail, after General", async () => {
  await renderAt("/settings/general");
  const rail = screen.getByRole("navigation", { name: en["settings.railLabel"] });
  const links = within(rail).getAllByRole("link");
  expect(links.slice(0, 2).map((link) => link.textContent?.trim())).toEqual([
    en["settings.tab.general"],
    en["settings.tab.look"],
  ]);
});

it("opens the Look page on Theme, then Corners", async () => {
  await renderAt("/settings/look");
  await screen.findByRole("heading", { name: en["settings.theme"] });
  expect(headings().slice(0, 2)).toEqual([en["settings.theme"], en["settings.shape"]]);
});

it("keeps theme, shape, motion and colours off the General page", async () => {
  await renderAt("/settings/general");
  await screen.findByRole("button", { name: /English/ });
  const general = headings();
  for (const key of ["settings.theme", "settings.shape", "settings.motion", "settings.colors"] as const) {
    expect(general, key).not.toContain(en[key]);
  }
});
