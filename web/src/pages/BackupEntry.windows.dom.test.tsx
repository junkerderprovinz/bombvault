// @vitest-environment jsdom
/**
 * What the menu of one backup opens on an entry's page.
 *
 * Marking, comparing, exporting and deleting a backup each have a window of
 * their own. Each has to ask the server about the place the backup lies at,
 * and say what the server answered as a sentence.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import type { BackupDiff, ExportPreview, TimelineRow } from "../lib/api";
import { ENTRY_ROUTE } from "../lib/backupEntry";
import { humanBytes } from "../lib/forecast";
import { I18nProvider, en } from "../lib/i18n";
import { AnomalyProvider } from "../lib/useAnomalies";
import { BOX, ZFS_ID, backupRun, serveEntries, timelineOf } from "./backupEntry.testsupport";

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
    zfsRestorePoints: vi.fn(),
    getZFSReplica: vi.fn(),
    checkRestore: vi.fn(),
    tagBackup: vi.fn(),
    diffBackups: vi.fn(),
    previewBackupExport: vi.fn(),
    exportBackup: vi.fn(),
    cancelBackup: vi.fn(),
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
const getTimeline = vi.mocked(api.getTimeline);
const getTimelineDeletePreview = vi.mocked(api.getTimelineDeletePreview);
const tagBackup = vi.mocked(api.tagBackup);
const diffBackups = vi.mocked(api.diffBackups);
const previewBackupExport = vi.mocked(api.previewBackupExport);
const exportBackup = vi.mocked(api.exportBackup);
const cancelBackup = vi.mocked(api.cancelBackup);

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

const streams: FakeEventSource[] = [];

class FakeEventSource {
  onmessage: ((ev: MessageEvent<string>) => void) | null = null;
  constructor(_url: string) {
    streams.push(this);
  }
  close(): void {}
}

function progress(frame: Record<string, unknown>) {
  act(() => {
    streams.at(-1)?.onmessage?.({ data: JSON.stringify(frame) } as MessageEvent<string>);
  });
}

async function renderPage(path: string) {
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
}

async function press(el: HTMLElement) {
  await act(async () => {
    fireEvent.click(el);
  });
}

// Three backups: today, yesterday and three days ago.
const RUNS = [0, 1, 3].map((daysAgo, i) => backupRun(`r${i}`, daysAgo));
const IMMICH_PATH = "/backups/container/immich";

const PREVIEW: ExportPreview = {
  folder: "/mnt/user/backups/bombvault/export",
  files: ["immich-20261006-034700.tar.gz"],
  replaces: [],
  bytes: 65 * 1024 ** 2,
  encryption: "none",
  key: "export:containers",
};

const DIFF: BackupDiff = {
  addedFiles: 2,
  removedFiles: 1,
  changedFiles: 12,
  addedBytes: 5 * 1024 ** 2,
  removedBytes: 1024 ** 2,
  changes: [
    { path: "/config/new.yaml", modifier: "+" },
    { path: "/logs/old.log", modifier: "-" },
    { path: "/data/library.db", modifier: "M" },
  ],
};

const rows = () => screen.queryAllByTestId("entry-backup");
const window_ = () => screen.getByRole("dialog");

async function openMenu(row = 0) {
  await press(within(rows()[row]).getByRole("button", { name: en["nav.more"] }));
  return within(screen.getByRole("menu"));
}

async function choose(row: number, action: string) {
  const menu = await openMenu(row);
  await press(menu.getByRole("menuitem", { name: action }));
}

/** The staged timeline with the first row changed. */
function withFirstRow(change: (row: TimelineRow) => void, more = [] as (typeof BOX)[]) {
  const timeline = timelineOf(RUNS, more);
  change(timeline.rows[0]);
  getTimeline.mockResolvedValue(timeline);
}

