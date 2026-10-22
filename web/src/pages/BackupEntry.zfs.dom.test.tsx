// @vitest-environment jsdom
/**
 * The page of a ZFS entry.
 *
 * A ZFS item is a root dataset and the tree below it. Its head says what the
 * last check and the last run made of the tree, its settings hold the tree
 * and what surrounds a snapshot, and a run opens on what happened to each
 * dataset. The api client is faked, so a control that writes nothing or calls
 * another endpoint shows here.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";

import type {
  AnomalyItem,
  AnomalySeriesInfo,
  AnomalyView,
  Run,
  ZFSDatasetView,
  ZFSMemberView,
  ZFSRestorePoint,
  ZFSRunDetail,
  ZFSSafetySnapshot,
} from "../lib/api";
import { ENTRY_ROUTE } from "../lib/backupEntry";
import { I18nProvider, countText, en } from "../lib/i18n";
import { isolateLtr } from "../lib/ltrFragments";
import { placementChanged } from "../lib/placementEvents";
import { formatRecent } from "../lib/reltime";
import { AnomalyProvider } from "../lib/useAnomalies";
import { replica } from "../components/zfs/replica/replica.testsupport";
import { TANK, ZFS_ID, backupRun, serveEntries, timelineOf, watched, zfsView } from "./backupEntry.testsupport";

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
    listRepos: vi.fn(),
    getSettings: vi.fn(),
    getAnomalies: vi.fn(),
    getAnomalySummary: vi.fn(),
    getAnomalyItems: vi.fn(),
    getAnomalyItemSeries: vi.fn(),
    getItemChecks: vi.fn(),
    getTimeline: vi.fn(),
    getSizeBreakdown: vi.fn(),
    getZFSReplica: vi.fn(),
    runZFSReplica: vi.fn(),
    listZFSReplicaServers: vi.fn(),
    getGroup: vi.fn(),
    zfsHostDatasets: vi.fn(),
    zfsRestorePoints: vi.fn(),
    zfsRunMembers: vi.fn(),
    listZFSSafetySnapshots: vi.fn(),
    deleteZFSSafetySnapshot: vi.fn(),
    patchZFSDataset: vi.fn(),
    deleteZFSDataset: vi.fn(),
    sweepZFSDataset: vi.fn(),
    probeZFSDataset: vi.fn(),
  };
});

const pushed: { message: string; severity?: string }[] = [];
vi.mock("../lib/toast", () => ({
  useToast: () => ({ push: (message: string, severity?: string) => pushed.push({ message, severity }) }),
}));

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
const getAnomalies = vi.mocked(api.getAnomalies);
const getAnomalySummary = vi.mocked(api.getAnomalySummary);
const getAnomalyItems = vi.mocked(api.getAnomalyItems);
const getTimeline = vi.mocked(api.getTimeline);
const zfsRestorePoints = vi.mocked(api.zfsRestorePoints);
const zfsRunMembers = vi.mocked(api.zfsRunMembers);
const listZFSSafetySnapshots = vi.mocked(api.listZFSSafetySnapshots);
const deleteZFSSafetySnapshot = vi.mocked(api.deleteZFSSafetySnapshot);
const patchZFSDataset = vi.mocked(api.patchZFSDataset);
const deleteZFSDataset = vi.mocked(api.deleteZFSDataset);
const sweepZFSDataset = vi.mocked(api.sweepZFSDataset);
const probeZFSDataset = vi.mocked(api.probeZFSDataset);

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

const ROOT = "tank/appdata";
const CHILD = "tank/appdata/plex";
const PATH = `/backups/zfs/${ZFS_ID}`;

function Elsewhere() {
  const { pathname, search } = useLocation();
  return <p>{`at ${pathname}${search}`}</p>;
}

async function renderPage(search = "") {
  await act(async () => {
    render(
      <I18nProvider>
        <MemoryRouter initialEntries={[`${PATH}${search}`]}>
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

const button = (name: string) => screen.getByRole("button", { name });
const card = (title: string) => screen.getByRole("heading", { level: 2, name: new RegExp(`^${title}`) }).parentElement!;
const runRows = () => screen.queryAllByTestId("entry-run");

async function confirmWith(name: string) {
  await press(within(await screen.findByRole("dialog")).getByRole("button", { name }));
}

/** The actions beside the entry's name. */
async function openMenu() {
  return within(document.querySelector<HTMLElement>('[data-new="entry-menu"]')!);
}

/** Opens the window that takes the entry off the list. */
async function openRemoval() {
  await press((await openMenu()).getByRole("button", { name: en["zfs.removeMissing"] }));
  return screen.findByRole("dialog");
}

function member(over: Partial<ZFSMemberView> = {}): ZFSMemberView {
  return {
    dataset: ROOT,
    relPath: "",
    hostMountpoint: "/mnt/tank/appdata",
    outcome: "backed-up",
    isNew: false,
    usedByDataset: 4096,
    lastBackupAt: 1_700_000_000,
    ...over,
  };
}

const plex = (over: Partial<ZFSMemberView> = {}) =>
  member({ dataset: CHILD, relPath: "plex", hostMountpoint: "/mnt/tank/appdata/plex", ...over });

