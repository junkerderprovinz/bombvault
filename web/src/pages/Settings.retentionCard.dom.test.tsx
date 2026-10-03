// @vitest-environment jsdom
// The Retention page is one card: a Local and an Off-site section, each with
// its shared rules and a switch per source, and one preview at the bottom.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router-dom";
import { AdvancedProvider } from "../lib/advanced";
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
            <AdvancedProvider>
              <ToastProvider>
                <SettingsPage />
              </ToastProvider>
            </AdvancedProvider>
          </I18nProvider>
        ),
      },
    ],
    { initialEntries: ["/settings/retention"] }
  );
  await act(async () => {
    render(<RouterProvider router={router} />);
  });
  await screen.findByRole("heading", { name: new RegExp(`^${en["settings.tab.retention"]}`) });
  return router;
}

function section(name: string): HTMLElement {
  return screen.getByRole("heading", { name: new RegExp(`^${name}`) }).closest("section") as HTMLElement;
}

beforeEach(() => {
  localStorage.setItem("bombvault.advanced", "1");
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
  it("draws one card with a Local and an Off-site section", async () => {
    await renderRetention();
    const cards = document.querySelectorAll("[data-search-card]");
    expect([...cards].map((c) => c.getAttribute("data-search-card"))).toEqual([
      en["settings.tab.retention"],
      en["source.local"],
      en["source.offsite"],
    ]);
    expect(section(en["source.local"]).closest(`[data-search-card="${en["settings.tab.retention"]}"]`)).not.toBeNull();
    expect(section(en["source.offsite"]).closest(`[data-search-card="${en["settings.tab.retention"]}"]`)).not.toBeNull();
  });

  it("shows each section's shared rules and a switch per source", async () => {
    await renderRetention();
    const local = section(en["source.local"]);
    const offsite = section(en["source.offsite"]);
    expect(within(local).getAllByRole("spinbutton").map((f) => (f as HTMLInputElement).value)).toEqual(["5", "7", "4", "6", "0"]);
    expect(within(offsite).getAllByRole("spinbutton").map((f) => (f as HTMLInputElement).value)).toEqual(["0", "0", "0", "12", "0"]);
    expect(within(local).getAllByRole("switch")).toHaveLength(6);
    expect(within(offsite).getAllByRole("switch")).toHaveLength(6);
  });

  it("unfolds a source's off-site fields under its switch and saves them as its own off-site rules", async () => {
    await renderRetention();
    const offsite = section(en["source.offsite"]);
    const toggle = within(offsite).getByRole("switch", {
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
    expect(within(offsite).getAllByRole("spinbutton")).toHaveLength(10);
    expect(within(section(en["source.local"])).getAllByRole("spinbutton")).toHaveLength(5);
  });

  it("has one preview for the whole card, asking about both copies", async () => {
    await renderRetention();
    const buttons = screen.getAllByRole("button", { name: new RegExp(en["retentionPreview.show"], "i") });
    expect(buttons).toHaveLength(1);
    expect(buttons[0].closest("section")).toBeNull();
    await act(async () => {
      fireEvent.click(buttons[0]);
    });
    await waitFor(() => expect(previews).toHaveLength(2));
    expect(previews).toEqual(expect.arrayContaining([["containers", undefined], ["containers", "offsite"]]));
  });

  it("links the additional off-site targets to the Off-site page", async () => {
    const router = await renderRetention();
    const link = within(section(en["source.offsite"])).getByRole("link", { name: en["settings.retentionExtraTargets"] });
    await act(async () => {
      fireEvent.click(link);
    });
    expect(router.state.location.pathname).toBe("/settings/offsite");
  });

  it("keeps the OR rule, the all-zero rule and the append-only note in (i) bubbles", async () => {
    await renderRetention();
    for (const key of ["settings.retentionCombineInfo", "settings.retentionHint", "settings.retentionImmutableNotPruned"] as const) {
      expect(screen.queryByText(en[key])).toBeNull();
      const tips = [...document.querySelectorAll("[aria-label]")].map((el) => el.getAttribute("aria-label") ?? "");
      expect(tips.some((tip) => tip.includes(en[key]))).toBe(true);
    }
  });
});
