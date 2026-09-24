// @vitest-environment jsdom
// The switch that stops every database dump at once sits with the domains, and
// switching it off has to name the databases whose dump is the only consistent
// copy there is, by name, before it is too late to notice.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { Container, Settings } from "../lib/api";

function baseSettings(over: Partial<Settings> = {}): Settings {
  return {
    encryptionEnabled: true,
    containersEnabled: true,
    vmsEnabled: false,
    flashEnabled: false,
    filesEnabled: false,
    configEnabled: false,
    receiverEnabled: false,
    fleetEnabled: false,
    pullEnabled: false,
    dbDumpsEnabled: true,
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

function container(over: Partial<Container> = {}): Container {
  return {
    name: "immich_postgres",
    image: "postgres:16",
    state: "running",
    status: "Up",
    ip: "",
    installed: true,
    includeInSchedule: true,
    lastBackup: null,
    lastBackupStarted: null,
    preHook: "",
    postHook: "",
    stopContainers: [],
    excludes: [],
    lastUpdateCheck: 0,
    lastUpdateResult: "",
    stack: "",
    dbEngine: "postgres",
    dbSuggestedEngine: "",
    dbTier: "curated",
    dbDumpOff: false,
    dbDumpEngine: "",
    dbDumpLabelOff: false,
    dbDumpsGlobalOff: false,
    dbDataCoverage: "live",
    dbDumpHookOverlap: false,
    ...over,
  };
}

const putBodies: Settings[] = [];
let settingsOnServer = baseSettings();
let containersOnServer: Container[] = [];

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
    listContainers: () => Promise.resolve({ ok: true, containers: containersOnServer }),
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

async function renderGeneralTab() {
  await act(async () => {
    window.location.hash = "#general";
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

function dumpsToggle() {
  return screen.getAllByRole("switch", { name: en["settings.dbDumps"] })[0];
}

beforeEach(() => {
  stubMatchMedia();
  stubResizeObserver();
  putBodies.length = 0;
  settingsOnServer = baseSettings();
  containersOnServer = [];
});

afterEach(cleanup);

describe("the switch for every database dump", () => {
  it("sits with the domain switches", async () => {
    await renderGeneralTab();

    const card = dumpsToggle().closest("div.rounded-card");
    expect(card).toBeTruthy();
    expect(within(card as HTMLElement).getByRole("switch", { name: en["settings.containersEnabled"] })).toBeTruthy();
  });

  it("names the databases that are left with no consistent copy", async () => {
    containersOnServer = [
      container({ name: "immich_postgres", dbDataCoverage: "live" }),
      container({ name: "nextcloud_db", dbDataCoverage: "none" }),
      container({ name: "paperless_db", dbDataCoverage: "stopped" }),
    ];
    await renderGeneralTab();

    fireEvent.click(dumpsToggle());
    const question = await screen.findByText(/immich_postgres/);
    expect(question.textContent).toContain("nextcloud_db");
    expect(question.textContent).not.toContain("paperless_db");
    expect(putBodies).toHaveLength(0);
  });

  it("names only the databases whose dump runs and is the only consistent copy", async () => {
    containersOnServer = [
      container({ name: "immich_postgres", dbDataCoverage: "live" }),
      container({ name: "switched_off_db", dbDataCoverage: "live", dbDumpOff: true }),
      container({ name: "label_off_db", dbDataCoverage: "none", dbDumpLabelOff: true }),
      container({
        name: "unconfirmed_db",
        dbTier: "lookalike",
        dbEngine: "",
        dbSuggestedEngine: "mysql",
        dbDataCoverage: "live",
      }),
      container({
        name: "confirmed_db",
        dbTier: "lookalike",
        dbEngine: "",
        dbSuggestedEngine: "mysql",
        dbDumpEngine: "mysql",
        dbDataCoverage: "none",
      }),
      container({ name: "unknown_db", dbDataCoverage: "unknown" }),
      container({ name: "by_label_db", dbTier: "label", dbDumpOff: true, dbDataCoverage: "live" }),
    ];
    await renderGeneralTab();

    fireEvent.click(dumpsToggle());
    const question = await screen.findByText(/immich_postgres/);
    expect(question.textContent).toContain("confirmed_db");
    expect(question.textContent).toContain("by_label_db");
    for (const quiet of ["switched_off_db", "label_off_db", "unconfirmed_db", "unknown_db"]) {
      expect(question.textContent).not.toContain(quiet);
    }
  });

  it("asks plainly when no database depends on its dump", async () => {
    containersOnServer = [container({ dbDataCoverage: "stopped" })];
    await renderGeneralTab();

    fireEvent.click(dumpsToggle());
    expect(await screen.findByText(en["settings.dbDumpsOffConfirmPlain"])).toBeTruthy();
  });

  it("saves the setting once the question is answered", async () => {
    await renderGeneralTab();

    fireEvent.click(dumpsToggle());
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: en["common.confirm"] }));

    await waitFor(() => expect(putBodies).toHaveLength(1));
    expect(putBodies[0].dbDumpsEnabled).toBe(false);
  });

  it("leaves the switch on when the question is declined", async () => {
    await renderGeneralTab();

    fireEvent.click(dumpsToggle());
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: en["common.cancel"] }));

    await waitFor(() => expect(dumpsToggle().getAttribute("aria-checked")).toBe("true"));
    expect(putBodies).toHaveLength(0);
  });

  it("switches back on without asking", async () => {
    settingsOnServer = baseSettings({ dbDumpsEnabled: false });
    await renderGeneralTab();

    fireEvent.click(dumpsToggle());
    await waitFor(() => expect(putBodies).toHaveLength(1));
    expect(putBodies[0].dbDumpsEnabled).toBe(true);
  });
});