beforeEach(() => {
  streams.length = 0;
  localStorage.clear();
  vi.stubGlobal("EventSource", FakeEventSource);
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => ({ ok: true, json: async () => ({ ok: false }) }))
  );
  serveEntries(mocks, { runs: RUNS });
  vi.mocked(api.listRuns).mockResolvedValue({ ok: true, runs: [] });
  vi.mocked(api.getAnomalies).mockResolvedValue({ ok: true, anomalies: [], nextCursor: "" });
  vi.mocked(api.getAnomalySummary).mockResolvedValue({ ok: false });
  vi.mocked(api.getAnomalyItems).mockResolvedValue({ ok: true, items: [] });
  getTimeline.mockResolvedValue(timelineOf(RUNS));
  vi.mocked(api.getTimelinePlace).mockResolvedValue({ ok: false });
  getTimelineDeletePreview.mockResolvedValue({ ok: true, delete: [], others: [] });
  vi.mocked(api.listDbDumps).mockResolvedValue({ ok: true, dumps: [] });
  vi.mocked(api.zfsHostDatasets).mockResolvedValue({ ok: false } as never);
  vi.mocked(api.zfsRestorePoints).mockResolvedValue({ ok: true, points: [] } as never);
  vi.mocked(api.getZFSReplica).mockRejectedValue(new Error("no replica"));
  vi.mocked(api.checkRestore).mockResolvedValue({ ok: false } as never);
  tagBackup.mockResolvedValue({ ok: true });
  diffBackups.mockResolvedValue({ ok: true, diff: DIFF });
  previewBackupExport.mockResolvedValue({ ok: true, export: PREVIEW });
  exportBackup.mockResolvedValue({ ok: true, started: true, export: PREVIEW });
  cancelBackup.mockResolvedValue({ ok: true, cancelled: true });
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

