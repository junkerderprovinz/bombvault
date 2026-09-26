// @vitest-environment jsdom
// Step 3 attaches the backups through storage places: each domain's row says
// where its backups lie, and the add window connects a place no row offers yet.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { Settings } from "../lib/api";
import { placesChanged, type DomainRow, type Place } from "../lib/places";
import { stubEventSource } from "../lib/placement.testsupport";

const stored = {
  containersPath: "backups/containers",
  vmsPath: "backups/vms",
  flashPath: "backups/flash",
  filesPath: "backups/files",
  configPath: "backups/config",
  configOffsite: "",
  containersOffsite: "",
  vmsOffsite: "",
  flashOffsite: "",
  filesOffsite: "",
  encryptionEnabled: true,
} as Settings;
const puts: Settings[] = [];

vi.mock("../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/api")>()),
  getSettings: () => Promise.resolve({ ok: true, settings: { ...stored }, hostMountRoot: "/host/user" }),
  putSettings: (s: Settings) => {
    puts.push(s);
    return Promise.resolve({ ok: true, warnings: [], notes: [], kept: [] });
  },
  detectEncryption: () => Promise.resolve({ ok: true, verdict: "absent", repos: [] }),
  discover: () => Promise.resolve({ ok: true, discovered: 0 }),
  discoverVMs: () => Promise.resolve({ ok: true, discovered: 0 }),
  discoverFiles: () => Promise.resolve({ ok: true, discovered: 0 }),
  getVMSSH: () => Promise.resolve({ ok: false }),
  discoverAll: () =>
    Promise.resolve({
      containers: 1,
      vms: 0,
      files: 0,
      skipped: [],
      skippedNeedsAction: false,
      paused: [],
      leftOpen: [],
      directRepos: [],
    }),
  listContainers: () => Promise.resolve({ ok: true, containers: [{ name: "plex", lastBackup: 1_700_000_000 }] }),
  listVMs: () => Promise.resolve({ ok: true, vms: [] }),
  listFileSets: () => Promise.resolve({ ok: true, fileSets: [] }),
}));

const unraid = {
  id: "p-unraid",
  name: "Unraid",
  provider: "unraid-folder",
  enabled: true,
  folders: { containers: "containers", vms: "vms" },
} as unknown as Place;

function row(domain: string): DomainRow {
  return {
    domain,
    homePlace: "p-unraid",
    storedIn: "p-unraid",
    chips: [],
    exceptions: [],
    paused: false,
    schedule: "daily 02:00",
    unreadable: false,
  };
}

vi.mock("../lib/places", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/places")>()),
  listPlaces: () => Promise.resolve({ ok: true, places: [unraid], unplaced: [] }),
  getStorageDomains: () => Promise.resolve({ ok: true, domains: [row("containers"), row("vms")] }),
  getPlacesCatalog: () => Promise.resolve({ ok: true, providers: [] }),
}));

const { default: Recovery } = await import("./Recovery");

beforeEach(() => {
  stubEventSource();
  puts.length = 0;
});
afterEach(cleanup);

async function renderPage() {
  await act(async () => {
    render(
      <MemoryRouter>
        <I18nProvider>
          <ToastProvider>
            <Recovery />
          </ToastProvider>
        </I18nProvider>
      </MemoryRouter>
    );
  });
}

/** The step card whose heading carries this title. */
function step(title: string): HTMLElement {
  return screen.getByRole("heading", { name: new RegExp(title) }).parentElement!;
}

describe("Recovery's attach step", () => {
  it("sets where each domain's backups lie on its row and has no location to type", async () => {
    await renderPage();
    const attach = step(en["recovery.step2"]);
    expect(within(attach).getByRole("region", { name: en["nav.containers"] })).toBeTruthy();
    expect(within(attach).getByRole("region", { name: en["nav.vms"] })).toBeTruthy();
    expect(within(attach).queryAllByRole("textbox")).toEqual([]);
  });

  it("links a row's schedule to the schedules in Settings", async () => {
    await renderPage();
    const containers = within(step(en["recovery.step2"])).getByRole("region", { name: en["nav.containers"] });
    expect(within(containers).getByRole("link", { name: "Daily at 02:00" }).getAttribute("href")).toBe("/settings#schedules");
  });

  it("connects a place no row offers yet through the add window", async () => {
    await renderPage();
    await act(async () => {
      fireEvent.click(within(step(en["recovery.step2"])).getByRole("button", { name: en["places.add"] }));
    });
    expect(screen.getByRole("dialog", { name: en["places.addTitle"] })).toBeTruthy();
  });

  it("saves the encryption choice on Connect & preview and leaves every location as stored", async () => {
    await renderPage();
    fireEvent.click(screen.getByRole("switch", { name: en["settings.encryptionLabel"] }));
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["recovery.connectPreview"] }));
    });
    expect(puts).toEqual([{ ...stored, encryptionEnabled: false }]);
  });

  it("forgets what Discover found once a row moves where the backups lie", async () => {
    await renderPage();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["recovery.discover"] }));
    });
    expect(screen.getByText("plex")).toBeTruthy();
    await act(async () => {
      placesChanged();
    });
    expect(screen.queryByText("plex")).toBeNull();
    expect(screen.getByText(en["recovery.noneDiscovered"])).toBeTruthy();
  });
});
