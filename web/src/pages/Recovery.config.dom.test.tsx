// @vitest-environment jsdom
// Restoring BombVault's own settings saves the config location first. When a
// storage place keeps the stored one, the restore must not run from a location
// the user did not type.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { Settings } from "../lib/api";
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
const restores: string[] = [];
let kept: string[] = [];

vi.mock("../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/api")>()),
  getSettings: () => Promise.resolve({ ok: true, settings: { ...stored }, hostMountRoot: "/host/user" }),
  putSettings: (s: Settings) => {
    puts.push(s);
    return Promise.resolve({ ok: true, warnings: [], notes: [], kept });
  },
  restoreConfig: (snapshot: string) => {
    restores.push(snapshot);
    return Promise.resolve({ ok: true, staged: true, autoRestart: false });
  },
  getVMSSH: () => Promise.resolve({ ok: false }),
}));

vi.mock("../lib/places", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/places")>()),
  listPlaces: () => Promise.resolve({ ok: true, places: [], unplaced: [] }),
}));

const { default: Recovery } = await import("./Recovery");

beforeEach(() => {
  stubEventSource();
  puts.length = 0;
  restores.length = 0;
  kept = [];
});
afterEach(cleanup);

async function restoreFrom(path: string) {
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
  fireEvent.change(screen.getByDisplayValue("backups/config"), { target: { value: path } });
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: en["recovery.configRestore"] }));
  });
}

describe("Recovery's own-settings restore", () => {
  it("restores nothing and shows the stored path again when a storage place keeps it", async () => {
    kept = ["configPath"];
    await restoreFrom("backups/old-config");
    expect(puts[0]?.configPath).toBe("backups/old-config");
    expect(restores).toEqual([]);
    expect(screen.getByDisplayValue("backups/config")).toBeTruthy();
    expect(screen.queryByDisplayValue("backups/old-config")).toBeNull();
    expect(screen.getAllByText(en["recovery.placeKept"]).length).toBeGreaterThan(0);
  });

  it("stages the restore from the typed path when the save keeps it", async () => {
    await restoreFrom("backups/old-config");
    expect(restores).toEqual(["latest"]);
    expect(screen.getByDisplayValue("backups/old-config")).toBeTruthy();
    expect(screen.queryByText(en["recovery.placeKept"])).toBeNull();
  });
});
