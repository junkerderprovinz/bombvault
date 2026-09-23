// @vitest-environment jsdom
// The 800 ms debounce is the only thing that writes a Settings text field. An
// edit still inside it has to survive leaving the page, a blank registry row
// must not vanish while another row is edited, the registry list has to keep
// what was typed during its own save, and an import must drop a pending edit
// instead of letting it land behind the import. The page runs against a
// mocked client, and the tests decide when each PUT resolves.
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
/** The tests resolve import applies too: the apply is the longest part of the
 *  window between the Import click and the reloaded configuration. */
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
          newTargets: [],
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

// Imported after vi.mock so the page picks up the mocked client.
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

/** Selects a tab through the deep link, because the strip measures itself in
 *  two passes and a label query there matches more than one node. */
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

    // Following a sidebar link unmounts the routed page.
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

    // Add a row: a blank one appears at the end and is not saved.
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["settings.registryAdd"] }));
    });
    expect(hostInputs()).toHaveLength(2);

    // Then go back and fix a typo in the first row's username.
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

    // Misaligned ids would send typing in the blank row to the first one.
    await waitFor(() => expect(hostInputs()).toHaveLength(2));
    await act(async () => {
      fireEvent.change(hostInputs()[1], { target: { value: "docker.io" } });
    });
    expect(hostInputs()[0].value).toBe("ghcr.io/updated");
    expect(hostInputs()[1].value).toBe("docker.io");
  });
});

describe("editing the registry list while its save is in flight", () => {
  it("keeps a row added after the PUT went out", async () => {
    settingsOnServer = baseSettings({
      registryAuths: [registry({ host: "ghcr.io", username: "old" })],
    });
    await renderPage();
    await gotoTab("storage");

    // Let the debounce elapse, so the PUT is in flight with its payload fixed.
    await act(async () => {
      fireEvent.change(hostInputs()[0], { target: { value: "ghcr.io/updated" } });
    });
    await act(async () => {
      vi.advanceTimersByTime(900);
    });
    await waitFor(() => expect(putCalls).toHaveLength(1));

    // A row added during the round trip, which the payload knows nothing
    // about.
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["settings.registryAdd"] }));
    });
    expect(hostInputs()).toHaveLength(2);

    await act(async () => {
      putCalls[0].resolve({ ok: true });
    });

    // The response must not carry the card back to the list it was sent with.
    await waitFor(() => expect(hostInputs()).toHaveLength(2));
    expect(hostInputs()[1].value).toBe("");

    // ...and the row ids still line up, so typing into the new row reaches it.
    await act(async () => {
      fireEvent.change(hostInputs()[1], { target: { value: "docker.io" } });
    });
    expect(hostInputs()[0].value).toBe("ghcr.io/updated");
    expect(hostInputs()[1].value).toBe("docker.io");
  });

  it("keeps characters typed after the PUT went out", async () => {
    settingsOnServer = baseSettings({
      registryAuths: [registry({ host: "ghcr.io" }), registry({ host: "quay.io" })],
    });
    await renderPage();
    await gotoTab("storage");

    await act(async () => {
      fireEvent.change(hostInputs()[0], { target: { value: "ghcr.io/updated" } });
    });
    await act(async () => {
      vi.advanceTimersByTime(900);
    });
    await waitFor(() => expect(putCalls).toHaveLength(1));

    // Second row edited while the first row's write is still open.
    await act(async () => {
      fireEvent.change(hostInputs()[1], { target: { value: "quay.io/typed-mid-flight" } });
    });
    await act(async () => {
      putCalls[0].resolve({ ok: true });
    });

    await waitFor(() => expect(hostInputs()[1].value).toBe("quay.io/typed-mid-flight"));
    expect(hostInputs()[0].value).toBe("ghcr.io/updated");
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

    // A save stays in flight, so everything queued after it waits, the import
    // included.
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

    // The debounce elapses while the import still waits its turn; the edit
    // must not queue itself behind it.
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

    // Nothing is in flight, so the import starts at once and then waits on its
    // own request, the longer half of the same window.
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
