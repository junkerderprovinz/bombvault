// @vitest-environment jsdom
// The ZFS page and one item's card. The mocked api client serves the item
// list, the host counts and the per-run detail, and records what the card
// sends back, so a toggle that writes nothing or a banner that calls the wrong
// endpoint shows up here.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { AdvancedProvider } from "../lib/advanced";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type {
  Run,
  ZFSDatasetView,
  ZFSDeleteResult,
  ZFSMemberView,
  ZFSRunDetail,
  ZFSSafetySnapshot,
} from "../lib/api";

// jsdom has no EventSource, and the backup tile opens the progress stream.
class NoopEventSource {
  onmessage: ((e: MessageEvent) => void) | null = null;
  close() {}
  addEventListener() {}
  removeEventListener() {}
}
(globalThis as unknown as { EventSource: unknown }).EventSource = NoopEventSource;

let items: ZFSDatasetView[] = [];
let runs: Run[] = [];
let runDetail: ZFSRunDetail = { ok: true, members: [], windowSeconds: -1 };
let safety: ZFSSafetySnapshot[] = [];
let deleteResult: ZFSDeleteResult = { ok: true };
let hostCounts = { notInItem: 0, unusedZvols: 0 };
let domainBusy = false;

const patches: { id: string; body: Record<string, unknown> }[] = [];
const calls: string[] = [];

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listZFSDatasets: () => Promise.resolve({ ok: true, datasets: items }),
    zfsConnection: () =>
      Promise.resolve({
        ok: true,
        code: "ok",
        target: "root@192.168.1.10",
        uriTarget: "",
        version: "2.3.4",
        detail: "",
        zfsBinary: "/usr/sbin/zfs",
        propagation: "slave",
        unpropagated: [],
      }),
    getVMSSH: () => Promise.resolve({ ok: true, host: "tower", publicKey: "ssh-ed25519 AAAA bv" }),
    zfsHostDatasets: () =>
      Promise.resolve({
        ok: true,
        available: true,
        code: "ok",
        target: "root@192.168.1.10",
        datasets: [],
        hiddenLegacy: 0,
        truncated: false,
        maxNameLength: 219,
        ...hostCounts,
      }),
    getSettings: () => Promise.resolve({ ok: true, hostMountRoot: "/host", settings: { perItemSchedules: false } }),
    listRuns: () => Promise.resolve({ ok: true, runs }),
    listRepos: () => Promise.resolve({ ok: true, repos: [] }),
    listContainers: () => Promise.resolve({ ok: true, containers: [] }),
    zfsRunMembers: (runId: string) => {
      calls.push(`members:${runId}`);
      return Promise.resolve(runDetail);
    },
    listZFSSafetySnapshots: (id: string) => {
      calls.push(`safety:${id}`);
      return Promise.resolve({ ok: true, snapshots: safety });
    },
    deleteZFSSafetySnapshot: (id: string, dataset: string, name: string) => {
      calls.push(`safetyDelete:${dataset}@${name}`);
      items = items.map((i) => (i.id === id ? { ...i, safetyCount: 0, safetyOldestAt: 0 } : i));
      return Promise.resolve({ ok: true });
    },
    patchZFSDataset: (id: string, body: Record<string, unknown>) => {
      patches.push({ id, body });
      return Promise.resolve({ ok: true });
    },
    deleteZFSDataset: (id: string, safetyToo: boolean) => {
      calls.push(`delete:${id}:${safetyToo}`);
      if (domainBusy) return Promise.reject(new actual.ApiError(409, "HTTP 409 Conflict"));
      return Promise.resolve(deleteResult);
    },
    sweepZFSDataset: (id: string) => {
      calls.push(`sweep:${id}`);
      return Promise.resolve({ ok: true, remaining: 0 });
    },
    probeZFSDataset: (id: string) => {
      calls.push(`probe:${id}`);
      return Promise.resolve({ ok: true });
    },
  };
});

const { ZFS } = await import("./ZFS");

function member(overrides: Partial<ZFSMemberView>): ZFSMemberView {
  return {
    dataset: "cache/appdata",
    relPath: "",
    hostMountpoint: "/mnt/cache/appdata",
    outcome: "backed-up",
    isNew: false,
    usedByDataset: 4096,
    lastBackupAt: 1_700_000_000,
    ...overrides,
  };
}

