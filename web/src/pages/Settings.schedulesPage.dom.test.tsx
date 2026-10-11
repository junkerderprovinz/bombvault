// @vitest-environment jsdom
// Settings > Schedule: the full backup as a plan sentence, the sections that
// run on a plan of their own as exceptions, and the entries each section takes
// along. The api module is mocked, so every write is a captured PUT or PATCH.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { BackupItem, Settings } from "../lib/api";

function baseSettings(over: Partial<Settings> = {}): Settings {
  return {
    containersEnabled: false,
    vmsEnabled: false,
    flashEnabled: false,
    filesEnabled: true,
    zfsEnabled: true,
    configEnabled: true,
    receiverEnabled: false,
    fleetEnabled: false,
    containersSchedule: "off",
    vmsSchedule: "off",
    flashSchedule: "off",
    filesSchedule: "off",
    zfsSchedule: "off",
    configSchedule: "weekly Sun 04:00",
    containersOffsiteSchedule: "",
    vmsOffsiteSchedule: "",
    flashOffsiteSchedule: "",
    configOffsiteSchedule: "",
    filesOffsiteSchedule: "",
    zfsOffsiteSchedule: "",
    everythingSchedule: "daily 03:00",
    everythingPreHook: "",
    everythingPostHook: "",
    everythingPreHookSet: false,
    everythingPostHookSet: false,
    catchUpMissed: true,
    perItemSchedules: false,
    registryAuths: [],
    ...over,
  } as unknown as Settings;
}

function item(over: Partial<BackupItem>): BackupItem {
  return {
    kind: "zfs",
    key: "z1",
    name: "tank/appdata",
    included: true,
    paused: false,
    effectiveSchedule: { kind: "everything", spec: "daily 03:00" },
    lastBackup: 0,
    lastRunStatus: "",
    sourceBytes: null,
    ...over,
  } as BackupItem;
}

const putBodies: Settings[] = [];
const patches: [string, unknown][] = [];
let settingsOnServer = baseSettings();
let itemsOnServer: BackupItem[] | Error = [];

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
    listItems: () =>
      itemsOnServer instanceof Error
        ? Promise.reject(itemsOnServer)
        : Promise.resolve({ ok: true, items: itemsOnServer, unlisted: [] }),
    patchZFSDataset: (id: string, patch: unknown) => {
      patches.push([id, patch]);
      return Promise.resolve({ ok: true });
    },
    getStatus: () => Promise.resolve({ ok: true }),
  };
});

const { SettingsPage } = await import("./Settings");

