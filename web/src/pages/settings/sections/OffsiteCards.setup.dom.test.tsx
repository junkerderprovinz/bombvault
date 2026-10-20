// @vitest-environment jsdom
// The setup window of a section's off-site copy saves a location typed into it
// before it closes, as the section's own off-site window does.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, en } from "../../../lib/i18n";
import { ToastProvider } from "../../../lib/toast";
import type { Settings } from "../../../lib/api";
import { SettingsStoreContext, type SettingsStore } from "../settingsStore";

const fake = await vi.hoisted(async () => (await import("../../../lib/placement.testsupport")).createPlacementApi());

vi.mock("../../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../../lib/api")>()),
  ...fake.api,
  listOffsiteTargets: () => Promise.resolve({ ok: true, targets: [] }),
  getCloudCredSets: () => Promise.resolve({ ok: true, sets: [] }),
  getCloud: () => Promise.resolve({ ok: true, s3KeyId: "", s3Region: "", restUser: "", s3StorageClass: "", s3SecretSet: false, restPasswordSet: false }),
}));

const { OffsiteCards } = await import("./OffsiteCards");

const LOCATION = "rest:http://nas:8000/bombvault-containers/containers";

function settings(): Settings {
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
  } as unknown as Settings;
}

function store(save: SettingsStore["save"]): SettingsStore {
  return {
    settings: settings(),
    setSettings: vi.fn(),
    savedBaseline: { current: null },
    allTargets: [],
    save,
    debouncedSave: vi.fn(),
  } as unknown as SettingsStore;
}

async function openSetup(save: SettingsStore["save"]) {
  await act(async () => {
    render(
      <MemoryRouter>
        <I18nProvider>
          <ToastProvider>
            <SettingsStoreContext.Provider value={store(save)}>
              <OffsiteCards nextHue={() => 0} />
            </SettingsStoreContext.Provider>
          </ToastProvider>
        </I18nProvider>
      </MemoryRouter>
    );
  });
  fireEvent.click(screen.getAllByRole("button", { name: en["offsite.wizard.setup"] })[0]);
  const field = await screen.findByPlaceholderText(en["offsite.wizard.repoUrlPlaceholder"]);
  fireEvent.change(field, { target: { value: LOCATION } });
  // Pressing Close takes the focus from the field first, which starts the save.
  fireEvent.blur(field);
  fireEvent.click(screen.getByRole("button", { name: en["offsite.wizard.close"] }));
}

describe("the setup window of an off-site copy", () => {
  beforeEach(() => {
    localStorage.setItem("bv-lang", "en");
    fake.reset();
  });
  afterEach(() => {
    cleanup();
    localStorage.clear();
  });

  it("saves a location typed before Close once the question is answered, then closes", async () => {
    const save = vi.fn().mockResolvedValue(true);
    await openSetup(save);
    fireEvent.click(await screen.findByRole("button", { name: "Confirm" }));
    await waitFor(() => expect(screen.queryByPlaceholderText(en["offsite.wizard.repoUrlPlaceholder"])).toBeNull());
    expect(save).toHaveBeenCalledTimes(1);
    expect(save.mock.calls[0][0]).toEqual({ containersOffsite: LOCATION });
  });

  it("stays open on the typed location when the save fails", async () => {
    const save = vi.fn().mockResolvedValue(false);
    await openSetup(save);
    fireEvent.click(await screen.findByRole("button", { name: "Confirm" }));
    await waitFor(() => expect(save).toHaveBeenCalledTimes(1));
    expect(screen.getByPlaceholderText<HTMLInputElement>(en["offsite.wizard.repoUrlPlaceholder"]).value).toBe(LOCATION);
  });
});
