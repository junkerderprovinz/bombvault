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
import { BOX, IMMICH, backupRun, serveEntries, timelineOf } from "./backupEntry.testsupport";

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
    getTimeline: vi.fn(),
    getTimelinePlace: vi.fn(),
    checkRestore: vi.fn(),
    cancelRestore: vi.fn(),
    listDbDumps: vi.fn(),
  };
});

vi.mock("../lib/toast", () => ({ useToast: () => ({ push: () => {} }) }));

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

async function renderPage() {
  await act(async () => {
    render(
      <I18nProvider>
        <MemoryRouter initialEntries={["/restore"]}>
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
const entry = (name: string) => entries().getByRole("button", { name: new RegExp(`^${name}`) }) as HTMLButtonElement;
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
    expect(entry("Windows 11").getAttribute("aria-pressed")).toBe("true");
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
