// @vitest-environment jsdom
/**
 * The window that restores an entry of any kind.
 *
 * It offers what the server can do for the kind and no more, starts on the
 * safe target, and sends each choice to the route that takes it. Replacing
 * the original asks for its name first.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";

import type { AnomalyView, RestoreCheckRequest, Run, TimelinePlace, ZFSRestorePoint } from "../../lib/api";
import type { Entry, EntryDetail } from "../../lib/entry";
import { I18nProvider, en } from "../../lib/i18n";
import {
  BOX,
  CONFIG,
  FLASH,
  IMMICH,
  PHOTOS,
  PIHOLE,
  SETTINGS,
  TANK,
  WIN11,
  backupRun,
  containerView,
  fileSetView,
  timelineOf,
  vmView,
  zfsView,
} from "../../pages/backupEntry.testsupport";
import type { TimelineState } from "../entry/BackupPlaces";

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    checkRestore: vi.fn(),
    restore: vi.fn(),
    restoreVM: vi.fn(),
    restoreFileSet: vi.fn(),
    restoreFileSetFiles: vi.fn(),
    restoreContainerFiles: vi.fn(),
    restoreContainerToPath: vi.fn(),
    restoreZFS: vi.fn(),
    zfsRestorePoints: vi.fn(),
    zfsHostDatasets: vi.fn(),
    listSnapshotFiles: vi.fn(),
    listDbDumps: vi.fn(),
    listFlashPlugins: vi.fn(),
    listRuns: vi.fn(),
    getTimelineDeletePreview: vi.fn(),
    browse: vi.fn(),
  };
});

vi.mock("../../lib/toast", () => ({ useToast: () => ({ push: () => {} }) }));

let openFindings: AnomalyView[] = [];
vi.mock("../../lib/useAnomalies", () => ({
  useOpenAnomalies: () => ({
    list: openFindings,
    byRunId: new Map(),
    flagged: new Set(openFindings.flatMap((a) => a.flaggedSnapshots ?? [])),
    error: false,
  }),
}));

const api = await import("../../lib/api");
const checkRestore = vi.mocked(api.checkRestore);
const restore = vi.mocked(api.restore);
const restoreVM = vi.mocked(api.restoreVM);
const restoreFileSet = vi.mocked(api.restoreFileSet);
const restoreContainerFiles = vi.mocked(api.restoreContainerFiles);
const restoreContainerToPath = vi.mocked(api.restoreContainerToPath);
const restoreZFS = vi.mocked(api.restoreZFS);
const zfsRestorePoints = vi.mocked(api.zfsRestorePoints);
const listSnapshotFiles = vi.mocked(api.listSnapshotFiles);
const listRuns = vi.mocked(api.listRuns);

const { RestoreWindow } = await import("./RestoreWindow");
const { RESTORE_NO_RUNTIME } = await import("../../lib/runReason");

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

// Three backups: today, yesterday and three days ago.
const RUNS = [0, 1, 3].map((daysAgo, i) => backupRun(`r${i}`, daysAgo));
const NEWEST = RUNS[0].snapshotId;

function timelineState(domain: TimelineState["domain"], more: TimelinePlace[] = []): TimelineState {
  return {
    ...timelineOf(RUNS, more),
    loading: false,
    error: null,
    reading: new Set(),
    loadPlace: vi.fn(async () => undefined),
    reload: vi.fn(),
    domain,
  };
}

const DETAIL: Record<string, EntryDetail> = {
  container: { kind: "container", container: containerView(), installed: [containerView()], linkCandidates: [] },
  vm: { kind: "vm", vm: vmView(), linkCandidates: [] },
  files: { kind: "files", set: fileSetView() },
  zfs: { kind: "zfs", dataset: zfsView() },
  flash: { kind: "flash" },
  config: { kind: "config" },
};
const ITEM = { container: IMMICH, vm: WIN11, files: PHOTOS, zfs: TANK, flash: FLASH, config: CONFIG };
const DOMAIN = { container: "containers", vm: "vms", files: "files", zfs: "zfs", flash: "flash", config: "config" } as const;

function entryOf(kind: keyof typeof ITEM, over: Partial<Entry> = {}): Entry {
  return { item: ITEM[kind], detail: DETAIL[kind], settings: SETTINGS, hostMountRoot: "/host/user", ...over };
}

function Elsewhere() {
  const { pathname, search } = useLocation();
  return <p>{`at ${pathname}${search}`}</p>;
}

const onClose = vi.fn();

async function open(entry: Entry, props: { timeline?: TimelineState; snapshot?: string; place?: string; deletable?: boolean } = {}) {
  const timeline = props.timeline ?? timelineState(DOMAIN[entry.item.kind]);
  await act(async () => {
    render(
      <I18nProvider>
        <MemoryRouter initialEntries={["/here"]}>
          <Routes>
            <Route
              path="/here"
              element={
                <RestoreWindow
                  entry={entry}
                  name={entry.item.name}
                  timeline={timeline}
                  ask={{ snapshot: props.snapshot, place: props.place }}
                  deletable={props.deletable}
                  onClose={onClose}
                />
              }
            />
            <Route path="*" element={<Elsewhere />} />
          </Routes>
        </MemoryRouter>
      </I18nProvider>
    );
  });
  return timeline;
}

async function press(el: HTMLElement) {
  await act(async () => {
    fireEvent.click(el);
  });
}

const dialog = () => screen.getAllByRole("dialog")[0];
// The confirmation opens above the window.
const question = () => screen.getAllByRole("dialog").at(-1)!;
const target = (key: "beside" | "replace" | "zip") =>
  within(dialog()).queryByRole("radio", { name: new RegExp(`^${en[`restoreWindow.${key}`]}`) });
const toFolder = () => within(dialog()).queryByRole("radio", { name: new RegExp(`^${en["restore.mode.toFolder"]}`) });
const named = (key: "restoreWindow.besideNamed" | "restoreWindow.replaceNamed" | "entry.restore.title", name: string) =>
  within(dialog()).getByRole("button", { name: en[key].replace("{name}", name) }) as HTMLButtonElement;

/** The check the window asked for last. */
function lastCheck(): RestoreCheckRequest {
  return checkRestore.mock.calls.at(-1)![0];
}