describe("a row of the backup list", () => {
  it("shows the size its backup read and the tag a person set", async () => {
    withFirstRow((row) => {
      row.size = 3 * 1024 ** 3;
      row.places[0].tags = ["container:immich", "bvrun:17", "before-upgrade"];
    });
    await renderPage(IMMICH_PATH);
    const first = within(rows()[0]);
    expect(first.getByTestId("entry-backup-size").textContent).toBe(humanBytes(3 * 1024 ** 3));
    expect(first.getByText("before-upgrade")).toBeTruthy();
    expect(first.queryByText("container:immich")).toBeNull();
    expect(first.queryByText("bvrun:17")).toBeNull();
  });

  it("falls back to what the run measured where the timeline has no size", async () => {
    await renderPage(IMMICH_PATH);
    expect(within(rows()[0]).getByTestId("entry-backup-size").textContent).toBe(humanBytes(RUNS[0].sourceBytes ?? 0));
  });

  it("does not open the backup when its menu is used", async () => {
    await renderPage(IMMICH_PATH);
    await openMenu();
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});

describe("the menu of a backup", () => {
  it("offers to compare, mark, export and delete it", async () => {
    await renderPage(IMMICH_PATH);
    const menu = await openMenu();
    expect(menu.getAllByRole("menuitem").map((item) => item.textContent)).toEqual([
      en["entry.row.compare"],
      en["snapshot.addTag"],
      en["entry.export.action"],
      en["common.delete"],
    ]);
  });

  it("offers to change the tag of a backup that has one", async () => {
    withFirstRow((row) => {
      row.places[0].tags = ["keep"];
    });
    await renderPage(IMMICH_PATH);
    const menu = await openMenu();
    expect(menu.getByRole("menuitem", { name: en["entry.row.tagChange"] })).toBeTruthy();
  });

  it("has no delete where the only place is append-only, and no compare for the only backup", async () => {
    const one = timelineOf(RUNS.slice(0, 1));
    one.places[0] = { ...one.places[0], appendOnly: true };
    getTimeline.mockResolvedValue(one);
    await renderPage(IMMICH_PATH);
    const menu = await openMenu();
    expect(menu.queryByRole("menuitem", { name: en["common.delete"] })).toBeNull();
    expect(menu.queryByRole("menuitem", { name: en["entry.row.compare"] })).toBeNull();
  });
});

describe("the tag window", () => {
  const field = () => within(window_()).getByRole("textbox", { name: en["entry.tag.title"] });

  async function type(text: string) {
    await act(async () => {
      fireEvent.change(field(), { target: { value: text } });
    });
    await press(within(window_()).getByRole("button", { name: en["settings.save"] }));
  }

  it("marks the backup at its place and reads the list again", async () => {
    await renderPage(IMMICH_PATH);
    await choose(0, en["snapshot.addTag"]);
    const before = getTimeline.mock.calls.length;
    await type("before-upgrade");
    expect(tagBackup).toHaveBeenCalledWith("containers", "immich", RUNS[0].snapshotId, ["before-upgrade"], [], "local");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(getTimeline.mock.calls.length).toBeGreaterThan(before);
  });

  it("changes a tag by taking the old one away", async () => {
    withFirstRow((row) => {
      row.places[0].tags = ["container:immich", "old"];
    });
    await renderPage(IMMICH_PATH);
    await choose(0, en["entry.row.tagChange"]);
    expect((field() as HTMLInputElement).value).toBe("old");
    await type("new");
    expect(tagBackup).toHaveBeenCalledWith("containers", "immich", RUNS[0].snapshotId, ["new"], ["old"], "local");
  });

  it("takes the tag away when the field is emptied", async () => {
    withFirstRow((row) => {
      row.places[0].tags = ["old"];
    });
    await renderPage(IMMICH_PATH);
    await choose(0, en["entry.row.tagChange"]);
    await type("  ");
    expect(tagBackup).toHaveBeenCalledWith("containers", "immich", RUNS[0].snapshotId, [], ["old"], "local");
  });

  it("writes at the place off the premises when only that one holds the backup", async () => {
    withFirstRow(
      (row) => {
        row.places = [{ place: BOX.place, snapshotIds: ["copy"], tags: [] }];
      },
      [{ ...BOX, state: "read" }]
    );
    await renderPage(IMMICH_PATH);
    await choose(0, en["snapshot.addTag"]);
    await type("keep");
    expect(tagBackup).toHaveBeenCalledWith("containers", "immich", "copy", ["keep"], [], BOX.place);
  });

  it("says in a sentence that the server keeps a tag for itself", async () => {
    tagBackup.mockResolvedValue({ ok: false, error: 'invalid tag "p1": this tag is reserved for BombVault' });
    await renderPage(IMMICH_PATH);
    await choose(0, en["snapshot.addTag"]);
    await type("p1");
    expect(within(window_()).getByRole("alert").textContent).toBe(en["entry.tag.reserved"].replace("{tag}", "p1"));
  });

  it("says what else the server refused with", async () => {
    tagBackup.mockResolvedValue({ ok: false, code: "domain-busy", error: "busy" });
    await renderPage(IMMICH_PATH);
    await choose(0, en["snapshot.addTag"]);
    await type("keep");
    expect(within(window_()).getByRole("alert").textContent).toBe(en["placementCode.domainBusy"]);
  });

  it("asks the server nothing when the tag stayed the same", async () => {
    await renderPage(IMMICH_PATH);
    await choose(0, en["snapshot.addTag"]);
    await press(within(window_()).getByRole("button", { name: en["settings.save"] }));
    expect(tagBackup).not.toHaveBeenCalled();
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});

describe("the comparison", () => {
  const compare = () => press(within(window_()).getByRole("button", { name: en["snapshot.compare"] }));

  it("compares the backup with the one before it and lists what came, went and changed", async () => {
    await renderPage(IMMICH_PATH);
    await choose(0, en["entry.row.compare"]);
    await compare();
    expect(diffBackups).toHaveBeenCalledWith("containers", "immich", RUNS[1].snapshotId, RUNS[0].snapshotId, "local");
    const result = within(await screen.findByTestId("compare-result"));
    expect(result.getByText(/12 files changed, 2 new, 1 deleted/)).toBeTruthy();
    expect(result.getByText(new RegExp(en["entry.compare.more"].replace("{size}", humanBytes(4 * 1024 ** 2))))).toBeTruthy();
    for (const path of ["/config/new.yaml", "/logs/old.log", "/data/library.db"]) expect(result.getByText(path)).toBeTruthy();
    expect(result.queryByText(en["entry.compare.truncated"])).toBeNull();
  });

  it("says when the server stopped listing paths", async () => {
    diffBackups.mockResolvedValue({ ok: true, diff: { ...DIFF, truncated: true } });
    await renderPage(IMMICH_PATH);
    await choose(0, en["entry.row.compare"]);
    await compare();
    expect(await screen.findByText(en["entry.compare.truncated"])).toBeTruthy();
  });

  it("names the dataset a path of a ZFS run lies in and the datasets only one run has", async () => {
    diffBackups.mockResolvedValue({
      ok: true,
      diff: {
        ...DIFF,
        changes: [{ part: "tank/appdata/db", path: "/base/1", modifier: "M" }],
        onlyAfter: ["tank/appdata/new"],
      },
    });
    await renderPage(`/backups/zfs/${ZFS_ID}`);
    await choose(0, en["entry.row.compare"]);
    await compare();
    expect(diffBackups).toHaveBeenCalledWith("zfs", ZFS_ID, RUNS[1].snapshotId, RUNS[0].snapshotId, "local");
    const result = within(await screen.findByTestId("compare-result"));
    expect(result.getByText(/tank\/appdata\/db/)).toBeTruthy();
    expect(result.getByText(en["entry.compare.onlyAfter"].replace("{list}", "tank/appdata/new"))).toBeTruthy();
  });

  it("says of a VM how much of its disks differ and that the guest stays closed", async () => {
    diffBackups.mockResolvedValue({
      ok: true,
      diff: { ...DIFF, disks: true, changes: [{ path: "/vdisk1.img", modifier: "M" }] },
    });
    await renderPage("/backups/vm/win11");
    await choose(0, en["entry.row.compare"]);
    await compare();
    const result = within(await screen.findByTestId("compare-result"));
    expect(
      result.getByText(new RegExp(en["entry.compare.diskBytes"].replace("{size}", humanBytes(DIFF.addedBytes))))
    ).toBeTruthy();
    expect(result.getByText(new RegExp(en["entry.compare.guest"]))).toBeTruthy();
    expect(result.getByText("/vdisk1.img")).toBeTruthy();
    expect(result.queryByText(/12 files changed/)).toBeNull();
  });

  it("asks for two different backups before it asks the server", async () => {
    await renderPage(IMMICH_PATH);
    await choose(0, en["entry.row.compare"]);
    await press(within(window_()).getByRole("combobox", { name: en["entry.compare.from"] }));
    await press(screen.getAllByRole("option")[0]);
    await compare();
    expect(diffBackups).not.toHaveBeenCalled();
    expect(within(window_()).getByRole("alert").textContent).toBe(en["entry.compare.pickTwo"]);
  });

  it("says why a comparison did not work", async () => {
    diffBackups.mockResolvedValue({ ok: false, error: "repository locked" });
    await renderPage(IMMICH_PATH);
    await choose(0, en["entry.row.compare"]);
    await compare();
    expect((await within(window_()).findByRole("alert")).textContent).toBe("repository locked");
  });
});

describe("the export", () => {
  const start = () => press(within(window_()).getByRole("button", { name: en["entry.export.action"] }));

  it("names the folder and the files before anything is written", async () => {
    await renderPage(IMMICH_PATH);
    await choose(0, en["entry.export.action"]);
    expect(previewBackupExport).toHaveBeenCalledWith(
      "containers",
      "immich",
      { form: "tar", snapshotId: RUNS[0].snapshotId },
      "local"
    );
    const preview = within(await screen.findByTestId("export-preview"));
    expect(preview.getByText(PREVIEW.folder)).toBeTruthy();
    expect(preview.getByText(new RegExp(PREVIEW.files[0].replaceAll(".", "\\.")))).toBeTruthy();
    expect(exportBackup).not.toHaveBeenCalled();
  });

  it("asks again for a repository and says which key it opens with and where that is", async () => {
    previewBackupExport.mockImplementation(async (_domain, _key, request) => ({
      ok: true,
      export:
        request.form === "repository"
          ? { ...PREVIEW, files: ["immich-restic/"], backups: request.snapshotId ? 1 : 3, bytes: undefined, encryption: "password" }
          : PREVIEW,
    }));
    await renderPage(IMMICH_PATH);
    await choose(0, en["entry.export.action"]);
    await press(within(window_()).getByRole("tab", { name: "restic" }));
    expect(previewBackupExport).toHaveBeenLastCalledWith(
      "containers",
      "immich",
      { form: "repository", snapshotId: RUNS[0].snapshotId },
      "local"
    );
    expect(await within(window_()).findByText(en["entry.export.keyPassword"])).toBeTruthy();
    expect(within(window_()).getByRole("link", { name: en["activityLog.typeRestore"] }).getAttribute("href")).toBe(
      "/settings/restore"
    );

    await press(within(window_()).getByRole("tab", { name: en["entry.export.scopeEvery"] }));
    expect(previewBackupExport).toHaveBeenLastCalledWith(
      "containers",
      "immich",
      { form: "repository", snapshotId: undefined },
      "local"
    );
    expect(await within(window_()).findByText(/3 backups/)).toBeTruthy();
  });

  it("says that sealed archives need the private key", async () => {
    previewBackupExport.mockResolvedValue({ ok: true, export: { ...PREVIEW, encryption: "age" } });
    await renderPage(IMMICH_PATH);
    await choose(0, en["entry.export.action"]);
    expect(await within(window_()).findByText(en["entry.export.keyAge"])).toBeTruthy();
  });

  it("warns about the files an export replaces", async () => {
    previewBackupExport.mockResolvedValue({ ok: true, export: { ...PREVIEW, replaces: PREVIEW.files } });
    await renderPage(IMMICH_PATH);
    await choose(0, en["entry.export.action"]);
    expect(
      await within(window_()).findByText(en["entry.export.replaces"].replace("{files}", PREVIEW.files[0]))
    ).toBeTruthy();
  });

  it("shows why the server cannot export, and starts nothing", async () => {
    previewBackupExport.mockResolvedValue({ ok: false, error: "the folder immich-restic is not empty" });
    await renderPage(IMMICH_PATH);
    await choose(0, en["entry.export.action"]);
    expect((await within(window_()).findByRole("alert")).textContent).toBe("the folder immich-restic is not empty");
    expect((within(window_()).getByRole("button", { name: en["entry.export.action"] }) as HTMLButtonElement).disabled).toBe(
      true
    );
  });

  it("starts the export, follows its run and ends on the path it wrote", async () => {
    await renderPage(IMMICH_PATH);
    await choose(0, en["entry.export.action"]);
    await screen.findByTestId("export-preview");
    await start();
    expect(exportBackup).toHaveBeenCalledWith(
      "containers",
      "immich",
      { form: "tar", snapshotId: RUNS[0].snapshotId },
      "local"
    );
    progress({ key: PREVIEW.key, phase: "maintenance", percent: 40, active: true });
    expect(within(window_()).getByRole("progressbar").getAttribute("aria-valuenow")).toBe("40");
    progress({ key: PREVIEW.key, phase: "maintenance", percent: 100, active: false });
    const result = within(await screen.findByTestId("export-result"));
    expect(result.getByText(`${PREVIEW.folder}/${PREVIEW.files[0]}`)).toBeTruthy();
    expect(within(window_()).getByRole("button", { name: en["common.done"] })).toBeTruthy();
  });

  it("stops a running export through its progress key", async () => {
    await renderPage(IMMICH_PATH);
    await choose(0, en["entry.export.action"]);
    await screen.findByTestId("export-preview");
    await start();
    progress({ key: PREVIEW.key, phase: "maintenance", percent: 10, active: true });
    await press(within(window_()).getByRole("button", { name: en["entry.export.cancel"] }));
    expect(cancelBackup).toHaveBeenCalledWith(PREVIEW.key);
    mocks.listItemRuns.mockResolvedValue({
      ok: true,
      runs: [backupRun("x1", 0, { kind: "export", status: "cancelled" })],
    });
    progress({ key: PREVIEW.key, phase: "maintenance", percent: 0, active: false });
    expect(await within(window_()).findByText(en["entry.export.cancelled"])).toBeTruthy();
  });

  it("says why an export failed, from the run it recorded", async () => {
    await renderPage(IMMICH_PATH);
    await choose(0, en["entry.export.action"]);
    await screen.findByTestId("export-preview");
    await start();
    mocks.listItemRuns.mockResolvedValue({
      ok: true,
      runs: [backupRun("x1", 0, { kind: "export", status: "failed", error: "no space left on device" })],
    });
    progress({ key: PREVIEW.key, phase: "maintenance", percent: 0, active: false });
    expect((await within(window_()).findByRole("alert")).textContent).toBe("no space left on device");
    expect(screen.queryByTestId("export-result")).toBeNull();
  });

  it("says that something else is running when the server will not start", async () => {
    exportBackup.mockResolvedValue({ ok: false, error: "a backup or restore is already running" });
    await renderPage(IMMICH_PATH);
    await choose(0, en["entry.export.action"]);
    await screen.findByTestId("export-preview");
    await start();
    expect((await within(window_()).findByRole("alert")).textContent).toBe(en["entry.export.busy"]);
  });
});

describe("deleting a backup", () => {
  it("asks the server at once about a backup that lies at one place", async () => {
    await renderPage(IMMICH_PATH);
    await choose(0, en["common.delete"]);
    await waitFor(() =>
      expect(getTimelineDeletePreview).toHaveBeenCalledWith("containers", "immich", RUNS[0].snapshotId, [])
    );
  });

  it("asks first where to delete a backup that lies at two places", async () => {
    withFirstRow(
      (row) => {
        row.places.push({ place: BOX.place, snapshotIds: ["copy"], tags: [] });
      },
      [{ ...BOX, state: "read" }]
    );
    await renderPage(IMMICH_PATH);
    await choose(0, en["common.delete"]);
    expect(getTimelineDeletePreview).not.toHaveBeenCalled();
    const choices = within(window_()).getAllByRole("radio");
    expect(choices).toHaveLength(3);
    expect(choices[2].getAttribute("aria-checked")).toBe("true");
    await press(within(window_()).getByRole("radio", { name: en["entry.delete.only"].replace("{place}", "Storage Box") }));
    await press(within(window_()).getByRole("button", { name: en["common.delete"] }));
    await waitFor(() =>
      expect(getTimelineDeletePreview).toHaveBeenCalledWith("containers", "immich", RUNS[0].snapshotId, [BOX.place])
    );
  });

  it("says which place keeps its copy because it is append-only", async () => {
    const vault = { ...BOX, place: "offsite:t2", label: "Vault", state: "read" as const, appendOnly: true };
    withFirstRow(
      (row) => {
        row.places.push(
          { place: BOX.place, snapshotIds: ["copy"], tags: [] },
          { place: vault.place, snapshotIds: ["locked"], tags: [] }
        );
      },
      [{ ...BOX, state: "read" }, vault]
    );
    await renderPage(IMMICH_PATH);
    await choose(0, en["common.delete"]);
    expect(within(window_()).getAllByRole("radio")).toHaveLength(3);
    expect(within(window_()).getByText(en["offsiteRemoval.appendOnly"].replace("{target}", "Vault"))).toBeTruthy();
  });
});

describe("a ZFS entry", () => {
  it("lists its runs like every other kind and opens one in the panel of its kind", async () => {
    await renderPage(`/backups/zfs/${ZFS_ID}`);
    expect(getTimeline).toHaveBeenCalledWith("zfs", ZFS_ID);
    expect(rows()).toHaveLength(3);
    await press(rows()[0]);
    expect(screen.getByText(en["entry.restore.title"].replace("{name}", "tank/appdata"))).toBeTruthy();
    await waitFor(() => expect(vi.mocked(api.zfsRestorePoints)).toHaveBeenCalledWith(ZFS_ID, "local"));
  });
});
