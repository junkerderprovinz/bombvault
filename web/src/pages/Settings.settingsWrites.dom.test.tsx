// @vitest-environment jsdom
// PUT /api/settings takes the whole settings object, which the page builds by
// merging one field onto its baseline of what the server last confirmed. Two
// writes inside one round trip therefore have to run in order, the second
// built on the first, and an import has to refresh the baseline so the next
// click does not send the old configuration back. The page runs against a
// mocked client, and the tests decide when each PUT resolves.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router-dom";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { Settings } from "../lib/api";

/** A settings object with every field the page reads on the General page. */
function baseSettings(over: Partial<Settings> = {}): Settings {
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
    containersOffsiteSchedule: "",
    vmsOffsiteSchedule: "",
    flashOffsiteSchedule: "",
    configOffsiteSchedule: "",
    filesOffsiteSchedule: "",
    everythingSchedule: "",
    everythingPreHook: "",
    everythingPostHook: "",
    retentionKeepLast: 5,
    retentionKeepDaily: 7,
    retentionKeepWeekly: 4,
    retentionKeepMonthly: 6,
    retentionKeepYearly: 0,
    defaultLanguage: "en",
    registryAuths: [],
    ...over,
  } as unknown as Settings;
}

/** One controllable PUT: the test decides when (and how) it resolves. */
type Pending = {
  body: Settings;
  resolve: (v: { ok: boolean; error?: string }) => void;
};

const putCalls: Pending[] = [];
let settingsOnServer = baseSettings();
let importApplyResult: { ok: boolean; error?: string } = { ok: true, applied: true };
const importApplyCalls: string[] = [];

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getSettings: () =>
      Promise.resolve({ ok: true, settings: settingsOnServer, hostMountRoot: "/host/user", platform: "unraid" }),
    putSettings: (s: Settings) =>
      new Promise((resolve) => {
        putCalls.push({ body: s, resolve });
      }),
    getAuth: () => Promise.resolve({ enabled: false, authed: false }),
    listContainers: () => Promise.resolve({ ok: true, containers: [] }),
    listVMs: () => Promise.resolve({ ok: true, vms: [] }),
    listFileSets: () => Promise.resolve({ ok: true, fileSets: [] }),
    listZFSDatasets: () => Promise.resolve({ ok: true, datasets: [] }),
    getStatus: () => Promise.resolve({ ok: true }),
    importSettingsPreview: () =>
      Promise.resolve({
        ok: true,
        preview: true,
        summary: {
          schemaVersion: 1,
          exportedAt: "2026-08-20T00:00:00Z",
          appVersion: "v8.0.0",
          offsiteTargets: 0,
          credentials: { present: false, cloud: false, rclone: false, notify: false, mqtt: false },
          settingsGroups: ["schedules"],
          newTargets: [],
        },
      }),
    importSettingsApply: (text: string) => {
      importApplyCalls.push(text);
      return Promise.resolve(importApplyResult);
    },
  };
});

// Imported after vi.mock so the page picks up the mocked client.
const { SettingsPage } = await import("./Settings");

let router: ReturnType<typeof createMemoryRouter>;

async function renderPage() {
  router = createMemoryRouter(
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
    { initialEntries: ["/settings/general"] }
  );
  await act(async () => {
    render(<RouterProvider router={router} />);
  });
}

/** A ToggleRow's switch, found by the accessible name its label gives it. */
function toggle(name: string) {
  return screen.getByRole("switch", { name });
}

/** Navigates to another settings page in place. The rail's own links do the
 * same thing, and the page must not remount when they do: a stale baseline
 * would otherwise survive the switch. */
async function gotoPage(page: string) {
  await act(async () => {
    await router.navigate(`/settings/${page}`);
  });
}

/** ThemeCard reads prefers-color-scheme on mount, and jsdom has no matchMedia. */
function stubMatchMedia() {
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
}

/** The page measures its rail on mount, and jsdom has no ResizeObserver.
 * Nothing here depends on the width, so a no-op is enough. */
function stubResizeObserver() {
  (globalThis as unknown as { ResizeObserver: unknown }).ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  };
}

