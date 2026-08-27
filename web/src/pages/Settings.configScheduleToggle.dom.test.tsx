// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// The Self-backup Card's on/off toggle remembers the cadence it switched off.
//
// The toggle writes the literal "off" over configSchedule, which is the only
// place that cadence is stored. Switching back on then wrote the shipped
// "daily 02:00" default, so a user who had set "weekly Sun 04:00", switched the
// schedule off and switched it on again silently got a different schedule than
// the one they had configured — with the confirmation toast reporting a
// successful save either way. The FlashZipExportCard's rememberedKeep is the
// sibling that already did this correctly.
//
// The remembered value lives for this page's lifetime, exactly like
// rememberedKeep's: once "off" is on the server the old string is genuinely
// gone. A reload therefore falls back to the default, which the last test pins
// so nobody mistakes the limit for a bug.
// ---------------------------------------------------------------------------
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { Settings } from "../lib/api";

function baseSettings(over: Partial<Settings> = {}): Settings {
  return {
    encryptionEnabled: true,
    containersEnabled: false,
    vmsEnabled: false,
    flashEnabled: false,
    filesEnabled: false,
    configEnabled: true,
    receiverEnabled: false,
    fleetEnabled: false,
    containersPath: "backups/containers",
    vmsPath: "backups/vms",
    flashPath: "backups/flash",
    filesPath: "backups/files",
    configPath: "backups/config",
    restoreFolder: "restore",
    containersSchedule: "off",
    vmsSchedule: "off",
    flashSchedule: "off",
    filesSchedule: "off",
    configSchedule: "weekly Sun 04:00",
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

const putBodies: Settings[] = [];
let settingsOnServer = baseSettings();

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getSettings: () =>
      Promise.resolve({ ok: true, settings: settingsOnServer, hostMountRoot: "/host/user", platform: "unraid" }),
    putSettings: (s: Settings) => {
      putBodies.push(s);
      return Promise.resolve({ ok: true });
    },
    getAuth: () => Promise.resolve({ enabled: false, authed: false }),
    listContainers: () => Promise.resolve({ ok: true, containers: [] }),
    listVMs: () => Promise.resolve({ ok: true, vms: [] }),
    listFileSets: () => Promise.resolve({ ok: true, fileSets: [] }),
    getStatus: () => Promise.resolve({ ok: true }),
  };
});

const { SettingsPage } = await import("./Settings");

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
  window.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
}

async function renderSchedulesTab() {
  await act(async () => {
    window.location.hash = "#schedules";
    render(
      <I18nProvider>
        <ToastProvider>
          <SettingsPage />
        </ToastProvider>
      </I18nProvider>
    );
  });
  await act(async () => {
    window.dispatchEvent(new HashChangeEvent("hashchange"));
  });
}

/** The Self-backup Card's switch, by the accessible name its label gives it.
 *  The Card title carries the same string, so the role filter is what
 *  distinguishes the control from its heading. */
function selfBackupToggle() {
  return screen.getAllByRole("switch", { name: en["settings.schedulesSelfBackup"] })[0];
}

beforeEach(() => {
  stubMatchMedia();
  stubResizeObserver();
  putBodies.length = 0;
  settingsOnServer = baseSettings();
});

afterEach(() => {
  cleanup();
});

describe("self-backup schedule toggle", () => {
  it("restores the cadence it switched off, not the shipped default", async () => {
    await renderSchedulesTab();

    // Off: the configured weekly is overwritten with "off" …
    fireEvent.click(selfBackupToggle());
    await waitFor(() => expect(putBodies.length).toBe(1));
    expect(putBodies[0].configSchedule).toBe("off");

    // … and back on: the user's own cadence, not "daily 02:00".
    fireEvent.click(selfBackupToggle());
    await waitFor(() => expect(putBodies.length).toBe(2));
    expect(putBodies[1].configSchedule).toBe("weekly Sun 04:00");
  });

  it("survives more than one round trip through off", async () => {
    await renderSchedulesTab();

    for (let i = 0; i < 2; i++) {
      fireEvent.click(selfBackupToggle());
      await waitFor(() => expect(putBodies.length).toBe(i * 2 + 1));
      fireEvent.click(selfBackupToggle());
      await waitFor(() => expect(putBodies.length).toBe(i * 2 + 2));
    }
    expect(putBodies.map((b) => b.configSchedule)).toEqual([
      "off",
      "weekly Sun 04:00",
      "off",
      "weekly Sun 04:00",
    ]);
  });

  it("falls back to the default when it has nothing to restore", async () => {
    // A page that loads with the schedule already off has never seen a cadence
    // to remember: the server keeps one string per domain, so the old value is
    // gone. Switching on then legitimately offers the shipped default.
    settingsOnServer = baseSettings({ configSchedule: "off" } as Partial<Settings>);
    await renderSchedulesTab();

    fireEvent.click(selfBackupToggle());
    await waitFor(() => expect(putBodies.length).toBe(1));
    expect(putBodies[0].configSchedule).toBe("daily 02:00");
  });
});