// An encrypted dataset whose key is not loaded, which a run skips.
const LOCKED = member({ dataset: "tank/appdata/enc", relPath: "enc", outcome: "key-not-loaded" });

/** Serves the entry with this dataset, and with the last run status the list knows. */
function serve(dataset: Partial<ZFSDatasetView> = {}, runs: Run[] = []) {
  serveEntries(mocks, {
    items: [{ ...TANK, lastRunStatus: dataset.lastRunStatus ?? "success" }],
    datasets: [zfsView(dataset)],
    runs,
  });
}

function zfsRun(id: string, over: Partial<Run> = {}): Run {
  return backupRun(id, 0, { domain: "zfs", target: ROOT, targetId: ZFS_ID, ...over });
}

function outcome(dataset: string, what: string, over: Partial<ZFSRunDetail["members"][number]> = {}) {
  return {
    dataset,
    outcome: what,
    resticSnapshot: "",
    isNew: false,
    bytesAdded: 0,
    filesNew: 0,
    filesChanged: 0,
    filesUnmodified: 0,
    durationMs: 0,
    ...over,
  };
}

async function openRun(detail: Partial<ZFSRunDetail>) {
  zfsRunMembers.mockResolvedValue({ ok: true, members: [], windowSeconds: -1, ...detail } as ZFSRunDetail);
  await renderPage("?tab=verify");
  await press(runRows()[0]);
  return within(await screen.findByRole("dialog"));
}

beforeEach(() => {
  pushed.length = 0;
  localStorage.clear();
  vi.stubGlobal("EventSource", NoopEventSource);
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => ({ ok: true, json: async () => ({ ok: false }) }))
  );
  serve();
  vi.mocked(api.listRuns).mockResolvedValue({ ok: true, runs: [] });
  vi.mocked(api.listRepos).mockResolvedValue({ ok: true, repos: [] } as never);
  getAnomalies.mockResolvedValue({ ok: true, anomalies: [], nextCursor: "" });
  getAnomalySummary.mockResolvedValue({ ok: false });
  getAnomalyItems.mockResolvedValue({ ok: true, items: [] });
  vi.mocked(api.getAnomalyItemSeries).mockResolvedValue({ ok: true, series: [] });
  vi.mocked(api.getItemChecks).mockResolvedValue({ ok: true, items: [] });
  getTimeline.mockResolvedValue({ ok: true, places: [], rows: [] });
  vi.mocked(api.getSizeBreakdown).mockResolvedValue({
    ok: true,
    breakdown: { state: "none", path: "", size: 0, files: 0, added: 0, children: [] },
  });
  vi.mocked(api.getZFSReplica).mockRejectedValue(new Error("no replica"));
  vi.mocked(api.listZFSReplicaServers).mockResolvedValue([]);
  vi.mocked(api.getGroup).mockRejectedValue(new Error("no group"));
  vi.mocked(api.zfsHostDatasets).mockResolvedValue({ ok: false, available: false } as never);
  zfsRestorePoints.mockResolvedValue({ ok: true, points: [] } as never);
  zfsRunMembers.mockResolvedValue({ ok: true, members: [], windowSeconds: -1 } as ZFSRunDetail);
  listZFSSafetySnapshots.mockResolvedValue({ ok: true, snapshots: [] } as never);
  deleteZFSSafetySnapshot.mockResolvedValue({ ok: true } as never);
  patchZFSDataset.mockResolvedValue({ ok: true } as never);
  deleteZFSDataset.mockResolvedValue({ ok: true } as never);
  sweepZFSDataset.mockResolvedValue({ ok: true, remaining: 0 } as never);
  probeZFSDataset.mockResolvedValue({ ok: true } as never);
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

