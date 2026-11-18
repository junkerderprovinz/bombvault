// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { I18nProvider, en } from "../../lib/i18n";
import type { RegistryAuthEntry, Settings } from "../../lib/api";
import { SettingsStoreContext, type SettingsStore } from "./settingsStore";
import { RegistriesCard } from "./RegistriesCard";

afterEach(cleanup);

function show(registryAuths: RegistryAuthEntry[] = []) {
  const save = vi.fn().mockResolvedValue(true);
  const store = { settings: { registryAuths } as unknown as Settings, save } as unknown as SettingsStore;
  render(
    <I18nProvider>
      <SettingsStoreContext.Provider value={store}>
        <RegistriesCard hueIndex={0} />
      </SettingsStoreContext.Provider>
    </I18nProvider>,
  );
  return save;
}

const saved = (save: ReturnType<typeof vi.fn>) => (save.mock.calls.at(-1)?.[0] as Pick<Settings, "registryAuths">).registryAuths;

describe("RegistriesCard", () => {
  it("adds a registry of the picked kind at its own host", async () => {
    const save = show();
    fireEvent.click(screen.getByRole("button", { name: en["settings.registryAdd"] }));
    const win = screen.getByRole("dialog");
    fireEvent.click(within(win).getByRole("button", { name: "GitHub ghcr.io" }));
    expect((within(win).getByLabelText(en["settings.registryHost"]) as HTMLInputElement).value).toBe("ghcr.io");
    fireEvent.change(within(win).getByLabelText(en["settings.registryUser"]), { target: { value: "octo" } });
    await act(async () => fireEvent.click(within(win).getByRole("button", { name: en["settings.save"] })));
    expect(saved(save)).toEqual([{ host: "ghcr.io", username: "octo", token: "", tokenSet: false, kind: "ghcr" }]);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("goes back from the form to the tiles and asks for a host before it saves", async () => {
    const save = show();
    fireEvent.click(screen.getByRole("button", { name: en["settings.registryAdd"] }));
    const win = screen.getByRole("dialog");
    fireEvent.click(within(win).getByRole("button", { name: `${en["tile.registry.other"]} ${en["tile.registry.ownAddress"]}` }));
    await act(async () => fireEvent.click(within(win).getByRole("button", { name: en["settings.save"] })));
    expect(within(win).getByText(en["tile.registry.hostFirst"])).toBeTruthy();
    expect(save).not.toHaveBeenCalled();
    fireEvent.click(within(win).getByRole("button", { name: en["common.back"] }));
    expect(within(win).getByRole("searchbox", { name: en["tile.registry.search"] })).toBeTruthy();
  });

  it("edits an entry and keeps its kind", async () => {
    const entry = { host: "registry.example.org", username: "me", token: "", tokenSet: true, kind: "harbor" };
    const save = show([entry]);
    fireEvent.click(screen.getByRole("button", { name: en["common.edit"] }));
    const win = screen.getByRole("dialog");
    fireEvent.change(within(win).getByLabelText(en["settings.registryUser"]), { target: { value: "you" } });
    await act(async () => fireEvent.click(within(win).getByRole("button", { name: en["settings.save"] })));
    expect(saved(save)).toEqual([{ ...entry, username: "you" }]);
  });
});
