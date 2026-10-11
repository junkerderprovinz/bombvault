// @vitest-environment jsdom
/**
 * The Settings tab of an entry's page.
 *
 * Every kind gets the card for its schedule and storage locations, and then
 * the cards of its own kind and no other. A switch has to reach the call of
 * the kind, and schedule and storage locations follow the default until the
 * entry holds a value of its own.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import { ENTRY_ROUTE } from "../lib/backupEntry";
import { I18nProvider, en } from "../lib/i18n";
import { AnomalyProvider } from "../lib/useAnomalies";
import {
  PLACEMENT,
  SET_ID,
  ZFS_ID,
  containerView,
  fileSetView,
  placementOptions,
  serveEntries,
  vmView,
} from "./backupEntry.testsupport";

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listItems: vi.fn(),
    listContainers: vi.fn(),
    listVMs: vi.fn(),
    listFileSets: vi.fn(),
    listZFSDatasets: vi.fn(),
    listItemRuns: vi.fn(),
    listRuns: vi.fn(),
    getSettings: vi.fn(),
    getAnomalies: vi.fn(),
    getAnomalySummary: vi.fn(),
    getAnomalyItems: vi.fn(),
    getTimeline: vi.fn(),
    getPlacementOptions: vi.fn(),
    setItemPlacement: vi.fn(),
    previewItemPlacement: vi.fn(),
    getContainerMounts: vi.fn(),
    getZFSReplica: vi.fn(),
    setInclude: vi.fn(),
    setVMInclude: vi.fn(),
    patchZFSDataset: vi.fn(),
    setVMBlockBackup: vi.fn(),
    setUpdateAfterBackup: vi.fn(),
  };
});

vi.mock("../lib/toast", () => ({ useToast: () => ({ push: () => {} }) }));

const api = await import("../lib/api");
const mocks = {
  listItems: vi.mocked(api.listItems),
  listContainers: vi.mocked(api.listContainers),
  listVMs: vi.mocked(api.listVMs),
  listFileSets: vi.mocked(api.listFileSets),
  listZFSDatasets: vi.mocked(api.listZFSDatasets),
  listItemRuns: vi.mocked(api.listItemRuns),
  getSettings: vi.mocked(api.getSettings),
};
const setItemPlacement = vi.mocked(api.setItemPlacement);
const setInclude = vi.mocked(api.setInclude);
const patchZFSDataset = vi.mocked(api.patchZFSDataset);
const setVMBlockBackup = vi.mocked(api.setVMBlockBackup);
const setUpdateAfterBackup = vi.mocked(api.setUpdateAfterBackup);

const { BackupEntryPage } = await import("./BackupEntry");

window.matchMedia = ((query: string) => ({
  matches: true,
  media: query,
  onchange: null,
  addListener: () => {},
  removeListener: () => {},
  addEventListener: () => {},
  removeEventListener: () => {},
  dispatchEvent: () => false,
})) as unknown as typeof window.matchMedia;

class NoopEventSource {
  onmessage: unknown = null;
  close() {}
}

async function renderSettings(entry: string) {
  await act(async () => {
    render(
      <I18nProvider>
        <MemoryRouter initialEntries={[`/backups/${entry}?tab=settings`]}>
          <AnomalyProvider>
            <Routes>
              <Route path={ENTRY_ROUTE} element={<BackupEntryPage />} />
            </Routes>
          </AnomalyProvider>
        </MemoryRouter>
      </I18nProvider>
    );
  });
}

async function press(el: HTMLElement) {
  await act(async () => {
    fireEvent.click(el);
  });
}

const cardTitles = () => screen.getAllByRole("heading", { level: 2 }).map((h) => h.textContent);
const card = (title: string) => screen.getByRole("heading", { level: 2, name: new RegExp(`^${title}`) }).parentElement!;

beforeEach(() => {
  localStorage.clear();
  vi.stubGlobal("EventSource", NoopEventSource);
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => ({ ok: true, json: async () => ({ ok: false }) }))
  );
  serveEntries(mocks);
  vi.mocked(api.listRuns).mockResolvedValue({ ok: true, runs: [] });
  vi.mocked(api.getAnomalies).mockResolvedValue({ ok: true, anomalies: [], nextCursor: "" });
  vi.mocked(api.getAnomalySummary).mockResolvedValue({ ok: false });
  vi.mocked(api.getAnomalyItems).mockResolvedValue({ ok: true, items: [] });
  vi.mocked(api.getTimeline).mockResolvedValue({ ok: true, places: [], rows: [] });
  vi.mocked(api.getPlacementOptions).mockImplementation(async (domain) => ({ ok: true, options: placementOptions(domain) }));
  vi.mocked(api.getContainerMounts).mockResolvedValue({ ok: false } as never);
  vi.mocked(api.getZFSReplica).mockRejectedValue(new Error("no replica"));
  setItemPlacement.mockResolvedValue({ ok: true, placement: PLACEMENT } as never);
  vi.mocked(api.previewItemPlacement).mockResolvedValue({ ok: true, added: [] });
  for (const call of [setInclude, patchZFSDataset, setVMBlockBackup, setUpdateAfterBackup]) {
    call.mockResolvedValue({ ok: true } as never);
  }
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

describe("the cards of each kind", () => {
  it("gives a container its own cards after the two every kind has", async () => {
    await renderSettings("container/immich");
    await waitFor(() => expect(cardTitles()).toContain(en["entry.settings.repository"]));
    expect(cardTitles()).toEqual([
      en["entry.settings.schedule"],
      en["entry.settings.repository"],
      en["entry.settings.container"],
      en["folders.title"],
      en["excludes.title"],
      en["stophook.title"],
      en["entry.settings.hooks"],
    ]);
  });

  it("adds the database card to a container that is one", async () => {
    serveEntries(mocks, { containers: [containerView({ dbEngine: "postgres", dbTier: "curated", dbDataCoverage: "stopped" })] });
    await renderSettings("container/immich");
    expect(within(card(en["entry.settings.database"])).getByRole("switch", { name: en["dbdump.toggle"] })).toBeTruthy();
  });

  it("draws beside the switch how a backup of the database runs, with the stop while the files are read", async () => {
    serveEntries(mocks, { containers: [containerView({ dbEngine: "postgres", dbTier: "curated", dbDataCoverage: "stopped" })] });
    await renderSettings("container/immich");
    const database = within(card(en["entry.settings.database"]));
    expect(database.getByRole("img", { name: en["entry.dump.label"] })).toBeTruthy();
    expect(database.getByText(en["entry.dump.twoPoints"])).toBeTruthy();
    expect(database.getByText(en["entry.dump.paused"])).toBeTruthy();
    expect(database.getByText("database.sql", { selector: "div" })).toBeTruthy();
  });

  it("draws the one restore point that is left while the dump is off, and nothing that moves", async () => {
    serveEntries(mocks, {
      containers: [containerView({ dbEngine: "postgres", dbTier: "curated", dbDataCoverage: "stopped", dbDumpOff: true })],
    });
    await renderSettings("container/immich");
    const database = within(card(en["entry.settings.database"]));
    const picture = database.getByRole("img", { name: en["entry.dump.labelWithout"] });
    expect(database.getByText(en["entry.dump.none"])).toBeTruthy();
    expect(database.getByText(en["entry.dump.without"])).toBeTruthy();
    expect(database.queryByText(en["entry.dump.paused"])).toBeNull();
    expect(picture.querySelector('[class*="glim-dump-"]')).toBeNull();
  });

  it("leaves the container running in the picture where its files are read live", async () => {
    serveEntries(mocks, { containers: [containerView({ dbEngine: "postgres", dbTier: "curated", dbDataCoverage: "live" })] });
    await renderSettings("container/immich");
    expect(within(card(en["entry.settings.database"])).queryByText(en["entry.dump.paused"])).toBeNull();
  });

  it("names what a container was called before, with the way to unlink it", async () => {
    serveEntries(mocks, { containers: [containerView({ aliases: ["immich-old"] })] });
    await renderSettings("container/immich");
    const former = within(card(en["entry.settings.formerNames"]));
    expect(former.getByRole("button", { name: en["takeover.unlinkName"].replace("{old}", "immich-old") })).toBeTruthy();
  });

  it("keeps only the schedule and the repository of a container that is gone", async () => {
    await renderSettings("container/pihole");
    await waitFor(() => expect(cardTitles()).toContain(en["entry.settings.repository"]));
    expect(cardTitles()).toEqual([en["entry.settings.schedule"], en["entry.settings.repository"]]);
    expect(screen.getByLabelText(en["entry.settings.inScheduleGone"])).toBeTruthy();
  });

  it("gives a VM its method and its block switch", async () => {
    serveEntries(mocks, { vms: [vmView({ method: "live", blockBackup: false })] });
    await renderSettings("vm/win11");
    const vm = within(card(en["entry.settings.vm"]));
    expect(vm.getByText(en["vm.method"])).toBeTruthy();
    await press(vm.getByRole("switch", { name: en["vm.blocks"] }));
    expect(setVMBlockBackup).toHaveBeenCalledWith("win11", true);
  });

  it("gives a folder set its folder, with the dialog that edits it", async () => {
    await renderSettings(`files/${SET_ID}`);
    const folder = within(card(en["files.path"]));
    expect(folder.getByText("/host/user/photos")).toBeTruthy();
    await press(folder.getByRole("button", { name: en["common.edit"] }));
    expect(screen.getByRole("dialog")).toBeTruthy();
  });

  it("says of a folder set without a folder that it has none", async () => {
    serveEntries(mocks, { fileSets: [fileSetView({ path: "" })] });
    await renderSettings(`files/${SET_ID}`);
    expect(within(card(en["files.path"])).getByText(en["files.noPath"])).toBeTruthy();
  });

  it("gives a ZFS entry its datasets and no storage location bar", async () => {
    await renderSettings(`zfs/${ZFS_ID}`);
    expect(within(card(en["entry.settings.zfs"])).getByText(en["zfs.children"])).toBeTruthy();
    expect(screen.queryByText(en["placement.title"])).toBeNull();
    expect(cardTitles()).not.toContain(en["entry.settings.repository"]);
  });

  it.each([
    ["the flash drive", "flash/flash", en["flash.zipExport.title"]],
    ["the self-backup", "config/config", en["config.settingsTitle"]],
  ])("gives %s the card of its kind", async (_, entry, title) => {
    await renderSettings(entry);
    await waitFor(() => expect(cardTitles()).toContain(title));
    expect(within(card(en["entry.settings.schedule"])).queryByRole("switch")).toBeNull();
  });
});

describe("the schedule", () => {
  it("takes the entry out of the schedule through the call of its kind", async () => {
    await renderSettings("container/immich");
    await press(within(card(en["entry.settings.schedule"])).getByRole("switch", { name: en["entry.settings.inSchedule"] }));
    expect(setInclude).toHaveBeenCalledWith("immich", false);
  });

  it("does the same for a ZFS entry", async () => {
    await renderSettings(`zfs/${ZFS_ID}`);
    await press(within(card(en["entry.settings.schedule"])).getByRole("switch", { name: en["entry.settings.inSchedule"] }));
    expect(patchZFSDataset).toHaveBeenCalledWith(ZFS_ID, { enabled: false });
  });

  it("offers a schedule of its own only while entries may have one", async () => {
    await renderSettings("container/immich");
    expect(screen.queryByText(`${en["schedule.overrideTitle"]}:`)).toBeNull();
    cleanup();

    serveEntries(mocks, { settings: { perItemSchedules: true } });
    await renderSettings("container/immich");
    const schedule = within(card(en["entry.settings.schedule"]));
    expect(schedule.getByText(`${en["schedule.overrideTitle"]}:`)).toBeTruthy();
    expect(schedule.getByText(en["schedule.overrideUsesDefault"])).toBeTruthy();
  });
});

describe("the storage locations", () => {
  it("say that an entry follows the default", async () => {
    await renderSettings("container/immich");
    expect(await screen.findByText(en["placement.followsDefault"])).toBeTruthy();
    expect(screen.queryByRole("button", { name: en["placement.reset"] })).toBeNull();
  });

  it("send an entry with copies of its own back to the default", async () => {
    serveEntries(mocks, {
      containers: [containerView({ placement: { ...PLACEMENT, copiesFollow: false, skip: ["t1"] } })],
    });
    await renderSettings("container/immich");
    expect(await screen.findByText(en["placement.ownCopies"])).toBeTruthy();
    await press(screen.getByRole("button", { name: en["placement.reset"] }));
    await waitFor(() =>
      expect(setItemPlacement).toHaveBeenCalledWith(
        { domain: "containers", key: "immich" },
        { home: { follow: true }, copies: { follow: true } }
      )
    );
  });

  it("name the repository and say that it is fixed once a backup exists", async () => {
    serveEntries(mocks, { containers: [containerView({ placement: { ...PLACEMENT, locked: true, lockReason: "first-backup" } })] });
    await renderSettings("container/immich");
    const repository = within(await waitFor(() => card(en["entry.settings.repository"])));
    expect(repository.getByText(/\/mnt\/user\/backups\/containers/)).toBeTruthy();
    expect(repository.getByLabelText(/^Fixed since the first backup/)).toBeTruthy();
  });
});

describe("a container's switches", () => {
  it("store the update after a backup", async () => {
    await renderSettings("container/immich");
    await press(within(card(en["entry.settings.container"])).getByRole("switch", { name: en["update.afterBackup"] }));
    expect(setUpdateAfterBackup).toHaveBeenCalledWith("immich", true);
  });
});
