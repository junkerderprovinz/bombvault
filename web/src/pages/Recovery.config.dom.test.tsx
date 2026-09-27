// @vitest-environment jsdom
// Restoring BombVault's own settings reads from the place the Self-Backup row
// names. A place owns the config path, so step 2 shows it and has nothing to type.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { OffsiteTarget, Settings } from "../lib/api";
import { placesChanged, type Place } from "../lib/places";
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
const restores: [string, string | undefined][] = [];

function place(id: string, name: string, provider: string, homeDomains: string[]): Place {
  return { id, name, provider, enabled: true, folders: {}, usage: { homeDomains } } as unknown as Place;
}

let places: Place[] = [];
let configTargets: OffsiteTarget[] = [];

vi.mock("../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/api")>()),
  getSettings: () => Promise.resolve({ ok: true, settings: { ...stored }, hostMountRoot: "/host/user" }),
  putSettings: (s: Settings) => {
    puts.push(s);
    return Promise.resolve({ ok: true, warnings: [], notes: [], kept: [] });
  },
  restoreConfig: (snapshot: string, source?: string) => {
    restores.push([snapshot, source]);
    return Promise.resolve({ ok: true, staged: true, autoRestart: false });
  },
  listOffsiteTargets: () => Promise.resolve({ ok: true, targets: configTargets }),
  getVMSSH: () => Promise.resolve({ ok: false }),
}));

vi.mock("../lib/places", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/places")>()),
  listPlaces: () => Promise.resolve({ ok: true, places, unplaced: [] }),
}));

const { default: Recovery } = await import("./Recovery");

beforeEach(() => {
  stubEventSource();
  puts.length = 0;
  restores.length = 0;
  stored.configPath = "backups/config";
  stored.configOffsite = "";
  places = [place("p-unraid", "Unraid", "unraid-folder", ["config", "containers"]), place("p-b2", "B2", "b2", [])];
  configTargets = [];
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

function configStep(): HTMLElement {
  return screen.getByRole("heading", { name: new RegExp(en["recovery.stepConfig"]) }).parentElement!;
}

async function restore() {
  await act(async () => {
    fireEvent.click(within(configStep()).getByRole("button", { name: en["recovery.configRestore"] }));
  });
}

async function offsite() {
  await act(async () => {
    fireEvent.click(within(configStep()).getByRole("tab", { name: en["source.offsite"] }));
  });
}

describe("Recovery's own-settings restore", () => {
  it("shows the Self-Backup row's place and path, has nothing to type and restores from there", async () => {
    await renderPage();
    const step = configStep();
    expect(within(step).getByText("Unraid")).toBeTruthy();
    expect(within(step).getByText("backups/config")).toBeTruthy();
    expect(within(step).getByText(en["recovery.configOtherHome"])).toBeTruthy();
    expect(within(step).queryAllByRole("textbox")).toEqual([]);
    await restore();
    expect(puts).toEqual([]);
    expect(restores).toEqual([["latest", undefined]]);
  });

  it("follows the Self-Backup row when it moves to another place", async () => {
    await renderPage();
    stored.configPath = "places/nas/config";
    places = [place("p-unraid", "Unraid", "unraid-folder", ["containers"]), place("p-nas", "NAS", "synology", ["config"])];
    await act(async () => {
      placesChanged();
    });
    const step = configStep();
    expect(within(step).getByText("NAS")).toBeTruthy();
    expect(within(step).getByText("places/nas/config")).toBeTruthy();
    expect(within(step).queryByText("backups/config")).toBeNull();
  });

  it("restores from the Self-Backup's copy it shows", async () => {
    stored.configOffsite = "b2:bucket/config";
    configTargets = [
      { id: "t-hz", name: "Hetzner", sortOrder: 1, placeId: "", enabled: true } as OffsiteTarget,
      { id: "t-b2", name: "B2 target", sortOrder: 0, placeId: "p-b2", enabled: true } as OffsiteTarget,
    ];
    await renderPage();
    await offsite();
    const step = configStep();
    expect(within(step).getByText("B2")).toBeTruthy();
    expect(within(step).getByText("b2:bucket/config")).toBeTruthy();
    expect(within(step).getByText(en["recovery.configOtherCopy"])).toBeTruthy();
    await restore();
    expect(restores).toEqual([["latest", "offsite:t-b2"]]);
  });

  it("says where to add a copy and restores nothing off-site while the Self-Backup has none", async () => {
    await renderPage();
    await offsite();
    const step = configStep();
    expect(within(step).getByText(en["recovery.configNoCopy"])).toBeTruthy();
    const button = within(step).getByRole("button", { name: en["recovery.configRestore"] }) as HTMLButtonElement;
    expect(button.disabled).toBe(true);
  });
});
