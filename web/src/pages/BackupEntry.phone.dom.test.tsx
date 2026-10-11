// @vitest-environment jsdom
/**
 * The page of an entry on a phone.
 *
 * A phone gets the same page in a narrower layout. A control that only the
 * wide page renders is a feature a phone cannot reach, so each tab is drawn
 * at both widths and the phone has to offer everything the desktop does.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import { ENTRY_ROUTE } from "../lib/backupEntry";
import { ENTRY_TABS } from "../lib/entry";
import { I18nProvider } from "../lib/i18n";
import { AnomalyProvider } from "../lib/useAnomalies";
import { DESKTOP_QUERY } from "../lib/useMediaQuery";
import {
  PIHOLE,
  SET_ID,
  ZFS_ID,
  backupRun,
  placementOptions,
  serveEntries,
  timelineOf,
  watched,
} from "./backupEntry.testsupport";

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getBackupItem: vi.fn(),
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
    getAnomalyItemSeries: vi.fn(),
    getTimeline: vi.fn(),
    getTimelinePlace: vi.fn(),
    getPlacementOptions: vi.fn(),
    getContainerMounts: vi.fn(),
    getItemChecks: vi.fn(),
    getSizeBreakdown: vi.fn(),
    listDbDumps: vi.fn(),
    zfsHostDatasets: vi.fn(),
    getZFSReplica: vi.fn(),
  };
});

vi.mock("../lib/toast", () => ({ useToast: () => ({ push: () => {} }) }));

const api = await import("../lib/api");
const mocks = {
  getBackupItem: vi.mocked(api.getBackupItem),
  listContainers: vi.mocked(api.listContainers),
  listVMs: vi.mocked(api.listVMs),
  listFileSets: vi.mocked(api.listFileSets),
  listZFSDatasets: vi.mocked(api.listZFSDatasets),
  listItemRuns: vi.mocked(api.listItemRuns),
  getSettings: vi.mocked(api.getSettings),
};

const { BackupEntryPage } = await import("./BackupEntry");

// useIsDesktop keeps the first MediaQueryList it gets, so the stub answers
// from a variable instead of being swapped per test.
let desktop = true;
window.matchMedia = ((query: string) => ({
  get matches() {
    return query === DESKTOP_QUERY ? desktop : false;
  },
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

const RUNS = [0, 1, 9].map((daysAgo, i) => backupRun(`r${i}`, daysAgo));
const CONTROLS = ["button", "switch", "tab", "link", "textbox", "combobox"];

/** What can be operated on the page, by role and name. */
async function controlsOf(path: string, wide: boolean): Promise<string[]> {
  desktop = wide;
  await act(async () => {
    render(
      <I18nProvider>
        <MemoryRouter initialEntries={[path]}>
          <AnomalyProvider>
            <Routes>
              <Route path={ENTRY_ROUTE} element={<BackupEntryPage />} />
            </Routes>
          </AnomalyProvider>
        </MemoryRouter>
      </I18nProvider>
    );
  });
  const found = CONTROLS.flatMap((role) =>
    screen.queryAllByRole(role).map((el) => `${role}: ${el.getAttribute("aria-label") ?? el.textContent ?? ""}`)
  );
  cleanup();
  return found;
}

beforeEach(() => {
  localStorage.clear();
  vi.stubGlobal("EventSource", NoopEventSource);
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => ({ ok: true, json: async () => ({ ok: false }) }))
  );
  serveEntries(mocks, { runs: RUNS, settings: { perItemSchedules: true } });
  vi.mocked(api.listRuns).mockResolvedValue({ ok: true, runs: [] });
  vi.mocked(api.getAnomalies).mockResolvedValue({ ok: true, anomalies: [], nextCursor: "" });
  vi.mocked(api.getAnomalySummary).mockResolvedValue({ ok: true, summary: { enabled: true, generation: 1 } } as never);
  vi.mocked(api.getAnomalyItems).mockResolvedValue({ ok: true, items: [watched()] });
  vi.mocked(api.getAnomalyItemSeries).mockResolvedValue({ ok: true, series: [] });
  vi.mocked(api.getTimeline).mockResolvedValue(timelineOf(RUNS));
  vi.mocked(api.getTimelinePlace).mockResolvedValue({ ok: false });
  vi.mocked(api.getPlacementOptions).mockImplementation(async (domain) => ({ ok: true, options: placementOptions(domain) }));
  vi.mocked(api.getContainerMounts).mockResolvedValue({ ok: false } as never);
  vi.mocked(api.getItemChecks).mockResolvedValue({ ok: true, items: [] });
  vi.mocked(api.getSizeBreakdown).mockResolvedValue({
    ok: true,
    breakdown: { state: "none", path: "", size: 0, files: 0, added: 0, children: [] },
  });
  vi.mocked(api.listDbDumps).mockResolvedValue({ ok: true, dumps: [] });
  vi.mocked(api.zfsHostDatasets).mockResolvedValue({ ok: false } as never);
  vi.mocked(api.getZFSReplica).mockRejectedValue(new Error("no replica"));
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

const ENTRIES = [
  ["a container", "container/immich"],
  ["a container that is gone", `container/${PIHOLE.key}`],
  ["a VM", "vm/win11"],
  ["a folder set", `files/${SET_ID}`],
  ["a ZFS item", `zfs/${ZFS_ID}`],
  ["the flash drive", "flash/flash"],
  ["the self-backup", "config/config"],
];

describe.each(ENTRIES)("the page of %s on a phone", (_, entry) => {
  it.each(ENTRY_TABS)("offers every control of the wide page under %s", async (tab) => {
    const path = `/backups/${entry}?tab=${tab}`;
    const wide = await controlsOf(path, true);
    const narrow = await controlsOf(path, false);
    expect(wide.length).toBeGreaterThan(3);
    expect(wide.filter((control) => !narrow.includes(control))).toEqual([]);
  });
});