describe("the head of a ZFS entry", () => {
  it("names the root, where it is mounted and what can be done with it", async () => {
    await renderPage();
    expect(screen.getByRole("heading", { level: 1 }).textContent).toBe(ROOT);
    expect(screen.getByText("/mnt/tank/appdata")).toBeTruthy();
    const corner = within(screen.getByTestId("page-actions"));
    expect(corner.getByRole("button", { name: en["containers.backupNow"] })).toBeTruthy();
    expect((await openMenu()).getByRole("button", { name: en["zfs.removeMissing"] })).toBeTruthy();
  });

  it("shows the failed run and not the check that says everything is fine", async () => {
    serve({ lastRunStatus: "failed", lastCheckCode: "ok" });
    await renderPage();
    expect(screen.getByText(en["entry.retryNote"], { exact: false })).toBeTruthy();
    expect(screen.queryByText(en["zfs.code.nothing-readable"])).toBeNull();
  });

  it("names the reason beside a run the check refused", async () => {
    serve({ lastRunStatus: "failed", lastCheckCode: "nothing-readable" });
    await renderPage();
    expect(screen.getByText(en["entry.retryNote"], { exact: false })).toBeTruthy();
    expect(screen.getByText(en["zfs.code.nothing-readable"])).toBeTruthy();
  });

  it("says an item was never checked instead of calling its state unknown", async () => {
    serve({ lastCheckCode: "", lastCheckAt: 0 });
    await renderPage();
    expect(screen.getByText(en["zfs.notChecked"])).toBeTruthy();
    expect(screen.queryByText(/Unknown state/)).toBeNull();
  });

  it("counts the datasets a run skips and names them", async () => {
    serve({ members: [member(), plex({ isNew: true }), LOCKED] });
    await renderPage();
    expect(screen.getByText(countText(en["zfs.skippedCount"], "en", 1))).toBeTruthy();
    expect(screen.getByLabelText("tank/appdata/enc")).toBeTruthy();
  });

  it("names the containers an interrupted snapshot left stopped", async () => {
    serve({ restartPending: ["plex", "sonarr"] });
    await renderPage();
    expect(screen.getByText(en["zfs.restartPending"].replace("{names}", "plex, sonarr"))).toBeTruthy();
    expect(screen.getByLabelText(en["zfs.restartPendingHint"])).toBeTruthy();
  });

  it("sweeps the leftover snapshots from its own note", async () => {
    serve({ leftoverCount: 3 });
    await renderPage();
    expect(screen.getByText(countText(en["zfs.leftovers"], "en", 3))).toBeTruthy();
    await press(button(en["zfs.removeLeftovers"]));
    expect(sweepZFSDataset).toHaveBeenCalledWith(ZFS_ID);
    expect(pushed).toContainEqual({ message: en["zfs.sweepDone"], severity: "success" });
  });

  it("says that the root is gone, with the way to remove the item beside its name", async () => {
    serve({ lastCheckCode: "not-found", lastRunStatus: "" });
    await renderPage();
    expect(screen.getByText(en["zfs.code.not-found"])).toBeTruthy();
    expect((await openMenu()).getByRole("button", { name: en["zfs.removeMissing"] })).toBeTruthy();
  });
});

describe("the head of a ZFS entry that replicates", () => {
  const AT = Math.floor(Date.now() / 1000) - 2 * 3600;
  const replicated = (replica: { lastAt: number; lastStatus: string; only?: boolean }) =>
    serveEntries(mocks, {
      items: [{ ...TANK, lastBackup: 0, lastRunStatus: "", replica }],
      datasets: [zfsView({ lastBackup: 0, lastRunStatus: "" })],
    });

  it("names the replica instead of waiting for a first backup", async () => {
    replicated({ lastAt: AT, lastStatus: "success" });
    await renderPage();
    expect(screen.getByText(en["zfs.replica.state.ok"].replace("{when}", formatRecent(AT, "en")))).toBeTruthy();
    expect(screen.queryByText(en["backups.status.waiting"])).toBeNull();
  });

  it("still waits for the first backup before any replica has run", async () => {
    replicated({ lastAt: 0, lastStatus: "" });
    await renderPage();
    expect(screen.getByText(en["backups.status.waiting"])).toBeTruthy();
  });

  it("waits for the first replica when the entry only replicates", async () => {
    replicated({ lastAt: 0, lastStatus: "", only: true });
    await renderPage();
    expect(screen.getByText(en["backups.status.waitingReplica"])).toBeTruthy();
    expect(screen.queryByText(en["backups.status.waiting"])).toBeNull();
  });

  it("says when the replica failed", async () => {
    replicated({ lastAt: AT, lastStatus: "failed" });
    await renderPage();
    expect(screen.getByText(en["zfs.replica.state.failed"])).toBeTruthy();
  });

  it("says Replicating while the replica runs and reads the entry again when it ends", async () => {
    const streams: { onmessage: ((ev: MessageEvent<string>) => void) | null }[] = [];
    class Stream {
      onmessage: ((ev: MessageEvent<string>) => void) | null = null;
      constructor() {
        streams.push(this);
      }
      close() {}
    }
    vi.stubGlobal("EventSource", Stream);
    replicated({ lastAt: 0, lastStatus: "" });
    await renderPage();
    const frame = (active: boolean) =>
      act(() => {
        const data = JSON.stringify({ key: `zfs-replica:${ZFS_ID}`, phase: "maintenance", percent: 50, active });
        streams.at(-1)?.onmessage?.({ data } as MessageEvent<string>);
      });
    await frame(true);
    expect(screen.getByText(en["zfs.replica.state.running"])).toBeTruthy();
    replicated({ lastAt: AT, lastStatus: "success" });
    await frame(false);
    expect(await screen.findByText(en["zfs.replica.state.ok"].replace("{when}", formatRecent(AT, "en")))).toBeTruthy();
  });
});