function stubBrowser() {
  window.matchMedia = ((query: string) => ({
    matches: true,
    media: query,
    onchange: null,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia;
  window.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
}

async function renderPage() {
  await act(async () => {
    render(
      <MemoryRouter initialEntries={["/settings/schedules"]}>
        <Routes>
          <Route
            path="/settings/:page"
            element={
              <I18nProvider>
                <ToastProvider>
                  <SettingsPage />
                </ToastProvider>
              </I18nProvider>
            }
          />
        </Routes>
      </MemoryRouter>
    );
  });
}

function card(titleKey: keyof typeof en): HTMLElement {
  const heading = screen.getByRole("heading", { name: new RegExp(`^${en[titleKey]}`) });
  return heading.parentElement as HTMLElement;
}

const SAVED = { timeout: 3000 };

beforeEach(() => {
  stubBrowser();
  putBodies.length = 0;
  patches.length = 0;
  settingsOnServer = baseSettings();
  itemsOnServer = [];
});

afterEach(() => {
  cleanup();
});

describe("the full backup", () => {
  it("reads as one plan sentence and names only the sections without a plan of their own", async () => {
    await renderPage();
    const full = card("settings.everythingTitle");
    expect(within(full).getByRole("button", { name: /full backup.*daily at 3:00/i })).toBeTruthy();
    expect(within(full).getByText(/^Includes: Folders and ZFS$/)).toBeTruthy();
  });

  it("says when every section runs on a plan of its own", async () => {
    settingsOnServer = baseSettings({ filesSchedule: "daily 01:00", zfsSchedule: "daily 01:30" });
    await renderPage();
    expect(within(card("settings.everythingTitle")).getByText(en["schedules.fullAllOwn"])).toBeTruthy();
  });

  it("says that only own plans run while it is off", async () => {
    settingsOnServer = baseSettings({ everythingSchedule: "off" });
    await renderPage();
    expect(within(card("settings.everythingTitle")).getByText(en["schedules.fullOff"])).toBeTruthy();
  });

  it("opens the plan editor from its sentence and saves a new mode", async () => {
    await renderPage();
    fireEvent.click(within(card("settings.everythingTitle")).getByRole("button", { name: /daily at 3:00/i }));
    const editor = await screen.findByRole("dialog", { name: en["settings.everythingTitle"] });
    fireEvent.click(within(editor).getByRole("tab", { name: en["cadence.off"] }));
    await waitFor(() => expect(putBodies.at(-1)?.everythingSchedule).toBe("off"), SAVED);
  });
});

describe("plans per section", () => {
  it("lists only the sections with a plan of their own", async () => {
    await renderPage();
    const plans = card("schedules.plans");
    expect(within(plans).getByRole("button", { name: /self-backup.*weekly/i })).toBeTruthy();
    expect(within(plans).queryByRole("button", { name: /^ZFS:/ })).toBeNull();
    expect(within(plans).getByText(en["schedules.plansRuleOthers"])).toBeTruthy();
  });

  it("has no warning about a double backup, since a section with its own plan leaves the full backup", async () => {
    await renderPage();
    expect(within(card("schedules.plans")).queryByText(/backed up twice/)).toBeNull();
  });

  it("gives a section back to the full backup when its exception is removed", async () => {
    await renderPage();
    fireEvent.click(screen.getByRole("button", { name: `${en["exceptions.remove"]}: ${en["nav.config"]}` }));
    await waitFor(() => expect(putBodies.at(-1)?.configSchedule).toBe("off"), SAVED);
  });

  it("adds a section as an exception with a weekly plan", async () => {
    settingsOnServer = baseSettings({ configSchedule: "off" });
    await renderPage();
    const plans = card("schedules.plans");
    expect(within(plans).getByText(en["schedules.plansRuleAll"])).toBeTruthy();
    fireEvent.click(within(plans).getAllByRole("button", { name: en["exceptions.add"] })[0]);
    fireEvent.click(await screen.findByRole("option", { name: en["nav.zfs"] }));
    await waitFor(() => expect(putBodies.at(-1)?.zfsSchedule).toBe("weekly Sun 04:00"), SAVED);
  });

  it("gives an off-site copy a plan of its own and takes it back", async () => {
    settingsOnServer = baseSettings({ zfsOffsiteSchedule: "daily 05:00" });
    await renderPage();
    expect(screen.getByText(en["schedules.offsiteRuleOthers"])).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: `${en["exceptions.remove"]}: ${en["nav.zfs"]}` }));
    await waitFor(() => expect(putBodies.at(-1)?.zfsOffsiteSchedule).toBe(""), SAVED);
  });
});

describe("scheduled entries", () => {
  it("counts the entries of each section on the schedule", async () => {
    itemsOnServer = [item({}), item({ key: "z2", name: "tank/media", included: false })];
    await renderPage();
    await waitFor(() => expect(within(card("schedules.entries")).getByText("1 of 2 scheduled")).toBeTruthy());
    expect(within(card("schedules.entries")).getByText("0 of 0 scheduled")).toBeTruthy();
  });

  it("still draws the plans when the entries cannot be loaded", async () => {
    itemsOnServer = new Error("offline");
    await renderPage();
    expect(card("schedules.plans")).toBeTruthy();
    expect(within(card("schedules.entries")).getAllByText("0 of 0 scheduled")).toHaveLength(2);
  });

  it("switches an entry in and out of the schedule from its section's window", async () => {
    itemsOnServer = [item({ included: false, effectiveSchedule: { kind: "none", spec: "", reason: "excluded" } })];
    await renderPage();
    const row = document.querySelector<HTMLElement>("[data-kind=\"zfs\"]")!;
    fireEvent.click(within(row).getByRole("button", { name: en["schedules.entriesButton"] }));
    const window = await screen.findByRole("dialog");
    expect(within(window).getByText("tank/appdata")).toBeTruthy();
    fireEvent.click(within(window).getByRole("switch", { name: `${en["entry.settings.inSchedule"]}: tank/appdata` }));
    await waitFor(() => expect(patches).toEqual([["z1", { enabled: true }]]));
  });

  it("updates every container after its backup once switched on", async () => {
    settingsOnServer = baseSettings({ containersEnabled: true, updateContainersAfterBackup: false });
    await renderPage();
    const updates = card("schedules.updates");
    const toggle = within(updates).getByRole("switch", { name: en["settings.updateEveryContainer"] });
    expect(toggle.getAttribute("aria-checked")).toBe("false");
    fireEvent.click(toggle);
    await waitFor(() => expect(putBodies.at(-1)?.updateContainersAfterBackup).toBe(true));
  });

  it("saves the switch for own plans per entry", async () => {
    await renderPage();
    fireEvent.click(screen.getByRole("switch", { name: en["schedules.perItem"] }));
    await waitFor(() => expect(putBodies.at(-1)?.perItemSchedules).toBe(true));
  });
});
