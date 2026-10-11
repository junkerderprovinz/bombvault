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
  await screen.findByRole("heading", { name: new RegExp(`^${en["settings.retentionOffsiteTitle"]}`) });
  return router;
}

function card(name: string): HTMLElement {
  return document.querySelector(`[data-search-card="${name}"]`) as HTMLElement;
}

const OFFSITE = en["settings.retentionOffsiteTitle"];
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
  it("draws the Off-site retention card and the Preview card in a row, and no card for the local rules", async () => {
    await renderRetention();
    const cards = [...document.querySelectorAll("[data-search-card]")].map((c) => c.getAttribute("data-search-card"));
    const at = cards.indexOf(OFFSITE);
    expect(cards.slice(at, at + 2)).toEqual([OFFSITE, PREVIEW]);
    expect(screen.getAllByText(en["settings.retentionYearly"])).toHaveLength(1);
  });

  it("shows the shared rules and a switch per source, named by the source", async () => {
    await renderRetention();
    expect(within(card(OFFSITE)).getAllByRole("spinbutton").map((f) => (f as HTMLInputElement).value)).toEqual(["0", "0", "0", "12", "0"]);
    const switches = within(card(OFFSITE)).getAllByRole("switch");
    expect(switches).toHaveLength(6);
    expect(switches[0].getAttribute("aria-label")).toBe(en["settings.ownOffsiteRetentionFor"].replace("{source}", en["nav.containers"]));
    expect(within(card(OFFSITE)).getByText(en["nav.containers"])).toBeTruthy();
  });

  it("unfolds a source's off-site fields under its switch and saves them as its own off-site rules", async () => {
    await renderRetention();
    const toggle = within(card(OFFSITE)).getByRole("switch", {
      name: en["settings.ownOffsiteRetentionFor"].replace("{source}", en["nav.containers"]),
    });
    await act(async () => {
      fireEvent.click(toggle);
    });
    await waitFor(() => expect(putCalls).toHaveLength(1));
    expect(putCalls[0].ownOffsiteRetention).toEqual({
      containers: { keepLast: 0, keepDaily: 0, keepWeekly: 0, keepMonthly: 12, keepYearly: 0 },
    });
    expect(putCalls[0].ownRetention).toEqual({});
    expect(within(card(OFFSITE)).getAllByRole("spinbutton")).toHaveLength(10);
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

  it("links the additional off-site targets to the off-site cards", async () => {
    const router = await renderRetention();
    const link = within(card(OFFSITE)).getByRole("link", { name: en["settings.retentionExtraTargets"] });
    await act(async () => {
      fireEvent.click(link);
    });
    expect(router.state.location.pathname).toBe("/settings/storage");
  });

  it("puts the OR rule and the append-only note in the rules card's (i)", async () => {
    await renderRetention();
    const tips = (name: string) =>
      [...card(name).querySelectorAll("h2 [aria-label]")].map((el) => el.getAttribute("aria-label") ?? "").join(" ");
    expect(tips(OFFSITE)).toContain(en["settings.retentionCombineInfo"]);
    expect(tips(OFFSITE)).toContain(en["settings.retentionImmutableNotPruned"]);
    for (const key of ["settings.retentionCombineInfo", "settings.retentionImmutableNotPruned"] as const) {
      expect(screen.queryByText(en[key])).toBeNull();
    }
  });
});
