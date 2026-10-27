// @vitest-environment jsdom
// What the person owns (language, theme, shape, motion, labels, colours)
// stands on a Look page of its own, second in the rail after General.
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { Settings } from "../lib/api";
import { LABEL_MODES } from "../lib/controls";

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

it("opens the Look page on one Appearance card that holds language, theme, corners and animation", async () => {
  await renderAt("/settings/look");
  await screen.findByRole("heading", { name: new RegExp(`^${en["tile.look.title"]}`) });
  expect(headings().slice(0, 3)).toEqual([en["tile.look.title"], en["settings.labels"], en["settings.colors"]]);
  const card = document.querySelector<HTMLElement>(`[data-search-card="${en["tile.look.title"]}"]`);
  expect(card).not.toBeNull();
  const rows = within(card as HTMLElement);
  expect(rows.getByRole("combobox", { name: en["language.label"] })).toBeTruthy();
  for (const key of ["settings.theme", "settings.shape", "settings.motion"] as const) {
    expect(rows.getByRole("tablist", { name: en[key] }), key).toBeTruthy();
  }
  expect(rows.getByRole("switch", { name: en["settings.quietToasts"] })).toBeTruthy();
});

it("keeps language, theme, shape, motion and colours off the General page", async () => {
  await renderAt("/settings/general");
  await screen.findByRole("heading", { name: new RegExp(`^${en["settings.domains"]}`) });
  const general = headings();
  for (const key of ["tile.look.title", "settings.labels", "settings.colors"] as const) {
    expect(general, key).not.toContain(en[key]);
  }
  expect(screen.queryByRole("combobox", { name: en["language.label"] })).toBeNull();
});

it("sets each label axis in a row of its own, the name at the start and its selector beside it", async () => {
  await renderAt("/settings/look");
  await screen.findByRole("heading", { name: new RegExp(`^${en["settings.labels"]}`) });
  const card = document.querySelector<HTMLElement>(`[data-search-card="${en["settings.labels"]}"]`)!;
  for (const axis of ["buttons", "sidebar", "tabs", "bottombar"] as const) {
    const name = en[`settings.labels.${axis}`];
    const selector = within(card).getByRole("tablist", { name });
    const row = selector.closest(".justify-between")!;
    expect(within(row as HTMLElement).getByText(name), axis).toBeTruthy();
  }
});

// The strips are card-sized, so they follow the Buttons label mode. Text mode
// shows the words alone; the others show the glyph, with or without them.
it.each(LABEL_MODES)("gives each corner and motion choice its glyph and keeps its name in %s mode", async (mode) => {
  localStorage.setItem("bv-labels-buttons", mode);
  try {
    await renderAt("/settings/look");
    for (const [axis, choices] of [
      ["settings.shape", ["round", "soft", "square"]],
      ["settings.motion", ["off", "subtle", "wild"]],
    ] as const) {
      const strip = await screen.findByRole("tablist", { name: en[axis] });
      for (const choice of choices) {
        const name = en[`${axis}.${choice}`];
        const segment = within(strip).getByRole("tab", { name });
        expect(segment.querySelector("svg") !== null, `${name} glyph`).toBe(mode !== "text");
        const words = segment.querySelector("[data-sel-label]");
        expect(words !== null, `${name} words`).toBe(mode !== "glyph");
      }
    }
  } finally {
    localStorage.removeItem("bv-labels-buttons");
  }
});