describe("the replica action of a ZFS entry", () => {
  const corner = () => within(screen.getByTestId("page-actions"));

  it("starts the replica from the corner, before the backup action", async () => {
    vi.mocked(api.getZFSReplica).mockResolvedValue(replica());
    vi.mocked(api.runZFSReplica).mockResolvedValue({ ok: true, runId: "r1" });
    await renderPage();
    const run = await corner().findByRole("button", { name: en["zfs.replica.replicateNow"] });
    const names = corner().getAllByRole("button").map((b) => b.getAttribute("aria-label") ?? b.textContent);
    expect(names.indexOf(en["zfs.replica.replicateNow"])).toBeLessThan(names.indexOf(en["containers.backupNow"]));
    await press(run);
    expect(api.runZFSReplica).toHaveBeenCalledWith(ZFS_ID);
  });

  it("is the only run action of an entry that only replicates", async () => {
    vi.mocked(api.getZFSReplica).mockResolvedValue(replica({ only: true }));
    await renderPage();
    await corner().findByRole("button", { name: en["zfs.replica.replicateNow"] });
    expect(corner().queryByRole("button", { name: en["containers.backupNow"] })).toBeNull();
  });

  it("offers no backup to an entry that only replicates while its replica view still loads", async () => {
    vi.mocked(api.getZFSReplica).mockReturnValue(new Promise(() => undefined));
    serveEntries(mocks, {
      items: [{ ...TANK, replica: { lastAt: 0, lastStatus: "", only: true } }],
      datasets: [zfsView()],
    });
    await renderPage();
    expect(corner().queryByRole("button", { name: en["containers.backupNow"] })).toBeNull();
  });

  it("says why a run was refused", async () => {
    vi.mocked(api.getZFSReplica).mockResolvedValue(replica());
    vi.mocked(api.runZFSReplica).mockResolvedValue({ ok: false, code: "domain-off" });
    await renderPage();
    await press(await corner().findByRole("button", { name: en["zfs.replica.replicateNow"] }));
    expect(pushed).toContainEqual({ message: en["zfs.code.domain-off"], severity: "fail" });
  });

  it("is absent while the entry replicates nowhere", async () => {
    vi.mocked(api.getZFSReplica).mockResolvedValue(replica({ target: { kind: "none", id: "" }, state: "never" }));
    await renderPage();
    await corner().findByRole("button", { name: en["containers.backupNow"] });
    expect(corner().queryByRole("button", { name: en["zfs.replica.replicateNow"] })).toBeNull();
  });

  it("is absent while a paired instance has not allowed the replica", async () => {
    vi.mocked(api.getZFSReplica).mockResolvedValue(
      replica({ target: { kind: "peer", id: "peer-1" }, state: "never", peerState: "asked" })
    );
    await renderPage();
    await corner().findByRole("button", { name: en["containers.backupNow"] });
    expect(corner().queryByRole("button", { name: en["zfs.replica.replicateNow"] })).toBeNull();
  });

  it.each(["refused", "off", "revoked"] as const)("is absent while a paired instance is %s", async (peerState) => {
    vi.mocked(api.getZFSReplica).mockResolvedValue(replica({ target: { kind: "peer", id: "peer-1" }, state: "never", peerState }));
    await renderPage();
    await corner().findByRole("button", { name: en["containers.backupNow"] });
    expect(corner().queryByRole("button", { name: en["zfs.replica.replicateNow"] })).toBeNull();
  });

  it("is there for a paired instance that allowed the replica", async () => {
    vi.mocked(api.getZFSReplica).mockResolvedValue(replica({ target: { kind: "peer", id: "peer-1" }, peerState: "allowed" }));
    await renderPage();
    expect(await corner().findByRole("button", { name: en["zfs.replica.replicateNow"] })).toBeTruthy();
  });

  it("is absent while ZFS backups are off", async () => {
    vi.mocked(api.getZFSReplica).mockResolvedValue(replica({ state: "waiting" }));
    await renderPage();
    await corner().findByRole("button", { name: en["containers.backupNow"] });
    expect(corner().queryByRole("button", { name: en["zfs.replica.replicateNow"] })).toBeNull();
  });

  describe("while something else has the replica", () => {
    const streams: { onmessage: ((ev: MessageEvent<string>) => void) | null }[] = [];
    class Stream {
      onmessage: ((ev: MessageEvent<string>) => void) | null = null;
      constructor() {
        streams.push(this);
      }
      close() {}
    }
    const frame = (key: string, active: boolean) =>
      act(() => {
        const data = JSON.stringify({ key, phase: "replicate", percent: 10, active });
        streams.at(-1)?.onmessage?.({ data } as MessageEvent<string>);
      });

    beforeEach(() => {
      streams.length = 0;
      vi.stubGlobal("EventSource", Stream);
      vi.mocked(api.getZFSReplica).mockResolvedValue(replica());
    });

    it("holds the action while the replica runs", async () => {
      await renderPage();
      await corner().findByRole("button", { name: en["zfs.replica.replicateNow"] });
      await frame(`zfs-replica:${ZFS_ID}`, true);
      expect((corner().getByRole("button", { name: en["zfs.replica.replicateNow"] }) as HTMLButtonElement).disabled).toBe(true);
    });

    it("holds the action while a bring back has the replica", async () => {
      await renderPage();
      await corner().findByRole("button", { name: en["zfs.replica.replicateNow"] });
      await frame(`zfs-replica-restore:${ZFS_ID}`, true);
      expect((corner().getByRole("button", { name: en["zfs.replica.replicateNow"] }) as HTMLButtonElement).disabled).toBe(true);
    });

    it("says the replica is up to date once a started run ends", async () => {
      vi.mocked(api.runZFSReplica).mockResolvedValue({ ok: true, runId: "r1" });
      await renderPage();
      await press(await corner().findByRole("button", { name: en["zfs.replica.replicateNow"] }));
      await frame(`zfs-replica:${ZFS_ID}`, true);
      await frame(`zfs-replica:${ZFS_ID}`, false);
      await waitFor(() =>
        expect(pushed).toContainEqual({ message: en["zfs.replica.upToDate"].replace("{name}", "tank/appdata"), severity: "success" }),
      );
    });

    it("names the first replica once a first run ends", async () => {
      vi.mocked(api.getZFSReplica).mockResolvedValue(replica({ state: "never" }));
      vi.mocked(api.runZFSReplica).mockResolvedValue({ ok: true, runId: "r1" });
      await renderPage();
      const button = await corner().findByRole("button", { name: en["zfs.replica.replicateNow"] });
      vi.mocked(api.getZFSReplica).mockResolvedValue(replica({ state: "running" }));
      await press(button);
      await frame(`zfs-replica:${ZFS_ID}`, true);
      vi.mocked(api.getZFSReplica).mockResolvedValue(replica());
      await frame(`zfs-replica:${ZFS_ID}`, false);
      await waitFor(() => expect(pushed.some((p) => p.severity === "success" && p.message.startsWith("tank/appdata: first replica to"))).toBe(true));
    });
  });
});

