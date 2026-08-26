// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// SettingsPage — what happens to an edit that is still inside its debounce.
//
// With the Save buttons gone the 800ms debounce is the ONLY thing that ever
// writes a text field, which changes what its two neighbouring behaviours mean:
//
//   1. LEAVING THE PAGE. The page-level cleanup cleared every pending timer, so
//      typing a value and clicking a sidebar link within 800ms threw the edit
//      away. No toast, no error, and the field had already shown the value as
//      accepted. Nothing was protected by the cancel — the debounce captures its
//      value explicitly and a post-unmount setState is a no-op — so an edit was
//      being lost for nothing.
//
//   2. TRIMMING BLANK REGISTRY ROWS. keepRegistryAuths drops untouched blank
//      rows, which was right when it ran on a Save CLICK ("I am finished") and
//      is wrong when it runs 800ms after a keystroke: adding a row and then
//      going back to fix a typo in an existing one made the new, still-empty row
//      vanish from under the cursor.
//
//   3. AN IMPORT ARRIVING WHILE ONE IS PENDING. An import replaces the whole
//      configuration, so an edit typed against the old one has to be dropped.
//      The drop used to happen inside the serialized write queue, i.e. whenever
//      the import reached the head of it — so a debounce that elapsed while an
//      earlier save still held the queue appended its write BEHIND the import
//      and wrote a pre-import value over the freshly imported configuration.
//
// All three are driven through the real page against a mocked client, with the
// test controlling when each PUT resolves.
//
// jsdom opted in explicitly (real typing, controlled promises) — see
// Selector.dom.test.tsx's header for this repo's naming convention.
// ---------------------------------------------------------------------------
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { RegistryAuthEntry, Settings } from "../lib/api";

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
    defaultLanguage: "en",
    registryAuths: [],
    ...over,
  } as unknown as Settings;
}

type Pending = { body: Settings; resolve: (v: { ok: boolean; error?: string }) => void };

const putCalls: Pending[] = [];
let settingsOnServer = baseSettings();
/** Import applies are controllable too: the window this file cares about runs
 *  from the Import click until the re-loaded configuration is installed, and
 *  the apply itself is the longest part of it. */
const importApplyCalls: string[] = [];
const importApplies: ((v: { ok: boolean; applied?: boolean; error?: string }) => void)[] = [];

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
          credentials: { present: false, cloud: false, rclone: false, notify: false },
          settingsGroups: ["schedules"],
        },
      }),
    importSettingsApply: (text: string) => {
      importApplyCalls.push(text);
      return new Promise((resolve) => {
        importApplies.push(resolve);
      });
    },
  };
});

// Imported AFTER vi.mock so the page picks up the mocked client.
const { SettingsPage } = await import("./Settings");

async function renderPage() {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <SettingsPage />
        </ToastProvider>
      </I18nProvider>
    );
  });
}

/** Select a tab through the page's own deep-link path — the strip measures
 *  itself in two passes, so a label query there matches more than one node. */
async function gotoTab(tab: string) {
  await act(async () => {
    window.location.hash = "#" + tab;
    window.dispatchEvent(new HashChangeEvent("hashchange"));
  });
}

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

function stubResizeObserver() {
  (globalThis as unknown as { ResizeObserver: unknown }).ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  };
}

function registry(over: Partial<RegistryAuthEntry> = {}): RegistryAuthEntry {
  return { host: "", username: "", token: "", tokenSet: false, ...over } as RegistryAuthEntry;
}

/** Every registry row's host input, in row order. */
function hostInputs() {
  return screen.getAllByLabelText(en["settings.registryHost"]) as HTMLInputElement[];
}

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  stubMatchMedia();
  stubResizeObserver();
  putCalls.length = 0;
  importApplyCalls.length = 0;
  importApplies.length = 0;
  settingsOnServer = baseSettings();
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("leaving the page with an edit still inside its debounce", () => {
  it("sends the edit instead of discarding it", async () => {
    settingsOnServer = baseSettings({ registryAuths: [registry({ host: "ghcr.io" })] });
    await renderPage();
    await gotoTab("storage");

    const host = hostInputs()[0];
    await act(async () => {
      fireEvent.change(host, { target: { value: "registry.example.com" } });
    });

    // Still well inside the 800ms window: nothing has been sent yet.
    expect(putCalls).toHaveLength(0);

    // The user clicks a sidebar link. SettingsPage is a routed component, so it
    // unmounts — which is where the edit used to die.
    await act(async () => {
      cleanup();
    });

    expect(putCalls.length).toBeGreaterThan(0);
    const sent = putCalls[putCalls.length - 1].body as unknown as { registryAuths: RegistryAuthEntry[] };
    expect(sent.registryAuths[0].host).toBe("registry.example.com");
  });

  it("still sends it normally when the debounce is allowed to elapse", async () => {
    settingsOnServer = baseSettings({ registryAuths: [registry({ host: "ghcr.io" })] });
    await renderPage();
    await gotoTab("storage");

    await act(async () => {
      fireEvent.change(hostInputs()[0], { target: { value: "quay.io" } });
    });
    await act(async () => {
      vi.advanceTimersByTime(900);
    });

    await waitFor(() => expect(putCalls).toHaveLength(1));
    const sent = putCalls[0].body as unknown as { registryAuths: RegistryAuthEntry[] };
    expect(sent.registryAuths[0].host).toBe("quay.io");
  });
});

