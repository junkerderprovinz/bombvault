// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { Settings } from "../lib/api";
import { targetPreview } from "../lib/placement.testsupport";

const fake = await vi.hoisted(async () => (await import("../lib/placement.testsupport")).createPlacementApi());
const stored = vi.hoisted(() => ({ containersOffsite: "", puts: [] as Settings[] }));

function baseSettings(): Settings {
  return {
    encryptionEnabled: true,
    containersEnabled: true,
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
    containersOffsite: stored.containersOffsite,
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
    offsiteRetentionKeepLast: 7,
    offsiteRetentionKeepDaily: 0,
    offsiteRetentionKeepWeekly: 0,
    offsiteRetentionKeepMonthly: 0,
    defaultLanguage: "en",
    registryAuths: [],
  } as unknown as Settings;
}

vi.mock("../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/api")>()),
  ...fake.api,
  getSettings: () =>
    Promise.resolve({ ok: true, settings: baseSettings(), hostMountRoot: "/host/user", platform: "unraid" }),
  putSettings: (s: Settings) => {
    stored.puts.push(s);
    return Promise.resolve({ ok: true, warnings: [], notes: [] });
  },
  getAuth: () => Promise.resolve({ enabled: false, authed: false }),
  listContainers: () => Promise.resolve({ ok: true, containers: [] }),
  listVMs: () => Promise.resolve({ ok: true, vms: [] }),
  listFileSets: () => Promise.resolve({ ok: true, fileSets: [] }),
  getStatus: () => Promise.resolve({ ok: true }),
  listOffsiteTargets: () => Promise.resolve({ ok: true, targets: [] }),
  getCloud: () =>
    Promise.resolve({
      ok: true, s3KeyId: "", s3Region: "", restUser: "", s3StorageClass: "", s3SecretSet: false, restPasswordSet: false,
    }),
  getCloudCredSets: () => Promise.resolve({ ok: true, sets: [] }),
  getRclone: () => Promise.resolve({ ok: true, remotes: [] }),
}));

const { SettingsPage } = await import("./Settings");

const PLACEHOLDER = "rest:http://host:8000/repo";

async function renderOffsiteTab() {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <SettingsPage />
        </ToastProvider>
      </I18nProvider>
    );
  });
  await act(async () => {
    window.location.hash = "#offsite";
    window.dispatchEvent(new HashChangeEvent("hashchange"));
  });
  await screen.findAllByPlaceholderText(PLACEHOLDER);
}

// The domain cards come in the order containers, VMs, flash, folders, self-backup.
function field(i: number): HTMLInputElement {
  return screen.getAllByPlaceholderText(PLACEHOLDER)[i] as HTMLInputElement;
}

async function type(i: number, value: string) {
  await act(async () => {
    fireEvent.change(field(i), { target: { value } });
  });
  await act(async () => {
    vi.advanceTimersByTime(900);
  });
}

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
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
  fake.reset();
  stored.containersOffsite = "";
  stored.puts.length = 0;
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("the off-site field", () => {
  it("saves a new location only after Enter and the question, then leaves the ticked items out", async () => {
    fake.reply("getNewTargetPreview", {
      ok: true,
      preview: targetPreview({ formerlyExcluded: [{ identity: "container:plex", skip: ["t-x"] }] }),
    });
    await renderOffsiteTab();
    await type(0, "b2:bucket:containers");
    expect(stored.puts).toHaveLength(0);
    await act(async () => {
      fireEvent.keyDown(field(0), { key: "Enter" });
    });
    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain("At its first run b2:bucket:containers receives every item not set to Local.");
    fireEvent.click(within(dialog).getByRole("switch", { name: "Leave these out here too" }));
    await act(async () => {
      fireEvent.click(within(dialog).getByRole("button", { name: en["common.confirm"] }));
    });
    await waitFor(() => expect(stored.puts).toHaveLength(1));
    expect(stored.puts[0].containersOffsite).toBe("b2:bucket:containers");
    await waitFor(() =>
      expect(fake.callsTo("excludeFromTarget")).toEqual([
        [{ domain: "containers", field: true, identities: ["container:plex"], default: false }],
      ])
    );
  });

  it("puts the stored location back when the question is cancelled", async () => {
    await renderOffsiteTab();
    await type(0, "b2:bucket:containers");
    await act(async () => {
      fireEvent.blur(field(0));
    });
    await act(async () => {
      fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: en["common.cancel"] }));
    });
    await waitFor(() => expect(field(0).value).toBe(""));
    expect(stored.puts).toHaveLength(0);
  });

  it("clears a location without asking", async () => {
    stored.containersOffsite = "b2:bucket:containers";
    await renderOffsiteTab();
    await type(0, "");
    await act(async () => {
      fireEvent.click(within(field(0).parentElement as HTMLElement).getByRole("button", { name: en["settings.save"] }));
    });
    await waitFor(() => expect(stored.puts).toHaveLength(1));
    expect(stored.puts[0].containersOffsite).toBe("");
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(fake.callsTo("getNewTargetPreview")).toEqual([]);
  });

  it("saves the flash field on Enter without asking", async () => {
    await renderOffsiteTab();
    await type(2, "b2:bucket:flash");
    await act(async () => {
      fireEvent.keyDown(field(2), { key: "Enter" });
    });
    await waitFor(() => expect(stored.puts).toHaveLength(1));
    expect(stored.puts[0].flashOffsite).toBe("b2:bucket:flash");
    expect(fake.callsTo("getNewTargetPreview")).toEqual([]);
  });
});