describe("removing a ZFS entry", () => {
  const SAFETY_TOO = countText(en["entry.remove.safety"], "en", 2);

  it("leaves for the list and says how many safety snapshots stayed behind", async () => {
    serve({ safetyCount: 2 });
    deleteZFSDataset.mockResolvedValue({ ok: true, safetyRemaining: 2 } as never);
    await renderPage();
    const dialog = await openRemoval();
    expect(within(dialog).getByRole("switch", { name: SAFETY_TOO })).toBeTruthy();
    await press(within(dialog).getByRole("button", { name: en["entry.remove.keepGo"] }));
    await waitFor(() => expect(deleteZFSDataset).toHaveBeenCalledWith(ZFS_ID, false));
    expect(pushed).toContainEqual({ message: countText(en["zfs.deleteKeptSafety"], "en", 2), severity: "warn" });
    expect(await screen.findByText("at /backups?kind=zfs")).toBeTruthy();
  });

  it("takes the safety snapshots along when asked", async () => {
    serve({ safetyCount: 2 });
    await renderPage();
    const dialog = await openRemoval();
    await press(within(dialog).getByRole("switch", { name: SAFETY_TOO }));
    await press(within(dialog).getByRole("button", { name: en["entry.remove.keepGo"] }));
    await waitFor(() => expect(deleteZFSDataset).toHaveBeenCalledWith(ZFS_ID, true));
  });

  it("says that taking the safety snapshots along needs a login password", async () => {
    serve({ safetyCount: 2 });
    deleteZFSDataset.mockResolvedValue({ ok: false, code: "password-not-set", error: "set a login password before deleting safety snapshots" });
    await renderPage();
    const dialog = await openRemoval();
    await press(within(dialog).getByRole("switch", { name: SAFETY_TOO }));
    await press(within(dialog).getByRole("button", { name: en["entry.remove.keepGo"] }));
    await waitFor(() => expect(pushed).toContainEqual({ message: en["secrets.deletePasswordNotSet"], severity: "fail" }));
    expect(screen.queryByText("at /backups?kind=zfs")).toBeNull();
  });

  it("says why a removal the server turned away did nothing", async () => {
    deleteZFSDataset.mockRejectedValue(new api.ApiError(409, "HTTP 409 Conflict"));
    await renderPage();
    await openRemoval();
    await confirmWith(en["entry.remove.keepGo"]);
    await waitFor(() => expect(pushed).toContainEqual({ message: "HTTP 409 Conflict", severity: "fail" }));
    expect(screen.getByRole("heading", { level: 1 }).textContent).toBe(ROOT);
  });
});

describe("the backups of a ZFS entry", () => {
  it("count the sites the server counted the replica into", async () => {
    serve({ sites: 2, rule321: "met" });
    await renderPage();
    expect(screen.getByText(en["placement.rule321Met"])).toBeTruthy();
    expect(screen.getByText("At 2 sites")).toBeTruthy();
  });

  it("are restored from the window a backup of the list opens, which offers the datasets of the tree", async () => {
    getTimeline.mockResolvedValue(timelineOf([zfsRun("r1")]));
    await renderPage();
    await press(screen.getByTestId("entry-backup"));
    const restore = await screen.findByRole("dialog", { name: en["entry.restore.title"].replace("{name}", ROOT) });
    expect(within(restore).getByRole("combobox", { name: en["zfs.restore.dataset"] })).toBeTruthy();
    expect(zfsRestorePoints).toHaveBeenCalledWith(ZFS_ID, expect.anything());
  });
});

