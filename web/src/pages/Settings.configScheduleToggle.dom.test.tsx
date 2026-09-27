// @vitest-environment jsdom
// The Self-backup Card's toggle writes "off" over configSchedule, the only
// place its cadence is stored, so switching back on has to restore the
// cadence it replaced rather than the shipped default. Like
// FlashZipExportCard's rememberedKeep, the value lives for the page's
// lifetime only; after a reload the default is all there is.
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
    zfsEnabled: true,
    configEnabled: true,
    receiverEnabled: false,
    fleetEnabled: false,
    containersPath: "backups/containers",
    vmsPath: "backups/vms",
    flashPath: "backups/flash",
    filesPath: "backups/files",
    zfsPath: "backups/zfs",
    configPath: "backups/config",
    restoreFolder: "restore",
    containersSchedule: "off",
    vmsSchedule: "off",
    flashSchedule: "off",
    filesSchedule: "off",
    zfsSchedule: "off",
    configSchedule: "weekly Sun 04:00",
    containersOffsite: "",
    vmsOffsite: "",
    flashOffsite: "",
    configOffsite: "",
    filesOffsite: "",
    zfsOffsite: "",
    containersOffsiteSchedule: "",
    vmsOffsiteSchedule: "",
    flashOffsiteSchedule: "",
    configOffsiteSchedule: "",
    filesOffsiteSchedule: "",
    zfsOffsiteSchedule: "",
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
    listZFSDatasets: () => Promise.resolve({ ok: true, datasets: [] }),
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

/** The Card title carries the same name, so the role picks the switch. */
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
    // Loaded with the schedule already off, the page has no cadence to
    // remember.
    settingsOnServer = baseSettings({ configSchedule: "off" } as Partial<Settings>);
    await renderSchedulesTab();

    fireEvent.click(selfBackupToggle());
    await waitFor(() => expect(putBodies.length).toBe(1));
    expect(putBodies[0].configSchedule).toBe("daily 02:00");
  });
});

describe("one schedule for every domain", () => {
  it("carries the ZFS cadence along with the others", async () => {
    settingsOnServer = baseSettings({ containersSchedule: "daily 02:00" });
    await renderSchedulesTab();

    await act(async () => {
      fireEvent.click(screen.getAllByRole("switch", { name: en["jobs.syncSchedules"] })[0]);
    });

    await waitFor(() => expect(putBodies.at(-1)?.zfsSchedule).toBe("daily 02:00"));
  });

  it("hands the ZFS cadence back to its own card when the sync is switched off", async () => {
    settingsOnServer = baseSettings({
      containersSchedule: "daily 02:00",
      vmsSchedule: "daily 02:00",
      flashSchedule: "daily 02:00",
      filesSchedule: "daily 02:00",
      zfsSchedule: "daily 02:00",
    });
    await renderSchedulesTab();

    const cadence = () => screen.getByRole("group", { name: en["jobs.zfsSection"] });
    expect(cadence().hasAttribute("disabled")).toBe(true);

    await act(async () => {
      fireEvent.click(screen.getAllByRole("switch", { name: en["jobs.syncSchedules"] })[0]);
    });

    expect(cadence().hasAttribute("disabled")).toBe(false);
  });
});
