// @vitest-environment jsdom
// The off-site settings offer one switch for an instance that keeps every
// backup on the premises, and only while no section copies off-site. While it
// is on, the settings for copies that do not exist step aside.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, en } from "../../../lib/i18n";
import { ToastProvider } from "../../../lib/toast";
import type { OffsiteTarget, Settings } from "../../../lib/api";
import { SettingsStoreContext, type SettingsStore } from "../settingsStore";

vi.mock("../../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../../lib/api")>()),
  listOffsiteTargets: () => Promise.resolve({ ok: true, targets: [] }),
}));

const { OffsiteCards } = await import("./OffsiteCards");

function settings(over: Partial<Settings> = {}): Settings {
  return {
    containersOffsite: "",
    vmsOffsite: "",
    flashOffsite: "",
    filesOffsite: "",
    zfsOffsite: "",
    configOffsite: "",
    compression: {},
    offsiteLimitUpload: 0,
    offsiteLimitDownload: 0,
    noOffsiteByChoice: false,
    ...over,
  } as unknown as Settings;
}

function target(over: Partial<OffsiteTarget>): OffsiteTarget {
  return { id: "t1", domain: "flash", name: "Storage Box", repo: "rest:http://box:8000/flash", enabled: true, sortOrder: 0, ...over } as OffsiteTarget;
}

async function show(over: Partial<Settings> = {}, allTargets: OffsiteTarget[] = []) {
  const autoSaveToggle = vi.fn().mockResolvedValue(true);
  const store = {
    settings: settings(over),
    setSettings: vi.fn(),
    savedBaseline: { current: null },
    allTargets,
    save: vi.fn().mockResolvedValue(true),
    debouncedSave: vi.fn(),
    autoSaveToggle,
    fieldBusy: {},
    fieldShake: {},
  } as unknown as SettingsStore;
  await act(async () => {
    render(
      <MemoryRouter>
        <I18nProvider>
          <ToastProvider>
            <SettingsStoreContext.Provider value={store}>
              <OffsiteCards nextHue={() => 0} />
            </SettingsStoreContext.Provider>
          </ToastProvider>
        </I18nProvider>
      </MemoryRouter>
    );
  });
  return autoSaveToggle;
}

const choice = () => screen.queryByRole("switch", { name: en["offsite.byChoice"] });

describe("the switch that keeps the data on the premises", () => {
  beforeEach(() => localStorage.setItem("bv-lang", "en"));
  afterEach(() => {
    cleanup();
    localStorage.clear();
  });

  it("is there while no section copies off-site, and saves the choice", async () => {
    const autoSaveToggle = await show();

    expect(choice()!.getAttribute("aria-checked")).toBe("false");
    fireEvent.click(choice()!);
    expect(autoSaveToggle).toHaveBeenCalledWith("noOffsiteByChoice", true, expect.any(Function), expect.any(Function));
  });

  it("shows a choice that is on", async () => {
    await show({ noOffsiteByChoice: true });
    expect(choice()!.getAttribute("aria-checked")).toBe("true");
  });

  it("hides the section rows and the bandwidth limits while it is on, and stays itself", async () => {
    await show({ noOffsiteByChoice: true });

    expect(screen.queryAllByRole("button", { name: en["offsite.wizard.setup"] })).toHaveLength(0);
    expect(screen.queryByText(en["settings.offsiteLimits"])).toBeNull();
    expect(screen.queryByLabelText(en["settings.limitUpload"])).toBeNull();
    expect(choice()!.getAttribute("aria-checked")).toBe("true");
  });

  it("shows the section rows and the bandwidth limits while it is off", async () => {
    await show();

    expect(screen.getAllByRole("button", { name: en["offsite.wizard.setup"] })).toHaveLength(6);
    expect(screen.getByText(en["settings.offsiteLimits"])).toBeTruthy();
    expect(choice()).not.toBeNull();
  });

  it("goes away once a section's off-site field holds a location", async () => {
    await show({ noOffsiteByChoice: true, containersOffsite: "rest:http://box:8000/containers" });
    expect(choice()).toBeNull();
  });

  it("goes away once a section has a switched-on off-site target", async () => {
    await show({}, [target({})]);
    expect(choice()).toBeNull();
  });

  it("stays while the only target is switched off", async () => {
    await show({ flashOffsite: "rest:http://box:8000/flash" }, [target({ enabled: false })]);
    expect(choice()).not.toBeNull();
  });
});