describe("the settings of a ZFS entry", () => {
  async function renderSettings() {
    await renderPage("?tab=settings");
    return within(card(en["entry.settings.zfs"]));
  }

  it("offer the child datasets, the excludes, the commands and the check", async () => {
    const settings = await renderSettings();
    expect(settings.getByText(en["zfs.children"])).toBeTruthy();
    expect(settings.getByText(en["zfs.excludes"])).toBeTruthy();
    expect(settings.getByText(en["zfs.preSnapshot"])).toBeTruthy();
    await press(settings.getByRole("button", { name: en["zfs.probe"] }));
    expect(probeZFSDataset).toHaveBeenCalledWith(ZFS_ID);
  });

  it("add a container to stop for the snapshot from the list of containers", async () => {
    const settings = await renderSettings();
    await press(settings.getByRole("button", { name: en["zfs.stopContainersPlaceholder"] }));
    const option = (await screen.findAllByRole("option"))[0];
    const name = option.textContent ?? "";
    await press(option);
    expect(patchZFSDataset).toHaveBeenCalledWith(ZFS_ID, { stopContainers: [name] });
  });

  it("open the connection to the server from its row", async () => {
    const settings = await renderSettings();
    await press(settings.getByRole("button", { name: en["zfs.connection.view"] }));
    expect(await screen.findByRole("dialog", { name: en["zfs.connection.title"] })).toBeTruthy();
  });

  it("calls a dataset no backup has reached yet by that, never an unknown state", async () => {
    serve({ members: [member({ outcome: "", lastBackupAt: 0 }), plex({ outcome: "", lastBackupAt: 0 })] });
    const settings = await renderSettings();
    expect(settings.getAllByText(en["zfs.member.pending"])).toHaveLength(2);
    expect(screen.queryByText(/Unknown state/)).toBeNull();
    expect(screen.queryByText(countText(en["zfs.skippedCount"], "en", 2))).toBeNull();
  });

  it("calls a dataset that only has a replica replicated", async () => {
    serve({ members: [member({ outcome: "", lastBackupAt: 0, replicated: true }), plex({ outcome: "", lastBackupAt: 0 })] });
    const settings = await renderSettings();
    expect(settings.getAllByText(en["zfs.member.replicated"])).toHaveLength(1);
    expect(settings.getAllByText(en["zfs.member.pending"])).toHaveLength(1);
  });

  it("pick the repository in a card of its own", async () => {
    await renderPage("?tab=settings");
    expect(within(card(en["entry.settings.repository"])).getByRole("combobox", { name: en["zfs.repo"] })).toBeTruthy();
  });

  it("use no native select", async () => {
    await renderSettings();
    expect(document.querySelector("select")).toBeNull();
  });

  it("mark a dataset the next run picks up and say why another is skipped", async () => {
    serve({ members: [member(), plex({ isNew: true }), LOCKED] });
    const settings = await renderSettings();
    expect(settings.getByText(en["zfs.member.new"])).toBeTruthy();
    expect(settings.getByText(en["zfs.code.key-not-loaded"])).toBeTruthy();
  });

  it("take snapshot commands only once a container is chosen to run them in", async () => {
    const settings = await renderSettings();
    const place = settings.getByRole("combobox", { name: en["zfs.hookContainer"] });
    expect(place.textContent).toContain(en["zfs.hookContainerNone"]);
    expect((settings.getByLabelText(en["zfs.preSnapshot"]) as HTMLInputElement).disabled).toBe(true);
    expect((settings.getByLabelText(en["zfs.postSnapshot"]) as HTMLInputElement).disabled).toBe(true);
  });

  it("offer the commands of an item that has a container for them", async () => {
    serve({ hookContainer: "postgres", preSnapshot: "pg_backup_start" });
    const settings = await renderSettings();
    expect((settings.getByLabelText(en["zfs.preSnapshot"]) as HTMLInputElement).disabled).toBe(false);
  });

  it("exclude a child through the tree", async () => {
    serve({ members: [member(), plex()] });
    const settings = await renderSettings();
    const tree = settings.getByRole("group", { name: en["zfs.children"] });
    await press(within(tree).getByRole("switch", { name: CHILD }));
    expect(patchZFSDataset).toHaveBeenCalledWith(ZFS_ID, { excludedChildren: [CHILD] });
  });
});

describe("the safety snapshots of a ZFS entry", () => {
  const SNAPSHOT: ZFSSafetySnapshot = {
    dataset: ROOT,
    name: "bombvault-prerestore-20260101120000",
    createdAt: 1_700_000_000,
    usedBytes: 2048,
  };
  const title = countText(en["zfs.safety.title"], "en", 1);

  async function showList() {
    listZFSSafetySnapshots.mockResolvedValue({ ok: true, snapshots: [SNAPSHOT] } as never);
    await renderPage("?tab=settings");
    return screen.findByRole("list", { name: title });
  }

  it("are listed in a card of their own and deleted one at a time", async () => {
    serve({ safetyCount: 1 });
    const list = await showList();
    expect(listZFSSafetySnapshots).toHaveBeenCalledWith(ZFS_ID);
    expect(within(list).getByText(/bombvault-prerestore-20260101120000/)).toBeTruthy();
    await press(within(list).getByRole("button", { name: en["common.delete"] }));
    await confirmWith(en["common.delete"]);
    await waitFor(() => expect(deleteZFSSafetySnapshot).toHaveBeenCalledWith(ZFS_ID, ROOT, SNAPSHOT.name));
  });

  it("offer a paired phone the sign-in when one is deleted", async () => {
    serve({ safetyCount: 1 });
    deleteZFSSafetySnapshot.mockResolvedValue({ ok: false, code: "own-login-needed", error: "sign in at this instance's web interface" });
    const list = await showList();
    await press(within(list).getByRole("button", { name: en["common.delete"] }));
    await confirmWith(en["common.delete"]);
    const ask = within(await screen.findByRole("dialog"));
    expect(ask.getByText(en["secrets.ownLoginNeeded"])).toBeTruthy();
    expect(ask.getByRole("button", { name: en["secrets.signInHere"] })).toBeTruthy();
    expect(pushed).toEqual([]);
  });

  it("drop their count and the age warning once the last one is gone", async () => {
    serve({ safetyCount: 1, safetyOldestAt: 1_600_000_000 });
    deleteZFSSafetySnapshot.mockImplementation((async () => {
      serve({ safetyCount: 0, safetyOldestAt: 0 });
      return { ok: true };
    }) as never);
    const list = await showList();
    expect(screen.getByText(en["zfs.safety.old"])).toBeTruthy();
    await press(within(list).getByRole("button", { name: en["common.delete"] }));
    await confirmWith(en["common.delete"]);
    await waitFor(() => expect(screen.queryByText(en["zfs.safety.old"])).toBeNull());
    expect(screen.queryByText(title)).toBeNull();
  });
});