function item(overrides: Partial<ZFSDatasetView>): ZFSDatasetView {
  return {
    id: "z1",
    dataset: "cache/appdata",
    enabled: true,
    excludes: [],
    scheduleCadence: "",
    repo: "",
    repoEffective: "ZFS datasets",
    stopContainers: [],
    restartPending: [],
    excludedChildren: [],
    hookContainer: "",
    preSnapshot: "",
    postSnapshot: "",
    hostMountpoint: "/mnt/cache/appdata",
    lastBackup: 1_700_000_000,
    lastRunStatus: "success",
    lastCheckCode: "ok",
    lastCheckDetail: "",
    lastCheckAt: 1_700_000_000,
    leftoverCount: 0,
    safetyCount: 0,
    safetyOldestAt: 0,
    members: [member({})],
    effectiveSchedule: { kind: "domain", spec: "0 3 * * *", alsoSpec: "" },
    ...overrides,
  };
}

function renderPage() {
  return render(
    <I18nProvider>
      <AdvancedProvider>
        <ToastProvider>
          <ZFS />
        </ToastProvider>
      </AdvancedProvider>
    </I18nProvider>,
  );
}

/** Waits for the first item card, which only renders after both fetches. */
async function renderWithItems() {
  const view = renderPage();
  await screen.findByText("cache/appdata");
  return view;
}

beforeEach(() => {
  items = [item({})];
  runs = [];
  runDetail = { ok: true, members: [], windowSeconds: -1 };
  safety = [];
  deleteResult = { ok: true };
  hostCounts = { notInItem: 0, unusedZvols: 0 };
  domainBusy = false;
  patches.length = 0;
  calls.length = 0;
  localStorage.clear();
});

afterEach(cleanup);

