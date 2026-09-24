// @vitest-environment jsdom
// Restoring a ZFS dataset writes over live data, so the panel has to refuse
// what the server would refuse and say why before the click, and it must never
// write without the safety snapshot unless that was answered twice.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { AdvancedProvider } from "../../lib/advanced";
import { I18nProvider, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import type {
  ZFSDatasetView,
  ZFSHostDataset,
  ZFSMemberView,
  ZFSRestoreAck,
  ZFSRestorePoint,
  ZFSRestoreRequest,
} from "../../lib/api";

// jsdom has no EventSource, and the restore progress banner subscribes to it.
class NoopEventSource {
  onmessage: ((e: MessageEvent) => void) | null = null;
  close() {}
  addEventListener() {}
  removeEventListener() {}
}
(globalThis as unknown as { EventSource: unknown }).EventSource = NoopEventSource;

let points: ZFSRestorePoint[] = [];
let ack: ZFSRestoreAck = { ok: true, started: true, target: "/mnt/cache/appdata" };
const sent: { id: string; req: ZFSRestoreRequest; source?: string }[] = [];

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    zfsRestorePoints: () => Promise.resolve({ ok: true, points }),
    restoreZFS: (id: string, req: ZFSRestoreRequest, source?: string) => {
      sent.push({ id, req, source });
      return Promise.resolve(ack);
    },
    listSnapshotFilesZFS: () => Promise.resolve({ ok: true, files: [] }),
    listRuns: () => Promise.resolve({ ok: true, runs: [] }),
    listOffsiteTargets: () => Promise.resolve({ ok: true, targets: [] }),
    browse: () => Promise.resolve({ ok: true, dirs: [] }),
  };
});

const { ZFSRestorePanel } = await import("./ZFSRestorePanel");

const STAMP = "bombvault-20260101120000";

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

function item(overrides: Partial<ZFSDatasetView> = {}): ZFSDatasetView {
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
    members: [member({}), member({ dataset: "cache/appdata/plex", relPath: "/plex" })],
    safetyOldestAt: 0,
    effectiveSchedule: { kind: "domain", spec: "0 3 * * *", alsoSpec: "" },
    ...overrides,
  };
}

function hostEntry(overrides: Partial<ZFSHostDataset>): ZFSHostDataset {
  return {
    dataset: "cache/appdata",
    type: "filesystem",
    hostMountpoint: "/mnt/cache/appdata",
    referenced: 1024,
    used: 4096,
    usedByDataset: 1024,
    mounted: true,
    encrypted: false,
    keyLoaded: true,
    visible: true,
    writable: true,
    memberCode: "",
    managedId: "z1",
    coveredBy: "",
    vmDisk: false,
    system: false,
    vmVolume: false,
    blockers: [],
    ...overrides,
  };
}

function renderPanel(view = item(), host = new Map<string, ZFSHostDataset>()) {
  return render(
    <I18nProvider>
      <AdvancedProvider>
        <ToastProvider>
          <ZFSRestorePanel item={view} host={host} hostMountRoot="/host" restoreFolder="user/restore" />
        </ToastProvider>
      </AdvancedProvider>
    </I18nProvider>,
  );
}

/** Opens the panel and waits for the restore points. */
async function openPanel(view = item(), host = new Map<string, ZFSHostDataset>()) {
  const rendered = renderPanel(view, host);
  fireEvent.click(screen.getByRole("button", { name: en["snapshots.title"] }));
  await screen.findByRole("combobox", { name: en["zfs.restore.dataset"] });
  return rendered;
}

/** The mode strip's tab for one destination. */
function modeTab(label: string) {
  return screen.getByRole("tab", { name: label });
}

beforeEach(() => {
  points = [
    {
      stamp: STAMP,
      time: 1_700_000_000,
      members: [
        { dataset: "cache/appdata", relPath: "", snapshotId: "aaa", outcome: "backed-up" },
        { dataset: "cache/appdata/plex", relPath: "/plex", snapshotId: "bbb", outcome: "backed-up" },
      ],
    },
  ];
  ack = { ok: true, started: true, target: "/mnt/cache/appdata" };
  sent.length = 0;
  localStorage.clear();
});