beforeEach(() => {
  stubMatchMedia();
  stubResizeObserver();
  putCalls.length = 0;
  importApplyCalls.length = 0;
  settingsOnServer = baseSettings();
  importApplyResult = { ok: true, applied: true };
});

afterEach(() => {
  cleanup();
});

// Each test renders the whole Settings page, which can take past five seconds
// when the suite runs in parallel.
describe("two settings writes inside one round-trip", { timeout: 15000 }, () => {
  it("sends the second one built on the first, not on the pre-first baseline", async () => {
    await renderPage();

    // Flip Containers on and leave its PUT unresolved.
    fireEvent.click(toggle(en["settings.containersEnabled"]));
    await waitFor(() => expect(putCalls.length).toBe(1));
    expect(putCalls[0].body.containersEnabled).toBe(true);

    // Flip Flash on while the first request is still open. A second switch,
    // so the first one's own in-flight guard does not apply.
    fireEvent.click(toggle(en["settings.flashEnabled"]));

    // Nothing may be sent yet: the second write waits for the first to land.
    await act(async () => {});
    expect(putCalls.length).toBe(1);

    await act(async () => {
      putCalls[0].resolve({ ok: true });
    });

    await waitFor(() => expect(putCalls.length).toBe(2));
    const second = putCalls[1].body;
    expect(second.flashEnabled).toBe(true);
    // Carrying the first field at its old value would leave the server with
    // one of the two changes while the UI shows both.
    expect(second.containersEnabled).toBe(true);

    await act(async () => {
      putCalls[1].resolve({ ok: true });
    });
  });

  it("keeps a rejected write out of the next one's baseline", async () => {
    await renderPage();

    fireEvent.click(toggle(en["settings.vmsEnabled"]));
    await waitFor(() => expect(putCalls.length).toBe(1));

    fireEvent.click(toggle(en["settings.flashEnabled"]));
    await act(async () => {
      // The backend refuses VMs, as its SSH check does when they are switched on.
      putCalls[0].resolve({ ok: false, error: "no working SSH connection" });
    });

    await waitFor(() => expect(putCalls.length).toBe(2));
    expect(putCalls[1].body.flashEnabled).toBe(true);
    expect(putCalls[1].body.vmsEnabled).toBe(false);

    await act(async () => {
      putCalls[1].resolve({ ok: true });
    });
  });
});

describe("a settings import", { timeout: 15000 }, () => {
  async function importAFile() {
    await gotoPage("system");
    const file = new File(['{"schemaVersion":1}'], "settings.json", { type: "application/json" });
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    await act(async () => {
      fireEvent.change(input, { target: { files: [file] } });
    });
    // The server now holds a different configuration than the page loaded.
    settingsOnServer = baseSettings({
      containersSchedule: "everyN 7 03:00",
      retentionKeepDaily: 30,
      filesEnabled: true,
    });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["settingsIO.confirmButton"] }));
    });
  }

  it("re-reads the configuration, so the next click cannot undo it", async () => {
    await renderPage();
    await importAFile();
    expect(importApplyCalls.length).toBe(1);

    // One click on an unrelated switch, the way a user would carry on.
    await gotoPage("general");
    fireEvent.click(toggle(en["settings.containersEnabled"]));
    await waitFor(() => expect(putCalls.length).toBe(1));

    const sent = putCalls[0].body;
    expect(sent.containersEnabled).toBe(true);
    // Everything the import changed is still there.
    expect(sent.containersSchedule).toBe("everyN 7 03:00");
    expect(sent.retentionKeepDaily).toBe(30);
    expect(sent.filesEnabled).toBe(true);

    await act(async () => {
      putCalls[0].resolve({ ok: true });
    });
  });

  it("leaves the baseline alone when the apply itself fails", async () => {
    await renderPage();
    importApplyResult = { ok: false, error: "unsupported schemaVersion 2" };
    await importAFile();

    await gotoPage("general");
    fireEvent.click(toggle(en["settings.containersEnabled"]));
    await waitFor(() => expect(putCalls.length).toBe(1));
    // The import did not happen, so the page's own (unchanged) configuration
    // is the correct thing to send.
    expect(putCalls[0].body.containersSchedule).toBe("daily 02:00");

    await act(async () => {
      putCalls[0].resolve({ ok: true });
    });
  });
});