describe("ZFS page", () => {
  it("builds the card from the root name, its mountpoint and the shared tiles", async () => {
    await renderWithItems();
    expect(screen.getByText("/mnt/cache/appdata")).toBeTruthy();
    expect(screen.getByRole("button", { name: en["containers.backupNow"] })).toBeTruthy();
    expect(screen.getByRole("button", { name: en["common.edit"] })).toBeTruthy();
    expect(screen.getByRole("button", { name: en["common.delete"] })).toBeTruthy();
    expect(screen.getByText(en["zfs.membersSummary"].replace("{n}", "1"))).toBeTruthy();
  });

  it("uses no native select anywhere", async () => {
    const { container } = await renderWithItems();
    fireEvent.click(screen.getByRole("button", { name: en["common.edit"] }));
    expect(container.querySelector("select")).toBeNull();
  });

  it("shows the failed run rather than the check that says everything is fine", async () => {
    items = [item({ lastRunStatus: "failed", lastCheckCode: "ok" })];
    await renderWithItems();
    expect(screen.getByText(en["run.statusFailed"])).toBeTruthy();
  });

  it("names the reason next to a run the preflight refused", async () => {
    items = [item({ lastRunStatus: "failed", lastCheckCode: "nothing-readable" })];
    await renderWithItems();
    expect(screen.getByText(en["run.statusFailed"])).toBeTruthy();
    expect(screen.getByText(en["zfs.code.nothing-readable"])).toBeTruthy();
  });

  it("says why a delete the server turned away did nothing", async () => {
    domainBusy = true;
    await renderWithItems();
    fireEvent.click(screen.getByRole("button", { name: en["common.delete"] }));
    fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: en["common.delete"] }));
    expect(await screen.findByText("HTTP 409 Conflict")).toBeTruthy();
  });

  it("names where a dataset the run could not see is mounted", async () => {
    items = [
      item({
        members: [
          member({}),
          member({ dataset: "cache/appdata/plex", relPath: "plex", hostMountpoint: "/mnt/cache/appdata/plex" }),
        ],
      }),
    ];
    runs = [
      {
        id: "r3",
        targetId: "z1",
        kind: "backup",
        status: "success",
        startedAt: 1_700_000_000,
        finishedAt: 1_700_000_060,
        snapshotId: "abc",
        bytes: 10,
        error: "",
        acknowledged: true,
        target: "cache/appdata",
        domain: "zfs",
      },
    ];
    const unseen = (dataset: string) => ({
      dataset,
      outcome: "not-visible",
      resticSnapshot: "",
      isNew: false,
      bytesAdded: 0,
      filesNew: 0,
      filesChanged: 0,
      filesUnmodified: 0,
      durationMs: 0,
    });
    runDetail = { ok: true, windowSeconds: -1, members: [unseen("cache/appdata/plex"), unseen("cache/appdata/gone")] };
    await renderWithItems();
    fireEvent.click(await screen.findByRole("button", { name: /→/ }));
    expect(
      await screen.findByText(en["zfs.code.not-visible"].replace("{path}", "/mnt/cache/appdata/plex")),
    ).toBeTruthy();
    expect(screen.getByText(en["zfs.notVisibleNoPath"])).toBeTruthy();
  });

  it("says an item was never checked instead of calling its state unknown", async () => {
    items = [item({ lastCheckCode: "", lastCheckAt: 0 })];
    await renderWithItems();
    expect(screen.getByText(en["zfs.notChecked"])).toBeTruthy();
    expect(screen.queryByText(/Unknown state/)).toBeNull();
  });

  it("counts the skipped datasets and marks the ones it picked up", async () => {
    items = [
      item({
        members: [
          member({}),
          member({ dataset: "cache/appdata/plex", relPath: "plex", isNew: true }),
          member({ dataset: "cache/appdata/enc", relPath: "enc", outcome: "key-not-loaded" }),
        ],
      }),
    ];
    await renderWithItems();
    expect(screen.getByText(en["zfs.skippedCount"].replace("{n}", "1"))).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: en["zfs.membersSummary"].replace("{n}", "3") }));
    expect(screen.getByText(en["zfs.member.new"])).toBeTruthy();
    expect(screen.getByText(en["zfs.code.key-not-loaded"])).toBeTruthy();
  });

  it("names the window a run held the applications for", async () => {
    runs = [
      {
        id: "r1",
        targetId: "z1",
        kind: "backup",
        status: "success",
        startedAt: 1_700_000_000,
        finishedAt: 1_700_000_060,
        snapshotId: "abc",
        bytes: 10,
        error: "",
        acknowledged: true,
        target: "cache/appdata",
        domain: "zfs",
      },
    ];
    runDetail = {
      ok: true,
      windowSeconds: 7,
      members: [
        {
          dataset: "cache/appdata",
          outcome: "backed-up",
          resticSnapshot: "abc",
          isNew: false,
          bytesAdded: 1024,
          filesNew: 2,
          filesChanged: 1,
          filesUnmodified: 9,
          durationMs: 1200,
        },
      ],
    };
    await renderWithItems();
    fireEvent.click(await screen.findByRole("button", { name: /→/ }));
    await waitFor(() => expect(calls).toContain("members:r1"));
    expect(await screen.findByText(en["zfs.window"].replace("{seconds}", "7"))).toBeTruthy();
  });

  it("hides the window on a run that stopped nothing", async () => {
    runs = [
      {
        id: "r2",
        targetId: "z1",
        kind: "backup",
        status: "success",
        startedAt: 1_700_000_000,
        finishedAt: 1_700_000_060,
        snapshotId: "abc",
        bytes: 10,
        error: "",
        acknowledged: true,
        target: "cache/appdata",
        domain: "zfs",
      },
    ];
    runDetail = { ok: true, windowSeconds: -1, members: [] };
    await renderWithItems();
    fireEvent.click(await screen.findByRole("button", { name: /→/ }));
    await waitFor(() => expect(calls).toContain("members:r2"));
    expect(screen.queryByText(/Containers were stopped for/)).toBeNull();
  });

  it("says how many safety snapshots a removed item left behind", async () => {
    items = [item({ safetyCount: 2 })];
    deleteResult = { ok: true, safetyRemaining: 2 };
    await renderWithItems();
    fireEvent.click(screen.getByRole("button", { name: en["common.delete"] }));
    fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: en["common.delete"] }));
    await waitFor(() => expect(calls).toContain("delete:z1:false"));
    expect(await screen.findByText(en["zfs.deleteKeptSafety"].replace("{n}", "2"))).toBeTruthy();
  });

  it("names the containers an interrupted snapshot left stopped", async () => {
    items = [item({ restartPending: ["plex", "sonarr"] })];
    await renderWithItems();
    expect(screen.getByText(en["zfs.restartPending"].replace("{names}", "plex, sonarr"))).toBeTruthy();
    expect(screen.getByLabelText(en["zfs.restartPendingHint"])).toBeTruthy();
  });

  it("sweeps the leftover snapshots from its own banner", async () => {
    items = [item({ leftoverCount: 3 })];
    await renderWithItems();
    expect(screen.getByText(en["zfs.leftovers"].replace("{n}", "3"))).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: en["zfs.removeLeftovers"] }));
    await waitFor(() => expect(calls).toContain("sweep:z1"));
  });

  it("lists the safety snapshots of an item and deletes one", async () => {
    items = [item({ safetyCount: 1 })];
    safety = [
      {
        dataset: "cache/appdata",
        name: "bombvault-prerestore-20260101120000",
        createdAt: 1_700_000_000,
        usedBytes: 2048,
      },
    ];
    await renderWithItems();
    fireEvent.click(screen.getByRole("button", { name: en["zfs.safety.title"].replace("{n}", "1") }));
    await waitFor(() => expect(calls).toContain("safety:z1"));
    const list = await screen.findByRole("list", { name: en["zfs.safety.title"].replace("{n}", "1") });
    expect(within(list).getByText(/bombvault-prerestore-20260101120000/)).toBeTruthy();
    fireEvent.click(within(list).getByRole("button", { name: en["common.delete"] }));
    fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: en["common.delete"] }));
    await waitFor(() =>
      expect(calls).toContain("safetyDelete:cache/appdata@bombvault-prerestore-20260101120000"),
    );
  });

  it("drops the count and the age warning once the last safety snapshot is gone", async () => {
    items = [item({ safetyCount: 1, safetyOldestAt: 1_600_000_000 })];
    safety = [
      {
        dataset: "cache/appdata",
        name: "bombvault-prerestore-20200101120000",
        createdAt: 1_600_000_000,
        usedBytes: 2048,
      },
    ];
    await renderWithItems();
    expect(screen.getByText(en["zfs.safety.old"])).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: en["zfs.safety.title"].replace("{n}", "1") }));
    const list = await screen.findByRole("list", { name: en["zfs.safety.title"].replace("{n}", "1") });
    fireEvent.click(within(list).getByRole("button", { name: en["common.delete"] }));
    fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: en["common.delete"] }));
    await waitFor(() => expect(screen.queryByText(en["zfs.safety.old"])).toBeNull());
    expect(screen.queryByText(en["zfs.safety.title"].replace("{n}", "1"))).toBeNull();
  });

  it("offers to remove an item whose root is gone", async () => {
    items = [item({ lastCheckCode: "not-found", lastRunStatus: "" })];
    await renderWithItems();
    expect(screen.getByText(en["zfs.code.not-found"])).toBeTruthy();
    expect(screen.getByRole("button", { name: en["zfs.removeMissing"] })).toBeTruthy();
  });

  it("saves the schedule switch straight away, with no save button", async () => {
    await renderWithItems();
    fireEvent.click(screen.getByRole("switch", { name: en["zfs.enabled"] }));
    await waitFor(() => expect(patches).toEqual([{ id: "z1", body: { enabled: false } }]));
    expect(screen.queryByRole("button", { name: /save/i })).toBeNull();
  });

  it("counts the volumes no VM uses and says what becomes of them", async () => {
    hostCounts = { notInItem: 4, unusedZvols: 2 };
    await renderWithItems();
    expect(await screen.findByText(en["zfs.notInItem"].replace("{n}", "4"))).toBeTruthy();
    expect(screen.getByText(en["zfs.unusedZvols"].replace("{n}", "2"))).toBeTruthy();
    expect(screen.getByLabelText(en["zfs.unusedZvolsHint"])).toBeTruthy();
  });

  it("leaves the volume line out when every volume belongs to a VM", async () => {
    hostCounts = { notInItem: 4, unusedZvols: 0 };
    await renderWithItems();
    await screen.findByText(en["zfs.notInItem"].replace("{n}", "4"));
    expect(screen.queryByText(/Volumes not used by any VM/)).toBeNull();
  });

  it("keeps the expert controls out of basic mode", async () => {
    await renderWithItems();
    fireEvent.click(screen.getByRole("button", { name: en["common.edit"] }));
    expect(screen.getByText(en["zfs.children"])).toBeTruthy();
    expect(screen.queryByText(en["zfs.excludes"])).toBeNull();
    expect(screen.queryByText(en["zfs.preSnapshot"])).toBeNull();
    expect(screen.queryByRole("button", { name: en["zfs.probe"] })).toBeNull();
  });

  it("offers excludes, commands and the probe in advanced mode", async () => {
    localStorage.setItem("bombvault.advanced", "1");
    await renderWithItems();
    fireEvent.click(screen.getByRole("button", { name: en["common.edit"] }));
    expect(screen.getByText(en["zfs.excludes"])).toBeTruthy();
    expect(screen.getByText(en["zfs.preSnapshot"])).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: en["zfs.probe"] }));
    await waitFor(() => expect(calls).toContain("probe:z1"));
  });

  it("takes snapshot commands only once a container is chosen to run them in", async () => {
    localStorage.setItem("bombvault.advanced", "1");
    await renderWithItems();
    fireEvent.click(screen.getByRole("button", { name: en["common.edit"] }));
    const place = screen.getByRole("combobox", { name: en["zfs.hookContainer"] });
    expect(place.textContent).toContain(en["zfs.hookContainerNone"]);
    expect((screen.getByLabelText(en["zfs.preSnapshot"]) as HTMLInputElement).disabled).toBe(true);
    expect((screen.getByLabelText(en["zfs.postSnapshot"]) as HTMLInputElement).disabled).toBe(true);
  });

  it("offers the commands of an item that has a container for them", async () => {
    localStorage.setItem("bombvault.advanced", "1");
    items = [item({ hookContainer: "postgres", preSnapshot: "pg_backup_start" })];
    await renderWithItems();
    fireEvent.click(screen.getByRole("button", { name: en["common.edit"] }));
    expect((screen.getByLabelText(en["zfs.preSnapshot"]) as HTMLInputElement).disabled).toBe(false);
  });

  it("excludes a child through the tree in the item's settings", async () => {
    items = [
      item({
        members: [member({}), member({ dataset: "cache/appdata/plex", relPath: "plex" })],
      }),
    ];
    await renderWithItems();
    fireEvent.click(screen.getByRole("button", { name: en["common.edit"] }));
    const tree = screen.getByRole("group", { name: en["zfs.children"] });
    fireEvent.click(within(tree).getByRole("switch", { name: "cache/appdata/plex" }));
    await waitFor(() =>
      expect(patches).toEqual([{ id: "z1", body: { excludedChildren: ["cache/appdata/plex"] } }]),
    );
  });

  it("shows the empty state before the first item", async () => {
    items = [];
    renderPage();
    expect(await screen.findByText(en["zfs.emptyTitle"])).toBeTruthy();
    expect(screen.getByLabelText(en["zfs.empty"])).toBeTruthy();
  });

  it("opens the add dialog from the empty state", async () => {
    items = [];
    renderPage();
    await screen.findByText(en["zfs.emptyTitle"]);
    fireEvent.click(screen.getByRole("button", { name: en["zfs.addDatasets"] }));
    expect(await screen.findByRole("dialog", { name: en["zfs.add.title"] })).toBeTruthy();
  });

  it("offers the add dialog next to the count of datasets in no item", async () => {
    hostCounts = { notInItem: 3, unusedZvols: 0 };
    await renderWithItems();
    const note = (await screen.findByText(en["zfs.notInItem"].replace("{n}", "3"))).closest("p");
    fireEvent.click(within(note as HTMLElement).getByRole("button", { name: en["zfs.addDatasets"] }));
    expect(await screen.findByRole("dialog", { name: en["zfs.add.title"] })).toBeTruthy();
  });

  it("opens the restore panel of an item", async () => {
    await renderWithItems();
    fireEvent.click(screen.getByRole("button", { name: en["snapshots.title"] }));
    expect(
      await screen.findByRole("group", { name: en["zfs.restore.title"].replace("{dataset}", "cache/appdata") }),
    ).toBeTruthy();
  });
});
