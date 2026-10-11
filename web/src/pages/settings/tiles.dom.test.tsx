// @vitest-environment jsdom
// The tiles that are new or took cards over: Instances, Storage locations,
// Restore and System. Each page is mounted on a store the test holds, so what
// a control sends is read straight off the store's functions.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import type { ComponentType } from "react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { I18nProvider, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import type { RegistryAuthEntry, Settings } from "../../lib/api";
import { SettingsStoreContext, type SettingsStore } from "./settingsStore";

const api = vi.hoisted(() => ({
  downloadRecoveryKit: vi.fn(),
  listStorageLocations: vi.fn(),
}));

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  ...api,
}));

const { InstancesPage } = await import("./pages/InstancesPage");
const { RestorePage } = await import("./pages/RestorePage");
const { SystemPage } = await import("./pages/SystemPage");
const { SidebarCard, PageLinkCard } = await import("./SidebarTile");

const PINS = "bombvault.navPins";

function settings(over: Partial<Settings> = {}): Settings {
  return {
    containersEnabled: true,
    receiverEnabled: false,
    fleetEnabled: false,
    pullEnabled: false,
    restoreFolder: "restore",
    registryAuths: [],
    resticCacheMaxMB: 4096,
    backupCores: 0,
    exportEncryptEnabled: false,
    exportAgeRecipients: "",
    ...over,
  } as unknown as Settings;
}

const save = vi.fn();
const toggleDomainEnabled = vi.fn();
const autoSaveField = vi.fn();

function store(s: Settings): SettingsStore {
  return {
    settings: s,
    setSettings: vi.fn(),
    hostMountRoot: "/host/user",
    save,
    debouncedSave: (_key: string, run: () => void) => run(),
    fieldPulse: {},
    toggleDomainEnabled,
    domainToggleBusy: {},
    domainToggleShake: {},
    autoSaveField,
    mergedFieldBusy: {},
    mergedFieldShake: {},
    applyImportedSettings: vi.fn(),
  } as unknown as SettingsStore;
}

function Where() {
  const { pathname } = useLocation();
  return <output data-testid="where">{pathname}</output>;
}

async function renderTile(Page: ComponentType, s: Settings = settings()) {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <MemoryRouter initialEntries={["/settings"]}>
            <Routes>
              <Route
                path="*"
                element={
                  <SettingsStoreContext.Provider value={store(s)}>
                    <Page />
                    <Where />
                  </SettingsStoreContext.Provider>
                }
              />
            </Routes>
          </MemoryRouter>
        </ToastProvider>
      </I18nProvider>
    );
  });
}

function card(title: string): HTMLElement {
  const el = document.querySelector<HTMLElement>(`[data-search-card="${title}"]`);
  if (!el) throw new Error(`no card ${title}`);
  return el;
}

function entry(over: Partial<RegistryAuthEntry>): RegistryAuthEntry {
  return { host: "", username: "", token: "", tokenSet: true, kind: "", ...over };
}

beforeEach(() => {
  localStorage.setItem("bv-lang", "en");
  // A window opens as a box at desktop widths and as a sheet on a phone.
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
  save.mockResolvedValue(true);
  api.listStorageLocations.mockResolvedValue({ ok: true, locations: [] });
});

afterEach(() => {
  cleanup();
  localStorage.clear();
  for (const fn of [save, toggleDomainEnabled, autoSaveField, ...Object.values(api)]) fn.mockReset();
});