describe("a run of a ZFS entry", () => {
  it("names the window it held the applications for", async () => {
    serve({}, [zfsRun("r1")]);
    const run = await openRun({
      windowSeconds: 7,
      members: [outcome(ROOT, "backed-up", { resticSnapshot: "abc", bytesAdded: 1024, filesNew: 2 })],
    });
    expect(zfsRunMembers).toHaveBeenCalledWith("r1");
    expect(await run.findByText(en["zfs.window"].replace("{seconds}", "7"))).toBeTruthy();
  });

  it("hides the window when it stopped nothing", async () => {
    serve({}, [zfsRun("r2")]);
    const run = await openRun({ windowSeconds: -1, members: [outcome(ROOT, "backed-up")] });
    await run.findByText(ROOT);
    expect(run.queryByText(/Containers were stopped for/)).toBeNull();
  });

  it("names where a dataset it could not see is mounted", async () => {
    serve({ members: [member(), plex()] }, [zfsRun("r3")]);
    const run = await openRun({ members: [outcome(CHILD, "not-visible"), outcome("tank/appdata/gone", "not-visible")] });
    expect(
      await run.findByText(en["zfs.code.not-visible"].replace("{path}", "/mnt/tank/appdata/plex"))
    ).toBeTruthy();
    expect(run.getByText(en["zfs.notVisibleNoPath"])).toBeTruthy();
  });

  it("says why it failed before it read any dataset, and prints the command's output once", async () => {
    serve({ lastRunStatus: "failed" }, [
      zfsRun("r4", { status: "failed", snapshotId: "", error: "pre-snapshot-failed: hook exited 1: about to fail" }),
    ]);
    const run = await openRun({ hookDetail: "hook exited 1: about to fail" });
    expect(await run.findByText(en["zfs.code.pre-snapshot-failed"])).toBeTruthy();
    expect(run.getAllByText(/about to fail/)).toHaveLength(1);
  });

  it("is called cancelled and not failed when it was cancelled", async () => {
    serve({ lastRunStatus: "cancelled" }, [
      zfsRun("r5", { status: "cancelled", snapshotId: "", error: "cancelled by the user" }),
    ]);
    const run = await openRun({ members: [outcome(ROOT, "not-reached")] });
    expect(within(runRows()[0]).getByText(en["run.statusCancelled"])).toBeTruthy();
    expect(screen.queryByText(en["run.statusFailed"])).toBeNull();
    expect(await run.findByText(en["runReason.cancelled"])).toBeTruthy();
  });

  it("names the dataset it hung on and says what to check", async () => {
    serve({ lastRunStatus: "failed" }, [
      zfsRun("r6", {
        status: "failed",
        snapshotId: "",
        error: "stopped by the stall guard after 2 hours without progress while reading tank/appdata",
      }),
    ]);
    const run = await openRun({ members: [outcome(ROOT, "stalled")] });
    expect(await run.findByText(en["zfs.code.stalled"])).toBeTruthy();
    expect(run.getByLabelText(en["zfs.fix.stalled"])).toBeTruthy();
  });

  it("is listed once it has finished, without the page being opened again", async () => {
    serve({}, [zfsRun("r1")]);
    await renderPage("?tab=verify");
    expect(runRows()).toHaveLength(1);
    serve({}, [zfsRun("r2"), zfsRun("r1")]);
    act(() => placementChanged());
    await waitFor(() => expect(runRows()).toHaveLength(2));
  });
});

function series(part: string, over: Partial<AnomalySeriesInfo> = {}): AnomalySeriesInfo {
  return {
    part,
    learning: { samples: 10, needed: 10 },
    typical: { sourceBytes: 20 * 1024 ** 3, resticMs: 60_000 },
    open: { critical: 0, warning: 0, info: 0 },
    retentionHeld: false,
    ...over,
  };
}

function watchedTree(): AnomalyItem {
  return watched({
    targetId: ZFS_ID,
    domain: "zfs",
    name: ROOT,
    datasets: [series(ROOT), series(CHILD, { open: { critical: 1, warning: 0, info: 0 }, retentionHeld: true })],
    retentionHeld: true,
  });
}

function point(stamp: string, time: number, childSnapshot: string): ZFSRestorePoint {
  return {
    stamp,
    time,
    members: [
      { dataset: ROOT, relPath: "", snapshotId: `${stamp}-root`, outcome: "backed-up" },
      { dataset: CHILD, relPath: "plex", snapshotId: childSnapshot, outcome: "backed-up" },
    ],
  };
}