describe("a blank registry row while another row is being edited", () => {
  it("stays on screen when the debounced save for a different row lands", async () => {
    settingsOnServer = baseSettings({
      registryAuths: [registry({ host: "ghcr.io", username: "old" })],
    });
    await renderPage();
    await gotoTab("storage");

    // Add a row: a blank one appears at the end and is deliberately not saved.
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["settings.registryAdd"] }));
    });
    expect(hostInputs()).toHaveLength(2);

    // Now go back and fix a typo in the FIRST row's username.
    const users = screen.getAllByLabelText(en["settings.registryUser"]) as HTMLInputElement[];
    await act(async () => {
      fireEvent.change(users[0], { target: { value: "corrected" } });
    });
    await act(async () => {
      vi.advanceTimersByTime(900);
    });
    await waitFor(() => expect(putCalls).toHaveLength(1));

    // The server is not asked to store the blank row...
    const sent = putCalls[0].body as unknown as { registryAuths: RegistryAuthEntry[] };
    expect(sent.registryAuths).toHaveLength(1);
    expect(sent.registryAuths[0].username).toBe("corrected");

    // ...and the row the user just added is still there, waiting to be filled in.
    await act(async () => {
      putCalls[0].resolve({ ok: true });
    });
    await waitFor(() => {
      expect(hostInputs()).toHaveLength(2);
    });
    expect(hostInputs()[1].value).toBe("");
  });

  it("keeps the row ids aligned, so the blank row is still the one that was added", async () => {
    settingsOnServer = baseSettings({
      registryAuths: [registry({ host: "ghcr.io", username: "old" })],
    });
    await renderPage();
    await gotoTab("storage");

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["settings.registryAdd"] }));
    });
    await act(async () => {
      fireEvent.change(hostInputs()[0], { target: { value: "ghcr.io/updated" } });
    });
    await act(async () => {
      vi.advanceTimersByTime(900);
    });
    await waitFor(() => expect(putCalls).toHaveLength(1));
    await act(async () => {
      putCalls[0].resolve({ ok: true });
    });

    // Typing into the surviving blank row must reach THAT row, not the first
    // one — which is what an id/row misalignment would break.
    await waitFor(() => expect(hostInputs()).toHaveLength(2));
    await act(async () => {
      fireEvent.change(hostInputs()[1], { target: { value: "docker.io" } });
    });
    expect(hostInputs()[0].value).toBe("ghcr.io/updated");
    expect(hostInputs()[1].value).toBe("docker.io");
  });
});

describe("importing settings while an edit is still inside its debounce", () => {
  /** Pick a file on the System tab and confirm the import. */
  async function confirmImport() {
    await gotoTab("system");
    const file = new File(['{"schemaVersion":1}'], "settings.json", { type: "application/json" });
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    await act(async () => {
      fireEvent.change(input, { target: { files: [file] } });
    });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["settingsIO.confirmButton"] }));
    });
  }

  it("drops the pending edit even when an earlier save is holding the write queue", async () => {
    settingsOnServer = baseSettings({ registryAuths: [registry({ host: "ghcr.io" })] });
    await renderPage();
    await gotoTab("general");

    // A save is in flight and deliberately never resolved yet, so everything
    // queued after it waits — including the import.
    await act(async () => {
      fireEvent.click(screen.getByRole("switch", { name: en["settings.containersEnabled"] }));
    });
    await waitFor(() => expect(putCalls).toHaveLength(1));

    // The user edits a registry host, arming the 800ms debounce...
    await gotoTab("storage");
    await act(async () => {
      fireEvent.change(hostInputs()[0], { target: { value: "typed-before-import.example.com" } });
    });

    // ...then imports a file. The server now holds a different configuration.
    await confirmImport();
    settingsOnServer = baseSettings({
      registryAuths: [registry({ host: "imported.example.com" })],
      retentionKeepDaily: 30,
    });
    expect(importApplyCalls).toHaveLength(0); // still stuck behind the in-flight save

    // The debounce elapses while the import is still waiting its turn. This is
    // the moment the queued cancel was too late for: the edit used to queue
    // itself BEHIND the import here.
    await act(async () => {
      vi.advanceTimersByTime(900);
    });

    // Let the in-flight save land, which releases the queue, and let the import
    // run all the way through.
    await act(async () => {
      putCalls[0].resolve({ ok: true });
    });
    await waitFor(() => expect(importApplyCalls).toHaveLength(1));
    await act(async () => {
      importApplies[0]({ ok: true, applied: true });
    });

    // Nothing may have been written after the import: the only PUT is the one
    // that was already in flight before it started.
    expect(putCalls).toHaveLength(1);

    // ...and the imported value is what the page now shows and holds.
    await gotoTab("storage");
    await waitFor(() => expect(hostInputs()[0].value).toBe("imported.example.com"));
  });

  it("ignores a keystroke made while the import is being applied", async () => {
    settingsOnServer = baseSettings({ registryAuths: [registry({ host: "ghcr.io" })] });
    await renderPage();

    // Nothing is in flight, so the import starts straight away — and then sits
    // on its own request, which is the longer half of the same window.
    await confirmImport();
    await waitFor(() => expect(importApplyCalls).toHaveLength(1));
    settingsOnServer = baseSettings({
      registryAuths: [registry({ host: "imported.example.com" })],
    });

    // The user keeps typing while the apply is still open.
    await gotoTab("storage");
    await act(async () => {
      fireEvent.change(hostInputs()[0], { target: { value: "typed-during-import.example.com" } });
    });
    await act(async () => {
      vi.advanceTimersByTime(900);
    });

    await act(async () => {
      importApplies[0]({ ok: true, applied: true });
    });

    // That keystroke was typed against the configuration the import replaced,
    // so it must never reach the server.
    expect(putCalls).toHaveLength(0);
    await waitFor(() => expect(hostInputs()[0].value).toBe("imported.example.com"));
  });
});
