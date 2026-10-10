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
    { initialEntries: ["/settings/retention"] }
  );
  await act(async () => {
    render(<RouterProvider router={router} />);
  });
  await screen.findByRole("heading", { name: new RegExp(`^${en["settings.retentionLocalTitle"]}`) });
  return router;
}

function card(name: string): HTMLElement {
  return document.querySelector(`[data-search-card="${name}"]`) as HTMLElement;
}

const LOCAL = en["settings.retentionLocalTitle"];
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

describe("the Retention page", () => {
  it("draws a Local retention, an Off-site retention and a Preview card", async () => {
    await renderRetention();
    const cards = [...document.querySelectorAll("[data-search-card]")].map((c) => c.getAttribute("data-search-card"));
    expect(cards).toEqual([LOCAL, OFFSITE, PREVIEW]);
  });

  it("shows each card's shared rules and a switch per source, named by the source", async () => {
    await renderRetention();
    expect(within(card(LOCAL)).getAllByRole("spinbutton").map((f) => (f as HTMLInputElement).value)).toEqual(["5", "7", "4", "6", "0"]);
    expect(within(card(OFFSITE)).getAllByRole("spinbutton").map((f) => (f as HTMLInputElement).value)).toEqual(["0", "0", "0", "12", "0"]);
    for (const [name, sentence] of [
      [LOCAL, en["settings.ownRetentionFor"]],
      [OFFSITE, en["settings.ownOffsiteRetentionFor"]],
    ] as const) {
      const switches = within(card(name)).getAllByRole("switch");
      expect(switches).toHaveLength(6);
      expect(switches[0].getAttribute("aria-label")).toBe(sentence.replace("{source}", en["nav.containers"]));
      expect(within(card(name)).getByText(en["nav.containers"])).toBeTruthy();
    }
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
    expect(within(card(LOCAL)).getAllByRole("spinbutton")).toHaveLength(5);
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

  it("links the additional off-site targets to the Off-site page", async () => {
    const router = await renderRetention();
    const link = within(card(OFFSITE)).getByRole("link", { name: en["settings.retentionExtraTargets"] });
    await act(async () => {
      fireEvent.click(link);
    });
    expect(router.state.location.pathname).toBe("/settings/offsite");
  });

  it("puts the OR rule in both rules cards' (i) and the append-only note in the off-site one", async () => {
    await renderRetention();
    const tips = (name: string) =>
      [...card(name).querySelectorAll("h2 [aria-label]")].map((el) => el.getAttribute("aria-label") ?? "").join(" ");
    expect(tips(LOCAL)).toContain(en["settings.retentionCombineInfo"]);
    expect(tips(LOCAL)).toContain(en["settings.retentionHint"]);
    expect(tips(OFFSITE)).toContain(en["settings.retentionCombineInfo"]);
    expect(tips(OFFSITE)).toContain(en["settings.retentionImmutableNotPruned"]);
    for (const key of ["settings.retentionCombineInfo", "settings.retentionHint", "settings.retentionImmutableNotPruned"] as const) {
      expect(screen.queryByText(en[key])).toBeNull();
    }
  });
});
