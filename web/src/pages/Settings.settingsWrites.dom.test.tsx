// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// SettingsPage — what a settings write is allowed to send.
//
// PUT /api/settings takes the WHOLE settings object, and the page builds that
// object by merging one field's new value onto its own baseline of "what the
// server last confirmed". Two things used to break that:
//
//   1. The baseline only moved when a PUT RETURNED, and nothing serialized the
//      writes. Every field auto-saves now, so two writes inside one round-trip
//      is the normal case (flip two switches; or edit the Containers cadence
//      with "sync" on, which arms a second debounce one render later). Both
//      objects were built from the pre-first baseline, so each carried the
//      other's field at its OLD value and whichever response landed last won.
//      The UI showed both edits, the server kept one, and the loser reappeared
//      reverted on the next reload.
//
//   2. A settings IMPORT replaced the whole configuration on the server and
//      never told the page, so the baseline still held the entire pre-import
//      configuration. One click on any switch afterwards PUT that object back
//      and silently undid the import — reporting a successful save while doing
//      it.
//
// Both are asserted through the real page against a mocked client: the test
// controls when each PUT resolves, which is what makes the overlap real rather
// than hypothetical.
//
// jsdom opted in explicitly (real clicks, controlled promises) — see
// Selector.dom.test.tsx's header for this repo's naming convention.
// ---------------------------------------------------------------------------
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { Settings } from "../lib/api";

/** A settings object with every field the page reads on the General tab. */
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
      return Promise.resolve(importApplyResult);
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

/** A ToggleRow's switch, found by the accessible name its label gives it. */
function toggle(name: string) {
  return screen.getByRole("switch", { name });
}

/** Select a tab through the page's own deep-link path (/settings#<tab>) rather
 * than by clicking the strip: the strip measures itself in two passes, so a
 * label query there matches more than one node. Switching tabs does NOT remount
 * the page — which is exactly why a stale baseline survives one. */
async function gotoTab(tab: string) {
  await act(async () => {
    window.location.hash = "#" + tab;
    window.dispatchEvent(new HashChangeEvent("hashchange"));
  });
}

/** Minimal matchMedia stub — ThemeCard reads prefers-color-scheme on mount and
 * jsdom does not implement matchMedia (same stub as Settings.themeCard's). */
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

/** Minimal ResizeObserver stub — the page measures its tab strip on mount and
 * jsdom does not implement the observer. Nothing here depends on the measured
 * width, so a no-op observer is enough. */
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

describe("two settings writes inside one round-trip", () => {
  it("sends the second one built on the first, not on the pre-first baseline", async () => {
    await renderPage();

    // Flip Containers on. Its PUT is in flight and deliberately not resolved.
    fireEvent.click(toggle(en["settings.containersEnabled"]));
    await waitFor(() => expect(putCalls.length).toBe(1));
    expect(putCalls[0].body.containersEnabled).toBe(true);

    // Flip Flash on while the first request is still open — a second switch,
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
    // The whole point: the second full-object PUT must not carry the first
    // field at its pre-click value, or the server ends up with only one of the
    // two changes while the UI shows both.
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
      // The backend refuses VMs (its documented OFF->ON SSH check).
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

describe("a settings import", () => {
  async function importAFile() {
    await gotoTab("system");
    const file = new File(['{"schemaVersion":1}'], "settings.json", { type: "application/json" });
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    await act(async () => {
      fireEvent.change(input, { target: { files: [file] } });
    });
    // The server now holds a DIFFERENT configuration than the one this page
    // loaded: that is exactly what an import is.
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
    await gotoTab("general");
    fireEvent.click(toggle(en["settings.containersEnabled"]));
    await waitFor(() => expect(putCalls.length).toBe(1));

    const sent = putCalls[0].body;
    expect(sent.containersEnabled).toBe(true);
    // Everything the import changed must still be there. Before the fix this
    // PUT carried the page's pre-import object and silently restored every
    // one of these to its old value, while reporting a successful save.
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

    await gotoTab("general");
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
