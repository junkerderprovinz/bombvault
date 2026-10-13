// @vitest-environment jsdom
/**
 * The Restore page.
 *
 * It lists every entry with a backup, shows the backups of the picked one on
 * a rail and opens the restore window from there. A restore that runs shows
 * at the top with the way to stop it.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";

import type { AnomalyView } from "../lib/api";
import { I18nProvider, countText, en } from "../lib/i18n";
import type { ProgressMap } from "../lib/progress";
import { RESTORE_NO_RUNTIME } from "../lib/runReason";
import { BOX, IMMICH, PHOTOS, backupRun, serveEntries, timelineOf } from "./backupEntry.testsupport";

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listItems: vi.fn(),
    getBackupItem: vi.fn(),
    listContainers: vi.fn(),
    listVMs: vi.fn(),
    listFileSets: vi.fn(),
    listZFSDatasets: vi.fn(),
    listItemRuns: vi.fn(),
    listRuns: vi.fn(),
    getSettings: vi.fn(),
    getTimeline: vi.fn(),
    getTimelinePlace: vi.fn(),
    checkRestore: vi.fn(),
    cancelRestore: vi.fn(),
    listDbDumps: vi.fn(),
    getVMSSH: vi.fn(async () => ({ ok: false })),
    restore: vi.fn(),
    restoreVM: vi.fn(),
    restoreFileSet: vi.fn(),
    getRclone: vi.fn(async () => ({ ok: false })),
  };
});

vi.mock("../lib/toast", () => ({ useToast: () => ({ push: () => {} }) }));

vi.mock("../lib/backupWatch", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/backupWatch")>()),
  fireAndWaitRun: vi.fn(),
}));

let openFindings: AnomalyView[] = [];
vi.mock("../lib/useAnomalies", () => ({
  useOpenAnomalies: () => ({ list: openFindings, byRunId: new Map(), flagged: new Set(), error: false }),
}));

let progress: ProgressMap = {};
vi.mock("../lib/progress", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/progress")>();
  return { ...actual, useProgress: () => progress };
});

const api = await import("../lib/api");
const mocks = {
  listItems: vi.mocked(api.listItems),
  getBackupItem: vi.mocked(api.getBackupItem),
  listContainers: vi.mocked(api.listContainers),
  listVMs: vi.mocked(api.listVMs),
  listFileSets: vi.mocked(api.listFileSets),
  listZFSDatasets: vi.mocked(api.listZFSDatasets),
  listItemRuns: vi.mocked(api.listItemRuns),
  getSettings: vi.mocked(api.getSettings),
};
const getTimeline = vi.mocked(api.getTimeline);
const getTimelinePlace = vi.mocked(api.getTimelinePlace);
const cancelRestore = vi.mocked(api.cancelRestore);
const fireAndWaitRun = vi.mocked((await import("../lib/backupWatch")).fireAndWaitRun);

const { Restore } = await import("./Restore");

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

// Three backups: today, yesterday and three days ago.
const RUNS = [0, 1, 3].map((daysAgo, i) => backupRun(`r${i}`, daysAgo));

function Elsewhere() {
  const { pathname } = useLocation();
  return <p>{`at ${pathname}`}</p>;
}

async function renderPage(path = "/restore") {
  await act(async () => {
    render(
      <I18nProvider>
        <MemoryRouter initialEntries={[path]}>
          <Routes>
            <Route path="/restore" element={<Restore />} />
            <Route path="*" element={<Elsewhere />} />
          </Routes>
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

const entries = () => within(screen.getByTestId("restore-entries"));
// While entries are being picked, each one is a checkbox.
const entry = (name: string) =>
  (entries().queryByRole("button", { name: new RegExp(`^${name}`) }) ??
    entries().getByRole("checkbox", { name: new RegExp(`^${name}`) })) as HTMLButtonElement;
const tile = (label: string) => screen.getByRole("button", { name: new RegExp(`^${label}`), pressed: undefined });
const railRows = () => screen.queryAllByTestId("rail-row");
const rail = () => screen.getByRole("listbox", { name: en["restoreWindow.backup"] });

beforeEach(() => {
  openFindings = [];
  progress = {};
  serveEntries(mocks);
  vi.mocked(api.listRuns).mockResolvedValue({ ok: true, runs: [] });
  getTimeline.mockResolvedValue(timelineOf(RUNS));
  getTimelinePlace.mockResolvedValue({ ok: false });
  vi.mocked(api.checkRestore).mockResolvedValue({ ok: false } as never);
  vi.mocked(api.listDbDumps).mockResolvedValue({ ok: true, dumps: [] });
  cancelRestore.mockResolvedValue({ ok: true, cancelled: true });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("the entries", () => {
  it("stand under tiles that count the ones with a backup", async () => {
    await renderPage();
    expect(within(tile(en["nav.containers"])).getByText(countText(en["restorePage.tileCount"], "en", 2))).toBeTruthy();
    expect(within(tile(en["nav.config"])).getByText(countText(en["restorePage.tileCount"], "en", 1))).toBeTruthy();
    expect(entries().getAllByRole("button")).toHaveLength(8);
    expect(entries().queryByText("bombvault")).toBeNull();
  });

  it("keep the count in words on a phone instead of turning into chips", async () => {
    await renderPage();
    expect(tile(en["nav.containers"]).textContent).toBe(en["nav.containers"] + countText(en["restorePage.tileCount"], "en", 2));
  });

  it("cannot be picked while they have no backup", async () => {
    await renderPage();
    expect(entry("gitea").disabled).toBe(true);
    expect(within(entry("gitea")).getByText(en["restorePage.none"])).toBeTruthy();
    expect(entry("immich").disabled).toBe(false);
  });

  it("are narrowed by a tile and by the search", async () => {
    await renderPage();
    await press(tile(en["nav.vms"]));
    expect(entries().getAllByRole("button")).toHaveLength(1);
    await press(tile(en["nav.vms"]));
    expect(entries().getAllByRole("button")).toHaveLength(8);

    fireEvent.change(screen.getByRole("searchbox", { name: en["restorePage.search"] }), { target: { value: "phot" } });
    expect(entries().getAllByRole("button")).toHaveLength(1);
    fireEvent.change(screen.getByRole("searchbox", { name: en["restorePage.search"] }), { target: { value: "nothing" } });
    expect(screen.getByText(en["restorePage.noMatch"])).toBeTruthy();
  });

  it("say so when nothing is backed up", async () => {
    mocks.listItems.mockResolvedValue({ ok: true, items: [], unlisted: [] });
    await renderPage();
    expect(screen.getByText(en["backups.empty"])).toBeTruthy();
    expect(screen.queryByTestId("restore-entries")).toBeNull();
  });

  it("can be read again after a failed load", async () => {
    mocks.listItems.mockResolvedValueOnce({ ok: false, items: [], unlisted: [] });
    await renderPage();
    expect(screen.getByText(en["backups.loadFailed"])).toBeTruthy();
    await press(screen.getByRole("button", { name: en["anomaly.retry"] }));
    expect(entries().getAllByRole("button")).toHaveLength(8);
  });
});

describe("the backups of the picked entry", () => {
  it("are those of the first entry that has any, newest on top", async () => {
    await renderPage();
    expect(getTimeline).toHaveBeenCalledWith("containers", "immich");
    expect(railRows()).toHaveLength(3);
    expect(railRows()[0].getAttribute("aria-selected")).toBe("true");
    expect(within(railRows()[0]).getByText(en["restorePage.recommended"])).toBeTruthy();
    expect(within(railRows()[0]).getByText(en["restoreWindow.whyNewest"])).toBeTruthy();
  });

  it("follow the entry that is picked", async () => {
    await renderPage();
    await press(entry("Windows 11"));
    expect(getTimeline).toHaveBeenLastCalledWith("vms", "win11");
    expect(entry("Windows 11").getAttribute("aria-expanded")).toBe("true");
  });

  it("show their size and the storage locations that hold them", async () => {
    const timeline = timelineOf(RUNS, [{ ...BOX, state: "read" }]);
    timeline.rows[0].size = 65 * 1024 ** 2;
    timeline.rows[0].places.push({ place: BOX.place, snapshotIds: ["copy"], tags: [] });
    getTimeline.mockResolvedValue(timeline);
    await renderPage();
    expect(within(railRows()[0]).getByText("65.0 MB")).toBeTruthy();
    expect(within(railRows()[0]).getByText("Storage Box")).toBeTruthy();
    expect(within(railRows()[1]).queryByText("Storage Box")).toBeNull();
  });

  it("offer to read a storage location nobody has read yet", async () => {
    getTimeline.mockResolvedValue(timelineOf(RUNS, [BOX]));
    await renderPage();
    await press(screen.getByRole("button", { name: en["timeline.check"] }));
    expect(getTimelinePlace).toHaveBeenCalledWith("containers", "immich", BOX.place);
  });

  it("mark the last good backup before an open finding and what came after it", async () => {
    openFindings = [
      {
        id: "a1",
        severity: "critical",
        domain: "container",
        name: "immich",
        targetId: "tg-immich",
        metric: "source_bytes_shrink",
        scopeKind: "item",
        observed: 1,
        expected: 2,
        details: {},
        lastGood: { runId: "r1", snapshotId: RUNS[1].snapshotId, at: RUNS[1].finishedAt },
      } as unknown as AnomalyView,
    ];
    await renderPage();
    expect(railRows()[1].getAttribute("aria-selected")).toBe("true");
    expect(within(railRows()[1]).getByText(en["restorePage.recommended"])).toBeTruthy();
    expect(within(railRows()[0]).getByText(en["restorePage.afterAnomaly"])).toBeTruthy();
    expect(within(railRows()[2]).queryByText(en["restorePage.afterAnomaly"])).toBeNull();
  });

  it("are picked with a click and with the arrow keys", async () => {
    await renderPage();
    await press(railRows()[2]);
    expect(railRows()[2].getAttribute("aria-selected")).toBe("true");
    fireEvent.keyDown(rail(), { key: "ArrowUp" });
    expect(railRows()[1].getAttribute("aria-selected")).toBe("true");
    fireEvent.keyDown(rail(), { key: "Home" });
    expect(railRows()[0].getAttribute("aria-selected")).toBe("true");
    fireEvent.keyDown(rail(), { key: "End" });
    expect(rail().getAttribute("aria-activedescendant")).toBe(railRows()[2].id);
  });

  it("follow the handle while it is dragged", async () => {
    await renderPage();
    const rows = railRows();
    rows.forEach((row, i) => {
      Object.defineProperty(row, "offsetTop", { value: i * 60 });
      Object.defineProperty(row, "offsetHeight", { value: 60 });
    });
    const handle = screen.getByTestId("rail-handle");
    handle.setPointerCapture = () => {};
    fireEvent.pointerDown(handle, { button: 0, pointerId: 1 });
    fireEvent.pointerMove(handle, { pointerId: 1, clientY: 150 });
    expect(railRows()[2].getAttribute("aria-selected")).toBe("true");
    fireEvent.pointerMove(handle, { pointerId: 1, clientY: 70 });
    expect(railRows()[1].getAttribute("aria-selected")).toBe("true");
    fireEvent.pointerUp(handle, { pointerId: 1 });
    expect(railRows()[1].getAttribute("aria-selected")).toBe("true");
  });

  it("open the restore window on the backup whose button is pressed", async () => {
    await renderPage();
    await press(within(railRows()[1]).getByRole("button", { name: en["snapshots.restore"] }));
    const window = await screen.findByRole("dialog", { name: en["entry.restore.title"].replace("{name}", "immich") });
    expect(within(window).getByRole("radio", { name: new RegExp(en["restoreWindow.beside"]) })).toBeTruthy();
    await waitFor(() =>
      expect(vi.mocked(api.checkRestore).mock.calls.at(-1)?.[0]).toMatchObject({ snapshotId: RUNS[1].snapshotId })
    );
  });

  it("opens the restore window at once while the container list is still read, and checks the backup", async () => {
    await renderPage();
    mocks.listContainers.mockReturnValue(new Promise(() => {}));

    await press(within(railRows()[1]).getByRole("button", { name: en["snapshots.restore"] }));

    const window = screen.getByRole("dialog", { name: en["entry.restore.title"].replace("{name}", "immich") });
    expect(within(window).getByRole("radio", { name: new RegExp(en["restoreWindow.beside"]) })).toBeTruthy();
    await waitFor(() =>
      expect(vi.mocked(api.checkRestore).mock.calls.at(-1)?.[0]).toMatchObject({ snapshotId: RUNS[1].snapshotId })
    );
  });

  it("can still be restored from an entry the person said not to back up", async () => {
    const setAside = {
      ...IMMICH,
      included: false,
      effectiveSchedule: { kind: "none", spec: "", reason: "excluded" },
      backupDeclined: true,
    } as const;
    serveEntries(mocks, { items: [setAside] });
    await renderPage();
    expect(entry("immich").disabled).toBe(false);
    expect(railRows()).toHaveLength(3);

    await press(within(railRows()[0]).getByRole("button", { name: en["snapshots.restore"] }));

    expect(mocks.getBackupItem).toHaveBeenCalledWith("container", "immich");
    expect(await screen.findByRole("dialog", { name: en["entry.restore.title"].replace("{name}", "immich") })).toBeTruthy();
  });

  it("say that the entry is gone when the server no longer knows it once its window opens", async () => {
    await renderPage();
    mocks.getBackupItem.mockRejectedValue(new api.ApiError(404, "not found"));

    await press(within(railRows()[0]).getByRole("button", { name: en["snapshots.restore"] }));

    const window = await screen.findByRole("dialog", { name: en["entry.restore.title"].replace("{name}", "immich") });
    expect(within(window).getByText(en["entry.missing"])).toBeTruthy();
  });

  it("open the restore window on Enter", async () => {
    await renderPage();
    fireEvent.keyDown(rail(), { key: "Enter" });
    expect(await screen.findByRole("dialog")).toBeTruthy();
  });

  it("lead to the page of the entry", async () => {
    await renderPage();
    await press(screen.getByRole("button", { name: en["restorePage.openEntry"] }));
    expect(screen.getByText("at /backups/container/immich")).toBeTruthy();
  });
});

describe("a restore that runs", () => {
  it("shows at the top with its progress and can be cancelled", async () => {
    progress = { "container:immich": { phase: "restore", percent: 42, active: true, lastSeen: Date.now() } };
    await renderPage();
    const running = screen.getByTestId("running-restore");
    expect(within(running).getByText(en["restorePage.restoring"].replace("{name}", IMMICH.name))).toBeTruthy();
    expect(within(running).getByText("42 %")).toBeTruthy();

    await press(within(running).getByRole("button", { name: en["restore.cancel"] }));
    await press(within(screen.getByRole("dialog")).getByRole("button", { name: en["common.confirm"] }));
    expect(cancelRestore).toHaveBeenCalledWith("container:immich");
  });

  it("is not shown while only a backup runs", async () => {
    progress = { "container:immich": { phase: "backup", percent: 42, active: true, lastSeen: Date.now() } };
    await renderPage();
    expect(screen.queryByTestId("running-restore")).toBeNull();
  });
});

describe("the emergency", () => {
  const wizard = () => screen.queryByRole("dialog", { name: en["emergency.title"] });
  const PATHS = { containersPath: "", vmsPath: "", flashPath: "", filesPath: "", zfsPath: "", configPath: "", configOffsite: "" };
  const OFFSITE = { containersOffsite: "", vmsOffsite: "", flashOffsite: "", filesOffsite: "", zfsOffsite: "" };

  beforeEach(() => serveEntries(mocks, { settings: { ...PATHS, ...OFFSITE } }));

  it("opens its window from the card and from the corner of the page", async () => {
    await renderPage();
    expect(wizard()).toBeNull();
    await press(screen.getByRole("button", { name: new RegExp(`^${en["restorePage.emergencyWay"]}`) }));
    expect(wizard()).not.toBeNull();
    await press(within(wizard()!).getByRole("button", { name: en["common.cancel"] }));
    await waitFor(() => expect(wizard()).toBeNull());

    await press(within(screen.getByTestId("page-actions")).getByRole("button", { name: en["entry.config.startRecovery"] }));
    expect(within(wizard()!).getByText(en["emergency.waysIntro"])).toBeTruthy();
  });

  it("opens on the step a link names", async () => {
    await renderPage("/restore?emergency=settings");
    expect(within(wizard()!).getByText(en["emergency.settingsIntro"])).toBeTruthy();
  });
});

describe("the other sources", () => {
  it("open the window for an old backup folder and the one for the Appdata.Backup plugin", async () => {
    await renderPage();
    await press(screen.getByRole("button", { name: new RegExp(`^${en["restorePage.oldFolder"]}`) }));
    const oldFolder = screen.getByRole("dialog", { name: en["restorePage.oldFolder"] });
    expect(within(oldFolder).getByRole("button", { name: en["recovery.foreignConnect"] })).toBeTruthy();
    await press(within(oldFolder).getByRole("button", { name: en["common.cancel"] }));

    await press(screen.getByRole("button", { name: new RegExp(`^${en["recovery.abTitle"]}`) }));
    const appdata = screen.getByRole("dialog", { name: en["recovery.abTitle"] });
    expect(within(appdata).getByRole("button", { name: en["recovery.abScan"] })).toBeTruthy();
  });

  it("do not offer another instance through the group, which the server cannot reach yet", async () => {
    await renderPage();
    expect(screen.queryByText(/another BombVault instance/i)).toBeNull();
  });
});

describe("restoring several entries", () => {
  const many = () => screen.getByRole("dialog", { name: /^Restore \d+ entr/ });
  const lines = () => within(many()).getAllByTestId("restore-many-entry");

  beforeEach(() => {
    fireAndWaitRun.mockImplementation(async ({ start }) => {
      const res = await start();
      return res.ok ? { ok: true } : { ok: false, error: res.error };
    });
    vi.mocked(api.restore).mockResolvedValue({ ok: true, started: true });
    vi.mocked(api.restoreVM).mockResolvedValue({ ok: true, started: true });
    vi.mocked(api.restoreFileSet).mockResolvedValue({ ok: true, started: true });
  });

  it("picks entries with Ctrl+click like the Backups list, and leaves ZFS to its own window", async () => {
    await renderPage();
    await act(async () => {
      fireEvent.click(entry("immich"), { ctrlKey: true });
    });
    await act(async () => {
      fireEvent.click(entry("Windows 11"), { ctrlKey: true });
    });
    expect(within(screen.getByTestId("bulk-bar")).getByText("2 selected")).toBeTruthy();
    expect(entry("immich").getAttribute("aria-checked")).toBe("true");
    expect(entries().getAllByRole("checkbox", { name: /^immich/ })).toHaveLength(1);
    expect(railRows()).toHaveLength(0);
    expect(entry("tank/appdata").disabled).toBe(true);
    expect(entries().getByLabelText(en["restoreMany.leftOut.zfs"])).toBeTruthy();
  });

  it("keeps the focus in place when the bar opens and closes", async () => {
    await renderPage();
    const select = () => screen.getByRole("button", { name: new RegExp(`^${en["backups.select"]}`) });
    await press(select());
    const bar = within(screen.getByTestId("bulk-bar"));
    expect(document.activeElement).toBe(bar.getByRole("checkbox", { name: en["containers.selectAll"] }));
    const done = bar.getByRole("button", { name: en["common.done"] });
    done.focus();
    await press(done);
    expect(screen.queryByTestId("bulk-bar")).toBeNull();
    expect(document.activeElement).toBe(select());

    await press(select());
    await act(async () => {
      fireEvent.keyDown(document, { key: "Escape" });
    });
    expect(screen.queryByTestId("bulk-bar")).toBeNull();
    expect(document.activeElement).toBe(select());
  });

  it("tells a screen reader how many entries are picked", async () => {
    await renderPage();
    await act(async () => {
      fireEvent.click(entry("immich"), { ctrlKey: true });
    });
    expect(within(screen.getByTestId("bulk-bar")).getByRole("status").textContent).toBe("1 selected");
  });

  it("restores each one next to its original by default, one after the other", async () => {
    await renderPage();
    await press(screen.getByRole("button", { name: new RegExp(`^${en["backups.select"]}`) }));
    await press(entry("immich"));
    await press(entry("Windows 11"));
    await press(entry("Photos"));
    await press(within(screen.getByTestId("bulk-bar")).getByRole("button", { name: en["snapshots.restore"] }));
    expect(lines()).toHaveLength(3);
    expect(within(many()).getByRole("radio", { name: new RegExp(en["restoreWindow.beside"]) }).getAttribute("aria-checked")).toBe("true");

    await press(within(many()).getByRole("button", { name: `${en["snapshots.restore"]} · 3` }));
    await waitFor(() => expect(lines().every((line) => line.dataset.outcome === "done")).toBe(true));
    const snapshot = RUNS[0].snapshotId;
    expect(api.restore).toHaveBeenCalledWith("immich", snapshot, true, "local", false, false, { target: "beside" });
    expect(api.restoreVM).toHaveBeenCalledWith("win11", snapshot, true, "local", false, { target: "beside" });
    expect(api.restoreFileSet).toHaveBeenCalledWith(PHOTOS.key, snapshot, true, "", "local", { target: "beside" });
    expect(fireAndWaitRun.mock.calls.map(([call]) => call.kind)).toEqual(["restore", "restore", "restore"]);
  });

  it("asks before it replaces the originals, backs them up first, and creates a gone container again", async () => {
    await renderPage();
    await press(screen.getByRole("button", { name: new RegExp(`^${en["backups.select"]}`) }));
    await press(entry("immich"));
    await press(entry("pihole"));
    await press(within(screen.getByTestId("bulk-bar")).getByRole("button", { name: en["snapshots.restore"] }));
    await press(within(many()).getByRole("radio", { name: new RegExp(en["restoreWindow.replace"]) }));
    await press(within(many()).getByRole("button", { name: `${en["snapshots.restore"]} · 2` }));
    expect(api.restore).not.toHaveBeenCalled();
    await press(within(screen.getAllByRole("dialog").at(-1)!).getByRole("button", { name: en["snapshots.restore"] }));
    await waitFor(() => expect(api.restore).toHaveBeenCalledTimes(2));
    const snapshot = RUNS[0].snapshotId;
    expect(api.restore).toHaveBeenCalledWith("immich", snapshot, true, "local", false, false, { backupFirst: true });
    expect(api.restore).toHaveBeenCalledWith("pihole", snapshot, true, "local", false, false, {});
  });

  it("offers a container Docker refused for its GPU or runtime one more try without them, to the same place", async () => {
    fireAndWaitRun.mockImplementation(async ({ start }) => {
      const res = await start();
      return res.ok ? { ok: true } : { ok: false, error: res.error };
    });
    vi.mocked(api.restore).mockResolvedValueOnce({ ok: false, error: RESTORE_NO_RUNTIME });
    await renderPage();
    await press(screen.getByRole("button", { name: new RegExp(`^${en["backups.select"]}`) }));
    await press(entry("immich"));
    await press(entry("Windows 11"));
    await press(within(screen.getByTestId("bulk-bar")).getByRole("button", { name: en["snapshots.restore"] }));
    await press(within(many()).getByRole("button", { name: `${en["snapshots.restore"]} · 2` }));
    await waitFor(() => expect(lines()[1].dataset.outcome).toBe("done"));
    expect(lines()[0].dataset.outcome).toBe("failed");
    expect(within(lines()[1]).queryByRole("button", { name: en["restore.withoutRuntime"] })).toBeNull();

    await press(within(lines()[0]).getByRole("button", { name: en["restore.withoutRuntime"] }));
    await waitFor(() => expect(lines()[0].dataset.outcome).toBe("done"));
    const snapshot = RUNS[0].snapshotId;
    expect(api.restore).toHaveBeenLastCalledWith("immich", snapshot, true, "local", false, true, { target: "beside" });
    expect(api.restoreVM).toHaveBeenCalledTimes(1);
    expect(within(lines()[0]).queryByRole("button", { name: en["restore.withoutRuntime"] })).toBeNull();
  });

  it("leaves out an entry without a backup from the moment picked for all", async () => {
    getTimeline.mockImplementation(async (domain) => timelineOf(domain === "vms" ? RUNS.slice(0, 1) : RUNS));
    await renderPage();
    await press(screen.getByRole("button", { name: new RegExp(`^${en["backups.select"]}`) }));
    await press(entry("immich"));
    await press(entry("Windows 11"));
    await press(within(screen.getByTestId("bulk-bar")).getByRole("button", { name: en["snapshots.restore"] }));
    await press(within(many()).getByRole("radio", { name: en["restoreMany.at"] }));
    await press(within(many()).getByRole("combobox", { name: en["restoreMany.atLabel"] }));
    const options = screen.getAllByRole("option");
    await press(options[options.length - 1]);
    expect(within(lines()[1]).getByText(en["restoreMany.noneBefore"])).toBeTruthy();
    await press(within(many()).getByRole("button", { name: `${en["snapshots.restore"]} · 1` }));
    await waitFor(() => expect(lines()[1].dataset.outcome).toBe("skipped"));
    expect(api.restore).toHaveBeenCalledWith("immich", RUNS[2].snapshotId, true, "local", false, false, { target: "beside" });
    expect(api.restoreVM).not.toHaveBeenCalled();
  });
});
