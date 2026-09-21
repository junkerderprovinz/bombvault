// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { NamedRepo, OffsiteTarget, SaveWarning, Settings } from "../lib/api";

const FIELD_TARGET: OffsiteTarget = {
  id: "t-f", domain: "containers", name: "B2", repo: "b2:bkt:containers", credsRef: "", storageClass: "",
  immutable: false, schedule: "", retentionKeepLast: 7, retentionKeepDaily: 0, retentionKeepWeekly: 0,
  retentionKeepMonthly: 0, limitUpload: 0, limitDownload: 0, growthBudgetGb: 0, enabled: true, createdAt: 1, sortOrder: 0,
};
const DIRECT: NamedRepo = {
  id: "d1", name: "B2 direct", repo: "b2:bkt:containers-direct", credsRef: "", storageClass: "", limitUpload: 0,
  limitDownload: 0, immutable: false, enabled: true, inUse: 2, companionOf: "t-f", companionLost: false,
};

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
    containersOffsite: FIELD_TARGET.repo,
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

const puts: Settings[] = [];
let putWarnings: SaveWarning[] = [];

vi.mock("../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/api")>()),
  getSettings: () =>
    Promise.resolve({ ok: true, settings: baseSettings(), hostMountRoot: "/host/user", platform: "unraid" }),
  putSettings: (s: Settings) => {
    puts.push(s);
    return Promise.resolve({ ok: true, warnings: putWarnings, notes: [] });
  },
  getAuth: () => Promise.resolve({ enabled: false, authed: false }),
  listContainers: () => Promise.resolve({ ok: true, containers: [] }),
  listVMs: () => Promise.resolve({ ok: true, vms: [] }),
  listFileSets: () => Promise.resolve({ ok: true, fileSets: [] }),
  getStatus: () => Promise.resolve({ ok: true }),
  listOffsiteTargets: () => Promise.resolve({ ok: true, targets: [FIELD_TARGET] }),
  listRepos: () => Promise.resolve({ ok: true, repos: [DIRECT] }),
  getCloud: () =>
    Promise.resolve({
      ok: true, s3KeyId: "", s3Region: "", restUser: "", s3StorageClass: "", s3SecretSet: false, restPasswordSet: false,
    }),
  getCloudCredSets: () => Promise.resolve({ ok: true, sets: [] }),
  getRclone: () => Promise.resolve({ ok: true, remotes: [] }),
}));

const { SettingsPage } = await import("./Settings");

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
  await screen.findByText("Also applies to B2 direct. Items whose only copy is there: 2.");
}

// Outside the advanced mode the retention card holds the tab's only number
// fields, "Keep last" first.
function keepLast(): HTMLInputElement {
  return screen.getAllByRole("spinbutton")[0] as HTMLInputElement;
}

async function typeKeepLast(value: string) {
  await act(async () => {
    fireEvent.change(keepLast(), { target: { value } });
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
  puts.length = 0;
  putWarnings = [];
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("the off-site retention card beside a used direct repository", () => {
  it("asks before keeping less and puts the saved value back when told no", async () => {
    await renderOffsiteTab();
    await typeKeepLast("3");
    expect((await screen.findByRole("dialog")).textContent).toContain(
      "Items whose only copy is in B2 direct: 2. Keeping less deletes their older snapshots for good at the next prune. Save anyway?"
    );
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["common.cancel"] }));
    });
    expect(puts).toHaveLength(0);
    await waitFor(() => expect(keepLast().value).toBe("7"));
  });

  it("saves the lower value once confirmed and shows what the server warns about", async () => {
    putWarnings = [{ code: "direct-retention-lowered", targetId: "t-f", targetName: "B2", items: 2 }];
    await renderOffsiteTab();
    await typeKeepLast("3");
    await act(async () => {
      fireEvent.click(await screen.findByRole("button", { name: en["common.confirm"] }));
    });
    await waitFor(() => expect(puts).toHaveLength(1));
    expect(puts[0].offsiteRetentionKeepLast).toBe(3);
    await screen.findByText("B2 direct now keeps less. Items whose only copy is there: 2.");
  });

  it("does not ask when the save keeps more", async () => {
    await renderOffsiteTab();
    await typeKeepLast("9");
    await waitFor(() => expect(puts).toHaveLength(1));
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});
