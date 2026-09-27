// @vitest-environment jsdom
// A dataset tree from another server's repository is restored into a folder,
// either whole, one run instant of every dataset, or one dataset of it.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { ForeignInventory, Snapshot } from "../lib/api";

class NoopEventSource {
  onmessage: ((e: MessageEvent) => void) | null = null;
  close() {}
  addEventListener() {}
  removeEventListener() {}
}
(globalThis as unknown as { EventSource: unknown }).EventSource = NoopEventSource;

let foreignInventory: ForeignInventory;
const foreignRestore = vi.fn((req: unknown) => {
  void req;
  return Promise.resolve({ ok: true, started: true });
});
const listForeignFiles = vi.fn((...args: unknown[]) => {
  void args;
  return Promise.resolve({
    ok: true,
    files: [{ path: "/mnt/cache/appdata/plex/Preferences.xml", type: "file", size: 10 }],
  });
});

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getSettings: () =>
      Promise.resolve({
        ok: true,
        settings: {
          restoreFolder: "restore",
          configPath: "backups/config",
          configOffsite: "",
          containersPath: "backups/containers",
          vmsPath: "backups/vms",
          flashPath: "backups/flash",
          filesPath: "backups/files",
          zfsPath: "backups/zfs",
          encryptionEnabled: true,
        },
        hostMountRoot: "/host/user",
      }),
    detectEncryption: () => Promise.resolve({ ok: false }),
    discover: () => Promise.resolve({ ok: true, discovered: 0 }),
    discoverVMs: () => Promise.resolve({ ok: true, discovered: 0 }),
    discoverFiles: () => Promise.resolve({ ok: true, discovered: 0 }),
    discoverZFS: () => Promise.resolve({ ok: true, discovered: 0 }),
    listContainers: () => Promise.resolve({ ok: true, containers: [] }),
    listVMs: () => Promise.resolve({ ok: true, vms: [] }),
    listFileSets: () => Promise.resolve({ ok: true, fileSets: [] }),
    listZFSDatasets: () => Promise.resolve({ ok: true, datasets: [] }),
    listRuns: () => Promise.resolve({ ok: true, runs: [] }),
    getVMSSH: () => Promise.resolve({ ok: true, host: "tower" }),
    foreignOpen: () => Promise.resolve({ ok: true, session: "s1", inventory: foreignInventory }),
    foreignClose: () => Promise.resolve({ ok: true }),
    foreignRestore: (req: unknown) => foreignRestore(req),
    listForeignFiles: (...a: unknown[]) => listForeignFiles(...a),
  };
});

const Recovery = (await import("./Recovery")).default;

function snap(id: string, dataset: string, stamp: string, time: string): Snapshot {
  const mount = "/mnt/" + dataset;
  return {
    id,
    time,
    paths: [`${mount}/.zfs/snapshot/${stamp}`],
    tags: ["bombvault", "zfs:" + dataset],
    hostname: "other",
  };
}

async function connect() {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <Recovery />
        </ToastProvider>
      </I18nProvider>
    );
  });
  const field = (label: string) =>
    (screen.getByText(label).closest("div") as HTMLElement).querySelector("input") as HTMLInputElement;
  fireEvent.change(field(en["recovery.foreignKey"]), { target: { value: "a".repeat(64) } });
  fireEvent.change(field(en["recovery.foreignLocation"]), { target: { value: "backups/other" } });
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: en["recovery.foreignConnect"] }));
  });
}

/** The row of the tree, the only foreign item in the repository. */
function treeRow(): HTMLElement {
  return screen.getByRole("button", { name: en["recovery.foreignRestore"] }).closest(".glim-hue") as HTMLElement;
}

function pick(row: HTMLElement, label: string, option: string) {
  fireEvent.click(within(row).getByRole("combobox", { name: label }));
  fireEvent.click(screen.getByRole("option", { name: option }));
}