const EMPTIED_AT = 1_789_086_400;
const GOOD_AT = 1_789_000_000;
const localTime = (at: number) => new Date(at * 1000).toLocaleString();
/** A backup as the restore window names it in its list. */
const listed = (at: number) => formatRecent(at, "en");

/** The timeline of the tree: one row per run under its stamp, which holds the
 *  snapshot of every dataset. */
function treeTimeline(points: ZFSRestorePoint[]) {
  return {
    ...timelineOf([]),
    rows: points.map((p) => ({
      key: p.stamp,
      time: new Date(p.time * 1000).toISOString(),
      places: [{ place: "local", snapshotIds: p.members.map((m) => m.snapshotId), tags: [] }],
    })),
  };
}

const restoreWindow = () => screen.findByRole("dialog", { name: en["entry.restore.title"].replace("{name}", ROOT) });

describe("what anomaly detection knows about a ZFS entry", () => {
  beforeEach(() => {
    serve({ members: [member(), plex()] });
    getAnomalySummary.mockResolvedValue({ ok: true, summary: { enabled: true, generation: 1 } } as never);
    getAnomalyItems.mockResolvedValue({ ok: true, items: [watchedTree()] });
    const points = [
      point("bombvault-20260921030000", EMPTIED_AT, "child-emptied"),
      point("bombvault-20260920030000", GOOD_AT, "child-good"),
    ];
    zfsRestorePoints.mockResolvedValue({ ok: true, points } as never);
    getTimeline.mockResolvedValue(treeTimeline(points));
  });

  const badge = (name: string, n: number) => en["anomaly.itemBadgeAria"].replace("{name}", name).replace("{n}", String(n));

  it("marks the dataset a finding is about in the tree, and no other", async () => {
    await renderPage("?tab=settings");
    const tree = within(card(en["entry.settings.zfs"]));
    const line = (await tree.findByRole("link", { name: badge(CHILD, 1) })).closest("li") as HTMLElement;
    expect(within(line).getByText(en["anomaly.retentionPaused"])).toBeTruthy();
    expect(tree.queryByRole("link", { name: badge(ROOT, 0) })).toBeNull();
  });

  it("offers the item's own sensitivity on the Verify tab", async () => {
    await renderPage("?tab=verify");
    expect(
      await within(card(en["anomaly.title"])).findByRole("radiogroup", { name: en["anomaly.items.sensitivity"] })
    ).toBeTruthy();
  });

  it("opens the restore on the dataset's last good backup and marks the emptied one", async () => {
    const finding = {
      id: "an-1",
      detector: "source",
      metric: "source_bytes_shrink",
      severity: "critical",
      state: "open",
      scopeKind: "zfsds",
      scopeId: CHILD,
      targetId: ZFS_ID,
      domain: "zfs",
      name: ROOT,
      part: CHILD,
      runId: "r2",
      lastRunId: "r2",
      lastRunAt: EMPTIED_AT,
      lastGood: { runId: "r1", snapshotId: "child-good", at: GOOD_AT },
      flaggedSnapshots: ["child-emptied"],
      details: {},
    } as unknown as AnomalyView;
    getAnomalies.mockResolvedValue({ ok: true, anomalies: [finding], nextCursor: "" });
    await renderPage(`?restore=child-good&dataset=${encodeURIComponent(CHILD)}`);

    const panel = await restoreWindow();
    const backupField = within(panel).getByRole("combobox", { name: en["restoreWindow.backup"] });
    expect(backupField.textContent).toContain(listed(GOOD_AT));
    const datasetField = within(panel).getByRole("combobox", { name: en["zfs.restore.dataset"] });
    await waitFor(() => expect(datasetField.textContent).toContain(CHILD));
    expect(within(panel).queryByText(en["anomaly.snapshotFlagged"])).toBeNull();

    await press(backupField);
    await press(
      await screen.findByRole("option", {
        name: new RegExp(listed(EMPTIED_AT).replace(/[.*+?^${}()|[\]\\]/g, "\\$&")),
      })
    );
    expect(await within(panel).findByText(en["anomaly.snapshotFlagged"])).toBeTruthy();
    expect(datasetField.textContent).toContain(CHILD);
  });

  it("says that the linked backup is gone and opens on the nearest one", async () => {
    await renderPage(`?restore=child-pruned&at=1788990000&dataset=${encodeURIComponent(CHILD)}`);

    const panel = await restoreWindow();
    const backupField = within(panel).getByRole("combobox", { name: en["restoreWindow.backup"] });
    expect(backupField.textContent).toContain(listed(GOOD_AT));
    const datasetField = within(panel).getByRole("combobox", { name: en["zfs.restore.dataset"] });
    await waitFor(() => expect(datasetField.textContent).toContain(CHILD));
    const notice = within(panel).getByRole("status");
    expect(notice.textContent).toContain(en["restore.missingPoint"].replace("{date}", isolateLtr(localTime(1_788_990_000))));
    expect(notice.textContent).toContain(en["restore.nearestPoint"].replace("{date}", isolateLtr(localTime(GOOD_AT))));
  });
});