afterEach(cleanup);

describe("ZFS restore panel", () => {
  it("asks for the restore point, then the dataset, then what to do with it", async () => {
    await openPanel();
    const body = screen.getByRole("group", {
      name: en["zfs.restore.title"].replace("{dataset}", "cache/appdata"),
    });
    const text = body.textContent ?? "";
    expect(text.indexOf(en["zfs.restore.point"])).toBeGreaterThanOrEqual(0);
    expect(text.indexOf(en["zfs.restore.point"])).toBeLessThan(text.indexOf(en["zfs.restore.dataset"]));
    expect(text.indexOf(en["zfs.restore.dataset"])).toBeLessThan(text.indexOf(en["zfs.restore.inPlace"]));
  });

  it("keeps selecting files, the whole tree and the source out of basic mode", async () => {
    await openPanel();
    expect(modeTab(en["zfs.restore.inPlace"])).toBeTruthy();
    expect(modeTab(en["zfs.restore.toFolder"])).toBeTruthy();
    expect(screen.queryByRole("tab", { name: en["zfs.restore.selectFiles"] })).toBeNull();
    expect(screen.queryByRole("tab", { name: en["source.local"] })).toBeNull();
    fireEvent.click(screen.getByRole("combobox", { name: en["zfs.restore.dataset"] }));
    expect(screen.queryByRole("option", { name: en["zfs.restore.wholeTree"] })).toBeNull();
  });

  it("adds them in advanced mode", async () => {
    localStorage.setItem("bombvault.advanced", "1");
    await openPanel();
    expect(modeTab(en["zfs.restore.selectFiles"])).toBeTruthy();
    expect(screen.getByRole("tab", { name: en["source.local"] })).toBeTruthy();
    fireEvent.click(screen.getByRole("combobox", { name: en["zfs.restore.dataset"] }));
    expect(screen.getByRole("option", { name: en["zfs.restore.wholeTree"] })).toBeTruthy();
  });

  it("will not write into a dataset the server has not mounted", async () => {
    await openPanel(item({ members: [member({ outcome: "not-mounted" })] }));
    expect(modeTab(en["zfs.restore.inPlace"]).hasAttribute("disabled")).toBe(true);
    expect(screen.getByText(en["zfs.restore.missingDataset"])).toBeTruthy();
  });

  it("restores the whole tree into a folder only, without calling the dataset missing", async () => {
    localStorage.setItem("bombvault.advanced", "1");
    await openPanel();
    fireEvent.click(screen.getByRole("combobox", { name: en["zfs.restore.dataset"] }));
    fireEvent.click(screen.getByRole("option", { name: en["zfs.restore.wholeTree"] }));
    expect(modeTab(en["zfs.restore.inPlace"]).hasAttribute("disabled")).toBe(true);
    expect(modeTab(en["zfs.restore.selectFiles"]).hasAttribute("disabled")).toBe(true);
    expect(modeTab(en["zfs.restore.toFolder"]).getAttribute("aria-selected")).toBe("true");
    expect(screen.queryByText(en["zfs.restore.missingDataset"])).toBeNull();
    expect(screen.queryByRole("switch", { name: en["zfs.restore.safetySnapshot"] })).toBeNull();
  });

  it("will not write through a read-only mapping", async () => {
    const host = new Map([["cache/appdata", hostEntry({ writable: false })]]);
    await openPanel(item(), host);
    expect(modeTab(en["zfs.restore.inPlace"]).hasAttribute("disabled")).toBe(true);
    expect(screen.getByText(en["zfs.code.read-only-mount"])).toBeTruthy();
  });

  it("takes the safety snapshot without being asked", async () => {
    await openPanel();
    expect(
      screen.getByRole("switch", { name: en["zfs.restore.safetySnapshot"] }).getAttribute("aria-checked"),
    ).toBe("true");
    fireEvent.click(screen.getByRole("button", { name: en["snapshots.restore"] }));
    fireEvent.click(
      within(await screen.findByRole("dialog")).getByRole("button", { name: en["snapshots.restore"] }),
    );
    await waitFor(() => expect(sent).toHaveLength(1));
    expect(sent[0].req).toEqual({
      stamp: STAMP,
      dataset: "cache/appdata",
      wholeTree: false,
      paths: [],
      targetPath: "",
      confirm: true,
      safetySnapshot: true,
      safetyOffConfirm: false,
      stopContainers: false,
    });
  });

  it("asks a second time before it restores without one", async () => {
    await openPanel();
    const safety = screen.getByRole("switch", { name: en["zfs.restore.safetySnapshot"] });
    fireEvent.click(safety);
    fireEvent.click(
      within(await screen.findByRole("dialog")).getByRole("button", { name: en["common.confirm"] }),
    );
    await waitFor(() => expect(safety.getAttribute("aria-checked")).toBe("false"));
    fireEvent.click(screen.getByRole("button", { name: en["snapshots.restore"] }));
    fireEvent.click(
      within(await screen.findByRole("dialog")).getByRole("button", { name: en["snapshots.restore"] }),
    );
    await waitFor(() => expect(sent).toHaveLength(1));
    expect(sent[0].req.safetySnapshot).toBe(false);
    expect(sent[0].req.safetyOffConfirm).toBe(true);
  });

  it("offers the stop switch only to an item that stops containers", async () => {
    await openPanel();
    expect(screen.queryByRole("switch", { name: /Stop/ })).toBeNull();
    cleanup();
    await openPanel(item({ stopContainers: ["plex", "sonarr"] }));
    const stop = screen.getByRole("switch", {
      name: en["zfs.restore.stopContainers"].replace("{names}", "plex, sonarr"),
    });
    expect(stop.getAttribute("aria-checked")).toBe("true");
  });

  it("shows a refusal the server coded as its sentence alone", async () => {
    ack = { ok: false, code: "read-only-mount", error: "read-only-mount: cache/appdata" };
    await openPanel();
    fireEvent.click(screen.getByRole("button", { name: en["snapshots.restore"] }));
    fireEvent.click(
      within(await screen.findByRole("dialog")).getByRole("button", { name: en["snapshots.restore"] }),
    );
    expect(await screen.findByText(en["zfs.code.read-only-mount"])).toBeTruthy();
    expect(screen.queryByText(/read-only-mount: cache\/appdata/)).toBeNull();
  });

  it("names the host mountpoint in a refusal about the dataset's mount", async () => {
    ack = { ok: false, code: "not-visible", error: "not-visible: cache/appdata" };
    await openPanel();
    fireEvent.click(screen.getByRole("button", { name: en["snapshots.restore"] }));
    fireEvent.click(
      within(await screen.findByRole("dialog")).getByRole("button", { name: en["snapshots.restore"] }),
    );
    expect(
      await screen.findByText(en["zfs.code.not-visible"].replace("{path}", "/mnt/cache/appdata")),
    ).toBeTruthy();
  });

  it("names the safety snapshot it took, ready to copy", async () => {
    ack = {
      ok: true,
      started: true,
      target: "/host/mnt/cache/appdata",
      safetySnapshot: "cache/appdata@bombvault-prerestore-20260101120000",
    };
    await openPanel();
    fireEvent.click(screen.getByRole("button", { name: en["snapshots.restore"] }));
    fireEvent.click(
      within(await screen.findByRole("dialog")).getByRole("button", { name: en["snapshots.restore"] }),
    );
    expect(
      await screen.findByText(
        en["zfs.restore.safetyTaken"].replace(
          "{snapshot}",
          "cache/appdata@bombvault-prerestore-20260101120000",
        ),
      ),
    ).toBeTruthy();
    expect(
      screen.getByText(en["zfs.restore.started"].replace("{target}", "/mnt/cache/appdata")),
    ).toBeTruthy();
    expect(screen.getByRole("button", { name: en["common.copy"] })).toBeTruthy();
  });
});