async function ready(button: () => HTMLButtonElement) {
  await waitFor(() => expect(button().disabled).toBe(false), { timeout: 3000 });
}

/** Types the name the confirmation asks for and confirms. */
async function typeAndConfirm(name: string) {
  expect(screen.getAllByRole("dialog")).toHaveLength(2);
  const ask = question();
  const confirmButton = within(ask).getByRole("button", { name: en["restoreWindow.replaceNamed"].replace("{name}", name) });
  expect((confirmButton as HTMLButtonElement).disabled).toBe(true);
  fireEvent.change(within(ask).getByRole("textbox"), { target: { value: name } });
  await press(confirmButton);
}

beforeEach(() => {
  openFindings = [];
  onClose.mockClear();
  vi.stubGlobal("EventSource", NoopEventSource);
  checkRestore.mockImplementation(async (req) => ({
    ok: true,
    ready: true,
    checks: [{ id: "repository", status: "ok" }],
    plan:
      req.target === "beside"
        ? ({ beside: { name: `${req.name}-restored`, folders: [] }, files: [], definition: [], shared: [] } as never)
        : null,
  }));
  for (const start of [restore, restoreVM, restoreFileSet, restoreContainerFiles, restoreContainerToPath]) {
    start.mockResolvedValue({ ok: true, started: true });
  }
  restoreZFS.mockResolvedValue({ ok: true, started: true, target: "/mnt/tank/appdata" } as never);
  zfsRestorePoints.mockResolvedValue({ ok: true, points: [] } as never);
  vi.mocked(api.zfsHostDatasets).mockResolvedValue({ ok: false } as never);
  listSnapshotFiles.mockResolvedValue({ ok: true, files: [{ path: "/config/app.db", type: "file", size: 10 }] } as never);
  vi.mocked(api.listDbDumps).mockResolvedValue({ ok: true, dumps: [] });
  vi.mocked(api.listFlashPlugins).mockResolvedValue({ ok: true, plugins: [] } as never);
  vi.mocked(api.getTimelineDeletePreview).mockResolvedValue({ ok: true, delete: [], others: [] } as never);
  listRuns.mockResolvedValue({ ok: true, runs: [] });
});