describe("the Instances tile", () => {
  it("leaves the page out of the sidebar until a module or the switch brings it in", async () => {
    await renderTile(InstancesPage);
    const pin = screen.getByRole("switch", { name: en["tile.sidebar.show"] });
    expect(pin.getAttribute("aria-checked")).toBe("false");
    fireEvent.click(pin);
    expect(JSON.parse(localStorage.getItem(PINS) ?? "{}")).toEqual({ instances: true });
    expect(screen.getByRole("switch", { name: en["tile.sidebar.show"] }).getAttribute("aria-checked")).toBe("true");
  });

  it("shows the page in the sidebar once a module is on, and lets it be taken out again", async () => {
    await renderTile(InstancesPage, settings({ receiverEnabled: true }));
    const pin = screen.getByRole("switch", { name: en["tile.sidebar.show"] });
    expect(pin.getAttribute("aria-checked")).toBe("true");
    fireEvent.click(pin);
    expect(JSON.parse(localStorage.getItem(PINS) ?? "{}")).toEqual({ instances: false });
  });

  it("holds the three module switches under the names their tabs carry", async () => {
    await renderTile(InstancesPage);
    const modules = within(card(en["tile.instances.modules"]));
    for (const key of ["instances.title", "receiver.title", "pull.title"] as const) {
      expect(modules.getByRole("switch", { name: en[key] }), key).toBeTruthy();
    }
    fireEvent.click(modules.getByRole("switch", { name: en["receiver.title"] }));
    expect(toggleDomainEnabled).toHaveBeenCalledWith("receiverEnabled", true);
  });

  it("says which modules are on in the row that opens the page", async () => {
    await renderTile(InstancesPage, settings({ fleetEnabled: true, pullEnabled: true }));
    expect(screen.getByText(`${en["instances.title"]}, ${en["pull.title"]}`)).toBeTruthy();
    cleanup();
    await renderTile(InstancesPage);
    expect(screen.getByText(en["tile.instances.none"])).toBeTruthy();
  });

  it("opens the Instances page and points to Pairing", async () => {
    await renderTile(InstancesPage);
    expect(screen.getByRole("link", { name: en["pairing.title"] }).getAttribute("href")).toBe("/settings/pairing");
    fireEvent.click(screen.getByRole("button", { name: en["instances.open"] }));
    expect(screen.getByTestId("where").textContent).toBe("/instances");
  });
});

describe("the sidebar switch of Storage locations", () => {
  function Tile() {
    return (
      <>
        <SidebarCard pin="storage" hueIndex={0} />
        <PageLinkCard to="/storage" label={en["storage.title"]} sub="" glyph={null} hueIndex={1} />
      </>
    );
  }

  it("starts on, takes the page out of the sidebar and still leads to it", async () => {
    await renderTile(Tile);
    const pin = screen.getByRole("switch", { name: en["tile.sidebar.show"] });
    expect(pin.getAttribute("aria-checked")).toBe("true");
    fireEvent.click(pin);
    expect(JSON.parse(localStorage.getItem(PINS) ?? "{}")).toEqual({ storage: false });
    fireEvent.click(screen.getByRole("button", { name: en["instances.open"] }));
    expect(screen.getByTestId("where").textContent).toBe("/storage");
  });
});

