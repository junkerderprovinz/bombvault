// @vitest-environment jsdom
/**
 * The Backups tab of an entry's page.
 *
 * The tab reads the entry's timeline once and shows it three ways: as two
 * weeks of bars, as the storage locations that hold backups, and as the list
 * a restore starts from. A row has to open the restore of its kind at the
 * place that was asked for.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";

import type { AnomalyView } from "../lib/api";
import { ENTRY_ROUTE } from "../lib/backupEntry";
import { I18nProvider, en } from "../lib/i18n";
import { AnomalyProvider } from "../lib/useAnomalies";
import {
  BOX,
  GITEA,
  IMMICH,
  ZFS_ID,
  backupRun,
  containerView,
  serveEntries,
  timelineOf,
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
    getTimelinePlace: vi.fn(),
    getTimelineDeletePreview: vi.fn(),
    listDbDumps: vi.fn(),
    zfsHostDatasets: vi.fn(),
    getZFSReplica: vi.fn(),
    checkRestore: vi.fn(),
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
const getAnomalies = vi.mocked(api.getAnomalies);
const getAnomalySummary = vi.mocked(api.getAnomalySummary);
const getTimeline = vi.mocked(api.getTimeline);
const getTimelinePlace = vi.mocked(api.getTimelinePlace);
const getTimelineDeletePreview = vi.mocked(api.getTimelineDeletePreview);
const listDbDumps = vi.mocked(api.listDbDumps);
const zfsHostDatasets = vi.mocked(api.zfsHostDatasets);

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

function Elsewhere() {
  const { pathname } = useLocation();
  return <p>{`at ${pathname}`}</p>;
}

async function renderPage(path: string) {
  await act(async () => {
    render(
      <I18nProvider>
        <MemoryRouter initialEntries={[path]}>
          <AnomalyProvider>
            <Routes>
              <Route path={ENTRY_ROUTE} element={<BackupEntryPage />} />
              <Route path="*" element={<Elsewhere />} />
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

// Five backups: today, yesterday, three and four days ago, and one from last month.
const RUNS = [0, 1, 3, 4, 30].map((daysAgo, i) => backupRun(`r${i}`, daysAgo));

const rows = () => screen.queryAllByTestId("entry-backup");
const bars = () => screen.queryAllByTestId("history-bar");
const card = (title: string) => screen.getByRole("heading", { level: 2, name: new RegExp(`^${title}`) }).parentElement!;

beforeEach(() => {
  localStorage.clear();
  vi.stubGlobal("EventSource", NoopEventSource);
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => ({ ok: true, json: async () => ({ ok: false }) }))
  );
  serveEntries(mocks, { runs: RUNS });
  vi.mocked(api.listRuns).mockResolvedValue({ ok: true, runs: [] });
  getAnomalies.mockResolvedValue({ ok: true, anomalies: [], nextCursor: "" });
  getAnomalySummary.mockResolvedValue({ ok: false });
  vi.mocked(api.getAnomalyItems).mockResolvedValue({ ok: true, items: [] });
  getTimeline.mockResolvedValue(timelineOf(RUNS));
  getTimelinePlace.mockResolvedValue({ ok: false });
  getTimelineDeletePreview.mockResolvedValue({ ok: true, delete: [], others: [] });
  listDbDumps.mockResolvedValue({ ok: true, dumps: [] });
  zfsHostDatasets.mockResolvedValue({ ok: false } as never);
  vi.mocked(api.getZFSReplica).mockRejectedValue(new Error("no replica"));
  vi.mocked(api.checkRestore).mockResolvedValue({ ok: false } as never);
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

describe("the backup history", () => {
  it("draws a bar for each day of the last two weeks that has a backup", async () => {
    await renderPage("/backups/container/immich");
    expect(bars()).toHaveLength(4);
    expect(within(card(en["entry.history.title"])).getByRole("group", { name: en["entry.history.label"] })).toBeTruthy();
  });

  it("marks the day a finding was raised on and recommends the backup before it", async () => {
    getAnomalySummary.mockResolvedValue({ ok: true, summary: { enabled: true, generation: 1 } } as never);
    getAnomalies.mockResolvedValue({
      ok: true,
      anomalies: [{ id: "a1", runId: "r0", lastRunId: "r0", severity: "warning", state: "open" } as AnomalyView],
      nextCursor: "",
    });
    await renderPage("/backups/container/immich");
    await waitFor(() => expect(bars().at(-1)!.className).toContain("bg-statusWarnSolid"));
    expect(bars().at(-2)!.className).toMatch(/bg-accent$/);
    expect(within(rows()[0]).getByText(en["anomaly.snapshotFlagged"])).toBeTruthy();
  });

  it("leads from a bar to its backup in the list", async () => {
    await renderPage("/backups/container/immich");
    expect(rows()[1].className).toContain("bg-carbon-surface2");
    await press(bars().at(-2)!.closest("button")!);
    expect(rows()[1].className).toContain("bg-carbon-surface3");
  });

  it("stays away while the two weeks hold no backup", async () => {
    serveEntries(mocks, { runs: [backupRun("old", 40)] });
    getTimeline.mockResolvedValue(timelineOf([backupRun("old", 40)]));
    await renderPage("/backups/container/immich");
    expect(screen.queryByRole("heading", { level: 2, name: new RegExp(en["entry.history.title"]) })).toBeNull();
    expect(rows()).toHaveLength(1);
  });
});

describe("the storage locations", () => {
  it("count the backups a read location holds and read another only when asked", async () => {
    getTimeline.mockResolvedValue(timelineOf(RUNS, [BOX]));
    getTimelinePlace.mockResolvedValue({
      ok: true,
      place: { ...BOX, state: "read" },
      rows: timelineOf(RUNS.slice(0, 2)).rows.map((row) => ({
        ...row,
        places: [{ place: BOX.place, snapshotIds: [`${row.key}-copy`], tags: [] }],
      })),
    });
    await renderPage("/backups/container/immich");
    const places = within(card(en["storage.title"]));
    expect(places.getByText("5 backups")).toBeTruthy();
    expect(places.getByText(en["zfs.notChecked"])).toBeTruthy();
    expect(getTimelinePlace).not.toHaveBeenCalled();

    await press(places.getByRole("button", { name: en["timeline.check"] }));
    expect(getTimelinePlace).toHaveBeenCalledWith("containers", "immich", BOX.place);
    await waitFor(() => expect(places.getByText("2 backups")).toBeTruthy());
    expect(within(rows()[0]).getByRole("button", { name: /Storage Box/ })).toBeTruthy();
  });

  it("say when a location cannot be read", async () => {
    getTimeline.mockResolvedValue(timelineOf(RUNS, [{ ...BOX, state: "unreadable", error: "timeout" }]));
    await renderPage("/backups/container/immich");
    const places = within(card(en["storage.title"]));
    expect(places.getByText(en["entry.places.unreadable"])).toBeTruthy();
    expect(places.getByRole("button", { name: en["restoreCheck.recheck"] })).toBeTruthy();
  });
});

describe("the list of backups", () => {
  it("splits the backups at one week", async () => {
    await renderPage("/backups/container/immich");
    expect(rows()).toHaveLength(5);
    const list = within(card(en["backups.title"]));
    expect(within(list.getByText(en["entry.list.thisWeek"]).parentElement!).getAllByTestId("entry-backup")).toHaveLength(4);
    expect(within(list.getByText(en["entry.list.older"]).parentElement!).getAllByTestId("entry-backup")).toHaveLength(1);
  });

  it("opens the restore of a container from a row", async () => {
    await renderPage("/backups/container/immich");
    await press(rows()[0]);
    const window = screen.getByRole("dialog");
    expect(screen.getByText(en["entry.restore.title"].replace("{name}", "immich"))).toBeTruthy();
    expect(within(window).getByText(en["restore.mode.inPlace"])).toBeTruthy();
    await press(within(window).getByRole("button", { name: en["common.close"] }));
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("opens a backup at the place whose mark was pressed", async () => {
    const both = timelineOf(RUNS, [{ ...BOX, state: "read" }]);
    both.rows[0].places.push({ place: BOX.place, snapshotIds: ["copy"], tags: [] });
    getTimeline.mockResolvedValue(both);
    await renderPage("/backups/container/immich");
    await press(
      within(rows()[0]).getByRole("button", { name: en["entry.list.restoreFrom"].replace("{place}", "Storage Box") })
    );
    const window = screen.getByRole("dialog");
    expect(within(window).getByText(/· Storage Box$/)).toBeTruthy();
    expect(within(window).getByRole("tab", { name: "Storage Box" }).getAttribute("aria-selected")).toBe("true");
  });

  it("asks the server what deleting a backup at its place takes", async () => {
    await renderPage("/backups/container/immich");
    await press(rows()[0]);
    await press(within(screen.getByRole("dialog")).getByRole("button", { name: en["common.delete"] }));
    await waitFor(() =>
      expect(getTimelineDeletePreview).toHaveBeenCalledWith("containers", "immich", RUNS[0].snapshotId, ["local"])
    );
  });

  it.each([
    ["the flash drive", "/backups/flash/flash", en["flash.download"]],
    ["the self-backup", "/backups/config/config", en["entry.config.startRecovery"]],
    ["a VM", "/backups/vm/win11", en["restore.open"]],
  ])("offers what %s can do with a backup", async (_, path, action) => {
    await renderPage(path);
    await press(rows()[0]);
    expect(within(screen.getByRole("dialog")).getByRole("button", { name: action })).toBeTruthy();
  });

  it("sends the self-backup to the emergency wizard", async () => {
    await renderPage("/backups/config/config");
    await press(rows()[0]);
    await press(within(screen.getByRole("dialog")).getByRole("button", { name: en["entry.config.startRecovery"] }));
    expect(screen.getByText("at /recovery")).toBeTruthy();
  });

  it("says so when there is no backup yet", async () => {
    serveEntries(mocks, { runs: [] });
    getTimeline.mockResolvedValue(timelineOf([]));
    await renderPage("/backups/container/gitea");
    expect(screen.getByText(en["entry.list.empty"].replace("{name}", GITEA.name))).toBeTruthy();
    expect(rows()).toHaveLength(0);
  });

  it("offers to recreate a container that is gone and has only its definition saved", async () => {
    getTimeline.mockResolvedValue(timelineOf([]));
    await renderPage("/backups/container/pihole");
    expect(screen.getByRole("button", { name: en["snapshots.recreate"] })).toBeTruthy();
  });

  it("says when the backups could not be read", async () => {
    getTimeline.mockResolvedValue({ ok: false, error: "repository locked" });
    await renderPage("/backups/container/immich");
    expect(screen.getByText("repository locked")).toBeTruthy();
  });

  it("opens the newest backup of an entry that is gone from the corner", async () => {
    await renderPage("/backups/container/pihole");
    await press(within(screen.getByTestId("page-actions")).getByRole("button", { name: en["snapshots.restore"] }));
    expect(screen.getByText(en["entry.restore.title"].replace("{name}", "pihole"))).toBeTruthy();
  });
});

describe("a restore link from a finding", () => {
  it("opens the backup it names", async () => {
    await renderPage(`/backups/container/immich?restore=${RUNS[1].snapshotId}&at=${RUNS[1].finishedAt}`);
    expect(await screen.findByRole("dialog")).toBeTruthy();
    expect(rows()[1].className).toContain("bg-carbon-surface3");
  });

  it("says that the backup is gone and names the nearest one", async () => {
    await renderPage(`/backups/container/immich?restore=vanished&at=${IMMICH.lastBackup}`);
    expect(await screen.findByRole("status")).toBeTruthy();
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});

describe("the database dumps", () => {
  it("stand beside the backups of a container that is a database", async () => {
    serveEntries(mocks, {
      runs: RUNS,
      containers: [containerView({ dbEngine: "postgres", dbTier: "curated", dbDataCoverage: "stopped" })],
    });
    await renderPage("/backups/container/immich");
    expect(listDbDumps).toHaveBeenCalledWith("immich", "local");
    await press(screen.getByRole("tab", { name: en["dbdump.listTitle"] }));
    expect(screen.getByRole("heading", { level: 2, name: new RegExp(`^${en["dbdump.listTitle"]}`) })).toBeTruthy();
    expect(rows()[0].closest("[hidden]")).not.toBeNull();
  });

  it("stay away from a container that is none", async () => {
    await renderPage("/backups/container/immich");
    expect(screen.queryByRole("tab", { name: en["dbdump.listTitle"] })).toBeNull();
    expect(listDbDumps).not.toHaveBeenCalled();
  });
});

describe("a ZFS entry", () => {
  it("shows its runs as the history, the places and the list of every other kind", async () => {
    await renderPage(`/backups/zfs/${ZFS_ID}`);
    expect(getTimeline).toHaveBeenCalledWith("zfs", ZFS_ID);
    expect(within(card(en["storage.title"])).getByText("5 backups")).toBeTruthy();
    expect(bars()).toHaveLength(4);
    expect(rows()).toHaveLength(5);
    await press(bars().at(-2)!.closest("button")!);
    expect(rows()[1].className).toContain("bg-carbon-surface3");
  });
});