afterEach(() => {
  vi.useRealTimers();
  cleanup();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

describe("a container", () => {
  it("starts on the copy next to the original and names it as the check answered", async () => {
    await open(entryOf("container"));
    expect(target("beside")?.getAttribute("aria-checked")).toBe("true");
    await ready(() => named("restoreWindow.besideNamed", "immich"));
    expect(lastCheck()).toEqual({ kind: "container", name: "immich", snapshotId: NEWEST, source: "local", target: "beside" });
    expect(within(target("beside")!).getByText(en["restoreWindow.besideSub"].replace("{name}", "immich-restored"))).toBeTruthy();

    await press(named("restoreWindow.besideNamed", "immich"));
    expect(restore).toHaveBeenCalledWith("immich", NEWEST, true, "local", false, false, { target: "beside" });
    expect(screen.getAllByRole("dialog")).toHaveLength(1);
  });

  it("replaces the original only once its name is typed, and backs it up first", async () => {
    await open(entryOf("container"));
    await press(target("replace")!);
    await ready(() => named("restoreWindow.replaceNamed", "immich"));
    expect(lastCheck()).toMatchObject({ kind: "container", backupFirst: true });
    expect(lastCheck().target).toBeUndefined();

    await press(named("restoreWindow.replaceNamed", "immich"));
    expect(restore).not.toHaveBeenCalled();
    await typeAndConfirm("immich");
    expect(restore).toHaveBeenCalledWith("immich", NEWEST, true, "local", false, false, { backupFirst: true });
  });

  it("leaves the backup of the current state out when its switch is off", async () => {
    await open(entryOf("container"));
    await press(target("replace")!);
    await press(screen.getByRole("switch", { name: en["restoreWindow.backupFirst"] }));
    await press(screen.getByRole("switch", { name: en["restoreWindow.leaveStopped"] }));
    await press(screen.getByRole("switch", { name: en["restore.withoutRuntime"] }));
    await ready(() => named("restoreWindow.replaceNamed", "immich"));
    await press(named("restoreWindow.replaceNamed", "immich"));
    await typeAndConfirm("immich");
    expect(restore).toHaveBeenCalledWith("immich", NEWEST, true, "local", true, true, {});
  });

  it("restores into the folder its section is set to", async () => {
    await open(entryOf("container", { settings: { ...SETTINGS, restoreFolders: { containers: "user/restore/containers" } } }));
    await press(toFolder()!);
    const start = () => within(dialog()).getByRole("button", { name: en["restoreWindow.toFolderNamed"] }) as HTMLButtonElement;
    await ready(start);
    expect(lastCheck()).toMatchObject({ kind: "containerTo", targetPath: "user/restore/containers" });
    await press(start());
    expect(restoreContainerToPath).toHaveBeenCalledWith("immich", NEWEST, "user/restore/containers", "local");
  });

  it("sends ticked files to the files route, which knows no copy next to the original", async () => {
    await open(entryOf("container"));
    await press(within(dialog()).getByRole("tab", { name: en["restore.mode.files"] }));
    expect(target("beside")).toBeNull();
    expect(screen.queryByRole("switch", { name: en["restoreWindow.backupFirst"] })).toBeNull();
    await waitFor(() => expect(listSnapshotFiles).toHaveBeenCalledWith("immich", NEWEST, "local"));
    await press(await within(dialog()).findByRole("checkbox", { name: "/config" }));
    await press(toFolder()!);
    const start = () => within(dialog()).getByRole("button", { name: en["restoreWindow.toFolderNamed"] }) as HTMLButtonElement;
    await ready(start);
    await press(start());
    expect(restoreContainerFiles).toHaveBeenCalledWith("immich", NEWEST, ["/config"], "user/bombvault/restore", true, "local");
  });

  it("lists the database dumps of a database", async () => {
    const container = containerView({ dbTier: "curated", dbEngine: "postgres" });
    await open(entryOf("container", { detail: { kind: "container", container, installed: [container], linkCandidates: [] } }));
    expect(within(dialog()).getByText(/PostgreSQL/)).toBeTruthy();
    await press(within(dialog()).getByRole("tab", { name: en["dbdump.listTitle"] }));
    expect(within(dialog()).queryByRole("radiogroup")).toBeNull();
    expect(vi.mocked(api.listDbDumps)).toHaveBeenCalledWith("immich", "local");
  });

  it("recreates one that is gone without asking for its name", async () => {
    const gone = containerView({ name: "pihole", installed: false, state: "" });
    await open(entryOf("container", { item: PIHOLE, detail: { kind: "container", container: gone, installed: [], linkCandidates: [] } }));
    expect(target("beside")).toBeNull();
    await ready(() => named("entry.restore.title", "pihole"));
    await press(named("entry.restore.title", "pihole"));
    expect(restore).toHaveBeenCalledWith("pihole", NEWEST, true, "local", false, false, {});
  });

  it("takes the backup from the storage location that was asked for", async () => {
    const timeline = timelineState("containers", [{ ...BOX, state: "read" }]);
    timeline.rows[0].places.push({ place: BOX.place, snapshotIds: ["copy"], tags: [] });
    await open(entryOf("container"), { timeline, place: BOX.place });
    expect(within(dialog()).getByRole("tab", { name: "Storage Box" }).getAttribute("aria-selected")).toBe("true");
    await ready(() => named("restoreWindow.besideNamed", "immich"));
    expect(lastCheck()).toMatchObject({ snapshotId: "copy", source: BOX.place });
  });

  it("offers the next storage location when the chosen one lost the backup", async () => {
    const timeline = timelineState("containers", [{ ...BOX, state: "read" }]);
    timeline.rows[0].places.push({ place: BOX.place, snapshotIds: ["copy"], tags: [] });
    restore.mockResolvedValueOnce({ ok: false, code: "snapshot-missing" } as never);
    await open(entryOf("container"), { timeline });
    await ready(() => named("restoreWindow.besideNamed", "immich"));
    await press(named("restoreWindow.besideNamed", "immich"));
    await waitFor(() => expect(timeline.loadPlace).toHaveBeenCalledWith("local"));
    expect(await screen.findByText(/Storage Box\?$/)).toBeTruthy();
  });

  it("offers to delete the backup where the list of an entry opened it", async () => {
    await open(entryOf("container"), { deletable: true });
    await press(within(dialog()).getByRole("button", { name: en["common.delete"] }));
    await waitFor(() =>
      expect(vi.mocked(api.getTimelineDeletePreview)).toHaveBeenCalledWith("containers", "immich", NEWEST, ["local"])
    );
  });
});

describe("the recommended backup", () => {
  const finding = {
    id: "a1",
    severity: "critical",
    state: "open",
    domain: "container",
    name: "immich",
    targetId: "tg-immich",
    metric: "source_bytes_shrink",
    scopeKind: "item",
    observed: 1,
    expected: 2,
    details: {},
    lastGood: { runId: "r1", snapshotId: RUNS[1].snapshotId, at: RUNS[1].finishedAt },
    flaggedSnapshots: [NEWEST],
  } as unknown as AnomalyView;

  it("is the last good one before an open finding", async () => {
    openFindings = [finding];
    await open(entryOf("container"));
    await ready(() => named("restoreWindow.besideNamed", "immich"));
    expect(lastCheck()).toMatchObject({ snapshotId: RUNS[1].snapshotId });
  });

  it("is offered again once another backup is picked", async () => {
    openFindings = [finding];
    await open(entryOf("container"), { snapshot: NEWEST });
    expect(within(dialog()).getByText(en["anomaly.snapshotFlagged"])).toBeTruthy();
    await press(within(dialog()).getByRole("button", { name: en["restoreWindow.useThis"] }));
    await ready(() => named("restoreWindow.besideNamed", "immich"));
    expect(lastCheck()).toMatchObject({ snapshotId: RUNS[1].snapshotId });
    expect(within(dialog()).queryByRole("button", { name: en["restoreWindow.useThis"] })).toBeNull();
  });
});

describe("a VM", () => {
  it("comes back next to the original or over it, as a whole", async () => {
    await open(entryOf("vm"));
    expect(target("beside")?.getAttribute("aria-checked")).toBe("true");
    expect(toFolder()).toBeNull();
    expect(within(dialog()).queryByRole("tab", { name: en["restore.mode.files"] })).toBeNull();
    await ready(() => named("restoreWindow.besideNamed", "Windows 11"));
    await press(named("restoreWindow.besideNamed", "Windows 11"));
    expect(restoreVM).toHaveBeenCalledWith("win11", NEWEST, true, "local", false, { target: "beside" });
  });

  it("stays off after a replace when asked to", async () => {
    await open(entryOf("vm"));
    await press(target("replace")!);
    await press(screen.getByRole("switch", { name: en["restoreWindow.leaveStopped"] }));
    await ready(() => named("restoreWindow.replaceNamed", "Windows 11"));
    await press(named("restoreWindow.replaceNamed", "Windows 11"));
    await typeAndConfirm("Windows 11");
    expect(restoreVM).toHaveBeenCalledWith("win11", NEWEST, true, "local", true, { backupFirst: true });
  });
});

describe("a folder set", () => {
  it("lands in a new folder next to its own", async () => {
    await open(entryOf("files"));
    await ready(() => named("restoreWindow.besideNamed", "Photos"));
    await press(named("restoreWindow.besideNamed", "Photos"));
    expect(restoreFileSet).toHaveBeenCalledWith(PHOTOS.key, NEWEST, true, "", "local", { target: "beside" });
  });

  it("only restores into a folder when it does not know where it came from", async () => {
    await open(entryOf("files", { detail: { kind: "files", set: fileSetView({ path: "" }) } }));
    expect(target("beside")).toBeNull();
    expect(target("replace")).toBeNull();
    expect(toFolder()?.getAttribute("aria-checked")).toBe("true");
  });
});

describe("the flash drive", () => {
  it("leaves as a ZIP and writes nothing", async () => {
    await open(entryOf("flash"));
    expect(target("zip")?.getAttribute("aria-checked")).toBe("true");
    expect(target("replace")).toBeNull();
    await waitFor(() => expect(lastCheck()).toEqual({ kind: "flash", snapshotId: NEWEST, source: "local" }));
    expect(within(dialog()).getByText(new RegExp(en["restoreWindow.says.zipFlash"]))).toBeTruthy();
  });

  it("lists the plugins of the backup", async () => {
    await open(entryOf("flash"));
    await press(within(dialog()).getByRole("tab", { name: en["flash.plugins"] }));
    await waitFor(() => expect(vi.mocked(api.listFlashPlugins)).toHaveBeenCalledWith(NEWEST, "local"));
  });
});

describe("a ZFS entry", () => {
  const point: ZFSRestorePoint = {
    stamp: NEWEST,
    time: RUNS[0].finishedAt ?? 0,
    members: [
      { dataset: "tank/appdata", relPath: "", snapshotId: "m1", outcome: "backed-up" },
      { dataset: "tank/appdata/db", relPath: "db", snapshotId: "m2", outcome: "backed-up" },
    ],
  };

  beforeEach(() => {
    zfsRestorePoints.mockResolvedValue({ ok: true, points: [point] } as never);
  });

  it("goes into a new dataset named after the original", async () => {
    await open(entryOf("zfs"));
    await waitFor(() => expect(zfsRestorePoints).toHaveBeenCalledWith(TANK.key, "local"));
    const fresh = () => within(dialog()).getByRole("radio", { name: new RegExp(`^${en["zfs.restore.newDataset"]}`) });
    await waitFor(() => expect(fresh().getAttribute("aria-checked")).toBe("true"));
    await press(fresh());
    expect((within(dialog()).getByRole("textbox", { name: en["zfs.restore.newDatasetName"] }) as HTMLInputElement).value).toBe(
      "tank/appdata-restored"
    );
    await waitFor(() =>
      expect(lastCheck().zfs).toMatchObject({ stamp: NEWEST, dataset: "tank/appdata", newDataset: "tank/appdata-restored" })
    );
  });

  it("writes into the dataset with a safety snapshot once its name is typed", async () => {
    await open(entryOf("zfs"));
    const inPlace = () => within(dialog()).getByRole("radio", { name: new RegExp(`^${en["zfs.restore.inPlace"]}`) });
    await waitFor(() => expect(inPlace()).toBeTruthy());
    await press(inPlace());
    await ready(() => named("restoreWindow.replaceNamed", "tank/appdata"));
    await press(named("restoreWindow.replaceNamed", "tank/appdata"));
    await typeAndConfirm("tank/appdata");
    expect(restoreZFS).toHaveBeenCalledWith(
      TANK.key,
      expect.objectContaining({ stamp: NEWEST, dataset: "tank/appdata", targetPath: "", safetySnapshot: true, confirm: true }),
      "local"
    );
  });

  it("only offers a folder and a new dataset for a dataset the host no longer has", async () => {
    await open(entryOf("zfs", { detail: { kind: "zfs", dataset: zfsView({ lastCheckCode: "not-found" }) } }));
    await waitFor(() => expect(within(dialog()).getByText(en["zfs.restore.missingDataset"])).toBeTruthy());
    expect(within(dialog()).queryByRole("radio", { name: new RegExp(`^${en["zfs.restore.inPlace"]}`) })).toBeNull();
    expect(toFolder()).toBeTruthy();
    expect(within(dialog()).queryByRole("tab", { name: en["restore.mode.files"] })).toBeNull();
  });
});

describe("the self-backup", () => {
  it("is sent to the emergency window, because the running instance keeps its settings", async () => {
    await open(entryOf("config"));
    expect(screen.getByText(en["restoreWindow.self.text"])).toBeTruthy();
    await press(screen.getByRole("button", { name: en["entry.config.startRecovery"] }));
    expect(screen.getByText("at /restore?emergency=settings")).toBeTruthy();
  });
});

describe("a restore that was started", () => {
  function finished(over: Partial<Run> = {}): Run {
    return backupRun("restore-1", 0, { kind: "restore", ...over });
  }

  it("draws its check and says where the copy stands", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    restore.mockImplementation(async () => {
      listRuns.mockResolvedValue({ ok: true, runs: [finished()] });
      return { ok: true, started: true };
    });
    await open(entryOf("container"));
    await act(() => vi.advanceTimersByTimeAsync(1000));
    await press(named("restoreWindow.besideNamed", "immich"));
    await act(() => vi.advanceTimersByTimeAsync(10_000));
    const done = screen.getByTestId("restore-done");
    expect(done.textContent).toBe(en["restoreWindow.doneBeside"].replace("{name}", "immich-restored"));
    expect(done.querySelector(".glim-check-draw")).not.toBeNull();
    expect(screen.getByText(en["restoreWindow.copyListed"])).toBeTruthy();
  });

  it("offers the retry without GPU and runtime when Docker refused them", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    restore.mockImplementation(async () => {
      listRuns.mockResolvedValue({ ok: true, runs: [finished({ status: "failed", error: RESTORE_NO_RUNTIME })] });
      return { ok: true, started: true };
    });
    await open(entryOf("container"));
    await act(() => vi.advanceTimersByTimeAsync(1000));
    await press(named("restoreWindow.besideNamed", "immich"));
    await act(() => vi.advanceTimersByTimeAsync(10_000));
    await press(screen.getByRole("button", { name: en["restore.withoutRuntime"] }));
    expect(restore).toHaveBeenLastCalledWith("immich", NEWEST, true, "local", false, true, { target: "beside" });
  });
});