describe("the Restore tile", () => {
  const registries = () => within(card(en["settings.registriesTitle"]));
  const openPicker = () => fireEvent.click(registries().getByRole("button", { name: en["settings.registryAdd"] }));
  const dialog = () => within(screen.getByRole("dialog"));

  it("leads to the Restore page and points to Verify for the trial restore", async () => {
    await renderTile(RestorePage);
    expect(screen.getByRole("link", { name: en["activityLog.typeVerify"] }).getAttribute("href")).toBe("/settings/integrity");
    fireEvent.click(screen.getByRole("button", { name: en["instances.open"] }));
    expect(screen.getByTestId("where").textContent).toBe("/restore");
  });

  it("holds the restore folder", async () => {
    await renderTile(RestorePage);
    expect(within(card(en["tile.restore.folders"])).getByText(en["settings.restoreFolder"])).toBeTruthy();
  });

  it("downloads the recovery kit and shows why the server refused it", async () => {
    api.downloadRecoveryKit.mockResolvedValue("set a login password first");
    await renderTile(RestorePage);
    await act(async () => {
      fireEvent.click(within(card(en["tile.restore.emergency"])).getByRole("button", { name: en["recovery.download"] }));
    });
    expect(api.downloadRecoveryKit).toHaveBeenCalledTimes(1);
    expect(screen.getByText("set a login password first")).toBeTruthy();
  });

  it("has no registries card while containers are off", async () => {
    await renderTile(RestorePage, settings({ containersEnabled: false }));
    expect(document.querySelector(`[data-search-card="${en["settings.registriesTitle"]}"]`)).toBeNull();
  });

  it("says so when no registry is stored", async () => {
    await renderTile(RestorePage);
    expect(registries().getByText(en["settings.registriesEmpty"])).toBeTruthy();
  });

  it("offers the registries as tiles in three groups, with a search", async () => {
    await renderTile(RestorePage);
    openPicker();
    for (const key of ["tile.registry.public", "tile.registry.selfHosted", "tile.registry.cloud"] as const) {
      expect(dialog().getByRole("heading", { name: en[key] }), key).toBeTruthy();
    }
    expect(dialog().getByRole("button", { name: "Docker Hub docker.io" })).toBeTruthy();
    expect(dialog().getByRole("button", { name: `${en["tile.registry.other"]} ${en["tile.registry.ownAddress"]}` })).toBeTruthy();

    fireEvent.change(dialog().getByRole("searchbox"), { target: { value: "gite" } });
    expect(dialog().queryByRole("button", { name: /Docker Hub/ })).toBeNull();
    expect(dialog().getByRole("button", { name: /Gitea/ })).toBeTruthy();

    fireEvent.change(dialog().getByRole("searchbox"), { target: { value: "zzz" } });
    expect(dialog().getByText(en["tile.registry.noMatch"])).toBeTruthy();
  });

  it("goes from a tile to its form and back", async () => {
    await renderTile(RestorePage);
    openPicker();
    fireEvent.click(dialog().getByRole("button", { name: /Gitea/ }));
    expect(screen.getByRole("dialog").getAttribute("aria-label")).toBe("Add Gitea");
    expect(dialog().getByLabelText(en["settings.registryHost"]).getAttribute("placeholder")).toBe("git.example.org");
    fireEvent.click(dialog().getByRole("button", { name: en["common.back"] }));
    expect(dialog().getByRole("heading", { name: en["tile.registry.selfHosted"] })).toBeTruthy();
  });

  it("fills in the host of a public registry", async () => {
    await renderTile(RestorePage);
    openPicker();
    fireEvent.click(dialog().getByRole("button", { name: /Quay/ }));
    expect((dialog().getByLabelText(en["settings.registryHost"]) as HTMLInputElement).value).toBe("quay.io");
  });

  it("asks for a host before it saves", async () => {
    await renderTile(RestorePage);
    openPicker();
    fireEvent.click(dialog().getByRole("button", { name: /Gitea/ }));
    await act(async () => {
      fireEvent.click(dialog().getByRole("button", { name: en["settings.save"] }));
    });
    expect(save).not.toHaveBeenCalled();
    expect(dialog().getByText(en["tile.registry.hostFirst"])).toBeTruthy();
  });

  it("stores the picked kind with the entry and closes the window", async () => {
    await renderTile(RestorePage);
    openPicker();
    fireEvent.click(dialog().getByRole("button", { name: /Gitea/ }));
    fireEvent.change(dialog().getByLabelText(en["settings.registryHost"]), { target: { value: " git.home.lan " } });
    fireEvent.change(dialog().getByLabelText(en["settings.registryUser"]), { target: { value: "jdp" } });
    fireEvent.change(dialog().getByLabelText(en["settings.registryToken"]), { target: { value: "s3cret" } });
    await act(async () => {
      fireEvent.click(dialog().getByRole("button", { name: en["settings.save"] }));
    });
    expect(save.mock.calls[0][0]).toEqual({
      registryAuths: [{ host: "git.home.lan", username: "jdp", token: "s3cret", tokenSet: false, kind: "gitea" }],
    });
    // What stays on screen holds no token, only that one is stored.
    expect(save.mock.calls[0][3]()).toEqual({
      registryAuths: [{ host: "git.home.lan", username: "jdp", token: "", tokenSet: true, kind: "gitea" }],
    });
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("keeps the window open when the server refuses the entry", async () => {
    save.mockResolvedValue(false);
    await renderTile(RestorePage);
    openPicker();
    fireEvent.click(dialog().getByRole("button", { name: /Quay/ }));
    await act(async () => {
      fireEvent.click(dialog().getByRole("button", { name: en["settings.save"] }));
    });
    expect(save).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("dialog")).not.toBeNull();
  });

  it("names a self-hosted registry by its kind, a public one by its host and an unknown one by its address", async () => {
    await renderTile(
      RestorePage,
      settings({
        registryAuths: [
          entry({ host: "git.home.lan", username: "jdp", kind: "gitea" }),
          entry({ host: "ghcr.io" }),
          entry({ host: "registry.home.lan" }),
        ],
      })
    );
    const rows = registries().getAllByRole("listitem");
    expect(rows.map((row) => row.querySelector("span.font-semibold")?.textContent)).toEqual([
      "Gitea",
      "GitHub",
      "registry.home.lan",
    ]);
    expect(rows[0].textContent).toContain("git.home.lan");
    expect(rows[0].textContent).toContain("User jdp");
    expect(rows[0].textContent).toContain(en["tile.registry.tokenSaved"]);
    expect(rows[0].querySelector("img")).not.toBeNull();
    expect(rows[2].querySelector("img")).toBeNull();
  });

  it("edits an entry without asking for its token again, and keeps its kind", async () => {
    const stored = [entry({ host: "git.home.lan", username: "jdp", kind: "gitea" }), entry({ host: "ghcr.io" })];
    await renderTile(RestorePage, settings({ registryAuths: stored }));
    fireEvent.click(registries().getAllByRole("button", { name: en["common.edit"] })[0]);
    expect(screen.getByRole("dialog").getAttribute("aria-label")).toBe("Edit Gitea");
    expect(dialog().queryByRole("button", { name: en["common.back"] })).toBeNull();
    expect(dialog().getByText(en["tile.registry.keepToken"])).toBeTruthy();
    fireEvent.change(dialog().getByLabelText(en["settings.registryUser"]), { target: { value: "other" } });
    await act(async () => {
      fireEvent.click(dialog().getByRole("button", { name: en["settings.save"] }));
    });
    expect(save.mock.calls[0][0]).toEqual({
      registryAuths: [entry({ host: "git.home.lan", username: "other", kind: "gitea" }), stored[1]],
    });
  });

  it("removes an entry at once", async () => {
    const stored = [entry({ host: "git.home.lan", kind: "gitea" }), entry({ host: "ghcr.io" })];
    await renderTile(RestorePage, settings({ registryAuths: stored }));
    await act(async () => {
      fireEvent.click(registries().getAllByRole("button", { name: en["settings.registryRemove"] })[0]);
    });
    expect(save.mock.calls[0][0]).toEqual({ registryAuths: [stored[1]] });
  });
});

describe("the System tile", () => {
  it("holds the cache limit and the cores as Performance, and saves a number typed into them", async () => {
    await renderTile(SystemPage);
    const performance = within(card(en["tile.system.performance"]));
    const cache = performance.getByLabelText(en["settings.cacheLimitLabel"]) as HTMLInputElement;
    expect(cache.value).toBe("4096");
    fireEvent.change(cache, { target: { value: "2048" } });
    expect(save.mock.calls[0][0]).toEqual({ resticCacheMaxMB: 2048 });
    fireEvent.change(performance.getByLabelText(en["settings.coresLabel"]), { target: { value: "-3" } });
    expect(save.mock.calls[1][0]).toEqual({ backupCores: 0 });
  });

  it("protects an export with a key from the export card", async () => {
    await renderTile(SystemPage);
    const data = within(card(en["settingsIO.title"]));
    fireEvent.click(data.getByRole("switch", { name: en["export.encrypt.enable"] }));
    expect(autoSaveField.mock.calls[0].slice(0, 2)).toEqual(["exportEncryptEnabled", true]);
    expect(data.queryByText(en["export.encrypt.recipients"])).toBeNull();
  });

  it("asks for the recipients while the protection is on", async () => {
    await renderTile(SystemPage, settings({ exportEncryptEnabled: true }));
    const data = within(card(en["settingsIO.title"]));
    expect(data.getByText(en["export.encrypt.recipients"])).toBeTruthy();
    expect(data.getByText(en["export.encrypt.recipientsRequired"])).toBeTruthy();
  });

  it("leaves the bug report to the Info page", async () => {
    await renderTile(SystemPage);
    expect(screen.queryByText(en["tile.info.bugReport"])).toBeNull();
  });
});