beforeEach(() => {
  foreignRestore.mockClear();
  foreignInventory = {
    containers: [],
    vms: [],
    fileSets: [],
    dbDumps: [],
    zfs: [
      {
        name: "cache/appdata",
        snapshots: [
          snap("a1a1a1a1a1", "cache/appdata", "bombvault-20260901T030000Z", "2026-09-01T03:00:00Z"),
          snap("b1b1b1b1b1", "cache/appdata/plex", "bombvault-20260901T030000Z", "2026-09-01T03:00:05Z"),
          snap("a2a2a2a2a2", "cache/appdata", "bombvault-20260902T030000Z", "2026-09-02T03:00:00Z"),
          snap("b2b2b2b2b2", "cache/appdata/plex", "bombvault-20260902T030000Z", "2026-09-02T03:00:05Z"),
        ],
      },
    ],
  };
});

afterEach(cleanup);

describe("a dataset tree from another server", () => {
  it("restores every dataset of the tree into the chosen folder", async () => {
    await connect();
    const row = treeRow();
    const restore = within(row).getByRole("button", { name: en["recovery.foreignRestore"] });
    expect((restore as HTMLButtonElement).disabled).toBe(true);

    fireEvent.change(within(row).getByPlaceholderText("user/appdata"), { target: { value: "restore/other" } });
    await act(async () => {
      fireEvent.click(restore);
    });

    expect(foreignRestore).toHaveBeenCalledTimes(1);
    expect(foreignRestore.mock.calls[0][0]).toMatchObject({
      session: "s1",
      domain: "zfs",
      item: "cache/appdata",
      snapshot: "latest",
      target: "restore/other",
      wholeTree: true,
    });
  });

  it("offers each run of the whole tree once", async () => {
    await connect();
    fireEvent.click(within(treeRow()).getByRole("combobox", { name: en["recovery.foreignLatest"] }));
    // Latest plus one entry per run, not one per dataset snapshot.
    expect(screen.getAllByRole("option")).toHaveLength(3);
  });

  it("restores one dataset from its own snapshots", async () => {
    await connect();
    const row = treeRow();
    pick(row, en["zfs.restore.dataset"], "cache/appdata/plex");

    fireEvent.click(within(row).getByRole("combobox", { name: en["recovery.foreignLatest"] }));
    const offered = screen.getAllByRole("option").map((o) => o.textContent ?? "");
    expect(offered).toHaveLength(3);
    expect(offered.slice(1).every((label) => label.includes("b2b2b2b2") || label.includes("b1b1b1b1"))).toBe(true);
    fireEvent.click(screen.getByRole("option", { name: new RegExp("b1b1b1b1") }));

    fireEvent.change(within(row).getByPlaceholderText("user/appdata"), { target: { value: "restore/plex" } });
    await act(async () => {
      fireEvent.click(within(row).getByRole("button", { name: en["recovery.foreignRestore"] }));
    });

    expect(foreignRestore.mock.calls[0][0]).toMatchObject({
      domain: "zfs",
      item: "cache/appdata/plex",
      snapshot: "b1b1b1b1b1",
      target: "restore/plex",
      wholeTree: false,
    });
  });

  it("restores picked files of one dataset", async () => {
    await connect();
    const row = treeRow();
    pick(row, en["zfs.restore.dataset"], "cache/appdata/plex");
    await act(async () => {
      fireEvent.click(within(row).getByRole("switch", { name: en["zfs.restore.selectFiles"] }));
    });
    expect(listForeignFiles).toHaveBeenCalledWith("s1", "cache/appdata/plex", "latest", "zfs");

    fireEvent.click(within(row).getByRole("checkbox", { name: "/mnt" }));
    fireEvent.change(within(row).getByPlaceholderText("user/appdata"), { target: { value: "restore/plex" } });
    await act(async () => {
      fireEvent.click(within(row).getByRole("button", { name: en["recovery.foreignRestore"] }));
    });

    expect(foreignRestore.mock.calls[0][0]).toMatchObject({
      item: "cache/appdata/plex",
      paths: ["/mnt"],
      wholeTree: false,
    });
  });
});
