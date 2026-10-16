// @vitest-environment jsdom
// The Retention page has three cards: the local keep rules, the off-site keep
// rules, each with its shared rules and a switch per source, and the preview.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router-dom";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { Settings } from "../lib/api";

function baseSettings(): Settings {
  return {
    encryptionEnabled: true,
    containersEnabled: true,
    containersPath: "backups/containers",
    containersSchedule: "daily 02:00",
    containersOffsite: "offsite/containers",
    vmsOffsite: "",
    flashOffsite: "",
    configOffsite: "",
    filesOffsite: "",
    zfsOffsite: "",
    retentionKeepLast: 5,
    retentionKeepDaily: 7,
    retentionKeepWeekly: 4,
    retentionKeepMonthly: 6,
    retentionKeepYearly: 0,
    ownRetention: {},
    offsiteRetentionKeepLast: 0,
    offsiteRetentionKeepDaily: 0,
    offsiteRetentionKeepWeekly: 0,
    offsiteRetentionKeepMonthly: 12,
    offsiteRetentionKeepYearly: 0,
    ownOffsiteRetention: {},
    compression: {},
    defaultLanguage: "en",
    registryAuths: [],
    vmsPath: "backups/vms",
    flashPath: "backups/flash",
    configPath: "backups/config",
    filesPath: "backups/files",
    zfsPath: "backups/zfs",
  } as unknown as Settings;
}

const putCalls: Settings[] = [];
const previews: [string, string | undefined][] = [];

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getSettings: () =>
      Promise.resolve({ ok: true, settings: baseSettings(), hostMountRoot: "/host/user", platform: "unraid" }),
    putSettings: (s: Settings) => {
      putCalls.push(s);
      return Promise.resolve({ ok: true });
    },
    getAuth: () => Promise.resolve({ enabled: false, authed: false }),
    listContainers: () => Promise.resolve({ ok: true, containers: [] }),
    listVMs: () => Promise.resolve({ ok: true, vms: [] }),
    listFileSets: () => Promise.resolve({ ok: true, fileSets: [] }),
    listZFSDatasets: () => Promise.resolve({ ok: true, datasets: [] }),
    getStatus: () => Promise.resolve({ ok: true }),
    listStorageLocations: () => Promise.reject(new Error("offline")),
    previewRetention: (domain: string, source?: string) => {
      previews.push([domain, source]);
      return Promise.resolve({
        ok: true,
        preview: {
          policy: { on: true, keepLast: 5, keepDaily: 0, keepWeekly: 0, keepMonthly: 0, keepYearly: 0, own: false },
          repos: [],
        },
      });
    },
  };
});

const { SettingsPage } = await import("./Settings");

async function renderRetention() {
  const router = createMemoryRouter(
    [
      {
        path: "/settings/:page",
        element: (
          <I18nProvider>
            <ToastProvider>
              <SettingsPage />
            </ToastProvider>
          </I18nProvider>
        ),
      },
    ],
    { initialEntries: ["/settings/storage"] }
  );
  await act(async () => {
    render(<RouterProvider router={router} />);
  });
  await screen.findByRole("heading", { name: new RegExp(`^${en["storage.keep.title"]}`) });
  return router;
}

function card(name: string): HTMLElement {
  return document.querySelector(`[data-search-card="${name}"]`) as HTMLElement;
}

const KEEP = en["storage.keep.title"];
const PREVIEW = en["restore.preview"];

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
  (globalThis as unknown as { ResizeObserver: unknown }).ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  };
  putCalls.length = 0;
  previews.length = 0;
});

afterEach(() => {
  cleanup();
  localStorage.clear();
});

describe("the retention cards of the Storage locations tile", () => {
  it("send the keep rules to the storage locations and keep no fields of their own", async () => {
    const router = await renderRetention();
    const cards = [...document.querySelectorAll("[data-search-card]")].map((c) => c.getAttribute("data-search-card"));
    const at = cards.indexOf(KEEP);
    expect(cards.slice(at, at + 2)).toEqual([KEEP, PREVIEW]);
    expect(within(card(KEEP)).getByText(en["settings.retentionWhere"])).toBeTruthy();
    expect(within(card(KEEP)).queryAllByRole("spinbutton")).toHaveLength(0);
    await act(async () => {
      fireEvent.click(within(card(KEEP)).getByRole("link", { name: en["storage.rules.open"] }));
    });
    expect(router.state.location.pathname).toBe("/storage");
  });

  it("has one preview, on its own card, asking about both copies", async () => {
    await renderRetention();
    const buttons = screen.getAllByRole("button", { name: new RegExp(en["retentionPreview.show"], "i") });
    expect(buttons).toHaveLength(1);
    expect(card(PREVIEW).contains(buttons[0])).toBe(true);
    expect(within(card(PREVIEW)).getByRole("combobox", { name: en["retentionPreview.sourceLabel"] })).toBeTruthy();
    await act(async () => {
      fireEvent.click(buttons[0]);
    });
    await waitFor(() => expect(previews).toHaveLength(2));
    expect(previews).toEqual(expect.arrayContaining([["containers", undefined], ["containers", "offsite"]]));
  });
});


describe("the storage locations tile", () => {
  it("says the locations could not be read instead of checking forever", async () => {
    await renderRetention();
    await waitFor(() => expect(screen.getByText(en["storage.loadFailed"])).toBeTruthy());
    expect(screen.queryByText(en["dashboard.checking"])).toBeNull();
  });
});
